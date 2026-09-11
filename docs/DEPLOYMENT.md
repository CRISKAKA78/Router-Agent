# 真机测试部署与启动指南

适用版本：2026-09-08，原生 C# / WPF 工作台和 C# / Blazor 模板生成器（ADR-037/036/035/034）。本指南按当前代码整理；客户端由你自行编译。先走同一局域网测试，再考虑跨网络部署。

## 1. 先弄清楚三样程序放在哪里

| 程序 | 放在哪里 | 作用 |
| --- | --- | --- |
| Management Server（router-server.exe / router-server） | Windows 电脑或 Linux 服务器 | 接收设备连接，提供 API，保存文件/工具仓库，提供维护入口 |
| Windows 客户端（RouterWorkbench.exe） | 你的 Windows 电脑 | 显示设备、执行操作、打开维护终端 |
| Probe（router-probe） | 被测路由器的 Linux 系统里 | 主动连接 Server，执行命令、传文件，连接路由器本地服务 |

最简单的组合：**你的 Windows 电脑同时运行 Server 和客户端，路由器只运行 Probe。** 不需要数据库、Redis、Docker、FRP、Node 服务或 Vite 服务来运行这套正式产品。Docker 是项目部分自动化测试的依赖，不是真机部署依赖。

本文统一举例：

- Windows 电脑的局域网 IPv4：`192.168.1.10`。
- 路由器的局域网 IPv4：`192.168.1.1`。
- 设备 ID：`my-router-001`。
- Server 部署目录：`C:\RouterProbeServer`。

**以上 IP 是示例，必须换成你自己的。** Windows 在 PowerShell 执行 `ipconfig`，找连接路由器的以太网/Wi-Fi 网卡 IPv4；不要选 Docker、WSL、虚拟机的虚拟网卡地址。建议固定电脑地址或配置 DHCP 地址保留，避免下一次变号。

`127.0.0.1` 表示“执行这个程序的机器自己”：写在路由器上就是路由器，写在电脑上就是电脑。`0.0.0.0` 只用于监听，不是客户端要连接的目的地址。

## 2. 测试前准备

1. 电脑能通过 SSH、Telnet、串口或厂商提供的方式进入路由器 Linux Shell。仅能打开管理网页还不足以安装 Probe。
2. 准备与路由器 CPU、libc、ABI 和内核兼容的 `router-probe`，见第 5 节。Windows 客户端和路由器 Probe 是两种不同程序。
3. 电脑和路由器之间可互通。初次测试避免访客 Wi-Fi、AP 隔离以及电脑休眠。
4. 需要测试 SSH/Telnet/Web 时，路由器自己必须已有对应服务。Probe 不安装或开启这些服务。
5. 当前版本未内置认证、TLS、RBAC。按项目设计，在可信局域网或受保护的管理网络内测试；不要把下面端口直接向整个公网开放。

### 端口对应关系

以下全部是 TCP：

| Server 端口 | 谁连接它 | 用途 |
| --- | --- | --- |
| 8080 | Windows 客户端 | HTTP API 和 WebSocket，客户端设置填这个端口 |
| 9000 | 路由器 Probe | 注册、心跳、Exec、文件传输 |
| 9001 | 路由器 Probe | Web/SSH/Telnet 的独立维护数据连接 |
| 20000～20199 | 电脑上的浏览器、SSH/Telnet 客户端 | 创建维护后临时分配的三个公共入口 |

路由器主动向 Server 发起连接，所以一般不需要在路由器 WAN 上做入站端口转发。维护流量路径是：电脑 → Server 临时入口 → 独立数据连接 → Probe → 路由器 `127.0.0.1:80/22/23`。

## 3. Windows 上准备 Server

本节在 **Windows PowerShell** 执行。普通 PowerShell 即可构建和运行，只有后面的防火墙配置需要管理员。

### 3.1 编译

#### 本仓库的一键重建并运行（Windows x64）

安装 Go 后，双击根目录 [server-windows.cmd](../server-windows.cmd)，或在 PowerShell 执行 `./server-windows.cmd`。它调用 [server-windows.ps1](../server-windows.ps1)，每次先删除专用的 `build/server-windows/`（旧 EXE 和本脚本的 Go 构建缓存），再用 `go build -a -trimpath` 从头编译 Server 及依赖，成功后在当前窗口运行；失败不启动。Go 模块下载缓存仍可复用，不清理其他项目的缓存。仅构建可用 `./server-windows.cmd -BuildOnly`。

清理路径固定且拒绝符号链接/junction。重新运行前先在旧 Server 窗口按 Ctrl+C 停止，不按进程名强杀其他 Server。持久文件、工具和模板目录固定为项目根目录 `data/server/repository/`，位于清理目录之外，不随重建删除；这也不会恢复或自动迁移此前丢失的 `cmd/server/data/`。

**模板更新与启动恢复（ADR-046）**：Server不要求预先存在模板。空库可直接启动；已有模板文件缺失会告警并建立空库。旧目录中的`interface_aliases`和退役注册结果字段在验证后自动清理，先在同目录保存`catalog.json.pre-upgrade-*.bak`原字节备份。设备绑定快照也会处理，因此无需删除设备目录。其他损坏、未知结构或读写错误仍显示具体路径，不能清空整个repository作为升级步骤。

操作顺序：重新运行`server-windows.cmd` → 打开`template-generator.cmd` → 使用最新版工程发布到当前Server → 在WPF中显式选择并应用模板。如果模板目录确已丢失，新发布产生新ID，需要重新选择，型号默认模板也需按需更新；仅发布不会自动改动已有设备。旧生成器工程导入仍按ADR-044拒绝，本次只处理Server已存数据。验证范围见[服务端启动修复](SERVER_STARTUP_VERIFICATION.md)。

一键脚本采用以下参数，端口保持现有默认值：

| 参数 | 值 |
| --- | --- |
| `-listen` | `0.0.0.0:9000` |
| `-http-listen` | `0.0.0.0:8080` |
| `-tunnel-bind` | `0.0.0.0` |
| `-tunnel-data-listen` | `0.0.0.0:9001` |
| `-tunnel-host` / `-tunnel-data-host` | `pcv6.criskaka.com` |
| 维护端口池 | `20000`～`20199` |

域名是对外访问地址，不是要绑定到网卡上的 IP。客户端设置填 `http://pcv6.criskaka.com:8080`；Probe 使用 `--server pcv6.criskaka.com:9000`。域名不加 IPv6 方括号；使用数值 IPv6 加端口时才写成 `[IPv6]:端口`。

2026-09-07 本机 DNS 查询获得 AAAA `2408:8256:3286:d12:13e:1401:aee9:267`，未获得 A 记录，该 IPv6 当时在本机网卡上。当前 Server 使用 Go `net.Listen("tcp", ...)`；本次 Windows 实测，以上 `0.0.0.0` 通配配置同时接受 IPv4/IPv6：8080/9000/9001 通过 `127.0.0.1`、`::1` 和该域名连接成功，API 返回 200；协议测试对端创建三个维护入口，逐个验证 IPv4/IPv6 接入，并检查下发数值 IPv6。此结果不表示所有平台的 `0.0.0.0` 都是双栈；仅 IPv6 监听也可显式配置 `[::]:端口` 和 `-tunnel-bind ::`。

DataHost 支持 A/AAAA，由 Server 每次创建维护时解析，双记录时优先 IPv4；仅 AAAA 时使用 IPv6。Probe 控制连接支持域名的 IPv4/IPv6 解析，数据连接支持数值 IPv6。路由器及客户端仍需可用 IPv6 路由和防火墙放行；本次验证来自本机，未验证公网远端或厂商路由器。脚本不自动修改防火墙。

验证环境：Windows PowerShell 5.1 执行 `.ps1 -BuildOnly`，再经 `.cmd -BuildOnly` 完成第二次构建，旧产物标记被删除；测试 Server 使用独立临时数据目录，测试结束已停止。此轮仅新增启动脚本与说明，未改 Server/Probe 网络实现，未重复全量业务回归。

#### 手动编译

安装 Go 后，重新打开 PowerShell，检查版本：

```powershell
go version
```

项目 `go.mod` 要求 Go 1.22 或更高兼容版本。提示找不到 `go` 时先安装 Go 并确认 PATH；已有 Server 成品可跳过编译，直接放到部署目录。

进入仓库根目录，编译并复制：

```powershell
Set-Location 'C:\Users\Administrator\Desktop\路由器探针平台'
New-Item -ItemType Directory -Force 'C:\RouterProbeServer' | Out-Null
go build -o 'C:\RouterProbeServer\router-server.exe' ./cmd/server
```

如果你的源码不在这个目录，替换 `Set-Location` 后的路径。第一次编译需要取得 `go.mod` 中的 Go 依赖；成功时通常没有输出。部署机只需要生成的 EXE，不需要安装 Go。

### 3.2 启动

以下命令假设电脑实际 IP 为 `192.168.1.10`，复制前修改两个 host 参数：

```powershell
Set-Location 'C:\RouterProbeServer'
.\router-server.exe `
  -listen '0.0.0.0:9000' `
  -http-listen '0.0.0.0:8080' `
  -repository-dir 'C:\RouterProbeServer\data\repository' `
  -tunnel-bind '0.0.0.0' `
  -tunnel-host '192.168.1.10' `
  -tunnel-data-listen '0.0.0.0:9001' `
  -tunnel-data-host '192.168.1.10' `
  -tunnel-port-first 20000 `
  -tunnel-port-last 20199
```

PowerShell 每行末尾的反引号表示“下一行还是同一条命令”，后面不能再加空格。不习惯多行时用这一行，效果相同：

```powershell
.\router-server.exe -listen '0.0.0.0:9000' -http-listen '0.0.0.0:8080' -repository-dir 'C:\RouterProbeServer\data\repository' -tunnel-bind '0.0.0.0' -tunnel-host '192.168.1.10' -tunnel-data-listen '0.0.0.0:9001' -tunnel-data-host '192.168.1.10' -tunnel-port-first 20000 -tunnel-port-last 20199
```

两种写法只执行一种。**这个窗口保持打开**，程序持续运行就是正常现象，按 `Ctrl+C` 停止 Server。当前程序不自动安装 Windows 服务。

参数说明：

| 参数 | 你需要理解的意思 |
| --- | --- |
| `-listen` | 让路由器可以接入控制端口 |
| `-http-listen` | 让桌面客户端可以访问 API；只在同一台电脑使用时也可绑定 `127.0.0.1:8080` |
| `-repository-dir` | 文件和工具的持久化目录；建议固定绝对路径 |
| `-tunnel-bind` | 临时维护入口监听在哪些本机网卡 |
| `-tunnel-host` | 返回给电脑使用的维护入口地址，必须能从电脑访问 |
| `-tunnel-data-listen` | 接收 Probe 维护数据连接的监听地址 |
| `-tunnel-data-host` | 告诉 Probe 数据连接去哪里，必须能从路由器访问 |

默认 API、维护绑定及两个 host 都是 loopback。**仅运行 `router-server.exe -listen :9000`，会出现设备能上线但真机维护打不开的情况。**

### 3.3 放行 Windows 防火墙

在开始菜单搜索 PowerShell，右键“以管理员身份运行”，执行下列规则。它允许本地子网访问本指南的测试端口；只需添加一次：

```powershell
New-NetFirewallRule -DisplayName 'RouterProbe LAN Test' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8080,9000,9001,20000-20199 -RemoteAddress LocalSubnet -Profile Any
```

如果设备来自另一个受信任网段，应将 `LocalSubnet` 换成明确的来源 IP/网段。公司安全软件可能另有网络策略。不要用关闭整个防火墙来代替规则。

测试结束如需撤销本条规则：

```powershell
Remove-NetFirewallRule -DisplayName 'RouterProbe LAN Test'
```

### 3.4 确认 Server 真正可用

另开一个普通 PowerShell 窗口，执行：

```powershell
Invoke-RestMethod 'http://127.0.0.1:8080/api/v1/devices' | ConvertTo-Json -Depth 6
Test-NetConnection 192.168.1.10 -Port 9000
Test-NetConnection 192.168.1.10 -Port 9001
```

API 应返回带 `data` 的 JSON；Probe 尚未启动时设备列表为空是正常的。端口检查应出现 `TcpTestSucceeded : True`。这些是电脑侧检查，不能代替路由器到电脑的连通性验证。

Server 根地址 `/` 不提供正式网页；不要把根路径 404 当成 API 失效。20000～20199 也不是启动时全部监听，只在创建维护后监听被分配的端口。

## 4. 如果 Server 放在 Linux

本节替代第 3 节的 Windows Server，不需要两边各启动一个。以下使用 Linux Shell，假设 Go 已安装、当前目录是源码根目录、主机 IPv4 为 `192.168.1.10`。

```sh
go version
mkdir -p build/server
go build -o build/server/router-server ./cmd/server
mkdir -p "$HOME/routerprobe-server"
cp build/server/router-server "$HOME/routerprobe-server/router-server"
cd "$HOME/routerprobe-server"
./router-server \
  -listen 0.0.0.0:9000 \
  -http-listen 0.0.0.0:8080 \
  -repository-dir "$HOME/routerprobe-server/data/repository" \
  -tunnel-bind 0.0.0.0 \
  -tunnel-host 192.168.1.10 \
  -tunnel-data-listen 0.0.0.0:9001 \
  -tunnel-data-host 192.168.1.10 \
  -tunnel-port-first 20000 \
  -tunnel-port-last 20199
```

Server 运行时保持终端打开。另一终端执行 `curl http://127.0.0.1:8080/api/v1/devices` 检查。上述端口不需要 root 权限，仓库目录必须可写。

Linux 防火墙同样要允许相应来源访问端口。例如 **已经使用 UFW**，且测试网段确实是 `192.168.1.0/24` 时：

```sh
sudo ufw allow from 192.168.1.0/24 to any port 8080 proto tcp
sudo ufw allow from 192.168.1.0/24 to any port 9000 proto tcp
sudo ufw allow from 192.168.1.0/24 to any port 9001 proto tcp
sudo ufw allow from 192.168.1.0/24 to any port 20000:20199 proto tcp
sudo ufw status
```

不是 UFW 的系统通过现有防火墙配置相同规则，无需为本项目换防火墙。不要远程盲目开启防火墙而遗漏已有 SSH 管理入口。

前台确认成功后，若希望退出 SSH 后继续运行，可在同一目录保存下面为 `start-server.sh`（修改 IP），并只启动一个实例：

```sh
#!/bin/sh
cd "$HOME/routerprobe-server" || exit 1
exec ./router-server -listen 0.0.0.0:9000 -http-listen 0.0.0.0:8080 \
  -repository-dir "$HOME/routerprobe-server/data/repository" \
  -tunnel-bind 0.0.0.0 -tunnel-host 192.168.1.10 \
  -tunnel-data-listen 0.0.0.0:9001 -tunnel-data-host 192.168.1.10 \
  -tunnel-port-first 20000 -tunnel-port-last 20199
```

先停止原前台实例，再运行：

```sh
chmod +x start-server.sh
nohup ./start-server.sh > server.log 2>&1 &
echo $! > server.pid
tail -n 50 server.log
```

停止前先用 `ps -p "$(cat server.pid)" -o pid,args` 核对仍是自己的 Server，再执行 `kill "$(cat server.pid)"`。PID 文件只供本次进程使用，主机重启后不要沿用旧 PID。此方式不提供开机自启或日志轮转。

## 5. 准备匹配真机的 Probe

### 5.1 先识别路由器

在 **路由器自身 Shell** 执行，保存输出：

```sh
uname -a
uname -m
cat /proc/cpuinfo
cat /etc/os-release
ls -l /lib/ld* /lib/libc* /usr/lib/libstdc++* /lib/libpthread*
```

部分固件没有 `/etc/os-release` 或某些库文件，对应报错并不等于设备坏了。需要综合型号、固件版本、CPU、大小端、libc（uClibc/musl/glibc）、动态加载器以及 ARM ABI 判断。

**目前仓库已验证 Linux x86_64；没有可直接宣称适配所有 mipsel/ARM/ARM64 路由器的现成工具链配置或通用二进制包。** 更改 `--arch` 仅修改上报标签，不会把 x86 程序变成 ARM 程序。

ADR-038：新版 Probe 默认上报架构与内核版本，无需为它们创建模板；已配置的 kernel 模板继续优先。开机时长在注册后立即首报并随心跳更新，服务端和 WPF 也需更新才能查看。WPF 在“设备 → 全部属性”显示 CPU 架构、内核版本、开机时长和时长上报时间；时长不足一分钟只显示秒，从最高有效单位显示到秒，按年=365日/月=30日换算。离线保留最后实测值，未收到心跳/采集失败分别显示等待上报/未获取。

### 5.2 Linux 同架构构建

在与目标运行环境兼容的 Linux 构建机执行；不是在 Windows PowerShell 直接执行。构建机需要 CMake 3.10+、支持 C++11 的 C++ 编译器、线程库和构建工具。Debian/Ubuntu 可用 `sudo apt-get install build-essential cmake` 安装构建工具。

在源码根目录：

```sh
cmake -S probe -B build/probe -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTING=OFF
cmake --build build/probe --parallel 2
file build/probe/router-probe
ldd build/probe/router-probe
```

输出文件：`build/probe/router-probe`。动态链接版本需要目标机具备兼容的加载器、libc、C++ 和线程运行支持。在新 Linux 上编译成功不代表能运行于老固件。

### 5.3 mipsel / ARM / ARM64 交叉编译

**MIPS/uClibc GCC5.4 新入口（2026-09-11）：** [probe-build-gcc54.cmd](../probe-build-gcc54.cmd) 从同目录password.txt读取编译机root密码，默认10.1.1.128，首次使用桌面gcc-5.4.tar.gz安装独立SDK；成品 `/root/router-probe-gcc54/output/router-probe`。已在MT7621/Fibocom FM160-CN设备验证自动串口、ATI/IMEI、占用跳过和恢复；未重启模组或替换生产实例。完整目录、参数、安全边界及证据见[GCC54实机说明](GCC54_AT_DEVICE_VERIFICATION.md)。下方GCC5.2入口保持原ARM用途。

**已配置的 GCC 5.2 一键入口：** 在 Windows 双击仓库根目录 [probe-build.cmd](../probe-build.cmd)，按提示输入 SSH 密码；已配置 SSH 密钥时直接使用密钥。首次连接按 OpenSSH 提示核对主机身份，脚本不保存密码。Windows 需要系统 `ssh.exe`、`tar.exe` 和 Windows PowerShell；远端使用已有 CMake、make、Python 3、binutils 及 `/root/gcc-5.2`。

脚本通过一次 SSH 登录将当前工作区 `probe` 源码（包含尚未提交的更新）上传到 `root@10.1.1.128`，先将上传的 Bash 脚本从 CRLF 规范化为 LF，再自动应用该 SDK 所需的整数转字符串、strtoull 和声明头兼容处理，执行 Release 交叉编译及 ELF 检查。兼容处理仅作用于远端副本，仓库产品代码不改动。Windows 不生成源码压缩包或构建产物。

- 固定成品：`/root/codex-probe-20260908-2123/output/router-probe`。
- 每次输入源码、兼容副本、补丁、临时文件、构建和日志：专用目录下 `runs/<时间-随机标识>/`；`build.log` 是构建日志，`verification.log` 是 ELF/依赖记录。
- `latest-build.txt` 指向最近成功构建目录。上传、配置或编译失败返回非零状态，保留上一份成功成品；窗口会保留结果。各次记录不自动清理。
- 输出为 ARMv7 小端 / EABI5 / uClibc 动态链接程序。目标需要兼容的 `/lib/ld-uClibc.so.0`、libpthread/libstdc++/libm/libgcc_s/libc，以及线程库依赖的 libdl；编译通过不代表目标固件运行验收。

高级调用为 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\probe-build.ps1`；可传 `-SshTarget root@10.1.1.128 -RemoteRoot /root/codex-probe-20260908-2123 -Toolchain /root/gcc-5.2` 调整构建主机或目录，仍要求对应 GCC 5.2 ARM/uClibc SDK。此入口不是通用的其他架构构建器。

2026-09-08 验证：Windows PowerShell 5.1 → SSH 真实主机全流程成功，CMake 3.28.3 / GCC 5.2.0 生成 280908 字节成品；记录在 `runs/20260908-213026-e3687d14`。传入不存在的工具链时，远端与 Windows 入口均返回 1，记录在 `runs/20260908-213159-94e0a6cc`；`cmp` 确认固定成品仍与成功版本一致，latest-build 指向保持。PowerShell 语法解析、远端 `bash -n` 和 `git diff --check` 通过。GCC 5.2 仍报告原 `collection.cpp` 的 `seconds` 可能未初始化警告；本次未改业务代码，未执行厂商固件测试。

2026-09-09 换行修复验证：原失败运行 `runs/20260909-010848-6f1a539c` 的脚本含 CRLF，远端 Bash 5.2.21 将 `pipefail` 后的 CR 当作选项内容。已将本地脚本保存为 LF，并在上传解包后、Bash 执行前使用 `sed` 去除行末 CR，覆盖 Windows 编辑器重新写入 CRLF 的情况。原失败脚本的独立副本经过同一处理后通过 `bash -n`；PowerShell AST 通过。执行 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\probe-build.ps1 -NetworkInterfaces eth0,eth1,br0`，真实 Windows → SSH → GCC 5.2.0 Release 构建成功，记录为 `runs/20260909-011116-bc8fcaa1`，成品 373112 字节，ELF ARMv7 小端/EABI5/uClibc，编译参数确认默认接口 `eth0,eth1,br0`；`cmp` 确认固定成品一致，latest-build 指向本次运行。成品与两份日志另外取回本地 `build/arm-gcc52-20260909/`。原 `seconds` 警告仍存在，未修改 Probe 业务逻辑，未在厂商路由器运行。

2026-09-10 邻居版本首轮交互认证构建 `runs/20260910-203128-4121706b` 在该SDK的C++头未导出`std::snprintf`处失败；全局`::snprintf`已由其`stdio.h`声明。一键脚本的每次运行副本适配现补充`<stdio.h>`并转换该调用，不修改仓库Probe产品源码。Git Bash语法检查、内嵌适配实际输出、适配后Probe核心编译和原始Probe 15项CTest已通过；BatchMode仍无认证，因此尚未用真实GCC5.2复跑，也未更新固定ARM成品或`latest-build.txt`。

优先取得**设备对应固件的 SDK/交叉工具链**。例如 OpenWrt 应匹配固件版本、target/subtarget 和 libc；厂商固件应使用厂商 SDK。仅凭 CPU 名字随意选编译器容易出现“文件存在却找不到”“缺少 GLIBC 版本”等错误。

下面只是工具链文件格式示例，路径必须来自你实际取得的 SDK，不能原样执行。文件名示例 `router-toolchain.cmake`，保存在源码根目录：

```cmake
set(CMAKE_SYSTEM_NAME Linux)
set(CMAKE_SYSTEM_PROCESSOR mipsel)
set(CMAKE_CXX_COMPILER /absolute/sdk/bin/actual-target-g++)
# 如果 SDK 要求显式 sysroot，填入其真实目录；否则按 SDK 说明配置。
# set(CMAKE_SYSROOT /absolute/sdk/sysroot)
```

按 SDK 说明先配置环境，然后在 Linux 构建机的源码根目录执行：

```sh
cmake -S probe -B build/probe-router \
  -DCMAKE_TOOLCHAIN_FILE="$PWD/router-toolchain.cmake" \
  -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTING=OFF
cmake --build build/probe-router --parallel 2
file build/probe-router/router-probe
```

ARM/ARM64 要换处理器、编译器及 SDK 要求的 ABI 参数；不同目标使用不同 build 目录。交叉生成的程序去目标路由器执行，不在 x86 构建机运行。静态链接是否可行也取决于 SDK，不能只加 `-static` 就承诺老内核兼容。

如果目前没有匹配 Probe，先完成 Server 和客户端连接，再提供路由器型号、固件版本与 5.1 输出确定构建方案。路由器只需运行成品，不需要安装 Go、CMake、Node 或 .NET。

## 6. 将 Probe 放到路由器并启动

### 6.1 传文件

第一次测试建议放 `/tmp`，避免修改固件目录；它可能占用内存且重启会丢失，不作为长期安装路径。

若路由器支持 SCP/SFTP，在 **电脑 PowerShell** 中执行（替换本地文件路径、用户与路由器 IP）：

```powershell
scp 'C:\你的文件目录\router-probe' root@192.168.1.1:/tmp/router-probe
```

首次连接应核对主机身份，输入的是路由器账号密码。路由器未提供 SFTP 子系统时，可在支持该选项的 OpenSSH SCP 中用 `scp -O` 尝试传统 SCP；传统模式也要求路由器有相应程序。两者都不支持时使用厂商允许的文件传输方式，不能靠尚未上线的 Probe 上传自身。

### 6.2 前台运行

在 **路由器 Shell**：

```sh
chmod +x /tmp/router-probe
/tmp/router-probe --help
/tmp/router-probe --server 192.168.1.10:9000 --device-id my-router-001
```

`--server` 填 **Server 地址与 9000 端口**，不带 `http://`，不能填路由器自己的 IP 或 8080。这里的 `192.168.1.10` 要和实际部署对应。

看到 `state=ONLINE device_id=my-router-001` 表示完成注册。之后默认约每 30 秒有心跳，断线自动按 1/2/5/10/30 秒退避重连。Server 日志也会记录设备 ONLINE。

每台路由器使用不同、固定的 `--device-id`。同一 ID 同时启动两个 Probe 会发生 Session 替换和互相挤下线；重连后维护需要重新创建。不要在前台实例还运行时再开后台实例。

省略 `--device-id` 时，在连接前执行一次 `nvram get SN`，5 秒超时；成功输出去掉首尾空白，必须为非空、单行、最多 128 bytes 的 UTF-8。命令缺失、失败或输出非法时启动报错，需显式指定稳定 ID；不生成随机 ID。`--device_id` 作为参数别名，显式传空值也会报错。Linux x86_64 测试替身已验证；厂商固件上的 nvram 仍需实测。

### 服务端属性模板

ADR-031 已增加专用 nvram / uci 来源。先更新 Server 与 Probe，再在每项属性的“采集来源”选择 nvram 并填 `SN`，或选择 uci 并填 `system.@system[0].hostname`。无需自己写 get 指令；两种来源都只读。命令来源继续可用，不同厂商的键由实际固件决定，不自动猜测。

设备 → 配置，可选择读取、写入、删除、提交；结果在同页“配置操作结果”查看。写入值允许空字符串，不能省略；写入/删除不会自动 commit 或重启服务。UCI commit 填配置包（如 `system`），nvram commit 提交整份 NVRAM；可能提交其他程序的暂存修改。先等当前任务结果，再发起依赖它的操作；响应不确定时查原任务，不创建替代写任务。

Probe 运行账号须有固件命令所需权限，PATH 须能找到 nvram/uci。新 capability 只说明支持配置任务，未安装命令会任务失败。OpenWrt 未提供可用 `nvram get SN` 时必须用 `--device-id` 指定稳定 ID。配置更改不会更新启动属性快照，即时验证使用读取任务；本次未在用户设备上执行写入或 commit。

使用独立 [模板生成器](TEMPLATE_GENERATOR.md)（`template-generator.cmd` 构建）新建模板；主工作台设置已移除模板配置。填写名称、属性标识、属性类型和来源，虚拟属性可参与计算，展示属性直接采集或引用公式。直接采集例如 `serial` / 序列号 / nvram `SN`，`kernel` / 内核版本 / 命令 `uname -r`；model、firmware 的指令由实际固件决定。先保存可编辑工程，再连接服务器发布。公式需要设备提供 awk，编译命令内的依赖读取共用展示属性的 1～30 秒超时（默认 5 秒），整次采集仍最多 60 秒。

模板保存在 Server 的 `repository-dir/probe-templates/catalog.json`（可通过 `-probe-template-file` 指定），无需复制模板文件到设备。停服备份时包括此文件及 lock，保留原文件/工具仓库；不要在服务运行时手改目录。客户端管理操作通过 `/api/v1/probe-templates`。

在WPF“待纳管”选择设备，指定已发布模板并确认纳管；已纳管设备通过“设备资料与模板”显式应用版本。Probe启动只填Server和稳定设备ID，模板经CONFIG_APPLY在线下发，离线修改在下次连接后生效；不再使用--template-id或--template-name。模板发布本身不会替换设备绑定快照，采样和错误在effective_metrics/属性页展示。

当前只支持工程8/草稿3及新版Server/Probe，删除旧模板准备连接、REGISTER结果快照和遥测降级。同步更新配套程序；已有旧数据需由用户保留备份后重新生成/建立，程序不会迁移或清空。设备平台的BusyBox/uClibc/旧内核适配继续保留。详见 [ADR-044验证](UI_REFINEMENT_VERIFICATION.md)。

### 6.3 确认成功后改为后台运行

先按 `Ctrl+C` 结束前台 Probe，确认设备上没有旧实例，再执行：

```sh
nohup /tmp/router-probe --server 192.168.1.10:9000 --device-id my-router-001 > /tmp/router-probe.log 2>&1 &
echo $! > /tmp/router-probe.pid
tail -n 50 /tmp/router-probe.log
```

BusyBox 固件不一定包含 `nohup`。提示命令不存在时先保留前台/串口会话测试，后续按实际固件的进程管理方式配置；单纯加 `&` 不保证退出 SSH 后进程还活着。

停止后台 Probe 前执行 `ps`，核对 `/tmp/router-probe.pid` 中的 PID 对应本次 Probe，再执行：

```sh
kill "$(cat /tmp/router-probe.pid)"
```

进程退出/设备重启后 PID 可能被复用，不能盲用旧 PID。初次联调先不改开机脚本；当前仓库没有通用的路由器开机自启安装器。测试期间留意 `/tmp` 日志大小。

## 7. 启动你编译的 Windows 客户端

构建细节见 [Windows 使用说明](../windows/README.md)。按仓库脚本生成后，启动：

```text
build/windows-desktop/win-x64/RouterWorkbench.exe
```

当前 ADR-036/035 使用原生 C# / WPF 自包含 EXE，不需要 TerminalAssets、WebView2、Vite 或 Node。构建入口为 `ui-windows.cmd`；旧 React 浏览器预览已移除；模板生成器默认 `127.0.0.1:5188`，两者都不是 Server API。

1. 打开客户端，进入“设置”。程序会先自动连接上次保存地址。
2. Server 地址填 `http://192.168.1.10:8080`。Server 在当前电脑上也可填 `http://127.0.0.1:8080`。
3. 点击“保存并连接”。不要加 `/api/v1`，不要填 9000、9001、临时维护端口或 `0.0.0.0`。
4. 在设备列表找到 `my-router-001`，确认在线。列表为空先排查 Probe，客户端连接成功不代表设备已经接入。

当前直接启动的 Go Server 提供 HTTP；只有部署层实际提供 HTTPS 时才填 `https://`。配置位于 `%LOCALAPPDATA%\RouterWorkbench\profile.json`，可在设置修改，无需手动编辑文件。

### 外部 SSH/Telnet 需要本机客户端

在 Windows PowerShell 检查：

```powershell
Get-Command ssh.exe -ErrorAction SilentlyContinue
Get-Command telnet.exe -ErrorAction SilentlyContinue
```

找不到时，使用 Windows 系统可选功能安装 OpenSSH 客户端或 Telnet 客户端，或在工作台原生选择器里选择兼容的命令行客户端。工作台只打开外部客户端，不提供内置终端。可在设置选择 ssh.exe/telnet.exe 或 PuTTY；新配置账号密码默认 admin，允许修改，密码在本机加密保存，外部客户端询问时可在维护页复制粘贴。修改偏好不会修改路由器实际账号。

## 8. 按顺序完成第一轮真机测试

### A. 先验证在线和模板属性

选择设备，在“全部属性”核对设备 ID、上报基础字段、模板名称/版本及所有扩展属性。失败项显示原因，长文本可换行滚动；未上报值不伪造。切换不同模板设备确认属性跟随更新。通用任务/Exec 入口已移除，命令联调可在后续外部 SSH/Telnet 中执行。

### B. 再验证维护入口

在维护页点击“开启维护”，默认租期 **240 分钟**。成功后得到 Web、SSH、Telnet 三个地址，端口由 Server 分配；使用界面给出的实际地址，不要假定始终是 20000/20001/20002。

| 入口 | Probe 实际连接的路由器地址 | 路由器要求 |
| --- | --- | --- |
| Web | `127.0.0.1:80` | HTTP 管理服务能从 loopback 访问 |
| SSH | `127.0.0.1:22` | SSH 服务已启动且监听该地址/通配地址 |
| Telnet | `127.0.0.1:23` | Telnet 服务已启动且监听该地址/通配地址 |

在路由器可用 `netstat -lnt` 检查监听；若没有该命令但有 `ss`，用 `ss -lnt`。仅监听路由器 LAN IP 的服务不一定接受 `127.0.0.1`。实际 loopback HTTP 可在具备 wget 的设备上用 `wget -O /dev/null http://127.0.0.1/` 检查；有认证时返回未授权也说明服务有响应。

创建维护成功表示 Server 入口已建立，**不代表路由器三种服务都可用**。设备只支持 SSH 时，先测试 SSH；没有 Telnet 不影响 Exec、文件和 SSH。当前不能把目标改成 443、8080、2222 等其他端口。

Web 在系统浏览器打开；SSH/Telnet 输入路由器自己的账号密码。首次 SSH 主机密钥核对属于 SSH 客户端正常行为。

Web 通道是原始 TCP 转发，不改写 HTML、重定向或 Cookie。厂商页面如果强制跳转私有地址/HTTPS 443，或要求固定 Host，可能首页有响应却无法完整登录；记录具体 URL/现象，不要据此认为支持任意厂商网页。

### C. 文件上传与下载

1. 在电脑创建小文本文件，例如内容 `hello router`，先导入工作台文件仓库。
2. 选择资产上传到设备路径 `/tmp/routerprobe-test.txt`，第一次不覆盖已有文件。
3. 在“文件 → 文件传输”确认完成后，用外部 SSH/Telnet 运行 `cat /tmp/routerprobe-test.txt` 检查内容。
4. 从设备下载这个路径，在文件传输页检查服务器提交 `committed`、资源释放 `released` 与最终执行结果。
5. 显式完成导入后形成仓库资产，再保存到电脑检查内容。

本轮先用小文件，避免占满设备 `/tmp`。上传路径指路由器路径，不是电脑路径。工具投放同样要求匹配架构/运行库；文件上传成功不等于二进制在设备上可执行，兼容字段未知时不要强行宣称兼容。

### D. 关闭、到期与重连

1. 主动关闭维护，确认旧终端和入口失效。
2. 创建短租期维护，例如在分钟输入框填 1 分钟，等待到期，确认关闭。
3. 在可恢复的测试条件下短暂断开网络再恢复，观察 Probe 重新 ONLINE、设备恢复以及新 Session；重新创建维护后再登录。
4. 退出客户端会释放本地 HTTP/WS，不关闭用户外部 Shell，也不会自动撤销 Server 上仍有效的维护或已创建任务。测试结束先主动关闭维护。

远程测试时保留串口或其他恢复手段，不要关闭唯一的管理通道。

## 9. 常见问题对照

| 现象 | 优先检查 |
| --- | --- |
| 客户端连接失败 | Server 是否还在运行；填的是 HTTP 8080；API 路径能否返回 JSON；防火墙和 IP |
| 客户端连接成功但没有设备 | Probe 是否 ONLINE；`--server` 是否指向 Server:9000；二进制是否能运行 |
| Probe 一直 RECONNECTING | Server IP/9000、网络隔离、防火墙；结合日志里的 reason |
| Probe 文件明明存在却提示 not found | 动态加载器不存在，或 libc/ABI 不匹配；不是简单重复 chmod |
| Exec format error / Illegal instruction | 架构、大小端、CPU 指令集或 ABI 不匹配，重新按设备 SDK 构建 |
| Permission denied | 执行位、用户权限、挂载点 noexec；确认设备允许在该路径执行 |
| 缺少 libstdc++.so / GLIBC_x.y | 构建运行库与固件不兼容，使用匹配 SDK，勿随意覆盖系统 libc |
| 设备在线、Exec 正常，但三个维护入口都失败 | `tunnel-data-host` 是否误为 loopback；9001 是否可达；`tunnel-host` 和入口端口防火墙 |
| 只有一种维护失败 | 对应本地 80/22/23 是否提供服务，是否绑定 loopback；本机 SSH/Telnet 是否安装 |
| 维护显示 unavailable | 实际建流失败；检查本地服务和 data 网络，后续连接成功可恢复 ready |
| 设备频繁替换 Session | 是否多个 Probe 使用相同 device-id；先消除重复实例 |
| cannot bind / address already in use | 旧 Server 或其他程序占用端口；不要再开一份。Windows 可用 Get-NetTCPConnection 查看 |
| Repository 锁定/初始化错误 | 同一目录是否已有 Server；目录权限；日志；不要删除元数据或锁标记来强行启动 |
| 反复开关维护后容量不足 | 每次用三个端口，释放后默认隔离 24 小时；200 个端口池最多容纳 66 组完整分配（无其他占用时） |
| 页面连接数较多时部分请求失败 | Probe 默认 8 条维护流，Server 每设备/维护也是 8；先关闭多余页面再判断 |
| 目录缺项 | 目录浏览是单次 Exec，最多显示 250 项；不是持续文件系统同步 |

确需扩大维护流并发时同时调整 Probe `--tunnel-connections N`（1～64）及 Server `-tunnel-session-connections N -tunnel-device-connections N`，根据设备内存决定；初次测试保留默认即可。

端口隔离容量不足可等待隔离到期或在规划并放行更大端口池后调整 `-tunnel-port-last`；配置变更需重启 Server，影响现有 Session。不要把频繁重启清空隔离当成安全复用机制。

## 10. 下次开机、停止与数据备份

正常启动顺序：**Server → 路由器 Probe → Windows 客户端 → 设置连接 → 创建设备维护**。Probe 先启动也会自动重试，但第一次按这个顺序容易排查。

正常停止顺序：关闭维护 → 退出客户端 → 停止 Probe → Server 前台按 `Ctrl+C`（Linux 后台用前述 TERM 方法）。需要恢复原测试网络配置时再移除防火墙规则。

- Server 的文件/工具 Repository 持久化，下次仍使用同一个 `-repository-dir`。
- 设备清单、Session、任务、维护和幂等账本只在当前 Server 进程保留，重启后不恢复；设备再次连接才重新出现。
- Probe 自身重启后的任务去重与结果恢复也未实现。响应不确定时先核对结果，不要把旧操作无条件再执行一遍。
- 备份时先停 Server，整体复制 `data/repository`，包括元数据与 blobs；不要只复制 EXE 或只备份一个 JSON。当前没有自动清理/在线备份工具。
- `/tmp/router-probe` 及其日志可能随路由器重启消失。真机通过后，再根据固件确定持久目录与启动管理方式。

## 11. 本指南核对记录

本次只补充部署说明和文档导航，没有修改业务代码或执行用户设备部署。核对了 `cmd/server/main.go`、`probe/src/main.cpp`、CMake、API/ADR 和 Windows 发布说明；本机 Windows Server 构建与 `-h` 参数检查通过。路由器固件、CPU/运行库适配和厂商登录仍须由具体真机验证，本指南不将它们标为已通过。

## 独立周期监控（ADR-039）

配套更新Server、Probe及WPF。Probe默认内置CPU/内存/网口5秒，存储60秒；例如：

```sh
router-probe --server HOST:9000 --device-id DEVICE --cpu-interval 2 --memory-interval 5 --disk-interval 60 --network-interval 1
```

周期接受0（关闭）或1～86400秒。生成器“模板配置→内置监控”配置周期；每个展示属性“高级设置”中另设属性周期，0表示应用配置时采集一次。发布后在设备资料中显式应用，无需重启；当前工程8/草稿3不导入旧格式。模板同标识优先且失败不回退；数值单位及网口/挂载点标识见 [TELEMETRY_DESIGN](TELEMETRY_DESIGN.md)。

WPF设备页有全部属性、存储空间、网口速率及连接历史。公网来源IP归属地由Windows访问ipwho.is HTTPS，只发送该IP；内网/保留地址不查询。查询受网络和供应商可用性影响，失败保留来源IP。Server处在代理后时展示代理地址，无法据此还原真实设备地址。


## 接口过滤与双栈出口（ADR-040）

新增能力需使用本次Server及Probe 0.2.0；旧ARM成品不包含本次修改。WPF产物与验证见 [MONITORING_V2_VERIFICATION](MONITORING_V2_VERIFICATION.md)。

1. 双击 `probe-build.cmd`，输入精确接口名，例如 `eth0,br0,ppp0`，回车采集默认全部接口；也可在有SSH认证的终端运行 `probe-build.cmd -NetworkInterfaces eth0,br0`。只读过滤保存在本次二进制的构建默认值中，构建机和输出位置沿用§5.3；不修改设备网络配置。
2. 生成器“内置监控”可设置出口IP周期（默认600秒）和“自定义采集接口”。勾选后填写逗号分隔接口名；空字符串选择默认全部，取消勾选沿用构建值。模板发布后需重启Probe生效。
3. Probe命令行可覆盖：`--network-interfaces eth0,br0 --network-interval 5 --egress-interval 600`。周期0关闭对应采集。接口最多32个、名称精确匹配，不存在时显示横杠并解释原因，不回退采集全部。
4. 出口探测需要设备已有curl或wget及相应HTTPS/IPv6能力、DNS和可信证书；使用IPv4/IPv6专用固定ipify服务，每协议请求有超时。归属地查询从Windows向ipwho.is发送上报的公网IP。一个协议或查询失败不会阻塞设备控制，界面悬停横杠查看原因。
5. Windows网口页双击接口查看收发曲线；累计流量从Probe首次采样开始，普通控制重连不清零，设备/Probe重启或接口重建/计数回退重新统计。曲线只保存窗口打开后最近10分钟，不跨应用重启保留。

本轮GCC5.2 SSH认证尚不可用；请在认证可用的终端构建后部署到目标固件。Linux x86_64测试Probe不可用于ARM/MIPS路由器。
