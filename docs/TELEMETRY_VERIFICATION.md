# ADR-039 监控与连接来源验证

2026-09-08。用户确认详细CPU识别、独立周期监控、模板优先方案，并追加WPF来源IP和公网归属地。本次在已有未提交ADR-038和交叉编译脚本改动之上实现，保留既有内容；没有Git提交/推送授权。

## 已实现

- C++11原生CPU/架构/频率、内存、文件系统空间和接口计数采集；硬件识别依据内核暴露事实，识别不了的字段为unknown，AArch64未知代际标为ARMv8+，不把64位硬件、内核运行架构和Probe位数混用。
- 内置四组独立周期、CLI优先、模板逐属性周期；模板worker可取消且公平轮转，心跳与原控制业务保持。能力协商后用EVENT，旧Server或小于8KiB控制上限不发送监控并记录原因。
- Device Service统一模板同键优先，错误不回退，未知清除旧值，Session隔离，最新样本与age_ms；API及devices WS通知。原registration/工具匹配不被周期采集改写。
- 生成器工程3兼容1/2、monitoring与属性interval_seconds分别编辑/导入/导出/发布；配置及模板版本重启Probe生效。
- WPF CPU详情、容量自动单位、存储与网口表格、稳定行、离线/过期状态；Server TCP来源IP，公网归属地HTTPS查询、缓存、非公网过滤、失败退避和取消。

## 环境与证据

Windows：Go、仓库build/dotnet10的.NET 10 SDK；Linux：项目外WSL2 RouterAgentTest，当前源码复制到`/work-runs/telemetry-20260908`。无工作区bind mount；完整维护回归使用独立network/mount/PID/devpts namespace。

| 检查 | 命令/证据与结果 |
| --- | --- |
| C++采集及回归 | `cmake --build build/phase5-probe -j4`、`ctest --test-dir build/phase5-probe --output-on-failure`：9/9通过。新增telemetry_tests覆盖ARMv8硬件/32位内核、ARMv7、频率未知、CPU差分/回退、内存GB与旧内核估算、网口差分/移除、真实statvfs与挂载过滤、独立配置。 |
| 首轮完整Linux Phase1～5 | `sh tests/verify-phase5.sh release`：C++、全部Go包、真实Probe集成192.520秒、vet及Server构建通过；日志`release.log`。 |
| 完整Go race | Release Probe + `go test -race ./cmd/... ./internal/... ./tests/... -count=1`：全部通过，真实集成196.406秒；`race.log`。不等同C++TSan。 |
| 最终变更专项 | 当前源码重编C++/9项CTest；`RMP_PROBE_BIN=... go test -race ./internal/device ./internal/gateway ./internal/api ./internal/probetemplate ./tests/integration -run 'TestTelemetry\|TestMonitoring\|TestSystemInfoRealProbeAPI' -count=1`及vet/Server构建，证据`final-check.log`。涵盖真实Probe模板覆盖、慢命令时独立采样、数值失败、注册快照不变、重连、API来源IP。 |
| Windows Go | `go test ./...`、`go vet ./...`、`go build -o build/telemetry/router-server.exe ./cmd/server`通过；`build/telemetry/windows-go-test.log`。真实Linux用例以Linux日志为准。 |
| 生成器 | 设置`RMP_GENERATOR_WSL=RouterAgentTest`；`dotnet test ProbeTemplateGenerator.sln -c Release --artifacts-path build/telemetry/dotnet`：148项通过，0跳过，含BusyBox。独立目录自包含发布通过；`build/telemetry/generator-tests.log`和`generator-publish.log`。 |
| WPF | `windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-telemetry`：153项通过，含真实Go HTTP/WS、监控表格数值/稳定行、CPU详情和来源IP、IP分类/查询/缓存/429/取消替身。`build/telemetry/wpf.log`。 |
| WPF布局 | `python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-telemetry/verification`：16份实际WPF矢量布局，包含浅深设备/存储/网口页；不是厂商实机或物理DPI验收。 |

## 未完成验证与环境限制

- 本次临时Blazor生成器服务启动及浏览器回归命令被自动审批审核拒绝，仅返回`blocked by policy`，没有具体原因；未改用其他方式绕过。浏览器脚本已补周期保存/发布断言，但本轮未执行，不以此前8组浏览器流程冒充本次通过。用户正在运行的旧生成器PID24828导致默认输出锁定，因此构建改到独立artifacts目录，未停止该进程。
- GCC5.2/uClibc编译机`root@10.1.1.128`的BatchMode SSH返回`Permission denied (publickey,password)`。本轮ARM交叉编译尚未执行，之前280908字节ARM包是旧版本；不能作为本次新Probe产物。
- 本机HTTPS实连`https://ipwho.is/8.8.8.8?...`超时，未证明本机网络中真实归属地成功；备用服务可达性探测也超时，产品没有加入未验证备用服务。查询解析、地址过滤及失败/取消行为由HTTP替身通过；UI在此环境如实显示未获取。
- CMake ASan/UBSan检查仍缺`libasan_preinit.o`、`-lasan`、`-lubsan`；未执行sanitizer。C++TSan既有缺库限制仍在。本次不将Go race当作C++sanitizer通过。
- 厂商ARM/MIPS固件、物理网口流量/外接存储及硬件型号/频率对照待目标设备验收；未改变既有Phase6最终验收状态或数据恢复遗留。

## 产物

- Windows主客户端：`build/windows-desktop-telemetry/win-x64/RouterWorkbench.exe`。
- Windows Server：`build/telemetry/router-server.exe`。
- 独立生成器：`build/telemetry/generator/ProbeTemplateGenerator.exe`及同目录依赖。
- Linux x86_64 Probe/Server：`build/telemetry/router-probe-linux-x86_64`、`build/telemetry/router-server-linux-x86_64`；不用于ARM/MIPS设备。

使用见[DEPLOYMENT](DEPLOYMENT.md#独立周期监控adr-039)，契约见[TELEMETRY_DESIGN](TELEMETRY_DESIGN.md)。运行数据、旧发布包和用户进程保持。
