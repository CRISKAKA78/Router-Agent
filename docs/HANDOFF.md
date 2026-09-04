# 项目接管手册

本项目是一套由跨平台 Management Server 和路由器端轻量 Probe 组成的远程运维平台。Phase 0 与 Phase 1A 已完成；当前已形成注册、心跳与基础断线重连的真实闭环，并按用户要求停止在 Phase 1A。

## 新接管者先做什么

严格按以下顺序阅读：

1. [../AGENTS.md](../AGENTS.md)
2. [PROJECT_STATUS.md](PROJECT_STATUS.md)
3. [ARCHITECTURE.md](ARCHITECTURE.md)
4. [ROADMAP.md](ROADMAP.md)
5. 与当前任务有关的 [PROTOCOL.md](PROTOCOL.md)、[API.md](API.md) 和 [DECISIONS.md](DECISIONS.md)

随后检查实际文件、Git 状态、构建和测试结果。不得依赖旧聊天或 AI 记忆替代仓库事实。

## 当前阶段

Phase 1 - Probe and Server TCP Control Link：进行中。

Phase 1A - TCP Session：已完成并验证。未进入 Phase 1B。

baseline commit: bc8d747dfc41a375c31698073005857c238ede51

Phase 1A 的代码与交付文档当前尚未另行提交。

## 当前能运行什么

- Go Management Server：接受 Probe TCP 连接，完成 REGISTER 校验、REGISTER_ACK、会话登记、HEARTBEAT_ACK 和 3 倍心跳失联处理。
- C++11 Probe：主动连接、注册、进入 ONLINE、发送心跳、校验 reply_to、检测 Server 失联并按 1/2/5/10/30 秒退避重连。
- 每次重连重新 REGISTER，并获得不同的新 session_id。
- TCP Decoder 能处理半包 Header、半包 Payload 和一次读取多帧。

最小构建、运行和测试命令见 [../README.md](../README.md)。

## 当前不能运行什么

TASK、exec、文件传输、Process Manager、Tunnel、HTTP / WebSocket API、数据库、Web / Windows / 微信小程序、CLI、MCP、AI Agent、VPN、FRP、SSH 和 Telnet 均未实现。不要创建这些能力的空 package 或占位实现。

## 实际仓库入口

| 路径 | 用途 |
| --- | --- |
| [../cmd/server/main.go](../cmd/server/main.go) | Management Server 命令入口 |
| [../internal/protocol](../internal/protocol) | Go Header、Frame 和 TCP stream decoder |
| [../internal/gateway](../internal/gateway) | REGISTER、HEARTBEAT 和 Session TCP Gateway |
| [../probe](../probe) | C++11 Probe、CMake 与 Probe 单元测试 |
| [../tests/integration](../tests/integration) | 启动真实 Probe 进程的断线重连集成测试 |
| [PROJECT_STATUS.md](PROJECT_STATUS.md) | 当前事实、验证结果和已知问题 |
| [PROTOCOL.md](PROTOCOL.md) | Probe TCP 协议规范性基线 |
| [ROADMAP.md](ROADMAP.md) | 阶段与里程碑状态 |

## Phase 1A 已完成内容

- 20-byte Header encode/decode 和 Big Endian。
- TCP stream framing 与 payload 上限。
- REGISTER 完整必选字段和范围校验。
- REGISTER_ACK success=true / false。
- HEARTBEAT / HEARTBEAT_ACK 和 message_id / reply_to。
- heartbeat_interval 10-300 秒与双方 3 倍失联阈值。
- 基础退避重连和重连后 session_id 更新。
- Linux x86_64 Server / Probe 构建、单元测试、竞争检测、真实进程集成测试和手工闭环。

详细命令与结果见 [PROJECT_STATUS.md](PROJECT_STATUS.md)。

## 不可擅自改变的基线

- Probe 主动连接 Server；消息为 20-byte Header + Payload，整数 Big Endian，magic 为 RMP1，version 为 1。
- message_id 从 1 开始，按单连接、单方向递增；直接 JSON Response 设置 RESPONSE 并包含 reply_to。
- REGISTER 失败仍返回 REGISTER_ACK success=false，随后关闭连接，Probe 不得进入 ONLINE。
- heartbeat_interval 来自 REGISTER_ACK，双方失联阈值为 3 倍；每次重新 REGISTER 生成新 session_id。
- 控制 TCP 不承载 SSH、Telnet 或 Web Tunnel 的持续流量。
- Management Server 使用 Go；Probe 使用 C++11 + CMake，保持轻量。
- Phase 1A 之后的能力必须等待明确授权，并继续遵守 [../AGENTS.md](../AGENTS.md)。

## 当前已知问题

- 用户与 Probe 身份认证、TLS、权限、租户、密钥和审计仍为 TBD。
- Probe / Server 重启后的任务和传输恢复仍为 TBD。
- 数据库、Tunnel 数据面、Relay、OpenAPI 和 WebSocket 协议仍为 TBD。
- mipsel / ARM / ARM64 的 toolchain files、最低内核和 libc 兼容矩阵尚未验证。
- 完整 Protocol 错误关闭矩阵仍为 TBD。

## 下一步

等待用户验收 Phase 1A。只有获得明确授权后才能进入 Phase 1B；不得自动实现 TASK、TASK_ACK、TASK_RESULT 或 exec。
