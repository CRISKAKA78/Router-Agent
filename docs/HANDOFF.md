# 项目接管手册

Phase 0～5 已验收。当前为用户授权的 Phase 6 Shared React / WebView2 重构，起点 `6f0ce71a5027b57d51e9a6c807794be45f7633b5`，正式架构依据 ADR-026；不以此前纯 XAML 实现继续开发。

依次阅读 AGENTS → 本文件 → PROJECT_STATUS → ARCHITECTURE → ROADMAP → 相关 API/PROTOCOL/DECISIONS，再核对实际代码、Git 和验证结果。

**React Shared Frontend 是今后 Windows 与 Web 的统一产品 UI 基线。**

- `frontend/`：React、TypeScript、Vite、Tailwind、Lucide；唯一的业务 HTTP/WS Client、状态与页面。
- `windows/RouterWorkbench/`：WinUI 薄 Shell 和 WebView2 本地资源；`RouterWorkbench.Core/` 只有平台策略、配置、外部启动。
- [PHASE6_DESIGN](PHASE6_DESIGN.md)：同源本地内容加载、Bridge、语义迁移和生命周期。
- [Windows README](../windows/README.md)、`windows/build.ps1`：构建、离线目录、运行要求。
- [PHASE6_VERIFICATION](PHASE6_VERIFICATION.md)：本轮测试命令与实际结果；不能用旧纯 XAML 的 61/98 项结果代替本轮证据。
- `frontend/tests/native.mjs` 与 `windows/verify-native.ps1`：真实 WebView2 / Go API / 测试对端闭环；测试专用捕获与调试端口不进入正式 EXE。
- `windows/verify-published.ps1`：正式原生窗口、随包 WebView2/WinUI、移除开发工具的子进程 PATH、正常退出检查；React 画面另通过实际窗口截图核对。
- `windows/verify-sandbox.ps1`：关闭网络的 Windows Sandbox 检查入口。本机策略拒绝未签名 EXE，未完成干净环境运行；用户随后明确改为“直接在本机测试”，未修改系统策略。

首次连接需在设置中保存 Server。API Origin 策略保持，Shell 用所选 Server origin 的本地拦截页面访问同源 API；服务器不需要提供静态页面。浏览器开发用 Vite proxy，未来正式 Web 部署尚未授权。

保留原幂等边界：不确定响应保留相同键和字节，确认 Server 未重启后显式重试；禁止自动创建替代任务。文件 committed/released 独立于 Task RESULT，下载 complete 保持资产身份。Maintenance 默认 240 分钟、自定义正租期、固定三入口和 Session 绑定不变。配置不保存业务快照、密码或 Tunnel 私有字段。

Phase 1～5 继续使用 `tests/verify-phase5.sh release|asan|race`，真实 80/22/23 服务测试必须在隔离网络/devpts 执行。Windows 适用 Go test/vet/build 同样保留。

历史稳定基线：Phase 5 `57c2b1f8f6da1069e4a2eb988224b94bafe9cf84`；首版 Windows `f8d099d6bb6ac4830199b122755cbf37f6a9e849`；纯 WinUI `6f0ce71a5027b57d51e9a6c807794be45f7633b5`。本轮最终状态与 SHA 以 PROJECT_STATUS、专项验证和 Git 为准。完成推送后停止等待验收，不进入下一阶段。
