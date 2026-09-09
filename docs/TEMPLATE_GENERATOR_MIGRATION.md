# 探针模板生成器 C# 迁移

## 当前分类编辑与磁盘展示（ADR-047）

“分组与排序”的字段按设备与系统、CPU、内存、磁盘、系统接口、物理端口、出口信息、连接与模板、自定义属性分类查看；添加内置展示属性时同样先选分类。分类/搜索和编辑行保持稳定，修改目标分组或组内顺序不重排编辑表，最终效果由预览反映。“管理分组与排序”定位当前属性所属分类。

磁盘属性继续使用分类和字段可见性；磁盘分类新增独立“显示存储空间页面”，写入presentation.storage_visible，省略为显示。只控制已应用模板的客户端展示，不停采集。默认属性分组为系统信息、资源监控、其他信息，旧builtin_interfaces归组显示到其他信息；外壳/系统端口由客户端专页展示。

工程8/草稿3保持，新增字段随保存、导入导出及发布往返。完整验证见[CUSTOMER_WORKSPACE_VERIFICATION](CUSTOMER_WORKSPACE_VERIFICATION.md)；下方旧接口分组与存储恒显说明由本节取代。

## 默认分组与平行预览（ADR-045）

DisplayLayout提供系统信息、资源监控、接口信息、其他信息四个默认组；模板只创建中间的自定义组。系统/资源固定在前，接口/其他固定在后。内置组可接收任意默认或模板字段，不允许删除或用自定义组重名；删除自定义组的字段回到其他信息。所有归组、顺序和可见性通过同一份presentation编辑，导出保持稳定ID。

预览使用与WPF一致的平行组页签。磁盘资源默认可见，逻辑/物理接口明细默认隐藏，session_id默认隐藏，字段visible优先。内置监控周期仍在模板配置中编辑；设备端只可覆盖接口周期与接口集合。工程8/草稿3及公式/规则格式保持，验证见[DEVICE_WORKSPACE_VERIFICATION](DEVICE_WORKSPACE_VERIFICATION.md)。

## 当前版本：展示选择与旧格式移除（ADR-044）

当前工程8、工作区草稿3（storage key `router-template-generator.workspace.v3`），只读取本版本，不迁移旧草稿或工程。当前运行模板仍可导入/导出；模板发布版本与工程格式版本独立。旧浏览器数据不读取也不自动删除。

模板配置仅“内置监控 / 分组与排序 / 物理端口采集”三个页签。分组页提供内置分类开关和每字段“显示/恢复默认”；磁盘、逻辑网口、物理端口明细默认隐藏，专用页面继续显示。预览使用同一开关，空组不显示。接口显示名及所有映射字段已删除，物理端口自身display_name仍有效。

数据表默认单行；鼠标拖动列头右边缘或聚焦列头后按左右键调整列宽，调窄后允许该列换行。按主题计算行距，整行分组标题可点击折叠。当前验证和入口见 [UI_REFINEMENT_VERIFICATION](UI_REFINEMENT_VERIFICATION.md)；下方各旧版本记录只保留历史事实。

## 当前扩展：独立物理口字节来源（ADR-043）

2026-09-09：物理端口采集增加独立`counters`配置、FNR100预设及原始MIB/TSV样本精确整数预览。工程7兼容1～6，既有属性与发布语义保持；预设需要显式选用，不能仅凭芯片名称套用。167项生成器测试和11组实际浏览器/API流程通过；配套ARM实机升级仍待编译机SSH认证。下文工程版本及验证记录保留历史含义，当前契约与验证见[外壳网口监控](PHYSICAL_PORT_MONITORING.md)。

## 模板配置工作区（ADR-042）

2026-09-09：四个主入口为“模板配置 / 属性与公式 / 预览与校验 / 导出与发布”。模板配置页以页签管理内置监控、分组与排序、接口显示名、物理端口采集；属性导航只在属性编辑时显示。全局设置不再置于单个属性顶部。

展示属性基本信息可直接选择所属分组和组内顺序，和配置页同一份presentation数据同步。统一字段表自动列出内置/自定义展示属性，支持名称/标识搜索，动态设备字段可单独补充。分组固定ID，删除回其他信息；属性改标识保留布局、复制排组末、删除清理。预览显示最终分组、排序和模拟结果，内置数据标为待设备采集。未显式排序的输入框显示“末尾”，移动按钮写入10递增的排序值。

物理口表单按自动/DSA/swconfig/厂商命令显示匹配字段，端口显示名与高级关联信息分开。自动生成内部ID，芯片编号初始留空、用途初始未知；提供字段说明、匹配摘要和别名覆盖提示。工程6兼容1～5及既有草稿，运行模板port省略表示未知；0仍有效。对应Server与Probe需更新，详见 [端口格式与操作](MANAGED_PROBES_DESIGN.md)。本轮验证见 [TEMPLATE_CONFIGURATION_VERIFICATION](TEMPLATE_CONFIGURATION_VERIFICATION.md)。


## 当前扩展：工程5与服务端型号映射（ADR-041）

工程5兼容1～4，新增 presentation（稳定分组ID、组和字段排序、接口别名）及 switch_probe（自动/DSA/swconfig/只读厂商采集、端口/上联/CPU内部口映射）。内置字段可直接加入展示组，无需造采集命令；只有内置监控/展示设置的模板允许空properties。编辑、预览、草稿、运行模板导入导出、发布均保留新元数据。

“导出与发布”可维护型号名称、精确别名和默认模板，模板搜索/选用由WPF纳管表单提供。型号更新使用原版本及现有原请求幂等/不确定响应恢复机制。发布只保存定义，在设备资料中显式应用才热更新；不再生成--template-id启动指令。协议和API见 [MANAGED_PROBES_DESIGN](MANAGED_PROBES_DESIGN.md)。

当前基线补充（ADR-040）：生成器工程4兼容1/2/3，增加精确采集接口与出口周期；WPF增加双栈出口/归属地/运营商、流量统计与曲线，属性横杠及悬停原因、设备列表和摘要按新需求展示。最终验证与限制见 [MONITORING_V2_VERIFICATION](MONITORING_V2_VERIFICATION.md)，下文旧记录保留历史含义。

## 启动脚本文件锁修复

2026-09-08：`template-generator.cmd` 重复点击会先识别监听端口的进程，只有本仓库默认旧 EXE 或当前端口构建的生成器才复用并打开浏览器，不再尝试覆盖运行中的文件。源码更新后，在原启动窗口按 Ctrl+C 停止，再点击入口加载新版；关闭浏览器标签不会停止生成器。其他程序占用端口时返回错误，可用 `template-generator.cmd -Port 5189` 选择其他端口，不终止原进程。

首次启动输出在 `build/template-generator/port-<端口>`，`-BuildOnly` 输出在 `build/template-generator/build-only`，通过 .NET 10 的 artifacts 路径同时隔离 bin/obj；启动时显式指定项目内容根目录，保留 Blazor/Fluent 静态资源。成功复用或正常退出不再暂停 CMD，失败仍保留错误窗口。复用实例不检查源码是否更新，不声称已加载新代码。

本次专项验证：Windows x64、PowerShell 5.1、.NET SDK 10.0.400；`powershell.exe -NoProfile -File template-generator.ps1 -NoBrowser` 首次构建 0 警告/0 错误并监听 5188，重复执行及 `cmd.exe /c template-generator.cmd -NoBrowser` 复用同一 PID 并返回 0；运行期间 `powershell.exe -NoProfile -File template-generator.ps1 -BuildOnly` 同样 0 警告/0 错误。首页及工作区 CSS、隔离 CSS、工作区 JS、Blazor JS 均 HTTP 200。临时 TcpListener 占用其他端口时入口返回 1、原监听仍在，测试结束仅关闭测试自身监听。未重跑与启动脚本无关的业务全量及浏览器编辑流程。

当前补充：ADR-039已增加独立监控及属性周期、工程3兼容导入，WPF消费监控与来源IP。具体变更和本次验证范围见 [TELEMETRY_DESIGN](TELEMETRY_DESIGN.md) / [TELEMETRY_VERIFICATION](TELEMETRY_VERIFICATION.md)，下文旧验证数量保留历史含义。

> 2026-09-08 清理更新（ADR-037）：仅保留新版 WPF 主 UI 与 Blazor 生成器；旧 UI 源码、宿主与专属脚本已移除。下文各次迁移的旧路径和测试结果保留为历史证据。当前构建/测试入口见 [DEVELOPMENT](DEVELOPMENT.md)，本次清理验证见 [UI_CLEANUP](UI_CLEANUP.md)。

2026-09-08，用户明确授权生成器迁移至 .NET 10 / ASP.NET Core / Blazor Web App / Microsoft Fluent UI Blazor，并重构为全屏配置工作区（ADR-034）。此文维护功能对照、工程边界与本次验证，旧证据见 TEMPLATE_GENERATOR 的历史段落。

## 旧工程盘点

已阅读 `frontend/src/template-generator` 全部产品文件，以及 compiler/rules 测试、草稿、公开 API Client 和服务器模板契约。

| 已有行为 | 数据与兼容要求 | 新实现职责 |
| --- | --- | --- |
| 新建、打开、保存、自动保存、内存联网示例、条件示例 | 文件上限 1 MiB，坏草稿不可自动覆盖 | Projects / Persistence |
| 模板名称、属性新增/选择/复制/删除 | 最多 128 属性，复制独立 ID 与规则且生成唯一 key | Models / EditorWorkspace |
| 展示/虚拟属性、标识、名称、来源、模拟值、超时 | 保留名称与标识字节限制、保留字段、标准字段和展示数量限制 | Attributes / TemplateCompiler |
| command、NVRAM、UCI | 仅编译来源，不在生成器执行采集命令 | TemplateCompiler |
| 数值公式、逻辑比较、依赖图 | 优先级、短路、拓扑依赖、循环/缺失引用、数值边界保持 | ExpressionParser / TemplateCompiler |
| 有序条件结果、默认文本、排序、复制、引用插入 | 首条命中；失败不变成默认文本；文本比较保留类型语义 | Attributes / TemplateCompiler |
| 预览、字段校验、命中项、导出 JSON | 模拟值仅在本机预览；只导出展示属性 | Preview / Validation |
| 连接、分页模板列表、导入、绑定、创建、版本更新、删除 | 公开 `/api/v1`，不直接访问 Go 存储；绑定带 origin/ID/version | Publishing |
| HTTP 响应不确定后的原请求重试/放弃 | 原 key、方法、地址和字节保持；暂停编辑及其他写入 | TemplatePublishingService |
| 主题和操作反馈 | 中文浅色/深色/系统、二次确认 | App Shell / Shared |

旧工程模型为 `format: router-agent-template-project`、`schema_version: 2`、`name`、`attributes[]`；属性字段为 `id/key/name/visibility/source/input/sample/timeout`，条件属性增加 `rules[{condition,value}]` 和 `fallback`。版本 1 工程读取后升级为 2，格式不变。运行模板为 `name/properties`，属性含 `name/command/timeout_seconds` 或 `name/source/key/timeout_seconds`。服务器绑定与草稿不进入运行模板。

## 工程与页面结构

```text
ProbeTemplateGenerator.sln
src/ProbeTemplateGenerator/
  Components/             App、路由、Layout、Shared
  Features/
    Projects/             文件格式、工作区入口
    Attributes/           属性导航、编辑与条件规则
    Preview/              模拟结果
    Validation/           字段问题
    Publishing/           连接、模板列表、发布交互
  Models/                 强类型工程与运行模板
  Services/               编辑状态、公式/编译、公开 API 调用
  Persistence/            浏览器草稿
  wwwroot/                CSS、轻量浏览器能力适配
  Program.cs
tests/ProbeTemplateGenerator.Tests/
```

一个 Web 主工程，一个测试工程；无 Domain/Application/Infrastructure 分层，无通用 Repository/Manager 框架。使用 Interactive Server，核心逻辑运行在本机 ASP.NET Core 进程；无需 Node 前端构建。JavaScript 仅提供浏览器存储、文件下载、主题和快捷键。

页面为全 viewport：50px Command Bar、40px 三阶段 Tabs、280px 可折叠属性导航、flex 编辑区、42px Status Bar。普通字段双列，脚本/公式跨列；分区采用标题和细分割线，高级设置折叠。窄窗口转单列，属性导航可收起。主操作保留新建/打开/保存，示例和主题收进更多菜单。

## 迁移影响

- 仅生成器取代旧 React/Win32 实现；主 RouterWorkbench 的 React、Win32、ConPTY 及 Go/Probe 技术栈保持。
- 新入口启动本机 ASP.NET Core 并通过浏览器访问，取代生成器的 WebView2 EXE。不涉及公网部署或认证系统。
- 工程版本、来源、规则与运行模板 JSON 保持。浏览器 origin 改变后不能直接读取旧 origin 的 localStorage；可导入旧工程或原生 `generator-draft.json`，恢复旧内容与目标。
- 新草稿持久化待定请求，避免页面刷新丢失原幂等写入；旧版本待定请求原本只在内存中，无法从历史文件恢复。

## 实施与验证

迁移完成。环境为 Windows x64、.NET SDK 10.0.400、Microsoft Fluent UI Blazor 4.14.4、实际 Edge 与项目外 WSL `RouterAgentTest`。本机没有 SDK，安装在专用 `build/dotnet10`，未替换系统现有 .NET。

| 验证点 | 命令 / 实际结果 |
| --- | --- |
| C# 主工程接入 | `build/dotnet10/dotnet.exe build src/ProbeTemplateGenerator/ProbeTemplateGenerator.csproj --no-restore -c Release`，0 警告/0 错误；修复实际静态资源启动、确认框字符串绑定与导入状态问题后，浏览器可编辑 |
| 原行为兼容、业务与状态 | 设置 `RMP_GENERATOR_WSL=RouterAgentTest`，执行 `build/dotnet10/dotnet.exe test tests/ProbeTemplateGenerator.Tests/ProbeTemplateGenerator.Tests.csproj --no-restore`：145 passed，0 failed，0 skipped |
| 旧编译器独立基准 | `Fixtures/legacy-compatibility.json` 保存从旧 TypeScript 实现实际捕获的 30 组工程、预览及完整命令；新实现逐字段及命令字节对照通过，清理后不依赖旧源码 |
| 真实命令 | 31 个 BusyBox 用例、32 次真实 shell/awk 执行，覆盖公式、短路、来源失败、非数值/除零、条件顺序、文本和转义；不执行用户路由器命令 |
| 发布与草稿 | 上述总数含 16 个发布/草稿用例，真实 Kestrel HTTP/WS 验证分页、首连/重连、快照恢复、取消等待、丢响应后的原键原字节重试、版本冲突、写前保存失败与删除；5 个编辑状态用例验证替换/取消、坏草稿保护与旧待定请求恢复 |
| 实际 UI 与 Go API | 启动本机生成器后 `node tests/template-generator-browser.mjs`：8 组流程通过，浏览器 console/page errors 为 0；使用隔离 Go Server 数据目录，不访问用户业务数据 |
| 布局 | 1920×1080、2560×1440、3440×1440、900×760 浅/深主题截图与实际尺寸检查通过；全屏宽度和底部固定位置正确、无页面横向溢出，折叠属性导航可用 |
| 发布与启动脚本 | `build/dotnet10/dotnet.exe publish src/ProbeTemplateGenerator/ProbeTemplateGenerator.csproj -c Release --no-restore -o build/template-generator-blazor/publish` 与 `template-generator.ps1 -BuildOnly` 通过；从发布目录在 5190 启动，实际 Edge 加载静态资源并计算示例值 75，无浏览器错误 |
| 清理影响 | `npm.cmd --prefix frontend run build`、`npm.cmd --prefix frontend test`：构建及原主前端 13/13 测试通过，保留原大 chunk 提示；`windows/build-native.ps1 -SkipFrontend -OutputName windows-native-cleanup` 主单 EXE 编译/静态依赖通过 |

浏览器 8 组覆盖属性切换/独立复制/确认删除/搜索、字段冲突与公式失败、Ctrl+S 下载与草稿重载、规则编辑/排序/错误定位/默认结果、多尺寸主题、真实 Go API 创建/更新/冲突/重新绑定/删除、v1/v2/native 导入与坏文件保护、手动新建展示/虚拟属性及 NVRAM/UCI/超时。结果和截图保存在 `build/template-generator-blazor/browser-results.json`、`workspace-*.png` 与 `editor-final-1920.png`。

兼容通过后删除 `frontend/src/template-generator` 八个旧文件及三份旧生成器浏览器脚本，移除 Vite/npm/原生 `TEMPLATE_GENERATOR` 构建与 Bridge 分支。`frontend/tests/templates.mjs` 转入新浏览器验证入口。没有清理已有构建产物、旧原生草稿或用户数据。主程序清理后编译产物为 `build/windows-native-cleanup/win-x64/RouterWorkbench.exe`，无需因生成器变化替换用户正在运行的旧主程序。

本次未改 Go Server/Probe/TCP 业务，未重跑其全量集成、原生 ConPTY 运行或厂商路由器实机验收。历史原生终端与数据恢复遗留仍按 PROJECT_STATUS 保留，生成器兼容测试不替代这些验收。

框架接入依据：[Blazor .NET 10 文档](https://learn.microsoft.com/en-us/aspnet/core/blazor/?view=aspnetcore-10.0)、[Fluent UI Blazor](https://github.com/microsoft/fluentui-blazor)、[Fluent 主题组件](https://fluentui-blazor.azurewebsites.net/DesignTheme)。
