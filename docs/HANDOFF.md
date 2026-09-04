# 项目接管手册

本项目是一套由跨平台 Management Server 和路由器端轻量 Probe 组成的远程运维平台。Probe 主动建立 TCP 控制长连接，Server 通过统一 Service 向 Web、微信小程序、Windows UI、CLI、MCP 和 AI Agent 提供能力。Phase 0 设计复核已经获得用户确认，业务代码尚未开始；当前只等待在真实 Git 仓库形成 baseline commit，随后进入 Phase 1A。

## 新接管者先做什么

按以下顺序阅读：

1. [../AGENTS.md](../AGENTS.md)
2. [PROJECT_STATUS.md](PROJECT_STATUS.md)
3. [ARCHITECTURE.md](ARCHITECTURE.md)
4. [ROADMAP.md](ROADMAP.md)
5. 与当前任务有关的 [PROTOCOL.md](PROTOCOL.md)、[API.md](API.md) 和 [DECISIONS.md](DECISIONS.md)

然后检查实际仓库文件和 Git 状态。事实来源层级以 AGENTS.md 为准，不要依赖旧聊天、AI 记忆或本手册之外的假设。

## 当前阶段

Phase 0 - Repository & Documentation Initialization：设计已确认。当前等待真实 Git baseline commit；完成 baseline 后已获授权进入 Phase 1A。

baseline commit: pending

## 已经完成

- 保留 v0.2 Word 文档作为原始设计输入。
- 建立项目入口、开发规则和变更记录。
- 建立架构、协议、API、路线图、事实快照、接管手册和决策记录。
- 将 Word 中的 TCP 控制协议整理为 Markdown 基线。
- 完成 Protocol v1 互操作细化，并把已解决的 Review 项移入规范正文。
- 增加 ADR-008 至 ADR-014。
- 将 Phase 1 拆分为 Phase 1A 至 Phase 1E；Phase 1A 技术决策门槛已经完成。
- Probe 技术栈确定为 C++11 + CMake，首轮 Linux x86_64 验证。
- REGISTER / HEARTBEAT 互操作字段契约已经冻结。
- 把仍未明确的安全、恢复、Tunnel、存储和 API 主题保留为 TBD。

## 当前能运行什么

没有可执行的业务程序，也没有实际远程运维功能。当前没有构建、运行或业务测试命令。

## 当前不能运行什么

TCP Server、Probe、注册、心跳、任务、文件传输、Tunnel、HTTP API、WebSocket、数据库、Web UI、微信小程序、Windows UI、CLI、MCP 和 AI Agent 均未实现。

## 仓库入口

| 文件 | 用途 |
| --- | --- |
| [../README.md](../README.md) | 项目总入口和文档导航 |
| [../AGENTS.md](../AGENTS.md) | 强制开发与交付规则 |
| [PROJECT_STATUS.md](PROJECT_STATUS.md) | 当前事实快照 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 系统边界、分层和模块职责 |
| [PROTOCOL.md](PROTOCOL.md) | Probe TCP 控制协议 |
| [API.md](API.md) | HTTP 与 WebSocket API 基线 |
| [ROADMAP.md](ROADMAP.md) | 阶段和里程碑状态 |
| [DECISIONS.md](DECISIONS.md) | 已确认决策和影响 |
| [../CHANGELOG.md](../CHANGELOG.md) | 已形成的用户可见变化 |
| [../路由器探针_TCP长连接控制协议设计_v0.2.docx](../路由器探针_TCP长连接控制协议设计_v0.2.docx) | v0.2 原始设计输入 |

Phase 0 完成后，仓库 Markdown 是持续维护的当前设计基线。Word v0.2 只作为 Phase 0 原始输入和历史参考，不覆盖后续正式写入 Markdown 或 ADR 的变化。

## 不可擅自改变的基线

- Probe 主动使用 TCP 长连接连接 Management Server。
- 消息使用 20 字节固定包头，整数为 Big Endian，magic 为 RMP1，主版本为 1。
- 控制载荷使用 JSON UTF-8；FILE_CHUNK 使用原始二进制，不使用 Base64。
- TASK_ACK 与 TASK_RESULT 分离。
- message_id、task_id 和 transfer_id 的职责不同，task_id 是业务幂等键。
- message_id 从 1 开始，按单连接和单发送方向独立递增；JSON Response 使用 reply_to。
- FILE_CHUNK 设置 BINARY，JSON 消息不设置 BINARY，MORE 在 Protocol v1 中为 0。
- transfer_id 使用 canonical UUID string 和 RFC 4122 16-byte wire format。
- TASK_ACK accepted=false 是最终拒绝，不再发送 TASK_RESULT；Phase 1 不实现 TASK_CANCEL。
- 同一 Probe 进程生命周期内，TCP 重连后不得重复执行已经接受的同一 task_id。
- 一个 Probe 控制连接同时最多存在一个 active file transfer，控制消息优先于 FILE_CHUNK。
- SSH、Telnet、Web Tunnel 数据流使用独立连接，不进入控制 TCP。
- Management Server 优先使用 Go，并保持跨平台和基础部署简单。
- API First，所有前端、MCP 和 AI 共用核心 Service。
- Probe 保持轻量，不承担具体 AI 故障诊断逻辑。
- 状态与交接必须维护在仓库文档中。

详细决定及其原因见 [DECISIONS.md](DECISIONS.md)。

## 当前已知问题

- 用户认证、Probe 身份认证、TLS、权限、租户、密钥轮换和审计仍为 TBD。
- Probe 和 Server 重启后的任务与传输恢复仍为 TBD。
- 数据库存储、Tunnel 数据面、Relay、OpenAPI 正式资源模型和 WebSocket 事件协议仍为 TBD。
- 文件任务排队或拒绝、协议错误关闭矩阵和幂等缓存配置仍为 TBD。
- 各目标架构的具体交叉工具链版本、最低内核与 libc 兼容矩阵后续通过真实设备验证补充。
- 当前目录没有 .git 元数据；baseline commit: pending。

## Phase 1A 前置门槛

设计门槛已经完成：

- Probe：C++11。
- 构建：CMake。
- 第一开发与验证平台：Linux x86_64。
- 后续交叉编译：CMake toolchain files -> mipsel / ARM / ARM64。
- REGISTER / REGISTER_ACK / HEARTBEAT / HEARTBEAT_ACK：完整字段与失败响应契约见 PROTOCOL.md。
- Management Server：Go，保持 ADR-003。

当前只剩一个操作性门槛：在真实 Git 仓库先提交 Phase 0 baseline。当前附件目录无 `.git`，所以 baseline commit 仍为 pending。

## 下一步最优先任务

在真实仓库执行以下顺序：

1. 提交当前文档作为 Phase 0 baseline，并把 commit ID 写回 PROJECT_STATUS / HANDOFF。
2. 将 ROADMAP 中 Phase 0 标记完成，Phase 1 与 Phase 1A 标记进行中。
3. 开始 Phase 1A，只实现 TCP framing、Header、REGISTER、REGISTER_ACK、HEARTBEAT、HEARTBEAT_ACK 和基础断线重连。
4. 先完成 Linux x86_64 Server + Probe 的闭环和测试，不提前实现 TASK、文件、Tunnel 或 API。

## 每次后续任务结束

运行相关验证，并按 [../AGENTS.md](../AGENTS.md) 同步 PROJECT_STATUS、HANDOFF 和 ROADMAP。协议、架构、API 或重要决策变化时同时更新对应专项文档；未同步这些文档的代码任务不算完整交付。
