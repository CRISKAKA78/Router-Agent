# 路由器远程运维平台

本项目用于建设一套由 Management Server 和路由器端 Probe 组成的远程运维平台。Management Server 统一承载设备、任务、文件与工具、Tunnel 和对外 API 等核心能力；Probe 主动连接 Server，并向上提供轻量、通用的设备控制原语。

Phase 0～5已验收。Phase 6提供原生Windows远程维护工作台，通过统一 `/api/v1` 和WebSocket使用设备、任务、文件、工具与Maintenance能力。Windows入口及构建见[使用说明](windows/README.md)，正式API见[API](docs/API.md)，交付事实见[PROJECT_STATUS](docs/PROJECT_STATUS.md)和[Phase 6验证](docs/PHASE6_VERIFICATION.md)。本轮Windows UI提交推送后等待验收。

## 系统关系

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


默认HTTP监听 `127.0.0.1:8080`，可用 `-http-listen` 指定地址。所有创建/修改HTTP请求带 `Idempotency-Key`，JSON Content-Type为application/json。例：

~~~sh
curl http://127.0.0.1:8080/api/v1/devices
curl -X POST http://127.0.0.1:8080/api/v1/tasks \
  -H 'Content-Type: application/json' -H 'Idempotency-Key: example-exec-1' \
  -d '{"device_id":"my-router","command":"uname -a","timeout_seconds":10}'
~~~

任务返回202与task_id，再GET `/api/v1/tasks/{task_id}` 查询。Maintenance通过POST `/api/v1/maintenance`提交device_id（可选lease_ms），返回Web/SSH/Telnet三入口。实时地址 `ws://127.0.0.1:8080/api/v1/events`；连接后按resync_required查询HTTP，收到resource_changed刷新。完整错误、分页、幂等/容量、文件上传、关闭与部署边界见[API](docs/API.md)。

## 当前不能做什么

仓库目前不支持Probe/Server进程重启后的任务恢复、Process Manager、通用Tunnel或任意端口，也不提供Web UI、微信小程序、操作CLI、MCP或AI Agent。认证、TLS、RBAC和完整审计仍为后续设计点；当前API用于可信本机或受保护管理网络。

## 文档导航

- [AGENTS.md](AGENTS.md)：所有 AI Agent 和开发者必须遵守的工作与交付规则。
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
