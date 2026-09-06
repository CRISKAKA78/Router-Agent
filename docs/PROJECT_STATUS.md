# 项目状态

最后更新：2026-09-06。

Phase 0～5 已验收。当前 Phase 6 从 `6f0ce71a5027b57d51e9a6c807794be45f7633b5` 迁移为 **C# / WinUI 3 Thin Shell + WebView2 + React / TypeScript Shared Frontend**，采用 Accepted ADR-026。

**React Shared Frontend 是今后 Windows 与 Web 的统一产品 UI 基线。** 旧 XAML 业务页和 C# 业务网络层已删除。后端、Probe、API 与 Tunnel 生产代码未修改。

当前实现包括设备/Session、首要 Maintenance 卡片及三入口、默认与自定义正租期、Exec/Task、文件、工具/版本/产物/兼容/投放、中文浅色/深色/系统主题、幂等原请求与快照恢复。Windows Shell 只负责本地内容、窗口、配置和受限平台操作。

构建、完整发布目录、测试命令和实际证据见 [Windows README](../windows/README.md)、[PHASE6_VERIFICATION](PHASE6_VERIFICATION.md)。React production build、9 项前端测试、21 项原生策略/文件检查、26 项真实 WebView2 集成检查、Windows Release 与 Phase 1～5 Linux Release/ASan/race、Windows Go test/vet/build 全部通过。正式发布目录在本机验证启动与正常退出。

干净 Windows Sandbox 已完成无 Node/dotnet 检查和离线运行库准备，但 Application Control 拒绝启动未签名 EXE。用户随后明确要求“直接在本机测试”，本轮按该范围完成验收；不宣称干净目标机运行已经通过。

仅 Repository 持久化；其余 Session/Task/维护/幂等账本不跨 Server 重启。维护固定 Probe 127.0.0.1:80/22/23、默认端口隔离 24 小时。正式公网 Web 部署、认证体系、微信、MCP、AI 与新 Tunnel 未实施。

交付：本轮以独立 commit 推送 main，实际 SHA/推送结果以 Git 和交付回复为准。完成后停止等待验收，不进入下一阶段。
