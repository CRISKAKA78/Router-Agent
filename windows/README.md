# Windows 远程维护工作台

C# / .NET 10 LTS / WinUI 3，中文 Windows 11 维护客户端。Fluent 控件、Mica、设备侧栏、分区导航、状态提示条和原生弹窗，支持浅色、深色及跟随系统。

## 安装和连接

复制整个发布目录到 Windows x64 电脑，运行 `RouterWorkbench.exe`。保留 DLL、资源和子目录，不能只复制 EXE。目录自带 .NET 和所需 Windows App SDK 组件，不需要 SDK、Node 或独立安装 Windows App Runtime。目标机需要[微软 Visual C++ x64 运行库](https://learn.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist)。实际验证 Windows 11 build 26200；最低目标 Windows 10 build 19041，旧系统矩阵未全部验证。

左下角“设置”填写管理服务器地址，例如 `http://127.0.0.1:8080`，点击“保存并连接”。地址不含 `/api/v1`，不是探针控制或隧道数据地址。右上角显示“已连接 · 实时同步”后可操作。详细地址和外部客户端配置保留在设置中。

配置位于 `%LOCALAPPDATA%\RouterWorkbench\profile.json`，兼容首版配置，新增外观偏好。仅保存地址、外观、客户端路径及 SSH 用户名，不保存密码、业务快照或隧道内部标识。API 面向可信本机或受保护管理网络；认证、TLS、RBAC 仍属部署边界与后续设计。使用系统证书验证。

## 日常使用

1. 左侧搜索并选择设备，查看名称、编号、在线和维护状态。
2. 在概览的维护卡片点击“开启远程维护”。默认 240 分钟；“调整”可设置正数分钟，支持对应整数毫秒的小数，范围与 API 一致。
3. 创建后直接看到剩余时间、三个临时入口和按钮；随时可“关闭维护”。维护记录、会话编号、释放事实及原因在“详细信息”中。
4. Web 调用默认浏览器；SSH 默认系统 `OpenSSH\ssh.exe`，Telnet 默认系统 `telnet.exe`。可指定已有 PuTTY.exe 并勾选对应模式。SSH 用户名默认 root；密码和主机密钥确认由外部客户端处理，不禁用验证、不自研协议。
5. “命令与任务”执行基础命令、查看服务器状态、退出码、标准输出／标准错误和截断标记。派发、编号及文件提交／释放事实在详情中。重发原任务保留业务身份，没有实时输出流或新增取消能力。
6. “文件”导入本机文件、另存、上传到设备、从设备下载和归档。下载完成后在任务页显式导入仓库或清理暂存。采用支持普通／管理员桌面进程的 Windows App SDK 文件选择器。
7. “工具与版本”创建工具、发布多产物不可变版本、查看服务器兼容性并投放所选兼容产物。规则多值用逗号分隔。投放只上传文件，执行时另行发起命令。

退出工作台不会杀死外部客户端，也不自动撤销服务器已有任务或维护。维护到期、会话替换和主动关闭仍由服务器决定。

## 连接恢复和错误

WebSocket 只通知刷新，首连和每次重连均获取 HTTP 完整快照，不假设历史事件重放。断线退避 1/2/4/8/16/30 秒，另每 5 秒回查；未同步时禁用写入和入口打开。页面切换不重复订阅，切换服务器先释放旧连接及在途网络操作。

倒计时是本机显示估计，到期回查服务器，不伪造“已关闭”；打开入口前也会查询维护。API 始终是业务事实来源。

写入绑定固定内容和幂等键，不自动重试。响应不确定时用“处理请求”核对状态，仅在服务器未重启时“重试原请求”；已核对副作用后可解除保护。服务器重启后旧键不保证去重。非空任务编号和派发不确定提示会保留。

主错误提示中文化，原始 API code、HTTP 状态和说明放入详情；区分不可达、设备离线、会话替换、维护创建失败、容量不足和文件／工具失败。共享错误码 `capacity_exhausted` 提示“维护端口池或服务器容量不足”，不猜测唯一内部原因。

列表每页 200 项，超过 10000 项显式失败。导入上限 1 GiB，受服务器更小限制约束。HTTP 超时 40 秒，文件保存体超时 5 分钟。

## 构建和验证

Windows 仓库根目录，安装 .NET 10 SDK 和 Go 后运行：

```powershell
./windows/build.ps1 -Verify
./windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify
```

默认目录 `build/windows-winui/win-x64`；可用 `-OutputDirectory` 指定新目录。请部署完整目录，不覆盖旧 WinForms 单文件包。固定微软 WinUI 组件和 Windows SDK BuildTools NuGet，无须 Visual Studio。未引用无关 AI／ML 包；WinUI 传递依赖含 WebView2 接口，程序不创建 WebView2，也不分发 Chromium。

验证包含独立 Core API／故障测试和真正的 WinUI 窗口，后者仅在 `VerifyUI=true` 编译，正式发布明确为 false。日志和截图在 `build/windows-winui/verification-*`。100% 桌面及实际 XAML 200% 光栅化、主题、窗口缩放、断线恢复与关闭均有验证；200% 使用公开 XAML 岛宿主，不改变用户显示设置。不同显示器移动和硬件组合仍需现场验证。

构建脚本还会独立启动正式发布程序，确认应用编译资源、本目录 WinUI 运行库及正常关闭。若发生未预期的界面初始化错误，可查看 `%LOCALAPPDATA%\RouterWorkbench\last-error.log` 的时间和诊断；普通网络／业务错误显示在界面中。

完整服务器／真实探针／Tunnel 回归沿用 `tests/verify-phase5.sh release|asan|race`，见 [Phase 6 验证记录](../docs/PHASE6_VERIFICATION.md)。

依据：[稳定版本](https://learn.microsoft.com/en-us/windows/apps/windows-app-sdk/release-channels)、[自包含部署](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/self-contained-deploy/deploy-self-contained-apps)、[桌面文件选择器](https://learn.microsoft.com/en-us/windows/apps/develop/files/using-file-folder-pickers)。
