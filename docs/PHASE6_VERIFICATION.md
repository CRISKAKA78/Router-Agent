# Phase 6 Shared React / WebView2 验证记录

日期：2026-09-06。起点为干净 main / origin/main `6f0ce71a5027b57d51e9a6c807794be45f7633b5`，架构依据 Accepted ADR-026。本次 Server、Probe、Go 依赖、公开 API、Protocol 与 Tunnel 生产代码未改动。旧纯 XAML 的 61/98 项历史结果不作为本轮证据。

**React Shared Frontend 是今后 Windows 与 Web 的统一产品 UI 基线。** 本轮交付可复用源码和 Windows 本地发布目录，不部署正式公网 Web。

## 构建与客户端验证

`windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify` 从锁文件恢复前端依赖，执行 TypeScript/Vite production build、Vitest、Windows Release publish、原生策略与真实 WebView2 集成。最终构建通过，0 警告、0 错误；production JS 247.14 kB、CSS 17.60 kB，未压缩原始体积。前端 9 项测试、原生策略/保存 21 项检查、实际 WebView2 26 项集成检查通过。

| 要求 | 本轮实际证据 |
| --- | --- |
| 本地资源 / SPA | 实际 WebView2 加载 production assets；受控 index 加 hash 路由、刷新后恢复；不存在 Vite 开发服务器依赖 |
| Bridge 正常与拒绝 | 实际 getProfile/连接/三入口/Windows 保存；任意 exec、非法 endpoint、脚本设置可执行路径被拒绝；策略覆盖未知方法、重复字段、外部来源/协议、路径穿越、参数注入 |
| Server 连接/切换 | React 经真实 Go HTTP/WebSocket 连接；切换另一 Server 清空旧设备/任务，再切回创建新连接所有者 |
| Device / Session replacement | 真实 Protocol v1 测试对端注册；旧连接下线与新 Session 替换后回查，界面显示新 Session |
| WebSocket 恢复 | 关闭真实 socket，断线期间替换 Session；新 socket 连入后 HTTP 全快照恢复；单元测试覆盖查询中通知不丢失、非法首事件拒绝后重连 |
| Maintenance | 实际双击仅创建一次；默认省略租期后期限精确 240 分钟；自定义 300ms 由 Server 到期、1000 分钟接受；主动关闭与三个 ready 入口 |
| Web / SSH / Telnet | 三按钮启动前回查 API，实际跨 Native Bridge 到测试捕获边界；平台测试验证默认浏览器 URL、系统 SSH 独立参数；生产保留系统客户端/PuTTY 启动实现 |
| Exec / Task | React 提交、真实 API 派发、测试对端 ACK/RESULT、页面显示最终结果；不把 202 作为任务成功 |
| File | 浏览器 File API 打开 Windows 选择器导入真实文件；Save Picker 选路径并逐字节核对输出；设备 upload 成功、download committed/released、显式 complete 后稳定资产 |
| Tool | 创建工具、版本/Artifact、查询 Server compatibility、显式投放并完成真实 API 流程 |
| 幂等 / 重复点击 | 在途写入单次准入；不确定响应保留相同键和相同 body 字节、拒绝新请求，显式重试；业务错误清除不确定状态；真实维护双击检查 |
| 网络 / 业务错误 | Vitest 分离 transport/解析不确定与结构化 API 错误；实际 WS 断线/下线/切换恢复；错误显示于页面及当前表单 |
| Light / Dark | 实际 WebView2 切换主题并截图；系统主题使用 matchMedia，本地配置保存偏好 |
| DPI / 窗口布局 | 实际 WebView2 的 960px、deviceScaleFactor=2 CDP 布局验证，无页面水平溢出；浅色/深色/任务/工具截图复核 |
| 退出 / 资源释放 | 单元测试取消阻塞 HTTP、join socket、dispose 幂等且不迟到发布；原生 WM_CLOSE 等待 JS shutdown 后退出 0；保存临时文件失败时清理且保留原目标 |
| 发布目录 | .NET/WinUI 自包含、Frontend 静态资源、Fixed Version WebView2、app-local VC DLL、可选 VC 离线安装器；正式非测试 EXE 本机启动/退出 |

集成运行日志：`build/react-shell-delivery.log`。最终 UI 截图与 fixture 日志：`build/react-shell/verification-20260906-203637/`。每次复现生成新的时间目录，构建产物与日志不提交 Git。`VerifyUI=true` 才包含 CDP 19222、测试配置入口与外部启动捕获；正式发布禁用 DevTools，不含测试 Probe。

测试对端只实现 Protocol v1 以验证 UI/API 闭环，不冒充实际 Linux Probe；实际 C++ Probe 及 Web/SSH/Telnet 通道见下面全量回归。外部客户端按钮检查不等于用户 SSH/PuTTY 登录、主机密钥或厂商设备网页验收。DPI 使用 WebView2 CDP emulation，不声称完成物理显示器 200% 或跨屏拖动矩阵。

## 本机与干净环境的验收范围

正式发布为 `build/windows-react/win-x64/`，Node/npm 仅用于构建。发布程序只加载随包前端；随包固定 Runtime 的 Microsoft 签名和 VC 安装器签名均有效。本机将完整包复制到 `build/react-shell/portable-20260906-204100/`，子进程 PATH 只含 Windows/System32，实际窗口正常显示 React 工作台；检查 .NET、WinUI 与 WebView2 进程均来自该副本，正常关闭 exit=0。证据为 `build/react-shell-portable.log` 与 `build/react-shell/portable-production.png`。这不卸载本机开发工具或改变系统 PATH。

正式 WinUI WebView2 的 UI Automation 查询在本机返回空节点；该尝试没有作为 React 加载断言。页面内容由 computer-use 实际窗口截图确认；自动 `verify-published.ps1` 检查原生窗口、随包进程/运行库及退出，最终通过，日志为 `build/react-shell-published-final.log`。React DOM 自动断言由前述实际 WebView2 集成测试承担。

实际尝试了网络关闭的 Windows Sandbox：确认没有 Node/dotnet 命令，发布目录只读映射后完整复制，离线 VC 运行准备成功；但 Windows Application Control 拒绝启动未签名 `RouterWorkbench.exe`，错误为 “An Application Control policy has blocked this file”。证据在 `build/react-shell/sandbox-20260906-203148/failure.txt`。未绕过或修改该策略，**干净 Windows 运行未验证通过**。

用户了解阻断后最终明确要求“直接在本机测试”，因此本轮按更新后的本机范围交付。不能把无开发工具 PATH、本地发布副本运行或沙盒准备成功描述为干净目标机独立运行成功。

## Phase 1～5 与 Tunnel 全量回归

本轮重新执行全部既有回归，不复用此前阶段结果。Windows 使用 Go 1.25.5、.NET SDK 10.0.400、Windows 11 build 26200；Linux 使用 WSL 隔离 Alpine、Go 1.26.3、GCC 15.2.0、CMake 4.2.3 与独立网络/mount namespace、devpts。

| 验证 | 本轮结果 |
| --- | --- |
| Linux Release C++11 / CTest | 4/4，4.72 秒 |
| Linux Phase 1～5 全量 Go + 真实 Probe | 全包通过，集成 170.462 秒 |
| C++ ASan / UBSan / LSan CTest | 4/4，6.02 秒，无报告 |
| ASan 真实 Probe Phase 4/5 | 通过，15.570 秒 |
| C++ TSan CTest | 4/4，6.96 秒，无报告 |
| Go race 全量 + TSan 真实 Probe | 全包通过，集成 177.110 秒，无报告 |
| Windows Go 全部源码包测试 | 全部通过，集成 0.144 秒；Linux 专用用例在 Linux 执行 |
| Windows / Linux Go vet 与 Server 构建 | 通过 |
| go mod verify / git diff --check | 通过 |

日志为 `build/react-shell-linux-release.log`、`build/react-shell-linux-asan.log`、`build/react-shell-linux-race.log`、`build/react-shell-windows-go.log`。后端生产差异为空。

## 复现

Windows 仓库根目录：

```powershell
./windows/build.ps1 -Verify
# 使用本轮独立 SDK：
./windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify
./windows/verify-published.ps1 -Executable ./build/windows-react/win-x64/RouterWorkbench.exe
./windows/verify-sandbox.ps1
go test ./cmd/... ./internal/... ./tests/... -count=1
go vet ./cmd/... ./internal/... ./tests/...
go mod verify
git diff --check
```

原生测试使用 loopback HTTP 18080、Probe 控制 18081、测试控制 18082、切换用 18083、CDP 19222、维护池 32200～32299，需空闲。正式程序没有这些测试入口。

Linux 隔离环境：

```sh
/bin/sh tests/verify-phase5.sh release
/bin/sh tests/verify-phase5.sh asan
/bin/sh tests/verify-phase5.sh race
```

需要 go.mod 锁定缓存、OpenSSH、busybox-extras 和 devpts；真实服务绑定 80/22/23，必须在隔离网络运行。脚本名称沿用 Phase 5，执行的仍是本轮源码。

## 已知限制与交付

- Windows x64 目录发布；干净目标机、Windows ARM64/x86、多物理显示器及所有旧 Windows 版本未完成环境矩阵。未签名应用可能被组织 Application Control 拒绝。
- 固定 WebView2/VC 版本需发布者随版本维护；尚未构建升级系统。
- 仅 Repository 持久化；WebSocket 不重放、无实时 stdout，Server 重启后幂等不保证；倒计时受本机时钟影响，API 为事实来源。
- Probe mipsel/ARM/ARM64、uClibc/老内核实机矩阵未执行，不从 Linux x86_64 推断通过。
- 正式公网 Web、认证/TLS/RBAC、微信、MCP、AI、自研 SSH/Telnet、通用转发及新数据面未实现。

本轮独立提交标题为 `refactor: adopt shared React frontend and Windows WebView2 shell`。最终 SHA 与 main 推送状态由实际 Git 记录及交付回复提供。按用户最终指定的本机范围完成验证后推送，停止等待验收。
