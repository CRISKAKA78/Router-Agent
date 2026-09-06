# 路由器远程运维平台架构基线

本文定义 Management Server、Probe、对外客户端和传输通道之间的长期边界。Phase 1 已实现 TCP Session、并发 exec、进程内幂等、跨 TCP 结果补报与双向文件传输；Phase 2 已实现 Device Inventory 和内部查询；Phase 3 已实现持久 File/Tool Repository、兼容判断和管理端文件投放/下载导入，验证状态见 PROJECT_STATUS。Phase 5已增加统一HTTP/WebSocket Adapter；其他未交付客户端仍是后续方向，实际验证以PROJECT_STATUS为准。

Phase 4 新增 `internal/tunnel.Service` 与 C++11 `TunnelManager`，采用 Accepted ADR-021 / ADR-022 的极简 TCP Maintenance，不使用 FRP/xfrpc。验证状态以 PROJECT_STATUS 为准。

## Phase 5 HTTP / WebSocket 模块与生命周期

- `internal/api.Server`是HTTP/WebSocket Adapter，`api.Run`组合两个listener及management.Server；`cmd/server`提供独立http-listen及容量配置。HTTP路径、DTO、错误、分页、幂等账本及事件wire属于Adapter；不访问Gateway连接表、Task/File内部map或Repository存储表。
- `management.Server`补齐CreateExec、Disconnect、Tasks和Revisions入口；任务摘要分页由Task Service查询，原Create/Resend/ACK/RESULT语义保持。Repository.ReadContent以reader回调提供流式内容，不把本地路径交给HTTP。
- Device/Task/File/Repository以原子变更计数提供刷新提示。Device不对心跳Seen计数；File在提交/释放/失败计数；Repository只对成功目录提交计数。计数不记录状态、不回调Adapter、不持有订阅者，原锁序保持。Maintenance只读原Service有界List，数据面未改变（仅冲突错误增加稳定sentinel供HTTP映射）。
- 一个250ms实时worker采样计数及Maintenance有界快照，四类resource_changed通知合并中间变化；files包括Repository目录。客户端首次/重连收到resync_required后查HTTP，不能把事件作为状态或审计事实。每客户端独立8槽队列、reader与writer、读写期限；满队列关闭客户端，业务和其他订阅者不等待。
- 幂等账本只记录HTTP请求指纹、完成信号与响应，默认4096项，不保存第二套业务对象，不淘汰身份。相同键等待原创建或返回原响应；指纹冲突409，满后拒绝新键。准入后创建context归API生命周期，客户端取消不撤销成功Task/Maintenance；流导入仍观察请求取消并完整校验再发布。task_id幂等和原Service资源生命周期保持独立。
- 原生HTTP监听限制默认128 TCP连接、32并行请求、64 WebSocket，JSON64KiB、流式资产1GiB及超时；查询输出分页。API添加的创建/重发受账本容量限制；Device/Task等原有Service历史与Repository磁盘保留政策不变。Response DTO隐藏LocalPath、内部诊断与数据面私有身份；临时三入口仍来自Maintenance Snapshot。
- shutdown先封闭API准入、取消准备、关闭HTTP及所有已升级WebSocket，再join已准入工作和实时worker；随后关闭Maintenance/Gateway/Repository。所有WaitGroup Add与closing检查在同一准入锁内，Close不会遗漏正在升级的连接。嵌入者若仅使用ServeHTTP，外层HTTP Server自行拥有网络超时、连接限额和listener关闭。
- 唯一新增Go依赖为gorilla/websocket v1.5.3（RFC6455适配）；Probe仍C++11且无变化。默认loopback HTTP8080，远程部署的认证/TLS/网络边界由部署层负责；认证/RBAC/审计/跨进程API恢复仍TBD。API.md为正式v1契约，ADR-023记录决定。

## Phase 4 模块与生命周期

- `management.Server` 组合 Repository、Gateway、可选 Maintenance Service，通过 `Maintenance()` 暴露用例。程序 `cmd/server` 默认启用独立 data listener；嵌入式旧调用方 `Config.Tunnel=nil` 保持原行为。
- `internal/tunnel` 拥有维护/入口模型、租期、固定三服务、端口池、随机流身份和一次性 token、并发准入、配对、Relay、释放和有界关闭历史。它只依赖 Control 接口，不能读取 Gateway 内部映射。
- Gateway 在 Session 发布后可返回 BindTunnel 撤销句柄；结束、替换、Disconnect、writer失败及Server关闭同步使旧句柄失效。发送在 writer 准入时复核当前 Session；状态按收到消息的连接身份调用 Service.Report。无可丢事件依赖；Device/Task/File 既有契约不变。
- Gateway → Device 的原锁序保持。Tunnel 调用 Bind/Enqueue 时不持 Tunnel 锁；Enqueue 不等待网络，Gateway 不回调 Tunnel 进行网络 I/O。Tunnel 锁保护准入、流表、token消费、状态与端口池；关闭 listener/socket 是本地撤销，数据复制和控制发送在锁外。WaitGroup Add 在准入锁内完成，closing 后不能新增被等待操作。
- Maintenance 先绑定Session再分配三端口，创建发布前复核；监听失败全部回滚。每个 accept 单独申请 pending+active 配额，超额直接关闭。共享 data listener 的无身份握手另有容量和期限，不能无限建立 goroutine。
- 每个支持 Tunnel 的 Device Session 有一个 Gateway 发送 worker、64 项控制队列；满时直接拒绝。写前复核 CONNECT context 和 Session。Maintenance 的 pending select 独立处理期限，writer 排队或阻塞不占用已释放维护资源；Gateway 在 Session 退出关闭 control socket 后 join 自己的 worker。
- 关闭顺序：状态closing → listener.Close → cancel → data socket reset → external socket.Close → 等待listener/已配对握手/Relay worker → 端口进入隔离 → closed/Released → 尽力入队CLOSE。本地释放不等待控制发送。未知身份的未完成data握手属于Server全局资源，由握手期限和Server Close管理。
- 端口隔离默认24小时，不占listener/worker，独立于128项关闭历史、最多池大小；只有Released且到达ReusableAfter才可重新分配，满池拒绝。原始TCP没有外部客户端维护身份，隔离不能保证超窗或Server重启后旧地址永久隔离；永久隔离需部署不重叠的池/地址，见ADR-022。
- DataHost可为IP或DNS主机名；Server在Create中最多5秒解析，优先IPv4并固定至本次维护结束。并行Create受MaxMaintenance限额限制，解析不持生命周期锁，失败不分配资源；下次Create刷新DNS。Probe收到的仍是数值IP，无DNS或多地址重试逻辑。
- Probe 每控制Session一个TunnelManager，固定目标查表，建流worker持有自己的两个socket，以可取消poll实现connect、握手与Relay；fd设置CLOEXEC与ExecForkMutex同步，发送使用MSG_NOSIGNAL，不改变Probe/exec的SIGPIPE disposition。控制Reader只验证并准入、取消或回收已完成worker，不执行Relay和DNS。
- Server每方向32KiB、Probe每方向16KiB，socket发送/接收有界；慢端使另一端停止读。EOF排空后传播SHUT_WR，反向仍可传输；Probe在读EOF后仍检查data socket错误，reset撤销本地连接，普通HUP仍排空缓冲。两端任一方向读写推进刷新整条连接空闲期限，默认24小时，绝对租期优先。Probe普通poll为20ms；持续HUP时另有20ms退让避免忙循环，取消还受调度影响。
- Probe默认8条流、每流一线程，硬上限64；Server每维护/设备默认8条。Probe默认Relay缓冲合计256KiB，线程栈由libc决定，低内存设备可调小；大量浏览器连接可同步提高双方限额。控制Session析构先取消Tunnel再收敛FileManager。
- Maintenance、配对token、流、监听与历史均为进程内状态；重启清空，新Session不能继承。仅固定loopback 80/22/23，不实现外部Adapter或通用映射。

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

控制 TCP 只承载注册、心跳、任务、结果、事件、必要文件传输及 Tunnel 控制消息。Phase 4 的 SSH、Telnet 和 Web 持续交互流量通过 TUNNEL_CONNECT 建立独立数据连接，不使用 TASK 缓存。

~~~text
Management Server -- TUNNEL_CONNECT --> Probe
                                           |
                                separate data connection
                                           |
                                  Tunnel or Relay Server
                                           |
                               SSH / Telnet / Web traffic
~~~

该边界防止一个阻塞的交互会话或大量数据流量影响设备心跳和管理命令。Phase 4固定三服务数据面与生命周期由ADR-021及本文开头决定；通用Tunnel和平台认证仍未设计。

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

File 与 Tool 在 Phase 3 由同一 `internal/repository` 包内的独立 FileService/ToolService 承载，共用目录事务以保护资产引用。存储与兼容性按 Accepted [ADR-019](DECISIONS.md#adr-019-phase-3-file-and-tool-repository) 实现；不替代 `internal/filetransfer` 的传输职责。

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

当前实际仓库已包含 `cmd/server`、`internal/protocol`、`internal/gateway`、`internal/task` 和 `probe`，实现了 Phase 1A 的 framing、注册、心跳与基础重连，以及 Phase 1B 的 TASK、TASK_ACK、TASK_RESULT、exec、timeout 和基础任务状态，Phase 1C 的并发、去重和跨连接补报；Phase 1D 已增加 internal/filetransfer 与 Probe FileManager；Phase 2 已增加 internal/device；Phase 3 已增加 internal/repository 与 internal/management。示意中的其他模块尚未创建或实现。

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

Device/Session 生命周期、历史保留、进程内存储与 Gateway 边界已由 [ADR-018](DECISIONS.md#adr-018-phase-2-device-management) 确认；长期存储引擎、重启恢复、备份与迁移仍未决定。

- Management Server 的内置持久化方案、数据模型、备份和迁移策略。
- 平台认证、授权、设备认证、链路加密、密钥管理和审计策略。
- mipsel、ARM、ARM64 的具体交叉工具链版本，以及最低内核与 libc 兼容矩阵。
- 通用Tunnel及平台访问控制；固定三服务的Relay/租约已由ADR-021决定。
- Probe 自身重启后的 task_id 缓存、未上报结果和任务恢复策略。
- Server 重启后的任务与会话恢复策略。
- Repository 之外的长期存储、在线备份/迁移、物理 GC 与跨平台实机兼容矩阵仍待后续阶段；Phase 3 范围已由 Accepted ADR-019 决定。真实 Probe 目前省略 libc/kernel/model，兼容判断不能假定这些资料已知。
- Management Server 的配置格式、日志、指标和运维接口。
- API 的资源模型、认证、错误格式和实时事件协议。
- 各前端的实现顺序和技术栈。

## Phase 1C 实际模块职责

- Probe TaskManager 在 RunClient 进程生命周期中拥有任务表、固定 worker pool 和有界结果缓存；RunSession 只处理连接。worker 不持有 socket，TCP 断开时继续执行已接受的 exec。
- Connection / Message Router 负责 ACK、冲突 ERROR、心跳和缓存结果发送；同一网络线程串行发送帧。在线 poll 最多等待 50 ms，每轮发送一个待补报 RESULT 并继续处理控制消息。
- TaskManager 的登记、状态和缓存共用互斥锁；执行和网络写不持有该锁。新连接开始新的补报轮次，不清空任务身份。默认并发数和缓存计费见 PROTOCOL.md。
- Server Task Service 保存任务及各次 session_id/message_id 派发记录，处理 ACK、RESULT 幂等和等待；Gateway 适配 CreateExec / ResendTask / WaitTaskResult / TaskSnapshot，不引入外部 API。

## Phase 1D 实际模块职责

- Go internal/filetransfer 负责文件任务准备、流式源/接收器、FILE 字段校验、会话传输状态和本地提交事实；不实现 File/Tool Repository。internal/task 保存不可变文件参数和业务结果。Gateway 的 CreateUpload/CreateDownload 为内部用例入口，路由 FILE 帧并提供优先级串行发送适配。
- C++ FileManager 为单 TCP Session 拥有一个文件 I/O worker、deadline watcher、有界 FIFO 和接收邮箱；Reader 登记 TASK/BEGIN 并路由控制帧，不执行磁盘读写。TaskManager 拥有跨 Session 的文件身份、transfer_id 绑定、状态和结果缓存。旧 FileManager 停止并收敛结果后才建立新会话。
- 两端 writer 对完整帧串行化；等待中的控制发送优先于 CHUNK，文件 END 在此前数据全部写出后发送。发送缓冲目标为 64 KiB，避免大量提前排入的文件字节抵消控制优先级。
- 临时文件在目标同目录创建，完成校验后 rename 或无覆盖 hard link 发布。Server FileSnapshot.Committed 表示下载本地提交事实，与 Probe 任务最终确认分开记录，支持 done ACK 丢失后保留完整文件但任务失败的已确认契约。

## Phase 2 实际模块职责

- `internal/device.Service` 拥有按稳定 device_id 索引的 Inventory、注册资料、当前 Session、在线状态、时间及有界历史。它使用 RWMutex，写操作 Publish/Seen/End 与只读 List/Get/Sessions 在内存中完成，不依赖 TCP、Task、File 或数据库。
- Gateway 保留 `device_id -> *session` 的传输路由映射、socket、writer、连接 Reader 和协议心跳 deadline。它在完整 REGISTER_ACK 写出后，持 Gateway 锁按发布顺序安装连接并调用 Device.Publish；最新发布的 Session 替换旧 Session，旧 socket 在锁外关闭。注册失败及 ACK 写失败不创建 Inventory。
- 连接结束、主动 Disconnect、writer 失效及 Server Close 撤下当前传输映射并同步调用 Device.End。Gateway 以当前连接指针保护清理，Device 再以 session_id 保护状态；旧回调不影响新 Session。合法 HEARTBEAT、TASK_ACK/RESULT 和既有 FILE 路由成功处同步 Seen；3 倍心跳 deadline 继续由连接适配层执行。
- 锁顺序为 Gateway → Device，Device 不回调 Gateway。Device 锁和 Gateway 连接表锁均不跨网络/文件 I/O；writer 失败通知可取得 Gateway 锁，Gateway 不在连接表锁内取得 writer 锁。Device 状态不依赖现有容量 128 的可丢 Events 通知。
- online/offline 是设备业务状态。Session replaced 时直接结束旧 Session 并发布新 Session，设备保持 online；LastOnlineAt 更新为新发布时刻，LastOfflineAt 保持原值。设备真正下线才更新 LastOfflineAt；未经历离线时为零值。
- 当前 Session 之外，默认保留最近 64 个已结束 Session 及对应注册快照；按结束操作顺序淘汰并暴露计数。离线设备、首次/最近时间和累计数保留至 Server 进程结束，设备数量无自动淘汰。只记录 Session 状态历史，不存心跳时序或审计事件流。
- `Server.Devices()` 暴露 `device.Query`，返回独立查询副本；当前/最近 Session、时间、历史容量与字段语义见 API.md。Task/File 规格、派发关联、幂等和提交事实仍属于原 Service；Device 历史淘汰不影响这些记录。

## Phase 3 实际模块职责

- `internal/repository.Store` 拥有持久目录、文件系统锁、schema_version=1 JSON 目录和资产/工具引用的一致提交；FileService 管理不可变字节、资产身份、查询与归档，ToolService 管理工具、版本、产物、约束与归档。稳定 UUID tool_id/asset_id/artifact_id 不因归档、去重或存储路径改变而重用，artifact_id 全仓库唯一。
- `internal/management.Service` 组合 Repository、device.Query 和 FileTasks 接口，实现兼容查询、资产上传、工具投放、下载暂存与显式导入、Operation 关联。它不编码 wire、不访问连接表；现有 gateway.Server 实现 FileTasks，所有传输仍由 internal/filetransfer 与 Task Service 执行。
- 程序入口改由 management.Server 组合持久 Store 与 Gateway。`-repository-dir` 默认 `./data/repository`，相对启动工作目录解析并记录绝对路径；创建目录或锁定失败即启动失败。Close 先停止 Gateway/传输，再关闭仓库。基础部署无新增外部服务或 Go 第三方依赖。
- 目录为 `blobs/sha256/<两位>/<摘要>`、`metadata/catalog.json`、`staging/` 和 `repository.lock`。资产名称只作标签，不参与路径；相同 SHA-256 的多个资产共用一个完整 blob。blob 先完整发布、再发布元数据；失败可能留下未引用的完整 blob，但不能留下可用资产指向半文件。
- Linux 使用 flock，Windows 使用 LockFileEx 锁定整个 Store 生命周期，进程退出由 OS 释放；锁文件保留初始化标记，防止仅有工具元数据的目录在 catalog 丢失后被静默初始化为空。目录移动后所有 blob 路径重新由摘要解析，ID 保持。
- 元数据提交在仓库互斥锁内复制当前目录、写同目录临时文件、Sync/Close 后发布；Linux rename，Windows MoveFileExW(REPLACE_EXISTING | WRITE_THROUGH)。失败不发布内存状态；成功重开可读。启动拒绝损坏格式、重复身份/JSON key、未知 schema、非法引用或缺失/大小不符 blob；使用或去重复用内容时重新验证摘要。该边界不承诺所有文件系统/突然断电下完整恢复。
- 管理端以设备注册快照执行兼容规则，缺失受限字段为 unknown，仅 compatible 可投放；实际传输派发前执行通用 Session 检查。版本/资产解析后，以仓库读锁准入文件准备与派发，归档/Store Close 等待此段完成；不把仓库锁扩大为 Gateway/Device 锁。派发检查在 writer 内短暂获取 Gateway 锁、核对当前连接并记录 Task 派发，锁释放后写帧；后续替换不改变已准入任务的原目标。
- `internal/filetransfer` 只增加可选 Expected 内容校验及 Released 句柄释放事实；Gateway 只增加可选 Session 前置条件。没有第二套文件传输、Repository wire 字段或 Probe 变化。下载按 Committed + Released 导入，Task RESULT 单独呈现，ACK 丢失不回滚已完整提交文件。
- 归档保留全部元数据与 blob，阻止新引用/投放而不取消已派发任务；版本标签不重用。崩溃遗留暂存、未引用 blob 和元数据临时文件只报告，不自动 GC。当前进程可以清理自己已释放的下载暂存，已提交文件须先导入。停服后备份整个目录；在线备份和迁移工具未实现。
- Repository 是进程内目录索引加单写者 JSON 快照，面向小规模仓库；内容流式处理，但元数据整体读写，资产/工具/版本数量和磁盘使用无自动淘汰。Operation/Task/transfer/Device/Session 仍为进程内状态，重启不恢复或自动重发任务。当前平台适配只实现 Linux/Windows；Phase 5通过Adapter复用以上仓库行为。
