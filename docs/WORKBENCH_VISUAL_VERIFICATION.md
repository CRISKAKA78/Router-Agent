# Workbench 视觉与连续属性页（ADR-049）

2026-09-09，用户审核 NETworkManager / Rider / Visual Studio 调研方案后授权重构。开始时工作区干净，基线提交 `604a2ed`。本轮只修改 WPF 展示、相关回归及文档，保留当前 .NET 10 技术栈、业务结构和公开接口。

## 实现范围

| 区域 | 当前实现 |
| --- | --- |
| 主窗口 | 左侧资源管理器默认 320 DIP，最小窗口按客户区可用宽度收缩；底部输出默认 140 DIP，沿用小窗口自动收起和 Ctrl+J。主体连续铺满，分隔柄保留原拖动范围，用 1 DIP 线条显示。 |
| 导航 | 五个工作区及设备列表/发现保持。主 Tab 用顶边强调和工作区底色，详情 Tab 用细下划线，接口子 Tab 用浅选中底色；接口采样时间仍是同排弹窗按钮。 |
| 属性 | 系统信息、资源监控、发现资料使用单个连续 Property Sheet；组标题背景带、细线及行间距区分内容，无卡片和新折叠层。传输详情复用同一属性行样式。 |
| 排列规则 | 默认系统字段分为设备标识、系统与硬件、运行与配置、连接与出口；资源字段按 CPU、内存、存储组织。页面任一可见字段存在模板展示覆盖时，保留已应用模板排列，用该页标题组织，不覆盖模板归组、顺序或可见性。 |
| 属性交互 | 继续使用 PropertyRow 稳定通知；同设备快照保留集合视图、选中对象和像素滚动。首次测量完整值；列边缘拖动，或聚焦手柄后左右键调整（Shift 微调），手动调窄换行。复制仍使用原始值和换行。 |
| 表格 | 文本靠左、数值靠右、短状态居中，去除常驻网格和交替色条，保留悬停、选中、表头底线、排序、滚动及复制。摘要与输出分别使用紧凑样式。 |
| 工具窗口 | 设置表单、纳管、改名、模板应用、采样与投放统一标签列、间距和底部操作区；普通工具栏按钮降低边框，维护链接靠左。原确认、取消、错误及异步调用保持。 |
| 字体与颜色 | 正文 13 DIP、辅助文字/分组标题 12 DIP、设备标题 15 DIP；Tab 32/30/28 DIP、普通行 28 DIP、属性行 26 DIP、输出行 24 DIP。沿用浅/深/系统主题，灰阶背景、蓝色交互强调、主题适配在线状态色，控件圆角不超过 2 DIP。 |

`PropertySheet` 只负责桌面阅读小节与集合视图；`PropertyGroups` 仍先执行已应用模板的展示过滤和排序。没有改动共享字段目录、Client 业务状态机、Server、Probe、Blazor、HTTP/WS/TCP 或依赖包。

## 验证

环境为 Windows、.NET SDK 10.0.400、仓库现有 Go 工具链、Python 3.14 / PyMuPDF。桌面测试运行实际 WPF 窗口、当前 Go Server 和 TestProbe 协议测试对端，使用隔离端口、配置和运行目录。

| 命令或范围 | 最终结果 |
| --- | --- |
| `build/dotnet10/dotnet.exe build windows/RouterWorkbench.Desktop.Tests -c Release --nologo` | 初轮编译成功，0 警告 / 0 错误。随后每次发布验证重新编译相关项目。 |
| `windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-inspector` | 当前 Go Server 构建、win-x64 自包含发布及 **314 项检查通过**；日志为 `build/windows-desktop-inspector/verification/desktop-checks.log`。 |
| `D:/Python314/python.exe windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-inspector/verification` | **35 份实际 WPF 布局**转 PNG，文字与图形检查通过；日志为同目录 `render-checks.log`。 |
| 差异与文档 | `git diff --check`、本轮文档相对链接检查通过。 |

回归覆盖默认阅读小节、模板显式顺序不被重排、分组集合与行对象稳定、100 行长分组虚拟化与 13 像素滚动、完整长值首次列宽、原文换行复制、实际手柄拖窄换行、键盘调宽以及 960 × 640 客户区不越界。原有维护、文件传输、配置、模板应用、采样弹窗、幂等重试、重连、切换和退出检查保持。

实际查看浅深色系统属性、资源、设备摘要、接口/曲线、维护、文件、配置、设置、纳管、改名及采样错误状态；窗口覆盖 960 × 640、1000 × 700、1480 × 920、1920 × 1080。典型输出：

- [浅色连续属性页](../build/windows-desktop-inspector/verification/inspector-light.png)、[深色连续属性页](../build/windows-desktop-inspector/verification/inspector-dark.png)、[最小窗口](../build/windows-desktop-inspector/verification/inspector-minimum.png)。
- [接口页](../build/windows-desktop-inspector/verification/managed-switch-ports.png)、[长值换行](../build/windows-desktop-inspector/verification/long-group-wrapped.png)、[设置](../build/windows-desktop-inspector/verification/settings-dialog.png)。
- 自包含成品：[RouterWorkbench.exe](../build/windows-desktop-inspector/win-x64/RouterWorkbench.exe)。构建目录不纳入 Git；`ui-windows.cmd` 仍从当前源码构建原生 WPF。

首轮实际回归发现 WPF 隐式 DataGridCell 样式压过样式字典中的属性单元格设置，导致无表头后缺少列宽手柄；改为属性表局部 CellStyle，保留原单元格基类与换行样式。视觉复核另发现 960 DIP 窗口的左右最小宽度之和会溢出客户区，改为按根客户区宽度限制左栏，并增加右边缘不越界断言。没有删除或降低既有断言。

布局输出改为导出 Window 的 WPF 视觉根，包含实际客户区背景和内容外边距，避免直接导出 Content 时在小弹窗中丢失背景或截掉底部；仍不含系统原生标题栏。

## 参考与边界

视觉研究以 [NETworkManager 固定源码版本](https://github.com/BornToBeRoot/NETworkManager/tree/2780d65469917a296dbc15f206107d72e2d0115c) 的主窗口、资源字典、导航/Tab、列表与网络接口详情为依据。学习连续详情、紧凑工具栏和清晰选择状态；没有引入其 MahApps.Metro、Dragablz 或复制业务实现。实施决定见 [ADR-049](DECISIONS.md#adr-049-原生-workbench-视觉与连续属性页)。

实际控件与矢量布局验证不等于物理多屏 DPI、真实鼠标手感、干净目标机或厂商路由器验收。发布 EXE 尚未直接启停验证：当前入口会读取用户默认配置并自动连接，未为本次样式验证替换用户配置或运行实例。测试宿主中的真实 MainWindow 创建、重连及关闭已覆盖；没有将 TestProbe 当作厂商设备。本轮未重新运行无代码变化的 Go 全量、C++ 或 Blazor 回归，未提交/推送。
