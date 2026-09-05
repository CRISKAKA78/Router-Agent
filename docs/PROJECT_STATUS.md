# 项目状态

最后更新时间：2026-09-05

## 当前阶段

Phase 0 与 Phase 1A～1D 已交付；Phase 1E Verification 已通过完整验收，Phase 1 完成。用户本轮授权对现有 Protocol v1 / Server / Probe 收口验收、修复明确 bug、独立提交并推送，然后停止。Phase 2 未开始。

- baseline：`bc8d747dfc41a375c31698073005857c238ede51`
- Phase 1A：`cd722b6f3fd6cfe5e8ccded256c5295828e4372f`
- Phase 1B：`6ed2434d646938617088f62030f3749a797616c0`
- R1-R4 修复：`59e65b4`
- Phase 1C：`71e5d1791224a4d952f468626e507c41fae9e502`
- Phase 1D：`f1d9fa08d047f4f46a8bc27119565a2f6d217ecc`
- Phase 1E 起点：main / origin/main 同为 `d61054543500fbf5a62c94cd2afe492637230b85`，启动工作区干净。
- Phase 1E 交付提交：本次独立 Verification commit（提交形成后通过 Git 历史查询，避免文档自引用 SHA）。

## 当前可验证能力

- Go Management Server 与 C++11 Probe：20-byte framing、REGISTER/ACK、心跳、3 倍失联判断、退避重连与新 session_id；注册 ACK 完整写出后才发布可派发 Session。
- 默认 4 workers 并发 exec，支持 cwd/env、stdout/stderr、输出截断、timeout 进程组 TERM/KILL 与 waitpid 回收；Reader 不执行命令。
- 同一 Probe 进程内 task_id 幂等；已接受 exec 跨 TCP 断线继续执行，新会话补报缓存 RESULT。重复任务查询已有状态，内容冲突保留原任务；Server 按 device/session/message/task 关联、幂等接收结果，不回退终态。
- 内部 CreateUpload/CreateDownload、TASK/ACK、FILE_BEGIN/ACK/CHUNK/END、RESULT 双向文件闭环；size/SHA-256 校验后同目录临时文件发布，支持 upload.mode 和本地覆盖策略。
- 文件单 active + 默认 8 个 FIFO 等待槽位；断线时未完成 active/queued 全部失败收敛，保留原 task/transfer 身份，重传使用新 ID。ready 省略 sha256_ok，done=true，failed=false。
- 文件和控制发送在帧边界仲裁；16 MiB 慢链路上传与下载期间心跳和 exec 均在文件完成前处理。文件接收邮箱最多 16 帧，Server worker 结束后释放邮箱中的残留文件块。
- 下载 FileSnapshot.Committed 独立保留本地完整文件提交事实；done ACK 丢失时可与 Probe 的 failed RESULT 并存。

## Phase 1E 修复与验证

具体缺陷、Protocol v1 的 14 项验收映射、验证命令与结果见 [PHASE1_VERIFICATION.md](PHASE1_VERIFICATION.md)。

修复：Server Unicode / load1 类型校验；Probe 未知扩展大整数兼容；协商缩小上限时立即复核缓冲 Header；未确认心跳记录容量；Server 文件终态邮箱释放；Server 入站 message_id 耗尽保护。没有改变 Accepted ADR 或新增 wire 消息。

已通过 Linux CTest、全量 Go 与真实 Probe 回归、C++ ASan/UBSan/LSan、TSan CTest 和真实 Probe 全量集成，以及 Windows 原生 Server 构建/Go/vet。最终 Go race 全部通过（真实 Probe 集成 145.712 秒），补充源文件变更与正向发布用例普通/race 均通过。

## 资源与已知边界

- Probe 默认最多 128 个已接受任务、8 MiB 身份/RESULT 计费预算；不淘汰已接受身份，满后拒绝新任务。预算不等同于 RSS 上限。Server 内存任务/派发历史随任务数保留；没有持久化或历史清理能力。
- Probe 每连接最多保存 1024 个未确认 HEARTBEAT；容量耗尽时在发送下一心跳前关闭连接并按既有规则重连，不淘汰旧 reply_to，不引入 ACK 超时协议。普通失联判断仍由任意合法消息刷新 last_seen。
- 接收邮箱满时会产生 TCP 背压；控制优先不能抢占已写出 TCP 字节、对端已接收缓冲或不可中断内核 I/O。文件无覆盖发布要求同目录 hard link 支持，失败不退化为覆盖。
- 主动 setsid 脱离原进程组的后代不受组 KILL 覆盖；其 pipe 有界排空。不可中断内核等待的直接子进程回收仍依赖内核。
- 新会话 max_control_payload 小于缓存 RESULT 时延后补报，保留原结果，不重新执行。
- Probe/Server 进程重启恢复、跨进程幂等、崩溃临时文件清理与持久化仍 TBD；boot_id 不能证明 Probe 进程连续性。
- 完整错误关闭矩阵、认证/TLS/权限、跨 CPU/最低内核/libc 实机矩阵仍是已记录的后续主题。
- 未实现 resume、TASK_CANCEL、Process Manager、Tunnel、HTTP/WebSocket API、数据库、UI、CLI、MCP、AI Agent、VPN、FRP、SSH/Telnet 通道。

## 下一步

Phase 1E 验收通过，以本次独立 Verification commit 提交并推送后停止等待验收。具备进入 Phase 2 的代码基础不等于获得 Phase 2 开发授权。历史 R1-R4 与操作事故保留于 PHASE1AB_REVIEW.md。
