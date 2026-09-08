# 探针模板生成器 C# 迁移

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
