# 项目状态

最后更新时间：2026-09-05

## 当前阶段

Phase 0、Phase 1A～1E、Phase 2 和 Phase 3 File and Tool Repository 均已完成实现与自动化验收。Phase 3 按用户确认的 Accepted ADR-019 R1～R6 及身份补充实现；Phase 1/2 全量回归通过。独立 Phase 3 commit 推送后停止等待用户验收，不进入 Phase 4 或外部 API/UI/MCP/AI Agent。

Phase 3 启动时 HEAD、main 与 fetch 后 origin/main 均为 `b9982f5d2765546d23e09c28977c27ceb510a368`，工作区干净。启动草案形成后，用户明确确认并要求 artifact_id 使用全 Repository 唯一 UUID，tool_id/asset_id/artifact_id 不因归档、去重或存储路径变化重用。全部实现和必要文档纳入一个独立 Phase 3 commit，不在该提交自身伪造自引用 SHA。

## 提交导航

- baseline：`bc8d747dfc41a375c31698073005857c238ede51`。
- Phase 1A：`cd722b6f3fd6cfe5e8ccded256c5295828e4372f`；Phase 1B：`6ed2434d646938617088f62030f3749a797616c0`。
- R1-R4：`59e65b4`；Phase 1C：`71e5d1791224a4d952f468626e507c41fae9e502`。
- Phase 1D：`f1d9fa08d047f4f46a8bc27119565a2f6d217ecc`；Phase 1E：`3de7924353d05261e6b01faa87cbca00a0a0d0a5`。
- Phase 2 / Phase 3 起点：`b9982f5d2765546d23e09c28977c27ceb510a368`。
- Phase 3：独立 `feat: complete Phase 3 file and tool repository` commit；实际 SHA 通过 Git 历史查询。

## 当前可验证能力

- `internal/repository` 的 FileService/ToolService：资产流式导入、查询/归档；工具/版本/多产物发布、查询与归档。tool_id、asset_id、artifact_id 为稳定 UUID，artifact_id 全仓库唯一；资产字节按 SHA-256 去重，不合并业务身份。版本不可变、标签不透明，重复发布相同规格返回原产物身份。
- 兼容性基于 Phase 2 注册快照，字段间 AND、集合内 OR；arch 有限别名、libc 小写匹配、model/kernel 精确匹配、capabilities 集合包含。缺失受限字段 unknown；只有 compatible、online 且声明 file 才能投放。多个兼容产物须显式选择，不自动 latest/升级/执行工具。
- `internal/management.Service` 组合设备、Repository 与原 Task/File 能力：工具投放、资产上传、设备下载暂存/显式导入、Operation 查询与既有快照/等待/重发。返回 task_id/transfer_id 与版本/资产关联；不确定派发保留 ID，不自动建替代任务。
- `-repository-dir` 默认 `./data/repository`，启动记录绝对路径。单进程目录锁、schema_version=1 JSON 元数据、不可变 blob；完整内容先发布、元数据后发布，失败保留旧内存/磁盘目录状态。Linux/Windows 重开、跨进程锁、目录移动与损坏拒绝测试通过。
- 下载须 Committed + Released 后重新校验并导入，重复/并发导入返回同一 asset_id；done ACK 丢失时完整资产可与 failed Task 并存。Released 只表示本地文件 worker 已释放句柄，不替代 TASK_RESULT。
- Gateway 只增加可选 Session 派发前置条件；File Transfer 只增加可选源内容期望与 Released 事实。Repository 逻辑没有进入 Gateway/Probe，没有新增 wire 字段或第二套传输。
- Phase 2 Device Inventory 保持稳定 device_id、REGISTER 全量资料快照、online/offline、当前/最近 Session 与默认最近 64 条已结束历史。最新成功发布 Session 胜出；LastOnlineAt 为发布时间，LastOfflineAt 只在整体下线更新，replaced 不更新。旧回调与历史淘汰不影响 Task/File。
- Phase 1 保持 TCP 注册/心跳/退避重连、默认四 worker exec、cwd/env/双流/timeout、进程内 task_id 幂等和跨 TCP 结果补报；文件 upload/download、单 active + 默认八 FIFO 等待槽位、size/SHA-256、提交和确认分离、帧边界控制优先均通过全量回归。

## 验证结果

完整映射、环境与复现命令见 [PHASE3_VERIFICATION.md](PHASE3_VERIFICATION.md)。历史 Phase 1/2 证据保留于各自验收文件。

| 验证 | 最终结果 |
| --- | --- |
| Linux Release C++11 构建/CTest | 通过，3/3，4.70 秒 |
| Linux Server 构建、全量 Go 与 Phase 1/2/3 真实 Probe 集成、vet | 通过；集成 152.691 秒 |
| C++ ASan/UBSan/LSan CTest | 通过，3/3，5.91 秒，无报告 |
| C++ TSan CTest | 通过，3/3，6.78 秒，无报告 |
| Linux Go race + TSan 真实 Probe 全量回归 | 通过；集成 159.799 秒，无 race/TSan 报告 |
| Windows 原生 Server 构建、全量 Go、vet | 通过；包含 Repository 平台适配和跨进程锁，Linux Probe 用例在 Linux 实际执行 |

## 已知限制

- Repository 仅本地文件系统单进程写入，元数据整体 JSON 快照面向小规模；无外部数据库、多 Server 共享写入、在线备份/迁移或完整断电恢复保证。当前平台适配为 Linux/Windows。
- 归档不回收磁盘，不取消已派发任务；无物理 GC、自动旧版本淘汰或设备端卸载。崩溃遗留暂存/未引用 blob 只报告，磁盘随导入增长。停服后一并备份元数据和 blobs。
- 真实 Probe 当前不发送 libc/kernel/model；受这些字段限制的工具为 unknown 并拒绝投放。兼容结论仅针对声明条件，不能证明二进制可执行；不推断 ARM ABI、libc 版本或 kernel 范围。嵌入式 CPU/libc/最低内核实机矩阵仍未覆盖。
- 仅 Repository 数据跨 Server 重启保留；Device、Session、Task、transfer、Operation 和下载 task_id 导入关联均为进程内状态。重启不恢复在线、不重发任务；boot_id 不证明 Probe 进程连续性。
- 保留 Phase 1 资源边界：Probe 默认最多 128 已接受身份、8 MiB 计费预算且不淘汰；每连接最多 1024 未确认心跳；文件接收邮箱最多 16 帧。Server 任务/设备总数无自动淘汰，控制优先不能抢占已发送 TCP 字节或不可中断 I/O。
- 无覆盖文件发布依赖 hard link；主动脱离 exec 进程组的后代不受组 KILL 覆盖。认证/TLS/权限、完整审计、重启任务恢复、resume、TASK_CANCEL、Tunnel、HTTP/WebSocket、UI、CLI 操作入口、MCP 和 AI Agent 未实现。

## 下一步

独立 Phase 3 commit 推送 GitHub 后停止等待用户验收。代码与自动化验收已具备申请进入 Phase 4 的基础；Phase 4 仍未开始，须用户明确验收与另行授权，Tunnel 数据面/Relay/生命周期等长期设计须先形成决策。
