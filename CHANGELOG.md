# 变更记录

## 2026-09-11 GCC5.4/MIPS免交互构建入口

- 新增probe-build-gcc54.cmd，从同目录password.txt读取编译机密码；首次自动安装桌面gcc-5.4.tar.gz，之后复用缓存，在/root/router-probe-gcc54保留独立构建记录与最近成功产物。
- 为该旧uClibc SDK在远端源码副本适配C++标准库声明差异，保持本地产品源码与原GCC5.2 ARM入口。密码不写入脚本、不打入源包，缺失/空/多行密码在上传前失败；不自动部署设备。
- FM160-CN实机通用AT身份、自动选口、占用与恢复验证见[说明](docs/GCC54_AT_DEVICE_VERIFICATION.md)。

## 2026-09-11 通用 AT 身份与串口自动发现（ADR-060）

- 模板新增默认关闭的蜂窝 AT 自动采集开关和周期；自动发现 USB 串口，固定查询 ATI/IMEI，不要求填写厂家或端口号。
- 按 USB 父设备分组、占用端口跳过、每轮重枚举；串口编号变化后重新识别，无可用口、部分成功、超时与过期明确展示，不停止拨号进程。
- 增加 cellular_identity_v1 能力、结构化最新快照 EVENT 与只读 Cellular API；旧 Probe 应用AT模板明确拒绝，不静默部分应用。
- WPF 设备详情新增“蜂窝模块”属性页，提供端口选择、ATI/IMEI原值、查询状态与观测时间；刷新仅回查快照。
- 本轮仅通用身份链路。SIM、驻网/信号和厂商扩展尚未实现，真实模块与拨号共存仍需验收，见[说明](docs/CELLULAR_AT.md)。

## 2026-09-10 智能邻居配置与近期发现（ADR-059）

- 生成器默认智能配置：在线参考设备、采集网络、结果开关及30秒被动刷新；域ID自动生成，原始接口、端口、租约与只读命令收进高级设置。支持复制设备已应用配置，明确设备验证状态及模板作用范围。
- 增加公开Server能力协商及只读网络检测；旧Server在发送邻居模板前明确阻止发布，兼容增加字段级错误；Probe能力、模板保存与设备应用分别处理。
- FNR100提供需显式测试/选择的只读预设，CPU口排除，wan及lan1～4均可计入LAN；环境或格式不符退回内核FDB，不按型号静默执行命令，不推断上级方向。
- WPF主动发现自动选择直连IPv4范围并显示数量/预计耗时；高级自定义规范化主机地址前缀并提前提示越界，保留Server/Probe最终验证、进度和完成统计。
- 默认显示Server有界近期发现（每设备1024条、24小时、仅内存）；60秒主动响应证据过期转为最近发现，不突然丢失默认记录，也不宣称历史设备在线。重启、Session/配置修订切换清空。
- 修复仅包含邻居配置的模板应用校验遗漏。范围、兼容性与未完成实机验收见[本轮说明](docs/NEIGHBOR_SMART_CONFIGURATION.md)。

## 2026-09-11 Probe 构建入口

- Probe构建恢复到root@10.1.1.128，从本地password.txt读取登录密码；源码经SFTP上传后编译，成品为/root/router-agent/router-agent。管理服务器地址与Linux Server构建上传保持原设置。

## 2026-09-11

- 默认Server地址改为47.119.168.150，HTTP API 8888；Server监听所有本机IP，WPF与模板生成器的新配置使用新地址。
- 新增Linux AMD64一键构建与SFTP上传/root/agent-server，默认读取本地私钥及口令；Probe默认接口br0、eth0、eth1、usb0，无接口询问。
- Windows移除SSH用户名/密码设置及复制，已关闭/失效维护不显示连接地址，文件管理默认/tmp/root，精简邻居发现等重复说明。

## 2026-09-10 LAN 与本机广播域邻居发现（ADR-056）

- 设备详情分别显示LAN下接设备与本机广播域设备的IP/MAC、转发端口、主机名及来源；两类允许重叠，端口未知/冲突另列，不查询或推断上级设备。
- 新Probe支持被动邻居/FDB/可选租约采集及手动限速IPv4主动发现、停止发现；旧Probe明确提示升级，缓存与租约不冒充在线。
- 模板生成器新增邻居配置页，贯通工程保存、导出、发布与更新；增加公开邻居查询/扫描/取消API及neighbors_v1协议能力。
- [使用、验证与部署限制](docs/NEIGHBOR_DISCOVERY.md)。当前厂商ARM部署待编译机认证及实机验收。

## 2026-09-10 Server 重启后的 Probe 循环断线修复

- Server 对格式合法但任务 ID 已不存在的补报结果记日志并忽略，保持控制连接和后续心跳/新任务正常；只需更新 Server，无需重启或升级 Probe。
- 补充旧结果的设备/会话/消息/任务 ID 与忽略原因，以及 ERROR 的错误码和具体原因日志；不记录命令输出。
- 保持未知任务不重建、已知任务幂等与冲突校验；不增加跨进程任务恢复。[验证与成品](docs/SERVER_RECONNECT_FIX_VERIFICATION.md)。

## 2026-09-10 四个操作页采用紧凑操作台（ADR-055）

- 文件页扩大目录浏览区，名称配图标左对齐，传输记录可展开；上传默认当前已读取目录，下载/继续/保存按实际状态启用。
- 维护链接与打开/复制操作同行，关闭的入口禁用并标明历史状态；维护记录收为可展开底栏，保留外部客户端及原租期规则。
- 配置表单随操作显示所需字段，结果紧随表单并注明实际设备、操作和键名；保留不自动提交、不重启的语义。
- 仓库工具选中后展开版本区，空仓库/无匹配/同步失败提供对应说明和恢复操作；投放继续显式确认。
- 保持设备详情方案3与共享字体、控件和主题。[验证与成品](docs/COMPACT_WORKSPACE_VERIFICATION.md)。

## 2026-09-10 字体与控件视觉统一（ADR-054）

- 保持方案3的信息与导航结构，统一按钮、输入、下拉框、复选框、菜单、提示、密码框和滚动条外观，覆盖浅深主题及焦点/悬停/禁用状态。
- 默认使用微软雅黑UI并保留用户字体设置；下拉、搜索、复制、关闭及导航统一使用Fluent矢量图标，修复图标默认尺寸覆盖模板的问题，箭头不再随字体偏移。
- 减少属性表中版本/地址等普通值的粗体，突出选中标签；完整值采用等宽字体和更清楚的行距。原业务、数据与操作保持。
- [组件规范](docs/COMPONENT_VISUAL_SPEC.md)与[本轮实际运行Design QA](design-qa.md)。

## 2026-09-10 可检索属性与完整值面板（ADR-053）

- 按用户选定的Product Design方案3实现：顶部连续设备摘要，详情页面由下拉选择，属性按原顺序连续阅读，支持按名称、字段标识和完整值检索。
- 选中属性打开完整值面板，支持操作显示项数并可查看/复制原始字符串；窄窗口下方停靠，保留复制和滚动。原有页面、模板可见性及全部功能保持。
- 工作区活动默认收起并可恢复；沿用浅色/深色/系统主题、字体设置及原生线性图标。未改业务逻辑、API或Probe协议。
- [运行对照、Design QA和成品](design-qa.md)。

## 2026-09-10 设备快照空时间修复

- 修复历史会话的开始/最后活动时间为空时，客户端设备快照解析失败并持续显示“快照刷新失败，正在恢复”的问题。未知时间保留“—”，无需修改后端数据。
- [验证与修复版程序](docs/SNAPSHOT_REFRESH_FIX_VERIFICATION.md)。

## 2026-09-09 弹性摘要与系统信息层级（ADR-052）

- 顶部改为名称/ID加六项独立摘要，固件与IP优先利用横向空间，运营商和归属地独立显示。IPv6保留原文，仅实际空间不足时省略并提供完整提示。
- 二级页面采用带背景和边界的Tab导航带，增加间距、hover反馈与清晰的激活态。
- 系统信息增加状态概览，详细字段按基础身份、系统与硬件、运行与连接、固件与版本组织；重要值突出、长值换行，窄窗口自动合并。保留现有页面、模板顺序与可见性、复制和列宽操作。
- [验证与成品](docs/SYSTEM_INFORMATION_HIERARCHY_VERIFICATION.md)。

## 2026-09-09 设备摘要与双列系统信息（ADR-051）

- 删除左下重复摘要，设备列表延伸至底部；新增随搜索、过滤、排序更新的视觉序号，支持设备名/ID/状态升降序及方向提示。
- 顶部以名称/ID和紧凑横向元信息显示设备摘要，长 IPv6 截断并保留完整提示；模板更新操作沿用原入口。
- 系统信息使用按语义分组的双列连续属性区，窄窗口/大字号自动合并。标签和值对齐、长能力自然换行，保留模板排序/过滤、原文复制、手动列宽与稳定刷新。
- 一级模块采用较大半粗图标标签、二级页面轻量显示，各详情页共用单色线性图标及分组标题带。输出区缩小并弱化时间/类型，隐藏再显示恢复拖拽高度。
- [验证与成品](docs/DEVICE_DETAILS_LAYOUT_VERIFICATION.md)。

## 2026-09-09 列居中、字体设置与选中状态（ADR-050）

- 所有表格列标题、字段和值统一居中，保留连续 Inspector 与原工作区结构。
- 顶部设置新增字体、字号、预览、应用并保存和恢复默认；主窗口、菜单与工具窗口立即生效，重启恢复，兼容旧配置。大字号同步调整行高、输出窄列和曲线标签。
- 修复首单元格焦点被画成选中底色及自动虚线框，区分行/Tab 悬停、下拉候选和实际选择；键盘导航保留独立焦点提示。修复右键切换设备时错误清空单选集合的异常。
- 验证方法、成品与限制见[外观与交互验证](docs/APPEARANCE_INTERACTION_VERIFICATION.md)。

## 2026-09-09 原生 Workbench 视觉与连续属性页（ADR-049）

- 主工作区、详情分类、接口子页采用三级 Tab；收紧左侧导航、摘要、列表与输出，减少边框和交替色条，统一浅深主题与对齐方式。
- 系统/资源信息改为连续 Property Sheet，以分组标题带和细线组织；模板显式顺序与可见性优先。保留完整值复制、首次适配列宽、手动调窄换行、虚拟化与稳定刷新，新增无表头列宽手柄和键盘调宽。
- 设置、纳管、改名、模板应用、采样与投放窗口统一表单间距和操作区；最小窗口自动收窄左栏，保持主工作区完整可见。
- 保留 C# / WPF、五个工作区及业务调用链，无新增 UI 依赖。[验证与成品](docs/WORKBENCH_VISUAL_VERIFICATION.md)。

## 2026-09-09 来源 IP 摘要与接口状态（ADR-048）

- 所选设备拆分“出口IP”和“运营商及归属地”，只显示服务器观察到的来源IP，并解析该IP；缺失、内网和查询失败保留未知状态及原因。
- 外壳端口、系统端口收纳到“接口状态”，点击分组展示；末尾同排“接口采样时间”打开弹窗，支持周期、接口范围与恢复模板默认，移除独立采样页。
- 工作区更名为设备详情、远程维护、文件管理、配置管理，仓库工具保持末项。验证与产物见[接口工作区验证](docs/INTERFACE_WORKSPACE_VERIFICATION.md)。

## 2026-09-09 客户工作区与模板分类（ADR-047）

- 设备列表/设备发现更名，发现页按钮对齐；设置改为顶部弹窗。设备模板更新支持搜索选择及明确应用，取消不提交。
- 外壳端口和系统端口并列，接口采样设置置于末页；删除接口详情、接口属性区和图上说明。模板增加独立存储页展示开关，磁盘属性仍可分别配置。
- 生成器按内置属性类别编辑与选择，修改展示分组不跳走或重新排列编辑行；预览仍按最终分组/顺序显示。
- 文件页改为设备目录浏览与本地上传下载，默认/tmp，可自定义目标路径；自动串联原文件协议流程，保留原请求重试。移除文件资产和Windows工具上传入口。
- 工作区末尾新增仓库工具：搜索名称/说明、查看版本、右键选择版本和兼容产物后确认投放，支持自定义目录，默认/tmp；不自动执行，管理员上传通道后续单独实现。
- 构建、测试、产物与限制见[客户工作区验证](docs/CUSTOMER_WORKSPACE_VERIFICATION.md)。

## 2026-09-09 服务端模板启动修复（ADR-046）

- 修复旧模板及设备绑定快照中的interface_aliases、旧注册结果字段使Server启动失败；自动保存原目录备份后清理退役元数据，保留设备资料、模板指令和绑定版本。
- 空模板库或已初始化后缺失模板文件不再阻止启动；缺失时明确告警，可在生成器发布模板后由客户端显式应用。公开发布仍拒绝旧字段，其他目录错误增加具体路径。
- Windows/Linux相关验证及当前数据副本的真实程序启动发布通过，见[验证记录](docs/SERVER_STARTUP_VERIFICATION.md)。

## 2026-09-09 设备工作区与原生出口（ADR-045）

- 属性改为系统信息、资源监控、自定义分组、接口信息、其他信息并行页签，保留独立存储与连接历史；默认及模板属性均可归组。列宽适配完整内容，移除展开/折叠按钮。
- 内存百分比附已用/总容量，双栈出口摘要换行；待纳管并入左侧资源管理器。新增模板实际版本与更新图标、设备右键复制/改名/模板更新（支持同版本重新应用）。
- 设备采样仅允许修改接口周期与接口集合；其他采样由模板应用控制，接口重配不重跑其他采集。CONFIG_APPLY增加应用代次以区分主动应用与重试/重连。
- 连接历史新增连续在线/离线时长，在线Session替换不重置，服务器断连暂停客户端计时。Probe内置有证书校验的原生HTTPS出口探测，无curl/wget依赖。
- 构建、验证与实机限制见[设备工作区交付](docs/DEVICE_WORKSPACE_VERIFICATION.md)。

## 2026-09-09 属性展示与UI完善（ADR-044）

- 删除接口显示名及接口映射全链路；属性页可按内置分类/具体字段选择展示，磁盘和网口明细默认隐藏，采集和专用页面保留。属性名称不再附标识，右键可复制原标识与完整值。
- 分组改为整行可折叠标题，取消属性表列头排序，按模板固定顺序。统一像素滚动、自动行高和维护链接间距；默认单行，手动调窄列后可换行，复制保留原文。
- 仅支持工程8/草稿3，删除旧格式迁移、旧模板准备/启动参数、同步启动采集/REGISTER结果快照、telemetry_v1降级与缺失uptime_valid推断。Server/Probe使用当前受管配置和遥测基线；设备平台适配继续保留。
- 配套构建和本机验证完成，厂商ARM/物理DPI及sanitizer限制见 [本次交付](docs/UI_REFINEMENT_VERIFICATION.md)。

## 2026-09-09 外壳网口监控（ADR-043）

- WPF将物理端口、逐口实时速率、累计流量、统计时长与曲线统一在网口页，系统接口与CPU内部口放入折叠区域。
- 模板可独立指定硬件字节来源；新增swconfig MIB与厂商原始计数后端、精确64位统计、错误断点和旧Probe能力提示。生成器增加FNR100板卡预设与计数样本预览，工程7兼容旧格式。
- 本机验证已完成，配套ARM与厂商部署状态见[专项说明](docs/PHYSICAL_PORT_MONITORING.md)；不表示其他芯片或固件已适配。

## 2026-09-09 模板配置工作区（ADR-042）

- 新增独立“模板配置”页，内置监控、分组与排序、接口显示名和物理端口采集各有页签，不再占用属性编辑页顶部。
- 展示属性可直接选择分组与组内顺序，统一配置表自动列出字段并支持搜索、上移/下移；重命名、复制、删除同步布局，预览按最终分组顺序展示。
- 物理口按采集方式区分必填与高级信息，自动生成内部标识，提供字段示例、匹配依据和别名覆盖提示。芯片端口号可留空，不再把未知值写成0；旧数字端口仍兼容。
- 生成器工程6兼容1～5与原草稿；使用省略编号的配置需同步更新Server/Probe。


## 2026-09-09 服务端纳管与动态探针配置（ADR-041）

- 新设备先进入待纳管池，支持忽略/恢复、单台及批量纳管；管理员可改名、选型号和搜索模板，服务端拒绝未纳管设备业务操作，资料跨重启保存。
- 模板由服务端型号目录和管理员指定，保留设备绑定版本；发布后显式应用。周期/接口范围/属性周期可在线修改，离线下次连接同步，显示待确认/已生效/失败/需升级状态。
- CPU静态详情每Probe进程仅探测两次，普通重连复用缓存；CPU使用率与当前频率继续周期更新。
- 支持DSA/sysfs、swconfig和可配置厂商只读命令的物理端口探测，分开显示上联、物理链路、管理状态与CPU内部口；未知状态不推断为DOWN。
- 生成器新增属性分组、组/字段数字排序、接口展示别名与型号默认模板维护；WPF支持展开/折叠和物理端口表。工程5兼容1～4，纯内置监控/布局模板不需要占位命令。
- 新增managed_config_v1协商与版本化CONFIG_APPLY/ACK；新版Probe忽略旧template-id/template-name选型参数。升级行为与已验证/待实机范围见 [改造说明](docs/MANAGED_PROBES_DESIGN.md)。

## 2026-09-09 ARM构建脚本修复

- 修复Windows换行导致远端Bash报 `set: pipefail` 无效选项：上传后自动规范化脚本行末CR，本地脚本使用LF。已用GCC5.2真实生成包含最新监控能力和 `eth0,eth1,br0` 默认接口的ARMv7/uClibc探针。

## 2026-09-09 设备展示与监控改进（ADR-040）

- Probe增加IPv4/IPv6出口探测、接口名白名单、每接口累计收发流量及统计时长；构建、模板与启动参数可配置，普通控制重连保留计数基线。
- 默认读取 `nvram get softver` 获取完整固件版本及 v 前设备型号，模板可分别覆盖。
- Windows增加独立双栈归属地/运营商、网口双击收发曲线；设备列表/摘要调整、在线绿离线灰、表格居中、无空格中文时长、缺失值横杠与悬停原因；移除重复按钮与冗余说明，保留对应功能。
- 兼容新增telemetry_v2协商、原因/秒数单位和模板配置；生成器工程4兼容1/2/3。验证及未完成实机范围见 [MONITORING_V2_VERIFICATION](docs/MONITORING_V2_VERIFICATION.md)。

## 2026-09-08 生成器启动修复

- 修复重复点击 `template-generator.cmd` 因运行中的 EXE 被占用而触发 MSB3026：同仓库同端口的编辑器直接复用，运行与仅构建输出隔离；其他程序占用端口时提示更换端口，成功返回不再停留在批处理暂停提示。

## 2026-09-08 独立周期监控与连接来源（ADR-039）

- Probe增加CPU型号/频率/详细架构与位数，以及CPU/内存/文件系统空间/网口速率；内置组与模板逐属性独立周期，模板同标识结果优先且失败不回退。
- EVENT能力协商、Session最新样本、API effective_metrics/source_ip及合并刷新；原注册快照和工具兼容含义保持。
- WPF增加详细CPU、容量自动换算、存储/网口表格、Server观察的来源IP和公网归属地查询；生成器工程3兼容旧工程并独立编辑监控与属性周期，配置重启Probe生效。
- 构建、验证与待验收范围见 [TELEMETRY_VERIFICATION](docs/TELEMETRY_VERIFICATION.md)。

本文件记录：

1. 已形成的用户可见产品行为变化。
2. 对客户端或开发者具有外部意义的协议或 API 契约变化。
3. 重要项目基线或治理规则变化。

本文件不记录普通内部重构、未完成计划、虚构功能或单纯开发过程流水账。

## Unreleased

### Added

- 增加 `probe-build.cmd` 一键交叉编译入口：上传当前 Probe 源码到指定 SSH 构建机，自动适配现有 GCC 5.2/uClibc 工具链并保留独立构建日志，成功后更新成品，失败保留上一份成功版本。

- Probe 默认采集架构与完整内核版本，保留显式架构和 kernel 模板优先；系统开机时长注册后首报并随心跳更新，新增兼容的 uptime_valid 和设备/Session runtime API。WPF 展示 CPU 架构、内核版本、开机时长与采样时间，按年月日时分秒省略前导空单位（年365日/月30日），明确等待、未知和离线状态；全部属性刷新保留选中行。

> 以下按历次交付记录产品演进；当前 UI 以 ADR-037/036/035/034 为准，旧 React/WinUI/Win32/WebView2 实现及入口已移除。

### Fixed

- Windows 连接成功后左侧状态和输出仍停留“正在连接”：现在记录连接成功、断线与重连，定时快照不重复刷日志。所选设备改为稳定行绑定、仅更新变化字段，修复五秒刷新引起的周期性闪烁。

- Windows 维护关闭后再次开启出现 409：新建维护和释放响应不再被旧快照覆盖，避免关闭历史记录而遗留当前维护；设备已有维护时显示现有通道并保留原租期，已关闭记录不能重复执行关闭。

### Changed

- 按 ADR-037 仅保留新版 C# / WPF 主工作台和 C# / Blazor / Fluent UI 模板生成器，移除 React、WinUI、Win32/WebView2 旧 UI、专属构建/验证脚本与内置终端遗留。当前入口统一为 `ui-windows.cmd` 和 `template-generator.cmd`；保留既有 Server/Probe 功能和模板/用户配置兼容。

- Windows 客户端按 ADR-036 将服务器配置迁入设置并启动自动连接；维护改为 Web/SSH/Telnet 链接和外部 Shell，移除内置终端与 WebView2 依赖。SSH 新配置默认 admin/admin，可编辑并在本机加密保存。客户工具管理及通用任务入口移除，文件传输/配置结果回到各自页；设备按实际模板显示全部属性和采集失败项，长文本支持换行滚动。

### Added

- Windows 主程序按 ADR-035 改为 .NET 10 / WPF 原生 C# 桌面工作区：菜单/工具栏、可调设备分栏、密集表格、属性面板、任务输出、状态栏、浅深/系统主题和键盘快捷键，替代旧卡片式工作台。设备、维护、任务、文件、工具和配置沿用原公开 API；仅内置终端复用本地 xterm.js/WebView2 与 ConPTY。`ui-windows.cmd` 构建自包含原生包，兼容已有 profile.json。

- 展示属性增加“条件结果”：按单个来源值或多个虚拟属性的算术/逻辑组合选择显示文本，支持文本相等/不等比较、规则排序与默认结果；预览标明命中规则。新增可编辑条件示例，复制保留独立规则，工程版本 2 兼容读取版本 1；发布仍使用原 command 模板，采集/运算失败不转为默认值（ADR-033）。

- 模板生成器属性支持复制：保留虚拟/展示类型、获取方式、公式、超时及模拟值，生成不重复的标识，在原属性后插入并选中副本，便于独立修改。

- 独立 `RouterTemplateGenerator.exe` 模板生成器：虚拟/展示属性、command/nvram/uci 来源、逻辑与算术公式、依赖校验、模拟预览、工程/运行模板导入导出、服务器发布及版本冲突/原请求重试。只将展示属性编译到原运行模板，公式依赖设备 awk；主工作台设置移除模板配置入口（ADR-032）。

- Probe 增加 nvram / uci 结构化读取、写入、删除及独立 commit 任务；参数直接传给固件程序，配置任务按接受顺序串行，保留任务幂等和断线结果补报。新增公开 config-tasks API、设备配置表单和任务中心结果展示；属性模板可直接选择 nvram 键或 uci 路径，只读采集并兼容原命令模板（ADR-031）。不自动 commit、重启服务或刷新启动快照。

- `ui-windows.cmd` 构建 Windows 单 EXE 客户端；C++ Win32 / WebView2 宿主取代 C# / WinUI，保留 React UI、受限 Bridge、文件保存与 ConPTY（ADR-030）。前端资源内嵌，不附带 .NET、WinUI 或 WebView2 Runtime，客户安装 WebView2。当前已编译，运行验收按用户要求由用户执行。

- Windows Server 增加 `server-windows.cmd` / `.ps1` 一键清理专用构建目录、全量编译并启动入口；持久数据独立保留，监听使用 `0.0.0.0`，对外维护和数据地址预设 `pcv6.criskaka.com`。

- 服务端统一管理探针属性模板，设置中支持新建、编辑、删除与版本冲突检查；探针通过 `--template-id` / `--template-name` 选择，启动执行有界自定义指令，设备详情显示采集值与失败原因。模板持久化、注册前 TEMPLATE_GET/REPLY 与注册快照扩展采用 ADR-029，旧 Probe 路径兼容。

- Windows 远程维护默认使用内置 xterm.js / ConPTY Shell，承载本机 SSH/Telnet 命令行客户端；支持输入、尺寸、字号、清屏、复制、会话切换与重新连接，同时保留外部客户端入口。维护到期、Session 替换、切页和退出释放内置进程；不保存密码，不自研 SSH/Telnet 或新增 Tunnel 数据面（ADR-027）。
- 远程文件面板通过有界单次 Exec 读取真实目录，支持选择/拖入文件并创建上传任务、选中文件创建下载任务；文件内容仍通过既有 File/Repository API，完整提交且释放后显式入库。
- 文件管理接入资产内容校验、Windows 保存、上传/下载、完整下载入库、暂存清理和归档；工具仓库接入版本/多产物、服务端兼容判断与显式投放。任务中心接入最终输出、原任务重发、状态筛选和 Operation 关联的工具投放分类。
- `preview.html` 和 `frontend/src/preview/` 保留已确认视觉的独立 Mock 参照，不进入 production bundle。

### Changed

- 独立探针模板生成器迁为 .NET 10 / Blazor / Microsoft Fluent UI Blazor 浏览器应用，采用全屏命令栏、三阶段页签、可搜索属性导航、双列表单与固定状态栏；支持 Ctrl+S、即时字段错误、浅色/深色/系统主题。核心逻辑改为强类型 C#，无需生成器 Node 构建链，保留所有来源、公式/规则、版本 1/2 工程与运行模板。新草稿持久化发布原请求并支持导入旧原生草稿；根启动入口改为本机浏览器，旧生成器 React/Win32 专用代码清除，主工作台技术栈保持（ADR-034）。

- 生成器及主工作台的二次确认改为说明操作对象和影响的弹框，直接确定或取消；删除、替换工程、断开设备及原请求重试/放弃等无需输入确认词或名称。重试仍需用户核对服务器未重启，沿用原请求身份和内容。

- Probe 支持省略 `--device-id` 时通过 `nvram get SN` 获取稳定设备 ID，并接受 `--device_id` 别名；显式参数优先，采集超时或非法结果阻止启动。Linux x86_64 自动化验证通过，厂商固件实测与 sanitizers 限制见 PROJECT_STATUS。

- 开发治理转为在现有 Router-Agent 基线上持续完善产品：用户可仅描述功能、行为、问题或体验，Agent 依据仓库自主完成普通需求、按影响范围验证并同步文档；明确设计变更确认和历史阶段授权的适用范围。新增 DEVELOPMENT 指南，保留 UI Freeze 与既有架构/协议边界；Phase 7 MCP、Phase 8 AI Agent、微信小程序、正式公网 Web 部署、新 Tunnel 数据面及大规模架构扩展暂缓（ADR-028）。本次治理改造不改变产品业务逻辑。

- 用户确认 UI Freeze；正式 React 入口采用已确认的导航、紧凑设备列表、概览、铺满高度的维护区、任务中心、文件管理和工具仓库布局，复用同一 API/WS 层，不再进行主动视觉设计。未提供的遥测与缺失数据如实显示，不回退到 Mock。

- Phase 6 产品 UI 正式迁移为共享 React / TypeScript 工作台，WinUI 3 作为 WebView2 薄 Shell；Windows 与未来 Web 共用页面、设计系统、HTTP/WebSocket 与状态管理，依据 ADR-026。
- 按确认截图实现一级导航、设备列表、维护主卡片和中文浅色/深色界面；保留 Exec、文件、工具版本/产物/兼容/投放与全部既有 API 语义。移除旧 XAML 业务页面和 C# 业务网络层。
- Windows Release 携带 production 静态资源、固定版本 WebView2、app-local VC DLL 和可选 VC++ 离线安装器；目标机不需要 Node/Vite。Native Bridge 限于明确平台能力并验证来源及输入，外部设备网页由系统浏览器打开。

- Phase 6 Windows界面以C# + WinUI 3正式替换WinForms：中文Fluent工作台、设备侧栏、突出远程维护卡片，浅色/深色/系统主题、Mica与自适应布局；服务器地址移入设置，维护/会话编号及释放原因移入详情。
- 命令、文件和工具使用列表与详情，原生弹窗和文件选择器支持桌面流程；错误主摘要中文化，原始API诊断放入详情。HTTP/WebSocket、幂等和系统浏览器/SSH/Telnet启动策略继续复用，后端契约不变。
- Windows发布改为包含.NET与所需WinUI组件的x64目录包，替代旧单文件包；需要保留整个目录及目标机Visual C++运行库。架构选择按ADR-025取代ADR-024的界面/部署部分。

### Fixed

- Phase 4：维护端口释放后默认隔离24小时，独立于关闭历史；新增 `ReusableAfter` 和 `-tunnel-port-reuse-delay`，池满拒绝提前复用。有限隔离不提供超窗或重启后的永久旧地址隔离，部署边界见ADR-022。
- `CloseMaintenance` 本地释放不再等待Probe控制发送；Gateway采用有界控制队列，撤销以data reset终止活动流，Probe在half-close后仍检测reset并回收空闲worker。
- `-tunnel-data-host`支持Server侧域名解析；每次维护固定解析所得IP，Probe保持数值IP握手。
- Probe默认Tunnel并发由64降至8、允许1～64；Server每维护/设备默认均为8。默认idle由5分钟改为24小时，并按整条连接进展计算；绝对维护租期保持默认240分钟。
- 当前Phase 4文档按ADR-022同步，FRP历史归档为仓库内摘要，接管不依赖本地临时Git保存项。

- Phase 1E：Server 严格拒绝非法 Unicode 与字符串形式 load1；Probe 忽略未知扩展字段中的合法大整数，已知 uint64 字段仍拒绝溢出。
- Probe 在 REGISTER_ACK 缩小帧上限后立即复核已缓冲 Header，防止等待超限 Payload；未确认心跳记录设为每连接最多 1024 项，满后沿用既有重连流程。
- Server 文件 worker 结束时释放残留文件块邮箱；入站 message_id 耗尽时结束连接，避免回绕为 0。
- Server 成功写出 REGISTER_ACK 后才发布可派发会话，修复注册/重连期间 TASK 抢先发送的问题。
- Server 传输写失败立即关闭并废弃连接，防止部分帧后续写入与 message_id 重用；CreateExec 对发送结果不确定的任务保留记录并返回 task_id 与 ErrDispatchUncertain。
- Probe exec 不再继承控制 socket；fd 的 close-on-exec 设置与 fork 使用统一同步。
- Probe timeout/worker 停止完整执行进程组 TERM、200 ms grace、KILL；后代持有 pipe 时有界排空并设置 truncated，防止 worker 因 EOF 等待挂住。

### Added

- Phase 6首版Windows远程维护工作台（f8d099d6，界面已由上述WinUI重构取代）：C# / .NET 10 LTS Windows Forms、自包含x64发布；Server设置、设备列表/Session、默认240分钟或正自定义租期Maintenance、三入口和主动关闭、Exec结果及文件/工具/版本/兼容产物/投放。
- Windows UI仅调用现有HTTP/WebSocket；首连/重连HTTP同步、状态失效刷新、响应不确定保留原幂等请求、重复点击防护、切换Server与退出异步释放资源。Web交系统浏览器，SSH/Telnet使用系统客户端或用户指定PuTTY，不缓存密码或Tunnel私有身份。Phase 5已验收，首版按ADR-024进入Windows UI交付，Server/Probe业务契约保持。

- Phase 5统一 `/api/v1` HTTP API：Device/Session、exec/Task及结果/原身份重发、文件资产与上传/下载导入、Tool/版本/Artifact/兼容/投放、Maintenance创建/查询/关闭和三入口。
- 独立HTTP监听默认127.0.0.1:8080；一致JSON/错误/分页、异步202及有界Idempotency-Key账本。HTTP取消不撤销成功创建的长期业务对象，不确定派发保留task_id；默认4096项不淘汰账本，满后拒绝新键。
- WebSocket `/api/v1/events`提供devices/tasks/files/maintenance状态变更通知，首连/重连重新同步；固定容量、多客户端隔离、慢消费者断开和Server关闭回收。files包括Repository目录变更。默认可信部署边界、容量和后续认证/TLS/RBAC设计点见ADR-023/API.md。
- HTTP隐藏Server本地路径与Tunnel数据面私有细节。复用Phase 4 Service及240分钟默认租约、正自定义租约和Session绑定；Probe协议与Tunnel数据面保持。

- Phase 4 新增极简自研TCP Maintenance Service：一次创建Web/SSH/Telnet临时入口，固定访问设备127.0.0.1:80/22/23；默认240分钟并允许自定义租期。独立data TCP与一次性token配对，保持原始字节、并发连接、half-close和有界背压。
- Maintenance严格绑定创建时Device Session，主动关闭、到期、替换或失联撤销入口并终止活动连接；等待Server相关资源释放后才归还端口。Server提供端口池、advertised地址、超时与分层连接限额配置，Probe保持C++11且没有新增重型依赖。
- 新增控制类型0x40～0x42与RMT1独立data握手；以Accepted ADR-021取代未交付FRP/xfrpc路线。不引入HTTP/WebSocket API或通用端口映射。

- Phase 3 新增持久化 File / Tool Repository 与内部资产查询、导入、归档、工具版本/产物发布和兼容查询。tool_id、asset_id、artifact_id 为稳定 UUID，artifact_id 全仓库唯一；版本不可变，重复内容共享 SHA-256 blob，归档保留身份与文件。
- 新增内部工具投放、资产上传和设备下载显式导入，复用既有文件任务；受限资料缺失返回 unknown，不自动投放或执行工具。保留不确定派发的任务身份，下载可分别呈现本地完整提交和最终失败结果。
- Server 支持 `-repository-dir`，默认 `./data/repository`，采用单进程目录锁与本地 JSON 元数据；Linux/Windows 无新增外部服务依赖。仓库数据跨重启保留，设备/任务/Session 不恢复。

- Phase 2 新增 Device Service / Inventory 内部 List/Get/Sessions 查询，保留 REGISTER 基础资料和 capabilities、设备在线状态、当前/最近 Session 及上线/离线/活动时间。
- 设备重连与在线 Session 替换保留稳定 device_id；替换更新 LastOnlineAt，不更新 LastOfflineAt。默认每设备保留最近 64 个已结束 Session，查询包含历史截断计数；仅进程内存储，历史淘汰不影响 Task/File 幂等与结果。

- Phase 1D 实现 upload/download、FILE_BEGIN/ACK/CHUNK/END，原始二进制分块、size/SHA-256 流式校验、同目录临时文件验证后发布和 upload.mode。
- 文件任务使用一个 active 与有界 FIFO，支持同 task_id 幂等、断线失败结果补报；内部 CreateUpload/CreateDownload 与 FileSnapshot 暴露文件能力和下载提交事实。
- 文件与控制发送在帧边界按优先级串行化，并限制内核发送缓冲目标；持续文件传输期间可处理心跳和并发 exec。

- 实现 Phase 1C：Probe 默认 4 workers 并发 exec、乱序结果关联、进程生命周期 task_id 去重；TCP 断开时已接受任务继续执行，新会话补报缓存结果。
- Server 增加内部 ResendTask，使用原 task_id 与规格重发，按 session_id/message_id/task_id 保存每次派发关联。
- 增加实际并发执行、重复任务、参数冲突、容量、断线继续执行和中继丢弃 ACK/RESULT 后补报的自动化覆盖。

- 实现 Phase 1B TASK、TASK_ACK、TASK_RESULT 与 exec 完整闭环，Management Server 可通过内部 Task Service 对在线 Probe 创建任务并等待结果。
- Probe 增加 HEARTBEAT_ACK / TASK 在线消息路由、单 worker 串行任务执行，以及 RECEIVED / QUEUED / RUNNING / SUCCESS / FAILED / TIMEOUT 基础状态流转。
- exec 支持 `/bin/sh -c`、cwd、env、独立 stdout/stderr、1 MiB 单流捕获上限、控制帧大小适配与 timeout 进程组清理。
- 增加 Phase 1B 协议、任务状态、真实进程 exec、超时、心跳不中断、unsupported task 和大输出自动化测试。
- 实现 Phase 1A 的 Go TCP Server 与 C++11 Probe 可运行闭环，包括 REGISTER、REGISTER_ACK、HEARTBEAT、HEARTBEAT_ACK、失联判断和基础退避重连。
- 实现 Protocol v1 20-byte Header、Big Endian 编解码和可处理 TCP 粘包/拆包的流式 framing。
- 增加协议、注册/心跳和真实 Probe 断线重连自动化测试。
- 初始化路由器远程运维平台的项目设计基线。
- 建立 README、AI Agent 开发规则、架构、协议、API、路线图、项目状态、接管手册和架构决策文档。
- 将 v0.2 Word 设计输入中的 TCP 控制协议整理为仓库内可持续维护的 Markdown 基线。
- 建立文档驱动的项目状态与交接治理规则。

### Changed

- Phase 1E 完成 Phase 1A～1D 整体收口验收，Phase 1 标记为完成；验收映射与完整回归、sanitizer、Windows 适用结果记录于 docs/PHASE1_VERIFICATION.md。独立 Verification commit 交付后停止等待验收，Phase 2 尚未开始。
- 按用户确认冻结 Phase 1C 互操作契约（ADR-015）：重复任务返回 queued/running/完成态 ACK，完成态随后返回原 RESULT；内容冲突使用 ERROR/INVALID_PAYLOAD，保留原任务。
- Server 接受缺 ACK 的已派发任务结果，重复 RESULT 幂等，冲突不覆盖终态；Probe 缓存有界且不淘汰已接受身份与结果，容量满后拒绝新任务。
- 更新阶段授权边界：Phase 1C 已独立提交推送，用户现已授权 Phase 1D；确认有界文件 FIFO、流式校验、同 task_id 不重复文件副作用和中断重传必须使用新 task_id/transfer_id（ADR-016）。P1-P5 已确认并写入 Accepted ADR-017：ready 省略 sha256_ok，done=true，failed=false；顺序 offset、失败收敛和最终发布/确认丢失语义为正式契约。Phase 1D 交付后停止，不进入 Phase 1E。

- Server 每条 Probe 连接的 HEARTBEAT_ACK、TASK 和 ERROR 统一串行写入，并共用单连接 Server 发送方向的 message_id 序列。
- Probe REGISTER capabilities 现在声明 `exec`；TASK_ACK 使用 RESPONSE/reply_to，TASK_RESULT 保持异步 task_id 关联且不设置 RESPONSE。
- Phase 0 形成 Git baseline `bc8d747dfc41a375c31698073005857c238ede51`，Phase 1A 完成 Linux x86_64 首轮验证。
- 明确运行事实、规范性设计、当前状态、接管入口、计划和历史设计输入的事实来源层级。
- 完成 Protocol v1 的 message_id、response correlation、flags、UUID、任务拒绝、文件传输、JSON、boot_id、幂等范围和心跳超时细化。
- 将 Phase 1 拆分为 Phase 1A 至 Phase 1E 可验证里程碑，并增加 Probe 技术栈前置决策门槛。
- 明确 Phase 0 需要用户确认和 Git baseline commit 后才能关闭。
- 确定 Probe Phase 1 技术栈为 C++11 + CMake，首轮 Linux x86_64 验证，后续使用 toolchain files 交叉编译。
- 冻结 Phase 1A REGISTER / REGISTER_ACK / HEARTBEAT / HEARTBEAT_ACK 字段契约、注册失败响应和 10-300 秒心跳范围。
