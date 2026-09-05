# 项目接管手册

项目由跨平台 Go Management Server 与轻量 C++11 Probe 组成。Phase 0 与 Phase 1A～1E 已完成实现和验收；当前停止等待用户验收，不进入 Phase 2。

## 接管顺序与事实来源

严格依次阅读 AGENTS.md、本文件、PROJECT_STATUS.md、ARCHITECTURE.md、ROADMAP.md，以及当前任务相关 PROTOCOL.md、API.md、DECISIONS.md；随后核对 main 代码、Git 状态与实际构建测试。当前事实见 PROJECT_STATUS，完整 Phase 1 验收证据见 [PHASE1_VERIFICATION.md](PHASE1_VERIFICATION.md)。

## 提交导航

- baseline：`bc8d747dfc41a375c31698073005857c238ede51`
- Phase 1A：`cd722b6f3fd6cfe5e8ccded256c5295828e4372f`
- Phase 1B：`6ed2434d646938617088f62030f3749a797616c0`
- R1-R4：`59e65b4`；历史问题与操作事故保留在 PHASE1AB_REVIEW.md。
- Phase 1C：`71e5d1791224a4d952f468626e507c41fae9e502`
- Phase 1D：`f1d9fa08d047f4f46a8bc27119565a2f6d217ecc`
- Phase 1E 起点：`d61054543500fbf5a62c94cd2afe492637230b85`，启动时等于 origin/main，工作区干净。
- Phase 1E：本次独立 `fix: complete Phase 1E verification` 提交，使用 `git log --format=fuller --grep="Phase 1E"` 获取实际 SHA；不在提交自身填入自引用 SHA。

## 运行闭环与入口

| 入口 | 实际职责 |
| --- | --- |
| cmd/server/main.go | TCP Server 程序；尚无外部任务 CLI/API |
| internal/protocol | framing、Unicode 基础校验 |
| internal/gateway | REGISTER/Session/心跳、消息路由、优先级串行 writer、内部任务与文件传输适配 |
| internal/task | 不可变任务规格、派发关联、ACK/RESULT 幂等、状态与等待 |
| internal/filetransfer | 流式源/接收器、FILE wire、下载提交事实与传输 Service；终态释放文件接收邮箱 |
| probe/src/client.cpp | 连接、注册、Reader/控制帧、心跳关联与缓存结果发送 |
| probe/src/task_manager.cpp | 进程级 worker pool、任务身份、有界缓存与结果重放 |
| probe/src/task.cpp | exec、cwd/env、stdout/stderr、timeout 与进程组清理 |
| probe/src/file_manager.cpp | Session 文件 FIFO、文件 I/O worker/deadline watcher、完整性与发布 |
| tests/integration/phase1a_test.go～phase1e_test.go | 真实 Server/Probe、故障中继、并发/重复/重连、文件与非法协议验收 |

Server 内部入口为 CreateExec、CreateUpload、CreateDownload、ResendTask、WaitTaskResult、TaskSnapshot、FileSnapshot，契约见 API.md。task_id 处理业务幂等，message_id/reply_to 只在单连接方向关联。每次重连重新注册并产生新 session_id。

Probe 默认四个 exec workers；TCP 断线后 exec 继续执行，完成结果新会话补报。文件共用一个 active 和八个 FIFO 等待槽位；断线时 active/queued 文件全部终止，不 resume。重复 ID 不重新执行或重新发布。

文件以完整 size/SHA-256 校验及同目录发布为边界。upload 发布后 ACK 丢失仍 success；download 本地提交后 done 丢失可以保留 Committed=true 和最终 failed。ready 必须省略 sha256_ok、done=true、failed=false。规范见 ADR-015～017 与 PROTOCOL.md。

## 验证与限制

完整 Release C++/Go/真实 Probe、Go race/vet、C++ ASan/UBSan/LSan、TSan 单测与真实 Probe 全量集成、Windows Server 原生适用测试均通过；精确命令和时长见验收记录。通用构建命令见 README。

- 已接受身份和结果保留，不淘汰；Probe 默认 128 身份、8 MiB 计费预算。Server 保留内存任务/派发历史，暂无历史清理或持久化。
- Probe 每连接最多 1024 未确认心跳；满后按既有重连流程处理，旧关联不在在线会话中被淘汰。
- 控制优先只作用于帧边界，不抢占已写出的 TCP 字节、对端缓冲或不可中断内核 I/O；文件系统能力与恢复边界见 PROJECT_STATUS。
- boot_id 不是 Probe 进程实例证明；Probe/Server 进程重启恢复、认证/TLS/权限和嵌入式 CPU/libc/内核实机矩阵仍待后续阶段。
- Phase 2 未开始。用户另行授权后才开展 Device Management；不自行新增数据库、API、Tunnel、UI、MCP 或 AI 能力。
