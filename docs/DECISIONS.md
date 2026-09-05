# 架构决策记录

本文件使用轻量 ADR 记录已经确认的重要设计决定。状态为 Accepted 的决定不得被实现静默改变；需要变更时，应新增取代决策并说明迁移与影响。

## ADR-001 Probe 主动建立 TCP 长连接

- 状态：Accepted
- 日期：2026-09-05
- 决定：Probe 主动连接 Management Server，并保持 TCP 控制长连接。
- 背景与原因：路由器通常位于 NAT 或防火墙之后，由设备主动连接公网管理端更符合网络可达条件。长连接也为注册、心跳、任务和事件提供统一通道。
- 影响：Probe 负责连接、注册、心跳、退避重连和会话更新；Server 维护 device_id 到当前连接的映射。每次重连后重新 REGISTER 并生成新的 session_id。

## ADR-002 控制 TCP 与 Tunnel 数据连接分离

- 状态：Accepted
- 日期：2026-09-05
- 决定：Probe 控制 TCP 不承载 SSH、Telnet 或 Web Tunnel 的持续数据流；这些流量使用独立数据连接。
- 背景与原因：阻塞的交互会话或大流量请求不能影响设备心跳和管理任务。
- 影响：open_tunnel 和 close_tunnel 通过控制链路管理 Tunnel 生命周期，实际交互字节流由独立 Tunnel 或 Relay 连接承载。Tunnel 数据面协议仍为 TBD。

## ADR-003 Management Server 优先采用 Go

- 状态：Accepted
- 日期：2026-09-05
- 决定：Management Server 优先使用 Go，实现为可直接运行的跨平台核心程序。
- 背景与原因：平台至少需要覆盖 Linux 和 Windows，并保持基础部署简单。
- 影响：实现应避免依赖 systemd、固定 Linux 路径或只能在单一操作系统运行的组件。基础功能不强制依赖一组微服务、外部数据库或消息队列。后续可扩展 macOS。

## ADR-004 采用 API First

- 状态：Accepted
- 日期：2026-09-05
- 决定：核心能力先实现于 Application 或 Service Layer，再由 HTTP、WebSocket、Web、微信小程序、Windows UI、CLI、MCP 和 AI Agent 等 Adapter 或 Client 使用。
- 背景与原因：多个前端需要共享一致的设备控制、任务、文件和 Tunnel 语义。
- 影响：UI 和 Adapter 不得直接操作 Probe 连接注册表、存储表或内部 Manager，也不得重新实现核心设备控制逻辑。公开 API 预留 /api/v1 版本前缀。

## ADR-005 项目状态持久化在仓库文档

- 状态：Accepted
- 日期：2026-09-05
- 决定：项目状态、接管信息、路线图、协议、架构和重要决策必须维护在仓库中，不依赖 AI 会话上下文。
- 背景与原因：全新的 AI Agent 或开发者必须能在没有历史聊天的情况下确认事实并继续开发。
- 影响：PROJECT_STATUS 保存当前事实快照，HANDOFF 提供接管入口，ROADMAP 保存阶段状态。每次有效开发任务结束必须同步这些文档；代码完成但文档未同步视为交付不完整。

## ADR-006 Probe 保持轻量

- 状态：Accepted
- 日期：2026-09-05
- 决定：Probe 只提供连接、协议、执行、文件、进程、Tunnel 和设备信息等通用原语，不承担具体 AI 故障诊断逻辑。
- 背景与原因：Probe 面向 BusyBox、uClibc、多种 CPU 架构和较老 Linux 内核，需要控制体积、依赖和资源消耗。诊断知识与工具编排更适合集中在管理端。
- 影响：管理端承担工具仓库、脚本、设备能力判断和 AI 编排。Probe 不使用重型数据库，只保存必要的小规模持久状态。

## ADR-007 按阶段保持可验证基线

- 状态：Accepted
- 日期：2026-09-05
- 决定：项目按 ROADMAP 阶段推进，每个阶段优先形成可构建、可运行、可测试的闭环；不得跨多个阶段长期堆积不可验证代码。
- 背景与原因：大量空框架和并行未完成阶段会掩盖真实进度，并降低后续接管可靠性。
- 影响：未实现能力必须在 PROJECT_STATUS 和 ROADMAP 中明确标记。不得用占位代码、空类、空接口或大量空目录表示完成。进入新阶段前应确认上一阶段最低验收标准，除非 ROADMAP 明确记录例外。

## ADR-008 Markdown 是持续维护的当前设计基线

- 状态：Accepted
- 日期：2026-09-05
- 决定：Phase 0 完成后，仓库内 Markdown 文档成为项目持续维护的当前设计基线。Word v0.2 保留为 Phase 0 原始设计输入和历史参考。
- 背景与原因：项目需要在保留原始设计来源的同时，允许经过正式确认的设计继续演进。
- 影响：Word v0.2 不得覆盖后续已经写入 Markdown 或 ADR 的正式变化。文档冲突必须按 AGENTS.md 的事实来源层级分类处理。需要改变 Accepted ADR 时新增 superseding ADR，不静默改写历史决定。

## ADR-009 Protocol v1 统一传输关联与编码规则

- 状态：Accepted
- 日期：2026-09-05
- 决定：Protocol v1 统一 message_id、reply_to、Flags、transfer_id、JSON、boot_id 和默认心跳超时的互操作规则。
- 背景与原因：v0.2 已经固定帧结构和消息类型，但这些细节若由 Server 与 Probe 分别推断，会产生不兼容实现。
- 影响：
  - message_id 从 1 开始，按单连接和单发送方向独立递增，重连重置，0 保留，达到 uint64 最大值时重连且不回绕。
  - RESPONSE JSON 使用 reply_to 关联请求；TASK_RESULT 和 EVENT 不设置 RESPONSE。
  - FILE_CHUNK 设置 BINARY，JSON 消息不设置 BINARY，MORE 在 Protocol v1 中为 0。
  - transfer_id 统一为 canonical UUID string 和 RFC 4122 16-byte wire format。
  - JSON 使用 UTF-8 object 并遵循 PROTOCOL.md 的基础校验规则。
  - boot_id 优先使用系统 boot identity，退化值只代表 Probe 生命周期。
  - 默认心跳间隔为 30 秒，双方默认 90 秒未收到合法消息时判定失联或重连。

## ADR-010 Phase 1 任务拒绝与幂等边界

- 状态：Accepted
- 日期：2026-09-05
- 决定：TASK_ACK accepted=false 是最终拒绝，不再发送 TASK_RESULT；Protocol v1 Phase 1 不发送 TASK_RESULT rejected，也不实现 TASK_CANCEL。task_id 去重至少覆盖同一个 Probe 进程生命周期，包括 TCP 断开和重连。
- 背景与原因：拒绝状态、取消范围和重连后的任务重复执行需要在编码前形成单一语义。
- 影响：Server 根据 accepted=false 将业务任务记为 rejected。0x13 保留但 Phase 1 按未支持操作处理。Probe 对 RUNNING 或已缓存完成的 task_id 返回已有状态或结果，不重新执行副作用操作。Probe 重启后的恢复和持久化仍为 TBD。

## ADR-011 Phase 1 文件传输由 Server 编排

- 状态：Accepted
- 日期：2026-09-05
- 决定：upload 和 download 的 transfer_id 均由 Management Server 创建，并贯穿 TASK 与全部 FILE 消息。Probe 不接收 local_asset、tool_id 或 asset 等 Server 内部标识。一个 Probe 控制连接同时最多存在一个 active file transfer。
- 背景与原因：统一 transfer_id、文件元数据和时序可以避免 Server 资产模型泄漏到 Probe，也能防止多个文件流饿死控制消息。
- 影响：upload 和 download 使用 PROTOCOL.md 中固定的 TASK_ACK、FILE_ACK 和 TASK_RESULT 时序。写入同一 socket 必须由单一 writer 或等价机制串行化；控制消息优先于 FILE_CHUNK。Phase 1 不实现 FILE_CHUNK 单块 ACK、sliding window 或断点续传。

## ADR-012 Phase 1 使用里程碑和技术决策门槛

- 状态：Accepted
- 日期：2026-09-05
- 决定：Phase 1 保持一个总阶段，但按 Phase 1A 至 Phase 1E 的可验证里程碑推进。进入 Phase 1A 业务开发前必须确认 Probe 语言、最低语言标准、构建系统、第一验证平台和交叉编译策略。
- 背景与原因：每个里程碑需要形成可构建、可运行、可测试的小闭环，避免一次性铺开整个 Phase 1。
- 影响：上述 Probe 技术选择保持 TBD，不得由实现者自行决定。Management Server 使用 Go 的 ADR-003 保持不变。用户确认 Phase 0 并形成 Git baseline 之前不得开始 Phase 1。
- 后续状态：该技术决策门槛已由 ADR-013 完成；Phase 1A 注册与心跳互操作契约由 ADR-014 完成。Git baseline 仍需在真实仓库形成。

## ADR-013 Probe 使用 C++11 与 CMake

- 状态：Accepted
- 日期：2026-09-05
- 决定：Probe 从 Phase 1 起使用 C++11 实现，使用 CMake 作为构建系统。第一开发与验证平台为 Linux x86_64；后续通过 CMake toolchain files 支持 mipsel、ARM 和 ARM64 交叉编译。
- 背景与原因：Probe 后续需要连接管理、协议编解码、并发任务、文件和 Tunnel 等多个长期模块，C++11 能在保持嵌入式兼容性的同时提供 RAII、线程和类型封装能力；CMake 只存在于构建主机，不增加目标路由器运行时依赖。
- 影响：Probe 代码不得依赖高于 C++11 的语言特性；Phase 1A 首先在 Linux x86_64 上形成闭环，再逐步验证交叉编译。目标侧基础依赖应尽量收敛到 libc、pthread、C++ 标准运行库和明确审查过的轻量依赖。具体 mipsel、ARM、ARM64 工具链版本及最低 libc/内核兼容矩阵后续按真实设备补充。

## ADR-014 Phase 1A 注册与心跳契约冻结

- 状态：Accepted
- 日期：2026-09-05
- 决定：REGISTER、REGISTER_ACK、HEARTBEAT 和 HEARTBEAT_ACK 的必选字段、类型、允许范围、注册失败响应以及 heartbeat_interval 范围按 PROTOCOL.md 的 Phase 1A 规范执行。
- 背景与原因：Phase 1A 的 Server 与 Probe 必须能够独立实现后稳定互操作，不能只依赖 JSON 示例推断字段语义。
- 影响：REGISTER_ACK 成功与失败均使用 0x02 并设置 RESPONSE；失败后连接关闭且 Probe 不进入 ONLINE。heartbeat_interval 允许 10-300 秒，双方失联阈值统一为 3 倍 heartbeat_interval；默认 30 秒对应 90 秒。任何字段契约变化必须先更新 PROTOCOL.md 并形成正式评审。

## 当前待决策主题

以下主题尚未形成 Accepted ADR：

- Management Server 的持久化、备份与迁移方案。
- 平台和设备的认证、授权、加密与审计方案。
- Tunnel 数据面协议和 Relay 拓扑。
- Probe 重启后的 task_id 缓存、未上报结果和任务恢复策略。
- Server 重启后的任务、Session 和传输恢复策略。
- Protocol 错误严重程度与关闭连接矩阵。
- API 的正式资源模型、错误、认证和实时事件协议。

协议层的具体未决项见 [PROTOCOL.md](PROTOCOL.md) 的 Protocol Review。

### 2026-09-05 Phase 1C 接管审查补充

审查时尚未确认的重复 TASK、内容冲突和补报契约，在启动检查后由用户明确确认，现见 ADR-015。审查历史保留在 [PHASE1AB_REVIEW.md](PHASE1AB_REVIEW.md)，既有 Accepted ADR 保持原文。

## ADR-015 Phase 1C 重复任务与跨连接结果契约

- 状态：Accepted
- 日期：2026-09-05
- 确认依据：用户在 Phase 1C 启动检查汇报后明确回复“确认”，接受 PROTOCOL.md 记录的三项契约及比较、重放范围建议。
- 性质：补充 ADR-009/010 未展开的互操作细节，不取代其传输编号、最终拒绝或进程生命周期去重决定。
- 决定：重复 queued/running 返回 accepted=true 及对应 state；完成态先发送 accepted=true/state=success/failed/timeout 的 TASK_ACK，再返回原 TASK_RESULT。ACK.reply_to 关联本次 TASK，RESULT 不设置 RESPONSE。
- 执行身份：比较已解析 type、timeout、command、cwd、env；env 键顺序无关，省略 cwd/env 与空值等价；忽略 created_at 和未知扩展字段。相同 ID 内容冲突通过 ERROR/INVALID_PAYLOAD 响应本次 TASK，保留原任务，绝不重执行。
- 结果恢复：每次成功 REGISTER 后重放缓存完成结果，包括旧连接上写成功的结果；运行中 exec 继续执行，完成后通过可用连接发送。不得伪造旧 ACK，不新增 RESULT_ACK。Server 对同 device_id 的已派发 task_id 可以在缺 ACK 时接受结果。
- 幂等终态：已定义结果业务字段完全相同才是重复 RESULT；幂等成功，冲突返回 ERROR/INVALID_PAYLOAD 且不覆盖首个终态；rejected 不可被结果改写。ACK 必须匹配 session_id/message_id/task_id 派发记录。
- 资源边界：允许有界缓存，不淘汰已接受任务的身份与结果，容量不足时拒绝新任务。具体容量和 worker 数属于实现配置；Probe 与 Server 重启恢复仍为 TBD。

## ADR-016 Phase 1D 授权与已确认文件传输约束

- 状态：Accepted
- 日期：2026-09-05
- 确认依据：用户明确要求开始 Phase 1D，并列出十二项已确认规则及交付边界。
- 性质：补充 ADR-011 原先未决定的等待策略；保留 ADR-009/010/011/015 的既有决定。
- 决定：每个 Probe 控制连接最多一个 active file transfer；其他已接受文件任务使用有界 FIFO，容量是实现配置，满时拒绝新文件任务。
- 编码：Server 创建 canonical UUID transfer_id，CHUNK 使用 RFC 4122 16 bytes；内容必须为 binary FILE_CHUNK 且设置 BINARY，其余 FILE 消息为 JSON。
- 生命周期：upload/download 兼容 Phase 1C task_id 幂等，重复 ID 不重复文件副作用；中断的 transfer_id 失败，重传必须使用新的 task_id 和 transfer_id，不实现 resume。
- 流与校验：流式读写，校验 size/SHA-256；不完整或校验失败的文件不可发布为最终文件；控制消息优先，每个 CHUNK 后重查控制队列。
- 边界：不做逐块 ACK、sliding window、多文件并行流或专用数据连接；保留 Phase 1A/B/C 行为，不进入 Phase 1E 或用户明确排除的后续能力。
- 交付：完整回归、Go race/vet、C++ 与真实 Probe 集成通过后，同步文档，独立提交 Phase 1D 并推送 GitHub，停止等待验收。该 ADR 接受时仅完成启动检查；后续实现与验证状态以 PROJECT_STATUS.md 为准。

## ADR-017 Phase 1D 文件互操作补充

- 状态：Accepted
- 确认依据：用户明确回复“确认 P1～P5，按草案继续实现”，并要求 ready 省略 sha256_ok、done 固定 true、failed 固定 false。
- 日期：2026-09-05
- 基线：main `5030322b58fcbb07cae2f3256a71eb9a750a479a`。
- 问题：现有 FILE 章节主要是示例；未定义有界 FIFO 晋升时的握手、全部必选字段、CHUNK 长度口径、传输阶段失败关联、排队任务断线处理和文件提交后确认丢失的结果边界。ADR-015 的执行身份字段只覆盖 exec。
- 决定：采用 PROTOCOL.md 末尾 P1-P5 规范，复用既有消息号，不增加 ACK 类型、取消消息或恢复协议。
- 影响：P1 明确 queued upload 可登记 FILE_BEGIN 但延后 ready；P2 冻结字段与帧长度口径；P3 将 Phase 1 接收限制为顺序 offset 并明确失败处理；P4 扩展文件任务执行身份；P5 固定超时、断线和提交确认边界。
- 与历史决定关系：补充 ADR-011/015；P3 正式取代 PROTOCOL.md 原来“发送端建议顺序发送、offset 支持乱序校验”在 Phase 1 的宽松表述，改为强制连续 offset。ADR-009/010/011/015 原文保留。
- 实施：按正式协议实现与测试 Phase 1D，全部验证后独立提交推送并停止等待验收。
