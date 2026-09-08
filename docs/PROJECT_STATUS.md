# 项目状态

最后更新：2026-09-08。

当前仅保留新版 C# / WPF 主工作台与 C# / Blazor / Fluent UI 探针模板生成器（ADR-037/036/035/034）。React 浏览器工作台、WinUI/Win32/WebView2 旧宿主、专属构建/验证脚本及旧 Bridge/ConPTY 已移出源码；当前 Client、Core 平台能力与测试仍保留，详见 [UI_CLEANUP](UI_CLEANUP.md)。本次用户已授权将当前产品源码和文档提交并推送 `origin/main`，实际提交/远端状态以 Git 为准；运行数据、旧包、源码压缩包和构建产物不纳入提交。

- 主 UI：`ui-windows.cmd` → `windows/build-desktop.ps1`。设备、维护、文件、配置、设置；启动自动连接，连接状态与输出同步，五秒刷新保持所选属性行。维护只展示公共链接并打开外部客户端，SSH 默认 admin/admin，可编辑并 DPAPI 加密保存；模板全部属性与失败项按上报快照展示。没有内置终端、客户工具管理或通用任务入口。文件/配置结果留在各自页面，管理员工具入口尚未建设。
- 生成器：`template-generator.cmd` → `src/ProbeTemplateGenerator`，本机浏览器默认 5188。支持虚拟/展示属性、command/NVRAM/UCI、公式/条件规则、预览、工程导入导出、草稿及服务器发布；版本 1/2 与旧原生草稿兼容。两套 UI 构建均不需要旧前端或 Node。
- Server/Probe：保留 Go 管理端与 C++11 Probe，模板持久化、启动采集、默认 `nvram get SN` 设备 ID、NVRAM/UCI 专用任务已实现；本次 UI 清理未改它们的业务逻辑或 API/TCP。Repository 与模板持久化，Session/Task/维护/幂等账本仍不跨进程恢复。

本次验证：WPF Release 自包含发布及 98 项当前 Go API/协议对端/原生检查通过；生成器 145 项 C# 测试（0 failed/0 skipped，含 WSL BusyBox）和 Release 发布通过；独立 tests 依赖安装及发布目录 Edge/Go API 8 组流程通过（无浏览器错误）；Windows `go test ./...` 与 `go vet ./...` 通过。详细命令、结果和清理后的发布位置见 [UI_CLEANUP](UI_CLEANUP.md)。以前的专项证据保留在 [WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md)、[TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md)、[ROUTER_CONFIG_VERIFICATION](ROUTER_CONFIG_VERIFICATION.md) 与 [PROBE_TEMPLATES_VERIFICATION](PROBE_TEMPLATES_VERIFICATION.md)。

当前方向为完善 Router-Agent，不进入后续阶段（ADR-028）。Phase 0～5 已验收；Phase 6 厂商 SSH/Telnet、NVRAM/UCI 固件副作用、交叉架构、物理多屏 DPI/鼠标/文件选择器/剪贴板及干净目标机仍待验收。历史 Win32/ConPTY 验收未完成且旧 UI 已退役，不视为当前产品待修实现。此前 C++ sanitizer 因 WSL 工具链缺运行库未完成；本次未改 Probe，不重跑 Linux 全量阶段回归或 sanitizer。

**必须保留的数据恢复遗留：此前 Agent 清理测试挂载误删原工作区，源码从远端 f73853f 加任务补丁恢复并重新验证；原未跟踪 `cmd/server/1.txt`、`cmd/server/data/` 尚未恢复，仍需备份来源。旧 Git 元数据不声称原样恢复，`build/windows-react` 曾从当天 09:13:52 卷影副本恢复，不是新版发布。详见 [恢复记录](PROBE_TEMPLATES_VERIFICATION.md)。本次旧 UI 清理不改变此事件或恢复状态。**

测试环境使用项目外 WSL 2 `RouterAgentTest`，见 [WSL_TEST_ENVIRONMENT](WSL_TEST_ENVIRONMENT.md)。Windows Server 入口 `server-windows.cmd`，数据在 `data/server/repository`；本机双栈接入历史验证通过，公网 IPv6 和厂商路由器尚待实测，使用见 [DEPLOYMENT](DEPLOYMENT.md)。

Phase 7 MCP、Phase 8 AI Agent、微信小程序、正式公网 Web 部署、新 Tunnel 数据面及其他大规模架构扩展暂缓。认证/TLS/RBAC、租户、完整审计等仍未决；不自动开展下一项功能。
