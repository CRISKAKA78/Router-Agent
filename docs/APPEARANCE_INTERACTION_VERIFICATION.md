# 列居中、字体设置与选中状态（ADR-050）

2026-09-09，用户在 ADR-049 工作区基础上要求列字段和值居中、自定义字体和字号，并提供未点击时出现蓝色单元格、虚线框和近似选中行的截图。本轮接续已有未提交的 Workbench 改造，Git 基线仍为 `604a2ed`；仅修改 WPF、本机字体偏好、相关测试和文档。

## 实现和原因

- 所有表格的列标题、字段名称和值统一居中，包括属性、左侧摘要、设备/发现、接口、维护链接、文件、配置及输出。分组标题和表单输入保留阅读布局。
- 顶部“设置 → 外观”新增本机字体选择、可编辑字号（10～24，允许小数）、预览、应用并保存和恢复默认。默认字体为 Segoe UI / Microsoft YaHei UI，正文默认 13 DIP；辅助文字、标题、曲线仍按基准字号保持层级。字体立即应用到主工作区、菜单、表格、提示和工具窗口。
- 字体偏好写入现有 ServerProfile 的 `ui_font_family` / `ui_font_size`，配置 schema 保持 1。旧配置缺省值兼容，未安装字体和越界字号回落默认；无效输入不会改写已保存设置。修改主题、服务器或 SSH 偏好仍保留字体；重建主窗口先恢复字体再使用。
- 行高、菜单和曲线标签随字号变化；输出时间/级别和设备状态列宽同步按字号调整。属性重新测量自动列宽，保留已手动调整的列宽、换行、选中对象和集合视图，恢复原阅读位置。窄窗口/大字号仍可通过已有分隔柄和滚动条查看内容。
- 原全局 DataGridCell 把 `IsKeyboardFocusWithin` 也画成 Selected 底色，WPF 默认焦点装饰又叠加虚线框。现在 Selected 底色只跟随实际选择；单元格焦点使用独立细框，只在 Tab/方向键等键盘导航后显示，鼠标操作退出该提示。没有禁用键盘或清除真实选择。
- 行悬停不再填充背景，Tab 悬停仅改变文字色，只有选中项显示强调线/背景。下拉候选高亮使用 Hover，实际已选值使用 Selected 与侧边强调线，避免混淆。
- 排查发现设备右键切换目标通过 `SelectedItems.Clear()` 操作单选表格，会抛异常；改为设置 `SelectedItem`，原复制/改名/模板操作保持。

主要实现入口为 `Themes/Controls.xaml`、`Typography.cs`、`InputFeedback.cs`、`SettingsView.cs`、`TableBehavior.cs` 及 Core 的 `ServerProfile.cs`。没有改变五个工作区、模板显示规则、HTTP/WS/TCP、Server、Probe、Blazor 或 Client 业务状态机，没有新增依赖。

## 验证方法与边界

环境：Windows、.NET SDK 10.0.400、仓库 Go 工具链、Python 3.14 / PyMuPDF。Desktop.Tests 运行实际 WPF 窗口、当前 Go Server 与 TestProbe 协议测试对端，使用隔离端口、配置和运行目录。

| 最终命令 | 结果 |
| --- | --- |
| `windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-fonts` | 当前 Go Server 构建、win-x64 自包含发布及 **340 项检查通过**；日志 `build/desktop-fonts-checks.log`。 |
| `D:/Python314/python.exe windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-fonts/verification` | **41 份实际 WPF 布局**转 PNG，文字/图形检查通过；日志 `build/desktop-fonts-render.log`。 |
| 差异与文档 | `git diff --check` 和本轮文档相对链接检查通过。 |

实际查看默认浅/深主题选中状态、居中维护列表、18 字号设置与主工作区、24 字号深色主工作区和曲线。输出时间完整，菜单和正文同步，属性行随文字增高。典型产物：

- [浅色选中状态](../build/windows-desktop-fonts/verification/selection-light.png)、[深色选中状态](../build/windows-desktop-fonts/verification/selection-dark.png)。
- [字体设置](../build/windows-desktop-fonts/verification/font-settings-18.png)、[18 字号工作区](../build/windows-desktop-fonts/verification/font-workspace-18.png)、[24 字号深色工作区](../build/windows-desktop-fonts/verification/font-workspace-24-dark.png)、[大字号曲线](../build/windows-desktop-fonts/verification/font-chart-24-dark.png)。
- 自包含成品：[RouterWorkbench.exe](../build/windows-desktop-fonts/win-x64/RouterWorkbench.exe)。正常关闭旧窗口后打开此成品，或通过 `ui-windows.cmd` 构建当前源码。构建目录不纳入 Git。

新增 `AppearanceChecks.cs` 在真实控件树中验证悬停、焦点、选中和字体资源传播，覆盖浅/深主题、Tab 与下拉候选、鼠标/键盘模式、右键目标、旧配置、保存/恢复、无效字号、18/24 字号、输出完整时间以及重建窗口恢复设置。原有维护、传输、模板、采样、幂等、切换与退出用例保留。

测试宿主无法稳定取得物理键盘焦点，因此测试通过 WPF 内部输入传播方法设置 MouseOver/Focus，并发送 WPF 路由事件；这不是物理鼠标移动或系统键盘验收。依据为官方 [MouseDevice 源码](https://raw.githubusercontent.com/dotnet/wpf/main/src/Microsoft.DotNet.Wpf/src/PresentationCore/System/Windows/Input/MouseDevice.cs) 和 [KeyboardDevice 源码](https://raw.githubusercontent.com/dotnet/wpf/main/src/Microsoft.DotNet.Wpf/src/PresentationCore/System/Windows/Input/KeyboardDevice.cs)。内部方法反射仅存在于测试，产品使用公开附加属性和输入事件。

早期验证先修正测试对单选集合的错误清空和直接路由事件注入方式；随后 338 项检查与 41 份布局通过。视觉复核发现菜单默认字号及输出固定列宽遗漏，补齐并增加回归。列宽诊断确认字号 24 时 DataGridColumn 仍保持 80/60 DIP，应用动态资源没有更新列对象；改为应用字体后按实际字体测量这些窄列，继续尊重手动列宽。复验曾因旧复制用例的 Windows `CLIPBRD_E_CANT_OPEN` 中断，日志保留在 `build/desktop-fonts-clipboard-failure.log`；测试仅对这个共享剪贴板临时错误最多重试 4 次，每次等待 250 毫秒，仍需真实复制成功及原内容断言通过。未跳过检查或修改产品复制业务。

WPF XPS/PNG 来自实际控件客户区布局，不含系统标题栏。物理鼠标连续操作、多显示器 DPI、厂商设备尚未验收。发布 EXE 使用默认用户配置，没有独立配置启动参数；本轮以隔离配置运行实际 MainWindow 验证启用/关闭及恢复，未直接启停发布 EXE。保留用户正在运行的窗口和原配置；曾因 `windows-desktop-appearance` 成品正在运行而改用独立 `windows-desktop-fonts` 输出目录。未提交或推送 Git。
