# 系统信息层级与弹性摘要（ADR-052）

2026-09-09。本轮基于当前未提交的 ADR-049～051 桌面改造继续实现，只修改 WPF 展示、必要回归和文档。用户明确要求保留整体布局、页面和操作，优化顶部摘要、二级导航及系统信息首屏。

## 实现与数据边界

- DeviceSummary：名称/ID保留在左；在线、固件、时长、出口IP、运营商、归属地为六个独立信息项。SummaryBlock/FlexibleSummaryPanel按真实文本测量和当前可用宽度分配，固件/IP优先获得较多宽度；没有固定最大宽度或字符串预先省略。窄窗口移动摘要到下一行，只有实际宽度不足时由TextBlock省略并保留完整Tooltip。
- 二级页面：公共DetailTab/PropertyTabs提供导航带、边框、间距和hover；选中同时使用半粗、浅色背景与Accent描边。页面和原操作不变，主模块及接口内部导航保持原层级。
- SystemOverview：六个紧凑概览块突出状态、开机时长、固件、出口IP、运营商、配置状态；状态点与较大值字号建立焦点。值自然换行。概览高度最多占详情区域48%且不超过240 DIP，超长/大字号仍能滚动，保留下方详情阅读空间。
- 系统详情：PropertySheet继续承担稳定行、完整复制、手动列宽、虚拟化和像素滚动。默认四类语义卡片分别为基础身份、系统与硬件、运行与连接、固件与版本；名称/ID、CPU/内核、在线/时长/来源IP和版本排在各组前面。宽屏两列、窄屏一列，字段值默认完整换行，关键值半粗、标签弱化。
- 模板显式顺序/可见性仍由PropertyGroups决定；同一语义出现在多个不连续位置时保留为连续片段，避免WPF分组把字段重新排序。资源/自定义属性页保留原默认单行和手动换行策略。
- Header与Overview共享现有Device快照和IP归属结果。IP仅取Server source_ip，IPv4映射显示语义保持，异步结果更新及取消仍走原流程；未增加查询、字段、状态机或依赖。未改Client、Server、Probe、Blazor、HTTP/WS/TCP。

## 本轮验证

环境：Windows、.NET SDK 10.0.400、仓库Go工具链、Python 3.14/PyMuPDF。WPF测试使用独立配置/端口/目录、真实WPF控件和本轮构建的Go Server；TestProbe为测试协议对端。

| 最终检查 | 结果 |
| --- | --- |
| `windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-hierarchy` | 自包含win-x64发布、当前Go Server构建、**473项桌面检查通过**；最终日志 `build/desktop-hierarchy-final.log`。 |
| `D:/Python314/python.exe windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-hierarchy/verification` | **47份实际WPF布局渲染通过**；日志 `build/desktop-hierarchy-render.log`。 |
| `git diff --check`及本轮文档链接检查 | 通过；仅检查本轮相关文档/链接，不将历史失效资料作为本轮运行入口。 |

- `SummaryHierarchyChecks` 在640/1280/2600 DIP、10/13/18/24字号测量真实WPF控件，验证无重叠、用满可用宽度、长字段宽屏完整显示和宽度优先级。
- `DetailsLayoutChecks` 在960/1480/1920窗口、浅色/深色验证六个概览块、四类卡片、字段不重不漏、默认完整换行、字重、最小窗口概览不遮住IP、二级Tab状态与原列表/输出操作。长固件、IPv6、ID、能力列表使用合成界面数据。
- `InspectorChecks` 保留模板顺序、稳定行与选择、手动列宽及键盘验证；`RefinementChecks`在资源页继续验证原单行/手动换行、完整复制、像素滚动和虚拟化。既有归属查询、断开重连、文件/配置/模板操作、字体保存与18/24字号回归继续执行。
- 实际WPF XPS经 `render.py` 转PNG；只表示控件真实布局，不包含系统原生标题栏，不等于物理鼠标/DPI或厂商设备验收。

实际复核默认双列、960窄窗口长字段、1920宽屏及24字号浅深主题。修正了选中Tab前景继承到全部概览值、窄窗口概览高度截断第二行IP两处布局问题，专项断言随最终473项复验通过。测试截图在长字段场景先回到页首，列宽键盘测试截图保留手工调整后的真实状态。

布局预览使用测试数据：[默认系统页](../build/windows-desktop-hierarchy/verification/inspector-light.png)、[1480长字段](../build/windows-desktop-hierarchy/verification/details-1480-light.png)、[960深色](../build/windows-desktop-hierarchy/verification/details-960-dark.png)、[1920宽屏](../build/windows-desktop-hierarchy/verification/details-1920-light.png)、[24字号](../build/windows-desktop-hierarchy/verification/font-workspace-24-dark.png)。

## 产物与限制

独立客户端：[RouterWorkbench.exe](../build/windows-desktop-hierarchy/win-x64/RouterWorkbench.exe)。原客户端正常关闭后可启动新包，或通过现有源码启动入口构建。未替换用户进程、运行数据、配置或设备Probe；未Git提交/推送。

本轮没有运行Blazor、Linux/Probe全量回归，因未修改这些模块；公网归属真实服务、物理输入/多屏DPI、厂商设备和发布EXE手工启停仍未验收。长ID截图为纯界面合成对象，其辅助日志可能包含对象不存在的404，不能作为设备连接或业务失败证据。Phase 6最终产品验收状态不由本轮界面验证代替。
