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

状态：已完成，Accepted ADR-018 与时间语义已实现，Phase 1/2 全量验收通过；独立 Phase 2 commit 交付后停止等待用户验收。

- [x] 设备清单、状态与能力模型。
- [x] 设备会话和连接状态管理。
- [x] 设备信息查询与历史状态的最小可用闭环。

正式 Device Service / Inventory 与内部 List/Get/Sessions；默认当前 Session 加最近 64 条已结束历史，进程内保留。LastOnlineAt 为最近发布时刻，LastOfflineAt 只在整体下线时更新，replaced 不更新。单元、真实 Probe、Phase 1 全量回归、Go race/vet、C++ sanitizers 与 Windows 适用验证均通过，见 [PHASE2_VERIFICATION.md](PHASE2_VERIFICATION.md)。未引入数据库、外部 API 或 Phase 3 能力。

## Phase 3 File and Tool Repository

状态：已完成，Accepted ADR-019 与稳定身份补充已实现并通过自动化验收，Phase 1/2 全量回归通过；独立 Phase 3 commit 推送后停止等待用户验收。

- [x] 文件资产管理。
- [x] 工具元数据、版本与设备兼容性。
- [x] 工具投放和文件传输的管理端闭环。

以 main `b9982f5d2765546d23e09c28977c27ceb510a368` 为启动基线。用户已明确确认资产/元数据持久化、版本唯一性、兼容规则、内容去重/身份、归档清理；artifact_id 全仓库唯一，所有业务身份保留不重用。验证映射与结果见 PHASE3_VERIFICATION；独立提交推送后停止等待验收，不进入 Phase 4。

## Phase 4 Tunnel

状态：首版及本轮修正的实现和规定验收通过。本轮从首版main `f92d73a0d6003835993967848f1f5fe009a0df89`干净基线修正，采用Accepted ADR-021 / ADR-022，保留极简自研TCP。验证记录见PHASE4_VERIFICATION；修正独立提交推送后停止等待验收。

- [x] Maintenance一次创建三个固定服务入口，默认240分钟与自定义租期。
- [x] 独立data TCP、一次性随机token配对、原始字节Relay、half-close和有界背压。
- [x] SSH真实登录/命令、Telnet双向交互、Web多TCP连接及三服务/多设备并发。
- [x] 主动关闭/到期/Session替换/断线撤销，pending/active流实际终止；端口完全释放后进入隔离，到期才复用。
- [x] 控制writer阻塞/队列满不阻塞本地释放；half-close后reset仍回收Probe，正常EOF排空保持。
- [x] Server侧DataHost域名解析、Probe默认8条流、整连接idle默认24小时；历史文档只依赖已提交仓库。
- [x] 错误/重复/迟到配对、本地/data失败、配额与资源回收、大流量下控制/文件正常。
- [x] Phase 1～3全量回归、Go race/vet、C++ CTest/sanitizers、Linux Probe、Windows/Linux Server适用验证。

首版提交为 `f92d73a0d6003835993967848f1f5fe009a0df89`；本轮修正提交标题为 `fix: isolate Phase 4 ports and decouple maintenance revocation`，实际SHA与main推送结果由Git记录提供。

## Phase 5 HTTP and WebSocket API

状态：已完成并获用户验收。稳定提交 `57c2b1f8f6da1069e4a2eb988224b94bafe9cf84`，采用Accepted ADR-023；历史验证见PHASE5_VERIFICATION。

- [x] /api/v1 HTTP API。
- [x] 设备、Session、exec/Task、资产/文件传输、工具/版本/Artifact/兼容与Maintenance能力。
- [x] 有界WebSocket状态通知、重连同步与慢消费者关闭。
- [x] Phase 1～5全量回归、race/vet、C++ sanitizers及Windows/Linux适用验证。

## Phase 6 User Interfaces

状态：Windows UI已实现并通过规定验证，待用户验收；Web与微信未开始。首版f8d099d6后，用户明确要求C# + WinUI 3正式重构，采用Accepted ADR-025。

- [x] Windows UI：C# / .NET 10 LTS WinUI 3，中文Fluent界面与自包含x64目录发布。
- [x] Server连接设置、设备/Session、Maintenance三入口/租期/关闭、Exec结果与File/Tool/版本/兼容产物/投放。
- [x] WebSocket重连HTTP同步、原请求幂等重试、错误区分、重复点击和连接/退出资源回收。
- [x] WinUI原生控件/弹窗、Light/Dark、100%/200% XAML、窗口缩放与真实API验证、Phase 1～5全量回归和平台检查；记录见PHASE6_VERIFICATION。
- [ ] Web 管理界面（未授权）。
- [ ] 微信小程序（未授权）。

本轮独立Phase 6 WinUI 3重构commit推送main后停止等待验收。Web/微信技术栈及后续实施仍TBD，不进入Phase 7/8。

## Phase 7 MCP

状态：未开始

- [ ] 基于同一 Service 或公开 API 的 MCP Adapter。

## Phase 8 AI Agent

状态：未开始

- [ ] 基于平台 API、MCP 和临时通道的 AI 运维能力。

AI 诊断逻辑属于管理端能力，不进入 Probe。
