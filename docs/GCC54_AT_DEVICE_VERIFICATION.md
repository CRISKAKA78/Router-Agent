# GCC 5.4 一键编译与真实设备 AT 验证

## 结果（2026-09-11）

用户已授权在 `root@10.1.1.128` 编译，使用桌面的 `gcc-5.4.tar.gz`，上传至 SSH `47.119.168.150:20004` 测试。工作仍在 `codex/at-discovery`；本轮未提交/推送。原设备 Probe 和拨号实例保留，仅启动并最终停止了独立测试实例。

**实际设备链路已通过，不再只是 PTY 模拟：**

| 项目 | 实测结果 |
|---|---|
| 设备 | Four-Faith 固件，MediaTek MT7621，Linux 4.4.198，MIPS32r2 小端/uClibc |
| 自动选中端口 | `/dev/ttyUSB1`，未指定厂家、USB端口号或VID/PID |
| ATI | Fibocom Wireless Inc. / FM160-CN |
| 固件标识 | `89641.1000.00.01.04.18`，SVN 18 |
| IMEI | `AT+CGSN` 独立查询成功，严格15位；本说明脱敏为 `860********1604` |
| 其他端口 | ttyUSB0握手超时；正常轮次选择ttyUSB1后，ttyUSB2/3标alternate，不继续查询 |
| 周期及数据链 | 测试模板10秒；真实Probe → SSH转发的隔离控制连接 → 当前Server → Cellular HTTP快照，连续两轮同Session |
| 真实占用 | 测试句柄仅打开ttyUSB1、不读写字节；探针将其标busy且ATI/IMEI均not_queried。本轮ttyUSB2也被观测为busy，探针未强抢 |
| 释放恢复 | 关闭本测试占用句柄，后续周期重新选中ttyUSB1并取得IMEI |
| 关闭采集 | 通过公开模板API更新并应用关闭配置，修订增加、最新蜂窝快照清空 |
| 原进程 | redial PID1419、原Probe PID9339的PID及/proc启动时间前后一致；测试控制SSH/心跳未中断 |
| 清理 | 临时Probe、占用句柄、测试Server与SSH转发已停止；原实例仍运行，测试二进制和日志保留 |

未执行模组重启、拨号重启、USB拔插或修改系统tty节点。真实重枚举/长期业务零影响仍未验收；ttyUSB2→ttyUSB9改号恢复已有上一轮PTY回归，不能写成这次物理热插拔通过。此次证明FM160-CN当前固件的通用身份查询，无厂商专用适配，SIM/信号/基站字段未扩展。

## 后续一键编译

在本worktree根目录：

1. 放置一个名为 `password.txt` 的文件，内容为 **10.1.1.128编译机root账号密码**，单行纯文本；可有末尾换行，不能含其他说明。无需在脚本填写密码。
2. 首次安装SDK需桌面有 `gcc-5.4.tar.gz`。本次已经安装成功，以后默认复用，不需再次上传；不覆盖主机已有 `/root/gcc-5.4`。
3. 双击 [probe-build-gcc54.cmd](../probe-build-gcc54.cmd)。它调用 [PowerShell入口](../probe-build-gcc54.ps1)；无密码或接口选择提示。窗口末尾的pause仅用于保留结果。
4. 查看 `SUCCESS` 和退出码。脚本只编译，不自动上传设备、重启拨号或替换运行探针。

```powershell
# 在仓库根目录运行；默认与双击入口相同
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\probe-build-gcc54.ps1

# 可选参数：外置密码文件、SDK压缩包或独立产物根目录
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\probe-build-gcc54.ps1 -PasswordFile C:\private\password.txt -ToolchainArchive C:\SDK\gcc-5.4.tar.gz -RemoteRoot /root/router-probe-gcc54
```

默认账号 `root@10.1.1.128`、SSH端口22，可通过 `-SshTarget`、`-Port` 调整；仍是该MIPS/uClibc GCC5.4专用入口，不是架构自动选择器。旧 [probe-build.cmd](../probe-build.cmd) 保持GCC5.2 ARM用途，本轮不改变其含义。

### 输出位置

```text
/root/router-probe-gcc54/
  output/router-probe              最近成功的MIPS可执行文件
  output/MBEDTLS-LICENSE.txt
  output/THIRD-PARTY.md
  latest-build.txt                 指向最近成功运行目录
  toolchain/gcc-5.4/               来自桌面压缩包的独立SDK缓存
  runs/<时间-随机标识>/
    input/probe/                  本次上传源码（包含未提交更新）
    src/probe/                    兼容处理后的实际编译副本
    gcc54-compat.patch            兼容修改证据
    build-driver.sh               去除CR后的脚本
    toolchain.cmake
    build.log
    verification.log
    output/router-probe           本次独立产物
```

每次使用独立run，不删除历史记录。远端flock串行化SDK安装/构建及输出发布；成功后才原子更新固定产物和latest-build指针，失败返回非零。首次SDK压缩包保留在对应安装run内。SDK缓存固定使用本次安装的工具链；若要另换SDK，使用新 `-RemoteRoot`，不要以为更换本地压缩包就自动覆盖既有缓存。

### 密码与主机身份

- `password.txt` 已加入仓库根 `.gitignore`，源包仅打包Probe和构建驱动，不上传该文件、Git目录或运行数据。忽略规则不等于加密；密码文件应只授权本机用户读取。
- [askpass助手](../scripts/ssh-password-file.cmd) 从文件读取密码供OpenSSH认证，密码不写入脚本、命令行或构建日志，不创建远端密码文件。本轮为验证创建的临时密码文件已删除，未替用户永久保存密码。
- 使用Windows自带OpenSSH/PowerShell/tar，无额外SSH库。首次主机密钥按TOFU保存到 `build/gcc54/known_hosts`，后续身份变化会拒绝，不自动删除或跳过校验。首次连接未做独立带外指纹认证，不声称已验证对端组织身份。
- 默认免交互流程已真实运行：从脚本同目录password.txt读取密码；SDK缓存存在时，本地SDK路径故意不存在仍编译成功。错误密码不循环重试，缺失/空/多行密码在上传前报错。

## 编译事实与兼容处理

编译机Ubuntu x86_64、CMake3.28.3、Python3.12.3，支持32位SDK宿主可执行文件；SDK为LEDE GCC5.4.0，target `mipsel-openwrt-linux-uclibc`。脚本需要bash、cmake、make、python3（支持tarfile data过滤）、file/readelf、flock、diff，以及运行该32位编译器所需的宿主库，当前机器已具备；不自动安装或升级系统依赖。

本次使用桌面451267385字节压缩包首次上传并在新目录解包，不复用主机已有同名SDK。解包检查成员路径/类型及链接边界，拒绝越界，不覆盖已有未标记SDK目录。

首轮真实编译暴露该SDK不向std导出to_string。沿用项目GCC5.2适配方式，仅远端src副本处理：整数to_string改为classic locale输出流、strtoull/snprint使用C声明，补充必要头文件；不全仓改写产品源码、不引入厂家查询逻辑。具体转换见[构建驱动](../scripts/build-probe-gcc54.sh)。

最终成功构建：`/root/router-probe-gcc54/runs/20260911-230640-7e2480c9`。strip后 **1131344字节**，ELF32小端、MIPS32r2、o32、soft-float，动态解释器 `/lib/ld-uClibc.so.0`；依赖libpthread.so.1、libstdc++.so.6、libm.so.1、libgcc_s.so.1、libc.so.1。已在目标固件运行，不只是ELF头匹配。

## 实测方式与证据

- 最新实机测试：`build/at-device-test/20260911-230834/`。目标临时目录 `/tmp/router-at-test-20260911-230834`，二进制 `router-probe` 已停止但保留；/tmp内容重启后可能消失。
- 使用临时device_id、独立本机Server/数据目录，以及仅目标loopback监听的SSH反向转发；不暴露公网测试HTTP服务，不连接或修改原生产Server的纳管配置。
- `registered.json`、`first-snapshot.json`、`cellular-api.json`、`later-snapshot.json`、`busy-port.json`、`recovered-port.json`、`disabled.json`记录真实API结果。身份原值仅在忽略的本机证据目录，不加入源码文档。
- `before.log` / `after.log`记录原进程及路由；`cleanup-confirmed.log`确认原进程仍在、没有临时探针/占用进程；`server.log` / `probe.log`为对应测试连接日志。
- `build/at-device-test/oneclick-default-password.log` 是最终默认密码入口真实构建；`build-verified.log`为远端bash语法与ELF/环境检查。`physical-latest-build.log`为同一最新产物的实机总结。
- [密码文件回归](../tests/probe_build_gcc54_test.py)：`python tests/probe_build_gcc54_test.py`，**4项通过**；覆盖特殊字符/空格原样传递、空/多行拒绝、缺文件在上传前失败和PowerShell语法。无伪造SSH成功。
- 本地另存最新产物 `build/at-device-test/router-probe-mipsel-gcc54`。一键脚本本身按用户要求只将正式产物放远端/root；该本机副本是本轮上传测试时取回。
- 本轮没有改Probe/Server/WPF产品逻辑；不重跑或冒充上一轮全量582/196/16项测试，这些历史结果见[CELLULAR_AT](CELLULAR_AT.md)。本轮新增脚本及真实GCC/设备测试单独记账。
