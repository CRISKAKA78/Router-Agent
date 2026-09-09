# 原生 C# 客户端（ADR-036 / ADR-035）

## 当前客户工作区（ADR-047）

左侧“设备列表 / 设备发现”，发现页四个按钮等宽对齐；设备资料列按内容适配。工作区“设备 / 维护 / 文件 / 配置 / 仓库工具”，顶部设置打开原配置弹窗。设备右键更新模板可搜索选择其他模板、更新当前版本或同版重应用，取消不提交。

外壳端口与系统端口同级，接口采样设置放在设备页末尾；删除接口详情/接口属性及图上说明段落。原接口属性归其他信息。存储页由已应用模板storage_visible控制，省略仍展示。

文件页进入/tmp，可输入目录、上一级、刷新和双击目录；上传选择本地文件后编辑设备完整目标路径，默认/tmp/文件名，下载选择设备文件和本地保存位置。原文件资产菜单、管理页及工具发布入口移除。客户端自动完成既有File API步骤，传输结果留在文件页；响应不确定时人工核对后继续原请求。

仓库工具支持名称/说明搜索、版本浏览、右键投放；确认框显示当前设备、显式版本与兼容产物、完整目标路径，支持浏览目录或填写路径，默认/tmp，覆盖需勾选。投放后只传输不执行，管理员上传通道仍待单独建设。最新产物与验证见[CUSTOMER_WORKSPACE_VERIFICATION](CUSTOMER_WORKSPACE_VERIFICATION.md)，下文旧导航及界面描述保留对应历史含义。

## 当前设备工作区（ADR-045）

设备资源管理器左侧并列已纳管/待纳管，待纳管保留忽略筛选、批量纳管和恢复。设备右键复制、修改设备名称、更新模板；同版本也可重新应用。所选设备双栈出口分行，新增实际模板版本，仅存在新版时显示更新图标。确认框展示当前和目标版本，等待Probe ACK后显示生效。

设备页顺序为系统信息/资源监控/模板自定义组/接口信息/其他信息/存储空间/连接历史。默认组不需模板创建，所有字段可以在模板内重新归组；资源组展示CPU/内存/磁盘，内存百分比附真实已用/总容量。存储保持原表格，连接历史展示连续在线与随后离线时长，不默认展示Session。接口页提供唯一动态采样入口（周期/接口），已展开详情在滚动页面内查看。

属性列按全部已有名称/值测量，默认完整单行，可横向滚动；首批有效数据继续适配，手动改宽后保持。删除全部展开/折叠、旧设备资料与模板和全采集配置入口；长行手动调窄后的换行/像素滚动仍保留。下方ADR-044折叠说明属旧实现，当前交付见[DEVICE_WORKSPACE_VERIFICATION](DEVICE_WORKSPACE_VERIFICATION.md)。

## 当前版本：属性展示和连续滚动（ADR-044）

全部属性按已应用模板的分类/字段开关过滤，磁盘和网口默认由专用页面展示；名称不附属性标识，悬停和右键仍能查看/复制原标识。属性表只有属性/值两列，排序只按模板，取消列头排序。分组标题横跨全表，背景/边界/留白形成区块，整行点击和键盘均可折叠；双击不重复翻转。组状态按当前窗口的Server/设备/稳定组ID保存。

所有DataGrid统一像素滚动与自动行高；默认单行展示，原数据中的换行仅在显示时压为间隔，复制保留原文。用户手动调窄列后启用该列换行，刷新不重新调整列宽；长组、长行可停在中间。维护链接去除重复垂直边距，行高包含字体下沿。验证与独立成品见 [UI_REFINEMENT_VERIFICATION](UI_REFINEMENT_VERIFICATION.md)；以下旧接口别名和旧兼容路径已撤销。

## 当前扩展：统一网口页（ADR-043）

2026-09-09：原网口速率与物理端口页合并为“网口”，同页展示外壳口链路、协商速率、硬件收发速率、累计流量、统计时长和曲线；逻辑接口与内部口收纳在高级区域。原始uint64整数、来源和口径可展开查看，失败不回退聚合接口流量。本轮独立包为`build/windows-desktop-ports/win-x64/RouterWorkbench.exe`，202项WPF检查通过；用户运行实例尚未替换。下文旧包路径与验证数量保留历史含义，完整状态见[外壳网口监控](PHYSICAL_PORT_MONITORING.md)。

## 当前扩展：待纳管池与分组属性（ADR-041）

主导航新增待纳管池（含忽略/恢复、单台和批量纳管），设备列表只显示managed。纳管表单保留探测型号，允许管理员改名、选型号、输入搜索模板；型号匹配来自Server，人工模板优先。设备资料中可选择并应用已发布版本，采集配置可热改内置周期/接口过滤/属性周期或恢复默认，离线保留期望配置。

全部属性按已确认模板展示元数据分组与数字排序，缺组归其他信息；展开/折叠在当前窗口按Server/设备/稳定组ID记忆，全部展开/折叠与刷新兼容。物理端口独立展示系统接口、交换机/芯片端口、上联、物理链路、管理状态、速率/双工及内部CPU口用途。精确接口别名只影响展示，计数/曲线仍使用真实接口身份。未提供/采集失败如实显示。详见 [MANAGED_PROBES_DESIGN](MANAGED_PROBES_DESIGN.md)。

当前基线补充（ADR-040）：生成器工程4兼容1/2/3，增加精确采集接口与出口周期；WPF增加双栈出口/归属地/运营商、流量统计与曲线，属性横杠及悬停原因、设备列表和摘要按新需求展示。最终验证与限制见 [MONITORING_V2_VERIFICATION](MONITORING_V2_VERIFICATION.md)，下文旧记录保留历史含义。

当前补充：ADR-039已增加独立监控及属性周期、工程3兼容导入，WPF消费监控与来源IP。具体变更和本次验证范围见 [TELEMETRY_DESIGN](TELEMETRY_DESIGN.md) / [TELEMETRY_VERIFICATION](TELEMETRY_VERIFICATION.md)，下文旧验证数量保留历史含义。

> 2026-09-08 清理更新（ADR-037）：仅保留新版 WPF 主 UI 与 Blazor 生成器；旧 UI 源码、宿主与专属脚本已移除。下文各次迁移的旧路径和测试结果保留为历史证据。当前构建/测试入口见 [DEVELOPMENT](DEVELOPMENT.md)，本次清理验证见 [UI_CLEANUP](UI_CLEANUP.md)。

## 连接状态与所选设备刷新修复（2026-09-08）

最新包为 `build/windows-desktop/win-x64/RouterWorkbench.exe`，已包含下方维护重开修复；用户运行的旧 reopen 进程保留。仅更新客户端，Server/Probe/API/刷新周期不变。

原因：Connect 只记录“正在连接”，ApplySnapshot 仅更新右侧 ConnectionText，左侧 StatusText 仍是最后一条日志；QuickProperties 每次快照重新分配 ItemsSource 和全部 PropertyRow。修复前新增回归在“同一快照保持属性数据源与行身份”断言失败。

MainWindow 现在统一观察连接对象和状态转换，首次成功、重连恢复与断开更新活动输出和左侧状态，重复快照不刷日志或覆盖新的业务操作结果。QuickProperties 只绑定一次 ObservableCollection，QuickProperty 在值真正变化时通知 Value；心跳只更新一格，缺失字段显示未提供，断开清空，切换设备更新对应值。

```powershell
go build -o build/windows-desktop/verification-status/router-server.exe ./cmd/server
./build/dotnet10/dotnet.exe run --project windows/RouterWorkbench.Desktop.Tests/RouterWorkbench.Desktop.Tests.csproj -c Release -- build/windows-desktop/verification-status/router-server.exe build/windows-desktop/verification-status
python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop/verification-status
powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-desktop.ps1 -BuildOnly
```

Windows x64/.NET 10、当前隔离 Go Server 与测试对端：98 项检查通过，包括服务端已启动后自动连接、两侧状态与成功日志、真实五秒刷新无集合重置/重复日志、WPF 行控件身份保持且无 Unloaded、心跳按值通知、空值清除、实际 WebSocket 中断与重连恢复、主动断开清空，以及既有维护开关与业务回归。12 份实际 WPF 矢量布局渲染通过；发布 EXE 启动/正常关闭退出码 0，证据为 `build/windows-desktop/verification-status/published-smoke.json`。未在用户物理屏幕上测量闪烁或多屏 DPI，不将控件/布局验证表述为厂商实机验收；未重跑未修改的 Server/Probe 全量阶段测试。

## 维护关闭后重新开启修复（2026-09-08）

当前修复包：`build/windows-desktop-reopen/win-x64/RouterWorkbench.exe`。默认目录的旧程序正在运行，因此单独发布，未停止旧进程；关闭旧窗口后可启动修复版。只需更新客户端，Server、Probe、端口隔离、公开 API 和 ADR 均保持。

根因：创建响应先返回新 maintenance_id，尚未更新的 HTTP 列表/忙碌通知触发 ApplySnapshot，UpdateMaintenance 将新 ID 当作不存在而退回已关闭历史。后续“关闭”只关闭历史 ID（Server 幂等返回成功），当前维护仍未释放，再次创建触发 conflict 409。修复前新增回归在“旧快照仍保持新维护 ID”断言处失败，Server 的释放后允许创建语义符合原规范。

修复：MaintenanceViews 保留最后一次成功创建/释放的公开 API 响应，在同连接/设备 Session 内合并尚未赶上的列表；已释放响应不能被旧 ready 覆盖，切换服务器清空。新建前回查设备当前维护，已有维护则选中现有链接、租期不变；读写之间遇到其他客户端创建的 conflict 时回查现有资源，不自动新建替代写请求。历史关闭按钮按实际释放状态禁用。确认框后的 CloseMaintenanceRecord 是实际关闭动作，回归直接调用该动作，未将程序化调用声称为物理鼠标验收。

```powershell
go build -o build/windows-desktop/verification-maintenance-reopen/router-server.exe ./cmd/server
./build/dotnet10/dotnet.exe run --project windows/RouterWorkbench.Desktop.Tests/RouterWorkbench.Desktop.Tests.csproj -c Release -- build/windows-desktop/verification-maintenance-reopen/router-server.exe build/windows-desktop/verification-maintenance-reopen
powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-desktop.ps1 -BuildOnly -OutputName windows-desktop-reopen
```

Windows x64/.NET 10，当前 Go Server 使用隔离 Repository 与测试对端。84 项检查通过，新增连续三轮开/关/重开、旧创建/关闭快照回放、选择历史后复用活动维护且不续租、关闭真实当前 ID 和即时禁用。自包含 Release 发布与实际启动/正常关闭（退出码 0）通过，证据在 `build/windows-desktop/verification-maintenance-reopen/published-smoke.json`。未对用户真实设备执行维护开关，未重跑未修改的 Go/Probe 全量阶段回归；厂商登录和物理 DPI 原有验收状态保持。

## 当前使用

运行 `ui-windows.cmd` 构建并启动，或直接运行 `build/windows-desktop/win-x64/RouterWorkbench.exe`。主程序为自包含 .NET 10 / WPF x64 EXE，**不需要 WebView2 或 TerminalAssets**；系统 SSH/Telnet 或自行选择的 PuTTY 仍需安装。构建需要 .NET 10 SDK。

连接地址仅在“设置 → 服务器连接”编辑，点击“保存并连接”；每次启动自动恢复上次保存地址，失败后沿用现有重连。首次无配置使用 http://127.0.0.1:8080。`profile.json` 位置和版本 1 兼容，错误地址不会覆盖已存地址，切换/退出取消并等待旧连接及清除旧结果。

维护页“开启维护”后显示 Web/SSH/Telnet 三条公开链接；点击链接或外部客户端按钮即可打开。启动前重新检查维护、Session、到期和端点。默认租期 240 分钟，可改正租期。页面没有内置 Shell 或远程目录。外部程序由用户管理，关闭主 UI 不撤销 Server 维护。

SSH 新配置默认账号/密码均为 admin；已有保存的用户名保留。设置中可分别编辑并保存 SSH 偏好，密码框掩码显示，使用 Windows 当前用户 DPAPI 存在 `profile.json.ssh-password`，不写普通 JSON、URL、日志或命令参数。外部客户端的认证提示仍由用户完成，维护页可显式“复制 SSH 密码”；不自动登录或接受主机密钥，也不会更改路由器实际账号。

## 页面与边界

| 页面 | 当前能力 |
| --- | --- |
| 设备 | 搜索/筛选/排序，全部上报属性、模板引用、采集失败、长文本与连接历史；不按当前模板重新解释旧快照，不补造遥测 |
| 维护 | 三通道链接、复制链接、外部 Web/SSH/Telnet、租期/关闭和维护历史 |
| 文件 | 资产导入/保存/上传/下载/归档，文件传输子页保留 committed/released 与最终 RESULT 分离、显式入库和清理 |
| 配置 | NVRAM/UCI 读写、删除和提交，结果留在本页；修改不自动 commit/重启/重采集 |
| 设置 | 服务器保存与连接、主题、外部客户端和 SSH 账号密码 |

客户工具管理、通用任务/任意 Exec 入口移除。管理员工具配置上传维护通道和任务入口重新规划尚未建设；Server/Probe/API 不变。公开工具/任务能力继续存在，不把隐藏客户入口当作服务端权限控制。

属性通过 `DeviceProperties` 统一构造；`FileTransfers` 只承载文件页传输，`DeviceViews` 展示配置结果；`SshPasswordStore` 仅管本机 SSH 密码。主 UI 删除 TerminalPane/WebView2 资源与包引用，Core 的历史 ConPTY 留给已有旧宿主。HTTP/WS、幂等、Repository 身份和独立维护数据面保持。

## 本轮验证（2026-09-08）

Windows x64、仓库 .NET 10 SDK；当前 Go Server 使用隔离 Repository 与 loopback 测试对端。以下不是厂商实机认证结果。

```powershell
go build -o build/windows-desktop/verification-customer/router-server.exe ./cmd/server
./build/dotnet10/dotnet.exe run --project windows/RouterWorkbench.Desktop.Tests/RouterWorkbench.Desktop.Tests.csproj -c Release -- build/windows-desktop/verification-customer/router-server.exe build/windows-desktop/verification-customer
python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop/verification-customer
powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-desktop.ps1 -BuildOnly
```

- 65 项自动检查通过：启动/重新打开自动连接、保存与无效地址保护、退出和切换清除旧结果、晚到配置响应不跳页/不覆盖文件选择、DPAPI 密码往返/无明文、SSH 参数与 IPv6 链接、维护关闭禁止启动、入口移除、30 项扩展属性和 2 项失败完整展示、切设备不残留、uint64 模板版本、文件与配置结果闭环、旧幂等/传输与保留 API 回归。移除内置终端后的同步关闭重入问题已修复并通过重新打开检查。
- 12 份 WPF 控件树 XPS → PNG 验证通过：五页浅深主题 1480 宽、1000×700 维护与 1920×1080 设备；图在 `build/windows-desktop/verification-customer`。这是实际控件矢量布局，不是物理屏幕捕获。
- Release 构建及自包含发布通过；将最终 EXE 单独复制到新目录（没有 TerminalAssets 或其他旁置文件），实际启动显示主窗口并正常关闭，退出码 0，记录在 `build/windows-desktop/verification-customer/published-smoke.json`。未重跑未修改的 Go/Probe 全量阶段回归和生成器检查。厂商外部 SSH/Telnet 认证、物理鼠标/多屏 DPI、实际剪贴板粘贴及干净目标机仍待验收。

## ADR-035 首次交付历史

以下是首次原生迁移的功能及验证记录；内置终端、任务/工具导航、顶部地址栏等已由上文 ADR-036 取代，旧证据不替代本轮验证。

2026-09-08。用户明确要求主程序采用原生 C#，按成熟工程工具重设计信息架构、效率、密度和视觉，保留已有核心业务。此授权取代旧主程序技术选择和页面冻结；独立模板生成器继续遵循 ADR-034。

## 使用和发布

双击根目录 `ui-windows.cmd`，或执行 `windows/build-desktop.ps1`，构建并启动原生工作区。`-BuildOnly` 只发布，`-Verify` 增加隔离客户端/原生集成检查。

当前包：`build/windows-desktop/win-x64/RouterWorkbench.exe`。主程序是自包含 .NET 10 / WPF x64 EXE（约 63 MiB），保留旁边的 `TerminalAssets`。主 UI 不需要 .NET、Node、Vite、WinUI 或 C++ 运行环境；内置终端单独需要 WebView2 Runtime 与本机 ssh.exe/telnet.exe。外部 PuTTY 仍可在设置中选择。构建机需要 .NET 10 SDK，验证另需 Go。

在顶部服务器输入框填写 `http(s)://主机:端口` 并连接；不附加 `/api/v1`。原 `%LOCALAPPDATA%/RouterWorkbench/profile.json` 路径和版本 1 字段保持兼容，保存地址、主题、用户名和客户端配置，不保存密码、任务快照或 Tunnel 私有字段。终端浏览器缓存独立位于 `RouterWorkbench/DesktopTerminal`。

入口脚本不清空目录、不停止用户进程；目标 EXE 正在运行时拒绝覆盖，可指定 `-OutputName windows-desktop-next`。旧 Win32/WinUI/React 源码、旧发布目录、生成器及运行数据保留，本轮未提交或推送。

## 工作区与功能对照

| 区域 | 原生实现与保留的能力 |
| --- | --- |
| 外壳 | 菜单、服务器工具栏、设备资源管理器、页签工作区、属性区、输出与状态栏；GridSplitter 调整分栏，较矮窗口默认收起输出 |
| 设备 | 搜索/在线筛选、稳定选择、可排序表格、设备属性、启动采集值/失败、会话历史、断开设备；缺少遥测明确为未提供 |
| 维护 | 默认 240 分钟与自定义正租期、创建/关闭、维护历史与公共入口；启动前回查 Maintenance 和设备 Session |
| 内置终端 | 独立 SSH/Telnet 页签、本机 ConPTY、窗口尺寸、字体、复制/粘贴、清屏及外部入口；密码与主机密钥仍由真实客户端提示 |
| 远程目录 | 最多 250 项的单次只读 Exec、NUL 分隔、路径转义、上级目录/双击进入、选择文件上传/下载；内容继续走 File API |
| 任务 | 当前/全部设备、状态/文本筛选、标准输出/错误、规格、原任务重发；收到 202 不宣称成功 |
| 文件 | 选择文件导入、流式摘要校验、资产属性、保存到本机、上传/下载、归档；committed/released/failed 与 RESULT 分开；显式下载入库和暂存清理 |
| 工具 | 创建/归档、不可变多产物版本发布/归档、服务端兼容性检查和所选产物投放；保留高级产物 JSON 编辑，不猜 latest、不自动执行 |
| 配置 | nvram/uci 读取、写入、删除、独立 commit；保留能力检查、1～30 秒超时和原参数；不自动提交/重启/重采集 |
| 设置 | 中文浅色/深色/系统、SSH 用户名、原生客户端选择与恢复系统默认 |

正文 13 DIP、表格行 28 DIP，活动区 25 DIP；中性色、单一蓝色 Accent、细边框、2 DIP 控件圆角，无渐变/装饰阴影/卡片流。表格支持排序、调整列宽、Ctrl+C；刷新保留排序与所选身份。窗口最小 960×640。Ctrl+O 连接，Ctrl+F 搜索设备，Ctrl+N 命令，F5 刷新，Ctrl+J 输出；终端内 Ctrl+Shift+C/V 复制/粘贴。

## 模块和业务边界

- `windows/RouterWorkbench.Desktop`：原生 WPF 视图与交互，按设备/任务/仓库/维护/设置聚合，`Themes/Controls.xaml` 和 `Theme.cs` 为视觉体系；`Ui.cs` 复用原生表格、分栏、工具栏与表单。
- `windows/RouterWorkbench.Client`：公开 DTO、HTTP Client、WebSocket/快照 worker、选择取消与目录解析。没有 Server 内部引用、Probe 协议实现或业务状态机。
- `windows/RouterWorkbench.Core`：复用已有配置、端点策略、外部启动、有界 ConPTY 与独立租期回收；本轮生产 Core 仅更新说明注释。
- `TerminalPane` 只加载固定本地 xterm.js，原依赖版本与 MIT 许可随资源保留；没有设备 API 或任意进程/路径 Bridge。输出在渲染完成后再取下一批，输入沿用 Core 上限。外部页面不能进入终端浏览器。
- `windows/RouterWorkbench.Desktop.Tests`：客户端错误/取消测试、当前 Go Server 和协议测试对端、WPF 控件操作、ConPTY 与 WebView2 初始化检查，生产 EXE 不含测试入口。

首个 `resync_required` 后查询全部 HTTP 分页，通知合并，5 秒恢复刷新；重连重新同步。切换/退出取消并等待旧请求和 socket，详情按连接、选择与取消代次隔离。响应丢失保留原 Mutation 的键与字节，暂停新写入；导入保留拒绝写入的原文件句柄，重试不重新选择内容。已知 API 错误解除待定；显式重试须用户核对服务器未重启，放弃本地记录不会撤销服务器操作。

TCP、HTTP schema、Go Server、C++ Probe、Repository 身份、固定维护 80/22/23、独立 data、端口隔离与未决认证架构均未更改。本次不恢复旧 WinUI 页面，不进入新阶段。

## 本轮验证

环境：Windows x64，仓库 SDK `build/dotnet10/dotnet.exe` 10.0.400、.NET/WPF 10.0.11；WebView2 SDK 1.0.3719.77，已有 Evergreen Runtime；本机 OpenSSH。测试 Server 来自当前工作区，使用独立 Repository、随机 loopback API/控制端口及 34200～34259 维护端口。协议对端明确为 fixture，不是厂商设备。

```powershell
go build -o build/windows-desktop/verification/router-server.exe ./cmd/server
./build/dotnet10/dotnet.exe run --project windows/RouterWorkbench.Desktop.Tests/RouterWorkbench.Desktop.Tests.csproj -c Release --no-restore -- build/windows-desktop/verification/router-server.exe build/windows-desktop/verification
python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop/verification
powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-desktop.ps1 -BuildOnly
```

- 51 项客户端/原生/集成检查通过：原请求重试字节、阻止替代请求、导入原文件不变、保存校验失败保留目标、退出取消等待；设备选择/筛选/排序保留；真实 Go API 和测试对端 Exec、文件往返/稳定入库、工具兼容/投放/归档、维护默认/自定义/关闭、NVRAM 读取 DTO、能力错误、Session replacement；实际 ConPTY 中文 I/O、resize、自然退出尾部输出，以及 WPF WebView2/xterm 加载、终端控制与关闭。
- 16 份实际 WPF 控件树 XPS 布局输出经 Python 3.14/PyMuPDF 1.26.7 转为 PNG：7 页浅色/深色 1480 宽、1000×700 维护和 1920×1080 设备；检查非空内容与像素并人工查看。图片位于 `build/windows-desktop/verification`，是矢量控件输出，不是桌面截图，不含原生标题栏和 WebView2 像素。
- Release 编译与自包含发布通过；直接启动发布 EXE 出现 Router Workbench 主窗口，正常关闭退出码 0，记录为 `build/windows-desktop/verification/published-smoke.json`。测试夹具和测试 Server 在结束时回收，旧用户进程保留。

原生屏幕捕获连续两次返回 `IGraphicsCaptureItemInterop.CreateForMonitor ... 0x80070057`，WPF 栅格输出也为空，未把空图计为通过；改用可核对的 XPS 输出。物理鼠标拖动/多屏 DPI、系统文件选择器真实点击、实际剪贴板往返、厂商 SSH/Telnet 登录、固件 NVRAM/UCI 副作用和无开发环境目标机仍未验收。本轮没有重跑未改动的 Go/Probe 全量 Phase 1～5、sanitizer 或生成器测试；既有误删数据和实机验收遗留继续以 PROJECT_STATUS 为准。
