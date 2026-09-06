# Phase 6 WinUI 3 验证记录

日期：2026-09-06。起点 `f8d099d6bb6ac4830199b122755cbf37f6a9e849`，HEAD/main/origin/main 一致，初始工作区干净。用户明确要求替换 WinForms，采用 ADR-025。首版 WinForms 的历史 84 项结果保留于该 Git 提交，不当作本次 WinUI 证据。本次 Server／Probe／Go 依赖／API／Tunnel 生产代码无变化。

## 客户端与窗口验证

`windows/build.ps1 -Verify`执行：Release 产品构建；独立 Core 测试；VerifyUI=true 编译的真实 WinUI 窗口验证；VerifyUI=false 正式目录发布。Core **61 项**、WinUI **98 项**断言通过，构建 0 警告／0 错误。测试失败返回非零；测试代码不进入正式发布。

| 要求 | 实际验证 |
| --- | --- |
| 连接／切换／断开／重连 | 设置 ContentDialog 保存并连接真实 Go Server；按钮断开和重连；切到独立 MockEvents 后旧连接不同步、旧设备／任务／维护／工具清空 |
| 设备列表与详情 | 实际 WinUI 列表和设备概览显示中文名称、在线／离线；WebSocket 后 HTTP 快照刷新；主操作区域不出现会话编号 |
| 会话替换 | 同设备新 TCP 注册，Core 验证原维护撤销、会话变化且仍在线；WinUI 概览刷新新会话，离线通知禁用维护 |
| 创建／关闭维护 | WinUI 控件自动化点击，真实 POST、GET 与通知刷新，三个入口就绪；主动关闭后 Released 来自服务器 |
| 默认 240 分钟／自定义租期 | 默认请求省略 lease_ms，期限差精确 240 分钟；原生时长弹窗 0.005 分钟转 300ms；零或不可表示租期错误，不创建对象；Core 另验证 150ms |
| 到期与历史 | 短租期服务器释放后 UI 回查显示关闭；当前维护选择服务器最新记录，历史在详情；倒计时不伪造终态 |
| Web／SSH／Telnet | WinUI 三按钮经过 GET 后调用启动边界；Core 验证浏览器 Shell URL，以及系统／PuTTY 模式实际 argv 捕获进程的独立主机、端口和用户参数；缺失客户端／非法地址拒绝 |
| 命令与结果 | 控件创建任务，测试对端 ACK／RESULT，UI 显示中文标准输出和错误；同键重试和重发原任务不重复执行 |
| 文件 | 原生桌面“导入文件”／“另存文件”对话框选真实路径，导入与保存逐字节一致；上传／下载弹窗发起真实操作并完成；Core 另验证 complete 稳定身份、cleanup、归档及提交／释放事实 |
| 工具／版本／兼容／投放 | 创建工具与发布版本 ContentDialog，服务器 compatible 结果决定投放按钮；显式产物投放到测试对端完成；Core 另验证多资源身份与归档 |
| WebSocket 恢复 | 真实 socket abort，重连后 HTTP 读取断线期间新设备；Core 查询屏障期间通知后再次查询，没有丢失失效标记，不假设重放 |
| 网络与业务错误 | 不可达、快照失败、维护冲突、设备离线、会话／容量／工具错误分别映射；WinUI InfoBar 显示中文摘要，原始诊断收进详情 |
| 重复点击／并发 | WinUI 拒绝禁用按钮的第二次 Invoke，维护／命令只创建一次；Core 在途屏障拒绝并发，丢失响应后同键同字节重试保留原任务 |
| 生命周期 | Core Dispose 幂等，取消卡住 HTTP、join socket／worker，关闭后不再发布；WinUI 向窗口发送正常 WM_CLOSE，等待资源释放再关闭 |
| 浅色／深色 | 实际切换 Root 主题并验证 ActualTheme、截图；标题栏按钮随主题更新，设置保存外观偏好 |
| 窗口缩放 | 1280×920 与 960×720，侧栏收缩、维护操作可见、详情滚动；截图核对留白与层级 |
| 高 DPI | 桌面 100%；同一实际 XAML 树临时放入公开 DesktopWindowXamlSource 宿主，OverrideScale=2，断言 XamlRoot.RasterizationScale=2、1920×1440 输出和维护控件布局，再恢复原比例 |
| 私有字段与状态边界 | Core DTO 没有 Tunnel token／connection_id；真实 API 响应验证不含私有字段；配置不保存业务快照；UI 不计算兼容或业务状态机 |
| 正式发布目录 | verify-published.ps1 独立启动非测试 EXE，确认原生窗口、从发布目录加载 Microsoft.UI.Xaml.dll、WM_CLOSE exit=0 |

WinUI 测试通过原生控件的 AutomationPeer／IInvokeProvider、真实 ContentDialog 和当前测试进程的文件对话框控制执行。原生文件对话框异步初始化后才可填路径和确认；测试等待可见／启用和布局就绪。使用 Windows App SDK 桌面选择器，避免旧 Windows.Storage.Pickers 的管理员模式限制。

截图覆盖断开、维护浅色／深色／紧凑／200%、任务、文件和工具。RenderTargetBitmap 不包含合成器的 Mica，截屏时临时使用对应纯色主题回退背景，随后恢复 Mica；不把该截图称作 Mica 实际合成结果。200% 测试改变测试宿主缩放，不改变用户显示设置；不声称已完成多物理显示器拖动矩阵。

测试专用 TestProbe 为 Protocol v1 对端，验证 API 请求／返回和界面闭环；它不是实际 Linux Probe，不进入正式应用。真实 C++ Probe／SSH／Telnet／HTTP 通道见下面的全量回归。

发布验证曾发现默认 dotnet publish 未包含模块化 WinUI 的应用 XBF／PRI，导致独立程序 XAML 初始化失败。已在项目发布目标显式加入编译资源，并将非测试发布程序的独立启动／正常退出检查纳入 build.ps1 -Verify；修复后通过。测试构建成功没有被用来代替发布程序检查。未预期的界面异常写入本机应用目录 last-error.log，普通 API 错误仍在界面展示。

## Phase 1～5 与 Tunnel 全量回归

本次从当前源码重新执行，未将首版结果复用为本轮结果。Windows：Go 1.25.5、.NET SDK 10.0.400、.NET 10.0.11、Windows 11 build 26200。Linux：WSL 隔离 Alpine，Go 1.26.3、GCC 15.2.0、CMake 4.2.3，独立网络／mount namespace 与 devpts。

| 验证 | 本轮结果 |
| --- | --- |
| Linux Release C++11／CTest | 4/4，4.72 秒 |
| Linux Phase 1～5 全量 Go + 真实 Probe | 全包通过，集成 169.963 秒 |
| C++ ASan／UBSan／LSan CTest | 4/4，5.95 秒，无报告 |
| ASan 真实 Probe Phase 4／5 | 通过，15.463 秒 |
| C++ TSan CTest | 4/4，6.92 秒，无报告 |
| Go race 全量 + TSan 真实 Probe | 全包通过，集成 177.456 秒，无报告 |
| Windows Go 全部源码包测试 | 通过，集成 0.163 秒；Linux 专用例在 Linux 执行 |
| Windows／Linux Go vet 与 Server 构建 | 通过 |
| Linux Server 真实进程检查 | HTTP 空设备列表成功，SIGTERM exit=0；首次 listener 未就绪由启动轮询恢复 |
| go mod verify／git diff --check | 通过 |

## 复现

Windows 仓库根目录：

```powershell
./windows/build.ps1 -Verify
# 本轮独立 SDK：
./windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify
go test ./cmd/... ./internal/... ./tests/... -count=1
go vet ./cmd/... ./internal/... ./tests/...
go mod verify
git diff --check
```

日志／截图：`build/windows-winui/verification-*`；正式目录：`build/windows-winui/win-x64`。测试控制／HTTP 使用随机 loopback 端口；Core 维护池 32000～32029，WinUI 维护池 32100～32129，须空闲。正式程序仅用户点击后才连接，不包含测试入口。

Linux 隔离环境执行：

```sh
/bin/sh tests/verify-phase5.sh release
/bin/sh tests/verify-phase5.sh asan
/bin/sh tests/verify-phase5.sh race
```

需准备 go.mod 锁定缓存、OpenSSH、busybox-extras 和 devpts。真实服务用例绑定 80／22／23，须在隔离网络执行。脚本保留 phase5 输出名，表示继承回归入口，不表示复用旧结果。进程检查用独立空仓库和 loopback 启动当前 Linux Server，读取 /api/v1/devices 后 SIGTERM 并等待退出码 0。

## 已知限制与交付边界

- 发布 Windows x64 非 MSIX 自包含目录，目标机需 Visual C++ x64 运行库；未执行 Windows ARM64／x86、全部 Windows 10 版本或多物理显示器矩阵。
- 外部入口测试覆盖启动边界与实际参数，不替代用户的浏览器、SSH／PuTTY 登录、主机密钥确认或厂商网页测试。真实通道字节流由 Phase 4／5 回归覆盖。
- 只有 Repository 持久化；WebSocket 不重放、无实时 stdout，服务器重启后不保证幂等。倒计时受本机时钟影响；API 是事实来源。
- Probe mipsel／ARM／ARM64、uClibc／老内核实机矩阵仍未执行，不能从 Linux x86_64 推断通过。
- 认证／TLS／RBAC、Web、微信、MCP、AI、自研 SSH／Telnet、通用转发及新数据面未实现。

独立提交标题：`refactor: rebuild Windows workbench with WinUI 3`。实际 SHA 与 main 推送状态以 Git 记录及交付回复为准；推送后停止等待验收。
