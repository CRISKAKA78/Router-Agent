# 通用 AT 身份采集与串口自动发现（ADR-060）

## 2026-09-11 实机测试追加授权

用户已授权使用47.119.168.150:20004的SSH设备测试，并在10.1.1.128以桌面gcc-5.4.tar.gz编译，产物放/root下独立目录；新增同目录password.txt免输入密码的一键脚本。不提交或推送密码，不更改现有GCC5.2入口。已完成GCC5.4/MIPS小端/uClibc编译及FM160-CN实机ATI/IMEI，自动选中ttyUSB1，真实占用跳过/释放恢复与API关闭清空通过。原Probe/拨号实例保留，临时测试实例已停止；本节取代下方首轮“尚未获得部署授权/物理模组均未验证”的当前含义。物理模组重启/重编号仍未测试。脚本、产物、真实结果及密码文件用法见[GCC54实机说明](GCC54_AT_DEVICE_VERIFICATION.md)。

## 首轮交付边界

2026-09-11 用户明确要求另建 worktree，先不适配厂家，打通 ATI、IMEI 和串口自动识别。工作目录为 `C:\Users\Administrator\Desktop\router-agent-at-discovery`，分支 `codex/at-discovery`，起点 `7671b5f44ab4b727167f29b27a1a6716e63054c3`。原目录、其他 worktree、用户运行实例及本地敏感文件均未改动。本轮未提交、推送或部署。收尾只读核对发现原工作目录已由外部工作切换到 `main` / `f331ffb8c05fd73fbfb5a186508b476f45c4020d`；本分支仍基于上述创建时起点，未自动合并、rebase或改动其他worktree。

已实现：模板启用 → CONFIG_APPLY/ACK → Probe 后台串口采集 → EVENT → Server 校验/最新快照 → HTTP → WPF 蜂窝模块属性检查器。没有增加任意 AT 控制台、手动触发 Task、SIM/驻网/信号、厂商解析器、数据库或新 Phase。

“通用”指本轮使用固定身份查询而非按厂家分支，**不表示所有模块和固件已实测通过**。ATI 保留文本，不自动据此认定厂家/制式；IMEI 查询接受严格 15 位数字，不把任意序列号或 ATI 内的数字当作 IMEI。

## 使用步骤

1. 使用本分支构建的 Server、Probe 和桌面客户端。旧 Probe 缺少 `cellular_identity_v1` 时，含 AT 配置的模板应用明确报 `unsupported_cellular`，不静默部分应用。
2. 在模板生成器“配置 → 蜂窝模块 AT”启用“启用AT自动探测”，默认周期 30 秒，有效范围 10～86400 秒。无需填写厂家、串口或命令。既有模板省略该配置，默认关闭。
3. 可直接导入 [最小运行模板](../examples/templates/cellular-auto.json)，也可在现有模板上启用后发布。导入、导出、保存或发布本身不会向设备发 AT；只有显式应用模板到设备后才启动。
4. 在设备详情选择“蜂窝模块”。选择自动发现的端口，查看 ATI、IMEI、实际查询命令、USB 设备路径、当前串口、占用/错误原因、周期、修订与观测时间。沿用现有属性检索、原始值复制和完整值面板。
5. “刷新快照”只回查 Server，不立即向模块发命令。关闭模板开关并重新应用后，停止采集、释放串口并清空当前身份快照。

```json
{"name":"通用 AT 自动探测","properties":{},"cellular_probe":{"interval_seconds":30}}
```

工程仍为格式 8、草稿 3。运行模板 `cellular_probe:{}` 默认 30 秒；只支持 `interval_seconds`，未知字段、null 周期和越界值拒绝。设备级网络采样覆盖仍只控制网络，不覆盖 AT 周期。

## 自动发现与租用

- 每轮重新枚举 `/sys/class/tty` 中的 `ttyUSB*`、`ttyACM*`，必须通过真实 USB 父设备 `idVendor/idProduct` 和设备节点 major/minor 一致性检查。不固定 VID/PID、厂家、USB 接口号或 tty 编号。
- 排除板载 ttyS/控制台、非 TTY QMI/MBIM；USB 接口描述明确包含 GPS/GNSS/NMEA/diag/qdss/debug/download 或为 DM 时跳过。没有描述的诊断口不能保证预先识别，后续握手失败会如实报告。
- 同一 USB 父设备分组，一个组选择一个最佳 AT 端口：ATI 与 IMEI 均成功优先，其次部分成功；完整成功后不继续查询其兄弟端口。不同 USB 父设备独立展示。顶层 ok 表示被选中端口的身份完整，仍须查看其他组/端口的占用或失败状态。
- 查询前扫描 `/proc/*/fd` 的设备元数据，不读取进程参数或数据。已占用不打开、不发送；权限或扫描完整性不足也拒绝尝试。上限 4096 个进程/65536 个句柄。
- 短时打开串口、复核节点身份、检查既有独占状态，获取 flock/TIOCEXCL 后再次检查占用。所有退出路径恢复 termios 并关闭自己的句柄。保持原波特率及调制解调器控制标志，不扫描波特率、不主动改 DTR/RTS/HUPCL、不停止拨号程序。
- `device_key` 为当前 USB 父设备的 sysfs 拓扑路径，ttyUSB2 变 ttyUSB9 而父路径不变时仍归为同一设备。它不是永久资产 ID：换 USB 插口、总线重排或更换模块后可以变化。不按旧 IMEI 拼接结果，不永久拉黑失败串口。
- 无可用端口时返回 no_ports/unavailable/error，下一周期再试。默认每轮结束后等待 30 秒；轮内查询总预算 15 秒，所以不是严格每 30 秒必有新结果。最多 16 个有效候选，枚举上限 4096 条；预算未覆盖端口下轮轮转，超过候选上限标 limited，不声称完整遍历所有设备。
- 该机制不能保证固件一定暴露一个空闲 AT 口。已有占用检查与独占锁也无法排除不协作的特权进程随后打开；驱动打开/关闭串口的硬件行为仍需目标机验收。

## 固定串行查询

| 顺序 | 命令 | 处理 |
|---|---|---|
| 1 | `AT` | 验证应答，失败则不执行后续身份命令 |
| 2 | `ATI` | 保留非空、多行身份文本，记录独立状态 |
| 3 | `AT+CGSN` | 首选 IMEI 查询 |
| 4 | `AT+CGSN=1` | 前一条明确拒绝或响应不是有效 IMEI 时回退 |
| 5 | `AT+GSN` | 同上，最后一次固定回退 |

不发送 ATE0、ATZ、CFUN、拨号、切卡或写入 IMEI 等命令。不自动启用 unsolicited 上报。支持回显、分段、多行和常见 URC 过滤；预查询要求至少 100ms 安静，等待最多 500ms。单命令 1500ms 超时，接收最多 4096 字节、有效文本最多 1024 字节。超时、I/O、二进制/格式错误或溢出后终止当前查询链，避免将迟到应答解释为下一条响应。取消和等待按短周期检查。

IMEI 支持裸 15 位数字以及已识别的 +CGSN/+GSN/IMEI 前缀和引号；不执行校验位真实性或厂商数据库认证。无 SIM 是否仍可查询、具体返回格式和指令支持最终以硬件验证为准。

## 协议与 API

规范分别见 [PROTOCOL](PROTOCOL.md)、[API](API.md)。新增能力 `cellular_identity_v1`，复用已有配置、EVENT、HTTP 快照同步，无新帧或 WebSocket topic。

- EVENT 完整替换本轮结果，携带 config_revision、interval_seconds、age_ms、status、reason、limited、ports。每个端口有自身 age_ms，ATI/IMEI 分别携带 command/status/value。
- Gateway 限制 64KiB、16 个端口、合法路径/查询/状态及严格 IMEI；拒绝 Probe 注入 Server 派生的采样时间。Device Service 校验当前 Session、已应用 revision/周期，旧事件不串到新配置。仅网络修订重绑定已有结果时保留原始年龄，不伪装为刚采样。
- Server 根据接收时间减年龄计算 sampled_at，不要求路由器时钟准确。离线、顶层或选中端口超过三倍周期则 stale。重新建立 Session、应用/关闭模板清空旧身份快照；不持久化身份历史。
- `GET /api/v1/devices/{id}/cellular` 返回标准 envelope 内 `data.snapshot`，没有结果时 null；GET 不发 AT。
- Device DTO 增加 cellular，纳管设备 DTO 增加 cellular_configuration（已应用配置），模板摘要增加 cellular_probe。现有 HTTP/WS 失效后回查机制不变。
- ATI、IMEI 在当前 API 和桌面按原值可见；本轮未新增遮罩、RBAC 或 TLS。Probe 正常日志不写完整 AT 响应。请按现有可信管理环境边界使用，不把此功能部署描述成新增隐私隔离能力。

## 代码入口

- Probe：[cellular.cpp](../probe/src/cellular.cpp)、[cellular.h](../probe/include/rmp/cellular.h)，由 TelemetryCollector 管理有界后台线程；只合并待发送最新结果，不阻塞心跳等待串口。
- Server：[网关校验](../internal/gateway/cellular.go)、[Device Service](../internal/device/cellular.go)、[只读 API](../internal/api/cellular.go)、[模板配置](../internal/probetemplate/cellular.go)。HTTP 不直接操作 Gateway 或表。
- 生成器：[CellularEditor](../src/ProbeTemplateGenerator/Features/Attributes/CellularEditor.razor)、[模型](../src/ProbeTemplateGenerator/Models/CellularSettings.cs)，沿用编译/发布/导入导出和能力门槛。
- WPF：[CellularView](../windows/RouterWorkbench.Desktop/CellularView.cs)、[公开 DTO](../windows/RouterWorkbench.Client/Cellular.cs)，沿用属性检查器，不实现串口控制逻辑。
- 自动化：[原生 PTY 测试](../probe/tests/cellular_tests.cpp)、[真实 Probe 集成](../tests/integration/cellular_linux_test.go)、[生成器测试](../tests/ProbeTemplateGenerator.Tests/CellularTests.cs)、[WPF 检查](../windows/RouterWorkbench.Desktop.Tests/CellularChecks.cs)。

## 本轮验证（2026-09-11）

环境：Windows amd64 Go1.25.5、.NET SDK10.0.400；专用 WSL RouterAgentTest 的 Alpine/GCC14.2/Go1.24.13。Linux 源码副本 `/work-runs/at-discovery-95b74326`，未替换设备上的 Probe。

- Windows `go test ./cmd/... ./internal/... ./tests/... -count=1`、`go vet ./cmd/... ./internal/... ./tests/...` 通过。Windows 原生 Probe 集成按平台条件跳过，不能替代下述 Linux 结果。
- Linux CMake build、CTest **16/16** 通过；覆盖占用跳过、ATI 分段/回显/URC、IMEI 回退、错误与取消、同 USB 多端口、多个 USB、重新编号和预算轮转。
- 真实 Probe 可执行文件 + Server + HTTP，使用子进程独占 mount namespace/PTY：已验证忙端口不发送、ttyUSB2→ttyUSB9 自动重识别、心跳和 Session 保持、端口后来被占用时停止查询并在释放后恢复、关闭模板后超过一个周期没有新命令。属于真实程序和模拟串口闭环，不是物理模块测试。
- Linux 全量最终通过：`RMP_PROBE_BIN=$PWD/build/probe/router-probe go test ./cmd/... ./internal/... ./tests/... -count=1`（实际Probe集成291.374秒），同范围vet及Linux Server build通过；`go test -race ./internal/... ./tests/integration -run "TestCellular|TestDevice|TestConfiguration|TestTelemetry|TestNeighbors" -count=1` 通过，属于定向race而非全套race。首次全量仅既有 TestNativeNeighborDiscovery 因隔离net namespace未重新挂载sysfs失败，读取到宿主接口；未修改或降低邻居断言，加入本命名空间 `mount -t sysfs sysfs /sys` 后全量复跑通过。
- 生成器单元/WSL 编译测试 **196 通过、0 跳过**（`RMP_GENERATOR_WSL=RouterAgentTest`）。TRX：`build/cellular-verification/cellular-generator.trx`。
- Windows 桌面自包含发布、**582 项**检查、**72 份实际 WPF 矢量布局**渲染通过，包含 AT 浅色/深色、窄窗占用状态及重编号/过期/清空。证据在 `build/windows-desktop-cellular/verification`；布局不包含原生标题栏，不代替鼠标输入/物理 DPI 验收。
- `tests/template-generator-browser.mjs` 新增 AT 配置宽窄/主题/校验/发布/重导入/关闭用例；`node --check` 通过。真实浏览器启动命令被执行策略拒绝，未运行新用例，不以单元测试冒充浏览器通过，也未换工具绕过限制。
- 本轮最小 `c++ -fsanitize=address,undefined` 链接检查仍缺 `libasan_preinit.o/-lasan/-lubsan`，未运行 sanitizer。ARM/mipsel/uClibc/GCC5.2 交叉构建、真实 USB 插拔/拨号重启共存、多厂家固件尚未验收。

### 可复现入口与日志

- Windows Go日志：`build/cellular-verification/windows-go.log`；生成器：设置 `RMP_GENERATOR_WSL=RouterAgentTest` 后运行 `.NET10 SDK dotnet test tests/ProbeTemplateGenerator.Tests -c Release`。
- WPF发布/检查入口：`windows/build-desktop.ps1 -BuildOnly -OutputName windows-desktop-cellular -Dotnet <.NET10SDK绝对路径>`；本轮最终检查运行 `dotnet run --project windows/RouterWorkbench.Desktop.Tests -c Release -- build/windows-desktop-cellular/verification/router-server.exe build/windows-desktop-cellular/verification`。日志 `build/cellular-verification/desktop-final.log`；`python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-cellular/verification` 渲染72份布局。
- Linux源副本先CMake配置/构建Probe，随后在独立 `unshare -mnpf --mount-proc` 中设 `mount --make-rprivate /`、启用lo、重新挂载sysfs及独立devpts再运行测试。AT真实Probe测试只在其子mount namespace绑定临时sysfs/dev，不改变生产Probe文件路径、不提供测试专用CLI/环境注入。
- 原生构建/CTest/full Go/race及sanitizer检查日志复制到 `build/cellular-verification/linux`。运行环境副本路径记录在 `build/at-test-environment.txt`。构建文件不纳入提交。

本轮 `git diff --check`、Go格式检查、最小模板JSON解析及9份变更Markdown的233个本地链接目标检查通过。

## 产物与下一步

- Windows 桌面：`build/windows-desktop-cellular/win-x64/RouterWorkbench.exe`。
- Windows Server：`build/windows-desktop-cellular/verification/router-server.exe`。
- Linux x86_64 测试 Probe：WSL 副本 `build/probe/router-probe`，**不可直接当作 ARM/mipsel 设备固件**。
- 本轮说明、三个状态入口、ADR、协议/API 与 Changelog 一起维护；构建输出不提交 Git。

首轮规划（现已部分由上方实机验证完成）：仅在明确目标设备/部署授权后：用目标工具链编译 Probe，确认 USB/sysfs/proc 权限，应用最小模板，核验真实 ATI/IMEI；在用户允许的维护窗口重启拨号/模组，观察串口编号变化、无拨号中断和自动恢复。随后再按厂家实际响应扩展 SIM 与驻网信息，不能把当前完成项等同于 RSRP/RSRQ/RSSI/SINR/TAC/ARFCN/PCI/CELLID/BAND 已实现。
