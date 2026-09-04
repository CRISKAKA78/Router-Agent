# 项目状态

最后更新时间：2026-09-05

## 当前阶段

Phase 0 - Repository & Documentation Initialization

状态：设计复核已通过并获得用户确认。当前仅等待在真实 Git 仓库形成 Phase 0 baseline commit；baseline 完成后立即进入 Phase 1A。

baseline commit: pending

## 当前代码状态

尚未开始 Management Server 或 Probe 的业务代码开发。仓库中没有 TCP Listener、Probe、API、数据库、Tunnel、前端或其他可执行模块。

## 当前可用功能

无实际远程运维功能。当前交付物仅为设计输入、项目入口和文档体系。

## 当前已完成

- 完整读取并复核 v0.2 Word 设计输入。
- 建立 README.md、AGENTS.md 和 CHANGELOG.md。
- 建立 ARCHITECTURE、PROTOCOL、API、ROADMAP、PROJECT_STATUS、HANDOFF 和 DECISIONS 文档。
- 将 TCP 控制协议整理为仓库内 Markdown 基线。
- 完成 Protocol v1 的 message_id、reply_to、Flags、UUID、任务拒绝、文件时序、背压、JSON、boot_id、幂等范围和心跳超时细化。
- 将已解决的 Protocol Review 项移入规范正文，保留仍未决定的安全、恢复和实现策略。
- 增加 ADR-008 至 ADR-012。
- 将 Phase 1 拆分为 Phase 1A 至 Phase 1E 可验证里程碑。
- 完成 Phase 1A 前置技术决策：Probe 使用 C++11 + CMake，首轮 Linux x86_64 验证，后续通过 toolchain files 交叉编译。
- 冻结 REGISTER / REGISTER_ACK / HEARTBEAT / HEARTBEAT_ACK 的字段、范围和失败响应契约。
- 核对文档中的阶段状态、架构边界、协议常量、任务与文件时序和未实现能力。

## 当前未完成

- Probe 与 Server TCP framing。
- REGISTER、HEARTBEAT、TASK 和文件传输。
- Device、Task、File、Tool 和 Tunnel 服务。
- HTTP API 与 WebSocket。
- 数据持久化。
- Web、Windows UI、微信小程序和 CLI。
- MCP 与 AI Agent。

## 测试与构建

暂无业务构建或测试，因为当前没有程序代码。

本轮只执行文档级检查，因为当前没有程序代码。检查范围包括必需文件、Markdown 相对链接、Phase 状态、协议常量、Server 与 Probe 职责、Word 与 Markdown 的关系、TASK 与 FILE 时序、TBD 和实际文件类型。

## 已知问题

- 用户认证、Probe 身份认证、TLS、权限、租户、密钥轮换和审计尚未设计。
- Management Server 的数据库与持久化、配置和运维方案尚未决定。
- Probe 重启后的 task_id 缓存、未上报结果和任务恢复尚未决定。
- Server 重启后的任务、Session 和传输恢复尚未决定。
- Tunnel 数据面和 Relay 架构尚未设计。
- OpenAPI 正式资源模型和 WebSocket 事件协议尚未设计。
- 文件任务排队或拒绝、协议错误关闭矩阵和幂等缓存配置仍为 TBD。
- 各目标架构的具体交叉工具链版本、最低内核与 libc 兼容矩阵仍需在真实设备验证时补充。

## 阻塞项

Phase 1A 的设计前置条件已经完成。当前唯一操作性门槛是在真实项目 Git 仓库中形成 Phase 0 baseline commit。

当前附件工作目录没有 `.git` 元数据，因此这里不能真实创建 commit，也不得伪造 commit ID。Codex/开发者开始 Phase 1A 时必须先在真实仓库提交当前 Phase 0 文档基线，然后再写业务代码。

## Git 状态

当前目录没有 .git 元数据，也没有可记录的 Git commit。

baseline commit: pending

## 下一步

1. 在真实 Git 仓库中把本次已确认文档形成 Phase 0 baseline commit。
2. 将 baseline commit ID 写回 PROJECT_STATUS.md 与 HANDOFF.md。
3. 将 Phase 0 标记为完成，将 Phase 1 / Phase 1A 标记为进行中。
4. 只实现 Phase 1A：TCP framing、Header encode/decode、REGISTER、REGISTER_ACK、HEARTBEAT、HEARTBEAT_ACK 和基础断线重连。
5. Phase 1A 必须先在 Linux x86_64 上形成可构建、可运行、可测试闭环，不提前实现 TASK、文件传输、Tunnel 或 API。
