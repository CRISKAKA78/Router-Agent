# Windows 原生 C# 工作台

双击根目录 [ui-windows.cmd](../ui-windows.cmd)，构建并启动 .NET 10 / WPF 客户端。发布文件为 `build/windows-desktop/win-x64/RouterWorkbench.exe`，自包含 x64 EXE，不需要目标机安装 .NET 或 WebView2。构建机需要 .NET 10 SDK；脚本优先使用 `build/dotnet10/dotnet.exe`，也可指定 `-Dotnet`。

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-desktop.ps1 -BuildOnly
powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-desktop.ps1 -BuildOnly -Verify
```

`-Verify` 另需 Go，编译隔离测试 Server 并执行 Desktop.Tests。目标 EXE 正在运行时脚本拒绝覆盖，可指定 `-OutputName windows-desktop-next`。不停止用户进程或清理运行数据。

在设置中保存服务器地址（例如 `http://127.0.0.1:8080`，不附 `/api/v1`），后续启动自动连接。导航为设备、维护、文件、配置、设置；支持中文浅色/深色/系统主题。

维护显示 Web/SSH/Telnet 公共链接并打开系统或已配置的外部客户端。SSH 新配置默认 admin/admin，可修改；密码通过当前 Windows 用户 DPAPI 加密保存，在维护页显式复制给外部客户端。没有内置终端、客户工具管理或通用任务入口；文件传输和配置结果留在各自页面。外部客户端与服务器维护不随工作台退出关闭。

配置保存在 `%LOCALAPPDATA%/RouterWorkbench/profile.json`；兼容既有配置，不缓存业务快照或 Tunnel 私有身份。完整行为与验证见 [WINDOWS_DESKTOP_MIGRATION](../docs/WINDOWS_DESKTOP_MIGRATION.md)。

| 工程 | 职责 |
| --- | --- |
| RouterWorkbench.Desktop | 原生 WPF 页面、布局、交互和本机密码保护 |
| RouterWorkbench.Client | 公开 API DTO、HTTP/WS、快照、幂等请求及取消释放 |
| RouterWorkbench.Core | 配置、外部客户端启动和端点校验 |
| RouterWorkbench.Desktop.Tests | 当前 WPF/Go API 集成与测试专用协议对端 |

独立生成器通过 [template-generator.cmd](../template-generator.cmd) 使用，源码在 [src/ProbeTemplateGenerator](../src/ProbeTemplateGenerator)，不依赖本工作台宿主。ADR-037 已移除 React、WinUI、Win32/WebView2 旧 UI 与专属脚本；历史记录见 [PHASE6_VERIFICATION](../docs/PHASE6_VERIFICATION.md)，清理说明见 [UI_CLEANUP](../docs/UI_CLEANUP.md)。
