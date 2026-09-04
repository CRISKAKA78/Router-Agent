# 路由器远程运维平台

本项目用于建设一套由 Management Server 和路由器端 Probe 组成的远程运维平台。Management Server 统一承载设备、任务、文件与工具、Tunnel 和对外 API 等核心能力；Probe 主动连接 Server，并向上提供轻量、通用的设备控制原语。

Phase 0 设计已经完成最终复核、获得用户确认并形成 Git baseline。Phase 1A TCP Session 已实现并通过 Linux x86_64 构建与测试，项目在此停止，等待 Phase 1B 的明确授权。

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

- 已建立项目入口、架构、协议、API 基线、路线图、状态快照、接管手册、决策记录和变更记录。
- 已将 Word 设计输入中的 TCP 协议整理为可维护的 Markdown 基线。
- 当前可构建并运行 Go Management Server 与 C++11 Probe。
- Probe 可主动连接、REGISTER、进入 ONLINE、收发 HEARTBEAT，并在断线后按 1/2/5/10/30 秒退避重连和取得新 session_id。
- baseline commit: `bc8d747dfc41a375c31698073005857c238ede51`。
- Probe 技术栈为 C++11 + CMake；Phase 1A 已在 Linux x86_64 完成首轮验证。
- REGISTER / HEARTBEAT 字段契约已冻结，当前实现符合该契约。

## 当前不能做什么

仓库目前不能下发任务、执行命令、传输文件或建立 Tunnel，也不提供 HTTP API、WebSocket、数据库、任何 UI、CLI、MCP 或 AI Agent。

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

终端二启动 Probe：

~~~sh
./build/probe/router-probe --server 127.0.0.1:9000 --device-id demo-router-001
~~~

运行自动化测试：

~~~sh
ctest --test-dir build/probe --output-on-failure
RMP_PROBE_BIN="$PWD/build/probe/router-probe" go test ./...
~~~

## 设计基线

Phase 0 完成后，仓库内 Markdown 文档是项目持续维护的当前设计基线。原始 Word v0.2 文档保留为 Phase 0 设计输入和历史参考，不覆盖后续经过正式确认并写入 Markdown 或 ADR 的变化。事实来源层级和冲突处理规则见 [AGENTS.md](AGENTS.md)。

任何协议、架构或重要决策的变化都必须经过明确评审，并同步更新对应文档。需要改变 Accepted ADR 时，应新增 superseding ADR，不能由实现静默改变历史决定。
