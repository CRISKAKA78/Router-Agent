# Phase 1E Verification 验收记录

日期：2026-09-05。验收起点：main 与 origin/main 均为 `d61054543500fbf5a62c94cd2afe492637230b85`，工作区干净。

## 验收结论

**Phase 1 验收通过，Phase 1A～1E 完成，具备申请进入 Phase 2 的条件。**此次范围为 Phase 1A～1D，沿用 Protocol v1 与 Accepted ADR-009～017，不新增后续功能或修改 wire 互操作语义。

## 本阶段修复

| 问题 | 处理与回归证据 |
| --- | --- |
| Server 把 `load1="0.21"` 当作 number 接受 | 解码后验证真实 number 类型；`TestHeartbeatNumberType` 修复前失败，修复后覆盖字符串、null、布尔、对象/数组、负数、溢出及合法小数/指数 |
| Server 将未配对 surrogate 替换后接受 | 复用文件协议已有的 Unicode 校验至公共 protocol helper；REGISTER、HEARTBEAT、TASK_ACK/RESULT 与 FILE 均拒绝非法 Unicode；`TestControlUnicodeContract` 修复前复现，合法中文、surrogate pair、字面反斜杠与 U+FFFD 保持可用 |
| Probe 未知扩展字段中的合法大整数导致整帧失败 | JSON parser 保留其原始值；已知 uint64 字段仍拒绝溢出；`probe_tests` 先复现，再验证前向兼容 |
| REGISTER_ACK 与下一超限 Header 同批到达时等待未到齐的 Payload | 应用协商上限后立即复核 decoder 的已有缓冲，无需再收到字节；真实 `TestProbeInvalidProtocolReconnect/control-limit-header-only` 修复前超时，修复后立即断开；完整合帧的 1024/1025 边界也验证 |
| 持续有合法流量但一直不确认心跳时，Probe pending 集合无限增长 | 每连接限制 1024 项；满后在下一 HEARTBEAT 前沿用断线重连，不淘汰仍有效的旧关联；C++ 验证容量、交错 ID、乱序/旧 ACK、重复 ACK 与容量释放 |
| Server 文件终态记录保留接收队列中的文件块 | worker 退出时断开记录对邮箱的引用；保留任务身份和快照；`TestEndedTransferReleasesMailbox` 覆盖满队列、20 个并发发送者、取消释放与终态不保留块数据 |
| Server 入站 message_id 计数存在理论回绕分支 | 在最大值处结束连接，避免下一帧接受保留值 0；非法零、重复与跳号由真实连接矩阵覆盖；未声称实际发送过 2^64 帧 |

已有设计未决主题仍按原文保留。未发现本次验收必须新增 Protocol v1 互操作语义才能解决的设计缺口，因此没有新增或改写 Accepted ADR。

## Protocol v1 验收基线映射

下列编号对应 PROTOCOL.md 的 14 项 Phase 1 验收基线。

| 编号 | 验收内容 | 可运行证据 |
| --- | --- | --- |
| 1、2 | 主动注册、重连、新 session_id | `TestProbeReconnectCreatesNewSession`、Gateway 注册/替换与 ACK 发布顺序回归 |
| 3 | TASK/ACK/RESULT 与 exec | `TestTaskExecEndToEnd`；成功/失败/cwd/env/双流截断/timeout/进程组与 fd 回收 |
| 4 | 至少 3 个并发任务、乱序完成且不串结果 | `TestConcurrentTasksOutOfOrderAndResend` 使用执行屏障与反序释放；C++ TaskManager 并发竞争 |
| 5、13 | 同 ID 副作用只执行一次，覆盖重连 | queued/running/终态重复、内容冲突、容量拒绝、ACK/RESULT 丢失及跨连接补报；`phase1c_test.go` 与 Task Service 幂等状态回归 |
| 6、7 | 二进制双向文件与完整性 | `TestFileRoundTrip`：0、1、65536、65537、3 MiB+17 字节；mode、提交事实与重复文件任务不重写 |
| 8、14 | 控制优先、单 active 与有界 FIFO | 原下载和新增上传各 16 MiB 慢链路测试，心跳 ACK 与 exec 在文件完成前到达；混合 FIFO、队满拒绝、Go 帧边界优先级测试与真实双向 Probe 控制处理 |
| 9、11 | 非法 Header/JSON、上限、message_id/reply_to/flags | Go/C++ codec 单测；新增 Server 31 组与真实 Probe 34 组非法输入，包含所有 16 个 TASK/HEARTBEAT flag bit、零/重复/跳号、非法 UTF-8、JSON 非 object、缺失/错误 reply_to、Header-only 上限、保留 TASK_CANCEL；Probe 每次都重连，最后可执行 exec |
| 10 | 协议日志标识 | Probe TASK_ACK 包含 message_id/task_id，file_active 包含 task_id/transfer_id；Gateway FILE 日志包含 message_id/session_id/transfer_id；真实集成捕获日志并用心跳 ACK 日志作为控制处理证据 |
| 12 | canonical UUID 与 RFC 4122 wire | Go `TestWireContract` 与 C++ `file_tests`，固定 UUID 二进制表示、offset/length/flags 校验 |

额外故障与资源验证：

- 保留全部 A～D 测试，覆盖最终 done ACK 丢失、临时文件清理、upload 已提交 success 与 download 本地 committed/任务 failed 的既有边界。
- `TestFileSourceMutationBeforeStreaming`：upload/download 在预读摘要后、ready 放行前修改同一源文件，任务 failed，无完整目标发布，临时文件清理，重连后仍能 exec。
- `TestProbeReconnectResourceConvergence`：同一 Probe 反复中断 8 次 upload；每次重连补报全部旧失败结果，文件描述符与线程数回到基线，目录无临时文件或部分目标。
- `TestServerInvalidProtocolConverges`：各错误连接退出后 Session 与 connection 集合归零；Server Close/Serve 正常结束。
- `TestReceiverSuccessfulPublication`：Linux/Windows 原生本地空文件和二进制文件发布，覆盖 overwrite=true/false；原有测试保留校验失败与并发创建目标不覆盖。

## 构建与测试结果

环境：Linux x86_64，Alpine 3.24.1 / GCC 15.2.0 / CMake 4.2.3 / Go 1.26.3；Windows amd64 原生 Go 1.25.5。Linux 验证使用已存在的隔离构建镜像，源码绑定为 `/work`。

| 验证 | 结果 |
| --- | --- |
| Release C++11 构建 + CTest | 通过，3/3，4.71 秒 |
| Linux Server `go build` | 通过 |
| Linux 全量 `go test ./... -count=1`，设置真实 Probe | 全部通过，集成 143.593 秒 |
| Linux 全量 `go test -race ./... -count=1`，设置真实 Probe | 全部通过，集成 145.712 秒，无 race 报告 |
| Linux `go vet ./...` | 通过 |
| C++ AddressSanitizer + UndefinedBehaviorSanitizer，`detect_leaks=1` CTest | 通过，3/3，5.85 秒，无报告 |
| C++ ThreadSanitizer CTest | 通过，3/3，6.83 秒，无报告 |
| TSan 真实 Probe 全量集成，`halt_on_error=1` | 通过，148.568 秒，无报告 |
| 新增源变更/正向发布用例补充普通运行 | 通过，真实集成 2.299 秒 |
| 同一补充用例 Go race + TSan Probe | 通过，真实集成 3.368 秒，无报告 |
| Windows Server 构建、全量 Go、vet | 通过；真实 Linux Probe 用例在 Windows 自动跳过，Linux 上按上表实际执行；新增正向发布用例也已在 Windows 单独通过 |
| `git diff --check` | 通过 |

主要复现命令（Linux，在仓库根目录）：

~~~sh
cmake -S probe -B build/phase1e-probe -DCMAKE_BUILD_TYPE=Release
cmake --build build/phase1e-probe --parallel 2
ctest --test-dir build/phase1e-probe --output-on-failure
go build -o build/server/router-server ./cmd/server
RMP_PROBE_BIN="$PWD/build/phase1e-probe/router-probe" go test ./... -count=1
RMP_PROBE_BIN="$PWD/build/phase1e-probe/router-probe" go test -race ./... -count=1
go vet ./...

cmake -S probe -B build/phase1e-asan -DCMAKE_BUILD_TYPE=Debug \
  -DCMAKE_CXX_FLAGS=-fsanitize=address,undefined -DCMAKE_EXE_LINKER_FLAGS=-fsanitize=address,undefined
cmake --build build/phase1e-asan --parallel 2
ASAN_OPTIONS=detect_leaks=1 ctest --test-dir build/phase1e-asan --output-on-failure

cmake -S probe -B build/phase1e-tsan -DCMAKE_BUILD_TYPE=Debug \
  -DCMAKE_CXX_FLAGS=-fsanitize=thread -DCMAKE_EXE_LINKER_FLAGS=-fsanitize=thread
cmake --build build/phase1e-tsan --parallel 2
ctest --test-dir build/phase1e-tsan --output-on-failure
RMP_PROBE_BIN="$PWD/build/phase1e-tsan/router-probe" TSAN_OPTIONS=halt_on_error=1 \
  go test ./tests/integration -count=1
~~~

## 交付与下一阶段

Phase 1E 修复与验收记录放在同一个独立 Verification commit，推送 GitHub main 后停止等待验收。提交 SHA 由 `git log --format=fuller --grep="Phase 1E"` 查询，不在提交自身写入自引用 SHA。

Phase 2 的前置条件是 Phase 1 完成验收；Phase 2 的设备模型与服务设计仍按 ROADMAP/ARCHITECTURE 开展，须用户另行授权。生产安全设计、嵌入式 CPU/libc/最低内核实机验证、进程重启恢复仍属后续工作；本次 Linux x86_64 验收不替代这些矩阵。
