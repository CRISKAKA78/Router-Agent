# 项目状态

最后更新时间：2026-09-05

## 当前阶段

Phase 0、Phase 1A TCP Session、Phase 1B Task and Exec 已形成可验证里程碑。接管审查 R1-R4 已按用户授权修复并通过回归；本次 Phase 1C 启动检查完成，因三项互操作契约仍未确认而暂停实现，详见 PROTOCOL.md 末尾。用户已授权 Phase 1C，但要求设计缺口先记录汇报。

baseline commit: bc8d747dfc41a375c31698073005857c238ede51

Phase 1A commit: cd722b6f3fd6cfe5e8ccded256c5295828e4372f

Phase 1B implementation commit: 6ed2434d646938617088f62030f3749a797616c0

当前 HEAD：`f0ed826`。本次 R1-R4 修复和审查文档在工作区，尚未提交或推送；没有 Phase 1C commit。

## 当前可用能力

- Go Management Server：TCP framing、注册、心跳、会话、内部 CreateExec / WaitTaskResult / TaskSnapshot，按 task_id 接收单次 ACK/RESULT。
- C++11 + CMake Probe：主动连接、注册、心跳、重连；单 worker 执行 `/bin/sh -c`，支持 cwd/env、独立 stdout/stderr、timeout 和输出截断。
- Server 完整写出 REGISTER_ACK 后发布会话；每条连接的主动 TASK 与 ACK/ERROR 共用串行 writer 和 message_id。
- Server 传输写失败或 message_id 耗尽时关闭并废弃 writer。发送结果不确定时 CreateExec 返回非空 task_id 与 ErrDispatchUncertain，保留派发记录，不自动重发或新建替代任务。发送前失败不保留任务。调用契约见 [API.md](API.md)。
- Probe socket 设置 FD_CLOEXEC；socket/pipe 创建及 FD_CLOEXEC 设置与 fork 共用互斥锁，兼容不支持原子 close-on-exec 创建的目标。
- timeout 或 worker 停止时向原进程组发送 TERM，完成 200 ms grace 后 KILL；在仍需发送组信号时保留直接子进程 PID，避免过早回收后 PID 重用。TERM 后约 400 ms 结束残留 pipe 排空，强制关闭时设置 truncated；直接子进程由 waitpid 回收。
- 两路输出各保留最多 1 MiB；通常持续排空，最终结果适配协商控制帧大小。TASK_ACK accepted=false 仍为最终拒绝，TASK_RESULT 不设置 RESPONSE。

## 当前未实现与限制

- 单 worker，多任务并发、完整 task_id 幂等和去重、断线继续执行/补报未实现。断线仍会停止当前 worker；不宣称已支持 Phase 1C。
- 重复 TASK 响应、同 ID 参数冲突、ACK 丢失和重复 RESULT 补报仍待明确确认，见 PROTOCOL.md 末尾。
- 原进程组的 KILL 不覆盖主动 setsid 脱离组的后代；本轮保证这类后代持有的 pipe 不会无限阻塞 worker，不提供进程容器或 Process Manager。直接子进程若陷入不可中断内核等待，回收仍取决于内核。
- 非阻塞审查建议尚未处理：更严格的 JSON 类型/Unicode 校验、队列和缓存容量、部分协商 payload 边界，见 [PHASE1AB_REVIEW.md](PHASE1AB_REVIEW.md)。
- Probe/Server 重启恢复、安全体系、mipsel/ARM/ARM64 实机及 libc/最低内核矩阵仍为 TBD 或未验证。
- 未实现 TASK_CANCEL、文件传输、Tunnel、HTTP/WebSocket API、数据库、UI、CLI、MCP、AI Agent、VPN、FRP、SSH/Telnet 通道。

## Phase 1C 启动检查验证

本次在 Windows 当前工作区运行 `go build -o build/server/router-server.exe ./cmd/server`、`go test ./... -count=1`、`go vet ./...`，均通过。未设置 RMP_PROBE_BIN，真实 Linux Probe 集成测试按现有规则跳过；未重新运行 Linux CTest 或 race，不作为 Phase 1C 完整回归。仅补充门槛与状态文档，未修改业务代码，未提交或推送；原有 R1-R4 工作区改动保持。

## 此前 R1-R4 构建与测试

Linux x86_64 隔离环境：Alpine 3.24，GCC 15.2.0、CMake 4.2.3、Go 1.26.3。

已通过：

- `cmake -S probe -B build/review-probe -DCMAKE_BUILD_TYPE=Release`
- `cmake --build build/review-probe --parallel 2`
- `ctest --test-dir build/review-probe --output-on-failure`：1/1 passed，包含新增后代进程清理测试。
- `go build -o build/server/router-server ./cmd/server`
- `RMP_PROBE_BIN=/work/build/review-probe/router-probe go test ./... -count=1`：全部通过，真实 Probe 集成 14.906 秒。
- `RMP_PROBE_BIN=/work/build/review-probe/router-probe go test -race ./... -count=1`：全部通过，真实 Probe 集成 15.963 秒。
- `go vet ./...`：通过。
- Windows：Server 原生构建、Go 单测和 go vet 通过；真实 Probe 测试在 Linux 中运行。

新增回归覆盖注册 ACK 阻塞时会话不可提前派发、同设备会话替换、部分/零字节/整帧写入后报错、失败 writer 多调用者复用拒绝、发送前失败及 message_id 耗尽、不确定派发记录保留、真实 exec 不继承 socket、忽略 TERM 且输出重定向的后代 KILL、脱离组持有 pipe 时的 timeout 与停止路径，以及子进程回收。C++ 测试的 subreaper 仅用于测试后代回收，Probe 运行时无此要求。

## 下一步

先明确确认 PROTOCOL.md 末尾的重复 TASK、内容冲突及跨连接补报契约，再实现已授权的 Phase 1C。实现完成后运行完整回归、同步交接文档、形成独立 Phase 1C commit 并推送 GitHub，然后停止等待验收，不进入 Phase 1D。R1-R4 修复仍未提交，须与 Phase 1C 交付区分。审查原始证据、先前操作事故与恢复限制保留在 [PHASE1AB_REVIEW.md](PHASE1AB_REVIEW.md)。
