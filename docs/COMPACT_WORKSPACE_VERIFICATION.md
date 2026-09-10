# 四个操作工作区：紧凑操作台验证

2026-09-10，ADR-055。本轮用户选择四页 Audit 的编号1，完成原生WPF维护、文件、配置、仓库工具改造。详情页继续ADR-053方案3，字体/共享控件继续ADR-054。

## 结果与程序

- 自包含Windows x64程序：[RouterWorkbench.exe](../build/windows-desktop-compact/win-x64/RouterWorkbench.exe)。已启动本轮独立实例，实际读取到“已连接”、1台在线设备；旧组件版用户进程和运行数据保持。
- **551项桌面回归、62份实际WPF布局渲染通过**。4份相同示例数据的最终WPF导出另计，只作为视觉证据；不计作真实设备业务验证。
- [最终四页预览](design/compact-workspaces.png)、[方案1联合对照](design/compact-comparison-full.png)、[局部对照](design/compact-comparison-detail.png)、根目录[Design QA](../design-qa.md)。
- 无Git提交/推送，不修改Client业务、Server、Probe、API/WS/TCP或生成器，不需要重启后端。

## 可观察行为

| 页面 | 完成行为 | 关键验证 |
|---|---|---|
| 文件 | 短工具栏、路径、图标与左对齐名称，36 DIP目录行；传输记录默认收起；上传默认当前已读取目录 | 文件选中/清除立即改变下载可用性；无原传输不能继续；保存仅对已提交释放下载启用；原上传下载、重试、原任务/幂等及本地保存回归通过 |
| 维护 | 链接和打开/复制同行；历史默认收起；关闭原因中文显示，失效入口禁用 | 就绪三通道启用、关闭后即时禁用，历史选择与重复开启复用，三轮关闭/重开和旧快照竞态通过；打开前HTTP复核保持 |
| 配置 | 条件表单；结果紧随表单，注明实际设备/系统/操作/键或包/时间 | 读取、写入、删除、NVRAM提交、UCI提交字段检查；编辑新键不改变旧结果归属；真实Go API与TestProbe配置读取结果通过；不自动提交/重启 |
| 工具 | 全宽工具列表，选择后展开版本；投放主按钮等待版本选择；空态提供刷新/清除搜索 | 无匹配说明、选择/取消版本区、明确版本后启用投放，以及既有兼容产物、确认投放且不执行回归通过 |

折叠区使用原生Expander与共享ToggleButton，支持ExpandCollapse自动化模式；标题数量与可访问名称同步。窄窗口维护动作/地址换行，配置表单换行，正文与结果保留滚动区域。浅/深主题和1000 DIP窗口、18字号布局已看图检查，宽屏默认字号另验。

## 命令、环境与证据

Windows本机，仓库.NET 10 SDK；隔离Go Server与TestProbe，不使用厂商设备执行修改。

```powershell
& windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-compact
python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-compact/verification
& build/dotnet10/dotnet.exe run --project windows/RouterWorkbench.Desktop.Tests -c Release -- --compact-export build/compact-qa/final-fixture
python windows/RouterWorkbench.Desktop.Tests/render.py build/compact-qa/final-fixture
git diff --check
```

[最终构建/551项回归日志](../build/windows-desktop-compact/final-build.log)、[62份布局日志](../build/windows-desktop-compact/final-render.log)、[样例导出日志](../build/compact-qa/final-fixture.log)。导出模式仅在测试程序集，通过生产WPF控件和示例DTO生成，不进入发布产品导航。

首轮发现Hyperlink内Run.Text默认TwoWay绑定只读Link导致异常，已显式OneWay并通过后续完整回归。旧详情布局测试的IPv6样例被后台快照覆盖，调整了异步等待后的样例设置时机，保留原断言。视觉首轮发现文件图标偏小、行密度和工具栏左内距不齐，调整20 DIP图标、36 DIP行和16 DIP工具栏后重新导出并联合比较。

## 实际窗口与限制

原生测试样例已检查[文件页](../build/compact-qa/files-native.png)、[关闭维护](../build/compact-qa/maintenance-native.png)、[配置读取](../build/compact-qa/config-native.png)及[键盘切换写入](../build/compact-qa/config-write-native.png)。这些截图来自间距微调前版本，不作为最终像素证据。

最终发布EXE启动及现有后端只读快照同步已通过UI树确认。随后桌面会话返回`GetCursorPos: 拒绝访问`及`CreateForMonitor`捕获失败，最终原生鼠标/整窗截图复验未完成；没有改系统设置或操作旧用户实例绕过。最终视觉结论依据修正后的真实WPF矢量渲染，与原生屏幕验收分开记录。

本轮没有在厂商设备创建维护、写配置、上传文件或投放工具；物理多屏DPI、完整读屏、干净目标机安装及厂商副作用仍未验收。不运行未受影响的Linux/ARM/Blazor全量构建。
