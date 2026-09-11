# 全工作树源码集成

本轮用户明确要求“把全部分支树改动提交并并入主分支，如有冲突帮我解决，最后同步推送github”。仅为源码整合、验证及Git交付；不部署、不重启生产Server/Probe，也不清理工作树、构建产物或运行数据。本机日志日期为2026-09-12（UTC+08）。

## 来源与范围

起始main/GitHub main均为`eb83e245ccab82179f67680588aee521659fb224`。共6棵工作树、10个本地分支；开始时没有分支独有的未合入提交，但三处工作区存在未提交源码。

| 来源 | 保存提交 | 本轮原始改动 |
| --- | --- | --- |
| main主工作区 | `c1a1127` | 组网成员配置、批量操作、恢复和WPF，34个文件 |
| codex/at-module-adaptation | `963a8b3` | FM160遥测/24项详情、模板能力门禁和趋势，42个文件 |
| codex/gostv3-device-poc | `54907b4` | Forwarding服务/API、Probe侧车、GOST/PTY测试和WPF，46个文件 |

AT合并提交为`ce40faf`；最终GOST合并/验证提交及推送状态以Git记录为准。其他AT发现、设备日志、旧组网及集成/计划分支已是main祖先，无需制造空提交。各来源分支与工作树保留，既有提交不重写；未推送独立功能分支，只将完整历史随main交付。

## 冲突与兼容处理

- main的组网ADR-066不变；AT来源066/067统一为067/068，修正正文、中文相邻引用和对应链接。GOST后续已获确认的产品接入补充登记为ADR-069，局部取代062/063的PoC-only限制，保留旧决定及来源提交。
- Management同时组合日志、组网、维护与Forwarding服务，初始化失败及关闭时释放对应已创建资源。保留EasyTier启动配置，并增加原分支Forwarding参数，不覆盖旧服务器默认值。
- Gateway会话同时保留日志请求和Forwarding有界队列；Probe注册能力取并集，forwarding_v1仍仅在侧车可执行时声明。设备日志与Forwarding的控制处理参数和两处读取调用均保留。
- API的HTTP路由不互相覆盖；WS默认及显式订阅同时保留networks和forwardings，并新增实际WebSocket握手/通知回归；缺GOST时基础网络查询仍正常。
- WPF保留设备详情、远程维护、内网穿透、串口透传、文件管理、配置管理、日志、仓库工具、异地组网九页，保留刷新、未知响应重试和能力门禁。九页窄窗口文本导航及三类新增页面实际WPF截图已核对。
- 凭据、build、运行数据不进入提交；GOST源提交仅修正一个Python文件尾部多余空行，功能不变。未升级分支之外的新依赖或恢复旧UI。

## 本轮验证

环境：Windows amd64 Go 1.25.5、.NET SDK 10.0.400；WSL Linux amd64 Go 1.24.13、Alpine GCC 14.2.0。

日志统一在主工作区忽略目录`build/integration-all-worktrees/`。Linux使用独立WSL `RouterAgentTest`，复制源码到`/work-runs/all-worktrees-20260912-0625`；所有真实Probe/GOST/EasyTier测试使用独立network/PID/mount/devpts namespace，私有sysfs，不bind mount Windows工作区。复测只同步两项新增/加强集成测试和资源就绪测试，产品源码与此前构建相同。

| 检查 | 命令/日志 | 实际结果 |
| --- | --- | --- |
| Windows Go | `go test ./cmd/... ./internal/... ./tests/... -count=1`；`go vet ./cmd/... ./internal/... ./tests/...`；`go build -o build/integration-all-worktrees/router-server.exe ./cmd/server` | 通过；go-windows-test.log / go-windows-vet.log |
| 合并回归 | `go test ./internal/api -run 'TestMergedNetworkAndForwardingEventTopics\|TestForwardingPublicUnavailable' -count=1 -v` | 通过；merge-regression.log；末次API/integration测试与vet另见windows-final-regression.log / go-windows-vet-final.log |
| WPF完整发布 | `powershell -NoProfile -File windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-allmerged` | 669项通过，自包含发布成功；wpf.log |
| 生成器 | `template-generator.ps1 -BuildOnly`；设置`RMP_GENERATOR_WSL=RouterAgentTest`后`build/dotnet10/dotnet.exe test ProbeTemplateGenerator.sln -c Release` | 构建通过，198项通过/0跳过；generator-build.log / generator-test.log |
| Linux C++ | namespace内`sh tests/verify-phase5.sh release`的CMake/CTest阶段 | 完整Probe构建及18项CTest通过；linux-release.log |
| Linux最终全量 | 设置本轮真实Probe/侧车/GOST路径与`RMP_FORWARDING_TEST_NET=1`后，`go test ./cmd/... ./internal/... ./tests/... -count=1`、完整vet、Server build | 全部通过，集成包386.638秒；完整vet/Server build通过；linux-final.log |
| Go并发与真实链路 | `go test -race ./cmd/... ./internal/... -count=1`；相同隔离环境与后端变量，`go test -race ./tests/integration -run 'TestForwardingProduct\|TestCellular\|TestNetworkAgent' -count=1 -v -timeout 180s` | 通过；linux-race.log。真实C++ Probe身份/v2/详情及端口重编号、日志共存、GOST TCP/UDP/串口注册PTY、组网断线回放，不是厂商硬件验收 |
| 官方EasyTier | 设置官方2.6.4二进制目录和新建测试种子DB，`sh tests/verify-overlay-official.sh` | 真实Web/core、机器ID及配置往返通过；official-easytier.log。仅复用既有官方测试二进制，不访问生产 |
| Windows GOST租期 | 设置实际GOST EXE及`RMP_FORWARDING_LONG_TESTS=1`，`go test ./internal/forwarding -count=1 -v -timeout 100s` | TCP/UDP/串口和真实60秒到期通过；gost-windows.log |
| 重连资源收敛 | 缺侧车3轮、实际侧车5轮，`go test ./tests/integration -run TestProbeReconnectResourceConvergence -count=N -v` | 全部通过；reconnect-regression.log；每轮8次重连，严格原有FD/线程上限、临时文件和未发布目标断言保持 |
| C++ ASan/UBSan | 隔离环境`sh tests/verify-phase5.sh asan` | 配置失败：缺libasan_preinit.o、libasan、libubsan；linux-asan.log。不宣称sanitizer通过、不自行安装库 |

### 首次失败与收口

首轮Linux完整Go回归仅`TestProbeReconnectResourceConvergence`失败，记录FD=8/6，线程12/12。新侧车的管道初始化异步发生，旧用例在worker启动前后混合计数；后续会话也可能在fork临时描述符关闭前测量。用例现在在每个被测会话发送真实inventory请求并验证同一session的回复后取样；不使用固定sleep或增加容忍阈值，不修改产品行为。无侧车路径保持原行为并独立重复验证。完整末次复测结果以上表为准，原失败日志保留。

### 本机产物与限制

- WPF：`build/windows-desktop-allmerged/win-x64/RouterWorkbench.exe`；未自动启动或替换用户原实例。截图在同目录verification，包括九页导航、蜂窝、穿透和组网；属于实际WPF离屏输出，不是现场多屏DPI验收。
- Windows Server：`build/integration-all-worktrees/router-server.exe`。Linux Server/Probe/新侧车在上述独立WSL源码副本build目录；GOST使用来源工作树已构建的匹配版本，未重新打包第三方后端或向设备投放。
- 本轮没有运行新合并源码的厂商双GCC远程构建/设备升级，没有执行新的生成器浏览器流程。来源轮次的实机证据保持，但不替代本轮整合后的硬件验收。
- 既有UDP压力问题、RSS暂停门槛、物理UART、不同网关LAN、双设备组网现场恢复及长稳仍按各专项文档保留；ASan/UBSan缺库为当前环境限制。Phase6最终产品验收与Phase7/8状态不变。

## Git交付

用户已授权提交和推送。最终交付检查为：所有本地分支均为main祖先、6棵工作树没有待提交源码，以及`git ls-remote --heads origin`的main与本地HEAD相同；结果以本轮最终提交和工具核对为准。推送使用普通快进，不强制推送，不删除分支，不包含凭据或构建数据。
