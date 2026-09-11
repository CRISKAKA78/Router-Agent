# 项目接管手册

## 当前接管：EasyTier ARM—Server 实机联调

- 组网 worktree 已快进到 main `bab2325`，主目录未提交的双架构构建改动未动；本轮修复未提交、合回或推送。
- 用户已明确授权仓库上传、ARM—Server测试及Management重启，覆盖前次不部署的任务边界。ARM ID为 `FE7140555489`，最新指定SSH入口20001；20004暂不测试。
- ARM/MIPS/mipsel core入库为包集合 `2.6.4-r1`；Management接入本机Web。双向ICMP、TCP/UDP echo、管理停机期间30/30 ping及MTU样本复测通过，见[实机证据、回退和遗留](OVERLAY_LIVE_VERIFICATION.md)。
- 上传完成判断、异步启动只读等待修复已有Windows定向回归/vet/build与Linux五包race。原加入记录仍uncertain：上游空路由回读丢失开关，不强制确认；实际网络保持运行。
- Web回环绑定/默认密码更换待用户确认；二层、中继、双路由器、长期稳定性及WPF交互未追加验收。接线见[组网文档](OVERLAY_NETWORK.md)。

## 前次接管：异地组网已合入本地main（历史记录）

- 本次用户明确授权审核并合入 `router-agent-overlay-network`，来源提交 `da6c560`，目标为包含 AT、智能邻居、日志及 GOST PoC 的 `main`；不推送、不部署、不替换运行实例。
- 本次验证：Windows Go/vet、自包含发布、WPF617项/81份布局通过；Linux18项CTest、完整真实Probe集成、五包race及最终AT/组网真实Probe定向race/vet通过。详细命令和剪贴板复跑记录见专项文档。
- 入口：[异地组网集成审核](OVERLAY_INTEGRATION.md)、[使用与部署接线](OVERLAY_NETWORK.md)。EasyTier ADR 原分支059统一为064；既有059～063含义不变。
- WPF 增加全局“异地组网”（合并日志后为第七工作区），入口不依赖设备上报；添加设备要求 `network_agent_v1`、在线、已纳管及 Server 配置服务接线。新包位于 `build/windows-desktop-overlaymerged/win-x64/RouterWorkbench.exe`。
- 回查确认配置版本及移除已确认停止成员的审核缺陷已修复；保留原操作ID/任务ID，不重新执行不确定操作。验证结果以专项记录为准。
- 真实设备三层互通、引擎仓库包/服务账号接线及二层仍未完成；GOST仍是PoC，不能混为本轮EasyTier产品功能。

## 前次接管：AT、智能邻居、日志与 GOST 集成

1. 用户已明确授权三工作树和 AT 前置智能邻居合入本地 main，旧“只读/不提交/不写原目录”属于历史任务边界。本次不推送、不部署、不改产品功能语义；联合验证已通过，集成结果已合回本地main；三个来源工作树与集成分支保留。
2. 先读[本次集成验证](WORKTREE_INTEGRATION.md)。入口为[智能邻居](NEIGHBOR_SMART_CONFIGURATION.md)、[AT](CELLULAR_AT.md)、[日志](DEVICE_LOGS_VERIFICATION.md)、[GOST](GOST_V3_POC.md)；原工作树与其忽略的运行证据保留，发布目录不迁移、不提交。
3. ADR-057/058保留main含义；智能邻居059、AT060、日志061、GOST提案062/鉴权063。编号映射见[DECISIONS](DECISIONS.md)，不重新推翻已确认设计。
4. 新WPF成品为`build/windows-desktop-integrated/win-x64/RouterWorkbench.exe`，本机Server测试产物在该目录同级verification内；原运行程序未替换。Probe仅做WSL Linux集成构建，不运行远端GCC或上传/安装。
5. GOST源码/补丁全部保留但不接入既有Maintenance/RMT1；用户另行测试，不能因合入main标记完整准入或产品功能上线。其压力/预算/硬件缺口仍以专项报告为准。

## 此前主分支记录（历史事实，非本次验证）
2026-09-12 异地组网首轮改造位于 `codex/overlay-network-research` 独立 worktree（ADR-064）：EasyTier 本机 Web API、持久网络操作、Probe 仓库安装/独立启动、WPF 第六工作区与真实观测拓扑已接入；不是生产/厂商互通验收。Windows 567 项检查、自包含发布及 Linux 16 项 CTest、完整 Go/integration、五包 race/vet 已通过，证据与限制见[专项记录](OVERLAY_NETWORK.md)。当前缺上游运行配置服务/账号、仓库兼容ELF包及本轮厂商构建，未替换生产Server/Probe；二层在三层实机通过后继续。无提交/推送。

2026-09-11 接管更新：用户确认迁移后的维护连接问题已解决；排查证据为 Probe 到 47.119.168.150:9001 的 SYN 未到达服务器网卡，未确认具体网络规则及修复方式。Server 已实际常驻、Probe 在线；不要将下文短时部署记录当成当前运行状态。本轮将既有迁移改动整理进本地 main 并删除迁移分支，不推送远端，提交和分支结果以 Git 为准；凭据、构建包和运行数据仍留本机。

2026-09-11 Probe最新入口为ADR-058：双击probe-build.cmd即可在10.1.1.128通过账号密码构建，password.txt由AskPass本地读取；不要改成47.119.168.150编译或使用私钥。产物/root/router-agent/router-agent；成功run为20260911-182649-f30c50be，构建/ELF通过，未替换设备Probe。完整规则及证据见[部署§5.3](DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)。

2026-09-11当前入口为ADR-057与[默认服务器验证](SERVER_DEFAULTS_VERIFICATION.md)。Linux一键构建上传用`server-linux-amd64.cmd`，远端启动`/root/agent-server/start.sh`；当前仅上传并做短时验证，未常驻启动。Windows成品`build/windows-desktop-serverdefaults/win-x64/RouterWorkbench.exe`，新配置默认47.119.168.150:8888，旧保存地址保留；不要恢复SSH凭据配置。Probe构建按ADR-058已恢复默认root@10.1.1.128，password.txt自动登录，成品/root/router-agent/router-agent；接口默认br0,eth0,eth1,usb0。凭据保持本地且Git忽略，无提交推送；下文为历史入口。

2026-09-10 用户已明确授权将当前累计源码、测试和文档提交并推送到 GitHub `CRISKAKA78/Router-Agent` 的 `main`。本次仅整理提交：核对远端基线、文件范围和 `git diff --check`，不重跑全量产品测试；构建包、运行数据和本地配置留在本机。下文各轮“未提交/推送”为当时记录，当前提交号与推送结果以 Git 为准；ARM、sanitizer 和实机验收缺口保持。

2026-09-10 本轮入口为[邻居发现](NEIGHBOR_DISCOVERY.md)与ADR-056：用户明确第二类为本机广播域全部记录，不是上级；不能固化FNR100的LAN1接线。LAN按配置转发端口证据筛选，和broadcast可以重叠。新增各层neighbors模块、Probe Neighbors、WPF NeighborView与生成器NeighborEditor；协议/API使用既有传输与幂等。配套Windows成品、557项WPF/67份布局及其他测试证据、通用模板示例见专项说明。厂商ARM首轮交互认证已进入GCC5.2编译，暴露旧uClibc不导出`std::snprintf`；`scripts/build-probe-gcc52.sh`现仅在远端副本增加`<stdio.h>`并改用`::snprintf`，本地转换/编译/15项CTest通过，真实GCC5.2仍需交互密码复跑。当前生产实例未替换。此前Server重连修复与ADR-053～055展示改动保留，不回退已有工作区，也未提交/推送。

2026-09-10 最新后端修复入口：[Server重连修复验证](SERVER_RECONNECT_FIX_VERIFICATION.md)。Gateway仅在通用RESULT校验通过且Task Service返回ErrTaskNotFound时记录并继续，不发送ERROR、不导入旧任务；其他结果校验保持。不要为处理旧补报清空Probe缓存或恢复跨进程任务。新增gateway孤立结果回归与真实Probe Server实例重建回归，Windows/Linux全量、Go race、14项CTest及隔离EXE通过，C++sanitizer仍缺库。成品 `build/server-reconnect-fix/router-server.exe`，生产Server仍为旧进程；正常关闭后运行 `server-windows.cmd`，无需重启Probe。详细证据见专项记录，无提交推送。桌面入口继续下方ADR-055。

2026-09-10 最新桌面入口为**ADR-055 + ADR-053/054**。本轮四页采用[方案1](design/compact-workspace-target.png)，详情页方案3保持；先读[紧凑工作区验证](COMPACT_WORKSPACE_VERIFICATION.md)和[Design QA](../design-qa.md)。CompactWorkspace复用工具栏/原生历史折叠；DeviceDirectoryView发选择和加载通知；MaintenanceViews复核入口可用性；配置结果绑定TaskDetail参数；ToolWorkspace选择后显示版本。551项回归、62份布局及发布通过，启动`build/windows-desktop-compact/win-x64/RouterWorkbench.exe`。最终原生鼠标/截图复验受桌面会话限制，实际WPF渲染已比较；旧进程/数据保留，无提交推送。下文“最新”为历史轮次。

2026-09-10 最新桌面入口为 **ADR-053/054**。先读[组件视觉规范](COMPONENT_VISUAL_SPEC.md)、[唯一方案3原图](design/property-inspector-target.png)与根目录[Design QA](../design-qa.md)。共享模板位于Themes/Inputs、Menus、Controls；WorkbenchIcon使用Fluent原始几何，默认尺寸必须放依赖属性元数据，避免覆盖模板。Typography统一字体/行距；正文根节点使用Text资源，不能继承选中Tab蓝色。保留PropertyInspectorWorkspace原页面选择、顺序、原始值复制和业务。519项检查、54份布局、16份离屏密度及发布通过；启动 `build/windows-desktop-components/win-x64/RouterWorkbench.exe`，无需重启后端。上一轮组件QA结论已撤回并重新比较；物理多屏DPI与完整读屏未验收，无Git提交/推送。

2026-09-10 最新修复入口：[快照刷新修复验证](SNAPSHOT_REFRESH_FIX_VERIFICATION.md)。Client/Models.cs的DeviceSession.StartedAt/LastSeenAt须保持可空；后端零时间合法返回null，不能改回必填或伪造日期。481项检查与现有后端实际快照同步通过。启动`build/windows-desktop-snapshotfix/win-x64/RouterWorkbench.exe`即可使用，后端无需重启；当前用户实例未替换，未提交/推送。UI仍为ADR-052。

2026-09-09 最新桌面入口为 ADR-052，先读[系统信息层级验证](SYSTEM_INFORMATION_HIERARCHY_VERIFICATION.md)。DeviceSummary/SystemOverview共享SummaryBlock/FlexibleSummaryPanel测量和分配空间；PropertySheet负责四类卡片、完整换行与顺序，Themes定义二级Tab状态。模板显式顺序、原属性对象/操作及业务调用保持。473项检查、47份布局和独立发布通过；成品 `build/windows-desktop-hierarchy/win-x64/RouterWorkbench.exe`。原客户端正常关闭后启动新包；用户实例/配置/数据保留，无提交/推送。物理输入/DPI、厂商设备和发布EXE手工启停限制见专项记录；下文“最新”为历史轮次。

2026-09-09 最新桌面基线为 ADR-049/050，先读[外观与交互验证](APPEARANCE_INTERACTION_VERIFICATION.md)。Themes/Ui 统一居中与状态样式；Typography/SettingsView 保存并应用字体，InputFeedback 区分键盘焦点，TableBehavior 测量字号变化后的自动列宽。连续 PropertySheet、模板排序/过滤和业务链路保持。340 项检查、41 份布局及自包含发布通过；成品 `build/windows-desktop-fonts/win-x64/RouterWorkbench.exe`。用户实例/配置未替换，无提交/推送；物理输入/DPI与发布 EXE 直接启停限制见专项记录。下方“最新”为历史轮次。

2026-09-09最新界面基线为ADR-048，先读[接口工作区验证](INTERFACE_WORKSPACE_VERIFICATION.md)。MainWindow/TelemetryViews负责source_ip摘要及异步归属；DeviceViews收纳接口状态子分组，SamplingView仅构建弹窗并经原profile API保存。306项桌面检查、32份布局及自包含发布通过；最新客户端`build/windows-desktop-interfaces/win-x64/RouterWorkbench.exe`。本机公网归属实查超时，不记作真实解析成功；其余实机限制见专项文档。未替换用户运行实例/数据。本轮源码、测试及文档纳入当前进度提交，按用户追加授权同步GitHub的origin/main；提交号与远端状态通过Git核对，构建包和运行数据留在本机。

2026-09-09已按当前用户授权将累积产品改造和ADR-047提交并推送至`origin/main`（`CRISKAKA78/Router-Agent`），源码提交`810de06be5b145c2a08504fd66589906eff5d405`已由远端查询确认。运行数据、构建包及本地配置未上传；随后文档提交仅同步此事实。下方各轮“未提交/推送”为历史说明，后续接管以Git记录核对最新状态，不推断实机已验收。

2026-09-09最新产品基线为ADR-047，十项改造与本机验证已完成。先读[客户工作区验证](CUSTOMER_WORKSPACE_VERIFICATION.md)：Client FileExchange/RemoteDirectory编排公开API，WPF DeviceDirectoryView/ToolWorkspace负责交互，PresentationEditor/DisplayLayout负责稳定分类与storage_visible。WPF280、生成器172、浏览器13及Windows/Linux相关Go验证通过；客户端`build/windows-desktop-customer/win-x64/RouterWorkbench.exe`。用户运行实例未替换；正常重启Server/生成器加载源码，模板改动须发布后显式应用。管理员上传仍后续实施，真实固件/物理DPI/发布EXE启动限制见专项文档。无Git提交/推送。

2026-09-09 最新启动修复为ADR-046：先读[启动修复与验证](SERVER_STARTUP_VERIFICATION.md)。probetemplate/enrollment加载存量目录时处理退役元数据并保留原字节备份；模板文件缺失可告警建空库。不要删除devices/catalog.json或放宽API旧字段校验。Windows/Linux相关测试与当前数据副本EXE启动发布通过；原数据和进程未动，未提交/推送。产品工作区基线仍为下方ADR-045。

2026-09-09当前基线为ADR-045。改造已实现，入口/契约/验证/产物见[DEVICE_WORKSPACE_VERIFICATION](DEVICE_WORKSPACE_VERIFICATION.md)。WPF248、生成器170、浏览器12、C++14及Windows/Linux Go全量/race/vet通过；只改接口不重跑其他采集，主动应用代次控制同版重新应用。最新独立客户端为`build/windows-desktop-workspace/win-x64/RouterWorkbench.exe`。

后续先读ADR-045与API/PROTOCOL最新节。ARM编译受10.1.1.128 SSH认证限制，公网出口在当前网络未取得成功值，C++ sanitizer缺库；保留现有用户进程/设备Probe与运行数据，未Git提交/推送。不得把Linux x86_64验证包当作ARM成品，或把本机TLS测试当作真实路由器公网验证。

## 上一轮接管入口

2026-09-09当前入口为ADR-044：DisplayLayout/PresentationEditor负责可见性；WPF TableBehavior/PropertyGroups负责单行/手动换行、像素滚动和整行折叠；Server保存presentation，Probe只走当前CONFIG_APPLY/EVENT。旧格式和接口映射已删除，REGISTER不再携带模板结果。

完成状态、命令、程序和限制见 [UI_REFINEMENT_VERIFICATION](UI_REFINEMENT_VERIFICATION.md)：WPF224、生成器169、浏览器12、C++13及真实Linux Release/race、Windows Go/vet通过。未动用户进程或数据，未Git提交/推送。新版独立EXE为`build/windows-desktop-refined/win-x64/RouterWorkbench.exe`；ARM/物理DPI仍需实机验收，sanitizer缺库。该轮接管依据ADR-044，不恢复旧兼容入口。

## 此前接管记录

下文旧版本/旧产物保留历史意义；当前基线和授权以上文为准。

2026-09-09 ADR-043：先读[PHYSICAL_PORT_MONITORING](PHYSICAL_PORT_MONITORING.md)。新增Probe `port_counters.cpp`，SystemSampler拥有进程级基线；模板counters→Gateway能力检查→switch数值指标→WPF网口页/PortRatePanel；生成器PhysicalPortProfiles提供FNR100预设，工程7。

代码、本机验证和Windows产物已完成，实机升级待编译机认证。Telnet 192.168.5.222已授权，原Probe `/tmp/root/router-probe --server pcv6.criskaka.com:9000`，用户允许备份/停止/覆盖/重启。`build/physical-ports/template-candidate.json`保留原7属性，不能覆盖其他设备的通用模板；先取得10.1.1.128 GCC5.2构建认证，再配套升级并验证。当前运行进程尚未替换；状态/证据及剩余验收见专项文档，无Git提交/推送授权。

2026-09-09 ADR-042接管入口：生成器ConfigurationView/PresentationEditor/SwitchEditor与EditorLayout共享展示数据；独立配置、属性归组和预览已完成。port可省略贯通生成器工程6、Go probetemplate和Probe switch_probe；使用该格式需配套新版Probe。先阅读 [TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md) 最新节和 [验证记录](TEMPLATE_CONFIGURATION_VERIFICATION.md)。

164项生成器、10组浏览器、12组C++及完整Linux Go race/Windows Go通过；修正原API慢消费者测试只读两条缓冲通知的假设并验证。C++sanitizer缺库、ARM及厂商实机仍未验收。未提交/推送，保留用户已有改动、数据与进程。用户已授权本轮实现，无需再次询问；以下为历史入口。


2026-09-09 ADR-041七项改造已完成代码与本机验证。新增 `internal/enrollment`（持久发现/纳管/型号/期望配置）→ Application → Gateway CONFIG_APPLY/ACK → Probe LiveTelemetry；WPF ManagedViews负责待纳管/配置/分组/物理口，Blazor PresentationEditor/ModelMappingEditor负责模板展示与型号目录。规范及实际用法见 [MANAGED_PROBES_DESIGN](MANAGED_PROBES_DESIGN.md)。

接管须知：新发现包含旧安装无档案设备均先pending；CLI不再选择模板。先在生成器发布并设置型号默认模板，再由WPF纳管或显式应用版本，发布不自动更新设备。管理员资料和注册事实分离；CPU只采两次，业务能力在Server复核纳管。12项C++、完整Linux/race、WPF195项、生成器158项/9组浏览器通过，见 [MANAGED_PROBES_VERIFICATION](MANAGED_PROBES_VERIFICATION.md)。

剩余为新版ARM/uClibc构建与厂商逐口验收、C++sanitizer缺库；本轮SSH免交互认证失败。下方旧产物/旧配置语义仅保留为历史，不能替代本轮验证。用户已有改动与运行数据保留，未Git提交/推送；无需重复索取七项实现授权。

2026-09-09 ARM构建入口已修复CRLF导致的 `set: pipefail` 错误：`probe-build.ps1` 在上传解包后规范化脚本行末CR，`scripts/build-probe-gcc52.sh` 保持LF。真实GCC5.2构建成功，默认接口 `eth0,eth1,br0`；远端 latest-build 为 `runs/20260909-011116-bc8fcaa1`，本地成品/日志在 `build/arm-gcc52-20260909/`。这解决了此前SSH认证和ARM成品缺口，未代替厂商运行验收；见 [DEPLOYMENT §5.3](DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)。

2026-09-09 ADR-040入口：Probe collection/telemetry → Gateway telemetry_v2 → Device effective_metrics → WPF DeviceProperties/TelemetryViews/NetworkRateWindow；模板工程4兼容1/2/3，构建入口支持接口白名单。source_ip保持TCP对端，egress_ipv4/ipv6为设备探测；统计基线在Probe进程内跨控制重连保留。最终验证与产物、浏览器/SSH/出口实网/实机缺口见 [MONITORING_V2_VERIFICATION](MONITORING_V2_VERIFICATION.md)。

生成器入口已修复 MSB3026：`template-generator.ps1` 先识别同仓库端口所属实例并复用，再按需构建到 `build/template-generator/port-<端口>`；BuildOnly 单独输出。加载源码更新需先在原启动窗口 Ctrl+C 停止，再启动；不自动结束用户进程。启动专项验证及用法见 [TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md#启动脚本文件锁修复)。

2026-09-08 ADR-039入口：Probe telemetry.cpp → Gateway EVENT → Device.Telemetry/effective_metrics → API/WPF；生成器monitoring与逐属性interval_seconds、工程3兼容1/2。CPU/内存/网口默认5秒、存储60秒，CLI优先且配置重启生效；模板失败不回退，新Session周期属性重采。来源IP为Server TCP对端，WPF仅查询公网归属地。实现和自动检查已完成，ARM编译SSH认证、浏览器启动审核拒绝、外部归属地超时及厂商验收等缺口见 [TELEMETRY_VERIFICATION](TELEMETRY_VERIFICATION.md)。

2026-09-08 新增 `probe-build.cmd` → `probe-build.ps1` → `scripts/build-probe-gcc52.sh`：上传当前 Probe 到 10.1.1.128，以 `/root/gcc-5.2` 编译；兼容处理只在远端副本，产物与记录全部位于 `/root/codex-probe-20260908-2123`。真实 Windows → SSH 编译及 ELF 检查通过；使用与证据见 [DEPLOYMENT §5.3](DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)，不代表固件验收。

2026-09-08 当前基线：只保留新版 C# / WPF 主 UI 与 C# / Blazor / Fluent UI 探针模板生成器（ADR-037/036/035/034）。旧 React、WinUI、Win32/WebView2 UI 及专属脚本、Bridge/ConPTY 已移出源码；不要依照旧阶段文档恢复它们。此前清理提交为 `bdf9f9b`；本次 ADR-038 没有 Git 提交/推送授权。

此前系统信息（ADR-038）：Probe `system_info.cpp` / `collection.cpp`；Gateway parseHeartbeat → Device.Heartbeat → API runtime；WPF DeviceProperties / DeviceViews。默认内核，显式模板优先；时长注册后首报、每心跳重采，未知/离线/新会话隔离，年月日时分秒省略前导空单位（年365日/月30日）。发布包及本次验证见 [SYSTEM_INFO_VERIFICATION](SYSTEM_INFO_VERIFICATION.md)，历史 Phase 文档的“默认 kernel 缺失”不再是当前行为。

依次阅读 AGENTS → 本文件 → [PROJECT_STATUS](PROJECT_STATUS.md) → ARCHITECTURE → ROADMAP → 相关 API/PROTOCOL/DECISIONS，再读 [DEVELOPMENT](DEVELOPMENT.md) 并核对当前代码、Git 和验证结果。普通需求自主完成范围内实现、验证和文档交付，沿用 ADR-028，不进入新阶段。

| 当前入口 | 接管位置 |
| --- | --- |
| `ui-windows.cmd` / `windows/build-desktop.ps1` | Desktop 为 WPF UI，Client 为公开 API/WS，Core 为配置与外部启动；[WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md) |
| `template-generator.cmd` / `template-generator.ps1` | `src/ProbeTemplateGenerator`；编辑状态 EditorWorkspace，公式 ExpressionParser/TemplateCompiler，发布 TemplatePublishingService；[TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md) |
| `server-windows.cmd` / `server-windows.ps1` | Go Server，持久数据 `data/server/repository`；[DEPLOYMENT](DEPLOYMENT.md) |
| 当前验证 | Desktop.Tests 已自带 TestProbe，生成器 C# 测试位于 tests/ProbeTemplateGenerator.Tests，Playwright 仅在 tests/package.json；命令见 DEVELOPMENT |

此前旧 UI 清理验证：98 项 WPF/当前 Go Server/协议对端检查、145 项 C# 生成器测试（含实际 WSL BusyBox）、发布目录 Edge/Go API 8 组流程、两套 Release 发布与 Windows Go test/vet 通过，见 [UI_CLEANUP](UI_CLEANUP.md)。旧源码归档在 Git 忽略的 `build/ui-cleanup`；旧发布包、用户进程、草稿与运行数据保留，不推送到 GitHub。

主 UI 导航为设备/待纳管/维护/文件/配置/设置；设置保存服务器，启动自动连接。维护使用公共 Web/SSH/Telnet 外部入口，SSH 默认 admin/admin 可修改并本机加密保存；无内置终端、通用任务或客户工具管理。registration保留启动快照，实时属性使用Server合并结果；监控WS提示加五秒回查，稳定行保留选择。维护重开 409 与连接状态/闪烁修复已含在当前 98 项检查中。

保留公开 API/WS、原幂等键和字节、切换/退出取消并等待；文件 committed/released 与 Task RESULT 分开，维护默认 240 分钟与固定 Probe 127.0.0.1:80/22/23、独立数据 TCP、默认 24 小时端口隔离保持。Probe/Server 的模板及配置任务证据在 PROBE_TEMPLATES_VERIFICATION / ROUTER_CONFIG_VERIFICATION，不把测试对端当厂商实机。

**真实遗留：此前 Agent 清理测试挂载误删原工作区。源码从 f73853f 加补丁恢复并重新验证；`cmd/server/1.txt` 与 `cmd/server/data/` 未恢复，需要备份来源。`build/windows-react` 曾从 09:13:52 卷影副本恢复，不是新版产物。详情见 [PROBE_TEMPLATES_VERIFICATION](PROBE_TEMPLATES_VERIFICATION.md)，不得隐去或把空数据目录当作恢复。**

Phase 0～5 已验收。当前 WPF/Blazor 的厂商固件、SSH/Telnet、物理 DPI/鼠标/选择器/剪贴板及干净目标机，Phase 6 最终产品验收仍待完成；C++ sanitizer 既有缺运行库问题保持。Linux 复用项目外 `RouterAgentTest`，见 [WSL_TEST_ENVIRONMENT](WSL_TEST_ENVIRONMENT.md)。旧 Win32/ConPTY 的未完成验收保留为历史记录，已无当前旧 UI 实现。

Phase 7/8、微信小程序、正式公网 Web、新 Tunnel 和大规模架构扩展暂缓；管理员工具入口及认证/TLS/RBAC 等尚未建设或未决。完成用户当前需求后交付并停止；未来提交/推送仍按当次明确授权。
