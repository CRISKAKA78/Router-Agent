# 项目状态

最后更新时间：2026-09-05

## 当前阶段

Phase 0、Phase 1A、Phase 1B 与 Phase 1C 已完成实现和验证。Phase 1C 的三项互操作补充已由用户明确确认并写入 PROTOCOL.md / ADR-015。当前准备提交、推送本次交付，然后停止等待验收；Phase 1D 未开始。

- baseline commit: `bc8d747dfc41a375c31698073005857c238ede51`
- Phase 1A commit: `cd722b6f3fd6cfe5e8ccded256c5295828e4372f`
- Phase 1B implementation commit: `6ed2434d646938617088f62030f3749a797616c0`
- Phase 1A/B R1-R4 修复与启动检查记录 commit: `59e65b4`
- Phase 1C implementation commit: pending（实现和完整本地回归已完成，待创建本阶段提交）

## 当前可用能力

- Go Server 与 C++11 Probe：TCP framing、注册、心跳、失联判断、重新注册和新 session_id；Server 注册 ACK 完整写出后才发布会话，写失败废弃连接。
- Probe 默认 4 workers，并发执行一次性 `/bin/sh -c`；支持 cwd/env、独立 stdout/stderr、单流 1 MiB 捕获上限、协商帧大小适配、timeout 进程组 TERM/KILL 和 waitpid 回收。
- Probe TaskManager 属于进程生命周期，任务不持有 socket；排队与运行中的 exec 跨 TCP 断线继续执行，缓存结果在新会话补报，包括此前写成功的结果。
- 同 task_id 的 queued/running/完成态重复请求返回已有状态或结果；内容冲突返回 ERROR/INVALID_PAYLOAD 并保留原任务。已接受任务的身份与结果不淘汰，保证同一 Probe 进程内不重复执行副作用。
- 默认最多 128 个已接受任务，8 MiB 身份/结果计费预算；新任务为结果预留协商帧上限，完成后释放未使用预留，容量不足时最终拒绝新任务。预算不是进程 RSS 上限，详见 PROTOCOL.md。
- Server 内部 CreateExec、ResendTask、WaitTaskResult、TaskSnapshot；每次重发使用原 task_id/规格，记录 session_id/message_id/task_id。发送结果不确定时保留记录并返回 ErrDispatchUncertain。
- Server 按 device_id/task_id 接受缺 ACK 的已派发任务补报；重复结果幂等，冲突不覆盖原终态，rejected 不可被 RESULT 改写。迟到 ACK 不回退状态，完成态 ACK 不替代 RESULT。
- fd 创建/close-on-exec 与 fork 同步，exec 不继承控制 socket；并发任务期间连接线程继续处理控制消息。

## 本次完整构建与回归

Linux x86_64：复用本地隔离 Alpine 构建镜像；工作区构建目录 `build/phase1c-probe`。

全部通过：

- `cmake -S probe -B build/phase1c-probe -DCMAKE_BUILD_TYPE=Release`
- `cmake --build build/phase1c-probe --parallel 2`
- `ctest --test-dir build/phase1c-probe --output-on-failure`：2/2 passed，4.70 秒。
- `go build -o build/server/router-server ./cmd/server`
- `RMP_PROBE_BIN=/work/build/phase1c-probe/router-probe go test ./... -count=1`：全部通过，真实 Probe 集成 21.958 秒。
- `RMP_PROBE_BIN=/work/build/phase1c-probe/router-probe go test -race ./... -count=1`：全部通过，真实 Probe 集成 23.026 秒。
- `go vet ./...`：通过。
- Windows 原生 Server 构建、`go test ./... -count=1`、`go vet ./...`：通过；Windows 不运行 Linux Probe 集成，该部分由上面的 Linux 回归覆盖。

Phase 1C 新增覆盖：三个不同任务在全部释放执行屏障之前同时运行，反序完成与结果关联；并发重发；同一 Probe 多次重连；queued/running/完成态去重；20 个线程同时投递同 ID；执行字段冲突及默认值/扩展字段归一；容量和结果预留；完成结果不淘汰；缓存帧不因较小协商值被改写；新旧 session 同值 message_id；缺 ACK、重复/冲突 RESULT 与 rejected 保护；真实中继分别丢弃 ACK/RESULT 并断开 TCP，验证重连补报且副作用计数仍为一次。原 Phase 1A/1B 及 R1-R4 自动化回归均保留并通过。Go race 验证 Go 代码；C++ 并发由实际 worker/登记竞争测试验证。

## 限制与遗留问题

- Probe / Server 进程重启后的任务持久化、恢复和去重仍为 TBD。boot_id 不能证明 Probe 进程连续性。
- 如新会话协商上限小于原缓存 RESULT，该结果保留并延后补报；恢复足够大的帧上限后可重放，不重新执行任务。重复 TASK 的该长度错误见 PROTOCOL.md。
- 未实现 TASK_CANCEL、文件传输、Tunnel、Process Manager、HTTP/WebSocket API、数据库、UI、CLI、MCP、AI Agent、VPN、FRP、SSH/Telnet 通道。
- 主动 setsid 脱离原进程组的后代不受组 KILL 覆盖；其 pipe 有界排空。不可中断内核等待的直接子进程回收仍依赖内核。
- 认证/TLS/权限、安全设计、跨 CPU/最低内核/libc 实机矩阵仍待后续阶段；更严格 JSON Unicode/类型校验及完整错误关闭矩阵保留在审查报告和专项文档中。

## 下一步

独立提交并推送 Phase 1C 后停止，等待用户验收。不得自动进入 Phase 1D。历史审查、R1-R4 修复和先前操作事故记录见 [PHASE1AB_REVIEW.md](PHASE1AB_REVIEW.md)。
