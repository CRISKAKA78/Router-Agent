# 紧凑操作台 Design QA（ADR-055）

final result: passed

2026-09-10，本轮编号1覆盖维护、文件、配置、仓库工具；此前详情页方案3继续保留。本结论为**最终WPF渲染的设计对照通过**，551项功能回归另行通过。最终发布程序的原生鼠标/截图复验受桌面会话错误阻止，不能表述为最终物理屏幕全量验收。

## 对照目标、状态和尺度

- 视觉真源：[本轮方案1](docs/design/compact-workspace-target.png)，1589×990像素。文件页是选定图绘制的主页面；其他三页按同轮Audit的紧凑操作规则延展，不宣称有另外三张逐像素目标。
- 最终实现：[实际WPF文件页](docs/design/compact-files-final.png)，窗口1466×913 DIP，客户区1450×874像素；[四页](docs/design/compact-workspaces.png)。来源是最终产品控件的XPS，经PyMuPDF以96 DPI栅格化，没有用网页或静态图片替代交互实现。
- 去除参考原生标题栏36源像素后，将客户区归一到1450×874；未修改原图。原生初轮截图1452×906像素，包括窗框，与最终客户区证据分开；没有浏览器CSS viewport或缩放。
- 浅色、默认微软雅黑UI、文件页、/tmp、无选中文件、历史收起。示例保留FNR10000、原IPv6和10个可见目录；数量如实为10，时间/活动条和工具数量属于样例数据。原图“共38项”、旧配置等待消息及无原传输仍可用的外观不作为行为要求。
- 先看[整页联合对照](docs/design/compact-comparison-full.png)，再看[工具栏/路径/表格联合对照](docs/design/compact-comparison-detail.png)：同一输入左为参考、右为修正后WPF。

## 发现与复验历史

| 级别 | 首轮证据与问题 | 修复与最终证据 |
|---|---|---|
| P2，已修复 | [初轮局部](build/compact-qa/comparison-files-detail.png)：目录图标偏小，行距偏紧，影响快速扫描 | 文件图标20 DIP、行高至少36 DIP；最终局部图重新比较，图文基线与行间节奏清楚 |
| P2，已修复 | [初轮整图](build/compact-qa/comparison-full.png)：主工具栏比目录正文左移8 DIP，路径和按钮相接 | 四页相关工具栏16 DIP左内距，路径操作留6 DIP间隔，上下8 DIP；最终对照确认共同起点和分隔 |
| P2，已修复 | 原生UI树显示维护记录标题为1、Expander名称仍为0 | 可访问名称绑定动态Header；最终WPF构建/回归通过，保留原生ExpandCollapse模式 |

首轮结果为blocked；以上调整之后重新构建并捕获最终实际WPF渲染，再次联合比较。本次复验没有进一步视觉修改，也没有剩余可执行P0/P1/P2。维护Run.Text绑定错误是回归发现的功能问题，修复/回归记录见[专项验证](docs/COMPACT_WORKSPACE_VERIFICATION.md)，不冒充视觉迭代。

## 必查表面

| 检查面 | 判断与保留差异 |
|---|---|
| 字体/排版 | 延续ADR-054微软雅黑UI与用户字号，文件名称/值常规字重，主按钮和激活导航突出；关闭链接变弱。参考无字体元数据，具体字形/抗锯齿不声称逐像素一致。窄屏配置标签和结果归属自然换行 |
| 间距/布局 | 短工具栏、单一路径、大目录表、可展开底部历史保留选定方向；图文与正文起点已校准。32 DIP普通控件沿用组件规范，参考概念图部分控件更高；历史完全收起后不额外占一行空任务提示，给目录更多空间 |
| 颜色/状态 | 浅色白底/蓝灰分隔、蓝色主动作与绿色在线状态沿用既有语义资源；深色复验另见测试渲染。禁用入口有状态文字并移除可点击外观，不仅依赖颜色 |
| 图标/资产 | 使用固定版本Microsoft Fluent原始矢量和MIT许可，新增上传/下载/上一级/文件原始几何；未手绘替代图标、使用位图贴皮或增加装饰插画。字体与图标尺寸独立 |
| 文案/内容 | 完整公共链接和原始值保持；requested显示“手动关闭”，历史上下文明确；配置不自动提交/重启的说明靠近操作；空仓库与无匹配给出可用恢复动作。样例状态标记仅存在测试模式，产品不出现设计说明 |
| 交互/可访问性 | 原生历史ExpandCollapse、选择影响可用性、条件字段、版本选择和结果归属通过回归；配置模式用真实键盘切换检查。表单标签/工具按钮有可访问名称，完整读屏未验收 |
| 窄屏/主题 | 最终62份实际WPF布局包含四页宽/窄、18字号及浅/深主题；维护动作换行、配置结果滚动。例：[窄屏文件](build/windows-desktop-compact/verification/compact-files-1000.png)、[配置](build/windows-desktop-compact/verification/compact-config-1000.png)、[深色维护](build/windows-desktop-compact/verification/dark-maintenance-1000.png) |

P3保留：参考图与Fluent库个别轮廓、细边框及原生文字栅格化存在轻微差异；实际表格保留水平分隔，未照搬概念图每一条竖线。共享字体/控件规范和真实数据优先，没有新增并行皮肤。

## 交付与测试缺口

[最终程序](build/windows-desktop-compact/win-x64/RouterWorkbench.exe)已启动，原生UI树确认现有后端已连接、1台设备在线。551项回归、62份布局、4份最终对照样例导出通过；完整命令和日志见[验证记录](docs/COMPACT_WORKSPACE_VERIFICATION.md)。

最终发布EXE原生鼠标/截图复验期间，桌面会话返回`GetCursorPos`拒绝访问和`CreateForMonitor`失败；最终证据采用真实WPF渲染，未将初轮截图当作最终截图。此前[原生文件页](build/compact-qa/files-native.png)、[维护关闭状态](build/compact-qa/maintenance-native.png)、[配置键盘切换](build/compact-qa/config-write-native.png)保留为初轮交互证据。多屏物理DPI、完整读屏和厂商设备修改副作用不在本轮已验收范围。

- [x] 方案1四页实现及必要状态完整。
- [x] 首轮P2修复，最终整页和局部联合对照完成。
- [x] 功能回归、发布、文档及已有改动保留。
- [ ] 最终原生整窗/鼠标复验：桌面会话恢复后可补充，不把本轮渲染结论扩展成物理屏幕结论。

---

<details>
<summary>此前ADR-054组件轮次报告（历史证据，非本轮验证）</summary>

# 方案3字体与组件 Design QA

final result: passed

2026-09-10，ADR-054。最终发布版本已完成原生整窗、详情及组件局部联合对照；本轮已识别的P2项均修复并复验，未发现剩余可执行P0/P1/P2。519项回归通过。此结论限于下述默认视觉和测试范围，不等于未知字库逐像素一致或物理多屏验收。

## 目标与证据口径

唯一目标为[方案3原图](docs/design/property-inspector-target.png)。用户本轮要求保持已实现的信息和导航结构，纠正整个WPF程序的文字与组件不协调。规格见[COMPONENT_VISUAL_SPEC](docs/COMPONENT_VISUAL_SPEC.md)。

撤回上一轮“组件视觉已通过”的结论：之前将文本箭头、默认控件皮肤和文字差异归为轻微差异，判定过宽。上一轮490项功能回归与布局改造仍是历史事实，不能代替本轮组件检查。旧报告保留在本机`build/component-baseline/previous-design-qa.md`。

- 参考1589×990像素；WPF窗口1480×920 DIP，原生截图1466×913像素，当前桌面96 DPI。参考归一到相同截图尺度，实现截图保留原像素；不存在浏览器viewport/CSS缩放。
- 比较状态：浅色、默认字体、FNR100系统信息、支持操作选中、完整值面板打开、活动收起。真实字段比概念图多，时长变化、运营商/归属地未知“—”保持真实语义。
- [最终原生窗口](docs/design/component-runtime.png)、[整窗联合对照](docs/design/component-comparison-full.png)、[详情联合对照](docs/design/component-comparison-detail.png)：参考在左、实现在右。
- 另检查[下拉框3倍细节](docs/design/component-dropdown-comparison.png)、[搜索](build/component-qa/compare-search.png)、[复制按钮](build/component-qa/compare-copy-button.png)、[完整值文字](build/component-qa/compare-value-text.png)。放大用于观察几何、基线和边框，不用插值后的抗锯齿推断未知字库。

## 发现、修正与复验

| 问题 | 级别与影响 | 修正及证据 |
|---|---|---|
| 文字下拉箭头受字体影响 | P2，用户举出的不协调实例 | 使用Fluent原始矢量，20 DIP图标框/32 DIP槽位；实际展开及局部联合对照复验 |
| 构造函数默认16 DIP覆盖模板尺寸 | P2，模板指定尺寸未生效 | 改为依赖属性元数据；两种字体、13/18/24字号验证实际尺寸与中心点 |
| 中文/拉丁组合、选择器字号和完整值行距不匹配 | P2，字面大小和阅读节奏不一致 | 实际样张确认微软雅黑UI；Display/Grayscale/布局取整，选择器16 DIP半粗，完整值15 DIP/24 DIP行距，设置默认文案同步 |
| 控件、菜单和模板列表混用皮肤 | P2，边框、圆角和状态失配 | 共享输入/按钮/下拉/复选/ListBox、菜单/右键/提示/密码及滚动条；保留原WPF PART和命令。模板弹窗实渲复验 |
| 页面正文继承激活Tab的蓝色 | P2，导航高亮传播到正文 | 页面根节点显式使用Text资源，五个工作区逐一回归；配置页最终渲染已复验 |
| 普通版本值过多加粗、选中标签不突出 | P2，视觉焦点偏移 | 常规值恢复正常字重，状态/动作与选中标签加重；顺序和原始内容不变 |
| 默认新字体使设备ID提前省略 | P2，常用标识可读性下降 | 调整名称/ID列分配比例，保留侧栏和列身份/排序；默认窗口检查完整ID |

[修改前](build/component-qa/before-runtime.png)、[初次共享组件](build/component-qa/main-selected-1.png)及最终截图保留。代码改动限于展示层和默认字体；未新增后台服务、业务数据或UI框架。

最终无后续视觉修改的比较确认：下拉文字和三角图标各自居中，普通控件与页面选择器保持用途对应的尺寸；搜索、复制与关闭使用同一图标系统，完整值七项行距清楚。保留的P3差异包括：生成图与WPF的具体字形/抗锯齿不同，库图标个别轮廓及复制按钮细边框色并非逐像素相同；左栏/底栏保持现有产品的密度和操作。这些不再与文本箭头错位、混合系统皮肤或导航颜色泄漏混为一类。

## 检查面

| 检查面 | 结果与证据 |
|---|---|
| 字体与排版 | 实际样板比较Segoe UI/微软雅黑UI/微软雅黑/等线；默认字体覆盖中文与拉丁，用户字体继续可选。字号、字重、完整值行距统一。原图无字库元数据，不宣称未知字库逐像素还原 |
| 空间与形状 | 32 DIP普通控件、36 DIP页面工具、1 DIP边框、4 DIP圆角；文本与箭头各自占位。保留连续属性与面板结构，宽/窄窗口、大字号没有控件互相覆盖 |
| 颜色与状态 | 浅色蓝灰资源，深色共用语义；hover/focus/selected/disabled分开，正文不继承导航蓝色。[主题与字号样板](build/component-qa/themes-matrix.png) |
| 图标/图像 | 23项Fluent原始SVG及精确几何，统一单色；MIT许可随发布保留。没有位图贴皮、文字假图标或新手绘图标；界面不需要照片/插图 |
| 内容与信息 | 五个工作区、原详情页、模板顺序、能力原始字符串、复制和未知状态语义保持。原生窗框、文件选择器和MessageBox仍由Windows绘制 |
| 交互/可访问性 | 原下拉可编辑/只读、弹出/选择、占位、子菜单、真实选择及独立焦点通过回归；复制/关闭有可访问名称。发布EXE另验键盘/页面操作；不宣称全量读屏通过 |
| 工作区/弹窗 | [其余四个工作区](build/component-qa/workspaces-matrix.png)、[重命名/模板/工具/采样弹窗](build/component-qa/dialogs-matrix.png)、设置、空态/错误/禁用和主题均来自最终WPF实例，集成使用隔离测试数据 |
| 窗口/密度 | 默认/960 DIP最小窗口、18/24字号、浅深主题均已检查；另有16份100/125/150/200%实际WPF XPS栅格图，[密度细节](build/component-qa/density-matrix.png)。离屏密度不是物理显示器或跨屏DPI证明 |

## 验证与交付

最终程序：[RouterWorkbench.exe](build/windows-desktop-components/win-x64/RouterWorkbench.exe)。自包含Windows x64原生WPF，无需重启后端；交付程序保持打开。

```powershell
& windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-components
python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-components/verification
python windows/RouterWorkbench.Desktop.Tests/component-density.py build/windows-desktop-components/verification
git diff --check
```

最终519项桌面检查（含24项组件和5项正文颜色检查）、54份WPF布局、16份离屏密度渲染通过。[构建/回归日志](build/component-verify-delivery.log)、[布局日志](build/component-render-delivery.log)、[密度日志](build/component-density-delivery.log)。集成使用隔离Go Server/TestProbe，发布程序只读查看现有设备。

发布EXE实际验证了[页面下拉展开](build/component-qa/final-dropdown-open.png)、[方向键/Enter切换资源监控](build/component-qa/keyboard-page-selection.png)、[Alt+F菜单与Enter打开设置](build/component-qa/keyboard-menu.png)、[最终设置](build/component-qa/settings-final.png)以及[四个其他工作区](build/component-qa/native-workspaces.png)。文件目录完成只读加载，工具空态正常；没有执行设备维护创建、配置写入或工具投放。错误/禁用、模板/采样提交与精确复制由隔离集成回归覆盖。

测试曾揭示16×16默认尺寸覆盖问题，已修复产品默认值；另一失败来自旧测试仍要求大量系统值加粗，现按方案3/ADR-054验证状态/动作加粗、其他值常规，没有删减业务断言。旧构建不代替最终日志。

物理多屏DPI、不同显示器文字栅格化、完整读屏、厂商设备副作用和干净目标机器安装仍未验收；生成图与原生渲染器之间的细微字形差异客观存在。本轮未改Client/Server/Probe业务，不运行无关Linux/ARM/Blazor全量构建，无Git提交或推送。

</details>
