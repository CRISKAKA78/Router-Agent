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

状态：进行中

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

### Phase 1C Concurrency Idempotency and Reconnect

状态：未开始

- [ ] 多任务并发。
- [ ] 乱序结果。
- [ ] task_id 幂等。
- [ ] TCP 重连后的任务关联。
- [ ] Probe 进程生命周期内的任务去重。
- [ ] 形成 Phase 1C 可构建、可运行、可测试闭环。

### Phase 1D File Transfer

状态：未开始

- [ ] upload。
- [ ] download。
- [ ] FILE_BEGIN。
- [ ] FILE_ACK。
- [ ] FILE_CHUNK。
- [ ] FILE_END。
- [ ] size 和 sha256 校验。
- [ ] 文件传输期间控制消息不被饿死。
- [ ] 形成 Phase 1D 可构建、可运行、可测试闭环。

### Phase 1E Verification

状态：未开始

- [ ] 协议单元测试。
- [ ] Server 与 Probe 集成测试。
- [ ] 非法 Header。
- [ ] 非法 JSON。
- [ ] payload limit。
- [ ] 重复 task_id。
- [ ] 网络中断。
- [ ] 文件中断。
- [ ] Phase 1 完整验收。

Phase 1 必须满足 [PROTOCOL.md](PROTOCOL.md) 的规则和验收基线。Phase 0、Phase 1A 与 Phase 1B 已完成；未获得明确授权前停止，不进入 Phase 1C。

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
