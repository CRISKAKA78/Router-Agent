# GCC 5.4 一键编译与真实设备 AT 验证

> 合入更新：用户已授权本工作树（含 AT 前置智能邻居）并入本地 main；当前进度、统一 ADR 编号及联合验证见 [WORKTREE_INTEGRATION](WORKTREE_INTEGRATION.md)。下文独立工作树、未提交或原目录不写入等语句记录当时事实，不限制本次授权；原始测试证据仍在对应工作树的忽略目录，不能当作本次联合测试结果。

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

## 当前一键编译入口（2026-09-12 更新）

按用户授权与 ADR-065，GCC5.4 已合入 [probe-build.cmd](../probe-build.cmd)。它一次构建 ARMv7 与 MIPS 小端，正式产物分别为 `/root/router-agent/router-agent-armv7`、`/root/router-agent/router-agent-mipsel`；旧 gcc54 `.cmd/.ps1` 仅是同一流程的兼容别名。

原密码辅助 `.cmd` 在中文仓库路径下启动失败的问题已改为英文临时目录原生 AskPass；SSH 登录先检查、完整归档 SFTP 上传、两份编译和检查成功后才发布。原已安装 GCC5.4 SDK 可只读复用，历史产物、缓存和 runs 不删除。当前参数、完整目录结构、默认接口、日志和实际双架构验证见 [统一构建说明](PROBE_BUILD_VERIFICATION.md)。

下方 GCC5.4 编译及 FM160-CN 实机内容保留为 2026-09-11 的历史证据，不代表本轮新组合产物已在设备运行。

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
