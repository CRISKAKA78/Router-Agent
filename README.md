# 路由器远程运维平台

Linux AMD64 服务端：双击 [server-linux-amd64.cmd](server-linux-amd64.cmd)，本机构建后默认使用根目录 `id_rsa` / `password.txt` 经 SFTP 上传至 `47.119.168.150:/root/agent-server`，运行远端 `start.sh` 启动；`-BuildOnly` 仅构建。客户端与模板生成器新配置默认 `http://47.119.168.150:8888`。[部署和验证](docs/SERVER_DEFAULTS_VERIFICATION.md)。

当前改造（ADR-044）：属性展示开关、整行分组、连续像素滚动与文字行高；删除接口映射及旧版兼容。仅工程8/草稿3及当前配套Server/Probe，使用与验证见 [改造交付](docs/UI_REFINEMENT_VERIFICATION.md)。

当前服务端纳管、动态采集、模板分组与交换机物理口功能的使用入口及升级规则见 [服务端纳管与动态探针配置](docs/MANAGED_PROBES_DESIGN.md)，验证范围见 [本轮验证](docs/MANAGED_PROBES_VERIFICATION.md)。

独立探针模板生成器：[template-generator.cmd](template-generator.cmd) 构建并启动 .NET 10 / Blazor 浏览器工作区（默认 `http://127.0.0.1:5188`），支持虚拟/展示属性、全部采集来源、公式与条件规则、工程导入导出、自动保存和服务器发布。生成器源码在 [src/ProbeTemplateGenerator](src/ProbeTemplateGenerator)，独立构建无需 Node；主 RouterWorkbench 现按 ADR-035 使用原生 C# / WPF。使用与迁移见 [模板生成器说明](docs/TEMPLATE_GENERATOR.md)。

Windows UI 当前入口：[ui-windows.cmd](ui-windows.cmd)，构建 .NET 10 / WPF 原生 C# 工程工作区（ADR-035），输出 `build/windows-desktop/win-x64/RouterWorkbench.exe`。ADR-036 收敛为设备/维护/文件/配置/设置，启动自动连接，维护只打开外部客户端，主 UI 自包含且不需要 WebView2/TerminalAssets；功能和验证见 [原生桌面说明](docs/WINDOWS_DESKTOP_MIGRATION.md)。ADR-037 已移除其余旧 UI 源码及构建入口；当前仅维护这两套 C# UI。

本项目用于建设一套由 Management Server 和路由器端 Probe 组成的远程运维平台。Management Server 统一承载设备、任务、文件与工具、Tunnel 和对外 API 等核心能力；Probe 主动连接 Server，并向上提供轻量、通用的设备控制原语。

Phase 0～5 已验收。当前提供原生 C# / WPF Windows 工作台和 C# / Blazor 探针模板生成器，通过公开 `/api/v1` 与 WebSocket 使用 Server。构建见 [Windows 使用说明](windows/README.md)，契约见 [API](docs/API.md)，状态见 [PROJECT_STATUS](docs/PROJECT_STATUS.md)。Phase 6 厂商实机与最终产品验收仍待完成。

当前方向是继续完善 Router-Agent 产品功能，暂缓 Phase 7 MCP、Phase 8 AI Agent、微信小程序、正式公网 Web 部署、新 Tunnel 数据面和其他大规模架构扩展。用户只需描述想增加的功能、要改变的行为、待解决的问题或最终体验；Agent 依据 [AGENTS](AGENTS.md) 和[开发指南](docs/DEVELOPMENT.md) 自主完成范围内实现、测试和必要文档同步，涉及已确认设计变更时再提出具体方案供确认。

## 系统关系

Probe 一键交叉编译：双击 [probe-build.cmd](probe-build.cmd)，默认采集 `br0,eth0,eth1,usb0`，默认在 `root@10.1.1.128` 使用原 GCC 5.2 编译，从根目录 `password.txt` 自动读取 SSH 登录密码。成品为 `/root/router-agent/router-agent`，见 [部署说明 §5.3](docs/DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)。

Windows Server 一键重建并运行：双击 [server-windows.cmd](server-windows.cmd)。专用构建目录每次清空，运行数据独立保留；监听及 IPv6 域名配置见 [部署说明](docs/DEPLOYMENT.md)。

~~~text
Web / 微信小程序 / Windows UI / CLI / MCP / AI Agent
                         |
                  HTTP / WebSocket API
                         |
                Application / Service
                         |
       Device / Task / File / Tool / Tunnel
                         |
                  Probe TCP Gateway
                         |
                       Probe
~~~

Server 与 Probe 之间使用 Probe 主动发起的 TCP 长连接。该连接负责注册、心跳、任务、事件和必要文件传输。SSH、Telnet、Web 等持续交互流量必须使用独立 Tunnel 数据连接。

## 当前状态

- File/Tool Repository 使用本地持久目录，稳定 UUID 资产/工具/产物身份、不可变版本、SHA-256 内容去重与逻辑归档。内部 management.Service 进行兼容性查询、工具投放、资产上传、下载显式导入；全部复用现有文件传输。
- Device Service 以稳定 device_id 保存最近 REGISTER 资料、当前/最近 Session 与在线状态。内部 `Server.Devices().List/Get/Sessions` 返回查询副本；默认每设备保留最近 64 条已结束 Session，Server 重启后清空。
- 已建立项目入口、架构、协议、API 基线、路线图、状态快照、接管手册、决策记录和变更记录。
- 已将 Word 设计输入中的 TCP 协议整理为可维护的 Markdown 基线。
- 当前可构建并运行 Go Management Server 与 C++11 Probe。
- Probe 可主动连接、REGISTER、进入 ONLINE、收发 HEARTBEAT，并在断线后按 1/2/5/10/30 秒退避重连和取得新 session_id。
- Management Server 可通过内部 Task Service 向在线 device_id 下发 exec TASK，等待 TASK_ACK 与 TASK_RESULT，并按 task_id 关联结果。
- Probe 在线态可路由 HEARTBEAT_ACK 与 TASK；exec 由默认 4 workers 在 Reader 之外通过 `/bin/sh -c` 执行，支持 cwd、env、timeout、独立 stdout/stderr 和有界输出。
- 同一 Probe 进程内的已接受 task_id 不重复执行；TCP 断线不停止 exec，新会话补报缓存结果，Server 按 task_id 关联并幂等处理重复结果。
- 内部 ResendTask 重发原任务，按 session_id/message_id/task_id 记录各次派发；Phase 5通过HTTP提供同身份重发入口。默认缓存最多 128 个已接受任务，8 MiB 身份/结果计费预算，满后拒绝新任务。
- baseline commit: `bc8d747dfc41a375c31698073005857c238ede51`。
- Phase 1A commit: `cd722b6f3fd6cfe5e8ccded256c5295828e4372f`。
- Probe 技术栈为 C++11 + CMake；Phase 1A 已在 Linux x86_64 完成首轮验证。
- REGISTER / HEARTBEAT 字段契约已冻结，当前实现符合该契约。

- 内部 CreateUpload/CreateDownload 实现二进制分块文件传输与 size/SHA-256 校验；同目录临时文件完成校验后才发布。一个 active transfer 加有界 FIFO，控制消息优先，重复 task_id 不重复文件副作用。


默认HTTP监听 `:8888`，可用 `-http-listen` 指定地址。所有创建/修改HTTP请求带 `Idempotency-Key`，JSON Content-Type为application/json。例：

~~~sh
curl http://47.119.168.150:8888/api/v1/devices
curl -X POST http://47.119.168.150:8888/api/v1/tasks \
  -H 'Content-Type: application/json' -H 'Idempotency-Key: example-exec-1' \
  -d '{"device_id":"my-router","command":"uname -a","timeout_seconds":10}'
~~~

任务返回202与task_id，再GET `/api/v1/tasks/{task_id}` 查询。Maintenance通过POST `/api/v1/maintenance`提交device_id（可选lease_ms），返回Web/SSH/Telnet三入口。实时地址 `ws://47.119.168.150:8888/api/v1/events`；连接后按resync_required查询HTTP，收到resource_changed刷新。完整错误、分页、幂等/容量、文件上传、关闭与部署边界见[API](docs/API.md)。

## 当前不能做什么

仓库目前不支持Probe/Server进程重启后的任务恢复、Process Manager、通用Tunnel或任意端口，也不提供正式公网 Web 管理 UI、微信小程序、操作CLI、MCP或AI Agent。认证、TLS、RBAC和完整审计仍为后续设计点；当前API用于可信本机或受保护管理网络。

## 文档导航

- [AGENTS.md](AGENTS.md)：所有 AI Agent 和开发者必须遵守的工作与交付规则。
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)：从简短产品需求到实现、模块定位、验证与文档交付的日常指南。
- [docs/HANDOFF.md](docs/HANDOFF.md)：新会话或新开发者的最短接管入口。
- [docs/PROJECT_STATUS.md](docs/PROJECT_STATUS.md)：当前仓库的事实快照。
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)：长期架构、系统边界和模块职责。
- [docs/PROTOCOL.md](docs/PROTOCOL.md)：Probe 与 Server 的 TCP 控制协议基线。
- [docs/API.md](docs/API.md)：HTTP 与 WebSocket API 的设计基线。
- [docs/ROADMAP.md](docs/ROADMAP.md)：阶段路线图与完成状态。
- [docs/DECISIONS.md](docs/DECISIONS.md)：已经确认的架构决策。
- [CHANGELOG.md](CHANGELOG.md)：对外可感知的项目变化。
- [路由器探针_TCP长连接控制协议设计_v0.2.docx](路由器探针_TCP长连接控制协议设计_v0.2.docx)：本轮初始化使用的原始设计输入。

## 构建与运行

首次使用自己的路由器测试，请先读[真机测试部署与启动指南](docs/DEPLOYMENT.md)：包含 Windows/Linux Server、端口与防火墙、Probe 架构适配、客户端连接、维护与文件测试、停止及故障排查。

以下命令已在 Linux x86_64 验证。先构建 Server 与 Probe：

~~~sh
mkdir -p build/server
go build -o build/server/router-server ./cmd/server

cmake -S probe -B build/probe -DCMAKE_BUILD_TYPE=Release
cmake --build build/probe --parallel
~~~

终端一启动 Server：

~~~sh
./build/server/router-server -listen :9000
~~~

Server 默认在启动工作目录下创建 `./data/repository`，也可指定 `-repository-dir /absolute/data/repository`（Windows 可使用本地盘符路径）。启动日志记录解析后的绝对路径；同一仓库只允许一个 Server 写入。Repository 启动失败时 Server 报错退出，不降级为空仓库。运行数据默认排除 Git，使用自定义仓库目录时应放在源码仓库之外。

资产与工具通过[HTTP API及内部Service](docs/API.md)管理。停服后整体备份仓库目录；归档保留文件，不自动清理旧版本或磁盘。设备/任务/Session 仍只在 Server 进程内保留。

终端二启动 Probe：

~~~sh
./build/probe/router-probe --server 127.0.0.1:9000 --device-id demo-router-001
~~~

运行自动化测试：

~~~sh
ctest --test-dir build/probe --output-on-failure
RMP_PROBE_BIN="$PWD/build/probe/router-probe" go test ./... -count=1
RMP_PROBE_BIN="$PWD/build/probe/router-probe" go test -race ./... -count=1
go vet ./...
~~~

## 设计基线

Phase 0 完成后，仓库内 Markdown 文档是项目持续维护的当前设计基线。原始 Word v0.2 文档保留为 Phase 0 设计输入和历史参考，不覆盖后续经过正式确认并写入 Markdown 或 ADR 的变化。事实来源层级和冲突处理规则见 [AGENTS.md](AGENTS.md)。

任何协议、架构或重要决策的变化都必须经过明确评审，并同步更新对应文档。需要改变 Accepted ADR 时，应新增 superseding ADR，不能由实现静默改变历史决定。
