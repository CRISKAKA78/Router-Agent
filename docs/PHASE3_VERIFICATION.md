# Phase 3 File and Tool Repository 验收记录

日期：2026-09-05。启动基线 HEAD/main/origin/main：`b9982f5d2765546d23e09c28977c27ceb510a368`，起始工作区干净。启动草案后，用户明确确认 ADR-019 R1～R6，并补充 artifact_id 为独立不透明 UUID、全 Repository 唯一，tool_id/asset_id/artifact_id 不因归档、去重或存储路径变化重用。ADR-019 转 Accepted 后实现。

**Phase 3 实现与自动化验收通过，Phase 1/2 全量回归通过。**没有改动 Probe 代码或 Protocol v1 wire，没有进入 Phase 4 或外部 Adapter。以独立 Phase 3 commit 提交推送后停止等待用户验收。

## 模型、兼容性与闭环

FileService 管理稳定资产 UUID、名称、Size/SHA256、不可变 blob 和归档；ToolService 管理工具 UUID、版本标签和多产物。每次新版本的产物使用全仓库唯一 artifact_id，产物固定引用 asset_id、Linux platform、mode 和兼容约束。相同内容可共享 blob，身份和引用不合并；相同版本相同规格重复发布返回旧身份，不同规格冲突。

兼容规则采用字段间 AND、集合内 OR、capabilities 必须包含全部要求，输出 compatible/incompatible/unknown 和逐项原因。arch 仅 amd64/x86_64、arm64/aarch64 等价；libc 小写，model/kernel 精确。明确 any 或不限制才允许跳过该字段；缺少受限信息为 unknown，仅 compatible 且 online/file 可以投放，多匹配必须指定产物。

management.Service 查询 Device → 固定版本/产物/资产/Session → 校验内容 → 调用原 CreateUpload → 保存 task_id/transfer_id 与管理端关联。Gateway 仅执行 Session 前置检查；源文件打开后再次检查 Expected Size/SHA256。非空 TaskID 与 ErrDispatchUncertain 保留，不自动重试为新身份。ResendTask 不重新传输或发布文件。

下载使用原 CreateDownload 写 Repository 自有暂存。CompleteDownload 要求 FileSnapshot.Committed + Released，再核对摘要并登记资产；TaskSnapshot 单独返回。done ACK 丢失后的 failed 不抹掉完整文件，不伪造成功；同任务重复/并发完成导入返回同一 asset_id。

## 持久化范围与失败边界

本地 `repository-dir`，默认 `./data/repository`；单进程目录锁，Linux flock / Windows LockFileEx。`metadata/catalog.json` 使用 schema_version=1；blob 按 SHA-256 保存，临时数据在 staging。目录移动只改变运行时绝对路径，不改变业务身份。

完整 blob 先发布，JSON 写同目录临时文件、Sync/Close 后由 Linux rename / Windows MoveFileExW 替换。失败保留旧目录/内存状态；未引用 blob 可遗留并报告。启动拒绝格式/schema/重复身份/引用错误、缺失或大小不符 blob；使用时校验摘要。锁文件保留初始化标记，缺失 catalog 不会静默清空仅包含工具的旧仓库。

Repository 字节、资产/工具/版本/产物与归档持久化。Task、transfer、Device、Session、Operation 和下载任务导入关联不持久化。归档保留文件，不自动 GC；备份须停服并整体复制目录，不承诺在线备份、迁移或任意文件系统突然断电后的恢复。

## 验收映射

| 行为/风险 | 测试证据 |
| --- | --- |
| 同内容独立资产、共享 blob、版本身份与归档保留、重开 | TestRepositoryPersistenceIdentityAndArchive |
| 版本/产物嵌套快照副本、归一化、并发资产 UUID | TestRepositoryCopiesConcurrentIDsAndNormalization、TestRepositoryInputValidationAndQueryIsolation |
| 跨工具/跨版本 artifact_id 不重用，目录移动身份不变 | 上述用例、TestRepositoryMovedDirectoryAndMissingCatalog |
| 元数据发布失败保留旧内存/磁盘、未引用 blob 报告、内容损坏拒绝 | TestRepositoryAtomicCatalogFailureAndCorruptContent |
| 取消/读失败/空文件、下载期望不符、暂存清理 | TestRepositoryImportFailuresAndEmptyFile |
| 错误 schema、重复 JSON key、重复全局 ID、引用错误、缺失 blob/catalog | TestRepositoryCatalogRejectsInvalidState、TestRepositoryMovedDirectoryAndMissingCatalog |
| Linux/Windows 同进程与跨进程排他锁、关闭释放 | TestRepositoryDirectoryLock（使用子进程验证） |
| 归档等待已准入使用完成 | TestRepositoryArchiveWaitsForAdmittedUse |
| 兼容三态、别名、端序、ARM 不推断、libc/model/kernel/capabilities | TestCompatibilityMatrix，多组子用例 |
| 真实 TCP REGISTER 完整约束与重注册可选字段清空 | TestRepositoryDeviceDeclarationTCP；保留 Phase 2 原有 Device 测试 |
| 未知/歧义/离线/缺 file/归档拒绝、不确定派发关联 | TestCompatibilitySelectionAndAdmission、TestDeployBindsAssetSessionAndUncertainIdentity |
| Session 选择后/文件准备后/writer 等待后替换，不派发旧资料 | TestRepositoryUploadSessionAndContentPreconditions、TestRepositoryDispatchChecksSessionAfterWriterWait |
| 原传输源摘要前置条件、本地字段不进 TASK | TestRepositoryUploadSessionAndContentPreconditions |
| 下载未提交/未释放/内容变化拒绝，失败清理，重复并发导入 | TestDownloadImportCommitDistinctFromResultAndIdempotent、TestDownloadImportRejectsIncompleteAndMutation |
| 程序组合生命周期与 Repository 重开、Device 不恢复 | TestManagementServerRepositoryLifecycle |
| 真实 Probe 工具投放、0755、字节一致、不自动执行、单独 exec、下载导入/去重、归档后重发 | TestRepositoryRealProbeToolDeploymentAndDownload |
| 真实 Probe 缺 libc/kernel/model 的兼容判断、200000-byte 二进制资产投放 | TestRepositoryRealProbeCompatibilityAndBinaryAsset |
| 真实下载 done ACK 丢失但保留资产/failed、中断上传不发布部分文件、重连旧 ID 重发 | TestRepositoryRealProbeCommitAckLossAndInterruptedDeployment |

保留 Phase 1 全部协议、非法输入、幂等、并发、FIFO、控制优先、源文件变更、提交/确认丢失及资源收敛测试，以及 Phase 2 生命周期/时间/历史/并发/真实 Probe 测试。未以 mock 代替全量真实 Probe 回归。

## 环境与最终结果

Windows amd64 原生 Go 1.25.5；Linux x86_64 使用既有 WSL 隔离镜像（源码绑定 `/work`），Alpine、GCC 15.2.0、CMake 4.2.3、Go 1.26.3。镜像仅为构建测试环境，不是项目运行依赖。

| 验证 | 实际结果 |
| --- | --- |
| 全新配置的 Phase 3 Release C++11 构建 + CTest | 通过，3/3，4.70 秒 |
| Linux Server go build、go vet | 通过 |
| Linux 全量 Go + Release 真实 Probe，Phase 1/2/3 | 全包通过；集成 152.691 秒 |
| C++ ASan/UBSan/LSan CTest，detect_leaks=1 | 通过，3/3，5.91 秒，无报告 |
| C++ TSan CTest，halt_on_error=1 | 通过，3/3，6.78 秒，无报告 |
| Linux 全量 Go race + TSan 真实 Probe，Phase 1/2/3 | 全包通过；集成 159.799 秒，无 race/TSan 报告 |
| Windows 原生 go test、Server go build、go vet | 通过；Linux Probe 用例在 Windows 跳过，在上述 Linux 全量回归实际执行 |
| git diff --check | 通过 |

最终 Linux 命令（仓库根目录）：

~~~sh
cmake -S probe -B build/phase3-probe -DCMAKE_BUILD_TYPE=Release
cmake --build build/phase3-probe --parallel 2
ctest --test-dir build/phase3-probe --output-on-failure
go build -o build/server/router-server ./cmd/server
RMP_PROBE_BIN=/work/build/phase3-probe/router-probe go test ./... -count=1
go vet ./...
cmake --build build/phase1e-asan --parallel 2
ASAN_OPTIONS=detect_leaks=1 ctest --test-dir build/phase1e-asan --output-on-failure
cmake --build build/phase1e-tsan --parallel 2
TSAN_OPTIONS=halt_on_error=1 ctest --test-dir build/phase1e-tsan --output-on-failure
RMP_PROBE_BIN=/work/build/phase1e-tsan/router-probe TSAN_OPTIONS=halt_on_error=1 \
  go test -race ./... -count=1
~~~

Sanitizer 配置沿用 PHASE1_VERIFICATION 中的 ASan/UBSan 与 TSan 构建目录并重新构建；Probe 源码没有变化。Windows 命令：

~~~powershell
go test ./... -count=1
go build -o build/server/router-server.exe ./cmd/server
go vet ./...
~~~

## 交付与限制

没有改变 Accepted ADR-009～018，ADR-019 已由用户确认。Repository 目录/版本/兼容/身份/归档均按确认方案实现。元数据整体快照和无自动清理面向小规模；磁盘与记录数随使用增长。兼容判断不是二进制执行保证，嵌入式 arch/libc/最低内核实机矩阵未完成。生产安全与完整重启恢复继续为既有后续主题。

独立提交标题 `feat: complete Phase 3 file and tool repository`，实际 SHA 由 Git 查询并在交付回复报告；不在提交自身填入自引用 SHA。完成推送后停止等待用户验收。具备申请进入 Phase 4 的代码与验证基础，但尚未开始 Phase 4，Tunnel 仍需另行授权和正式设计。
