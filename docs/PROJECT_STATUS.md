# 项目状态

## 本次源码交付

- 双架构Probe一键构建、FTV300无stat目录读取/空错误反馈/LF脚本修复及相关23个文件，已获本次用户明确的提交与GitHub main推送授权；此前已合入的14个本地提交一并同步。
- 本轮是源码整理交付，不新增产品实现、不再次执行设备部署或重跑全量构建。检查范围为Git差异、文件/凭据排除、本地链接和远端快进/提交一致性；源码提交与推送结果以实际Git记录为准。
- 下方未提交/未推送描述属于各轮历史，不覆盖本次授权。Probe新双架构设备运行、组网uncertain可信回查/安全收口与二层等未完成项保持原状态。

## 本轮更新合入

- 用户已授权将新增组网修复合入本地main，源码快照f723973；Windows完整Go测试/vet/build、Linux相关六包race/真实Probe文件与仓库及组网定向race/vet/build通过。命令与结果见[第二轮集成](OVERLAY_INTEGRATION.md#第二轮实机修复更新合入)。
- 原uncertain记录、上游空路由回读缺口与安全收口仍待处理；不因本次合并重放操作或追加远端部署。主工作区双架构构建与远程目录修复的未提交改动独立保留，不混入提交。

## 当前：EasyTier ARM—Server 数据面实测通过，配置回查仍有缺口

- worktree同步至main `bab2325`，保留主目录其他任务改动；原实机轮次代码/文档由本次用户另行授权本地合入，未推送。
- 按用户新授权导入ARM/MIPS/mipsel core为包集合 `2.6.4-r1`，备份更新Management并连接本机Web API；未升级Probe。
- ARM `FE7140555489`（用户更新SSH入口20001）与Server的10.144.144.2 ↔ 10.144.144.1双向ICMP、TCP/UDP echo、管理停机30/30 ping、MTU1360复测通过。20004不再测试。
- 上传Released判断及异步运行轮询修复经定向回归、构建和Linux五包race；原uncertain加入记录保留，上游空路由回读缺陷未掩盖，不能宣称UI完整闭环。
- [证据与限制](OVERLAY_LIVE_VERIFICATION.md)：安全收口待确认，未测二层/中继/双路由器/设备重启恢复/长期稳定性；当前网络保留运行。

## 配套工作：FTV300 远程目录读取修复（2026-09-12）

- 实机确认 FTV300/FJB130161591 缺少 stat，而旧客户端列目录依赖该命令；失败任务退出码2且stderr为空。客户端增加无stat的只读元数据回退、未知值显示“未提供”、空错误诊断及Shell脚本LF归一化；API/Probe不变。
- 真机FTV300目录与29字节文件下载/SSH字节比对通过，FNR100原stat路径通过；WPF631项检查、81份布局输出及自包含发布通过，WSL 8组Shell回归通过，见[专项验证](REMOTE_DIRECTORY_FIX_VERIFICATION.md)。
- 新程序：`build/windows-desktop-filefix/win-x64/RouterWorkbench.exe`；正在运行的旧客户端未替换，需关闭旧窗口后使用新包。未重启或部署Server/Probe，未提交/推送；其他工作区改动保留。

## 前次：Probe 双架构构建已完成（2026-09-12）

- ADR-065：`probe-build.cmd` 一次构建 ARMv7/GCC5.2 与 MIPS小端/GCC5.4，产物统一为 `/root/router-agent/router-agent-armv7`（965240字节）和 `router-agent-mipsel`（1231440字节）。旧 gcc54 入口只转发；旧产物/SDK/runs 保留。
- 中文路径 AskPass 启动失败及 CopyTo 管道异常已修复；先 SSH 登录检查、完整 SFTP 上传、两架构成功后发布。本机6项回归、编译机4项发布/失败保护回归、真实双 GCC 构建和 ELF/cmp/默认接口/CRLF 检查通过；记录 `20260912-012702-aa9ec5c9`，见[专项验证](PROBE_BUILD_VERIFICATION.md)。
- 本轮未更改产品业务逻辑、未部署设备或启动 Probe，未重跑 Server/WPF 全量测试，未提交/推送。既有 collection.cpp 编译警告仍保留；设备运行验收与异地组网服务接线/互通仍待完成。

## 前次：异地组网已合入本地main（历史记录）

- 在既有三工作树合并基线上接入 EasyTier 三层首轮代码：Server 编排/API、Probe 仓库安装/独立进程引导、WPF 全局“异地组网”与观测拓扑。用户已授权本地审核与合并；当前提交结果及证据见[专项记录](OVERLAY_INTEGRATION.md)。
- 本次验证：Windows Go/vet、自包含发布、WPF617项/81份布局通过；Linux18项CTest、完整真实Probe集成、五包race及最终AT/组网真实Probe定向race/vet通过。详细命令和剪贴板复跑记录见专项文档。
- 原分支ADR-059统一为ADR-064，原智能邻居/AT/日志/GOST编号与默认服务器、原GCC5.2及独立GCC5.4入口均保持。
- 已修复回查确认后成员配置版本未更新、已确认停止成员仍不能移除，以及 network_agent ACK 误带“不支持任务”消息；未改变不确定操作不重放规则。
- 新桌面包：`build/windows-desktop-overlaymerged/win-x64/RouterWorkbench.exe`。配套厂商Probe构建、上游配置服务/账号与ELF包、两台设备三层互通/恢复/MTU仍待完成；二层尚未实现。未推送、未生产部署、未替换用户程序。

## 前次基线：三工作树联合集成

- 用户已授权 AT（含前置智能邻居）、设备日志、GOST PoC 全部合入本地 main；已在 `codex/integrate-at-logs-gost` 完成冲突整合与联合验证，并合回本地 main。不推送、不部署。
- 保留 main 默认服务器47.119.168.150、8888/9000/9001端口、原GCC5.2构建入口、独立GCC5.4入口与现有维护行为。Probe同版声明邻居检测、AT、日志能力，Gateway并列处理事件；API字段错误与日志自定义消息并存。
- 已通过 Windows Go/vet、WPF605项/77份离屏布局、生成器196项（0跳过）及构建、Probe17项CTest；Linux真实Probe完整Release（291.801秒）、完整Go race（293.660秒）、vet及构建均通过。最新结果与命令见[集成验证](WORKTREE_INTEGRATION.md)。
- 新Windows成品：`build/windows-desktop-integrated/win-x64/RouterWorkbench.exe`。GOST原45项44通过及资源/实机缺口不变，产品API/Probe监管/WPF尚未接入，后续由用户单独测试；本次不运行新GOST实机验证。
- MIPS/厂商设备、物理DPI及Phase6最终验收未追加完成；生成器真实浏览器启动被执行策略拒绝，不能把单元或WPF测试视作该项通过。

## 此前主分支记录（历史事实，非本次验证）
2026-09-12 异地组网首轮改造位于 `codex/overlay-network-research` 独立 worktree（ADR-064）：EasyTier 本机 Web API、持久网络操作、Probe 仓库安装/独立启动、WPF 第六工作区与真实观测拓扑已接入；不是生产/厂商互通验收。Windows 567 项检查、自包含发布及 Linux 16 项 CTest、完整 Go/integration、五包 race/vet 已通过，证据与限制见[专项记录](OVERLAY_NETWORK.md)。当前缺上游运行配置服务/账号、仓库兼容ELF包及本轮厂商构建，未替换生产Server/Probe；二层在三层实机通过后继续。无提交/推送。

2026-09-11 迁移运行状态：已实测 47.119.168.150 上 Server 常驻运行、FNR100 Probe 在线；维护故障定位为 Probe 到 Server 9001/TCP 数据连接不可达，用户随后确认已解决，解决后的端到端连接未由 Agent 复测。本轮按用户授权整理既有 ADR-057/058 迁移改动并合入本地 main、删除迁移分支；实际提交与分支状态以 Git 为准，不推送远端。仅进行差异、凭据排除及合并完整性检查，不重跑产品测试；下文未常驻/未提交为历史记录。

2026-09-11 **ADR-058 Probe构建纠正已完成**：默认root@10.1.1.128，password.txt自动密码认证，经SFTP上传源码并使用原/root/gcc-5.2。真实构建产物 `/root/router-agent/router-agent`，785556字节、ARMv7/EABI5/uClibc；默认接口及管理地址保持ADR-057。记录和验证见[部署§5.3](DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)。厂商运行验收仍未完成，无Git提交/推送。

2026-09-11 **ADR-057已实现**：默认Server/API为47.119.168.150:8888，监听所有本机IP；Linux AMD64静态Server与start.sh已真实SFTP上传/root/agent-server，并通过远端IPv4/IPv6短时启动验证，未常驻启动。Probe默认采集br0,eth0,eth1,usb0；WPF移除SSH凭据、关闭维护隐藏链接、文件默认/tmp/root，精简邻居说明。560项WPF、67份布局、176项生成器、15组浏览器、15项CTest及Windows相关Go/Linux完整真实Probe集成验证通过；命令、产物与实机范围见[专项验证](SERVER_DEFAULTS_VERIFICATION.md)。原已保存连接地址保留，本轮未Git提交/推送。

2026-09-10 用户已明确授权将当前累计源码、测试和文档提交并推送到 GitHub `CRISKAKA78/Router-Agent` 的 `main`。本次仅整理提交：核对远端基线、文件范围和 `git diff --check`，不重跑全量产品测试；构建包、运行数据和本地配置留在本机。下文各轮“未提交/推送”为当时记录，当前提交号与推送结果以 Git 为准；ARM、sanitizer 和实机验收缺口保持。

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
