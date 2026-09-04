# 变更记录

本文件记录：

1. 已形成的用户可见产品行为变化。
2. 对客户端或开发者具有外部意义的协议或 API 契约变化。
3. 重要项目基线或治理规则变化。

本文件不记录普通内部重构、未完成计划、虚构功能或单纯开发过程流水账。

## Unreleased

### Fixed

- Server 成功写出 REGISTER_ACK 后才发布可派发会话，修复注册/重连期间 TASK 抢先发送的问题。
- Server 传输写失败立即关闭并废弃连接，防止部分帧后续写入与 message_id 重用；CreateExec 对发送结果不确定的任务保留记录并返回 task_id 与 ErrDispatchUncertain。
- Probe exec 不再继承控制 socket；fd 的 close-on-exec 设置与 fork 使用统一同步。
- Probe timeout/worker 停止完整执行进程组 TERM、200 ms grace、KILL；后代持有 pipe 时有界排空并设置 truncated，防止 worker 因 EOF 等待挂住。

### Added

- 实现 Phase 1C：Probe 默认 4 workers 并发 exec、乱序结果关联、进程生命周期 task_id 去重；TCP 断开时已接受任务继续执行，新会话补报缓存结果。
- Server 增加内部 ResendTask，使用原 task_id 与规格重发，按 session_id/message_id/task_id 保存每次派发关联。
- 增加实际并发执行、重复任务、参数冲突、容量、断线继续执行和中继丢弃 ACK/RESULT 后补报的自动化覆盖。

- 实现 Phase 1B TASK、TASK_ACK、TASK_RESULT 与 exec 完整闭环，Management Server 可通过内部 Task Service 对在线 Probe 创建任务并等待结果。
- Probe 增加 HEARTBEAT_ACK / TASK 在线消息路由、单 worker 串行任务执行，以及 RECEIVED / QUEUED / RUNNING / SUCCESS / FAILED / TIMEOUT 基础状态流转。
- exec 支持 `/bin/sh -c`、cwd、env、独立 stdout/stderr、1 MiB 单流捕获上限、控制帧大小适配与 timeout 进程组清理。
- 增加 Phase 1B 协议、任务状态、真实进程 exec、超时、心跳不中断、unsupported task 和大输出自动化测试。
- 实现 Phase 1A 的 Go TCP Server 与 C++11 Probe 可运行闭环，包括 REGISTER、REGISTER_ACK、HEARTBEAT、HEARTBEAT_ACK、失联判断和基础退避重连。
- 实现 Protocol v1 20-byte Header、Big Endian 编解码和可处理 TCP 粘包/拆包的流式 framing。
- 增加协议、注册/心跳和真实 Probe 断线重连自动化测试。
- 初始化路由器远程运维平台的项目设计基线。
- 建立 README、AI Agent 开发规则、架构、协议、API、路线图、项目状态、接管手册和架构决策文档。
- 将 v0.2 Word 设计输入中的 TCP 控制协议整理为仓库内可持续维护的 Markdown 基线。
- 建立文档驱动的项目状态与交接治理规则。

### Changed

- 按用户确认冻结 Phase 1C 互操作契约（ADR-015）：重复任务返回 queued/running/完成态 ACK，完成态随后返回原 RESULT；内容冲突使用 ERROR/INVALID_PAYLOAD，保留原任务。
- Server 接受缺 ACK 的已派发任务结果，重复 RESULT 幂等，冲突不覆盖终态；Probe 缓存有界且不淘汰已接受身份与结果，容量满后拒绝新任务。
- 更新阶段授权边界：本次 Phase 1C 独立提交并推送后停止等待验收，Phase 1D 尚未授权。

- Server 每条 Probe 连接的 HEARTBEAT_ACK、TASK 和 ERROR 统一串行写入，并共用单连接 Server 发送方向的 message_id 序列。
- Probe REGISTER capabilities 现在声明 `exec`；TASK_ACK 使用 RESPONSE/reply_to，TASK_RESULT 保持异步 task_id 关联且不设置 RESPONSE。
- Phase 0 形成 Git baseline `bc8d747dfc41a375c31698073005857c238ede51`，Phase 1A 完成 Linux x86_64 首轮验证。
- 明确运行事实、规范性设计、当前状态、接管入口、计划和历史设计输入的事实来源层级。
- 完成 Protocol v1 的 message_id、response correlation、flags、UUID、任务拒绝、文件传输、JSON、boot_id、幂等范围和心跳超时细化。
- 将 Phase 1 拆分为 Phase 1A 至 Phase 1E 可验证里程碑，并增加 Probe 技术栈前置决策门槛。
- 明确 Phase 0 需要用户确认和 Git baseline commit 后才能关闭。
- 确定 Probe Phase 1 技术栈为 C++11 + CMake，首轮 Linux x86_64 验证，后续使用 toolchain files 交叉编译。
- 冻结 Phase 1A REGISTER / REGISTER_ACK / HEARTBEAT / HEARTBEAT_ACK 字段契约、注册失败响应和 10-300 秒心跳范围。
