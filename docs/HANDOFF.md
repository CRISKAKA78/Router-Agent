# 项目接管手册

项目由跨平台 Go Management Server 与轻量 C++11 Probe 组成。Phase 0、Phase 1A、Phase 1B、Phase 1C 已实现并验证；当前完成 Phase 1C 交付后停止等待验收，不进入 Phase 1D。

## 新接管者先做什么

严格依次阅读 AGENTS.md、本文件、PROJECT_STATUS.md、ARCHITECTURE.md、ROADMAP.md，以及当前任务相关 PROTOCOL.md、API.md、DECISIONS.md。然后核对真实 Git 状态、代码、构建与测试；不得依赖旧会话推断进度。

## 当前交付与提交

- baseline commit: `bc8d747dfc41a375c31698073005857c238ede51`
- Phase 1A commit: `cd722b6f3fd6cfe5e8ccded256c5295828e4372f`
- Phase 1B implementation commit: `6ed2434d646938617088f62030f3749a797616c0`
- Phase 1A/B R1-R4 修复和启动检查记录：`59e65b4`，与本次 Phase 1C 实现分开提交。
- Phase 1C implementation commit: `71e5d1791224a4d952f468626e507c41fae9e502`，已推送 GitHub main，等待验收。

用户已在启动检查后明确确认重复 TASK、内容冲突和跨连接结果补报建议；正式契约位于 PROTOCOL.md 末尾与 Accepted ADR-015。ADR-009/010 未被静默改写。

## 当前运行闭环

Server 可通过内部 CreateExec 创建 exec，通过 ResendTask 重发同一业务任务、通过 WaitTaskResult 等待结果。Gateway 适配每次传输，Task Service 保存派发关联与业务状态，缺 ACK 的已派发任务可以接收重连补报，重复结果不会覆盖或回退终态。

Probe 在进程生命周期内维护任务表、默认 4 workers 和有界缓存；TCP 断开不停止排队/运行中的 exec，新会话补报缓存结果。同 ID 同内容只返回已有状态/结果，同 ID 不同内容返回 ERROR 并保留原任务。缓存不淘汰已接受身份，满后拒绝新任务。

exec 支持 cwd/env、独立 stdout/stderr、有界输出和 timeout 进程组回收；Reader/连接线程继续处理心跳与控制帧。每次重新 REGISTER 产生新 session_id，message_id 在单连接单方向重新编号。

## 实际入口

| 路径 | 用途 |
| --- | --- |
| [../cmd/server/main.go](../cmd/server/main.go) | Server 入口，尚无外部任务 CLI/API |
| [../internal/gateway](../internal/gateway) | 注册、会话、消息路由、串行发送、CreateExec/ResendTask 传输适配 |
| [../internal/task](../internal/task) | 业务任务、派发关联、ACK/RESULT 幂等、状态与等待 |
| [../probe/src/client.cpp](../probe/src/client.cpp) | 连接/重连、消息路由、心跳、ACK 与缓存结果发送 |
| [../probe/src/task_manager.cpp](../probe/src/task_manager.cpp) | 进程级 worker pool、任务身份、缓存、去重与容量 |
| [../probe/src/task.cpp](../probe/src/task.cpp) | TASK 解析、exec、输出捕获、timeout 与 RESULT 编码 |
| [../probe/tests/task_manager_tests.cpp](../probe/tests/task_manager_tests.cpp) | 并发登记、queued/running/完成态去重、容量与重放 |
| [../tests/integration/phase1c_test.go](../tests/integration/phase1c_test.go) | 真实并发/乱序、重连、冲突及 ACK/RESULT 丢失补报 |

完整构建/回归结果见 [PROJECT_STATUS.md](PROJECT_STATUS.md)，通用运行命令见 [../README.md](../README.md)。Linux CTest 2/2、全量 Go 测试、真实 Probe 集成、Go race、vet 和 Windows 原生构建/单测/vet 均通过。

## 接管边界

- 尚无文件传输、Tunnel、HTTP/WebSocket、数据库、UI、MCP、AI Agent、SSH/Telnet 通道，不实现 TASK_CANCEL。
- 去重覆盖同一 Probe 进程，进程重启恢复与持久化仍 TBD；不要把 boot_id 当成 Probe 进程实例 ID。
- 默认缓存容量、计费预算及降低协商帧上限时的结果延后补报限制见 PROTOCOL.md / PROJECT_STATUS.md。
- R1-R4 已修复；剩余已知限制和历史操作事故保留在 [PHASE1AB_REVIEW.md](PHASE1AB_REVIEW.md)，不因本次 1C 删除历史证据。
- Phase 1C 已提交推送，停止等待验收。后续阶段必须另获明确授权。
