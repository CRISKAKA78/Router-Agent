# 项目状态

最后更新时间：2026-09-05

## 当前阶段

Phase 1 - Probe and Server TCP Control Link：进行中。

Phase 1A - TCP Session：已完成并验证。按用户要求在此停止，未进入 Phase 1B。

baseline commit: bc8d747dfc41a375c31698073005857c238ede51

## 当前代码状态

- Management Server 使用 Go，实现位于 `cmd/server`、`internal/protocol` 和 `internal/gateway`。
- Probe 使用 C++11 + CMake，实现位于 `probe`，仅依赖 C++ 标准库、POSIX socket 和 pthread。
- Server 与 Probe 共用 Protocol v1 的 20-byte Header、Big Endian、message_id 和 reply_to 语义。
- 当前工作树包含尚未提交的 Phase 1A 实现与交付文档；Phase 0 baseline commit 保持独立。

## 当前可用功能

- Probe 主动 TCP connect 到 Server。
- TCP stream framing 支持 Header/Payload 拆包和一次读取多帧。
- REGISTER 必选字段、类型、长度、ASCII 和数组范围校验。
- REGISTER_ACK success=true / false；失败时 Probe 不进入 ONLINE，并遵守 retry_after 或本地退避。
- REGISTER 成功后形成随机不透明 session_id；每次重连重新 REGISTER 并生成新 session_id。
- Probe 按 REGISTER_ACK 的 heartbeat_interval 发送 HEARTBEAT，Server 返回带正确 reply_to 的 HEARTBEAT_ACK。
- Server 与 Probe 均使用 `3 * heartbeat_interval` 失联阈值。
- Probe 使用 1/2/5/10/30 秒、之后固定 30 秒的基础退避重连；TCP connect 成功后重置退避。
- 协议与状态日志包含 message_id、reply_to、device_id 和 session_id 中的相关标识。

## 当前未实现

TASK、TASK_ACK、TASK_RESULT、exec、文件传输、Process Manager、Tunnel、HTTP / WebSocket API、数据库、任何 UI、CLI、MCP、AI Agent、VPN、FRP、SSH / Telnet 通道和后续安全体系均未实现。

## 构建与测试

第一验证环境：Linux x86_64（WSL2，Alpine 3.24.1，GCC 15.2.0，CMake 4.2.3，Go 1.26.3）。

已通过：

- `go build -o build/server/router-server ./cmd/server`
- `cmake -S probe -B build/probe -DCMAKE_BUILD_TYPE=Release`
- `cmake --build build/probe --parallel 2`
- `ctest --test-dir build/probe --output-on-failure`：1/1 passed。
- `RMP_PROBE_BIN="$PWD/build/probe/router-probe" go test ./... -count=1`：全部通过；真实 Probe 进程断线后约 1 秒重连并获得不同 session_id。
- `RMP_PROBE_BIN="$PWD/build/probe/router-probe" go test -race ./... -count=1`：全部通过。
- `go vet ./...`：通过。
- 实际启动 Server 与 Probe 后观察到 REGISTER / REGISTER_ACK 和连续 HEARTBEAT / HEARTBEAT_ACK；停止并重启 Server 后，Probe 按 1/2/5/10/30 秒序列退避并用新 session_id 重新上线。

自动化覆盖 Header encode/decode、Big Endian、半包 Header、半包 Payload、多帧一次读取、bad magic、unsupported version、payload 超限、REGISTER 合法/缺字段/超范围、REGISTER_ACK success=false、HEARTBEAT_ACK reply_to 和断线重连新 session_id。

## 已知问题与待决策

- 用户认证、Probe 身份认证、TLS、权限、租户、密钥轮换和审计尚未设计，本阶段未实现。
- Probe / Server 重启后的任务与传输恢复、数据库、Tunnel 数据面、OpenAPI 与 WebSocket 协议仍为后续阶段主题。
- mipsel、ARM、ARM64 toolchain files 和真实设备兼容矩阵尚未验证，本阶段只完成 Linux x86_64。
- Protocol 错误关闭矩阵仍为 TBD；Phase 1A 对 bad magic 直接关闭，对可关联的 unsupported version / payload 超限返回 ERROR 后关闭，不扩展未决矩阵。

## 阻塞项

Phase 1A 无阻塞项并已完成。进入 Phase 1B 需要用户明确授权。

## Git 状态

Phase 0 baseline：`bc8d747dfc41a375c31698073005857c238ede51`。

Phase 1A 代码与文档当前位于 baseline 之后的工作树，尚未创建 Phase 1A commit。

## 下一步

停止开发并等待用户验收或明确授权 Phase 1B；不得自动实现 TASK、exec 或任何后续能力。
