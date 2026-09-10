# 设备详情与系统信息布局（ADR-051）

2026-09-09。本轮接续 `604a2ed` 上已有未提交的 ADR-049/050 改造，按用户给定规范实施；不改 Device/DTO、Client 业务编排、Server、Probe、模板生成器、HTTP/WS/TCP。

## 实现

- 左侧只保留资源管理器、搜索、在线过滤和列表/发现。DeviceList 为生成/回收的可见行计算序号；名称、ID、状态走 WPF 原生列排序与方向标记。轻 Accent 选中背景与左边细线、设备线性图标和小状态点共用主题。
- DeviceSummary 固定复用名称/ID和五个元信息控件，刷新只改显示值。宽窗口横向排列，窄窗口把元信息移至身份下方换行；固件/身份/来源地址保留完整 Tooltip。IPv4 映射地址显示为 IPv4，纯 IPv6 超过 24 字符时截断，来源仍为 source_ip，异步解析取消和失败原因沿用原链路。
- WorkbenchIcon 提供统一 16 单位单色线性矢量；Typography 和公共样式分离一级模块、二级页面、分组标题、正文与辅助文字。内容区明确恢复正文常规字重，避免模块标签的半粗样式继承到整页。
- 系统信息的 PropertySheetColumns 使用两张连续属性表，默认硬件/探针能力/连接出口在左、系统/运行在右，模板信息并入探针分组。显示宽度不足 `820 × 当前字号/13` 时合并。显式模板排序按连续区间保留，不改字段归组、隐藏配置或数据。
- 标签统一起点和列宽，值列占剩余宽度；长能力文本自然换行，Boot ID 等长值默认截断、完整提示和原文复制，保留手动调窄换行、列宽手柄、键盘、稳定行和虚拟化。资源/模板自定义页使用相同图标化分组带；接口、存储和历史复用公共标题，无新增卡片或业务分类。
- 输出默认 112 DIP，时间/类型弱化、正文左对齐；拖拽后保存用户高度，隐藏再显示恢复。保留清空和原有日志处理。模板更新右键菜单与有更新时的顶部按钮沿用原操作。

## 验证入口

环境为 Windows、.NET SDK 10.0.400、仓库 Go 工具链及 Python 3.14/PyMuPDF。测试使用独立配置/端口/目录、真实 WPF 控件和当前 Go Server，TestProbe 为测试协议对端。

| 最终检查 | 结果 |
| --- | --- |
| `build/dotnet10/dotnet.exe run --project windows/RouterWorkbench.Desktop.Tests/RouterWorkbench.Desktop.Tests.csproj -c Release -- build/windows-desktop-details/verification/router-server.exe build/windows-desktop-details/verification` | **408项检查通过**，日志 `build/desktop-details-checks.log`。当前Go Server由本轮脚本Verify入口构建，未使用旧发布Server。 |
| `windows/build-desktop.ps1 -BuildOnly -OutputName windows-desktop-details` | 最终源码win-x64自包含单EXE发布成功，日志 `build/desktop-details-publish.log`。 |
| WPF XPS渲染与内容检查 | **47份布局通过**，含新增6份960/1480/1920浅深主题长字段场景。 |
| 差异与文档链接 | `git diff --check` 和本轮文档相对链接检查通过。 |

- `D:/Python314/python.exe windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-details/verification`：实际 WPF XPS 转 PNG 并检查文字和图形；日志 `build/desktop-details-render.log`。
- `DetailsLayoutChecks` 覆盖三列真实表头点击、升降序指示、搜索/在线过滤、虚拟滚动后序号、选择保持、960/1480/1920 宽度浅深主题、长身份/固件/IPv6/能力列表、双列合并和输出高度恢复。既有摘要测试迁至顶部控件，保留刷新、旧值清理、异步归属、重连及模板更新的行为断言。
- `InspectorChecks` / `AppearanceChecks` / `RefinementChecks` 继续覆盖显式模板顺序、稳定属性对象、手动列宽、完整复制、像素滚动、虚拟化、字体保存及 18/24 字号。

实际复核了默认浅色双列、长字段窄屏深色、18/24字号和输出文本布局。新增检查也验证模块半粗字重不进入属性正文、自动列宽不会因双列合并保留旧宽度；由此修复了逻辑树字体继承及WPF星号列显示宽度缓存问题。表头测试使用列头实际OnClick路径，字符串比较按WPF视图文化规则执行。早期失败均已修正并在最终408项检查中复验。

典型布局：[双列系统信息](../build/windows-desktop-details/verification/inspector-light.png)、[1480长字段](../build/windows-desktop-details/verification/details-1480-light.png)、[960深色长字段](../build/windows-desktop-details/verification/details-960-dark.png)、[24字号](../build/windows-desktop-details/verification/font-workspace-24-dark.png)。长ID场景是界面专用合成数据，不是服务器实际纳管对象；其辅助输出可能包含详情404。部分截图在测试拖拽/手动列宽后生成，默认输出高度仍为112 DIP。

截图为实际控件矢量布局，不包含系统标题栏，不代表物理鼠标、多屏 DPI 或厂商固件验收；发布EXE直接启停未单独执行，窗口启停/重建通过隔离配置的实际MainWindow验证。表头点击和拖动通过 WPF 控件方法/路由事件驱动；不移动用户的物理光标。

成品：[RouterWorkbench.exe](../build/windows-desktop-details/win-x64/RouterWorkbench.exe)。用户运行实例、配置和数据保留，未提交/推送；需正常关闭旧客户端后打开该成品或运行 `ui-windows.cmd` 加载新界面。
