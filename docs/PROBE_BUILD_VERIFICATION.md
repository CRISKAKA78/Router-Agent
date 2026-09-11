# Probe 双架构一键构建与验证

## 当前入口（ADR-065，2026-09-12）

双击仓库根目录 [probe-build.cmd](../probe-build.cmd)，一次上传当前 Probe 源码，在 `root@10.1.1.128:22` 顺序构建 ARMv7 与 MIPS 小端。窗口末尾 pause 仅保留结果；脚本不会安装设备、启动 Probe 或重启 Server。

| 架构 | 编译器 | 最终产物 | 本轮大小 |
| --- | --- | --- | --- |
| ARMv7 小端 / EABI5 / uClibc | `/root/gcc-5.2`，GCC 5.2.0 | `/root/router-agent/router-agent-armv7` | 965240 字节 |
| MIPS 小端 / MIPS32r2 / uClibc | GCC 5.4.0，target `mipsel-openwrt-linux-uclibc` | `/root/router-agent/router-agent-mipsel` | 1231440 字节 |

两份均为 strip 后 ELF32 动态链接程序，解释器 `/lib/ld-uClibc.so.0`。目标固件仍须提供兼容动态库；本轮未进行设备安装或运行验收。默认采集接口均为 `br0,eth0,eth1,usb0`，Server 默认配置不变。

### 使用条件与参数

- 根目录 `password.txt` 为编译机 root 登录密码，单行文本，可带 BOM 和末尾换行。缺失、空或多行在 SSH 前拒绝；不使用私钥，不把密码放进命令行或上传包。
- Windows 需要 PowerShell、OpenSSH（ssh/sftp）、tar 和系统 .NET Framework C# 编译器。原生 AskPass 在英文 TEMP 目录中临时编译，密码文件仍可处于中文项目路径；TEMP 含非 ASCII 字符时明确报错，可设置为已有可写英文目录。结束后清理本轮本地临时文件。
- GCC5.4 优先使用 `/root/router-agent/toolchain/gcc-5.4`；不存在有效安装标记时，兼容复用 `/root/router-probe-gcc54/toolchain/gcc-5.4` 已安装缓存。不搬移、删除或修改旧 SDK。两处均无安装标记时，SFTP 上传桌面 `gcc-5.4.tar.gz`，按原安全检查安装到新根目录的 toolchain 下；不覆盖无标记的已有目录。
- 高级参数：`-SshTarget`、`-Port`、`-RemoteRoot`、`-Toolchain`（仅 ARM GCC5.2）、`-NetworkInterfaces`、`-PasswordFile`、`-ToolchainArchive`。例如 `probe-build.cmd -NetworkInterfaces eth0,br0` 同时应用于两种架构。
- 旧 `probe-build-gcc54.cmd` / `.ps1` 仅作为统一入口的兼容别名，现也构建两种架构，默认输出到 `/root/router-agent`。不再维持独立的流式上传实现。

### 记录和失败行为

```text
/root/router-agent/
  router-agent-armv7
  router-agent-mipsel
  MBEDTLS-LICENSE.txt
  THIRD-PARTY.md
  latest-build.txt
  runs/<时间-随机标识>/
    source.tar.gz
    input/probe/
    input/scripts/
    build.log
    armv7/  # src、build、output、gcc52-compat.patch、build.log、verification.log
    mipsel/ # src、build、output、gcc54-compat.patch、build.log、verification.log
```

一次归档仅包含 Probe 和三份构建驱动；Windows 源文件未提交的更新也会参与本次编译。完整归档经 SFTP 上传后才解包，三份 Bash 脚本统一去 CR，再进入按 SDK 隔离的兼容副本。

远端根目录 flock 串行化双架构构建和发布；两份编译、strip、ELF32/小端/机器类型检查与 ARMv7 属性检查均成功后，才逐文件 rename 更新分名产物，最后更新 `latest-build.txt`。任一编译或校验失败返回非零，保留上次两份固定产物和成功指针；每份 rename 是原子的，不宣称两份文件为跨文件原子事务。历史 `router-agent`、旧 GCC54 的 `output/router-probe` 及旧 runs 不删除、不作为新入口的更新目标。

### 原失败原因与修复

原 GCC54 入口直接把中文仓库路径中的 `.cmd` 设置为 SSH_ASKPASS。OpenSSH 实测返回 `CreateProcessW failed error:2` / `ssh_askpass: posix_spawnp: No such file or directory`，登录退出后，源码 CopyTo 才报“管道已结束”。同一辅助脚本置于英文临时目录，以及原生 C# AskPass，均可用原密码登录；报错轮次 `20260912-011217-f5bd25fd` 未创建远端目录。

统一入口采用临时原生 AskPass，先做 SSH 登录检查，保留 SSH 错误输出，再进行完整文件 SFTP 传输。沿用 `build/gcc54/known_hosts` 并读取用户默认 OpenSSH known_hosts，首次 TOFU、后续主机密钥变化拒绝的规则保持，不删除主机密钥记录。

## 本轮验证

- Windows 中文仓库路径运行 `cmd.exe /d /c "echo.|probe-build.cmd -ToolchainArchive C:\nonexistent-sdk-cache-test.tar.gz"`，退出码 **0**。故意给出不存在的本地 SDK 路径，确认使用旧已安装缓存；未再次上传 SDK。日志：`build/probe-dual-build-final.log`。
- 成功运行目录：`/root/router-agent/runs/20260912-012702-aa9ec5c9`。真实 GCC5.2 ARMv7、GCC5.4 MIPS32r2 Release 编译和 strip 通过；file/readelf 验证上述格式。`cmp` 确认两个固定产物与各自 run/output 一致，均可执行，latest-build 指向本轮，两个 CMakeCache 的默认接口一致。
- `python tests/probe_build_gcc54_test.py`：**6 项通过**，包含原生 AskPass/中文密码文件路径/特殊字符、两个入口的缺失/空/多行密码拒绝、非法接口、真实本机 SSH 连接拒绝及两个 PowerShell AST。
- `tests/probe_build_publication_test.py` 在编译机通过 `python3 -` 接收当前驱动和测试源码：**4 项通过**。使用明确的编译器测试替身，验证 ARM 失败、MIPS 失败、错误 ELF 均不覆盖旧产物，成功则发布两个文件与指针；不把替身当成真实编译。WSL RouterAgentTest 无 python3，未安装额外依赖，改用已有编译机 Python。
- 三份驱动的 CRLF 测试副本经过与入口完全相同的 sed 表达式后，`bash -n` 和与原 LF 文件 `cmp` 均通过。远端证据在成功 run 的 `crlf-check/`，本地汇总 `build/probe-dual-verification.log`。
- 首轮联调在 MIPS 驱动 CRLF 处失败，记录 `20260912-012544-91f2cefc`；修复远端 shell 转义及脚本归一化后，上述最终轮次通过。旧 `collection.cpp` 的 `seconds` 可能未初始化警告在两架构仍出现，未扩展修改产品业务代码。
- 本轮仅修改构建入口、驱动、回归与文档；没有重跑 Server/WPF/Probe 全量业务测试，没有新设备部署/实机验收或 Git 提交推送。
