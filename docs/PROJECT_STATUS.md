# 项目状态

最后更新时间：2026-09-05

## 当前阶段

Phase 1 - Probe and Server TCP Control Link：进行中。

Phase 1A - TCP Session：已完成并验证。

Phase 1B - Task and Exec：已完成并验证。按用户要求在此停止，未进入 Phase 1C。

baseline commit: bc8d747dfc41a375c31698073005857c238ede51

Phase 1A commit: cd722b6f3fd6cfe5e8ccded256c5295828e4372f

Phase 1B implementation commit: pending

## 当前代码状态

- Management Server 使用 Go，实现位于 `cmd/server`、`internal/protocol`、`internal/gateway` 和 `internal/task`。
- Probe 使用 C++11 + CMake，实现位于 `probe`，仅依赖 C++ 标准库、POSIX socket、pipe、poll、进程与 pthread 能力。
- Server 与 Probe 共用 Protocol v1 的 20-byte Header、Big Endian、message_id、reply_to 和 task_id 语义。
- Phase 1A 已提交为 `cd722b6f3fd6cfe5e8ccded256c5295828e4372f`；Phase 1B 等待本次独立提交。

## 当前可用功能

- Phase 1A 的 TCP framing、REGISTER、REGISTER_ACK、HEARTBEAT、HEARTBEAT_ACK、失联判断与基础断线重连保持可用。
- Management Server 可通过内部 `CreateExec` / `WaitTaskResult` 能力对当前在线 device_id 创建 exec 任务并等待最终结果。
- Server 生成 task_id，下发 TASK，校验 TASK_ACK 的 RESPONSE、reply_to 和 task_id，并按 task_id 接收 TASK_RESULT。
- 每条 Server 连接以同一写锁串行发送 REGISTER_ACK、HEARTBEAT_ACK、TASK 和 ERROR，共用从 1 开始的单连接发送 message_id 序列。
- Probe 在线消息路由当前只处理 HEARTBEAT_ACK 与 TASK，不再假定所有 Server 消息都是 HEARTBEAT_ACK。
- Probe 对合法 exec 立即返回 `TASK_ACK accepted=true state=queued`；对 unsupported task type 返回 `accepted=false state=rejected` 且不产生 TASK_RESULT。
- Probe 使用单 Task Worker 串行执行任务，TCP Reader 与命令执行分离；当前不包含 Phase 1C 多任务并发。
- exec 使用 `/bin/sh -c`，支持必选 command、可选 cwd 与 env，分别捕获 stdout / stderr。
- stdout 与 stderr 各自最多保留 1 MiB；pipe 始终继续排空，超限丢弃并设置 `truncated=true`，最终 JSON 还会裁剪到协商的 max_control_payload。
- timeout 到期后对独立进程组发送 SIGTERM，200 ms 后仍未退出则发送 SIGKILL，并使用 waitpid 回收直接子进程。
- TASK_RESULT 区分 success、failed 和 timeout，包含时间、exit_code、stdout、stderr、truncated 与空 result object，且不设置 RESPONSE。

## 当前未实现

Phase 1C 的多任务并发、乱序结果能力验收、完整 task_id 幂等、TCP 重连后的任务关联与结果缓存均未实现。文件传输、Process Manager、Tunnel、TASK_CANCEL、HTTP / WebSocket API、数据库、任何 UI、CLI、MCP、AI Agent、VPN、FRP、SSH / Telnet 通道和后续安全体系也未实现。

## 构建与测试

本轮验证环境：Linux x86_64（Docker Desktop WSL2 内的隔离 Alpine 3.20 根文件系统，GCC 13.2.1，CMake 3.29.3，Go 1.22.10）。

已通过：

- `go build -o build/server/router-server ./cmd/server`
- `cmake -S probe -B build/phase1b-probe -DCMAKE_BUILD_TYPE=Release`
- `cmake --build build/phase1b-probe --parallel 2`
- `ctest --test-dir build/phase1b-probe --output-on-failure`：1/1 passed。
- `RMP_PROBE_BIN="$PWD/build/phase1b-probe/router-probe" go test ./... -count=1`：全部通过。
- `RMP_PROBE_BIN="$PWD/build/phase1b-probe/router-probe" go test -race ./... -count=1`：全部通过。
- `go vet ./...`：通过。

自动化覆盖 Phase 1A 的 Header、framing、注册、心跳与真实进程断线重连回归，以及 Phase 1B 的 TASK 字段、TASK_ACK flags/reply_to/task_id、TASK_RESULT flags/task_id、成功执行、exit 7、独立 stderr、cwd/env、timeout 进程清理、双路大输出有界截断、执行期间心跳、unsupported task 拒绝和注册响应后紧随 TASK 的多帧读取。

## 已知问题与待决策

- Phase 1B 只有单 worker 串行执行；Phase 1C 的并发、乱序结果与 task_id 幂等尚未实现。
- TCP 会话断开时当前执行会被本地终止，未实现跨重连继续执行、结果补报或缓存；这些属于 Phase 1C 与后续恢复设计。
- Probe 自身重启和 Server 重启后的任务恢复仍为 TBD。
- 用户认证、Probe 身份认证、TLS、权限、租户、密钥轮换和审计尚未设计。
- mipsel、ARM、ARM64 toolchain files 和真实设备兼容矩阵尚未验证，本阶段仍只验证 Linux x86_64。
- Protocol 错误关闭矩阵仍为 TBD，本阶段未扩展未决矩阵。

## 阻塞项

Phase 1B 无实现阻塞项并已完成。进入 Phase 1C 需要用户明确授权。

## Git 状态

Phase 0 baseline：`bc8d747dfc41a375c31698073005857c238ede51`。

Phase 1A commit：`cd722b6f3fd6cfe5e8ccded256c5295828e4372f`（`feat: complete Phase 1A TCP session`）。

Phase 1B implementation commit：pending。

## 下一步

停止开发并等待用户验收或明确授权 Phase 1C；不得自动实现多任务并发、task_id 幂等、重连任务恢复或任何后续能力。
