# Windows 远程维护工作台

Windows 客户端采用 C# / WinUI 3 Thin Shell + WebView2。**React Shared Frontend 是今后 Windows 与 Web 的统一产品 UI 基线。** 业务页面、HTTP/WebSocket、状态和错误处理位于 `frontend/`；C# 不再维护第二套业务 API Client 或 XAML 页面。

## 使用

发布目录为 `build/windows-react/win-x64/`。保留完整目录，直接启动 `RouterWorkbench.exe`。包内含 .NET、WinUI、Frontend、固定版本 WebView2Runtime 和 app-local VC DLL，目标机不需要 Node、Vite、.NET SDK、提权安装或在线前端服务。另附 `Install-Prerequisites.ps1` 和 VC++ 离线安装器，供采用系统集中运行库的部署使用。

在“系统设置”输入 `http(s)://主机:端口`，保存并连接。选择设备 → 开启远程维护 → Web / SSH / Telnet。默认 240 分钟，自定义正租期没有 360 分钟上限。SSH/Telnet 默认打开页面内置终端，需要本机 ssh.exe / telnet.exe；可在原生选择器中选择相应命令行客户端。远程维护的“新建会话”菜单保留外部 SSH/Telnet，可调用系统客户端或已选择的 PuTTY；GUI PuTTY 只用于外部入口。Web 使用默认浏览器。工作台不安装这些运维客户端、不自研登录协议。

内置终端支持输入、窗口尺寸、字号、清屏、复制和独立 SSH/Telnet 会话；主机密钥与认证使用客户端原生提示，不自动接受未知密钥，不保存密码。离开维护页、切换设备/服务器、Session 替换、维护关闭/到期或退出会关闭内置进程。右侧目录点击刷新后通过单次只读 Exec 读取，最多 250 项；上传下载仍使用既有文件任务。

文件先导入仓库再上传到设备；设备下载在任务中显示 committed/released 与最终 RESULT，显式导入后成为稳定资产。工具版本可包含多个 Artifact，兼容性由 Server 判定。响应不确定时先核对 Server 是否重启，再显式重试相同请求或核对后放弃。

配置：`%LOCALAPPDATA%\RouterWorkbench\profile.json`，只保存地址、主题、客户端路径和用户名。WebView2 本地用户数据也在此目录；不缓存业务快照或 Tunnel 身份。退出关闭内置终端并等待网络资源释放，不关闭用户已打开的外部程序，也不撤销 Server 已创建的维护或任务。

## 构建与验证

构建机需要 Windows x64、.NET 10 SDK、Node 20.19+/22.12+（本轮使用 Node 24）、Go（验证时）。

```powershell
./windows/build.ps1 -Verify
# 仓库中独立 SDK：
./windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify
./windows/verify-sandbox.ps1
# 可选：Docker 隔离的真实 SSH / Telnet 服务（本机需对应命令行客户端）
docker build -t router-agent-terminal-test:local -f windows/terminal-test.Dockerfile windows
./windows/verify-terminal-services.ps1 -Dotnet ./build/dotnet/dotnet.exe
```

脚本执行 npm ci、TypeScript/Vite production build、Windows Release publish；默认下载并缓存微软 Fixed Version WebView2 152.0.4191.62 与 VC++ 离线安装器，核对 Microsoft 签名。已有离线 Fixed Runtime 可通过 `-WebView2Runtime <目录>` 提供。发布资源不要求目标机下载。固定 WebView2 版本由后续发布维护者升级并回归。

`-Verify` 执行 Vitest、原生策略测试、真实 Go API + WebView2 全流程，再检查正式发布 EXE 的正常启动/退出。测试临时使用 loopback 18080/18081/18082、调试 19222、维护端口 32200～32299，请确保空闲。调试端口和入口捕获只编译进 VerifyUI=true；正式包禁用 DevTools，不带 Probe 测试代码。

可选终端测试占用 loopback 20222/20223，容器内生成一次性 SSH 密码/主机密钥；已知主机文件只放测试目录，测试结束删除密码并移除自身容器。检查 Windows 原生客户端经 ConPTY 完成实际命令往返和 SSH 远端窗口更新，不修改用户 known_hosts。Docker 仅用于验证，不是 Windows 产品运行依赖。

Sandbox 验证需要 Windows Sandbox 功能；映射发布目录只读、结果目录可写、网络关闭，在新环境复制到本地后启动正式程序，检查无 Node/dotnet 命令、真实 React 无障碍树、本地运行库和退出。结果在 `build/react-shell/sandbox-*`。本轮沙盒 Application Control 拒绝未签名 EXE，用户随后明确要求直接在本机测试；本机通过不等于干净目标机已通过，详见专项验证记录。

```text
frontend/
  src/              React UI / API / WebSocket / platform / design system
  tests/native.mjs  实际 WebView2 集成验证
  dist/             Vite 生产静态资源（构建生成）
windows/
  RouterWorkbench/       WinUI Shell / WebView2 / local resources
  RouterWorkbench.Core/  profile / launch / bridge policy
  RouterWorkbench.Tests/ native policy / fixture / Windows picker tests
build/windows-react/win-x64/
  RouterWorkbench.exe + .NET / WinUI 文件
  Frontend/index.html + assets/
  WebView2Runtime/        随包固定版本运行库
  Prerequisites/         VC++ 离线安装器
```

## 浏览器开发与后续复用

```powershell
$env:RMP_SERVER='http://127.0.0.1:8080'
npm --prefix frontend ci
npm --prefix frontend run dev
```

浏览器使用 Vite 显示的同源地址连接。开发代理只绑定 loopback；正式部署应由同源反向代理承载静态资源和 `/api/v1`。浏览器平台通过文件选择/Blob 下载和系统浏览器完成相应能力，SSH/Telnet 显示本机客户端连接提示。业务组件不区分 Windows 与 Browser。公网 Web 部署、认证/TLS/RBAC 仍属后续设计，本轮未实施。

设计与实际限制见 [PHASE6_DESIGN](../docs/PHASE6_DESIGN.md)、[PHASE6_VERIFICATION](../docs/PHASE6_VERIFICATION.md) 和 ADR-026。Windows ARM64/x86、多物理显示器、全部旧 Windows 版本及各路由器实机架构矩阵仍需相应环境验收。
