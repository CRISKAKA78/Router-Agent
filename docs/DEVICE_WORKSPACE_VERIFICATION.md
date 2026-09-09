# 设备工作区改造（ADR-045）

2026-09-09，已完成代码、跨层本机验证和Windows发布。厂商ARM部署、当前网络的公网出口成功探测及C++ sanitizer仍有下述限制，不计为已通过。

## 完成条件

- [x] 属性列按全部实际内容适配，手动列宽与阅读位置保持；双栈摘要分行。
- [x] 系统信息/资源监控/模板分组/接口信息/其他信息并列，存储和连接历史保留专页；默认/动态属性自由归组。
- [x] 内存百分比附实际已用/总容量。
- [x] 接口设置唯一设备采样覆盖，其他计划保持；显式模板应用与同版本重新应用闭环。
- [x] 所选设备模板版本/更新提示、右键复制/改名/更新；待纳管移入左侧。
- [x] 连续在线和随后离线时长，Session替换、客户端重连与历史边界正确。
- [x] Probe原生HTTPS双栈出口，在空PATH环境使用真实TLS对端验证；保持证书验证。
- [x] WPF/生成器/浏览器/Windows Go/真实Linux/Go race/C++及Windows发布验证。
- [ ] GCC5.2/uClibc ARM配套构建和厂商实机部署；公网双栈成功探测；C++ sanitizer环境验证。

## 实现入口

- `shared/DevicePresentation.cs`共享内置目录/默认分组；WPF `PropertyGroups`、`DeviceViews`、`TableBehavior`负责并行页、全内容列宽、顺序、可见性和长行阅读。接口详情默认展开，滚动容器保持曲线高度。
- WPF `DeviceActions`负责右键和模板确认，`ManagedViews`只提供接口动态设置及纳管；`TelemetryViews`组合内存实际容量。所选设备保留实际确认版本，新版检测不伪装成已应用。
- `internal/management/enrollment.go`限制设备覆盖并维护应用代次；`internal/device/connections.go`在Device锁内维护连续周期；`internal/api/connections.go`提供公开HTTP快照。现有Session历史接口不移除。
- Probe `live_config.cpp`/`telemetry.cpp`区分接口重配和重新应用，静态缓存按应用代次区分；`egress.cpp`/`tls_config.h`提供原生HTTPS和静态Mbed TLS。GCC构建脚本增加C编译器与随包许可证。

## 本轮验证

环境：Windows x64，.NET10.0.400、Go1.25.5、Node24.12.0、Edge；WSL RouterAgentTest为GCC14.2/musl、Go1.24.13。Linux隔离源码位于`/work-runs/device-workspace-20260909`，完整测试使用独立network/pid/mount/devpts namespace，不挂载用户工作区。

| 范围 | 命令与结果 |
| --- | --- |
| Windows Go | `go test ./cmd/... ./internal/... ./tests/... -count=1`、`go vet ./cmd/... ./internal/... ./tests/...`及Server构建通过；Windows不能执行Linux Probe的用例不作为真实Probe证据 |
| Linux全量 | `RMP_PROBE_BIN=$PWD/build/probe/router-probe go test ./cmd/... ./internal/... ./tests/... -count=1`通过，integration为231.134s；`go vet`、Linux Server构建通过。`full-tests-final.log`、`vet.log` |
| Go race | 同隔离环境`go test -race ./cmd/... ./internal/... ./tests/... -count=1`通过，integration为233.495s。`race-final.log` |
| C++ | 最终`cmake --build build/probe -j4`、`ctest --test-dir build/probe --output-on-failure`：14/14通过；包含所有内置信任根解析、HTTP分帧、静态缓存/同版重应用/接口重配。`ctest-final.log` |
| 原生HTTPS | 最终`TestNativeEgressHTTPS`使用真实Go TLS测试服务和C++原生客户端，空PATH，8个场景通过（含Go race）：IPv4、IPv6 chunked、跨族地址拒绝、错误证书主机名、超限响应、超时、取消、证书到达前连接关闭。`native-egress-final.log` |
| 最终Probe回归 | 保留现有采集互斥锁保护revision，避免新增32位平台64位atomic依赖；最终二进制在Go race下重跑原生HTTPS、受管配置/重连和物理端口专项，通过，6.185s。`final-probe-regression.log` |
| WPF | `.NET10 run --project windows/RouterWorkbench.Desktop.Tests -c Release -- <本轮Server.exe> <隔离输出>`：248项通过。含分组顺序、列宽/手动换行/像素滚动、内存容量、实际API模板更新/重应用/改名/离线设置、连续周期与Session替换、原文件/维护/重连流程。`wpf-release.log` |
| WPF布局 | `python windows/RouterWorkbench.Desktop.Tests/render.py build/device-workspace/wpf`：26份真实WPF XPS转PNG通过；检查浅/深主题、1000/1480/1920窗口、资源页/存储/接口曲线/连接历史/确认框。短确认框按实际内容验证，不用长页面字数判定。不是物理屏幕/DPI/触控板验收 |
| 生成器 | `RMP_GENERATOR_WSL=RouterAgentTest`下`.NET10 test ProbeTemplateGenerator.sln -c Release`：170通过、0跳过；包含内置/自定义/动态字段归组及导出往返。`generator-final.log` |
| 浏览器 | 本轮隔离5199端口Blazor + 真实Go API，`node tests/template-generator-browser.mjs`：12组通过，无浏览器错误；覆盖归组、预览、编辑、导出/发布/冲突及原公式/规则。`browser-tests.log` |
| 发布 | `windows/build-desktop.ps1 -BuildOnly -OutputName windows-desktop-workspace`成功生成自包含EXE；`template-generator.ps1 -BuildOnly`通过，0警告/错误。未替换用户运行实例 |

证据根为`build/device-workspace`，Linux日志复制到其`linux`子目录。首次旧测试仍依赖curl模拟出口和任意设备覆盖而失败，已改为接口专用/模板应用断言与真实原生HTTPS测试，保留原失败日志。公网补测发现精简TLS遗漏SHA-384，已修复并补生产根证书解析回归；提前EOF的无验证结果不会再被误报为证书拒绝。

文档相对链接检查和`git diff --check`通过；本轮未进行Git提交或推送。

## 产物与实际限制

- 自包含Windows客户端：`build/windows-desktop-workspace/win-x64/RouterWorkbench.exe`；Server：`build/device-workspace/router-server.exe`。生成器仍使用`template-generator.cmd`，源代码及本机构建已更新。
- Linux Server/Probe在`build/device-workspace/linux`，仅为musl x86_64验证产物，不是ARM部署包。Probe动态依赖仅原有libstdc++/libgcc/libc，无TLS共享库；TLS静态链接，许可证随产物保留。
- `ssh -o BatchMode=yes -o StrictHostKeyChecking=yes root@10.1.1.128`返回Permission denied，无法执行GCC5.2/uClibc交叉编译。本轮未替换192.168.5.222现有Probe，也未将x86_64验证包作为设备成品。
- 当前网络实测api.ipify.org在TLS证书到达前关闭（EOF，正确显示tls_handshake_failed）；api6.ipify.org为connect_failed。没有公网出口成功值可验收，未跳过证书校验或使用source_ip冒充出口；本地双栈HTTPS成功与失败场景已有真实对端证据。
- 已尝试C++ ASan/UBSan和TSan，CMake链接检查缺libasan_preinit.o/libtsan_preinit.o及对应运行库；未通过，日志保留。Go race已实际通过。
- 连接周期仍为Server进程内有界历史，服务重启后不补写未知区间；用户实机、物理DPI及正式Phase6验收不因本轮自动完成。

## 初始工作区

HEAD为bdf9f9b，已有188项已修改或未跟踪路径；本轮保留这些既有改动、运行数据及用户进程。历史验证属于ADR-044，不能替代本轮结果。
