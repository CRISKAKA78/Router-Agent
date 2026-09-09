# 客户工作区改造与验证（ADR-047）

2026-09-09，依据用户合并十项需求的明确实施授权。实现仅涉及当前WPF、Blazor、共享展示模型和Server展示字段；无新阶段、Probe消息或工具管理员发布通道。保留进入任务前已有未提交改动、用户进程和运行数据，没有Git提交/推送。

## 完成行为

| 需求 | 可观察结果 |
| --- | --- |
| 磁盘展示配置 | 磁盘属性沿用分类/字段开关，磁盘分类可独立关闭存储空间页面；跟随已应用模板，只影响显示 |
| 导航与发现布局 | 设备列表/设备发现更名，发现操作等宽两列，资料列按内容适配 |
| 更新模板 | 可搜索并选择模板，显示当前/目标版本，明确应用；支持取消、切换模板、更新版本和同版重应用 |
| 接口页 | 外壳端口/系统端口并列，删除指定空态说明、接口详情和接口属性区，采样设置放在设备页最后；旧接口属性归其他信息 |
| 流量图 | 删除图外说明段落，保留曲线、图例、坐标和悬停数据 |
| 设置 | 移除工作区设置页，顶部设置弹窗保留服务器、主题和外部客户端配置 |
| 模板属性分类 | 内置选择与展示编辑按类别收纳，修改归组/顺序后编辑行不移动，分类/滚动保持，预览反映最终排序 |
| 文件 | 移除文件资产与发布菜单；进入/tmp，支持目录浏览、本地上传和设备下载、编辑路径；自动串联File API及校验保存 |
| 仓库工具 | 工作区最后一项，搜索名称/说明、查看版本，右键确认版本/兼容产物/目录后投放，默认/tmp，不自动执行 |

文件目录通过既有有界exec读取，内容使用内置FILE协议。服务器仍保存资产，Windows不展示资产管理；不确定响应保留原字节/幂等键，下载完整提交/释放和设备RESULT分别展示。切换设备或退出取消本地等待，不声称撤销已经提交的设备操作。

## 实现入口

- WPF：DeviceViews、ManagedViews、DeviceActions、SamplingView、SettingsView、RepositoryViews、DeviceDirectoryView、FileTransfers、ToolWorkspace、ActionWindow。
- Client：FileExchange、RemoteDirectory、WorkspaceConnection；shared/DevicePresentation提供字段默认分组与稳定编辑分类。
- 生成器：PresentationEditor、AttributeEditor、DisplayLayout、EditorLayout、PresentationSettings。
- Server：internal/probetemplate/presentation.go保存可选storage_visible，沿用公开API严格布尔校验、持久目录及已应用模板快照。
- 规范：[ADR-047](DECISIONS.md#adr-047-客户端文件工作区与分类模板编辑)、[API](API.md)、[架构](ARCHITECTURE.md)。协议未变化。

## 本轮验证

Windows使用.NET SDK 10.0.400、现有Go、Python/PyMuPDF、Edge/Playwright；Linux使用项目外WSL RouterAgentTest。日志与布局均在`build/customer-workspace/`。

| 命令/范围 | 结果与证据 |
| --- | --- |
| `go test ./internal/probetemplate ./internal/api ./internal/enrollment ./internal/management` | 通过，go-tests.log；true/false往返、null拒绝、持久重载及快照隔离 |
| 同四包`go vet`、`go build -o build/customer-workspace/router-server.exe ./cmd/server` | 通过 |
| Linux同四包`go test -race`、`go vet`、Server构建 | 通过，linux-race.log、linux-build.log；源码复制到`/work-runs/customer-workspace-20260909`，未挂载Windows工作区 |
| `build/dotnet10/dotnet.exe test ProbeTemplateGenerator.sln -c Release`，`RMP_GENERATOR_WSL=RouterAgentTest` | 172通过、0跳过，generator-tests.log |
| `template-generator.ps1 -BuildOnly` | 成功，0警告/0错误，generator-build.log |
| `node tests/template-generator-browser.mjs` | 13组全部通过，无浏览器错误，browser.log及browser/；使用5197隔离生成器及脚本专用Server |
| `build/dotnet10/dotnet.exe run --project windows/RouterWorkbench.Desktop.Tests -c Release -- build/customer-workspace/router-server.exe build/customer-workspace/wpf` | 280项通过，wpf.log；真实Go Server和测试协议对端 |
| `D:/Python314/python.exe windows/RouterWorkbench.Desktop.Tests/render.py build/customer-workspace/wpf` | 31份实际WPF XPS渲染/文字/图形检查通过，render.log；人工查看关键页和弹窗 |
| `windows/build-desktop.ps1 -BuildOnly -OutputName windows-desktop-customer` | 自包含win-x64单文件发布成功，publish.log |
| `git diff --check`与本轮文档链接检查 | 通过；Git行末转换提示不属于空白错误 |

桌面用例覆盖：目录特殊字符、上传下载内容一致、继续原任务、不确定上传响应重试保持相同键/字节且不重复准备资产、下载取消后继续、文件已提交而设备RESULT失败的如实状态、模板取消/搜索/更换/同版/新版确认、工具取消无任务、版本/兼容产物及自定义路径、设置弹窗、存储页默认/隐藏、采样末页和既有断线/重连/退出。

浏览器实际验证改变字段目标分组后DOM行顺序与位置保持，分类内搜索、磁盘页开关、工程导出/导入和真实Server发布。界面检查发现并修复了工具下拉显示对象调试文本、默认目标提示被覆盖、发现资料列过窄、独立磁盘复选框尺寸问题。旧界面选择器与图表说明字数断言按新界面更新，图表仍验证单位/图例/时间轴及矢量内容。早期本地测试失败均在修正后复验通过。

## 产物和使用

新版客户端：`build/windows-desktop-customer/win-x64/RouterWorkbench.exe`。独立输出不覆盖旧包。`ui-windows.cmd`仍可从当前源码构建启动。

Server与生成器源码已更新；原运行实例未替换，正常退出后分别运行`server-windows.cmd`、`template-generator.cmd`加载。模板显示配置须发布并在设备上显式应用，不能仅修改草稿就视为生效。管理员工具上传通道不在此次范围。

## 验证边界

- WPF自动化使用真实API和TestProbe测试对端，不代表厂商固件文件系统、BusyBox命令实际行为或物理DPI已验收。未改Probe源码，本轮不重跑无关C++/ARM构建或部署设备。
- 发布EXE已成功生成，完整桌面启动/操作/退出由同源码测试宿主验证。未直接启动发布EXE：当前入口会自动写入用户默认连接配置并连接原Server，本轮保持用户配置和实例不变；发布EXE在干净目标机启停仍待验收。
- 曾尝试后台启动浏览器测试服务，被自动审批拒绝（仅返回blocked by policy）；改用工具跟踪的前台隔离进程完成浏览器测试，无待审批动作。
- 本轮不改变此前ARM/公网出口/sanitizer等遗留状态；没有将这些旧问题算作此次通过。
