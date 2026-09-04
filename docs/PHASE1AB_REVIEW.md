# Phase 1A / 1B 接管审查

日期：2026-09-05。审查基线：`f0ed826`（Phase 1B implementation：`6ed2434`）。

原始审查结论：**审查时不能安全进入 Phase 1C。** 当时发现阻塞缺陷，只交付审查与待确认事项。下文 R1-R4 保留原始证据；用户随后授权修复，最新修复结果见文末。本报告不代表 Phase 1C 已完成。

## 1. 必须修复的问题

### R1 — P1：新 Session 在 REGISTER_ACK 之前对任务下发可见

- 位置：`internal/gateway/server.go:381`、`:394`，以及 `CreateExec`。
- Server 先把 candidate 放入 `sessions`，再关闭旧连接，最后发送 REGISTER_ACK。此时并发的 CreateExec 可以获取 candidate 并先取得连接写锁。
- 临时 Go 测试在替换旧连接的 Close 处设置同步屏障，确定性得到第一帧 `TASK(type=0x10, message_id=1)`，第二帧 `REGISTER_ACK(type=0x02, message_id=2)`。Probe 注册状态要求首帧为 REGISTER_ACK，因而关闭连接；已经创建的任务也可能悬置。
- 修复目标：只有成功写出 REGISTER_ACK 的连接才可被业务派发使用；发布和同 device_id 会话替换需要共同保证顺序。增加重连注册与并发派发回归。

### R2 — P1：部分写失败后仍复用连接和 message_id

- 位置：`internal/gateway/server.go:308`、`:211`。
- `connectionWriter.sendJSON` 在 WriteFrame 失败时直接返回，没有关闭或废弃连接，计数器也保持原值。CreateExec 的错误路径随后删除任务记录。
- 临时 Go 故障注入让首次 Write 返回 `n=7, error`；第二次发送仍成功，`message_id=1`，连接 `closed=false`。这样会把下一帧插入上一帧剩余载荷，并破坏接收端 framing。即使接收端收齐了前一任务而本地写入报错，直接删除记录也会失去副作用任务的关联依据。
- 修复目标：任何传输写失败后废弃该连接，禁止后续 writer 继续使用；区分编码/长度等发送前失败与发送结果不确定，后者保留任务标识及派发事实，供重连关联。`message_id` 耗尽也应主动结束连接。

### R3 — P1：exec 子进程继承控制 socket

- 位置：`probe/src/client.cpp:232`，`probe/src/task.cpp:382`、`:412` 附近的 fork/exec 路径。
- socket 创建时没有 close-on-exec，子进程只关闭本任务 pipe 的多余端点。真实 Probe 下发 `for f in /proc/$$/fd/*; do readlink "$f"; done`，stdout 包含 `socket:[13507]`（该编号仅为本次运行证据）。
- 影响：命令及其后代可持有控制连接引用；Probe 仅 close 旧 socket 时，其生命周期不再由 Connection Manager 独占。进入跨连接继续执行和多 worker 后问题更明显。
- 修复目标：socket 不得泄漏到 exec 程序；并发创建 fd 与 fork 必须使用原子 close-on-exec 或一致的兼容同步方案。现有 `pipe()+fcntl(FD_CLOEXEC)` 在单 worker 下尚无多 exec fork 竞争，但不能直接照搬进多 worker。

### R4 — P1：timeout 的进程组清理和 pipe 等待不完整

- 位置：`probe/src/task.cpp:446-501`，`probe/src/client.cpp:125-129`。
- 情形一：直接子进程已回收、两个 pipe 均 EOF 时，循环可以在 200 ms grace 结束前退出，未执行 SIGKILL。下发 `sh -c 'trap "" TERM; echo $$ > <临时pid文件>; exec sleep 8' >/dev/null 2>&1 & wait`，timeout=1，1.00 秒返回 timeout，但记录的后代 PID 仍存活。复现工具已清理该 PID。
- 情形二：`setsid sleep 6 & wait`，timeout=1。脱离原进程组的后代继续持有 stdout/stderr，SIGKILL 原组后循环仍等待 EOF，实测 6.00 秒才返回 timeout。持续持有 pipe 的进程会持续占住 worker。
- 影响：前者残留执行；后者使任务超时和 Session 析构中的 worker join 都失去时间上界，可能阻止重连。扩大 worker 数不能修复它。
- 修复目标：分别管理直接子进程回收、进程组 grace/KILL 和输出排空期限，避免提前结束清理或无界等待 EOF；测试时覆盖忽略 TERM、重定向输出、脱离组且持有 pipe 的后代。不要把此修复扩展为 Phase 1D 或 Process Manager。

## 2. 建议优化但不单独阻塞的问题

- **JSON 校验不完全符合基线**：`internal/gateway/messages.go` 的 load1 解码接受数值字符串 `"0.21"`；包含未配对 Unicode surrogate `\ud800` 的 device_id 被 Go JSON 替换后接受。两个输入均已通过临时测试复现。建议明确拒绝错误类型和非法 Unicode，补上跨语言一致性测试。
- **资源容量**：Probe 的队列、Server 任务记录、Probe 的 pending heartbeat 集合没有容量或清理上界。后者在对端持续发送有效 TASK、却不确认心跳时会持续增长。Phase 1C 应选择明确的并发/排队/缓存配置；缓存淘汰不能突破进程生命周期去重要求。具体数值本身是实现配置，不要求新增协议字段。
- **锁与线程**：现有队列锁不覆盖执行或 socket 写，writer 的锁独立串行化完整帧，未发现这些锁之间已证实的反向获取死锁。已证实的挂起来自 R4 的等待路径。多 worker 所需的任务登记、ACK 与启动顺序、结果缓存和连接切换必须以新的并发测试验收，不能用当前 Go race 结果证明 C++ 安全。
- **协商帧上限**：Probe 注册 decoder 会先按 1 MiB 解出同批全部帧，再应用 REGISTER_ACK 的较小 max_control_payload；同批尾随 TASK 未重新校验新上限。TASK_ACK 的错误原因也没有按协商上限约束，超长未知 type 会被回显。建议增加 1024-byte 协商值下的合帧与拒绝响应测试。

## 3. Phase 1C 设计门槛

已确认且应直接遵守：不同 task_id 可并发和乱序完成；message_id 仅限连接/方向；reply_to 关联当前请求帧；每次注册产生新 session；exec 断线后继续运行并缓存/补报；同一 Probe 进程内已接受 task_id 永不重复执行；TASK_ACK accepted=false 是最终拒绝；不实现 TASK_CANCEL。

以下互操作细节在 PROTOCOL.md 中仍缺少完整响应契约，已在其末尾记录为待确认，未更改 Accepted ADR：

1. QUEUED / RUNNING / 已完成任务收到重复 TASK 时，各自发送哪些帧、TASK_ACK.state 的枚举及顺序；已完成任务是否仍先 ACK 再重发 RESULT。
2. 同一 task_id 携带不同 type、timeout 或 params 时，怎样报告冲突，并保持原任务终态不变。
3. ACK 丢失后的重连补报、RESULT 已到达但 Probe 无法确认时的再次补报：Server 如何接受已派发但缺 ACK 的迟到结果、如何处理重复终态；重发 TASK 的 reply_to 必须以 session/message_id 派发记录关联，不能跨 session 比较裸 message_id。

当前 `internal/task/service.go:169-206` 只存一个 MessageID，拒绝重复 ACK/RESULT，只允许 accepted ACK 的 state=queued，并要求 RESULT 前已经保存 accepted ACK。正常已 ACK 任务的迟到 RESULT 可按 device_id/task_id 接收；但这些限制尚不能支撑上述完整 1C 行为。它们属于 Phase 1C 尚待实现的差距，不作为虚构的 Phase 1B 完成项。

结果缓存容量是实现配置；如果淘汰结果，应保留足以阻止重执行的身份记录，或者在容量不足时停止接受新任务。不得将缓存淘汰解释为允许重执行，也不得把系统 boot_id 当作可靠的 Probe 进程 incarnation ID。

## 4. 自动化覆盖与验证

现有测试覆盖 framing 半包/多帧、基本注册/心跳、单次重连、正常 TASK/ACK/RESULT、TASK 与 HEARTBEAT_ACK 并发写、exec 成功/失败/cwd/env/双流截断/普通 timeout/执行中心跳及未知任务类型拒绝。

明显遗漏：R1-R4 的边界；至少三个任务同时运行且乱序完成；QUEUED/RUNNING/完成后三种重复 task_id；并发重复投递和参数冲突；ACK 丢失/RESULT 丢失/重连时完成；多次重连及新旧 session 的同值 message_id；重复 RESULT 不回退终态；容量耗尽；多 worker 的 fd 继承竞争；部分写与连接失效。

本次验证：Windows Go 单测与 go vet 通过。临时 Go overlay 测试在工作区之外运行，R1、R2 和 JSON 契约测试按预期失败，证明缺陷；未把这些复现写入产品源码。

Linux x86_64 隔离验证环境：Alpine 3.24，GCC 15.2.0、CMake 4.2.3、Go 1.26.3。已重新构建当前源码并通过：

- `cmake -S probe -B build/review-probe -DCMAKE_BUILD_TYPE=Release`
- `cmake --build build/review-probe --parallel 2`
- `ctest --test-dir build/review-probe --output-on-failure`：1/1。
- `go build -o build/server/router-server ./cmd/server`
- `RMP_PROBE_BIN=/work/build/review-probe/router-probe go test ./... -count=1`：全部通过，真实集成测试 14.579 秒。
- `RMP_PROBE_BIN=/work/build/review-probe/router-probe go test -race ./... -count=1`：全部通过，真实集成测试 15.678 秒。
- `go vet ./...`：通过。

真实 Probe 的 R3/R4 定向复现结果见上文。现有回归通过不抵消已复现缺陷，也不代表 Phase 1C 验收通过。

## 5. 操作事故与恢复

清理首次临时 Linux 验证环境时，绑定的工作区未成功卸载，错误的后续清理删除了仓库目录及本地 build 产物。这是本次审查操作失误，不是项目缺陷，已立即向用户说明。

审查开始时 `git status --short` 为空，HEAD 为 `f0ed826`。随后从原 origin `https://github.com/CRISKAKA78/Router-Agent.git` 恢复相同 HEAD 和已核对的最近提交历史；36 个受版本控制文件恢复，`git fsck --full` 通过，恢复后工作区干净。本地忽略的旧 build 产物未原样恢复，本次从恢复后的源码重新构建验证。原 `.git` 的本地 reflog、配置及可能存在的未推送 refs 不能通过重新 clone 原样恢复；开始审查时未盘点所有本地 refs，因此不能声称本地 Git 元数据已完整恢复。没有改写远端历史。

## 6. 授权修复结果（2026-09-05）

用户在审查后明确要求开始并继续修复。本轮仅修复 R1-R4 和必要回归，未采纳尚待确认的 Phase 1C wire 契约。

| 问题 | 修复 | 已通过的自动化验证 |
| --- | --- | --- |
| R1 | REGISTER_ACK 完整写出后，在 registry 锁内发布会话并检查 Server 是否关闭；旧会话关闭不持有全局锁。连接处理 WaitGroup 的 Add 与 Close 的注册检查也在同一锁内排序 | `TestRegisterPublishesOnlyAfterAck`：首次注册、同 device_id 替换，阻塞 ACK 写入期间新会话不可见，后续 TASK 从 message_id=2 开始 |
| R2 | writer 保存永久 failed 状态并关闭连接；write 已尝试的错误返回 message_id，CreateExec 保留记录并返回 task_id + ErrDispatchUncertain；发送前失败仍移除未派发记录 | `TestWriterFailurePermanentlyInvalidatesStream`、`TestCreateExecKeepsUncertainDispatch`、`TestWriterPreWriteFailures`：覆盖零字节/部分/整帧后报错、多个调用者、编码/长度/deadline 失败与计数耗尽 |
| R3 | socket 设置 FD_CLOEXEC；socket/pipe 创建与设置 FD_CLOEXEC、fork 共用 ExecForkMutex，父进程在 fork 后释放，子进程仅 exec/_exit | 真实 Probe `TestTaskExecEndToEnd` 检查 shell 的 /proc/$$/fd，不含 socket；其余 exec 与重连回归通过 |
| R4 | TERM 后即使 shell 已退出且 pipe EOF，也完成 200 ms grace 后的组 KILL；仍需发信号时不提前回收直接子进程 PID。TERM 后约 400 ms 关闭仍未 EOF 的读取端并标记 truncated | C++ `TestExecDescendantCleanup` 覆盖忽略 TERM 且重定向输出的后代、setsid 后代持有 pipe 的 timeout 和 stop；校验同组后代由 SIGKILL 结束、排空有界且无测试后代未回收 |

R4 边界：原进程组清理不声称能终止主动 setsid 的后代；它们的 pipe 不再阻塞 worker。测试 runner 使用 subreaper 收回测试后代，生产 Probe 不依赖 subreaper。不可中断内核等待的直接子进程不能由用户态保证即时回收。

完整验证：Linux Server/Probe 构建、CTest 1/1（4.58 秒测试时间）、真实 Probe `go test ./... -count=1`（集成 14.906 秒）、`go test -race ./... -count=1`（集成 15.963 秒）、`go vet ./...` 均通过；Windows Server 原生构建、单测和 vet 通过。命令与当前限制见 PROJECT_STATUS.md。本次代码和文档变更尚未提交或推送。

## 下一步

R1-R4 修复完成，等待验收；进入 Phase 1C 仍须明确确认 PROTOCOL.md 中的待讨论互操作补充。非阻塞建议保留，不自动扩展本轮修复范围；不进入 Phase 1D。
