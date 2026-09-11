# 项目路线图

- [x] 2026-09-11 服务器迁移连接故障已排查，用户确认解决；Agent 未复测解决后的端到端连接。本轮依授权整理既有迁移源码/测试/文档到本地 main 并删除迁移分支，实际合并结果以 Git 为准，不推送远端；不改变 Phase 6 最终产品验收状态。

- [x] 2026-09-11 ADR-058：恢复10.1.1.128密码自动构建，产物改为/root/router-agent/router-agent；真实GCC5.2编译及ARMv7/EABI5/uClibc检查通过，厂商安装/运行验收保持未完成。

- [x] 2026-09-11 ADR-057：统一服务器默认值、Linux AMD64构建/SFTP上传和远端短时启动、Probe默认接口免询问、Windows维护/凭据/目录及说明简化；已验证范围与未完成实机项见[专项验证](SERVER_DEFAULTS_VERIFICATION.md)。

2026-09-10 用户已明确授权将当前累计源码、测试和文档提交并推送到 GitHub `CRISKAKA78/Router-Agent` 的 `main`。本次仅整理提交：核对远端基线、文件范围和 `git diff --check`，不重跑全量产品测试；构建包、运行数据和本地配置留在本机。下文各轮“未提交/推送”为当时记录，当前提交号与推送结果以 Git 为准；ARM、sanitizer 和实机验收缺口保持。

- [x] 2026-09-10 ADR-056实现与本机验证：LAN下接与本机广播域分开显示且允许重叠，被动邻居/FDB/租约、原生IPv4扫描/取消、API及模板编辑/发布闭环完成；Windows/Linux、真实Probe、Go race、15项CTest、生成器176项/浏览器14组及WPF557项/67份布局证据见[邻居发现](NEIGHBOR_DISCOVERY.md)。不新增阶段。
- [ ] ADR-056配套ARM/uClibc编译、FNR100升级及真实终端/外壳映射/VLAN/NDP验收；首轮交互认证已进入GCC5.2编译，旧uClibc的`std::snprintf`适配遗漏已修复并完成本地转换/编译回归，真实工具链仍需交互密码复跑。生产进程与设备配置保留，C++sanitizer缺库未通过，见专项记录。

- [x] 2026-09-10 Server重启后的Probe循环断线修复：合法未知结果仅记日志并忽略；Windows/Linux全量、真实Probe跨Server实例重建及再次重连、Go race、14项CTest、隔离Windows成品启动通过。[验证与部署入口](SERVER_RECONNECT_FIX_VERIFICATION.md)。保持旧任务不重建与既有缓存规则，不新增阶段。
- [ ] 本次修复加载到用户生产Server：现有进程/数据保留，正常关闭后重新运行 `server-windows.cmd` 生效；无需升级Probe。C++ sanitizer缺库限制见专项验证。

- [x] 2026-09-10 ADR-055：按本轮编号1完成四页紧凑操作台，历史按需展开、文件图文对齐和条件动作、维护同行入口、配置条件表单及结果归属、工具选择后版本区；551项回归、62份实际WPF布局、发布和最终渲染对照通过。[验证](COMPACT_WORKSPACE_VERIFICATION.md)。设备详情方案3保持，不新增阶段。
- [ ] ADR-055最终原生鼠标/整窗截图复验：发布EXE只读连接已确认，桌面会话输入/捕获失败，尚需会话恢复后补验；不影响已完成的功能回归与实际WPF渲染事实。

- [x] 2026-09-10 ADR-054：按方案3统一字体与共享控件，修复箭头尺寸/基线、正文继承蓝色、模板列表皮肤、字重/行距及默认设备ID宽度；519项检查、54份布局、16份离屏密度和自包含发布通过。[组件规范](COMPONENT_VISUAL_SPEC.md)、[实际Design QA](../design-qa.md)。上一轮组件视觉通过结论已撤回，本项另行复验；保持结构和业务，不新增阶段。

- [x] 2026-09-10 ADR-053：采用用户选定的方案3，完成连续属性/页面选择/检索/完整值面板/活动条；490项检查、50份WPF布局和自包含发布通过。此为结构/功能历史事实；组件视觉以ADR-054复验为准。[目标、对照与证据](../design-qa.md)。保持既有功能、信息结构和业务，不新增阶段。
- [ ] 多屏物理DPI、厂商固件副作用和干净目标机安装仍待验收；本轮本机原生窗口检查不等于Phase 6最终产品验收。

- [x] 2026-09-10 会话空时间兼容修复：解决设备快照反复恢复；481项桌面检查、自包含发布和用户现有后端只读同步通过。[验证与程序](SNAPSHOT_REFRESH_FIX_VERIFICATION.md)。保持ADR-052界面与现有API/协议。

- [x] 2026-09-09 ADR-052：顶部弹性摘要、二级Tab导航带、系统首屏概览与四类语义卡片完成。473项桌面检查、47份WPF布局和独立发布通过；[证据与成品](SYSTEM_INFORMATION_HIERARCHY_VERIFICATION.md)。保持整体布局、页面与业务，不新增阶段。
- [ ] ADR-052物理输入/多屏DPI、发布EXE手工启停及厂商设备验收；本轮测试对端与WPF矢量布局不代替实机和Phase 6最终验收。

- [x] 2026-09-09 ADR-051：顶部设备摘要、左侧序号/排序、两级导航权重、统一线性图标、双列系统信息与辅助输出调整。408项检查、47份WPF布局和独立发布通过；[证据与成品](DEVICE_DETAILS_LAYOUT_VERIFICATION.md)。仅当前展示层迭代，物理输入/DPI、厂商实机和Phase 6最终验收保持待办。

- [x] 2026-09-09 ADR-050：所有表格列居中、字体/字号保存与恢复、菜单/工具窗口字号适配、悬停/焦点/真实选择分离和设备右键异常修复。340 项桌面检查、41 份 WPF 布局及自包含发布通过；[验证与成品](APPEARANCE_INTERACTION_VERIFICATION.md)。沿用当前产品结构，物理输入/DPI与发布 EXE 直接启停仍未验收。

- [x] 2026-09-09 ADR-049：原生 Workbench 视觉、三级 Tab、连续 Inspector、列表/表单与最小窗口调整；314 项桌面检查、35 份实际 WPF 布局及自包含发布通过。[验证与产物](WORKBENCH_VISUAL_VERIFICATION.md)。保持原业务和技术栈，不新建阶段；物理 DPI、厂商设备及发布 EXE 直接启停仍未验收。

- [x] 2026-09-09 ADR-048：来源IP与运营商/归属地分行摘要、接口状态子分组和采样时间弹窗、四项工作区更名；306项桌面检查、32份布局及自包含发布通过。[验证与产物](INTERFACE_WORKSPACE_VERIFICATION.md)。公网归属服务本机实查超时，真实查询及物理DPI验收仍未完成。源码、测试和配套文档纳入本次进度提交，用户已授权推送GitHub main。

- [x] 2026-09-09当前项目源码快照上传GitHub：源码提交`810de06`已推送至`CRISKAKA78/Router-Agent`的main并通过远端哈希核对，包含累计改造、测试和文档，排除运行数据与构建产物。

- [x] 2026-09-09 ADR-047：导航/端口清理、模板分类稳定编辑及独立磁盘展示、模板选择弹框、设备文件浏览与本地上传下载、仓库工具搜索及确认投放完成。WPF280、生成器172、浏览器13、31份布局和Windows/Linux相关Go验证通过；[产物与限制](CUSTOMER_WORKSPACE_VERIFICATION.md)。
- [-] 管理员仓库工具上传通道后续单独实现，本轮仅移除Windows发布入口；真实固件操作、物理DPI及发布EXE启停仍待验收。

- [x] 2026-09-09 服务端模板启动恢复（ADR-046）：存量退役字段先备份后清理，空/缺失模板库启动与后续发布、设备显式应用闭环；Windows相关测试/vet/build、Linux相关race/vet/build及当前数据副本真实EXE启动发布通过，见[验证记录](SERVER_STARTUP_VERIFICATION.md)。

- [x] 2026-09-09 ADR-045实现与本机验证：并列分组/资源监控、接口专属设置、模板更新/重新应用、连续连接时长、待纳管迁移、内存实际容量及Probe原生HTTPS。WPF248、生成器170、浏览器12、C++14与Windows/Linux全量/race/vet通过；[证据与产物](DEVICE_WORKSPACE_VERIFICATION.md)。
- [ ] ADR-045 GCC5.2/uClibc ARM构建/厂商部署、实际公网双栈成功探测与物理DPI验收；编译机认证失败，当前公网端点握手/IPv6连通失败，C++ sanitizer缺库。

- [x] 2026-09-09 ADR-044：删除接口映射和旧兼容；属性显示选择、名称、模板顺序、整行折叠、单行/手动换行与像素滚动完成。WPF224、生成器169、浏览器12、C++13及Windows Go/真实Linux Release/race/vet通过；[验证与成品](UI_REFINEMENT_VERIFICATION.md)。
- [ ] ADR-044厂商ARM配套部署与物理DPI/触控板验收；C++sanitizer因环境缺库未通过。未新建阶段、未替换用户进程、未提交/推送。

以下为历史里程碑；旧版本兼容和接口别名已由ADR-044撤销。

- [x] 2026-09-09 ADR-043实现与本机验证：统一网口页、逐物理口字节后端/精确计数与同页曲线、FNR100模板预设/样本预览、旧Probe能力保护；13组C++、完整Linux Release/race及Windows Go、WPF202项、生成器167项及11组浏览器通过，见[专项说明](PHYSICAL_PORT_MONITORING.md)。
- [ ] ADR-043配套ARM编译/实机升级：等待10.1.1.128 SSH登录信息；Telnet MIB查询已确认，逐口受控流量/拔插及WAN真实业务尚未验收，C++sanitizer仍缺库。

- [x] 2026-09-09 ADR-042：独立模板配置页、共享归组/排序与预览、物理口表单说明及可选编号兼容；164项生成器、10组浏览器、12组C++、完整Linux Go race和Windows Go验证完成，见 [TEMPLATE_CONFIGURATION_VERIFICATION](TEMPLATE_CONFIGURATION_VERIFICATION.md)。
- [ ] ADR-042新版ARM/厂商物理口验收；C++sanitizer因缺库未运行通过。


- [x] 2026-09-09 ADR-041：待纳管、持久管理员资料、服务端型号模板映射、动态采集、CPU两次、物理口适配、分组排序/折叠及接口别名；12项C++、完整Linux/Go race、WPF195项、生成器158项/9组浏览器通过，见 [MANAGED_PROBES_VERIFICATION](MANAGED_PROBES_VERIFICATION.md)。
- [ ] ADR-041新版ARM/uClibc构建、厂商交换机/机壳端口逐口验收与真实DPI；本轮SSH认证未通过，C++sanitizer缺库。此前ADR-040 ARM产物不包含本轮功能。

- [x] ARM构建CRLF修复：本地LF、上传后行末CR规范化、原失败脚本回归、真实GCC5.2构建及ARMv7/uClibc检查通过；默认接口eth0,eth1,br0，成品及证据见 [DEPLOYMENT](DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)。
- [x] ADR-040：双栈出口、接口过滤、默认型号固件、网口累计流量/时长与10分钟曲线、WPF设备展示；C++10项、完整Linux/race、WPF166项、生成器154项及发布通过，见 [验证](MONITORING_V2_VERIFICATION.md)。
- [ ] ADR-040部署与实机验收：GCC5.2编译已完成，厂商设备运行、真实双栈出口、物理网口/DPI仍待完成；浏览器启动被自动审核拒绝，C++sanitizer仍缺库。

- [x] 生成器重复启动文件占用修复：复用同仓库已有实例、隔离运行/检查构建输出、明确其他程序端口冲突；Windows 启动、复用、运行中构建及 HTTP 静态资源验证通过。

状态标记：

- [x] 已完成并可验证
- [ ] 未完成
- [-] 暂缓或未决

## 当前工作方向：Router-Agent 产品持续完善

- [x] ADR-039实现与自动验证：详细CPU、独立周期监控/模板优先、来源IP与公网归属地查询、WPF表格、生成器工程3；C++9项、WPF153项、生成器148项、真实Linux/race及发布通过，见 [TELEMETRY_VERIFICATION](TELEMETRY_VERIFICATION.md)。
- [ ] ADR-039部署验收：GCC5.2编译机SSH认证后交叉编译、厂商固件；浏览器启动被自动审核拒绝、当前网络公网归属地超时、C++sanitizer缺库，尚未验收。

- [x] Probe GCC 5.2 一键交叉编译：Windows 双击上传当前源码、远端副本兼容处理、独立构建记录与成功产物更新；10.1.1.128 真实编译及 ARMv7 ELF 检查通过，厂商运行验收仍待完成，见 [DEPLOYMENT](DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)。

- [x] 内置设备信息与开机时长（ADR-038）：默认架构/内核、心跳运行状态、公开 API 与年月日时分秒展示；8 项 C++、真实 Linux 专项、Phase 1～5/race、Windows Go/WPF 120 项和 Release 构建通过，C++ sanitizer 缺库及厂商实机仍待验证，见 [SYSTEM_INFO_VERIFICATION](SYSTEM_INFO_VERIFICATION.md)。

- [x] 旧 UI 清理（ADR-037）：仅保留 WPF 主 UI 和 Blazor 生成器，迁出必要测试对端与浏览器依赖，移除 React/WinUI/Win32/WebView2 源码及专属入口；当前交付与验证见 [UI_CLEANUP](UI_CLEANUP.md)。

- [x] 连接状态与所选设备闪烁：连接完成/重连状态和日志同步，固定行按值通知，保留五秒刷新且不重建控件；98 项检查、12 份布局与 Release 启停通过，见 [WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md)。

- [x] 维护关闭后重开 409：修复旧快照覆盖新维护选择、释放状态回退和重复创建；84 项原生/当前 Go 对端检查及修复包启停通过，未修改 Server/Probe，见 [WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md)。

- [x] 客户端入口收敛（ADR-036）：设置与启动自动连接、维护通道链接/外部 Shell、SSH admin/admin 与可编辑加密偏好、移除客户工具和通用任务入口、模板全部属性；65 项自动检查、12 份布局与 Release 发布通过，见 [WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md)。
- [-] 管理员工具配置/上传维护入口和新任务入口规划：本轮未实现，等待后续具体需求。

- [x] Windows 主程序原生 C# / WPF 工程工作区（ADR-035）：整体布局、密集原生控件、浅深主题与现有设备/维护/任务/文件/工具/配置能力迁移；51 项客户端/Go 对端/WPF/ConPTY/WebView2 检查、16 份实际 WPF 矢量布局、Release 自包含发布通过。见 [WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md)。
- [ ] 当前 WPF 包的厂商设备 SSH/Telnet、物理多屏 DPI/鼠标拖动及干净目标机验收。远程桌面原生捕获不可用；不将矢量布局验证当作屏幕验收，不影响下方保留的历史风险记录。

- [x] 生成器 C# / Blazor 技术迁移与紧凑全屏工作区（ADR-034）：完整功能、工程版本 1/2、旧草稿与运行模板兼容；145 项 C#、8 组 Edge/Go API、宽窄主题、Release 发布启动通过，旧生成器代码清理及主前端/原生构建通过。见 [TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md)。

- [x] 展示属性条件结果（ADR-033）：单值映射、多虚拟属性条件组合、文本比较、有序规则/默认值、预览命中项，复制与版本 1/2 工程兼容。87 项前端/BusyBox、浏览器/API、生成器单 EXE 构建通过，见 [TEMPLATE_GENERATOR](TEMPLATE_GENERATOR.md)。
- [x] 二次确认改为提示弹框，移除确认词输入；属性支持独立复制、唯一标识与自动选中。54 项前端/BusyBox、浏览器/API 复制与直接确认/取消及两个单 EXE 编译通过，见 [TEMPLATE_GENERATOR](TEMPLATE_GENERATOR.md)。
- [x] 独立模板生成器（ADR-032）：虚拟/展示属性、现有来源、逻辑/算术公式、工程导入导出及服务器发布；主 UI 设置移除模板配置。51 项前端/BusyBox、浏览器/API、专项原生 WebView2 检查及两个单 EXE 编译通过，见 [TEMPLATE_GENERATOR](TEMPLATE_GENERATOR.md)。
- [-] 历史 Win32/ConPTY 与旧原生生成器验收：当时未完成；相应 UI 已按 ADR-037 移除，不再作为当前验收项。新版厂商实机与物理交互验收仍独立保留。

- [x] nvram/uci 专用读取、写入、删除、独立 commit 与模板只读来源：Accepted ADR-031，API/Probe/React 闭环及 Release/真实 Probe/Go race/Windows Go/前端浏览器验证通过；单 EXE 编译完成，见 [验证](ROUTER_CONFIG_VERIFICATION.md)。
- [ ] nvram/uci 厂商固件及交叉架构验收；C++ ASan/UBSan/TSan 因当前运行库缺失未完成，新 EXE 完整 Win32 业务验收仍待用户执行。

- [x] 按用户确认 ADR-030 将宿主移植为 C++ Win32 / WebView2，单 EXE 构建入口与应用资源内嵌完成，编译通过。
- [-] 历史 Win32 客户端最终验收：当时由用户自行执行，未记录通过；已由 WPF 取代并按 ADR-037 移除。

- [x] Windows Server 一键清理重建/启动脚本及 pcv6.criskaka.com 配置；本机 API/控制/数据/维护入口双栈接入验证通过，远端 IPv6 验收独立待实测。

- [x] 本机 Linux 测试环境迁入项目外独立 WSL 2 RouterAgentTest；复用指引、重启、CTest 与模板集成验证完成，项目原镜像可由用户移除。

2026-09-07 用户明确要求暂不进入后续阶段，继续完善已有产品。普通功能依据 AGENTS / DEVELOPMENT / Accepted ADR 自主完成，不需要用户逐项提供技术提示词；这是当前基线内的持续迭代，不是新增 Phase，也不代表 Phase 6 最终验收完成。

- [x] Agent Governance / Repository Guidance：明确普通功能自主范围、设计变更确认、模块导航、按影响验证与文档交付规则；本次仅文档改造。
- [x] 服务端设备属性模板：按 ID/名称选择、自定义采集指令、服务端持久化/API/设置管理、设备快照展示；ADR-029 已确认，Release/真实 Probe/浏览器/Go race 验证通过。
- [x] Probe 默认 `nvram get SN` 设备 ID：显式参数优先与失败校验、C++ 单测及真实 Probe 测试替身闭环通过。
- [ ] 模板改动的 C++ ASan/UBSan/TSan（当前工具链缺运行库）及厂商固件实测；不记为已通过。
- [ ] 恢复误删的原 cmd/server/1.txt、cmd/server/data/，需可用备份来源；事件与已恢复范围见 PROBE_TEMPLATES_VERIFICATION。
- [ ] 按用户后续具体需求完善当前功能、行为和体验；每项单独形成可验证闭环，不预先扩展功能清单。
- [ ] 用户实机与 Phase 6 最终产品验收，沿用下方尚未完成项。
- [-] 正式公网 Web 部署、微信小程序、Phase 7 MCP、Phase 8 AI Agent。
- [-] 新 Tunnel 数据面及其他大规模架构扩展；既有维护缺陷仍可在 Accepted 边界内修复。

以下 Phase 0～6 为历史里程碑，旧 UI 路径与“继续冻结/复用 React”是当时事实，已由 ADR-034～037 取代；历史“推送后停止/不得进入下一 Phase”描述各次交付边界，当前任务范围以上述方向和最新明确授权为准。

## Phase 0 Repository and Documentation Initialization

状态：已完成

- [x] 完整阅读 v0.2 Word 设计输入。
- [x] 建立 README.md、AGENTS.md 和 CHANGELOG.md。
- [x] 建立 ARCHITECTURE、PROTOCOL、API、ROADMAP、PROJECT_STATUS、HANDOFF 和 DECISIONS 文档。
- [x] 将 TCP 协议转换为仓库内 Markdown 基线。
- [x] 完成 Protocol v1 互操作细化，并保留未决安全、恢复、Tunnel、存储和 API 主题。
- [x] 建立项目事实来源层级和历史设计输入规则。
- [x] 将 Phase 1 拆分为可验证里程碑并设置技术决策门槛。
- [x] 核对文档间的阶段、架构、协议、任务与文件时序和能力状态。
- [x] 用户确认 Phase 0 最终文档。
- [x] 形成明确的 Git baseline commit。
- [x] 用户已授权在形成 Git baseline 后进入 Phase 1。

baseline commit: bc8d747dfc41a375c31698073005857c238ede51

## Phase 1 Probe and Server TCP Control Link

状态：已完成（Phase 1A～1E 验收通过）

Phase 1 保持一个总阶段，按 Phase 1A 至 Phase 1E 顺序推进。每个里程碑必须形成可构建、可运行、可测试的小闭环；不得一次性铺开整个 Phase 1。

### Phase 1 前置技术决策

进入 Phase 1A 业务开发前的设计门槛已经完成：

- [x] Probe：C++。
- [x] 最低语言标准：C++11。
- [x] 构建系统：CMake。
- [x] 第一开发与验证平台：Linux x86_64。
- [x] 交叉编译策略：后续使用 CMake toolchain files 适配 mipsel、ARM、ARM64；具体工具链版本按真实设备补充。
- [x] REGISTER、REGISTER_ACK、HEARTBEAT 和 HEARTBEAT_ACK 的完整字段契约与注册失败响应已写入 PROTOCOL.md。

Management Server 使用 Go 的 Accepted 决策保持不变。Phase 0 Git baseline、Phase 1A 与 Phase 1B 均已完成。

### Phase 1A TCP Session

状态：已完成

- [x] TCP framing，包括半包 Header、半包 Payload 和一次读取多帧。
- [x] 20-byte Header encode / decode 与 Big Endian 整数处理。
- [x] REGISTER 与字段校验。
- [x] REGISTER_ACK success=true / false。
- [x] HEARTBEAT。
- [x] HEARTBEAT_ACK 与 reply_to 校验。
- [x] heartbeat_interval 与 3 倍失联判断。
- [x] 1/2/5/10/30 秒基础断线重连。
- [x] 重连后重新 REGISTER 并生成新 session_id。
- [x] Linux x86_64 Server + Probe 可构建、可运行、可测试闭环。

### Phase 1B Task and Exec

状态：已完成

- [x] TASK。
- [x] TASK_ACK。
- [x] TASK_RESULT。
- [x] exec。
- [x] timeout。
- [x] 基础任务状态机。
- [x] 形成 Phase 1B 可构建、可运行、可测试闭环。
- [x] 接管审查 R1-R4 修复：注册与派发顺序、失败 writer 失效和不确定派发保留、exec socket 隔离、TERM/KILL 与 pipe 排空回归。

### Phase 1C Concurrency Idempotency and Reconnect

状态：已完成实现与验证，独立提交 `71e5d17` 已推送 GitHub main；后续 Phase 1D/1E 已完成。用户已明确确认三项互操作契约，见 PROTOCOL.md / ADR-015；R1-R4 前置修复已单独提交为 `59e65b4`。

- [x] 多任务并发。
- [x] 乱序结果。
- [x] task_id 幂等。
- [x] TCP 重连后的任务关联。
- [x] Probe 进程生命周期内的任务去重。
- [x] 形成 Phase 1C 可构建、可运行、可测试闭环。

验证：默认 4 workers；三个任务在执行屏障同时等待并反序完成；queued/running/完成态重复任务、同 ID 内容冲突、缓存容量、多次重连和真实 ACK/RESULT 丢失补报测试通过。Linux CTest、全量 Go 测试与真实 Probe 集成、Go race、vet，以及 Windows Server 构建/单测/vet 均通过，详见 PROJECT_STATUS.md。

### Phase 1D File Transfer

状态：已完成实现与全量验证，独立实现提交 `f1d9fa08d047f4f46a8bc27119565a2f6d217ecc` 已推送 GitHub main；后续 Phase 1E 已完成整体验收。P1-P5 与 sha256_ok 补充已确认，正式契约见 ADR-016/017。

- [x] upload。
- [x] download。
- [x] FILE_BEGIN。
- [x] FILE_ACK。
- [x] FILE_CHUNK。
- [x] FILE_END。
- [x] size 和 sha256 校验。
- [x] 文件传输期间控制消息不被饿死。
- [x] 形成 Phase 1D 可构建、可运行、可测试闭环。

### Phase 1E Verification

状态：已完成，Phase 1 完整验收通过；本次独立 Verification commit 交付后停止等待用户验收。

- [x] 协议单元测试。
- [x] Server 与 Probe 集成测试。
- [x] 非法 Header。
- [x] 非法 JSON。
- [x] payload limit。
- [x] 重复 task_id。
- [x] 网络中断。
- [x] 文件中断。
- [x] Phase 1 完整验收。

Protocol v1 的 14 项验收基线已映射至可运行测试，完整结果见 [PHASE1_VERIFICATION.md](PHASE1_VERIFICATION.md)。A～D 全量回归、C++/Go、Go race/vet、真实 Probe、C++ sanitizers 和 Windows Server 适用验证均通过。明确实现 bug 已修复，没有新增后续能力或变更 Accepted ADR。Phase 2 须另行授权。

## Phase 2 Device Management

状态：已完成，Accepted ADR-018 与时间语义已实现，Phase 1/2 全量验收通过；独立 Phase 2 commit 交付后停止等待用户验收。

- [x] 设备清单、状态与能力模型。
- [x] 设备会话和连接状态管理。
- [x] 设备信息查询与历史状态的最小可用闭环。

正式 Device Service / Inventory 与内部 List/Get/Sessions；默认当前 Session 加最近 64 条已结束历史，进程内保留。LastOnlineAt 为最近发布时刻，LastOfflineAt 只在整体下线时更新，replaced 不更新。单元、真实 Probe、Phase 1 全量回归、Go race/vet、C++ sanitizers 与 Windows 适用验证均通过，见 [PHASE2_VERIFICATION.md](PHASE2_VERIFICATION.md)。未引入数据库、外部 API 或 Phase 3 能力。

## Phase 3 File and Tool Repository

状态：已完成，Accepted ADR-019 与稳定身份补充已实现并通过自动化验收，Phase 1/2 全量回归通过；独立 Phase 3 commit 推送后停止等待用户验收。

- [x] 文件资产管理。
- [x] 工具元数据、版本与设备兼容性。
- [x] 工具投放和文件传输的管理端闭环。

以 main `b9982f5d2765546d23e09c28977c27ceb510a368` 为启动基线。用户已明确确认资产/元数据持久化、版本唯一性、兼容规则、内容去重/身份、归档清理；artifact_id 全仓库唯一，所有业务身份保留不重用。验证映射与结果见 PHASE3_VERIFICATION；独立提交推送后停止等待验收，不进入 Phase 4。

## Phase 4 Tunnel

状态：首版及本轮修正的实现和规定验收通过。本轮从首版main `f92d73a0d6003835993967848f1f5fe009a0df89`干净基线修正，采用Accepted ADR-021 / ADR-022，保留极简自研TCP。验证记录见PHASE4_VERIFICATION；修正独立提交推送后停止等待验收。

- [x] Maintenance一次创建三个固定服务入口，默认240分钟与自定义租期。
- [x] 独立data TCP、一次性随机token配对、原始字节Relay、half-close和有界背压。
- [x] SSH真实登录/命令、Telnet双向交互、Web多TCP连接及三服务/多设备并发。
- [x] 主动关闭/到期/Session替换/断线撤销，pending/active流实际终止；端口完全释放后进入隔离，到期才复用。
- [x] 控制writer阻塞/队列满不阻塞本地释放；half-close后reset仍回收Probe，正常EOF排空保持。
- [x] Server侧DataHost域名解析、Probe默认8条流、整连接idle默认24小时；历史文档只依赖已提交仓库。
- [x] 错误/重复/迟到配对、本地/data失败、配额与资源回收、大流量下控制/文件正常。
- [x] Phase 1～3全量回归、Go race/vet、C++ CTest/sanitizers、Linux Probe、Windows/Linux Server适用验证。

首版提交为 `f92d73a0d6003835993967848f1f5fe009a0df89`；本轮修正提交标题为 `fix: isolate Phase 4 ports and decouple maintenance revocation`，实际SHA与main推送结果由Git记录提供。

## Phase 5 HTTP and WebSocket API

状态：已完成并获用户验收。稳定提交 `57c2b1f8f6da1069e4a2eb988224b94bafe9cf84`，采用Accepted ADR-023；历史验证见PHASE5_VERIFICATION。

- [x] /api/v1 HTTP API。
- [x] 设备、Session、exec/Task、资产/文件传输、工具/版本/Artifact/兼容与Maintenance能力。
- [x] 有界WebSocket状态通知、重连同步与慢消费者关闭。
- [x] Phase 1～5全量回归、race/vet、C++ sanitizers及Windows/Linux适用验证。

## Phase 6 User Interfaces

产品基线（2026-09-07）：UI Freeze + Production Integration，已形成提交 `3f239fc`；当前在此基础上持续完善产品。

- [x] 用户确认已完成视觉设计，冻结实际 React 页面，记录 UI_FREEZE。
- [x] 用户确认默认内置 Shell，同时允许外部客户端，新增 Accepted ADR-027。
- [x] 正式入口复用冻结页面结构与样式，保留独立视觉参照。
- [x] ConPTY 平台适配编译及中文 I/O、尺寸、积压关闭、原生租期释放检查。
- [x] 设备/Session、Maintenance、Exec/Task、File/Tool 全流程真实 WebView2 验证。
- [x] 内置客户端/外部入口、错误/并发/断线/重连/退出检查；隔离 OpenSSH/Telnet 命令往返与远端尺寸更新。
- [x] 本轮完整 Windows 发布、13 项前端测试、27 项原生检查、31 项 WebView2 集成及 Phase 1～5 适用回归。
- [x] 同步实际证据并形成正式仓库源码与文档；独立提交/推送以 Git 记录为准。
- [ ] 用户实机与最终产品验收；不主动进入下一阶段。
- [x] 整理真机部署启动指南（DEPLOYMENT），核对当前 Server/Probe 参数并完成 Windows Server 构建；设备固件适配和真机验收仍待实际执行。

以下为上一稳定 React 重构基线 `404b083` 的已完成里程碑，不替代本轮接入验证：

状态：用户授权从 6f0ce71 将 Windows 客户端迁移为共享 React UI + WinUI 3 WebView2 Thin Shell，采用 Accepted ADR-026。React 是今后 Windows 与 Web 的统一产品 UI 基线，公网 Web 部署尚未开始。

- [x] React / TypeScript / Vite / Tailwind / Lucide 工作台及 Light/Dark/系统主题。
- [x] Device/Session、Maintenance 三入口与租期、Exec/Task、File、Tool/Version/Artifact/兼容/投放迁移。
- [x] 唯一 TypeScript HTTP/WS Client、幂等、快照恢复、查询合并与切换/退出取消。
- [x] WinUI Thin Shell、受控本地静态资源、严格平台 Bridge；旧 XAML 业务 UI 和 C# 网络层删除。
- [x] React production build、Windows Release 与包含固定 WebView2 的离线发布目录。
- [x] 本轮 9 项前端测试、21 项原生策略/保存检查、26 项实际 WebView2 集成与本机正式发布启动/退出。
- [-] 无网络干净 Windows Sandbox 运行：Application Control 拒绝未签名 EXE；用户明确将本轮验收改为直接在本机测试，干净目标机运行尚未验证。
- [x] Phase 1～5、Tunnel、Linux Release/ASan/race 与 Windows Go test/vet/build 回归。
- [-] 正式公网 Web 管理界面部署（暂缓；未来复用当前 React 基线）。
- [-] 微信小程序（暂缓）。

继续保持 UI Freeze；开发预览按具体任务需要使用，不要求每次任务持续运行。不进入 Phase 7/8。

## Phase 7 MCP

状态：暂缓（2026-09-07 用户明确要求，未实施）

- [-] 基于同一 Service 或公开 API 的 MCP Adapter。

## Phase 8 AI Agent

状态：暂缓（2026-09-07 用户明确要求，未实施）

- [-] 基于平台 API、MCP 和临时通道的 AI 运维能力。

AI 诊断逻辑属于管理端能力，不进入 Probe。
