# Phase 2 Device Management 验收记录

日期：2026-09-05。起点：main 与 origin/main 均为 `3de7924353d05261e6b01faa87cbca00a0a0d0a5`，启动工作区干净。用户确认 ADR-018 D1～D4 及时间语义后开始实现。**Phase 2 验收通过，Device Management 完成；Phase 1 全量回归通过。**

## 模型与范围

新增 `internal/device` 内存 Service，以稳定 device_id 管理 REGISTER 基础字段和 capabilities、online/offline、当前/最近 Session、首次/最近时间与计数。每条 Session 保存注册快照、开始/活动/结束时间及原因。`Server.Devices()` 提供 List/Get/Sessions 内部查询，每次返回完整副本。

注册 ACK 完整写出后发布当前 Session；最新发布者替换旧会话，设备保持 online，旧回调不影响新会话。LastOnlineAt 为最近一次成功发布时间；LastOfflineAt 只在整体 online → offline 时更新。默认保留当前 Session 加最近 64 个已结束 Session，历史按结束操作顺序淘汰，计数可见；离线设备与首次时间保留。没有持久化，Server 重启清空。

Gateway 管理 socket/writer、连接注册和帧路由，按同一锁顺序同步更新 Device。设备业务元数据只在 Device Service 中拥有；Task/File 的规格、派发记录、幂等与提交事实仍由原 Service 管理。没有改动 Probe、wire 消息、Protocol v1 字段或 ADR-009～017。

## 新增测试映射

| 验收内容 | 证据 |
| --- | --- |
| 完整注册资料、未知能力、可选字段清空、离线查询 | `TestLifecycleAndTimeSemantics`、`TestDeviceGatewayLifecycle` |
| LastOnlineAt/LastOfflineAt、replaced 不离线、重连与旧回调 | 同上，使用确定时间的 Service 测试与实际 TCP；`TestDeviceRealProbeReplacement` 使用两个真实 Probe |
| 失败注册/ACK 写失败不创建 Inventory、ACK 完整写出才发布 | `TestDeviceInvalidRegisterAndInvalidActivity`、`TestDeviceRegisterAckPublicationAndFailure`，保留 Phase 1 `TestRegisterPublishesOnlyAfterAck` |
| 心跳、TASK_ACK/RESULT 活动与非法消息不刷新 | `TestDeviceGatewayLifecycle`、`TestDeviceTaskTrafficUpdatesActivity`、`TestDeviceInvalidRegisterAndInvalidActivity` |
| 3 倍心跳 deadline、超时离线、writer 失败与不确定派发 | `TestDeviceTimeoutAndWriterFailure` 观察实际 60 秒 deadline，再提前触发底层 pipe 超时；不伪称实际等待了 60 秒 |
| 并发 Session 替换及 Server Close | `TestDeviceConcurrentReplacementAndServerClose`，12 条并发真实 TCP 注册；`TestConcurrentQueriesAndLifecycle`，8 goroutine 共 800 次发布/活动/结束与查询 |
| 有界历史、首次时间/累计数保留、无持久化 | `TestHistoryBoundAndInventoryLifetime` 覆盖容量 1/3/64、100 次注册、最终离线及新 Service 空状态 |
| 输入与查询副本隔离，包括嵌套 Session/capabilities | `TestQueryAndInputCopies`、并发查询测试、真实 Probe 查询 |
| 不依赖可丢事件队列 | `TestDeviceInventoryDoesNotDependOnEvents` 在 Events 满时验证上线与下线查询 |
| 真实 Probe 的 REGISTER、四次重连、历史截断和旧任务记录保留 | `TestDeviceInventoryRealProbeReconnectHistory` 使用容量 2，查询 5 个 Session/2 条已结束历史/2 条已淘汰历史，保留并重发旧 task_id，检查派发记录 |

Phase 1 原有全部 Go、C++ 与真实 Probe 测试保留，覆盖并发 exec、重复任务、ACK/RESULT 丢失、文件中断、FIFO、提交后确认丢失、非法协议和控制优先级。Device 历史淘汰不参与 Task/File 清理。

## 环境与复现

Windows amd64 原生 Go 1.25.5。Linux x86_64 使用既有 WSL 隔离构建镜像，源码绑定 `/work`，Alpine / GCC 15.2.0、Go 1.26.3、CMake 4.2.3；不引入项目运行依赖。

Linux 仓库根目录：

~~~sh
cmake -S probe -B build/phase2-probe -DCMAKE_BUILD_TYPE=Release
cmake --build build/phase2-probe --parallel 2
ctest --test-dir build/phase2-probe --output-on-failure
go build -o build/server/router-server ./cmd/server
RMP_PROBE_BIN=/work/build/phase2-probe/router-probe go test ./... -count=1
RMP_PROBE_BIN=/work/build/phase2-probe/router-probe go test -race ./... -count=1
go vet ./...
~~~

沿用 Phase 1E 配置重新构建 ASan/UBSan 与 TSan Probe，运行 `ASAN_OPTIONS=detect_leaks=1 ctest --test-dir build/phase1e-asan --output-on-failure`、TSan CTest，以及 `RMP_PROBE_BIN=/work/build/phase1e-tsan/router-probe TSAN_OPTIONS=halt_on_error=1 go test ./tests/integration -count=1`；初次配置命令见 PHASE1_VERIFICATION.md。

Windows：`go test ./... -count=1`、`go build -o build/server/router-server.exe ./cmd/server`、`go vet ./...`。真实 Linux Probe 在 Windows 不运行；在上述 Linux 环境设置 RMP_PROBE_BIN 后实际执行。

## 结果

| 验证 | 实际结果 |
| --- | --- |
| Linux Release C++11 构建 + CTest | 通过，3/3，4.70 秒 |
| Linux Server `go build`、`go vet ./...` | 最终代码通过 |
| Linux 最终全量 `go test ./... -count=1`，设置 Release 真实 Probe | 全包通过；Phase 1/2 集成 149.921 秒 |
| Linux 最终全量 Go race + TSan 真实 Probe，`halt_on_error=1` | 全包通过；Phase 1/2 集成 156.221 秒，无 race/TSan 报告 |
| C++ ASan/UBSan/LSan CTest，`detect_leaks=1` | 通过，3/3，5.91 秒，无报告 |
| C++ TSan CTest | 通过，3/3，7.03 秒，无报告 |
| Windows 原生 Server 构建、全量 Go、vet | 最终代码通过；真实 Linux Probe 用例在 Windows 跳过，在 Linux 实际执行 |
| `git diff --check` | 通过 |

最终组合回归命令：

~~~sh
RMP_PROBE_BIN=/work/build/phase1e-tsan/router-probe TSAN_OPTIONS=halt_on_error=1 \
  go test -race ./... -count=1
~~~

新增 13 个测试函数，另含多组子用例。时间排序调整后重新运行最终普通与组合回归；没有省略既有 Phase 1 用例。C++ 源码未改动，sanitizer 构建沿用既有 Phase 1E 配置并重新构建验证。

## 已知限制与交付边界

- 仅 Server 进程内保留，不实现数据库/磁盘快照/进程重启恢复；设备数量无自动淘汰，内存随已见设备数增长。
- 历史是 Session 状态记录，不是完整审计、每次心跳或可重放事件流；超过容量的历史不可查询，计数显示截断。
- capabilities 是 Probe 声明，不是设备认证、授权或 Server 已实现能力的证明；boot_id 不证明 Probe 进程连续性。
- 未实现 Phase 3 File/Tool Repository、HTTP/WebSocket/UI/MCP、Tunnel 或其他后续功能；既有 Phase 1 资源和平台矩阵限制保持。
- 验收已通过，以独立 `feat: complete Phase 2 device management` commit 交付并推送 GitHub，随后停止等待用户验收。实际 SHA 由 `git log --format=fuller --grep="Phase 2"` 查询，不在提交自身写入自引用 SHA。
- 具备申请进入 Phase 3 的代码与验证基础；Phase 3 仍未开始，须用户验收与授权，其具体存储、版本、兼容性设计仍须下一阶段确认，不由本阶段隐含决定。
