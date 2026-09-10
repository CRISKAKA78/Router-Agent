# Server 重启后 Probe 循环断线修复

## 现场与原因

2026-09-10 用户授权 Telnet 只读检查 192.168.5.222，并随后要求修复。设备为 FE7140555489，运行 `/tmp/root/router-probe --server pcv6.criskaka.com:9000`。以下时间以本机 Server 的北京时间为准：

- 16:53:54 Server 进程启动，16:54:04 首次注册后立即断线，之后循环；Server 控制台保留 ONLINE/DISCONNECTED，最近 64 条会话结束原因均为 protocol_error。
- 用户粘贴的 Probe 日志每次 REGISTER 后立即补报 TASK_RESULT，随后 `unexpected message type 0xfe`；17:13:28 重启 Probe 后建立新会话，17:18:41 仍在线，累计 919 个连接周期。
- 当时 Server 的该设备任务查询为空。Probe stdout/stderr 指向原 Telnet `/dev/pts/0`，未落盘，`/var/log/messages` 为空；原错误正文和具体旧任务 ID 未保留。现场原因是由时间线、协议日志和代码共同推定，并非声称取到了遗失的错误正文。
- 新增回归在旧代码上实际失败，收到 `{"reply_to":2,"code":"INVALID_PAYLOAD","message":"task not found"}`，帧类型为 254（0xfe），证明该路径会阻断后续心跳。

## 修复范围

`internal/gateway/server.go` 只对 Task Service 的 `ErrTaskNotFound` 分支记录并忽略已完成通用校验的 TASK_RESULT，继续本连接读取。不发 ERROR，兼容现有将 ERROR 当作致命协议错误的 Probe；未知任务不写入任务表、不创建替代任务。合法活动更新时间保持。

补报日志包含设备/会话/消息/任务 ID 和 `reason=task_not_found`；已知任务拒绝包含任务 ID 和具体原因；发送 ERROR 前记录 reply_to/code/detail，日志不包含任务 stdout/stderr。错误 JSON/flags/status/缺字段、错误设备、未派发或 rejected 任务、冲突终态仍拒绝，不放宽文件结果校验。

无 Probe、WPF、API DTO 或持久化模型改动。ADR-015 的补报、进程内去重和未知任务不改写约束保持；PROTOCOL 补充此修复规则，不引入新消息、ACK、epoch 或跨进程恢复能力。

## 验证

测试入口：

- `internal/gateway/orphan_result_test.go`：完成/失败/超时/重复/文件形状的未知结果后，同一 TCP 心跳及新任务成功，未知任务未创建，诊断不泄漏输出；8 种非法或已知任务冲突仍报错/断开且原快照保持。
- `tests/integration/server_restart_test.go`：真实 Linux Probe 已完成任务及运行中任务跨越整个 Gateway/Task/File Service 重建，旧 Probe 进程保持；等待完整心跳间隔、执行新任务、再次控制重连，确认旧结果未导入且旧命令副作用各执行一次。测试重建 Server 实例，不冒充在用户设备上重启生产 Server。

环境：Windows x64 / Go 1.25.5；WSL RouterAgentTest / Linux x86_64 / Go 1.24.13 / GCC 14.2。当前源码复制到 `/work-runs/server-reconnect-fix-20260910`，Linux 完整业务测试使用独立 network/pid/mount/devpts namespace，未挂载用户工作区。

| 检查 | 命令与结果 |
| --- | --- |
| 旧代码复现 | `go test ./internal/gateway -run TestOrphan -count=1`：新用例实际收到 ERROR/INVALID_PAYLOAD/task not found 而失败；保留其他断言后实现修复，再运行通过 |
| Windows 全量 | `go test ./cmd/... ./internal/... ./tests/... -count=1`、`go vet ./cmd/... ./internal/... ./tests/...`：通过；Windows 下依赖 Linux Probe 的用例跳过，其实际证据以下列 Linux 运行结果为准 |
| Windows 成品 | `go build -trimpath -o build/server-reconnect-fix/router-server.exe ./cmd/server`：通过；独立 loopback 56585/56586 和 `build/server-reconnect-fix/smoke-repository` 启动该 EXE，以 TCP 测试对端注册、补报未知任务并发送心跳，收到正常 ACK；公开 API 确认任务数为 0、同一 Session 仍在线。结束后仅停止该测试进程，日志 `windows-smoke-server.log` |
| Linux Release 全量 | 隔离环境 `sh tests/verify-phase5.sh release`：构建、14/14 CTest、全量 Go（含真实 Probe）、vet 及 Linux Server 构建通过；integration 244.278s，包含新增 Server 重启场景 |
| Go race | 同一隔离源码/namespace下，`RMP_PROBE_BIN=$PWD/build/phase5-probe/router-probe go test -race ./cmd/... ./internal/... ./tests/... -count=1`：全量通过，integration 246.736s，包含真实Probe与本轮重启回归 |
| C++ ASan/UBSan/TSan | 本次尝试 `sh tests/verify-phase5.sh asan` 与 `race`，CMake 编译器链接检查失败：缺 `libasan_preinit.o`、`-lasan`、`-lubsan` 和 `libtsan_preinit.o`、`-ltsan`。未通过，未升级环境依赖；Probe C++ 源码未改 |

日志与 Windows 成品位于 `build/server-reconnect-fix/`，已复制 Linux 的 release/go-race/asan/tsan 日志；Linux 验证产物只用于 x86_64 测试，不是路由器 ARM 部署包。专项链接检查及本轮文件 `git diff --check` 通过。Windows冒烟日志最后的DISCONNECTED来自测试完成后主动关闭测试socket，不是旧结果触发断线。

## 部署边界

现场设备 Probe、用户本机 Server/桌面进程和运行数据保留。正常关闭当前 Server 后，重新运行仓库根目录 `server-windows.cmd`，入口会从修复后的源码构建并按现有 9000/8888 配置启动；修复在加载新构建后生效，无需重启或升级路由器 Probe。也可使用上述独立成品配合现有完整启动参数，不能把无参数双击 EXE 当作现有配置启动。

旧任务历史仍不会跨 Server 重启恢复，原 Probe 进程内缓存仍保持有界且不淘汰。此次未替换生产 Server，未在真实路由器上主动重启服务验收，未执行 Git 提交或推送。
