# 旧 UI 清理与当前 C# 交付

2026-09-08，按用户明确要求与 Accepted ADR-037，只保留新版 C# 主 UI 和 C# 探针模板生成器，并提交推送当前项目。

## 保留与移除

| 保留 | 当前职责 |
| --- | --- |
| windows/RouterWorkbench.Desktop | .NET 10 / WPF 主 UI，设备/维护/文件/配置/设置 |
| windows/RouterWorkbench.Client、RouterWorkbench.Core | 公开 HTTP/WS、幂等/快照/取消、配置与外部客户端启动 |
| src/ProbeTemplateGenerator、ProbeTemplateGenerator.sln | .NET 10 / Blazor / Fluent UI 生成器 |
| windows/RouterWorkbench.Desktop.Tests、tests/ProbeTemplateGenerator.Tests | 当前客户端、协议对端、生成器与兼容测试 |
| tests/template-generator-browser.mjs、tests/package.json、锁文件 | 独立 Playwright 浏览器验证，沿用已有 1.63.0 版本，不参与产品构建 |
| ui-windows.cmd、windows/build-desktop.ps1、template-generator.cmd/.ps1 | 两套当前 UI 的启动/构建入口 |

移除源码目录 `frontend`（React/预览/旧生成器残留）、`windows/RouterWorkbench`（WinUI/WebView2）、`windows/native`（Win32/WebView2）、`windows/RouterWorkbench.Tests` 及其专属脚本：build.ps1、build-native.ps1、Install-Prerequisites.ps1、verify-native/published/sandbox/terminal-services.ps1、terminal-test.Dockerfile。

新版依赖的 TestProbe 原样迁入 Desktop.Tests 并调整命名空间；Core 删除无调用方的 EmbeddedTerminal、TerminalSessions、PickedFileSave 和旧 JSON Bridge/本地网页资源策略，保留 WPF 使用的 profile/endpoint 校验及外部启动。生成器测试改为从 tests 安装 Playwright，不再引用 frontend/node_modules；兼容基准 fixture 与旧草稿导入逻辑保持。

自动审批拒绝批量递归删除，因此旧源码移入 Git 忽略的 `build/ui-cleanup/retired-source`，另有 `legacy-ui-source.zip` 保存清理前源码。旧目录已不在当前源码/Git 提交中，本地归档可用于恢复；不清理既有 build 发布目录或用户运行数据，不终止用户原有进程。`.gitignore` 排除整个 data 运行目录及根目录 probe.zip，提交不含这些内容、SDK、依赖缓存或产物。

ADR-037 明确取代旧 UI 保留要求；AGENTS、README、Windows README、DEVELOPMENT、ARCHITECTURE、API 的调用方说明与三个状态入口已同步。旧 ADR 原文和专项历史验证仍保留，UI_FREEZE/PHASE6_DESIGN 标明历史适用范围。Go Server、C++ Probe 与 API/TCP 的现有产品改动按用户“当前项目”授权一并提交，本次清理未追加其业务变更。

## 本次验证

环境：Windows x64，仓库 `build/dotnet10/dotnet.exe` .NET 10 SDK，已安装 Edge，项目外 WSL 2 `RouterAgentTest`（BusyBox）。

| 命令 / 操作 | 结果 |
| --- | --- |
| `powershell.exe -NoProfile -ExecutionPolicy Bypass -File windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-cleanup` | Release 自包含发布、当前 Go Server 编译、98 项 WPF/Client/协议测试对端检查通过，生成 12 份实际 WPF XPS 布局 |
| 设置 `RMP_GENERATOR_WSL=RouterAgentTest`；`build/dotnet10/dotnet.exe test ProbeTemplateGenerator.sln -c Release --nologo` | 145 passed，0 failed，0 skipped；保留公式/规则/草稿/发布/旧命令兼容与实际 BusyBox 检查 |
| `build/dotnet10/dotnet.exe publish src/ProbeTemplateGenerator/ProbeTemplateGenerator.csproj -c Release --no-restore -o build/ui-cleanup/generator-publish --nologo` | Release 发布通过 |
| `npm.cmd --prefix tests ci --ignore-scripts --no-audit --no-fund` | 独立 tests 锁文件安装成功，3 个测试包；无需旧 frontend |
| 从 generator-publish 运行 `dotnet ProbeTemplateGenerator.dll --urls http://127.0.0.1:5193`；设置 GENERATOR_URL、RMP_SERVER_BIN 和 RMP_GENERATOR_OUTPUT，执行 `npm.cmd --prefix tests run test:generator` | 发布目录静态资源、Edge 实际编辑/导入导出/主题/宽窄布局/真实 Go API 发布等 8 组流程通过，无浏览器错误，结果在 build/ui-cleanup/browser |
| 启动本次自包含 RouterWorkbench.exe，确认 Router Workbench 主窗口，再调用正常窗口关闭 | 发布包成功启动并正常结束；过程记录在 build/ui-cleanup/desktop-smoke.json，跨命令进程对象未取得退出码，不宣称退出码 0 |
| `go test ./...`、`go vet ./...` | Windows 适用测试与静态检查通过；部分测试使用缓存，Windows/Linux 平台与真实 Probe 的覆盖边界保持 |
| `git diff --cached --check`、Markdown 相对链接与提交路径检查 | 通过；本地文档链接无失效项，新增/修改提交不包含旧 UI 目录、运行数据、依赖缓存或构建产物 |

WPF 检查包含维护重复关闭/重开 409 回归、旧快照隔离、连接状态与五秒刷新不重建行、文件与配置结果、DPAPI、原幂等字节和取消释放。生成器在新发布目录进行验证，原用户发布包与进程保留。

本次未修改 UI 视觉或 Probe 业务，不重跑 Linux 全量阶段测试、C++ sanitizer、物理 DPI 或厂商实机。既有厂商设备、干净目标机与数据恢复遗留继续见 PROJECT_STATUS；这些自动检查不替代 Phase 6 最终产品验收。

本次验证包：`build/windows-desktop-cleanup/win-x64/RouterWorkbench.exe` 与 `build/ui-cleanup/generator-publish`。默认构建入口仍输出各自常规目录；源码推送到 `origin/main`，实际提交 SHA 和远端状态由 Git 提供。
