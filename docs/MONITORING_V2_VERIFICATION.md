# ADR-040 验证与交付

2026-09-09，用户审核计划后授权实施。在已有未提交的 ADR-038/039 与构建脚本基础上修改，未提交/推送 Git，未覆盖运行数据或用户进程。

## 已完成

- Probe 0.2.0 默认 `nvram get softver`，全文固件、首个 v 前型号；模板分别覆盖且失败不回退。设备侧双栈出口、默认600秒独立采集；接口白名单在采集报文前过滤，构建/模板/CLI配置。每接口累计收发字节、统计秒数，控制重连保留采样基线。
- telemetry_v2 能力协商、原因字段、出口两协议独立更新、整网口组替换；公开 API 转交 Device Service 最新指标。source_ip 仍为 TCP 对端，不能代替缺失的另一协议出口。
- WPF 名称/ID/绿在线灰离线列表、8项所选设备摘要、无空格时长、表头/值居中、横杠加原因悬停；属性不拼来源/时间，删除重复按钮与说明但保留菜单功能。双击接口打开10分钟收发曲线、流量与时长；离线/失败断点、关闭释放。
- 生成器工程4，兼容1/2/3、草稿与原运行模板；接口及出口周期保留在复制、导入/导出、编译和发布中。

## 验证证据

Windows 使用仓库 .NET 10 SDK 和 Go；Linux 为项目外 WSL2 `RouterAgentTest`，当前源码复制到 `/work-runs/adr040-20260909`。真实维护回归在独立 network/mount/PID/devpts namespace，无工作区 bind mount。

| 检查 | 命令与结果 |
| --- | --- |
| C++ | `cmake --build build/phase5-probe -j4`、`ctest --test-dir build/phase5-probe --output-on-failure`：10/10通过，包括固件模板优先、接口缺失/过滤、计数回退、双栈命令、v1降级。最终记录在 Linux `race-final.log`。 |
| 完整 Linux Release | `sh tests/verify-phase5.sh release`：10项CTest、所有Go包、真实Probe集成201.870秒、vet和Server构建通过；`release-final.log`。首次新测试误用1秒心跳，修正为协议要求的10秒后重跑通过。 |
| 最终 Go race | 最终Probe重新构建及10项CTest；`RMP_PROBE_BIN=... go test -race ./cmd/... ./internal/... ./tests/... -count=1` 全部通过，真实集成208.017秒，后续 `go vet ./...` 与Server构建通过；`race-final.log`。包含独立IPv4/IPv6失败清旧值、API来源分离、CLI覆盖模板、真实lo接口计数及普通重连不重置时长。 |
| Windows Go | `go test ./...`、`go vet ./...`、`go build -o build/adr040/router-server.exe ./cmd/server`通过；`build/adr040-windows-go.log`。真实Linux用例以Linux记录为准。 |
| 生成器 | `RMP_GENERATOR_WSL=RouterAgentTest`，`dotnet test ProbeTemplateGenerator.sln -c Release --artifacts-path build/adr040/dotnet`：154通过，0跳过；含真实WSL BusyBox命令。自包含Release发布通过。日志 `build/adr040-generator-tests.log` / `adr040-generator-publish.log`。 |
| WPF | `windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-adr040`：166项通过；`build/adr040-wpf.log`。覆盖HTTP/WS实际Go对端、图表样本/断点/保留/释放、字段顺序、流量/时长、查询缓存/IPv6解析/失败/取消、维护/文件/配置全流程。 |
| WPF布局 | `python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-adr040/verification`：18份实际WPF布局，人工查看设备属性/网口/浅深图表；包含新增图表。不等于物理DPI与鼠标实机验收。 |
| 构建默认接口 | Linux另建 `build/filter-default`，配置 `-DRMP_NETWORK_INTERFACES=lo`；实际Probe通过20-byte协议测试对端注册，不传 `--network-interfaces`，网络报文7项均为lo。`filter-build.log`。PowerShell脚本AST语法检查通过。该环境没有python3，单次协议验证使用已有Go运行，无安装依赖。 |
| 公网查询 | Windows `curl.exe` 实连 `ipwho.is/8.8.8.8` 返回美国/加利福尼亚州/聖荷西/Google LLC。出口IPv4 ipify实连TLS握手失败，IPv6端点连接超时；不能视为设备双栈出口实网验收通过。 |

后续ARM构建已于2026-09-09完成：修复上传脚本CRLF后，使用用户提供的认证通过真实GCC5.2编译，默认接口eth0,eth1,br0，成品373112字节；构建日志、换行回归与ELF证据见 [DEPLOYMENT §5.3](DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)。

## 未完成项

- 自动审批审核拒绝启动隔离 Blazor 验证实例，只返回 `blocked by policy`，没有具体原因；本轮浏览器交互/发布流程没有运行。脚本已增加接口/出口周期断言，不能用历史浏览器结果替代。
- CMake ASan/UBSan仍缺 `libasan_preinit.o/-lasan/-lubsan`，TSan缺 `libtsan_preinit.o/-ltsan`；当前环境无法执行，Go race不替代C++ sanitizers。
- 厂商路由器的双栈出口、HTTPS工具/证书、NVRAM固件输出、物理网口计数及实际鼠标/DPI尚待实机验收。此前数据恢复遗留与Phase6最终验收不改变。

## 产物

- Windows主客户端：`build/windows-desktop-adr040/win-x64/RouterWorkbench.exe`。
- ARMv7/uClibc Probe：`build/arm-gcc52-20260909/router-probe`，2026-09-09后续修复构建，默认接口eth0,eth1,br0。
- Windows Server：`build/adr040/router-server.exe`。
- 生成器：`build/adr040/generator/ProbeTemplateGenerator.exe`及同目录依赖。
- Linux x86_64 Probe：`build/adr040/router-probe-linux-x86_64`；Server：`build/adr040/router-server-linux-x86_64`，均不适用于ARM/MIPS路由器。

配置和升级见 [DEPLOYMENT](DEPLOYMENT.md#接口过滤与双栈出口adr-040)，规范见 [TELEMETRY_DESIGN](TELEMETRY_DESIGN.md#adr-040-双栈出口与流量统计)。
