# 独立探针模板生成器

> 当前只维护工程8/草稿3与三项模板配置页签；接口显示名已删除。旧工程/草稿不导入，属性显示在“分组与排序”选择。最新操作与验证见 [ADR-044交付](UI_REFINEMENT_VERIFICATION.md)；下方早期版本入口已被取代。

> 2026-09-08 清理更新（ADR-037）：仅保留新版 WPF 主 UI 与 Blazor 生成器；旧 UI 源码、宿主与专属脚本已移除。下文各次迁移的旧路径和测试结果保留为历史证据。当前构建/测试入口见 [DEVELOPMENT](DEVELOPMENT.md)，本次清理验证见 [UI_CLEANUP](UI_CLEANUP.md)。

2026-09-08，生成器按 ADR-034 迁为 .NET 10 / ASP.NET Core / Blazor Web App / Microsoft Fluent UI Blazor 的浏览器配置工作区，核心模型、编辑状态、公式、校验、文件格式和发布均使用 C#。保留 ADR-032/033 的全部生成器语义，服务器模板持久化、API、Probe 协议及启动快照保持 ADR-029/031。功能对照与证据见 [迁移记录](TEMPLATE_GENERATOR_MIGRATION.md)。

## 启动与使用

- Windows：双击 [template-generator.cmd](../template-generator.cmd)，构建后打开 `http://127.0.0.1:5188`。使用期间保持启动窗口运行，Ctrl+C 停止。需要 .NET 10 SDK；脚本优先使用本次准备的 `build/dotnet10/dotnet.exe`，也可 `template-generator.ps1 -Dotnet <路径>`。不需要 Node 或 WebView2。
- `template-generator.ps1 -BuildOnly` 只构建；`-NoBrowser` 不自动打开浏览器；`-Port 5190` 可更改本机端口。地址改变后浏览器草稿按 origin 隔离，先保存工程文件。
- 开发：`dotnet run --project src/ProbeTemplateGenerator`；解决方案为 `ProbeTemplateGenerator.sln`，测试用 `dotnet test ProbeTemplateGenerator.sln`。发布用 `dotnet publish src/ProbeTemplateGenerator -c Release -o build/template-generator-blazor/publish`；该目录需要 .NET 10 ASP.NET Core Runtime，可在目录中运行 `dotnet ProbeTemplateGenerator.dll --urls http://127.0.0.1:5188`。
- 本次发布目录为 `build/template-generator-blazor/publish`。原生成器 EXE 和旧数据保留，主工作台无需因本次生成器迁移升级。

操作顺序：

1. 顶部“新建”创建工程，“打开”导入工程/运行模板/旧草稿；更多菜单提供内存联网示例、条件示例与浅色/深色/系统外观。左侧顶部填写模板名称，可搜索属性。
2. 添加虚拟属性和展示属性，分别填写唯一标识、名称与获取方式。虚拟属性只作为中间值；展示属性可直接采集、计算数值，或选择“条件结果 · 输出不同文本”。点击“复制属性”会在原属性后插入并选中副本，保留来源、公式、规则、默认文本、超时与模拟值；名称追加“（副本）”，标识自动使用 `_copy` / `_copy_2` 等避免重复。规则及其他配置可独立修改，引用仍指向原有标识，不复制其依赖；工程达到 128 项时禁用复制。
3. 在直接采集属性中填模拟值，进入“预览与校验”查看展示结果及命中的规则编号/默认结果。预览不执行命令，不读取真实设备。
4. 顶部“保存”或 Ctrl+S 下载可编辑的 `.project.json`，保留虚拟属性、公式、条件规则、默认文本、来源和模拟值。顶部“已保存”指浏览器草稿自动保存完成；文件名旁的小点表示本轮修改尚未导出为独立工程文件。
5. “导出与发布”可导出 `.template.json`，或连接服务器并发布为新模板。更新需要绑定服务器模板的 ID/版本，沿用版本冲突和原字节幂等重试。
6. 用生成器显示的 `--template-id` 启动 Probe。模板更新仅在下一次 Probe 启动生效，普通重连不重采集。

删除属性或服务器模板、替换工程、更新服务器模板、重试或放弃原请求等二次确认，显示具体操作按钮与“取消”，无需输入单词或名称。取消或 Escape 都不执行操作；Tab 焦点保留在确认框内，关闭后恢复焦点。

服务器只保存运行模板，不能从编译后的命令反推出虚拟属性/公式/条件规则。继续编辑应保留原工程；导入服务器模板会将编译命令作为普通 command 来源导入。绑定仅选择更新目标，不立即写入服务器。

## 多种条件结果（ADR-033）

每条规则由“满足条件”和“显示文本”组成，支持添加、上移、下移与弹框确认删除。条件按从上到下的顺序判断，第一条成立后输出该条文本；全部不成立时输出必填的默认文本。一次采集只输出一个最终结果，文本直接填写，无需加引号。

例如虚拟属性标识为 `wan_proto`，来源选 NVRAM、键名填 `wan_proto`，也可用自定义命令 `nvram get wan_proto`。展示属性选择“条件结果”，设置：

| 满足条件 | 显示文本 |
| --- | --- |
| `wan_proto == 0` | 4G |
| `wan_proto == 1` | 5G |
| `wan_proto == 2` | 有线连接 |
| 全部不满足（默认） | 未知连接类型 |

这只是逻辑示例，数字含义由用户按固件调整。多个虚拟属性可组合，例如 `a != 0 && b == 0`、`a == 0 && b != 0`、`a != 0 && b != 0` 分别填写不同文本；条件也可使用 `(a + b) / 2 >= 10`，或引用另一虚拟数值公式。

文本来源使用英文双引号比较，例如 `wan_proto == "dhcp"`、`wan_proto != "pppoe" && link_up != 0`。支持 JSON 字符串转义，比较保留大小写，`"01"` 与 `"1"` 不同；采集值先去首尾空格/Tab/CR/LF，空文本可与 `""` 比较。双引号文本仅用于直接采集值的 `==` / `!=`，数值公式用不带引号的数值比较。文字不作隐式真假或算术转换；数字条件遇到非数字会报错，不能把数字条件和同一输入的非数字文本比较混用来规避类型错误。

规则最多 32 条，每条条件最多 1024 字符/256 符号/32 层嵌套。规则文本和默认文本必须非空，最终仍遵守展示字段长度、UTF-8、已有字段单行及 libc ASCII 限制；编译命令超过 4096 字节时禁止发布。条件结果为最终展示文本，不能再被其他属性引用；需要复用判断输入时引用原始虚拟属性或数值公式。

默认结果只处理“所有条件均为假”。所有规则的传递依赖先采集、数值依赖先校验，中间公式先计算；任何来源失败、无效数值或除零都保留错误，不变成默认文本。条件本身按顺序求值，命中后不计算后续条件中的算术；现有逻辑短路与总采集预算保持。

## 获取与计算语义

| 获取方式 | 设备行为 |
| --- | --- |
| 自定义命令 | 沿用 Probe 的 `/bin/sh -c`，由探针账号执行 |
| NVRAM | `nvram get KEY`，键名沿用 ADR-031 校验 |
| UCI | `uci get package.section.option`，支持已有匿名 section 语法 |
| 公式 | 生成器解析并编译表达式，通过原 command 来源执行；设备需有 POSIX awk，BusyBox awk 可用 |
| 条件结果 | 采集依赖后按条件选择文本，通过原 command + awk 执行；无需升级 Server 或增加 Probe 规则引擎 |

公式支持 `+ - * /`、`&& || !`，以及与/或/非、AND/OR/NOT（也接受小写）和括号。支持数字比较 `== != < <= > >=`，不支持赋值、函数调用、字符串拼接或任意脚本语法。优先级由高到低为：一元非/正负号，乘除，加减，大小比较，相等比较，与，或；括号可改变顺序。

引用使用属性标识，例如 `(memory_total - memory_free) / memory_total * 100`、`link_up && has_address`。输入为十进制数字（可含小数、指数），或 true/false（1/0）；非零为真，逻辑和比较输出 1/0。浮点中间结果限制为有限值且绝对值不超过 `1e308`，输出 12 位有效数字。禁止用字符串真假规则猜测配置值。除零、非数值、缺失来源等失败不给出替代值。

每个展示属性分别读取其全部传递依赖，并按拓扑顺序计算；同一展示属性内部的共享依赖只读一次，不跨展示属性缓存采样。所有引用属性先求值，即使最终逻辑分支短路，也不绕过依赖采集失败。只使用虚拟属性而未被展示属性引用时，该虚拟属性不会执行或上报。虚拟属性的原始值及失败不会单独显示，依赖失败表现为相应展示属性的现有 `command_failed` 等失败原因。

展示属性设置 1～30 秒总超时（默认 5 秒），覆盖其全部依赖读取和计算；虚拟属性不单独计时。整次仍受 Probe 60 秒采集预算约束。标准字段 serial/model/firmware/hostname/kernel/libc 沿用原字段及标签，显式 hostname 参数仍优先。最多 32 个自定义展示属性加 6 个标准字段；工程最多 128 项属性，单条编译命令最多 4096 bytes，运行模板最多 48 KiB。无法满足限制时禁止导出/发布并指出原因。

NVRAM/UCI 直接来源需支持 ADR-031 的 Probe。公式编译为既有 command，不新增 Probe 消息或内置表达式引擎，但设备必须实际提供来源命令与 awk；示例中的 eth0、ip、proc/sys 路径需按固件调整。内存示例计算“非空闲比例”，不冒充可用内存统计。虚拟属性是展示分类，不是秘密存储。

## 本地保存与平台边界

生成器使用作用域内的 C# TemplatePublishingService 访问公开 `/api/v1`，首连/重连 WebSocket 后重新取得 HTTP 快照，并定时刷新模板列表。ASP.NET Core 进程地址与 Go 管理服务器地址独立；切换服务器取消并等待旧请求，不改变浏览器 origin。生成器不操作 Probe TCP 连接或 Go 存储。

工程格式为 `schema_version: 2`，规则使用 `source: "rules"`、`rules: [{condition, value}]` 与 `fallback`。新版读取并升级版本 1 工程/草稿，版本 1 的原有来源和公式保持；旧生成器不能读取版本 2 工程。运行模板格式不变。

草稿在当前浏览器 origin 自动保存，包含工程、绑定目标、外观与待定原请求，最大 1 MiB；读取失败时保留原草稿，用户明确打开或新建后恢复保存。工程/运行模板/草稿文件导入上限 1 MiB。新版可读同 origin 的旧 `router-template-generator.draft.v1` 存储；不同 origin 或旧 WebView2 私有存储不能自动访问，请用“打开”导入旧工程，或 `%LOCALAPPDATA%/RouterTemplateGenerator/generator-draft.json`，原生草稿同时保留发布绑定。导入只读取所选文件，不写入旧目录。

HTTP 写入前先持久化原请求 key/字节，响应不确定时暂停编辑与新发布，刷新页面后仍能恢复；用户连接原服务器并核对未重启后重试，或核对执行结果后放弃。界面显示原服务器、目标 ID 和预期版本；不确定期间仍允许下载工程备份。文件保存通过浏览器下载，默认保存位置由浏览器设置决定。

## 当前迁移验证（ADR-034）

.NET 10.0.400 / Fluent UI 4.14.4 的 Release 构建与发布目录启动通过。145 项 C# 测试通过（0 跳过），含 30 组旧实现完整编译命令对照及 32 次实际 BusyBox 执行；实际 Edge + Go API 的 8 组交互通过。1920/2560/3440 宽屏与 900 窄窗口、浅深主题检查通过。具体命令、产物与验收范围见 [迁移记录](TEMPLATE_GENERATOR_MIGRATION.md)。

## 条件结果历史验证（ADR-033，旧 React/Win32 实现）

- Windows 设置 `$env:RMP_GENERATOR_WSL='RouterAgentTest'` 后执行 `npm.cmd --prefix frontend test`：87 项通过；新增 33 项规则检查，其中 18 项运行实际 BusyBox shell/awk，连同原 14 项共 32 项真实命令检查。覆盖 0/1/2/其他映射、多个输入组合、数值中间公式、文字比较、优先顺序、中文与引号/反斜杠/百分号等原样输出、默认与错误分离、循环/缺失引用、长度与数量、复制独立性和版本 1/2 工程读取。
- `node frontend/tests/templates.mjs`：既有生成器/主设置/设备断开回归与新增 `generator-rules.mjs` 均通过。真实 Edge + 隔离 Go API 验证从来源选择器创建规则、规则添加/排序/删除取消、复制、命中编号/默认结果、非法条件、修改模拟值、草稿重载、工程导入导出及运行模板发布。服务器保存的 command 与导出内容一致，虚拟属性不发布；Server 使用未改动的 `build/router-config/router-server.exe`，协议测试对端不代表厂商实机。
- 1360×1000 浅色、800×600 深色、560×700 浅色规则界面没有页面横向溢出或按钮文字裁切；实际查看深色规则与预览截图，文字和结果完整。截图/结果在 `build/template-generator-check/rules-*.png`、`rules-preview.png`、`rules-result.txt`。
- `npm.cmd --prefix frontend run build:generator` 与 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-native.ps1 -SkipFrontend -TemplateGenerator -OutputName windows-native-rules` 通过，单 EXE 与静态依赖检查通过。未覆盖用户正在运行的旧程序。

本轮不修改原生、主工作台、Server/Probe/API/TCP，不重跑全量后端或原生运行验收；厂商设备、物理 DPI、原生文件选择器、历史 ConPTY 启动失败及原数据恢复遗留保持原状态。

## 上次确认与复制交付验证

- Windows 设置 `$env:RMP_GENERATOR_WSL='RouterAgentTest'` 后执行 `npm.cmd --prefix frontend test`：54 项通过，包含新增的属性复制独立性、唯一标识/名称长度和数量边界检查，以及 14 项实际 WSL BusyBox shell/awk 检查。
- `node frontend/tests/templates.mjs`：真实 Edge + 隔离 Go API 的虚拟/公式属性复制与独立修改、所有测试确认框无输入字段、取消/Escape 不操作、快速连续同类确认、发布/更新/冲突/删除、服务端已提交但响应丢失后的原字节重试、导入导出/草稿及主工作台断开设备的确认/取消通过。结果及截图在 `build/template-generator-check/result.txt` 和 `confirm-copy-removal.png`；同时覆盖 1360/800/560 宽度、浅深色布局及主设置入口移除。浏览器截图中确认说明与按钮显示完整。
- `npm.cmd --prefix frontend run build`、`run build:generator` 通过；主包保留 Vite 的大于 500 kB 提示，无构建错误。最终单 EXE 与静态依赖检查均通过：

  ```powershell
  powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-native.ps1 -SkipFrontend -TemplateGenerator -OutputName windows-native-templatecopy
  powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-native.ps1 -SkipFrontend -OutputName windows-native-confirm
  ```

本次只修改前端确认及属性复制，不修改原生业务、Server/Probe/API/TCP。本轮未重跑原生运行、配置任务专项和后端全量测试；下列首次交付证据保留其原范围，不能当作本轮 EXE 的完整运行验收。

## 首次生成器交付验证（本轮未全部重跑）

- Windows `npm --prefix frontend run build`、`run build:generator` 通过。
- 设置 `RMP_GENERATOR_WSL=RouterAgentTest` 后 `npm --prefix frontend test`：51 项通过，其中 14 项在实际 WSL BusyBox shell/awk 执行生成命令，覆盖算术/逻辑、无效值/除零、来源失败、输出作为数据处理和依赖顺序。
- `node frontend/tests/templates.mjs`（转入 `generator.mjs`）：真实 Edge + Go API 的编辑、预览、草稿恢复、导入导出、旧 command/nvram/uci 来源、创建/更新/冲突/删除、服务端已提交但响应丢失后的原字节重试、主设置移除与设备采集结果展示通过。使用独立临时 Server 数据及协议测试对端，不修改用户数据。
- `node frontend/tests/router-config.mjs`：模板来源验证迁至生成器脚本，现有设备配置任务行为回归通过。
- 浏览器截图检查 1360×1000 浅色、800×600 深色、560×700 浅色，长中文名称换行，未发现页面横向溢出或按钮文字裁切。截图在 `build/template-generator-check`；这些是浏览器截图，不冒充原生物理 DPI 验收。
- 两个正式单 EXE 编译及静态 DLL 依赖检查通过：`powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-native.ps1 -SkipFrontend -TemplateGenerator`，主工作台使用同一命令去掉 `-TemplateGenerator` 并加 `-OutputName windows-native-generator`。`-Verify` 的前 24 项原生策略/配置/草稿检查通过，随后既有 `ConPTY child startup/output` 失败；本次未修改终端实现，没有将完整原生测试记为通过。
- 专项测试宿主使用最终生成器资源及额外 `/DVERIFY_NATIVE`，本次实际编译命令保存在 `build/windows-native-template/compile-verify.cmd`，通过 `cmd.exe /d /c build/windows-native-template/compile-verify.cmd` 执行；生成的 `RouterWorkbench.Verify.exe` 用于下项 CDP 检查，正式 `win-x64` 目录只有生成器 EXE，不包含调试端口。
- 隐藏原生窗口的 WebView2 专项检查见 `frontend/tests/generator-native.mjs` 及 `build/template-generator-check/native-result.txt`；启动、跨服务器草稿恢复、发布及 125% 内容缩放 DOM 检查通过。隐藏窗口截图不可用；文件选择器实操、物理多屏 DPI、厂商设备与完整 Win32 产品验收不在本次已通过范围。

本次未修改 Server/Probe 业务源码、API schema 或 TCP Protocol，不重跑 Phase 1～5 全量/全量 race/sanitizer。历史工作区误删数据的未恢复遗留仍按 PROJECT_STATUS / PROBE_TEMPLATES_VERIFICATION 保留。
