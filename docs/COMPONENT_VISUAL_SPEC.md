# 方案3字体与共享控件规范

2026-09-10。用户要求保持方案3结构，统一整个WPF主程序的文字渲染与组件风格，并明确授权执行规划。此规范对应ADR-054，细化ADR-053，默认外观以[原图](design/property-inspector-target.png)为准；状态和原图未绘制的组件沿用同一规则，不改变业务行为。

## 基础规格

ADR-055补充：维护、文件和工具主工具栏采用16 DIP左内距、8 DIP上下内距，与页面正文对齐；普通控件仍至少32 DIP高。目录行至少36 DIP高，文件图标20×20 DIP，名称左对齐、其他列维持居中。维护通道行至少44 DIP高，窄窗口动作换行且地址自动换行。历史与版本使用同一原生Expander/共享ToggleButton，动态可访问名称随标题数量更新。设备详情方案3结构不变。

| 项目 | 默认基线 | 字体偏好与缩放 |
|---|---|---|
| UI字体 | Microsoft YaHei UI，中文和拉丁字符共同校准 | 保留已选字体；缺失字体回到可用默认字体 |
| 辅助/正文/控件/属性/主导航/详情标题 | 12 / 13 / 14 / 15 / 16 / 18 DIP | 按原用户字号偏好同步增长；原始值用Consolas及中文回退 |
| 文字排版 | Display、Grayscale、布局取整、像素对齐；完整值行距1.6倍 | 不修改Windows ClearType、缩放或用户系统设置 |
| 普通控件 | 最小32 DIP高；页面选择与搜索36 DIP | 内容增长可增高，避免固定高度裁字 |
| 边框与圆角 | 1 DIP边框，控件4 DIP圆角 | 主题统一资源；禁止页面单独补偿箭头位置 |
| 图标 | Microsoft Fluent System Icons，16/20单位原始资源 | 独立于用户正文字体；等比绘制，固定图标槽位 |
| 下拉箭头 | Caret Down 16 Filled，20 DIP图标框；32 DIP尾部槽位 | 水平/垂直居中，保留原下拉与编辑行为 |
| 输入框 | 左右10 DIP内距，搜索图标独立前置槽位 | 候选/输入/占位文字使用同一字号体系 |
| 按钮 | 普通、主要、工具栏、图标按钮四种明确用途 | hover/pressed/disabled/focus完整，复制/关闭使用标准图标 |
| 选择状态 | 浅蓝底与蓝色重点；hover使用更浅背景 | 不把hover或键盘焦点当作业务选中状态 |

页面选择器使用16 DIP半粗字和16 DIP左内距，保持36 DIP基准高度；普通表单下拉沿用14 DIP和10 DIP内距。模板选择列表的ListBox/Item也使用共享主题。正文根节点显式使用Text资源，避免逻辑父TabItem将选中蓝色传播到整个页面。完整值保留原只读TextBox及文本选择/复制功能，行距通过WPF Block排版属性设置。

## 实现入口

- `Themes/Inputs.xaml`：Button、TextBox、ComboBox/Item、CheckBox及焦点外观。
- `Themes/Menus.xaml`：主菜单/子菜单/右键菜单、ToolTip、PasswordBox。
- `Themes/Controls.xaml`：主题入口、窗口、导航、表格、分隔与滚动控件。
- `Typography`、`Theme`：字号和色彩资源；`ControlChrome`只提供图标/搜索外观选项。
- `WorkbenchIcon`渲染`Themes/Icons.xaml`中的原始库资源；`Assets/Fluent/sources.json`记录版本和来源，LICENSE随发布复制。不引入整套UI框架或网络运行依赖。
- `Desktop.Tests --components`：真实WPF组件样板；交互与视觉验收使用相同产品样式，样板不进入用户业务导航。

## 验收规则

文字箭头、基线偏移、控件混入系统默认皮肤、状态缺失、图标随正文字体变形，均为必须修复项。整窗构图、组件原尺寸/放大细节、浅深主题、中文/英文/数字、默认与大字号、100/125/150/200%渲染分别核验；物理屏幕与离屏DPI证据必须区分。

所有五个工作区和现有业务弹窗复用共享样式。原页面身份/信息顺序、数据绑定、键盘操作、复制与菜单命令保持。功能回归和视觉一致性分别给出结论；最终记录见[Design QA](../design-qa.md)。

资源依据：[Fluent System Icons](https://github.com/microsoft/fluentui-system-icons)、[WPF文字渲染](https://learn.microsoft.com/en-us/dotnet/desktop/wpf/advanced/typography-in-wpf)。图中字体没有原始设计元数据，字体选择按实际样张校准，不宣称还原未知字库。
