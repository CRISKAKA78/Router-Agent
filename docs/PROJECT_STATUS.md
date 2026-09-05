# 项目状态

最后更新时间：2026-09-05

## 当前阶段

Phase 0 与 Phase 1A～1E 已交付；Phase 2 Device Management 已按 Accepted ADR-018 完成实现与验收，Phase 1 全量回归通过。范围为设备清单、状态与能力、Session 管理、设备与最小历史查询，不实现外部 API 或 Phase 3。独立 Phase 2 commit 交付后停止等待用户验收。

Phase 2 启动基线：main 与 fetch 后 origin/main 均为 `3de7924353d05261e6b01faa87cbca00a0a0d0a5`，起始工作区干净。启动草案 D1～D4 及 LastOnlineAt/LastOfflineAt 时间语义均已获用户明确确认。当前验证结果见 [PHASE2_VERIFICATION.md](PHASE2_VERIFICATION.md)。

- baseline：`bc8d747dfc41a375c31698073005857c238ede51`
- Phase 1A：`cd722b6f3fd6cfe5e8ccded256c5295828e4372f`
- Phase 1B：`6ed2434d646938617088f62030f3749a797616c0`
- R1-R4 修复：`59e65b4`
- Phase 1C：`71e5d1791224a4d952f468626e507c41fae9e502`
- Phase 1D：`f1d9fa08d047f4f46a8bc27119565a2f6d217ecc`
- Phase 1E 起点：main / origin/main 同为 `d61054543500fbf5a62c94cd2afe492637230b85`，启动工作区干净。
- Phase 1E 交付提交：`3de7924353d05261e6b01faa87cbca00a0a0d0a5`。
- Phase 2 交付提交：本次独立 `feat: complete Phase 2 device management` commit；通过 Git 历史查询实际 SHA，避免文档自引用。

## 当前可验证能力

- 正式 `internal/device.Service` / Inventory，内部 `Server.Devices().List/Get/Sessions` 查询独立快照。按稳定 device_id 保留最近成功 REGISTER 全部基础字段、capabilities、当前/最近 Session、设备 online/offline、首次/最近时间和计数；可选字段省略后清空，未知能力保留为声明。
- ACK 完整写出后发布 Session；最新成功发布者替换旧 Session，旧活动/清理不影响新会话。LastOnlineAt 为最近发布时间；LastOfflineAt 只在整体 online → offline 更新，replaced 保持 online。设备状态同步更新，不依赖可丢 Events。
- 默认每设备保留当前 Session 加最近 64 个已结束 Session，可配置容量；按结束顺序查询，包含注册快照、last_seen、结束时间/原因及已淘汰数量。离线设备不删除，Device 历史淘汰不影响 Task/File。
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

## Phase 2 验证

新增 13 个测试函数及子用例，覆盖 Device Service、Gateway 边界与真实 Probe；保留全部 Phase 1 用例。最终 Linux 普通全量集成 149.921 秒，Go race + TSan Probe 全量集成 156.221 秒，均通过。C++ Release/ASan/UBSan/LSan/TSan CTest 均 3/3 通过；Linux Server 构建/vet 与 Windows 原生构建/Go/vet 通过。命令和验收映射见 PHASE2_VERIFICATION.md。

## 资源与已知边界

- Device Inventory 仅在 Server 进程内保留，跨 TCP 重连不丢失，Server 重启清空。Session 历史有界，设备数量无自动淘汰，总内存随已见设备数增长；不保存心跳时序、完整审计或可重放事件流，不实现数据库持久化。
- Probe 默认最多 128 个已接受任务、8 MiB 身份/RESULT 计费预算；不淘汰已接受身份，满后拒绝新任务。预算不等同于 RSS 上限。Server 内存任务/派发历史随任务数保留；没有持久化或历史清理能力。
- Probe 每连接最多保存 1024 个未确认 HEARTBEAT；容量耗尽时在发送下一心跳前关闭连接并按既有规则重连，不淘汰旧 reply_to，不引入 ACK 超时协议。普通失联判断仍由任意合法消息刷新 last_seen。
- 接收邮箱满时会产生 TCP 背压；控制优先不能抢占已写出 TCP 字节、对端已接收缓冲或不可中断内核 I/O。文件无覆盖发布要求同目录 hard link 支持，失败不退化为覆盖。
- 主动 setsid 脱离原进程组的后代不受组 KILL 覆盖；其 pipe 有界排空。不可中断内核等待的直接子进程回收仍依赖内核。
- 新会话 max_control_payload 小于缓存 RESULT 时延后补报，保留原结果，不重新执行。
- Probe/Server 进程重启恢复、跨进程幂等、崩溃临时文件清理与持久化仍 TBD；boot_id 不能证明 Probe 进程连续性。
- 完整错误关闭矩阵、认证/TLS/权限、跨 CPU/最低内核/libc 实机矩阵仍是已记录的后续主题。
- 未实现 resume、TASK_CANCEL、Process Manager、Tunnel、HTTP/WebSocket API、数据库、UI、CLI、MCP、AI Agent、VPN、FRP、SSH/Telnet 通道。

## 下一步

本次独立 Phase 2 commit 提交并推送后停止等待用户验收。已具备申请进入 Phase 3 的代码与验证基础，但 Phase 3 继续未开始，须用户验收与授权；File/Tool Repository 的存储、版本和兼容性模型需下一阶段另行确认。历史 R1-R4 与操作事故保留于 PHASE1AB_REVIEW.md。
