# 项目接管手册

项目由跨平台Go Management Server与轻量C++11 Probe组成。Phase 0～4已形成闭环；本轮从Phase 4首版 `f92d73a0d6003835993967848f1f5fe009a0df89` 的干净main修正端口隔离、独立撤销、域名和轻量默认值，保留自研TCP Maintenance。当前验证与交付状态见PROJECT_STATUS和[PHASE4_VERIFICATION](PHASE4_VERIFICATION.md)。修正独立提交推送后停止，不进入Phase 5。

## Phase 4 接管入口

先读Accepted ADR-021及superseding ADR-022、[PHASE4_DESIGN](PHASE4_DESIGN.md)、PROTOCOL的Phase 4章节与API内部Maintenance契约。`management.Server.Maintenance()`提供Create/Get/List/CloseMaintenance；一次创建三个固定入口，默认240分钟。data listener与控制TCP独立；C++ TunnelManager不包含HTTP/SSH/Telnet实现或第三方Tunnel程序。

重要文件：`internal/tunnel`（租约/端口/配对/Relay）、`internal/gateway/tunnel.go`（可撤销Session绑定及发送准入）、`probe/src/tunnel.cpp`（固定目标/worker/取消/half-close）、`tests/integration/phase4_test.go`（真实协议/负载/故障验收）。运行配置集中在`cmd/server`的`-tunnel-*`及Probe的`--tunnel-connections`。

默认loopback绑定；远程部署配置data IP或域名、维护入口host及bind。域名每次Create在Server解析，Probe仍收到IP。默认Probe/Server每维护8条流，较大浏览器并发须显式同步提高双方限额；Probe上限64。idle默认24小时，正常维护由默认4小时租期控制。创建接口仍为内部Go Service。

Released表示Server本地资源释放，不等待控制writer或Probe ACK。释放后的端口默认隔离24小时，ReusableAfter表示最早可复用时刻；隔离记录独立于关闭历史。默认200端口池在隔离窗口内最多支持66次三入口创建。超窗或Server重启后不保证旧地址永久隔离；永久隔离需部署不重叠的池/地址。原始TCP入口没有外部客户端Maintenance身份，token只配对Probe data流。

放弃FRP/xfrpc方向的归档摘要见DECISIONS.md ADR-020；当前实现、构建与验收均以已提交仓库为准。

## 接管顺序与事实来源

严格依次阅读 AGENTS.md、本文件、PROJECT_STATUS.md、ARCHITECTURE.md、ROADMAP.md，以及当前任务相关 PROTOCOL.md、API.md、DECISIONS.md；随后核对 main 代码、Git 状态与实际构建测试。当前事实见 PROJECT_STATUS，完整 Phase 1 验收证据见 [PHASE1_VERIFICATION.md](PHASE1_VERIFICATION.md)。

## 提交导航

- Phase 4首版：`f92d73a0d6003835993967848f1f5fe009a0df89`；本轮独立修正标题 `fix: isolate Phase 4 ports and decouple maintenance revocation`，实际SHA和推送状态以Git记录为准。

- baseline：`bc8d747dfc41a375c31698073005857c238ede51`
- Phase 1A：`cd722b6f3fd6cfe5e8ccded256c5295828e4372f`
- Phase 1B：`6ed2434d646938617088f62030f3749a797616c0`
- R1-R4：`59e65b4`；历史问题与操作事故保留在 PHASE1AB_REVIEW.md。
- Phase 1C：`71e5d1791224a4d952f468626e507c41fae9e502`
- Phase 1D：`f1d9fa08d047f4f46a8bc27119565a2f6d217ecc`
- Phase 1E 起点：`d61054543500fbf5a62c94cd2afe492637230b85`，启动时等于 origin/main，工作区干净。
- Phase 1E / Phase 2 起点：`3de7924353d05261e6b01faa87cbca00a0a0d0a5`，Phase 2 启动时等于 origin/main，工作区干净。
- Phase 2 / Phase 3 起点：`b9982f5d2765546d23e09c28977c27ceb510a368`，本次启动时 HEAD、main 与 fetch 后 origin/main 一致，工作区干净。
- Phase 3：独立 `feat: complete Phase 3 file and tool repository` commit；实际 SHA 通过 `git log --format=fuller --grep="Phase 3"` 查询。

## Phase 3 接管要点

用户已明确确认 [ADR-019](DECISIONS.md#adr-019-phase-3-file-and-tool-repository) R1～R6，并补充 artifact_id 为整个 Repository 全局唯一的独立 UUID；tool_id、asset_id、artifact_id 不因归档、去重或存储路径变化而重用。FileService/ToolService 共用本地 JSON 目录与 blob，management.Service 组织工具投放和下载导入；接口见 API.md。

当前真实 Probe 不上报 libc/kernel/model；缺失受限字段为 unknown，投放须 compatible。兼容检查绑定当前 Session，发送准入前复核；复用 CreateUpload/CreateDownload，保留非空 task_id + ErrDispatchUncertain、旧 ID 重发和下载 Committed 与最终 RESULT 分离。CompleteDownload 在 Committed + Released 后显式导入，重复返回原资产身份。

默认仓库 `./data/repository`，用 `-repository-dir` 配置；启动记录绝对路径。单写者目录锁、JSON schema_version=1、不可变 blob；停服后整体备份，不支持在线备份、多 Server 共享写入、物理 GC 或任务重启恢复。归档不回收空间，崩溃遗留只报告，不自动删除。代码本地路径细节和锁顺序见 ARCHITECTURE。

## 运行闭环与入口

| 入口 | 实际职责 |
| --- | --- |
| cmd/server/main.go | 组合 Management Server 与持久 Repository；尚无外部任务 CLI/API |
| internal/management | File/Tool/Device 与既有传输的编排、Operation、下载导入 |
| internal/repository | FileService、ToolService、兼容规则、本地持久目录和稳定身份 |
| internal/protocol | framing、Unicode 基础校验 |
| internal/gateway | REGISTER/Session/心跳、消息路由、优先级串行 writer、内部任务与文件传输适配 |
| internal/device | 设备 Inventory、注册资料、Session 业务状态与有界历史、内部查询 |
| internal/task | 不可变任务规格、派发关联、ACK/RESULT 幂等、状态与等待 |
| internal/filetransfer | 流式源/接收器、FILE wire、下载提交事实与传输 Service；终态释放文件接收邮箱 |
| probe/src/client.cpp | 连接、注册、Reader/控制帧、心跳关联与缓存结果发送 |
| probe/src/task_manager.cpp | 进程级 worker pool、任务身份、有界缓存与结果重放 |
| probe/src/task.cpp | exec、cwd/env、stdout/stderr、timeout 与进程组清理 |
| probe/src/file_manager.cpp | Session 文件 FIFO、文件 I/O worker/deadline watcher、完整性与发布 |
| tests/integration/phase1a_test.go～phase1e_test.go | 真实 Server/Probe、故障中继、并发/重复/重连、文件与非法协议验收 |
| tests/integration/phase3_test.go | 真实工具投放/执行分离、字节/权限、幂等、兼容准入与下载确认丢失 |

Server 内部入口为 CreateExec、CreateUpload、CreateDownload、ResendTask、WaitTaskResult、TaskSnapshot、FileSnapshot，契约见 API.md。task_id 处理业务幂等，message_id/reply_to 只在单连接方向关联。每次重连重新注册并产生新 session_id。

设备查询通过 `Server.Devices()` 返回的 `device.Query` 使用 List/Get/Sessions。Gateway 仅保留传输连接映射，注册发布、活动与关闭同步更新 Device Service；不能依赖 Events 重建设备状态。LastOnlineAt 为最近 Session 发布时间，LastOfflineAt 仅在整体 online → offline 更新，replaced 不更新。当前/最近 Session、快照副本、历史顺序和字段契约见 API.md，设计原因见 Accepted ADR-018。

Probe 默认四个 exec workers；TCP 断线后 exec 继续执行，完成结果新会话补报。文件共用一个 active 和八个 FIFO 等待槽位；断线时 active/queued 文件全部终止，不 resume。重复 ID 不重新执行或重新发布。

文件以完整 size/SHA-256 校验及同目录发布为边界。upload 发布后 ACK 丢失仍 success；download 本地提交后 done 丢失可以保留 Committed=true 和最终 failed。ready 必须省略 sha256_ok、done=true、failed=false。规范见 ADR-015～017 与 PROTOCOL.md。

## 验证与限制

完整 Release C++/Go/真实 Probe、Go race/vet、C++ ASan/UBSan/LSan、TSan 单测与真实 Probe 全量集成、Windows Server 原生适用测试均通过；精确命令和时长见验收记录。通用构建命令见 README。

Phase 3历史普通全量集成152.691秒，Go race + TSan Probe全量集成159.799秒，各CTest 3/3。Phase 4最终验证以PHASE4_VERIFICATION为准，旧阶段停止授权已由ADR-021更新。

- 已接受身份和结果保留，不淘汰；Probe 默认 128 身份、8 MiB 计费预算。Server 保留内存任务/派发历史，暂无历史清理或持久化。
- Probe 每连接最多 1024 未确认心跳；满后按既有重连流程处理，旧关联不在在线会话中被淘汰。
- 控制优先只作用于帧边界，不抢占已写出的 TCP 字节、对端缓冲或不可中断内核 I/O；文件系统能力与恢复边界见 PROJECT_STATUS。
- boot_id 不是 Probe 进程实例证明；Probe/Server 进程重启恢复、认证/TLS/权限和嵌入式 CPU/libc/内核实机矩阵仍待后续阶段。
- Device Inventory 只在 Server 进程内保存；默认当前 Session + 最近 64 条已结束历史，容量可配置，查询暴露截断计数。离线设备保留；历史淘汰不删除 Task/File 记录。Server 重启清空，没有数据库、心跳时序或审计重放。
- Phase 2 交付时 Phase 1/2 全量回归通过，证据见 [PHASE2_VERIFICATION.md](PHASE2_VERIFICATION.md)：最终普通集成 149.921 秒，Go race + TSan Probe 集成 156.221 秒；C++ 各 CTest、Linux/Windows 构建和 vet 均通过。此处为历史交付证据，Phase 3 完成前须对最终实现重新全量验证。
