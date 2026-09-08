# 项目接管手册

2026-09-08 当前基线：只保留新版 C# / WPF 主 UI 与 C# / Blazor / Fluent UI 探针模板生成器（ADR-037/036/035/034）。旧 React、WinUI、Win32/WebView2 UI 及专属脚本、Bridge/ConPTY 已移出源码；不要依照旧阶段文档恢复它们。用户已授权本次当前项目提交并推送 `origin/main`，实际 SHA 和远端状态查 Git。

依次阅读 AGENTS → 本文件 → [PROJECT_STATUS](PROJECT_STATUS.md) → ARCHITECTURE → ROADMAP → 相关 API/PROTOCOL/DECISIONS，再读 [DEVELOPMENT](DEVELOPMENT.md) 并核对当前代码、Git 和验证结果。普通需求自主完成范围内实现、验证和文档交付，沿用 ADR-028，不进入新阶段。

| 当前入口 | 接管位置 |
| --- | --- |
| `ui-windows.cmd` / `windows/build-desktop.ps1` | Desktop 为 WPF UI，Client 为公开 API/WS，Core 为配置与外部启动；[WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md) |
| `template-generator.cmd` / `template-generator.ps1` | `src/ProbeTemplateGenerator`；编辑状态 EditorWorkspace，公式 ExpressionParser/TemplateCompiler，发布 TemplatePublishingService；[TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md) |
| `server-windows.cmd` / `server-windows.ps1` | Go Server，持久数据 `data/server/repository`；[DEPLOYMENT](DEPLOYMENT.md) |
| 当前验证 | Desktop.Tests 已自带 TestProbe，生成器 C# 测试位于 tests/ProbeTemplateGenerator.Tests，Playwright 仅在 tests/package.json；命令见 DEVELOPMENT |

本次旧 UI 清理验证：98 项 WPF/当前 Go Server/协议对端检查、145 项 C# 生成器测试（含实际 WSL BusyBox）、发布目录 Edge/Go API 8 组流程、两套 Release 发布与 Windows Go test/vet 通过。清单和最新验证包位置见 [UI_CLEANUP](UI_CLEANUP.md)。旧源码归档在 Git 忽略的 `build/ui-cleanup`；旧发布包、用户进程、草稿与运行数据保留，不推送到 GitHub。

主 UI 导航仅设备/维护/文件/配置/设置；设置保存服务器，启动自动连接。维护使用公共 Web/SSH/Telnet 外部入口，SSH 默认 admin/admin 可修改并本机加密保存；无内置终端、通用任务或客户工具管理。设备属性是上报模板快照，五秒回查按值更新，不重建所选属性行。维护重开 409 与连接状态/闪烁修复已含在当前 98 项检查中。

保留公开 API/WS、原幂等键和字节、切换/退出取消并等待；文件 committed/released 与 Task RESULT 分开，维护默认 240 分钟与固定 Probe 127.0.0.1:80/22/23、独立数据 TCP、默认 24 小时端口隔离保持。Probe/Server 的模板及配置任务证据在 PROBE_TEMPLATES_VERIFICATION / ROUTER_CONFIG_VERIFICATION，不把测试对端当厂商实机。

**真实遗留：此前 Agent 清理测试挂载误删原工作区。源码从 f73853f 加补丁恢复并重新验证；`cmd/server/1.txt` 与 `cmd/server/data/` 未恢复，需要备份来源。`build/windows-react` 曾从 09:13:52 卷影副本恢复，不是新版产物。详情见 [PROBE_TEMPLATES_VERIFICATION](PROBE_TEMPLATES_VERIFICATION.md)，不得隐去或把空数据目录当作恢复。**

Phase 0～5 已验收。当前 WPF/Blazor 的厂商固件、SSH/Telnet、物理 DPI/鼠标/选择器/剪贴板及干净目标机，Phase 6 最终产品验收仍待完成；C++ sanitizer 既有缺运行库问题保持。Linux 复用项目外 `RouterAgentTest`，见 [WSL_TEST_ENVIRONMENT](WSL_TEST_ENVIRONMENT.md)。旧 Win32/ConPTY 的未完成验收保留为历史记录，已无当前旧 UI 实现。

Phase 7/8、微信小程序、正式公网 Web、新 Tunnel 和大规模架构扩展暂缓；管理员工具入口及认证/TLS/RBAC 等尚未建设或未决。完成用户当前需求后交付并停止；未来提交/推送仍按当次明确授权。
