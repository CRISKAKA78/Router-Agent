# 路由器远程运维平台架构基线

本文定义 Management Server、Probe、对外客户端和传输通道之间的长期边界。Phase 1A 已实现 TCP Session，Phase 1B 已实现 Task and Exec，Phase 1C 已实现并发、进程内幂等和跨 TCP 会话结果补报；其余模块仍是已经确认的架构约束和后续实现方向，不表示已经实现。

## 项目目标

项目提供一套面向路由器和嵌入式 Linux 设备的远程运维平台。Management Server 统一管理设备连接、任务、文件与工具、临时 Tunnel 和对外 API；Probe 运行在设备侧，通过轻量、通用的控制原语执行管理端下发的操作。

基础部署应尽量保持简单：Management Server 目标为可直接运行的跨平台程序，至少支持 Linux 和 Windows，后续可扩展 macOS；Probe 面向 BusyBox、uClibc、mipsel、ARM、ARM64 和较老 Linux 内核环境。

## 系统边界

平台核心范围包括：

- Management Server 的 Probe TCP Gateway、设备会话、任务、文件与工具、Tunnel 和对外 API。
- Probe 的连接管理、协议编解码、通用任务执行、文件、进程、Tunnel 和设备信息能力。
- Web、微信小程序、Windows UI、CLI、MCP 与 AI Agent 使用的统一后端能力。

平台不把以下职责放入 Probe：

- 具体故障诊断知识或 AI 推理。
- 管理端的工具仓库与诊断编排。
- Web、移动端或桌面端的业务逻辑。
- 用户、权限和平台级审计策略。

## 总体架构

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
                    Control TCP
                         |
                       Probe
                         |
        Exec / File / Process / Tunnel / Info
~~~

所有面向用户或 AI 的操作先沉淀为 Management Server 的稳定核心能力，再通过 Adapter 暴露。任何客户端都不得成为第二套设备控制实现。

## Management Server

Management Server 是平台核心程序，优先使用 Go 实现。

它长期承担以下职责：

- 接受并管理 Probe 主动建立的 TCP 控制连接。
- 维护 device_id 到当前连接和 session_id 的映射。
- 管理设备清单、在线状态与设备会话。
- 创建、调度、等待和查询任务，保存任务结果。
- 管理文件资产、工具及其向设备的传输。
- 创建和关闭临时 Tunnel，并追踪 Tunnel 状态。
- 通过 HTTP 和 WebSocket 向多个客户端公开同一套核心能力。

第一版可以采用模块化单体。模块可以在未来拆分，但不得绕过 Service Layer 或形成第二套核心逻辑。基础运行不得强制用户部署一组微服务、外部数据库或消息队列。

## Probe

Probe 是与 Management Server 独立的设备端程序。Phase 1 起采用 **C++11 + CMake**：第一开发与验证平台为 Linux x86_64；后续通过 CMake toolchain files 适配 mipsel、ARM 和 ARM64 交叉工具链。目标侧只运行编译后的 Probe，不依赖 CMake。Probe 实现不得使用高于 C++11 的语言特性，并应避免不必要的重型运行时依赖。

Probe 应保持轻量：

- 主动连接 Server，负责注册、心跳、断线重连和会话状态。
- 提供 exec、文件、进程、Tunnel 和设备信息等通用原语。
- 在同一个 Probe 进程生命周期内使用 task_id 保证业务幂等；TCP 断开并重连后不得重复执行已经接受的同一 task_id。
- 支持受控并发和结果乱序返回。
- 只允许持久化必要的小规模状态。
- 不内置具体 AI 排障流程、设备诊断知识库或重型数据库。

Probe 的内部职责建议划分为 Connection Manager、Protocol Codec、Message Router、Task Manager、Exec Manager、File Manager、Process Manager、Tunnel Manager、Device Info Collector 和 Local State Store。该划分是职责基线，不要求 Phase 0 创建空源码目录或空实现。

### Probe 本地状态与幂等边界

device_id、Server 地址和基础配置是 Probe 本地持久化的基本候选。以下状态是否持久化仍为 TBD：

- 尚未上报的 TASK_RESULT。
- 已完成的 task_id。
- 正在执行的任务。
- 文件传输状态。

Protocol v1 Phase 1 只要求同一个 Probe 进程生命周期内的 task_id 幂等。即使 TCP 断开并重新连接，已经接受的同一 task_id 也不得再次执行副作用操作。Probe 自身重启后的 task_id 幂等、结果补报和任务恢复仍为 TBD。

## Server 与 Probe 控制链路

Probe 主动向 Server 建立 TCP 长连接，并按以下生命周期运行：

~~~text
CONNECT -> REGISTER -> REGISTER_ACK -> ONLINE
                                      |
                         HEARTBEAT / TASK / EVENT / FILE
                                      |
                    disconnect -> backoff -> reconnect
~~~

控制链路使用固定 20 字节二进制包头、JSON 控制载荷和 FILE_CHUNK 二进制文件块。协议细节以 [PROTOCOL.md](PROTOCOL.md) 为准。

连接与业务标识具有不同职责：

- message_id 用于一条 TCP 会话内的传输追踪。
- task_id 是任务的业务标识和幂等键；同一个 Probe 进程生命周期内可以跨 TCP 连接关联。
- transfer_id 标识一次文件传输。
- session_id 标识一次 REGISTER 后形成的设备会话；每次重连重新注册时生成新的 session_id。

## 控制面与 Tunnel 数据面

控制 TCP 只承载注册、心跳、任务、结果、事件和必要文件传输。SSH、Telnet 和 Web 等持续交互流量通过 open_tunnel 任务建立独立数据连接。

~~~text
Management Server -- TASK open_tunnel --> Probe
                                           |
                                separate data connection
                                           |
                                  Tunnel or Relay Server
                                           |
                               SSH / Telnet / Web traffic
~~~

该边界防止一个阻塞的交互会话或大量数据流量影响设备心跳和管理命令。Tunnel 的具体数据面协议、Relay 拓扑、认证和生命周期仍为 TBD。

## API First 与多前端

Management Server 采用 API First。客户端只消费公开 API 或稳定的内部 Service 接口：

| 客户端 | 定位 | 边界 |
| --- | --- | --- |
| Web | 主要管理界面 | 只调用 HTTP API 与 WebSocket |
| 微信小程序 | 移动端快速维护入口 | 不依赖后端内部实现 |
| Windows UI | 本地桌面管理工具 | 可调用同一 API 或内嵌 Web UI，不复制核心业务 |
| CLI | 调试与自动化薄客户端 | 不形成独立业务实现 |
| MCP 与 AI Agent | AI 工具入口 | 复用 Service 或 API，不直接拼装 Probe 协议帧 |

HTTP API 负责设备查询、任务创建、文件管理和 Tunnel 创建等请求响应操作。WebSocket 或等价实时接口负责设备上线离线、任务状态变化和日志流等实时事件。详细基线见 [API.md](API.md)。

## 分层与模块边界

~~~text
HTTP Handler ----\
WebSocket --------+--> Application / Service Layer --> Domain Services --> Probe Gateway
MCP Adapter ------+
CLI Adapter -----/
~~~

### Adapter Layer

Adapter 负责协议适配、参数解析、认证与上下文获取以及响应转换。它不得直接操作 Probe 连接、存储结构或拼装 TCP 协议帧。

### Application 与 Service Layer

该层组织用例和业务流程，为 HTTP、WebSocket、CLI、MCP 和其他 Adapter 提供可复用能力。接口应能被单元测试直接调用，不依赖启动 HTTP 服务。

### Domain Services

- Device：设备清单、设备状态和会话语义。
- Task：任务创建、调度、状态、幂等、等待与结果。
- File：文件资产及文件传输业务。
- Tool：工具元数据、兼容性和向设备投放。
- Tunnel：临时 Tunnel 的创建、状态与关闭。

File 与 Tool 可以在早期由同一模块承载，但职责必须能够区分。具体存储模型为 TBD。

### Probe TCP Gateway

Gateway 只负责连接注册、帧编解码、消息路由和协议适配。设备、任务、文件和 Tunnel 的业务规则属于对应 Service。

## 跨平台原则

- Management Server 的目标平台至少为 Linux 与 Windows，后续可扩展 macOS。
- 不依赖 systemd、固定 Linux 路径或仅单一操作系统可用的基础组件。
- 早期优先保持单程序加配置文件的运行方式。
- 外部数据库、消息队列和拆分服务可以作为未来扩展，但不得成为基础功能的默认强制依赖。
- 平台相关能力必须收敛在清晰的适配层中。
- Probe 必须根据嵌入式 Linux 的 libc、架构、内核和资源限制设计。

## 当前建议仓库结构

下面是后续实现的职责示意，不表示应在 Phase 0 创建所有目录：

~~~text
repo/
├─ cmd/
│  └─ server/          Management Server 入口 后续
├─ internal/
│  ├─ probe/           TCP Gateway 与协议适配
│  ├─ device/          Device 与 Session
│  ├─ task/            Task Service 与状态
│  ├─ file/            File 与 Tool Service
│  ├─ tunnel/          Tunnel Service
│  └─ api/             HTTP 与 WebSocket Adapter
├─ probe/              路由器 Probe 独立源码 后续
├─ docs/
├─ tests/              按需建立的集成与协议测试
├─ AGENTS.md
├─ README.md
└─ CHANGELOG.md
~~~

当前实际仓库已包含 `cmd/server`、`internal/protocol`、`internal/gateway`、`internal/task` 和 `probe`，实现了 Phase 1A 的 framing、注册、心跳与基础重连，以及 Phase 1B 的 TASK、TASK_ACK、TASK_RESULT、exec、timeout 和基础任务状态，Phase 1C 的并发、去重和跨连接补报；示意中的其他模块尚未创建或实现。

## 已确认的架构约束

- Probe 主动连接 Management Server。
- 控制 TCP 与 SSH、Telnet、Web Tunnel 数据连接分离。
- Management Server 优先使用 Go，并作为跨平台核心程序。
- 基础运行尽量不强制依赖外部数据库、消息队列或一组微服务。
- API First，多前端只作为 Client 或 Adapter。
- 核心业务、API Adapter 和 Probe Gateway 清晰分层。
- Probe 保持轻量，不承担具体 AI 故障诊断逻辑。
- Probe 从 Phase 1 起采用 C++11 + CMake，首轮 Linux x86_64 验证，后续通过 CMake toolchain files 适配 mipsel、ARM、ARM64。
- 项目状态和交接信息必须持久化在仓库文档中。
- 项目按阶段保持可构建、可运行和可测试，不用占位实现伪造进度。

## 待讨论

- Management Server 的内置持久化方案、数据模型、备份和迁移策略。
- 平台认证、授权、设备认证、链路加密、密钥管理和审计策略。
- mipsel、ARM、ARM64 的具体交叉工具链版本，以及最低内核与 libc 兼容矩阵。
- Tunnel 数据面协议、Relay 部署拓扑、访问控制和租约模型。
- Probe 自身重启后的 task_id 缓存、未上报结果和任务恢复策略。
- Server 重启后的任务与会话恢复策略。
- 文件与工具仓库的存储、版本、兼容性和清理策略。
- Management Server 的配置格式、日志、指标和运维接口。
- API 的资源模型、认证、错误格式和实时事件协议。
- 各前端的实现顺序和技术栈。

## Phase 1C 实际模块职责

- Probe TaskManager 在 RunClient 进程生命周期中拥有任务表、固定 worker pool 和有界结果缓存；RunSession 只处理连接。worker 不持有 socket，TCP 断开时继续执行已接受的 exec。
- Connection / Message Router 负责 ACK、冲突 ERROR、心跳和缓存结果发送；同一网络线程串行发送帧。在线 poll 最多等待 50 ms，每轮发送一个待补报 RESULT 并继续处理控制消息。
- TaskManager 的登记、状态和缓存共用互斥锁；执行和网络写不持有该锁。新连接开始新的补报轮次，不清空任务身份。默认并发数和缓存计费见 PROTOCOL.md。
- Server Task Service 保存任务及各次 session_id/message_id 派发记录，处理 ACK、RESULT 幂等和等待；Gateway 适配 CreateExec / ResendTask / WaitTaskResult / TaskSnapshot，不引入外部 API。
