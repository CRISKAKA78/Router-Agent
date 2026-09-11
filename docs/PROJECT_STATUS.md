# 项目状态

## 当前：ADR-057智能邻居配置已实现，产品验收有明确缺口

工作分支核对为 `GPT6API-TEST`。生成器智能/高级配置、公开能力协商与只读网络检测、显式FNR100预设、WPF默认直连扫描和Server1024条/24小时近期视图已接通；主动响应60秒后转近期，不推断上级、不把历史说成在线。仅邻居配置模板的应用校验遗漏已修复。没有新阶段、数据库或Tunnel改造。

本轮Windows Go/vet、生成器189项（0跳过）、WPF572项/69份布局与独立构建通过；最终Linux15项CTest、真实Probe Phase1～5全量/vet/build及核心包/邻居集成race通过。浏览器完整交互、物理DPI、ARM/uClibc/FNR100实机及缺库sanitizer未验收，不沿用旧测试冒充。详见[实施、验证、产物与限制](NEIGHBOR_SMART_CONFIGURATION.md)。未提交/推送/部署，用户Server/Probe进程和数据未替换。

## 此前记录（非本轮授权与验证）

历史整理任务（不构成本轮授权）：2026-09-10 用户已明确授权将当时累计源码、测试和文档提交并推送到 GitHub `CRISKAKA78/Router-Agent` 的 `main`。本次仅整理提交：核对远端基线、文件范围和 `git diff --check`，不重跑全量产品测试；构建包、运行数据和本地配置留在本机。下文各轮“未提交/推送”为当时记录，当前提交号与推送结果以 Git 为准；ARM、sanitizer 和实机验收缺口保持。

2026-09-10 **ADR-056邻居发现已实现**：LAN下接与本机广播域两份清单允许重叠，已按用户纠正取消上级分类；Probe被动读取/限速ARP扫描及取消、Server公开API、WPF两页、生成器配置/发布闭环已接通。Windows/Linux Go、真实Linux Probe完整回归/race、15项CTest、生成器176项/浏览器14组、WPF557项/67份布局及独立发布通过记录见[邻居发现](NEIGHBOR_DISCOVERY.md)。成品为 `build/neighbors/router-server.exe` 和 `build/windows-desktop-neighbors/win-x64/RouterWorkbench.exe`。ARM交互认证后的首轮GCC5.2构建在`std::snprintf`处失败；一键构建的远端副本适配已补为`::snprintf`并通过脚本语法、转换结果、适配后核心编译及原始Probe 15项CTest，真实GCC5.2复跑仍需交互密码。C++sanitizer仍缺库；FNR100未部署新Probe，保留用户生产进程/数据，无Git提交/推送。下方“最新”为历史轮次。

2026-09-10 **Server 重启后 Probe 循环断线已修复**：合法未知 TASK_RESULT 记日志并忽略，保留心跳/新任务，未知任务不重建；只需更新 Server，Probe 保持。旧代码回归已复现 `task not found`。Windows/Linux全量、真实Linux Probe跨Server实例重建及再次重连、Go race、14项CTest与隔离Windows EXE通过；C++sanitizer仍缺库，见[验证](SERVER_RECONNECT_FIX_VERIFICATION.md)。成品 `build/server-reconnect-fix/router-server.exe`；当前生产进程/数据未替换，正常关闭Server后重开 `server-windows.cmd` 生效。未提交/推送。

2026-09-10 **ADR-055紧凑操作台已实现**：本轮方案1覆盖维护、文件、配置、仓库工具；详情页继续ADR-053方案3，组件继续ADR-054。551项桌面回归、62份WPF布局、自包含发布及最终渲染联合对照通过。[验证与限制](COMPACT_WORKSPACE_VERIFICATION.md)、[Design QA](../design-qa.md)。程序`build/windows-desktop-compact/win-x64/RouterWorkbench.exe`已启动并只读同步现有1台在线设备；最终原生鼠标/截图复验被桌面会话错误阻止，未冒充通过。未修改后端或替换旧用户实例，无提交/推送。以下保留各轮历史记录。

2026-09-10 当前桌面视觉基线为 **ADR-053方案3 + ADR-054字体/共享控件校准**：默认微软雅黑UI，统一输入/按钮/下拉/复选/列表/菜单/提示/密码与滚动条，Fluent矢量替代文字和手绘图标；修复模板尺寸被覆盖、正文继承导航蓝色、字重与行距及设备ID宽度。原结构、信息顺序和业务保持。**519项桌面检查、54份WPF布局、16份离屏密度渲染、自包含发布通过**；实际窗口和组件联合对照见根目录[Design QA](../design-qa.md)，规范见[组件视觉规格](COMPONENT_VISUAL_SPEC.md)。成品 `build/windows-desktop-components/win-x64/RouterWorkbench.exe`。上一轮组件视觉通过结论已撤回并重新验收；物理多屏DPI仍未验收。未提交/推送，下方旧产物为历史记录。

2026-09-10 修复会话空时间导致的快照刷新失败：C# Session开始/最后活动时间允许null，与现有API一致。481项桌面检查及独立发布通过，并只读连接用户现有后端确认快照已同步、读到1台设备。最新修复程序为`build/windows-desktop-snapshotfix/win-x64/RouterWorkbench.exe`；ADR-052界面保持，未替换运行实例/数据，未提交推送。[验证记录](SNAPSHOT_REFRESH_FIX_VERIFICATION.md)。

2026-09-09 当前桌面基线为 ADR-052：顶部六项弹性摘要、有边界的二级Tab、系统信息首屏概览与四类语义卡片已完成，长值按真实空间展开/换行。**473项检查、47份WPF布局及自包含发布通过**，见[系统信息层级验证](SYSTEM_INFORMATION_HIERARCHY_VERIFICATION.md)。成品 `build/windows-desktop-hierarchy/win-x64/RouterWorkbench.exe`；整体布局、页面、模板规则和业务/通信保持。用户进程与数据未替换，未提交/推送；物理输入、多屏DPI、厂商设备及发布EXE手工启停仍未验收。ADR-051其余列表/主导航/输出行为保留，下文为此前各轮事实。

2026-09-09 当前桌面视觉基线为 ADR-049/050：Workbench 连续 Inspector 保持，所有表格列居中；设置支持保存字体和字号，修复悬停/首单元格焦点混用选中外观及设备右键异常。340 项桌面检查、41 份 WPF 布局和自包含发布通过，成品 `build/windows-desktop-fonts/win-x64/RouterWorkbench.exe`，见[外观与交互验证](APPEARANCE_INTERACTION_VERIFICATION.md)。五个工作区、模板规则和业务链路保持；物理鼠标/DPI及发布 EXE 直接启停未验收。用户进程/配置保留，未提交或推送。以下为此前各轮事实。

2026-09-09 ADR-048已实现：所选设备拆分出口IP/运营商及归属地，仅以Server的source_ip显示并解析；接口状态包含外壳/系统端口，末尾同排接口采样时间打开弹窗；导航改为设备详情、远程维护、文件管理、配置管理。桌面306项检查、32份WPF布局及自包含发布通过，详见[本轮验证](INTERFACE_WORKSPACE_VERIFICATION.md)。成品为`build/windows-desktop-interfaces/win-x64/RouterWorkbench.exe`；公网归属服务本机实查超时，厂商实机/物理DPI及发布EXE直接启停未验收。未替换用户进程或运行数据。本轮源码、测试及文档纳入当前进度提交；用户已追加授权推送至GitHub的origin/main，构建产物与运行数据不上传。

2026-09-09当前项目源码已按用户授权上传GitHub：`CRISKAKA78/Router-Agent`的main已确认包含源码提交`810de06be5b145c2a08504fd66589906eff5d405`，包含累计产品改造、测试、文档及必要依赖源码，排除运行数据、构建包和本地配置。下文“未提交/推送”为各开发轮次结束时的历史记录。上传前完成提交范围、凭据模式、文件大小及差异检查；保留上游依赖生成文件原有末尾空行。本次没有新增功能或重新执行产品测试，验证事实仍以各专项记录为准。

2026-09-09 ADR-047十项改造已实现：设备列表/发现、模板选择、顶部设置、并列端口页、采样末页、图上说明清理、模板分类稳定编辑和独立磁盘展示、设备文件上传下载、仓库工具查询与确认投放。WPF280、生成器172（0跳过）、浏览器13组、31份WPF布局渲染及Windows/Linux相关Go测试/vet/build通过，Linux相关race通过。自包含客户端为`build/windows-desktop-customer/win-x64/RouterWorkbench.exe`；[本轮验证与限制](CUSTOMER_WORKSPACE_VERIFICATION.md)。管理员上传通道未建设；未替换用户进程/设备Probe或运行数据，未提交/推送。

2026-09-09 服务端模板启动死锁已修复（ADR-046）：旧interface_aliases及设备reported旧结果字段自动备份后清理，空/缺失模板库可启动并发布新模板，设备资料和绑定保留。Windows相关Go测试/vet/build、Linux相关race/vet/build，以及当前运行数据副本的真实Windows EXE启动/查询/发布通过。证据与范围见[服务端启动修复](SERVER_STARTUP_VERIFICATION.md)；原运行数据/用户进程未改动，无Git提交/推送。重新运行server-windows.cmd即可加载修复。

2026-09-09 ADR-045已实现并完成本机验证：平行默认/模板分组与资源监控、完整列宽、内存已用/总量、左侧待纳管、摘要模板更新与右键操作、接口专属采样、同版重新应用、连续连接时长和Probe原生HTTPS出口。详见[证据与限制](DEVICE_WORKSPACE_VERIFICATION.md)。

WPF248、生成器170（0跳过）、浏览器12、C++14、Windows Go/vet及真实Linux全量/race/vet通过；最终原生HTTPS另有8个真实TLS场景通过。自包含客户端为`build/windows-desktop-workspace/win-x64/RouterWorkbench.exe`。ARM编译机认证失败，sanitizer缺库，当前公网端点握手/IPv6连通未成功；均未计作实机通过。用户进程/数据保留，未提交/推送。

## 上一轮基线

2026-09-09 ADR-044已实现并完成本机验证：接口映射全链路删除，属性分类/字段开关、纯名称、整行折叠、模板固定顺序、默认单行/手动换行及连续像素滚动。只保留工程8/草稿3，移除旧模板准备、启动采集/REGISTER快照、遥测降级和心跳推断。

Windows Go/vet、真实Linux完整Release/race/vet、13组C++、WPF224项、生成器169项（0跳过）、12组浏览器通过；独立Windows客户端在`build/windows-desktop-refined/win-x64/RouterWorkbench.exe`。详见 [本次证据与产物](UI_REFINEMENT_VERIFICATION.md)。ARM厂商部署/物理DPI未验收；sanitizer因缺库未通过。用户进程/数据/已有改动保留，未Git提交/推送。

## 此前验证记录

下文旧工程兼容、接口别名、旧Probe路径与旧产物属于历史事实，当前行为以ADR-045及上文证据为准。

2026-09-09 ADR-043代码与本机验证完成：WPF统一网口页/同页曲线，模板独立物理字节来源，FNR100五口预设与原始样本预览，Probe精确uint64计数/重连基线，旧Probe能力检查。13组C++、完整Linux Release/race及Windows Go、重连race专项、WPF202项、生成器167项和11组浏览器通过。详见[PHYSICAL_PORT_MONITORING](PHYSICAL_PORT_MONITORING.md)。

实机Telnet已确认逐口MIB及链路，尚未部署新版Probe：10.1.1.128交叉编译机SSH免密失败，已请求登录信息。新版ARM、逐口受控传输/拔插未验收；C++sanitizer缺库。保留正在运行的Server/UI/Probe、原设备配置与已有改动，实机专用模板候选保留7项原属性；未Git提交/推送。后续完成配套构建与部署无需重新索取本轮实现/Probe替换授权。

2026-09-09 ADR-042已实现：独立模板配置页、四个配置分类、属性归组/排序与统一表同步、分组预览、按后端解释物理口和可选芯片编号。工程6兼容1～5；Server/Probe保留未知编号而不补0。164项生成器测试、10组浏览器、12组C++、完整真实Linux Go race/vet/构建、Windows Go测试/vet/构建通过。原慢消费者测试的缓冲区假设已修正并复验；首轮release部分失败保留记录。详见 [TEMPLATE_CONFIGURATION_VERIFICATION](TEMPLATE_CONFIGURATION_VERIFICATION.md)。

本轮未构建ARM/厂商实机验收；C++sanitizer仍缺运行库。生成器旧实例需正常退出后重启加载新版，用户进程未替换；无Git提交/推送。下文ADR-041及更早记录保留对应历史事实。


2026-09-09 ADR-041已实现：持久待纳管/忽略池、管理员名称/型号、服务端型号模板映射与版本固定、在线热配置/离线同步、CPU静态两次采集、DSA/swconfig/厂商命令物理端口、模板分组排序/折叠与接口别名。当前配套入口仍是WPF与Blazor；[操作与契约](MANAGED_PROBES_DESIGN.md)。

本轮验证：12项CTest、真实Linux Phase1～5完整回归、完整Go race/vet及Linux/Windows Server构建通过；WPF195项和生成器158项（无跳过）、9组真实浏览器流程通过。最终补丁另跑相关race/真实Probe专项。详细命令/产物见 [MANAGED_PROBES_VERIFICATION](MANAGED_PROBES_VERIFICATION.md)。

当前限制：新版ARM构建尚未验证（10.1.1.128免交互SSH认证失败）；C++ ASan/UBSan/TSan运行库缺失，未通过；厂商芯片与机壳端口映射/实际固件/DPI仍待实机。下方旧ARM成品属于ADR-040，不能当作本轮产物。保留既有未提交改动、运行数据与用户进程，未提交或推送。以下此前记录为对应历史验证，本轮生效规则以ADR-041为准。

2026-09-09 ARM构建修复：上传后的 Bash 脚本在执行前规范化行末CR，本地脚本保存为LF，解决 `set: pipefail` 无效选项。使用用户提供的认证，真实Windows PowerShell → 10.1.1.128 GCC5.2构建成功，默认接口 `eth0,eth1,br0`，ARMv7/uClibc成品373112字节，位于 `build/arm-gcc52-20260909/router-probe`；先前ARM认证/成品缺口已解决，厂商运行仍待验收。详细证据见 [DEPLOYMENT §5.3](DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)。

2026-09-09 ADR-040：双栈出口、构建/模板/CLI接口过滤、默认型号/固件、网口流量/时长与WPF曲线及设备展示已实现。最终10项CTest、完整Linux/Go race、Windows Go/WPF166项、生成器154项及发布通过；归属地实连成功。浏览器启动被自动审核拒绝、GCC5.2 SSH未认证、出口实网TLS/IPv6超时、C++sanitizer缺库与厂商实机仍待验收；产物及证据见 [MONITORING_V2_VERIFICATION](MONITORING_V2_VERIFICATION.md)。

2026-09-08 生成器启动文件锁修复：重复启动复用本仓库同端口实例，按端口与 BuildOnly 隔离构建输出，其他程序占用端口时明确报错。Windows/.NET 10.0.400 实测首次启动、CMD 重复启动、运行期间 BuildOnly（0 警告/错误）、首页/四项静态资源 HTTP 200、外部端口冲突保留原监听通过；说明见 [TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md#启动脚本文件锁修复)。

2026-09-08 ADR-039已实现：CPU型号/频率/详细架构与位数、CPU/内存/存储/网口独立周期、模板同键优先、WPF监控表格及来源IP/公网归属地。9项C++、真实Linux全量及Go race、最终专项、Windows Go/WPF153项、生成器148项和两套自包含发布通过；浏览器启动被自动审核拒绝，ARM交叉编译SSH未认证，公网归属地实连超时，sanitizer缺库/厂商实机仍待验收。产物及证据见 [TELEMETRY_VERIFICATION](TELEMETRY_VERIFICATION.md)。

最后更新：2026-09-09。

此前 Probe 一键交叉编译入口 `probe-build.cmd`：自动上传当前源码到 10.1.1.128、在远端副本处理 GCC 5.2/uClibc 兼容并构建，所有构建文件集中在专用 `/root/codex-probe-20260908-2123`。Windows PowerShell → SSH 全流程和 ARMv7 ELF 检查通过，成品 280908 字节；失败返回非零且保留原成功成品已实测，详见 [DEPLOYMENT](DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)。目标固件运行仍待验收，未改本地 Probe 业务代码。

此前已完成 ADR-038：默认兼容架构/内核采集、系统开机时长心跳 → Device/Session runtime API → WPF；年月日时分秒从最高有效单位显示到秒，年365日/月30日。同设备全部属性按值更新并保留选择。8 项 C++、真实 Linux 专项及 Phase 1～5/race、Windows Go test/vet、WPF 120 项检查、Release 发布及12份布局通过；证据与产物见 [SYSTEM_INFO_VERIFICATION](SYSTEM_INFO_VERIFICATION.md)。本次未授权 Git 提交/推送。

当前仅保留新版 C# / WPF 主工作台与 C# / Blazor / Fluent UI 探针模板生成器（ADR-037/036/035/034）。React 浏览器工作台、WinUI/Win32/WebView2 旧宿主、专属构建/验证脚本及旧 Bridge/ConPTY 已移出源码；当前 Client、Core 平台能力与测试仍保留，详见 [UI_CLEANUP](UI_CLEANUP.md)。此前 UI 清理已形成 `bdf9f9b`；那次提交/推送授权不延续到本次任务。运行数据、旧包、源码压缩包和构建产物不纳入提交。

- 主 UI：`ui-windows.cmd` → `windows/build-desktop.ps1`。设备、维护、文件、配置、设置；启动自动连接，连接状态与输出同步，五秒刷新保持所选属性行。维护只展示公共链接并打开外部客户端，SSH 默认 admin/admin，可编辑并 DPAPI 加密保存；模板全部属性与失败项展示；ADR-039动态结果按Server合并后的effective_metrics显示。没有内置终端、客户工具管理或通用任务入口。文件/配置结果留在各自页面，管理员工具入口尚未建设。
- 生成器：`template-generator.cmd` → `src/ProbeTemplateGenerator`，本机浏览器默认 5188。支持虚拟/展示属性、command/NVRAM/UCI、公式/条件规则、预览、工程导入导出、草稿及服务器发布；工程3兼容版本1/2与旧原生草稿，增加内置监控与逐属性周期。两套 UI 构建均不需要旧前端或 Node。
- Server/Probe：保留 Go 管理端与 C++11 Probe，模板持久化、启动采集、默认 `nvram get SN` 设备 ID、NVRAM/UCI 专用任务及 ADR-038 内置系统信息已实现。Repository 与模板持久化，Session/Task/维护/幂等账本仍不跨进程恢复。

此前生成器145项与Edge/Go API8组流程是历史证据；本次生成器148项及发布通过，浏览器未运行。此前 UI 清理与专项证据见 [UI_CLEANUP](UI_CLEANUP.md)、[WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md)、[TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md)、[ROUTER_CONFIG_VERIFICATION](ROUTER_CONFIG_VERIFICATION.md) 与 [PROBE_TEMPLATES_VERIFICATION](PROBE_TEMPLATES_VERIFICATION.md)。

当前方向为完善 Router-Agent，不进入后续阶段（ADR-028）。Phase 0～5 已验收；Phase 6 厂商 SSH/Telnet、NVRAM/UCI 固件副作用、交叉架构、物理多屏 DPI/鼠标/文件选择器/剪贴板及干净目标机仍待验收。历史 Win32/ConPTY 验收未完成且旧 UI 已退役，不视为当前产品待修实现。本次重新尝试 C++ sanitizer，WSL 仍缺 ASan/UBSan/TSan 运行库，未完成该检查。

**必须保留的数据恢复遗留：此前 Agent 清理测试挂载误删原工作区，源码从远端 f73853f 加任务补丁恢复并重新验证；原未跟踪 `cmd/server/1.txt`、`cmd/server/data/` 尚未恢复，仍需备份来源。旧 Git 元数据不声称原样恢复，`build/windows-react` 曾从当天 09:13:52 卷影副本恢复，不是新版发布。详见 [恢复记录](PROBE_TEMPLATES_VERIFICATION.md)。本次不改变此事件或恢复状态。**

测试环境使用项目外 WSL 2 `RouterAgentTest`，见 [WSL_TEST_ENVIRONMENT](WSL_TEST_ENVIRONMENT.md)。Windows Server 入口 `server-windows.cmd`，数据在 `data/server/repository`；本机双栈接入历史验证通过，公网 IPv6 和厂商路由器尚待实测，使用见 [DEPLOYMENT](DEPLOYMENT.md)。

Phase 7 MCP、Phase 8 AI Agent、微信小程序、正式公网 Web 部署、新 Tunnel 数据面及其他大规模架构扩展暂缓。认证/TLS/RBAC、租户、完整审计等仍未决；不自动开展下一项功能。
