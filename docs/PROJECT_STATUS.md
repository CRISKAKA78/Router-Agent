# 项目状态

最后更新：2026-09-07。

当前阶段：**UI Freeze + Production Integration**。用户已确认现有 React 实际页面视觉，禁止主动重新设计布局或视觉语言；默认内置 Shell，允许外部客户端，按 Accepted ADR-027 实施。视觉基线与接入清单见 [UI_FREEZE](UI_FREEZE.md)。此前 Mock 阶段已经结束。

- 冻结源码保留在 `frontend/src/preview/`；正式 `index.html` 已切换到复用该布局的 `frontend/src/ui/` 页面。
- 设备、任务、文件、工具页面已接入既有 TypeScript API/WS 层；不使用 Mock 回退，未提供遥测明确显示未提供。下载提交/释放与任务最终 RESULT 独立显示，工具分类由真实 Operation 关联，不把 202 当成功。
- ConPTY 平台适配和 xterm.js 渲染默认内置 SSH/Telnet，保留外部入口。原生策略、中文 I/O、尺寸、自然退出尾部输出、积压关闭与独立租期释放通过；隔离 OpenSSH 的真实登录/命令/resize 和 BusyBox Telnet 命令往返通过。
- 前端构建及 13 项测试、27 项原生策略/终端/文件检查、31 项真实 WebView2 集成、活动内置进程 WM_CLOSE 回收、正式发布启动/退出通过。Linux Release/ASan+UBSan/race+TSan 与 Windows Go test/vet/build 通过，证据见 PHASE6_VERIFICATION。
- 本机 Vite 5173 保持运行；`/preview.html` 为冻结参照，`/` 为正式数据入口。首次使用在设置中连接服务器。

Phase 0～5 已验收。当前 Phase 6 从 `6f0ce71a5027b57d51e9a6c807794be45f7633b5` 迁移为 **C# / WinUI 3 Thin Shell + WebView2 + React / TypeScript Shared Frontend**，采用 Accepted ADR-026。

**React Shared Frontend 是今后 Windows 与 Web 的统一产品 UI 基线。** 旧 XAML 业务页和 C# 业务网络层已删除。后端、Probe、API 与 Tunnel 生产代码未修改。

既有业务层包括设备/Session、首要 Maintenance 卡片及三入口、默认与自定义正租期、Exec/Task、文件、工具/版本/产物/兼容/投放、中文浅色/深色/系统主题、幂等原请求与快照恢复。Windows Shell 只负责本地内容、窗口、配置和受限平台操作。

上一提交 `404b083` 的构建、完整发布目录、测试命令和实际证据见 [Windows README](../windows/README.md)、[PHASE6_VERIFICATION](PHASE6_VERIFICATION.md)。React production build、9 项前端测试、21 项原生策略/文件检查、26 项真实 WebView2 集成检查、Windows Release 与 Phase 1～5 Linux Release/ASan/race、Windows Go test/vet/build 全部通过。正式发布目录在本机验证启动与正常退出。

干净 Windows Sandbox 已完成无 Node/dotnet 检查和离线运行库准备，但 Application Control 拒绝启动未签名 EXE。用户随后明确要求“直接在本机测试”，本轮按该范围完成验收；不宣称干净目标机运行已经通过。

仅 Repository 持久化；其余 Session/Task/维护/幂等账本不跨 Server 重启。维护固定 Probe 127.0.0.1:80/22/23、默认端口隔离 24 小时。正式公网 Web 部署、认证体系、微信、MCP、AI 与新 Tunnel 未实施。

交付：本轮 UI Freeze + Production Integration 独立提交以 Git 为准；发布目录 `build/windows-react/win-x64/`。保留开发服务器，停止等待用户验收，不进入下一阶段。真实 WebView2 使用真实 Go API 与 Protocol 测试对端，ConPTY 实际登录及 Linux Probe/Tunnel 各有独立集成证据；不宣称已完成用户实机路由器登录或厂商网页验收。
