# 项目状态

最后更新时间：2026-09-05

## 当前阶段

Phase 0、Phase 1A/B/C 已交付。Phase 1D 文件闭环已实现，最终全量回归通过，独立实现提交已推送 GitHub main，现停止等待验收。P1-P5 与 FILE_ACK 的 sha256_ok 字段约束已由用户确认，见 Accepted ADR-016/017。Phase 1E 未开始，完成本阶段独立提交推送后停止等待验收。

- baseline commit: `bc8d747dfc41a375c31698073005857c238ede51`
- Phase 1A commit: `cd722b6f3fd6cfe5e8ccded256c5295828e4372f`
- Phase 1B commit: `6ed2434d646938617088f62030f3749a797616c0`
- R1-R4 修复 commit: `59e65b4`
- Phase 1C commit: `71e5d1791224a4d952f468626e507c41fae9e502`
- Phase 1D 启动 main：`5030322b58fcbb07cae2f3256a71eb9a750a479a`（已核对 origin/main 相同）；本阶段 implementation commit: `f1d9fa08d047f4f46a8bc27119565a2f6d217ecc`（已推送 origin/main）。

## Phase 1D 当前能力与验证

- 内部 CreateUpload/CreateDownload -> TASK/ACK -> FILE_BEGIN/ACK ready -> binary CHUNK -> END/ACK done -> RESULT 已形成真实 Server/Probe 闭环。没有外部 HTTP/CLI 入口。
- File Service 持有流式源/接收器、会话传输关联及本地提交事实；Gateway 只适配协议和串行发送；Task Service 保留不可变文件参数与跨连接派发记录。
- Probe 独立文件 worker 与 deadline watcher；共享进程 TaskManager 身份/结果缓存，文件默认一个 active + 八个 FIFO 等待槽位。exec 仍使用四个 workers。file_queue_capacity 属于 ClientConfig，不新增外部控制接口。
- 接收数据邮箱最多 16 帧，文件流缓冲按 chunk_size 分配；SHA-256 增量处理，源文件预读摘要后在发送时再次校验。文件字节不进入任务缓存，不使用 Base64。
- 接收端同目录独占临时文件，校验完整 size/SHA-256 后发布；不创建父目录。overwrite=false 通过硬链接发布保证不覆盖并发新建目标，overwrite=true 使用 rename 替换，失败保留原目标；Probe 应用 upload.mode。
- 两端帧边界优先控制消息；每块重新参与优先级仲裁。TCP 发送缓冲目标设为 64 KiB，限制内核预先排入的文件字节；内核可调整实际容量。已发送的字节不可被抢占。
- 断线终止本连接所有未完成文件任务，临时文件清理，缓存失败结果并在重连补报。已发布 upload 保留 success；download done 丢失时保留 Server 本地 Committed=true 与完整文件，Probe RESULT 可为 failed。相同任务不能重新传输或重复发布。
- 自动化已覆盖空/单字节/块边界/大二进制双向往返、1024-byte JSON 上限与独立 CHUNK 上限、mode、重复任务、跨连接结果重放、混合 FIFO、队满拒绝、运行/排队冲突、中断、size/SHA-256/offset/flags 失败、timeout、最终 ACK 丢失及慢链路控制优先。

最终全量验证（2026-09-05）全部通过：

- Linux：`cmake --build build/phase1d-probe --parallel 2`；`ctest --test-dir build/phase1d-probe --output-on-failure`：3/3，4.70 秒。
- Linux：`go build -o build/server/router-server ./cmd/server`。
- Linux：`RMP_PROBE_BIN=/work/build/phase1d-probe/router-probe go test ./... -count=1`：全部通过，真实 Probe 集成 66.292 秒。
- Linux：同一 Probe 的 `go test -race ./... -count=1`：全部通过，真实集成 67.881 秒；`go vet ./...` 通过。
- Windows：原生 `go test ./... -count=1`、`go vet ./...`、Server 构建通过；Linux Probe 的运行由上述真实 Linux 集成覆盖。
- `git diff --check` 通过。原 Phase 1A/B/C 测试保留；旧“upload 不支持”测试的任务类型换为仍不支持的 start_process，以保留原拒绝行为测试意图。

Go race 验证 Go 并发；C++ 由 worker、任务竞争与真实 Probe 集成覆盖。此次交付不代表已完成 Phase 1E 的整个 Phase 1 验收。

## 当前可用能力

- Go Server 与 C++11 Probe：TCP framing、注册、心跳、失联判断、重新注册和新 session_id；Server 注册 ACK 完整写出后才发布会话，写失败废弃连接。
- Probe 默认 4 workers，并发执行一次性 `/bin/sh -c`；支持 cwd/env、独立 stdout/stderr、单流 1 MiB 捕获上限、协商帧大小适配、timeout 进程组 TERM/KILL 和 waitpid 回收。
- Probe TaskManager 属于进程生命周期，任务不持有 socket；排队与运行中的 exec 跨 TCP 断线继续执行，缓存结果在新会话补报，包括此前写成功的结果。
- 同 task_id 的 queued/running/完成态重复请求返回已有状态或结果；内容冲突返回 ERROR/INVALID_PAYLOAD 并保留原任务。已接受任务的身份与结果不淘汰，保证同一 Probe 进程内不重复执行副作用。
- 默认最多 128 个已接受任务，8 MiB 身份/结果计费预算；新任务为结果预留协商帧上限，完成后释放未使用预留，容量不足时最终拒绝新任务。预算不是进程 RSS 上限，详见 PROTOCOL.md。
- Server 内部 CreateExec、ResendTask、WaitTaskResult、TaskSnapshot；每次重发使用原 task_id/规格，记录 session_id/message_id/task_id。发送结果不确定时保留记录并返回 ErrDispatchUncertain。
- Server 按 device_id/task_id 接受缺 ACK 的已派发任务补报；重复结果幂等，冲突不覆盖原终态，rejected 不可被 RESULT 改写。迟到 ACK 不回退状态，完成态 ACK 不替代 RESULT。
- fd 创建/close-on-exec 与 fork 同步，exec 不继承控制 socket；并发任务期间连接线程继续处理控制消息。

## 限制与遗留问题

- 不实现 resume、逐块 ACK、sliding window、并行文件流或专用文件数据连接。必须重传时创建新 task_id/transfer_id。
- 文件发布依赖目标文件系统的 rename/hard link 能力；无覆盖发布不支持 hard link 时失败。控制优先不能抢占已写出的 TCP 字节或不可中断内核 I/O；进程崩溃后的临时文件清理属于未实现的恢复范围。
- 最终 done ACK 丢失可造成下载文件已提交而任务 failed；调用者应同时查看 FileSnapshot.Committed 和 TASK_RESULT。

- Probe / Server 进程重启后的任务持久化、恢复和去重仍为 TBD。boot_id 不能证明 Probe 进程连续性。
- 如新会话协商上限小于原缓存 RESULT，该结果保留并延后补报；恢复足够大的帧上限后可重放，不重新执行任务。重复 TASK 的该长度错误见 PROTOCOL.md。
- 未实现 TASK_CANCEL、Tunnel、Process Manager、HTTP/WebSocket API、数据库、UI、CLI、MCP、AI Agent、VPN、FRP、SSH/Telnet 通道。
- 主动 setsid 脱离原进程组的后代不受组 KILL 覆盖；其 pipe 有界排空。不可中断内核等待的直接子进程回收仍依赖内核。
- 认证/TLS/权限、安全设计、跨 CPU/最低内核/libc 实机矩阵仍待后续阶段；更严格 JSON Unicode/类型校验及完整错误关闭矩阵保留在审查报告和专项文档中。

## 下一步

Phase 1D 实现、全量回归、文档同步与独立提交推送均已完成，现停止等待用户验收。具备申请进入 Phase 1E 的实现与回归基础，但 Phase 1E 未获授权、未开始。历史 R1-R4 审查与操作事故见 PHASE1AB_REVIEW.md。
