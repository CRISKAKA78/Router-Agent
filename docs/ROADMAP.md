# 项目路线图

状态标记：

- [x] 已完成并可验证
- [ ] 未完成
- [-] 暂缓或未决

## Phase 0 Repository and Documentation Initialization

状态：已完成

- [x] 完整阅读 v0.2 Word 设计输入。
- [x] 建立 README.md、AGENTS.md 和 CHANGELOG.md。
- [x] 建立 ARCHITECTURE、PROTOCOL、API、ROADMAP、PROJECT_STATUS、HANDOFF 和 DECISIONS 文档。
- [x] 将 TCP 协议转换为仓库内 Markdown 基线。
- [x] 完成 Protocol v1 互操作细化，并保留未决安全、恢复、Tunnel、存储和 API 主题。
- [x] 建立项目事实来源层级和历史设计输入规则。
- [x] 将 Phase 1 拆分为可验证里程碑并设置技术决策门槛。
- [x] 核对文档间的阶段、架构、协议、任务与文件时序和能力状态。
- [x] 用户确认 Phase 0 最终文档。
- [x] 形成明确的 Git baseline commit。
- [x] 用户已授权在形成 Git baseline 后进入 Phase 1。

baseline commit: bc8d747dfc41a375c31698073005857c238ede51

## Phase 1 Probe and Server TCP Control Link

状态：已完成（Phase 1A～1E 验收通过）

Phase 1 保持一个总阶段，按 Phase 1A 至 Phase 1E 顺序推进。每个里程碑必须形成可构建、可运行、可测试的小闭环；不得一次性铺开整个 Phase 1。

### Phase 1 前置技术决策

进入 Phase 1A 业务开发前的设计门槛已经完成：

- [x] Probe：C++。
- [x] 最低语言标准：C++11。
- [x] 构建系统：CMake。
- [x] 第一开发与验证平台：Linux x86_64。
- [x] 交叉编译策略：后续使用 CMake toolchain files 适配 mipsel、ARM、ARM64；具体工具链版本按真实设备补充。
- [x] REGISTER、REGISTER_ACK、HEARTBEAT 和 HEARTBEAT_ACK 的完整字段契约与注册失败响应已写入 PROTOCOL.md。

Management Server 使用 Go 的 Accepted 决策保持不变。Phase 0 Git baseline、Phase 1A 与 Phase 1B 均已完成。

### Phase 1A TCP Session

状态：已完成

- [x] TCP framing，包括半包 Header、半包 Payload 和一次读取多帧。
- [x] 20-byte Header encode / decode 与 Big Endian 整数处理。
- [x] REGISTER 与字段校验。
- [x] REGISTER_ACK success=true / false。
- [x] HEARTBEAT。
- [x] HEARTBEAT_ACK 与 reply_to 校验。
- [x] heartbeat_interval 与 3 倍失联判断。
- [x] 1/2/5/10/30 秒基础断线重连。
- [x] 重连后重新 REGISTER 并生成新 session_id。
- [x] Linux x86_64 Server + Probe 可构建、可运行、可测试闭环。

### Phase 1B Task and Exec

状态：已完成

- [x] TASK。
- [x] TASK_ACK。
- [x] TASK_RESULT。
- [x] exec。
- [x] timeout。
- [x] 基础任务状态机。
- [x] 形成 Phase 1B 可构建、可运行、可测试闭环。
- [x] 接管审查 R1-R4 修复：注册与派发顺序、失败 writer 失效和不确定派发保留、exec socket 隔离、TERM/KILL 与 pipe 排空回归。

### Phase 1C Concurrency Idempotency and Reconnect

状态：已完成实现与验证，独立提交 `71e5d17` 已推送 GitHub main；后续 Phase 1D/1E 已完成。用户已明确确认三项互操作契约，见 PROTOCOL.md / ADR-015；R1-R4 前置修复已单独提交为 `59e65b4`。

- [x] 多任务并发。
- [x] 乱序结果。
- [x] task_id 幂等。
- [x] TCP 重连后的任务关联。
- [x] Probe 进程生命周期内的任务去重。
- [x] 形成 Phase 1C 可构建、可运行、可测试闭环。

验证：默认 4 workers；三个任务在执行屏障同时等待并反序完成；queued/running/完成态重复任务、同 ID 内容冲突、缓存容量、多次重连和真实 ACK/RESULT 丢失补报测试通过。Linux CTest、全量 Go 测试与真实 Probe 集成、Go race、vet，以及 Windows Server 构建/单测/vet 均通过，详见 PROJECT_STATUS.md。

### Phase 1D File Transfer

状态：已完成实现与全量验证，独立实现提交 `f1d9fa08d047f4f46a8bc27119565a2f6d217ecc` 已推送 GitHub main；后续 Phase 1E 已完成整体验收。P1-P5 与 sha256_ok 补充已确认，正式契约见 ADR-016/017。

- [x] upload。
- [x] download。
- [x] FILE_BEGIN。
- [x] FILE_ACK。
- [x] FILE_CHUNK。
- [x] FILE_END。
- [x] size 和 sha256 校验。
- [x] 文件传输期间控制消息不被饿死。
- [x] 形成 Phase 1D 可构建、可运行、可测试闭环。

### Phase 1E Verification

状态：已完成，Phase 1 完整验收通过；本次独立 Verification commit 交付后停止等待用户验收。

- [x] 协议单元测试。
- [x] Server 与 Probe 集成测试。
- [x] 非法 Header。
- [x] 非法 JSON。
- [x] payload limit。
- [x] 重复 task_id。
- [x] 网络中断。
- [x] 文件中断。
- [x] Phase 1 完整验收。

Protocol v1 的 14 项验收基线已映射至可运行测试，完整结果见 [PHASE1_VERIFICATION.md](PHASE1_VERIFICATION.md)。A～D 全量回归、C++/Go、Go race/vet、真实 Probe、C++ sanitizers 和 Windows Server 适用验证均通过。明确实现 bug 已修复，没有新增后续能力或变更 Accepted ADR。Phase 2 须另行授权。

## Phase 2 Device Management

状态：未开始

- [ ] 设备清单、状态与能力模型。
- [ ] 设备会话和连接状态管理。
- [ ] 设备信息查询与历史状态的最小可用闭环。

## Phase 3 File and Tool Repository

状态：未开始

- [ ] 文件资产管理。
- [ ] 工具元数据、版本与设备兼容性。
- [ ] 工具投放和文件传输的管理端闭环。

## Phase 4 Tunnel

状态：未开始

- [ ] Tunnel 控制流程。
- [ ] 独立数据连接与 Relay。
- [ ] SSH、Telnet 和 Web 临时访问闭环。

## Phase 5 HTTP and WebSocket API

状态：未开始

- [ ] /api/v1 HTTP API。
- [ ] 设备、任务、文件、工具、Tunnel 和会话能力。
- [ ] 实时事件接口。

## Phase 6 User Interfaces

状态：未开始

- [ ] Web 管理界面。
- [ ] Windows UI。
- [ ] 微信小程序。

各前端的实现顺序和技术栈为 TBD。

## Phase 7 MCP

状态：未开始

- [ ] 基于同一 Service 或公开 API 的 MCP Adapter。

## Phase 8 AI Agent

状态：未开始

- [ ] 基于平台 API、MCP 和临时通道的 AI 运维能力。

AI 诊断逻辑属于管理端能力，不进入 Probe。
