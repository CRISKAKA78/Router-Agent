# 路由器远程运维平台架构基线

## Forwarding 产品接入（ADR-069）

独立internal/forwarding.Service持有映射、租期、持久端口隔离、Session撤销和公开状态；Management组合，API/WS只经Application访问。Gateway只提供当前Session有界管理运输；Probe C++ ForwardingManager通过私有管道监管Linux Go侧车router-forwarding-agent，由侧车检查直连IPv4/串口白名单并持有GOST。缺侧车不影响基础Probe。

数据不走控制或旧RMT1：每映射独立TLS Relay/随机内部凭据，设备主动连接并固定证书验证；LAN绑定本机源IP代理目标，串口TCP经serialauth.Gate认证后才连loopback反向后端。进程/Session退出撤销，不恢复0租期条目。外部串口仍明文注册，不隐含全局控制TLS/API认证已完成。旧Maintenance固定目标/正租期不变。详见[产品接入](FORWARDING_IMPLEMENTATION.md)。


## 组网成员配置与恢复（ADR-066）

`overlay.Service` 持有网络、成员配置/修订、期望状态、稳定设备映射与操作历史；WPF 不复制服务端状态机。成员工作线程独立串行，正常执行和周期性核实共享实际配置验证。`WebClient` 使用同机 Web 的结构化配置读取及 ShowNodeInfo 只读 RPC，TOML 仅在服务端内存解析，不公开原始配置或密码。

管理驱动区分仍在执行的 Task 与 Server 重启丢失的暂态历史，复用既有 Probe inspect/install/start 和仓库/File链路恢复已成功配置但临时引擎丢失的在线成员；停止意图不会被后台恢复覆盖。成员配置变更重建单个网络实例，不重启 Probe 或整台设备。未变更 Probe TCP 指令和数据面协议。
## 蜂窝模块适配边界（ADR-067/068）

`统一模板（telemetry/details开关/周期） → Probe USB分组与ATI实际身份 → 内置只读命令规则 → Gateway白名单/会话/配置校验 → Device Service字段与物理单位归一化 → 公开API/WS → WPF属性与三图`。

下方ADR-060的通用身份架构继续作为v1。首个扩展规则为FM160-CN，未知模块回退通用身份；同型号路由器不绑定同一模组。Probe每轮重新识别，不实现身份缓存/热更新规则，UART采集仍独立后台运行且不承载控制流。Server不自行打开串口，不把解析逻辑复制到Client/HTTP Adapter。生成器只编辑意图与能力门禁，WPF只消费统一数据。具体契约见PROTOCOL/API，真实支持范围见[FM160验证](FM160_TELEMETRY_VERIFICATION.md)。

详情仍复用同一采集worker及只读快照服务，通过独立details能力/事件保护旧版本。Server的`cellular_fm160_details.go`集中处理小区、PDP、载波、单次温度和锁定配置投影，字段名/分组/来源由Server派生；WPF仅负责展示和有界趋势。锁定配置读取不是设备写控制服务，没有新增数据面或任意AT入口。手册与实机变体边界见[详细验证](FM160_DETAILS_VERIFICATION.md)。

## 串口鉴权入口（ADR-063 / ADR-069）

`internal/serialauth`负责一条映射的外部TCP注册、凭据摘要校验、连接准入/独占和固定loopback后端的双向字节复制。验证入口在`tests/poc/gostv3/registration`，默认loopback且只允许loopback监听，避免PoC自行暴露公网；它不是正式Server启动入口。

当前调用链为Client → 公开Application/API → 新Forwarding服务，数据另走“外部TCP → 注册鉴权入口 → 私有GOST反向TCP映射 → 设备GOST串口端”。鉴权成功之前不拨后端；旧Maintenance/RMT1保持不变。此模块不实现UART、GOST配置监管、公开HTTP API、设备Session绑定或持久化，以上由 ADR-069 的 Forwarding 服务、Gateway 和设备侧车实现并分别验证。Gate的context取消/到期/轮换回收已测试，不等于远端辅助进程和UART已确认回收。

LAN TCP/UDP保持GOST数据面；PoC补丁只在忽略的第三方源码副本构建，可复现补丁及回归保存在`tests/poc/gostv3/patches`，不把GOST依赖并入产品go.mod。详见[GOST验证](GOST_V3_POC.md)。

## 设备日志（ADR-061）

新增 `internal/devicelog` 负责查询/参数模型和有界原文解码；Management连接该Service与Gateway、Task和File/Repository；HTTP只调用Application，不绕过服务访问会话或资产内部。Gateway关联当前Session的只读EVENT请求与响应；配置和快照依旧走TASK幂等。持续查看采用小批量pull，不新建Tunnel或后台无人订阅的采集任务。

Probe `device_logs.cpp` 承担NVRAM结构化调用、文件尾读、历史目录发现和临时快照；读取worker与主控制循环隔离，配置任务与router_config串行。实时开启是用户已授权的持久写入（两个开关+commit），不再采用临时开关恢复方案。

WPF `LogWorkspace` 经公开 `DeviceLogClient` 显示实时/历史/分析原文，文件传输复用 `FileExchange`；切页/切设备/Server释放并等待旧读取，保留不确定请求原身份。Server对gzip多个拼接成员完整校验，再发布TXT Asset或有限原文预览。厂商解析器只保留明确的待样本状态与后续扩展位置，不建空接口、不产生虚假语义结果。

## 默认部署与客户端偏好（ADR-057）

Server CLI默认全接口监听，对外地址47.119.168.150，HTTP8888/控制9000/数据9001；内部Service缺省保持测试与嵌入用途。Windows本机交叉编译Linux AMD64，再由独立SFTP脚本上传Server和启动脚本，构建凭据不进入产品。WPF/Core移除SSH账号/密码与DPAPI存储职责，仅保存外部程序偏好，旧账号字段读取时忽略。关闭或失效维护隐藏公共地址，文件初始目录/tmp/root。模块/API/协议边界保持，见[验证与部署](SERVER_DEFAULTS_VERIFICATION.md)。


## 通用 AT 身份采集增量（ADR-060，2026-09-11）

可选运行模板 cellular_probe 启用 Probe 的 CellularCollector：只枚举 USB-backed ttyUSB/ttyACM，经占用检查与短期独占发送固定 AT/ATI/IMEI 查询。采集在 TelemetryCollector 所有的可取消后台线程中运行，串口不进入 TCP Gateway；待发送结果合并为一份最新 EVENT，不以串口等待阻塞心跳。每轮重枚举，按 USB 父设备分组而非 tty 编号绑定，限额与共存边界见[CELLULAR_AT](CELLULAR_AT.md)。

Gateway 校验有界事件与能力，Device Service 校验 Session/已应用 revision 并持有最新内存快照、派生观测年龄及 stale。HTTP Adapter 仅通过 Application/Device Service 查询；生成器管理模板开关/周期，WPF 通过公开 Device DTO 展示，不复制 AT 或设备状态机。无新数据库、任务类型、帧或数据面；SIM/射频/厂商适配未实现。

## 邻居发现（ADR-056 / ADR-059）

Probe的SystemSampler惰性持有进程级Neighbors采集器；其有界worker读取内核邻居/FDB、可选租约和显式选择的厂商表。TaskManager复用采集器执行原生ARP扫描/取消，并用原生neighbor_inspect任务读取sysfs及RTM_GETADDR。普通检测无Shell；显式FNR100测试及预设使用固定只读命令和环境/格式核验，失败回退内核。控制会话释放或重配置请求停止扫描，不引入AI、外部依赖服务或数据面。

Gateway校验EVENT/RESULT与派发Session/revision，Device Service验证域、端口和配置，并持有最新快照、90秒只读检测与每设备1024条/24小时的近期发现。近期数据放在设备记录，不复制进历史Session；配置修订、Session替换清空，离线保留至到期，Server重启不恢复。不将历史、缓存或租约作为在线状态。Task Service保留派发前统计基线与扫描完成summary，重复/迟到结果不重复合并。

Management组合只读查询、检测及扫描；HTTP Adapter经公开Application/Service工作。WPF NeighborView默认近期清单，自动推导直连范围，保留高级自定义；Blazor NeighborEditor默认智能配置，使用公开参考设备/能力接口，保留离线高级编辑。两客户端只共享无状态C#地址规范化/显示模型（shared/NeighborNetworks.cs），不复制设备业务状态机。Server能力协商与Probe能力、模板版本/应用修订分开。继续保持LAN按证据筛选、广播域全量且允许重叠，不推断上级方向。

规范、操作与本轮证据：[智能邻居配置](NEIGHBOR_SMART_CONFIGURATION.md)、API/PROTOCOL ADR-059章节。原始验证保留在[邻居发现](NEIGHBOR_DISCOVERY.md)。

## 四个操作工作区（ADR-055，紧凑操作台）

MaintenanceViews、RepositoryViews、DeviceViews中的配置区及ToolWorkspace采用本轮方案1。CompactWorkspace仅复用工具栏和原生Expander，按剩余窗口高度为历史/版本分配空间；不承担业务状态机。DeviceDirectoryView负责文件图标、左对齐名称与选择/加载通知，原RemoteDirectory/FileExchange继续处理目录和传输。维护入口可用性来自现有快照，并保留打开前HTTP复核；配置结果取TaskDetail实际参数，独立于正在编辑的表单。工具选中后才显示版本区，投放沿用原确认与兼容性查询。此节局部取代下方四页布局和文件名称居中说明，ADR-053设备详情与ADR-054组件保持。[方案和验证](COMPACT_WORKSPACE_VERIFICATION.md)。

## 字体与公共控件（ADR-054）

在ADR-053既有结构内，Themes/Controls合并Inputs、Menus、Icons；ControlChrome仅携带图标/搜索外观选项，WorkbenchIcon渲染随源码固定的Fluent原始几何。Typography维护统一字号与完整值行距，默认Microsoft YaHei UI，原用户字体偏好继续由ServerProfile保存。原WPF控件负责输入、选择、菜单、焦点与可访问性，模板不复制业务状态。Desktop.Tests中的组件样板只用于校准和回归，不进入产品导航。规范与证据见[组件视觉规范](COMPONENT_VISUAL_SPEC.md)及[Design QA](../design-qa.md)。

## 设备详情展示布局（ADR-053，方案3）

DeviceSummary 使用现有快照及异步归属结果表达持续摘要，SummaryBlock/FlexibleSummaryPanel 测量文本并分配宽度，不增加数据查询或预先截短地址。DeviceList 只为可见行附加视图序号，使用原 WPF 集合排序，不扩展 Device DTO。

DeviceViews 保留原详情页面及专用视图，PropertyInspectorWorkspace 用页面名称/ID选择器驱动原 TabItem；选项不直接承载 UIElement，避免 WPF 重新挂载页面。PropertySheet 在原模板过滤和排序之后展示单一连续属性表，检索只过滤原 PropertyRow 集合视图。语义类别和完整原值进入所选字段面板，能力值的紧凑摘要不改变源值或复制内容；宽窗口侧边、窄窗口下方停靠。首屏不再构建 SystemOverview 或 PropertySheetColumns，活动条默认收起。WorkbenchIcon、Themes/Controls、Ui、Typography 继续提供既有主题、字体偏好和线性图标。此节取代下方对应旧布局说明；Client、Service、Gateway/Probe 边界及 API/协议不变。唯一视觉目标和验证见根目录 [Design QA](../design-qa.md)。

## 原生 Workbench 展示层（ADR-049）

ADR-050 在该展示层统一表格居中，并以 Typography 动态资源应用本机字体/字号，SettingsView 通过既有 ServerProfile 保存偏好；InputFeedback 只区分鼠标与键盘的焦点提示，不控制业务选择。现有窗口继承资源，属性自动列宽重新测量而手动列宽保持；详见[外观与交互验证](APPEARANCE_INTERACTION_VERIFICATION.md)。

WPF 保留 ADR-048 的主工作区与接口结构。Themes/Controls 和 Ui 统一主工作区、详情、接口三级 Tab、密集列表及表单；PropertySheet 在 PropertyGroups 完成已应用模板过滤和排序之后组织连续阅读小节，存在模板字段展示覆盖时保留原顺序。DeviceViews 沿用稳定 PropertyRow 通知和集合视图，TableBehavior 管理首次完整列宽、无表头列宽手柄、手动换行及像素滚动。外观分组不进入共享字段目录、API 或业务状态机；不引入 UI 框架依赖。验证见 [WORKBENCH_VISUAL_VERIFICATION](WORKBENCH_VISUAL_VERIFICATION.md)。

## 来源摘要与接口状态（ADR-048）

WPF工作区名称为设备详情、远程维护、文件管理、配置管理、仓库工具。DeviceViews在接口状态内维护外壳端口/系统端口子分组，SamplingView只在末尾同排按钮点击后构建弹窗，保存通过原公开profile API。表单捕获目标设备、连接和资料版本，取消不修改配置，旧设备或连接不接收当前表单操作。

左下角出口IP仅使用公开source_ip，TelemetryViews优先通过现有Client IpLocation解析该地址并分行展示运营商/归属地；继续保留取消、缓存和失败退避。Probe双栈遥测与详细属性保持。本节取代下方ADR-045/047的对应摘要、接口导航与采样末页说明，没有Server/Probe职责或协议变化。

## 客户工作区与模板分类（ADR-047）

WPF工作区顺序为设备、维护、文件、配置、仓库工具；设置仅顶部弹窗。设备专用外壳端口和系统端口同级，接口采样设置在设备页末尾；属性表不再有接口分组或接口详情区。模板的storage_visible跟随已应用快照独立控制存储页，既有builtin_interfaces属性展示到其他信息。

shared/DevicePresentation提供稳定编辑分类；Blazor PresentationEditor按来源分类和字段ID维持编辑行，最终展示归组/排序独立计算，修改归组不移动编辑位置。DisplayLayout与WPF采用同一可见性语义。

Client RemoteDirectory复用公开exec任务读取目录；FileExchange只编排一个操作员请求的资产准备、传输与本地保存步骤，服务端File/Task状态仍为事实来源。未完成步骤保留原请求/任务，不自动替换；切换设备/连接取消本地等待。WPF DeviceDirectoryView、RepositoryViews负责设备目录与交互，ToolWorkspace负责只读仓库浏览及显式投放确认，均经公开API使用服务能力。内容沿用内置FILE协议，服务端资产持久化/清理语义保持，管理员上传通道另行实现。

本节局部取代下方旧工作区导航、接口属性和存储恒显说明。验证及入口见[CUSTOMER_WORKSPACE_VERIFICATION](CUSTOMER_WORKSPACE_VERIFICATION.md)。

## 服务端存量目录加载（ADR-046）

`internal/catalogupgrade`仅供probetemplate/enrollment持久目录加载使用，清理明确退役的模板展示字段与注册结果元数据；验证通过后先备份原字节，再复用各Service原子保存。API/协议/生成器不调用该入口，仍严格接受现行格式。模板文件缺失可告警建立空库，设备目录与型号绑定保持；其他损坏或未知结构保留明确错误。Management汇总启动告警，外部更新仍通过原模板与设备配置API。

## 设备分组与采集边界（ADR-045）

WPF DeviceViews按稳定组ID维护同级属性页；系统/资源/自定义/接口/其他之后保留存储与连接历史专页。PropertyGroups负责展示过滤与排序，TableBehavior测量所有行并在手动调整前适配列宽。shared/DevicePresentation.cs由WPF Client和Blazor生成器共享内置属性目录与默认分组。模板管理默认组、动态字段及自定义字段的展示元数据，专用存储/接口数据契约不变。

DeviceActions仅经公开API执行改名/模板应用；Explorer纳管池迁入左侧。Management更新用例限制设备级覆盖为InterfaceSampling，持久目录保存独立template_generation；Gateway复用原配置发送/ACK。LiveTelemetry区分仅接口重配与显式模板应用，SystemSampler保留硬件/静态模板/累计计数边界。

Device Service新增连续ConnectionPeriod视图，在既有Publish/End锁内观察真正上下线；在线Session替换不切断周期。API通过device.Query.Connections返回副本和Server秒数；WPF ConnectionHistory仅展示/校准，不复制服务端设备状态机。历史仍进程内有界，未引入数据库、完整审计或跨重启恢复。

Probe egress.cpp使用独立受限的原生HTTPS请求和静态TLS，固定外部出口服务；信任根、DNS/响应容量及取消边界见PROTOCOL。取消/采样不占控制线程，不调用设备HTTP命令。证书/时间验证不影响现有控制链路TLS未决边界。

本节取代下方属性折叠、任意设备采样和外部HTTP工具实现描述。证据与限制见[DEVICE_WORKSPACE_VERIFICATION](DEVICE_WORKSPACE_VERIFICATION.md)。

## 当前工作区与唯一版本路径（ADR-044）

生成器仅保留工程8/草稿3和当前运行模板，删除接口映射全链路及旧格式导入/迁移。DisplayLayout与WPF属性展示使用同一分类缺省和字段覆盖语义；Server仅保存展示元数据，不过滤采样事实。TableBehavior统一WPF表格的单行文本、原文复制、用户调窄后换行、像素滚动；PropertyGroups负责模板顺序、整行可访问折叠标题和阅读位置。Blazor浏览器脚本仅增加列宽交互，不承载项目业务状态。

Probe只用LiveTelemetry响应服务端配置，删除启动同步采集、模板准备连接、旧协商/降级。Device Service从ConfigTemplate建立waiting字段，并通过EVENT保存独立样本，REGISTER不再承载模板结果。公开客户端通过API/WS读取active_template和effective_metrics；现有Application边界、数据面及设备平台支持保持。

以下早期章节中的工程兼容、接口别名和旧REGISTER路径已被ADR-044取代，不是当前实现入口。

## 外壳网口统计（ADR-043）

模板switch_probe.counters独立描述硬件字节来源，原backend保留链路职责；PortCounterSampler由进程级SystemSampler拥有，执行有界查询、整数差分、失败/重置/来源切换与控制重连基线维护。Server校验port_counters_v1能力并沿用CONFIG_APPLY和完整switch遥测，公开API保留原始整数与统计口径。WPF主表以已确认外壳口为对象，同页展示曲线；逻辑接口与CPU/待核验口折叠展示，不能按关联eth名称拼接逐口流量。生成器工程7提供板卡预设及原始样本解析，不执行预览命令。无新增服务/数据库/持续数据通道；契约见[PHYSICAL_PORT_MONITORING](PHYSICAL_PORT_MONITORING.md)。

## 模板编辑职责（ADR-042）

Blazor的ConfigurationView只编辑模板全局设置，AttributeEditor编辑单个属性；EditorWorkspace/EditorLayout共享presentation状态，DisplayLayout提供字段目录和分组顺序，预览沿用相同规则。工程6导入旧工程，Server/Probe仅扩展可选端口编号语义，不承载编辑器导航或公式图。物理口仍由Probe现有采集后端读取，省略编号保留探测事实或未知；详见 [生成器入口](TEMPLATE_GENERATOR_MIGRATION.md)。


## 服务端纳管与配置所有权（ADR-041）

`internal/enrollment.Service` 独立持有 `repository-dir/devices/catalog.json`：发现资料、pending/managed/ignored、管理员名称、型号目录/别名/默认模板、设备绑定版本快照、配置覆盖和期望修订号。使用独占文件锁、临时文件同步替换、32MiB容量限制，不引入外部数据库。其持久化不恢复在线Session、任务、维护或最后已确认的实时数据。

Application 协調型号/模板解析和引用保护；HTTP Adapter只调用公开Service用例。Gateway完成REGISTER_ACK后在锁外持久登记发现，再发布Session；各业务派发复核纳管状态。每条支持热配置的控制Session拥有可取消下发worker，复用有界控制writer，结束Session后关闭连接并等待worker退出。Device Service保存已确认配置与独立实时样本，注册历史不变。

Probe `LiveTelemetry` 独立协调采集切换，CPU静态缓存和累计流量基线由进程级SystemSampler拥有。物理接口采集在独立可取消worker中执行，DSA/sysfs、swconfig或模板配置的有界只读厂商命令输出统一端口指标；板卡映射与逻辑接口别名分别管理。

WPF通过统一管理快照把未纳管设备放入单独池，管理员手动纳管/改名/选型号和搜索模板；设备设置下发采样覆盖。全部属性按实际应用模板的presentation分组、排序、展开/折叠，未分组置末尾。Blazor维护展示元数据和型号映射，发布不自动影响设备；工程5兼容1～4。具体协议/API与升级规则见 [MANAGED_PROBES_DESIGN](MANAGED_PROBES_DESIGN.md)。本节取代下文早期仅注册快照/启动选模板的规则。

本文定义 Management Server、Probe、对外客户端和传输通道之间的长期边界。Phase 1 已实现 TCP Session、并发 exec、进程内幂等、跨 TCP 结果补报与双向文件传输；Phase 2 已实现 Device Inventory 和内部查询；Phase 3 已实现持久 File/Tool Repository、兼容判断和管理端文件投放/下载导入，验证状态见 PROJECT_STATUS。Phase 5已增加统一HTTP/WebSocket Adapter；其他未交付客户端仍是后续方向，实际验证以PROJECT_STATUS为准。

Phase 4 新增 `internal/tunnel.Service` 与 C++11 `TunnelManager`，采用 Accepted ADR-021 / ADR-022 的极简 TCP Maintenance，不使用 FRP/xfrpc。验证状态以 PROJECT_STATUS 为准。

## ADR-040 监控与展示补充

Probe新增独立可取消的固定HTTPS出口命令worker，与内置采样/模板worker分离；接口过滤与累计字节/时长由Probe负责，SystemSampler在进程内跨控制Session保存基线。Gateway协商v2并校验，Device Service保留最新组与模板优先；API不直接访问内部采样状态。WPF通过公开快照维护窗口内最多10分钟的曲线，关闭释放，不引入时序数据库。source_ip仍是TCP对端，egress_ipv4/ipv6才是设备双栈探测。

DeviceProperties按registration与effective_metrics展示；未知值为横杠、悬停原因，来源/采样时间不拼入属性值。时长使用年/月/天/小时/分钟/秒无空格，最近心跳使用runtime.reported_at。模板工程4兼容1/2/3，监控配置包含接口白名单与出口周期。

## 当前客户端边界（ADR-037 / ADR-036 / ADR-035 / ADR-034）

项目仅保留两个 C# UI：Windows 原生 WPF 工作台与本机 Blazor 探针模板生成器。React 浏览器工作台、WinUI/Win32/WebView2 旧宿主及专属构建/Bridge/ConPTY 已移除；历史演进与验证见 [PHASE6_DESIGN](PHASE6_DESIGN.md)、[PHASE6_VERIFICATION](PHASE6_VERIFICATION.md)，不作为现存模块导航。

`windows/RouterWorkbench.Desktop` 为 .NET 10 / WPF，导航为设备、待纳管、维护、文件、配置、设置。`RouterWorkbench.Client` 只通过公开 HTTP/WS 查询快照、提交原幂等请求并管理取消和释放，不复制 Server 状态机。Core 保留用户配置、端点校验及外部客户端启动。

服务器地址在设置保存，启动自动恢复连接。HTTP 首连/重连回查快照，五秒恢复刷新按值更新稳定属性行。设备属性由 `DeviceProperties` 从 registration 快照完整展示，包含标准字段、扩展属性、模板版本和采集失败；不读取当前模板定义重新解释历史值。ADR-038 增加 runtime 开机时长和采样时间，全部属性表在同设备/同字段集合时仅通知变化值，保留选择；时长从最高有效单位连续显示到秒，年=365日、月=30日，离线不累计。

维护页仅展示公共 Web/SSH/Telnet 链接，启动前回查维护及设备 Session。主程序没有 WebView2、内置终端、工具管理或通用任务页。外部进程由用户管理，退出 UI 不撤销 Server 维护；SSH 账号和认证由外部客户端处理，工作台不保存或复制凭据，启动参数仅包含入口主机和端口。文件/配置结果在各自页面，committed/released 与最终 Task RESULT 分开。

`ui-windows.cmd` → `windows/build-desktop.ps1` 发布自包含 x64 EXE，不依赖 Node/Vite/C++/WebView2 构建。当前实现与验证见 [WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md)。

`src/ProbeTemplateGenerator` 使用 .NET 10 / ASP.NET Core / Blazor / Fluent UI，经本机 HTTP 供浏览器使用。C# 按 Feature 管理工程、编辑状态、表达式、规则、编译和公开模板 API 发布；JS 仅提供存储、下载、快捷键和主题。虚拟属性与规则编译为原 command/nvram/uci 模板，Server/Probe 不增加属性图或公式状态。版本 1/2 工程与旧原生草稿可导入，待定写入保留原请求。入口 `template-generator.cmd`，工程边界见 [TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md)。

测试在 Desktop.Tests 与 ProbeTemplateGenerator.Tests，协议对端只编入 Desktop.Tests。`tests/package.json` 的 Playwright 仅为生成器浏览器验证提供依赖，不进入产品构建。清理范围见 [UI_CLEANUP](UI_CLEANUP.md)。

## Phase 5 HTTP / WebSocket 模块与生命周期

- `internal/api.Server`是HTTP/WebSocket Adapter，`api.Run`组合两个listener及management.Server；`cmd/server`提供独立http-listen及容量配置。HTTP路径、DTO、错误、分页、幂等账本及事件wire属于Adapter；不访问Gateway连接表、Task/File内部map或Repository存储表。
- `management.Server`补齐CreateExec、Disconnect、Tasks和Revisions入口；任务摘要分页由Task Service查询，原Create/Resend/ACK/RESULT语义保持。Repository.ReadContent以reader回调提供流式内容，不把本地路径交给HTTP。
- Device/Task/File/Repository以原子变更计数提供刷新提示。Device不对心跳Seen计数；File在提交/释放/失败计数；Repository只对成功目录提交计数。计数不记录状态、不回调Adapter、不持有订阅者，原锁序保持。Maintenance只读原Service有界List，数据面未改变（仅冲突错误增加稳定sentinel供HTTP映射）。
- 一个250ms实时worker采样计数及Maintenance有界快照，四类resource_changed通知合并中间变化；files包括Repository目录。客户端首次/重连收到resync_required后查HTTP，不能把事件作为状态或审计事实。每客户端独立8槽队列、reader与writer、读写期限；满队列关闭客户端，业务和其他订阅者不等待。
- 幂等账本只记录HTTP请求指纹、完成信号与响应，默认4096项，不保存第二套业务对象，不淘汰身份。相同键等待原创建或返回原响应；指纹冲突409，满后拒绝新键。准入后创建context归API生命周期，客户端取消不撤销成功Task/Maintenance；流导入仍观察请求取消并完整校验再发布。task_id幂等和原Service资源生命周期保持独立。
- 原生HTTP监听限制默认128 TCP连接、32并行请求、64 WebSocket，JSON64KiB、流式资产1GiB及超时；查询输出分页。API添加的创建/重发受账本容量限制；Device/Task等原有Service历史与Repository磁盘保留政策不变。Response DTO隐藏LocalPath、内部诊断与数据面私有身份；临时三入口仍来自Maintenance Snapshot。
- shutdown先封闭API准入、取消准备、关闭HTTP及所有已升级WebSocket，再join已准入工作和实时worker；随后关闭Maintenance/Gateway/Repository。所有WaitGroup Add与closing检查在同一准入锁内，Close不会遗漏正在升级的连接。嵌入者若仅使用ServeHTTP，外层HTTP Server自行拥有网络超时、连接限额和listener关闭。
- 唯一新增Go依赖为gorilla/websocket v1.5.3（RFC6455适配）；Probe仍C++11且无变化。默认loopback HTTP8080，远程部署的认证/TLS/网络边界由部署层负责；认证/RBAC/审计/跨进程API恢复仍TBD。API.md为正式v1契约，ADR-023记录决定。

## Phase 4 模块与生命周期

- `management.Server` 组合 Repository、Gateway、可选 Maintenance Service，通过 `Maintenance()` 暴露用例。程序 `cmd/server` 默认启用独立 data listener；嵌入式旧调用方 `Config.Tunnel=nil` 保持原行为。
- `internal/tunnel` 拥有维护/入口模型、租期、固定三服务、端口池、随机流身份和一次性 token、并发准入、配对、Relay、释放和有界关闭历史。它只依赖 Control 接口，不能读取 Gateway 内部映射。
- Gateway 在 Session 发布后可返回 BindTunnel 撤销句柄；结束、替换、Disconnect、writer失败及Server关闭同步使旧句柄失效。发送在 writer 准入时复核当前 Session；状态按收到消息的连接身份调用 Service.Report。无可丢事件依赖；Device/Task/File 既有契约不变。
- Gateway → Device 的原锁序保持。Tunnel 调用 Bind/Enqueue 时不持 Tunnel 锁；Enqueue 不等待网络，Gateway 不回调 Tunnel 进行网络 I/O。Tunnel 锁保护准入、流表、token消费、状态与端口池；关闭 listener/socket 是本地撤销，数据复制和控制发送在锁外。WaitGroup Add 在准入锁内完成，closing 后不能新增被等待操作。
- Maintenance 先绑定Session再分配三端口，创建发布前复核；监听失败全部回滚。每个 accept 单独申请 pending+active 配额，超额直接关闭。共享 data listener 的无身份握手另有容量和期限，不能无限建立 goroutine。
- 每个支持 Tunnel 的 Device Session 有一个 Gateway 发送 worker、64 项控制队列；满时直接拒绝。写前复核 CONNECT context 和 Session。Maintenance 的 pending select 独立处理期限，writer 排队或阻塞不占用已释放维护资源；Gateway 在 Session 退出关闭 control socket 后 join 自己的 worker。
- 关闭顺序：状态closing → listener.Close → cancel → data socket reset → external socket.Close → 等待listener/已配对握手/Relay worker → 端口进入隔离 → closed/Released → 尽力入队CLOSE。本地释放不等待控制发送。未知身份的未完成data握手属于Server全局资源，由握手期限和Server Close管理。
- 端口隔离默认24小时，不占listener/worker，独立于128项关闭历史、最多池大小；只有Released且到达ReusableAfter才可重新分配，满池拒绝。原始TCP没有外部客户端维护身份，隔离不能保证超窗或Server重启后旧地址永久隔离；永久隔离需部署不重叠的池/地址，见ADR-022。
- DataHost可为IP或DNS主机名；Server在Create中最多5秒解析，优先IPv4并固定至本次维护结束。并行Create受MaxMaintenance限额限制，解析不持生命周期锁，失败不分配资源；下次Create刷新DNS。Probe收到的仍是数值IP，无DNS或多地址重试逻辑。
- Probe 每控制Session一个TunnelManager，固定目标查表，建流worker持有自己的两个socket，以可取消poll实现connect、握手与Relay；fd设置CLOEXEC与ExecForkMutex同步，发送使用MSG_NOSIGNAL，不改变Probe/exec的SIGPIPE disposition。控制Reader只验证并准入、取消或回收已完成worker，不执行Relay和DNS。
- Server每方向32KiB、Probe每方向16KiB，socket发送/接收有界；慢端使另一端停止读。EOF排空后传播SHUT_WR，反向仍可传输；Probe在读EOF后仍检查data socket错误，reset撤销本地连接，普通HUP仍排空缓冲。两端任一方向读写推进刷新整条连接空闲期限，默认24小时，绝对租期优先。Probe普通poll为20ms；持续HUP时另有20ms退让避免忙循环，取消还受调度影响。
- Probe默认8条流、每流一线程，硬上限64；Server每维护/设备默认8条。Probe默认Relay缓冲合计256KiB，线程栈由libc决定，低内存设备可调小；大量浏览器连接可同步提高双方限额。控制Session析构先取消Tunnel再收敛FileManager。
- Maintenance、配对token、流、监听与历史均为进程内状态；重启清空，新Session不能继承。仅固定loopback 80/22/23，不实现外部Adapter或通用映射。

## 项目目标

项目提供一套面向路由器和嵌入式 Linux 设备的远程运维平台。Management Server 统一管理设备连接、任务、文件与工具、临时 Tunnel 和对外 API；Probe 运行在设备侧，通过轻量、通用的控制原语执行管理端下发的操作。

基础部署应尽量保持简单：Management Server 目标为可直接运行的跨平台程序，至少支持 Linux 和 Windows，后续可扩展 macOS；Probe 面向 BusyBox、uClibc、mipsel、ARM、ARM64 和较老 Linux 内核环境。

## 系统边界

平台核心范围包括：

- Management Server 的 Probe TCP Gateway、设备会话、任务、文件与工具、Tunnel 和对外 API。
- Probe 的连接管理、协议编解码、通用任务执行、文件、进程、Tunnel 和设备信息能力。
- Web、微信小程序、Windows UI、CLI、MCP 与 AI Agent 使用的统一后端能力。

平台不把以下职责放入 Probe：

- 具体故障诊断知识或 AI 推理。
- 管理端的工具仓库与诊断编排。
- Web、移动端或桌面端的业务逻辑。
- 用户、权限和平台级审计策略。

## 总体架构

~~~text
Web / 微信小程序 / Windows UI / CLI / MCP / AI Agent
                         |
                  HTTP / WebSocket API
                         |
                Application / Service
                         |
       Device / Task / File / Tool / Tunnel
                         |
                  Probe TCP Gateway
                         |
                    Control TCP
                         |
                       Probe
                         |
        Exec / File / Process / Tunnel / Info
~~~

所有面向用户或 AI 的操作先沉淀为 Management Server 的稳定核心能力，再通过 Adapter 暴露。任何客户端都不得成为第二套设备控制实现。

## Management Server

Management Server 是平台核心程序，优先使用 Go 实现。

它长期承担以下职责：

- 接受并管理 Probe 主动建立的 TCP 控制连接。
- 维护 device_id 到当前连接和 session_id 的映射。
- 管理设备清单、在线状态与设备会话。
- 创建、调度、等待和查询任务，保存任务结果。
- 管理文件资产、工具及其向设备的传输。
- 创建和关闭临时 Tunnel，并追踪 Tunnel 状态。
- 通过 HTTP 和 WebSocket 向多个客户端公开同一套核心能力。

第一版可以采用模块化单体。模块可以在未来拆分，但不得绕过 Service Layer 或形成第二套核心逻辑。基础运行不得强制用户部署一组微服务、外部数据库或消息队列。

## Probe

ADR-039：`telemetry.cpp`内置采集和模板命令各自有界worker，主控制循环发送合并EVENT；CPU/内存/磁盘/网口独立周期，模板逐属性周期和公平轮转，不阻塞心跳。`device.Telemetry`持有Session最新组，Device Service产生模板优先effective_metrics，API只转交公开快照；监控变化更新devices revision。Server由RemoteAddr记录来源IP，WPF通过Client的IpLocation对所选公网IP异步查询，缓存和取消不进入Probe或核心设备状态。契约见 [TELEMETRY_DESIGN](TELEMETRY_DESIGN.md)。

ADR-031：`internal/routerconfig` 定义共享配置参数校验；Management 提供 CreateRouterConfig 用例，Task Service 持有不可变结构化规格，Gateway 检查 Session capability 并复用现有任务发送。HTTP 提供 config-tasks，WPF 配置表单通过 C# Client 使用公开 API，结果留在配置页。

Probe `router_config.cpp` 校验参数并构造 argv，`task.cpp` 复用有界执行器直接执行 PATH 中固件程序；`TaskManager` 在原 worker pool 中按接受顺序串行调度配置任务，允许 Exec 并行。模板 source 与配置任务共用参数/执行路径，来源仅 get。无新队列服务、协议消息或厂商库依赖，不改变身份、注册快照、文件与 Tunnel 生命周期。

ADR-029：`internal/probetemplate.Service` 独立拥有设备属性模板持久化、唯一名称、版本冲突与删除身份墓碑；`management.Server` 组合其生命周期，HTTP Adapter 通过它管理，Gateway 通过只读 Resolve 接口响应 0x05/0x06 准备连接。模板不作为 File/Tool 资产，不改变 Repository schema 或稳定资产身份。

Probe `collection.cpp` 在正常注册前执行所选模板的有界命令采集；准备连接不创建设备，采集后仍由 Gateway 的成功 REGISTER 发布完整 Device/Session 快照。registration中的扩展属性和失败摘要保持启动采集结果，重连复用本进程快照。ADR-039新增独立周期采集，实时结果单独保存。模板管理现由 ADR-034 的独立 C# / Blazor 生成器承担，WPF 设备详情继续显示公开 registration 字段。默认 ID 在 `identity.cpp` 从显式参数或一次 `nvram get SN` 取得。

ADR-038：Probe `system_info.cpp` 拥有本机架构、uname 内核和系统开机秒数采集；不执行外部采集命令。已知编译目标/端序和显式 arch 保持工具兼容含义，内核是启动默认值且显式模板优先。注册后立即首个心跳，后续按协商间隔重读系统时长。Gateway 校验 uptime/uptime_valid 后，按原 Gateway → Device 锁序同步调用 Heartbeat；Device 原子更新当前 Session 的最后运行采样及活动时间，拒绝旧会话/迟到采样，返回独立副本。API 通过原 Application 查询暴露 runtime，不读取 Gateway 表；既有 Session 历史仅多带最后采样，不保存每次心跳或推送逐心跳事件。

Probe 是与 Management Server 独立的设备端程序。Phase 1 起采用 **C++11 + CMake**：第一开发与验证平台为 Linux x86_64；后续通过 CMake toolchain files 适配 mipsel、ARM 和 ARM64 交叉工具链。目标侧只运行编译后的 Probe，不依赖 CMake。Probe 实现不得使用高于 C++11 的语言特性，并应避免不必要的重型运行时依赖。

Probe 应保持轻量：

- 主动连接 Server，负责注册、心跳、断线重连和会话状态。
- 提供 exec、文件、进程、Tunnel 和设备信息等通用原语。
- 在同一个 Probe 进程生命周期内使用 task_id 保证业务幂等；TCP 断开并重连后不得重复执行已经接受的同一 task_id。
- 支持受控并发和结果乱序返回。
- 只允许持久化必要的小规模状态。
- 不内置具体 AI 排障流程、设备诊断知识库或重型数据库。

Probe 的内部职责建议划分为 Connection Manager、Protocol Codec、Message Router、Task Manager、Exec Manager、File Manager、Process Manager、Tunnel Manager、Device Info Collector 和 Local State Store。该划分是职责基线，不要求 Phase 0 创建空源码目录或空实现。

### Probe 本地状态与幂等边界

device_id、Server 地址和基础配置是 Probe 本地持久化的基本候选。以下状态是否持久化仍为 TBD：

- 尚未上报的 TASK_RESULT。
- 已完成的 task_id。
- 正在执行的任务。
- 文件传输状态。

Protocol v1 Phase 1 只要求同一个 Probe 进程生命周期内的 task_id 幂等。即使 TCP 断开并重新连接，已经接受的同一 task_id 也不得再次执行副作用操作。Probe 自身重启后的 task_id 幂等、结果补报和任务恢复仍为 TBD。

## Server 与 Probe 控制链路

Probe 主动向 Server 建立 TCP 长连接，并按以下生命周期运行：

~~~text
CONNECT -> REGISTER -> REGISTER_ACK -> ONLINE
                                      |
                         HEARTBEAT / TASK / EVENT / FILE
                                      |
                    disconnect -> backoff -> reconnect
~~~

控制链路使用固定 20 字节二进制包头、JSON 控制载荷和 FILE_CHUNK 二进制文件块。协议细节以 [PROTOCOL.md](PROTOCOL.md) 为准。

连接与业务标识具有不同职责：

- message_id 用于一条 TCP 会话内的传输追踪。
- task_id 是任务的业务标识和幂等键；同一个 Probe 进程生命周期内可以跨 TCP 连接关联。
- transfer_id 标识一次文件传输。
- session_id 标识一次 REGISTER 后形成的设备会话；每次重连重新注册时生成新的 session_id。

## 控制面与 Tunnel 数据面

控制 TCP 只承载注册、心跳、任务、结果、事件、必要文件传输及 Tunnel 控制消息。Phase 4 的 SSH、Telnet 和 Web 持续交互流量通过 TUNNEL_CONNECT 建立独立数据连接，不使用 TASK 缓存。

~~~text
Management Server -- TUNNEL_CONNECT --> Probe
                                           |
                                separate data connection
                                           |
                                  Tunnel or Relay Server
                                           |
                               SSH / Telnet / Web traffic
~~~

该边界防止一个阻塞的交互会话或大量数据流量影响设备心跳和管理命令。Phase 4固定三服务数据面与生命周期由ADR-021及本文开头决定；通用Tunnel和平台认证仍未设计。

## API First 与多前端

Management Server 采用 API First。客户端只消费公开 API 或稳定的内部 Service 接口：

| 客户端 | 定位 | 边界 |
| --- | --- | --- |
| Web | 主要管理界面 | 只调用 HTTP API 与 WebSocket |
| 微信小程序 | 移动端快速维护入口 | 不依赖后端内部实现 |
| Windows UI | 原生WinUI 3中文Fluent远程维护工作台 | 仅调用统一HTTP API与WebSocket，不复制核心业务 |
| CLI | 调试与自动化薄客户端 | 不形成独立业务实现 |
| MCP 与 AI Agent | AI 工具入口 | 复用 Service 或 API，不直接拼装 Probe 协议帧 |

HTTP API 负责设备查询、任务创建、文件管理和 Tunnel 创建等请求响应操作。WebSocket 或等价实时接口负责设备上线离线、任务状态变化和日志流等实时事件。详细基线见 [API.md](API.md)。

## 分层与模块边界

~~~text
HTTP Handler ----\
WebSocket --------+--> Application / Service Layer --> Domain Services --> Probe Gateway
MCP Adapter ------+
CLI Adapter -----/
~~~

### Adapter Layer

Adapter 负责协议适配、参数解析、认证与上下文获取以及响应转换。它不得直接操作 Probe 连接、存储结构或拼装 TCP 协议帧。

### Application 与 Service Layer

该层组织用例和业务流程，为 HTTP、WebSocket、CLI、MCP 和其他 Adapter 提供可复用能力。接口应能被单元测试直接调用，不依赖启动 HTTP 服务。

### Domain Services

- Device：设备清单、设备状态和会话语义。
- Task：任务创建、调度、状态、幂等、等待与结果。
- File：文件资产及文件传输业务。
- Tool：工具元数据、兼容性和向设备投放。
- Tunnel：临时 Tunnel 的创建、状态与关闭。

File 与 Tool 在 Phase 3 由同一 `internal/repository` 包内的独立 FileService/ToolService 承载，共用目录事务以保护资产引用。存储与兼容性按 Accepted [ADR-019](DECISIONS.md#adr-019-phase-3-file-and-tool-repository) 实现；不替代 `internal/filetransfer` 的传输职责。

### Probe TCP Gateway

Gateway 只负责连接注册、帧编解码、消息路由和协议适配。设备、任务、文件和 Tunnel 的业务规则属于对应 Service。

## 跨平台原则

- Management Server 的目标平台至少为 Linux 与 Windows，后续可扩展 macOS。
- 不依赖 systemd、固定 Linux 路径或仅单一操作系统可用的基础组件。
- 早期优先保持单程序加配置文件的运行方式。
- 外部数据库、消息队列和拆分服务可以作为未来扩展，但不得成为基础功能的默认强制依赖。
- 平台相关能力必须收敛在清晰的适配层中。
- Probe 必须根据嵌入式 Linux 的 libc、架构、内核和资源限制设计。

## 当前建议仓库结构

下面是后续实现的职责示意，不表示应在 Phase 0 创建所有目录：

~~~text
repo/
├─ cmd/
│  └─ server/          Management Server 入口 后续
├─ internal/
│  ├─ probe/           TCP Gateway 与协议适配
│  ├─ device/          Device 与 Session
│  ├─ task/            Task Service 与状态
│  ├─ file/            File 与 Tool Service
│  ├─ tunnel/          Tunnel Service
│  └─ api/             HTTP 与 WebSocket Adapter
├─ probe/              路由器 Probe 独立源码 后续
├─ docs/
├─ tests/              按需建立的集成与协议测试
├─ AGENTS.md
├─ README.md
└─ CHANGELOG.md
~~~

当前实际仓库已包含 `cmd/server`、`internal/protocol`、`internal/gateway`、`internal/task` 和 `probe`，实现了 Phase 1A 的 framing、注册、心跳与基础重连，以及 Phase 1B 的 TASK、TASK_ACK、TASK_RESULT、exec、timeout 和基础任务状态，Phase 1C 的并发、去重和跨连接补报；Phase 1D 已增加 internal/filetransfer 与 Probe FileManager；Phase 2 已增加 internal/device；Phase 3 已增加 internal/repository 与 internal/management。示意中的其他模块尚未创建或实现。

## 已确认的架构约束

- Probe 主动连接 Management Server。
- 控制 TCP 与 SSH、Telnet、Web Tunnel 数据连接分离。
- Management Server 优先使用 Go，并作为跨平台核心程序。
- 基础运行尽量不强制依赖外部数据库、消息队列或一组微服务。
- API First，多前端只作为 Client 或 Adapter。
- 核心业务、API Adapter 和 Probe Gateway 清晰分层。
- Probe 保持轻量，不承担具体 AI 故障诊断逻辑。
- Probe 从 Phase 1 起采用 C++11 + CMake，首轮 Linux x86_64 验证，后续通过 CMake toolchain files 适配 mipsel、ARM、ARM64。
- 项目状态和交接信息必须持久化在仓库文档中。
- 项目按阶段保持可构建、可运行和可测试，不用占位实现伪造进度。

## 待讨论

Device/Session 生命周期、历史保留、进程内存储与 Gateway 边界已由 [ADR-018](DECISIONS.md#adr-018-phase-2-device-management) 确认；长期存储引擎、重启恢复、备份与迁移仍未决定。

- Management Server 的内置持久化方案、数据模型、备份和迁移策略。
- 平台认证、授权、设备认证、链路加密、密钥管理和审计策略。
- mipsel、ARM、ARM64 的具体交叉工具链版本，以及最低内核与 libc 兼容矩阵。
- 通用Tunnel及平台访问控制；固定三服务的Relay/租约已由ADR-021决定。
- Probe 自身重启后的 task_id 缓存、未上报结果和任务恢复策略。
- Server 重启后的任务与会话恢复策略。
- Repository 之外的长期存储、在线备份/迁移、物理 GC 与跨平台实机兼容矩阵仍待后续阶段；Phase 3 范围已由 Accepted ADR-019 决定。新版 Probe 默认采集 kernel；libc/model 仍依赖模板，任何字段缺失时兼容判断不能假定已知。
- Management Server 的配置格式、日志、指标和运维接口。
- API 的资源模型、认证、错误格式和实时事件协议。
- 各前端的实现顺序和技术栈。

## Phase 1C 实际模块职责

- Probe TaskManager 在 RunClient 进程生命周期中拥有任务表、固定 worker pool 和有界结果缓存；RunSession 只处理连接。worker 不持有 socket，TCP 断开时继续执行已接受的 exec。
- Connection / Message Router 负责 ACK、冲突 ERROR、心跳和缓存结果发送；同一网络线程串行发送帧。在线 poll 最多等待 50 ms，每轮发送一个待补报 RESULT 并继续处理控制消息。
- TaskManager 的登记、状态和缓存共用互斥锁；执行和网络写不持有该锁。新连接开始新的补报轮次，不清空任务身份。默认并发数和缓存计费见 PROTOCOL.md。
- Server Task Service 保存任务及各次 session_id/message_id 派发记录，处理 ACK、RESULT 幂等和等待；Gateway 适配 CreateExec / ResendTask / WaitTaskResult / TaskSnapshot，不引入外部 API。

## Phase 1D 实际模块职责

- Go internal/filetransfer 负责文件任务准备、流式源/接收器、FILE 字段校验、会话传输状态和本地提交事实；不实现 File/Tool Repository。internal/task 保存不可变文件参数和业务结果。Gateway 的 CreateUpload/CreateDownload 为内部用例入口，路由 FILE 帧并提供优先级串行发送适配。
- C++ FileManager 为单 TCP Session 拥有一个文件 I/O worker、deadline watcher、有界 FIFO 和接收邮箱；Reader 登记 TASK/BEGIN 并路由控制帧，不执行磁盘读写。TaskManager 拥有跨 Session 的文件身份、transfer_id 绑定、状态和结果缓存。旧 FileManager 停止并收敛结果后才建立新会话。
- 两端 writer 对完整帧串行化；等待中的控制发送优先于 CHUNK，文件 END 在此前数据全部写出后发送。发送缓冲目标为 64 KiB，避免大量提前排入的文件字节抵消控制优先级。
- 临时文件在目标同目录创建，完成校验后 rename 或无覆盖 hard link 发布。Server FileSnapshot.Committed 表示下载本地提交事实，与 Probe 任务最终确认分开记录，支持 done ACK 丢失后保留完整文件但任务失败的已确认契约。

## Phase 2 实际模块职责

- `internal/device.Service` 拥有按稳定 device_id 索引的 Inventory、注册资料、当前 Session、在线状态、时间及有界历史。它使用 RWMutex，写操作 Publish/Seen/End 与只读 List/Get/Sessions 在内存中完成，不依赖 TCP、Task、File 或数据库。
- Gateway 保留 `device_id -> *session` 的传输路由映射、socket、writer、连接 Reader 和协议心跳 deadline。它在完整 REGISTER_ACK 写出后，持 Gateway 锁按发布顺序安装连接并调用 Device.Publish；最新发布的 Session 替换旧 Session，旧 socket 在锁外关闭。注册失败及 ACK 写失败不创建 Inventory。
- 连接结束、主动 Disconnect、writer 失效及 Server Close 撤下当前传输映射并同步调用 Device.End。Gateway 以当前连接指针保护清理，Device 再以 session_id 保护状态；旧回调不影响新 Session。合法 HEARTBEAT、TASK_ACK/RESULT 和既有 FILE 路由成功处同步 Seen；3 倍心跳 deadline 继续由连接适配层执行。
- 锁顺序为 Gateway → Device，Device 不回调 Gateway。Device 锁和 Gateway 连接表锁均不跨网络/文件 I/O；writer 失败通知可取得 Gateway 锁，Gateway 不在连接表锁内取得 writer 锁。Device 状态不依赖现有容量 128 的可丢 Events 通知。
- online/offline 是设备业务状态。Session replaced 时直接结束旧 Session 并发布新 Session，设备保持 online；LastOnlineAt 更新为新发布时刻，LastOfflineAt 保持原值。设备真正下线才更新 LastOfflineAt；未经历离线时为零值。
- 当前 Session 之外，默认保留最近 64 个已结束 Session 及对应注册快照；按结束操作顺序淘汰并暴露计数。离线设备、首次/最近时间和累计数保留至 Server 进程结束，设备数量无自动淘汰。只记录 Session 状态历史，不存心跳时序或审计事件流。
- `Server.Devices()` 暴露 `device.Query`，返回独立查询副本；当前/最近 Session、时间、历史容量与字段语义见 API.md。Task/File 规格、派发关联、幂等和提交事实仍属于原 Service；Device 历史淘汰不影响这些记录。

## Phase 3 实际模块职责

- `internal/repository.Store` 拥有持久目录、文件系统锁、schema_version=1 JSON 目录和资产/工具引用的一致提交；FileService 管理不可变字节、资产身份、查询与归档，ToolService 管理工具、版本、产物、约束与归档。稳定 UUID tool_id/asset_id/artifact_id 不因归档、去重或存储路径改变而重用，artifact_id 全仓库唯一。
- `internal/management.Service` 组合 Repository、device.Query 和 FileTasks 接口，实现兼容查询、资产上传、工具投放、下载暂存与显式导入、Operation 关联。它不编码 wire、不访问连接表；现有 gateway.Server 实现 FileTasks，所有传输仍由 internal/filetransfer 与 Task Service 执行。
- 程序入口改由 management.Server 组合持久 Store 与 Gateway。`-repository-dir` 默认 `./data/repository`，相对启动工作目录解析并记录绝对路径；创建目录或锁定失败即启动失败。Close 先停止 Gateway/传输，再关闭仓库。基础部署无新增外部服务或 Go 第三方依赖。
- 目录为 `blobs/sha256/<两位>/<摘要>`、`metadata/catalog.json`、`staging/` 和 `repository.lock`。资产名称只作标签，不参与路径；相同 SHA-256 的多个资产共用一个完整 blob。blob 先完整发布、再发布元数据；失败可能留下未引用的完整 blob，但不能留下可用资产指向半文件。
- Linux 使用 flock，Windows 使用 LockFileEx 锁定整个 Store 生命周期，进程退出由 OS 释放；锁文件保留初始化标记，防止仅有工具元数据的目录在 catalog 丢失后被静默初始化为空。目录移动后所有 blob 路径重新由摘要解析，ID 保持。
- 元数据提交在仓库互斥锁内复制当前目录、写同目录临时文件、Sync/Close 后发布；Linux rename，Windows MoveFileExW(REPLACE_EXISTING | WRITE_THROUGH)。失败不发布内存状态；成功重开可读。启动拒绝损坏格式、重复身份/JSON key、未知 schema、非法引用或缺失/大小不符 blob；使用或去重复用内容时重新验证摘要。该边界不承诺所有文件系统/突然断电下完整恢复。
- 管理端以设备注册快照执行兼容规则，缺失受限字段为 unknown，仅 compatible 可投放；实际传输派发前执行通用 Session 检查。版本/资产解析后，以仓库读锁准入文件准备与派发，归档/Store Close 等待此段完成；不把仓库锁扩大为 Gateway/Device 锁。派发检查在 writer 内短暂获取 Gateway 锁、核对当前连接并记录 Task 派发，锁释放后写帧；后续替换不改变已准入任务的原目标。
- `internal/filetransfer` 只增加可选 Expected 内容校验及 Released 句柄释放事实；Gateway 只增加可选 Session 前置条件。没有第二套文件传输、Repository wire 字段或 Probe 变化。下载按 Committed + Released 导入，Task RESULT 单独呈现，ACK 丢失不回滚已完整提交文件。
- 归档保留全部元数据与 blob，阻止新引用/投放而不取消已派发任务；版本标签不重用。崩溃遗留暂存、未引用 blob 和元数据临时文件只报告，不自动 GC。当前进程可以清理自己已释放的下载暂存，已提交文件须先导入。停服后备份整个目录；在线备份和迁移工具未实现。
- Repository 是进程内目录索引加单写者 JSON 快照，面向小规模仓库；内容流式处理，但元数据整体读写，资产/工具/版本数量和磁盘使用无自动淘汰。Operation/Task/transfer/Device/Session 仍为进程内状态，重启不恢复或自动重发任务。当前平台适配只实现 Linux/Windows；Phase 5通过Adapter复用以上仓库行为。

## EasyTier 异地组网边界（ADR-064）

`internal/overlay` 管理网络定义、成员身份、持久操作、上游 Web 配置 API 与归一化观测；由 Management 组合原 Repository/File/Gateway 能力。HTTP/WS 与 WPF 仅访问公开 Application API，不直接操作上游账号或 Probe 连接表。EasyTier 是可选独立数据面，不是基础 Server 启动必需服务；同主机 Web API 与远端 core 配置接入分离。

管理连接失效不拆除网络，显式停用与 Maintenance 租约完全独立；上游写结果不确定不得自动重放。三层首轮实现、部署限制与二层后续门槛见 [OVERLAY_NETWORK](OVERLAY_NETWORK.md)。其他架构边界保持。
