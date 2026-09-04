# 路由器远程运维平台

本项目用于建设一套由 Management Server 和路由器端 Probe 组成的远程运维平台。Management Server 统一承载设备、任务、文件与工具、Tunnel 和对外 API 等核心能力；Probe 主动连接 Server，并向上提供轻量、通用的设备控制原语。

Phase 0 设计已经完成最终复核并获得用户确认。业务代码尚未开始；当前只等待在真实 Git 仓库形成 Phase 0 baseline commit，随后进入 Phase 1A TCP Session。

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
- 当前没有可构建、可运行的 Server 或 Probe。
- 当前没有任何实际远程运维功能。
- baseline commit: pending（当前附件目录无 `.git`，需在真实项目仓库完成）。
- Probe 技术栈已确定为 C++11 + CMake；Phase 1A 首轮在 Linux x86_64 验证。
- REGISTER / HEARTBEAT 字段契约已冻结；形成 Git baseline 后可以进入 Phase 1A。

## 当前不能做什么

仓库目前不能监听 Probe 连接、注册设备、收发心跳、下发任务、传输文件、建立 Tunnel，也不提供 HTTP API、WebSocket、Web UI、微信小程序、Windows UI、CLI、MCP 或 AI Agent。

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

TBD。当前没有业务代码，因此没有构建或运行命令。进入 Phase 1 后，应在实现可运行闭环的同时补充经过验证的命令。

## 设计基线

Phase 0 完成后，仓库内 Markdown 文档是项目持续维护的当前设计基线。原始 Word v0.2 文档保留为 Phase 0 设计输入和历史参考，不覆盖后续经过正式确认并写入 Markdown 或 ADR 的变化。事实来源层级和冲突处理规则见 [AGENTS.md](AGENTS.md)。

任何协议、架构或重要决策的变化都必须经过明确评审，并同步更新对应文档。需要改变 Accepted ADR 时，应新增 superseding ADR，不能由实现静默改变历史决定。
