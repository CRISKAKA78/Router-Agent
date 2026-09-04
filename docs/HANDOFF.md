# 项目接管手册

本项目是一套由跨平台 Management Server 和路由器端轻量 Probe 组成的远程运维平台。Phase 0、Phase 1A 与 Phase 1B 已完成；当前已形成注册、心跳、基础重连和单 worker exec 任务闭环，并按用户要求停止在 Phase 1B。

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

Phase 1A - TCP Session：已完成并验证。

Phase 1B - Task and Exec：已完成并验证。未进入 Phase 1C。

baseline commit: bc8d747dfc41a375c31698073005857c238ede51

Phase 1A commit: cd722b6f3fd6cfe5e8ccded256c5295828e4372f

Phase 1B implementation commit: 6ed2434d646938617088f62030f3749a797616c0

## 当前能运行什么

- Go Management Server：接受 Probe 连接，完成 Phase 1A 注册/心跳/会话，并通过独立 Task Service 对在线 device_id 创建 exec、发送 TASK、处理 TASK_ACK/TASK_RESULT 和等待结果。
- C++11 Probe：在线路由 HEARTBEAT_ACK 与 TASK，通过单 worker 执行 `/bin/sh -c`，返回 accepted/rejected ACK 和 success/failed/timeout RESULT。
- exec 支持 cwd、env、独立 stdout/stderr、单流 1 MiB 捕获上限、控制帧大小适配、超时进程组终止与 waitpid 回收。
- exec 运行期间 TCP Reader 与心跳保持工作；同一 socket 的主动写由双方各自串行化，message_id 沿单连接单方向递增。
- 每次重连重新 REGISTER 并获得不同的新 session_id；Phase 1A 自动化回归继续通过。

最小构建、运行和测试命令见 [../README.md](../README.md)，详细验证结果见 [PROJECT_STATUS.md](PROJECT_STATUS.md)。

## 当前不能运行什么

Phase 1C 多任务并发、完整 task_id 幂等、断线任务恢复和结果补报尚未实现。TASK_CANCEL、文件传输、Process Manager、Tunnel、HTTP / WebSocket API、数据库、Web / Windows / 微信小程序、CLI、MCP、AI Agent、VPN、FRP、SSH 和 Telnet 也未实现。

## 实际仓库入口

| 路径 | 用途 |
| --- | --- |
| [../cmd/server/main.go](../cmd/server/main.go) | Management Server 命令入口；当前不提供外部任务 CLI/API |
| [../internal/protocol](../internal/protocol) | Go Header、Frame、消息类型和 TCP stream decoder |
| [../internal/gateway](../internal/gateway) | TCP Session、在线消息路由、协议适配和串行连接写入 |
| [../internal/task](../internal/task) | Server 内存任务状态、ACK/RESULT 关联与等待能力 |
| [../probe/src/task.cpp](../probe/src/task.cpp) | Probe TASK 解析、exec、timeout、输出捕获和结果编码 |
| [../probe/src/client.cpp](../probe/src/client.cpp) | Probe Connection、Message Router、单 Task Worker 和心跳 |
| [../tests/integration](../tests/integration) | 启动真实 Probe 的 Phase 1A / Phase 1B 集成测试 |
| [PROJECT_STATUS.md](PROJECT_STATUS.md) | 当前事实、验证结果和已知问题 |
| [PROTOCOL.md](PROTOCOL.md) | Probe TCP 协议规范性基线 |
| [ROADMAP.md](ROADMAP.md) | 阶段与里程碑状态 |

## Phase 1B 已完成内容

- TASK、TASK_ACK、TASK_RESULT 和 message_id / reply_to / flags 语义。
- Management Server 内部 CreateExec、WaitTaskResult 和基础任务状态记录。
- Server 单连接写锁，主动 TASK 与 HEARTBEAT_ACK / ERROR 共用发送序列。
- Probe HEARTBEAT_ACK / TASK 消息路由、RECEIVED → QUEUED → RUNNING → SUCCESS / FAILED / TIMEOUT 基础状态。
- 单 worker 串行 exec，不阻塞 TCP Reader。
- `/bin/sh -c`、cwd、env、独立 pipe、poll 同时读取、单流 1 MiB 上限与 truncated。
- timeout 的 SIGTERM、200 ms grace、SIGKILL 和 waitpid 回收。
- Linux x86_64 C++ 单测、真实 Probe 集成、Phase 1A 回归、Go race 和 go vet 验证。

## 不可擅自改变的基线

- Probe 主动连接 Server；消息为 20-byte Header + Payload，整数 Big Endian，magic 为 RMP1，version 为 1。
- message_id 从 1 开始，按单连接、单方向递增；TASK_ACK 等直接 JSON Response 设置 RESPONSE 并包含 reply_to；TASK_RESULT 不设置 RESPONSE。
- TASK_ACK accepted=false 是最终拒绝，不再发送 TASK_RESULT；Phase 1 不实现 TASK_CANCEL。
- exec 是一次性非交互命令，不是 SSH Shell；控制 TCP 不承载 SSH、Telnet 或 Web Tunnel 的持续流量。
- Management Server 使用 Go；Probe 使用 C++11 + CMake，保持轻量。
- Phase 1C 及之后能力必须等待明确授权，并继续遵守 [../AGENTS.md](../AGENTS.md)。

## 当前已知问题

- 单 worker 之外的多任务并发、乱序结果与 task_id 幂等属于 Phase 1C，尚未实现。
- TCP 断开时运行中任务当前终止，不补报、不缓存、不跨连接恢复。
- 用户与 Probe 身份认证、TLS、权限、租户、密钥和审计仍为 TBD。
- Probe / Server 重启后的任务和传输恢复仍为 TBD。
- mipsel / ARM / ARM64 toolchain files、最低内核和 libc 兼容矩阵尚未验证。
- 完整 Protocol 错误关闭矩阵仍为 TBD。

## 下一步

等待用户验收 Phase 1B。只有获得明确授权后才能进入 Phase 1C；不得自动实现多任务并发、task_id 幂等、跨连接任务关联或后续阶段能力。
