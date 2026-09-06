# Phase 6 Windows UI 设计

日期：2026-09-06；基线 main `57c2b1f8f6da1069e4a2eb988224b94bafe9cf84`，fetch 后 HEAD/main/origin/main 一致，初始工作区干净。用户已验收 Phase 5 并授权自行选择 Windows 技术栈和现有能力的操作方式。本次实现仅 Windows 桌面工作台，接受依据见 ADR-024。

## 已确认约束与未决边界

保留 Accepted ADR-009～023，Server / Probe 无业务语义变更。客户端只通过 `/api/v1` 与 `/api/v1/events` 调用 Management Server；不引用 Go internal 包、不读取 Repository/数据库、不实现 Probe 或 Tunnel 协议。Maintenance 固定三入口、默认 240 分钟及正租期、Session 绑定、独立数据面与端口隔离继续由 Server 实现。API 的 `task_id + dispatch_uncertain`、下载 Committed/Released 与 RESULT 分离、稳定资产身份和版本/兼容语义保持。

认证、TLS、RBAC、租户、完整审计、跨重启任务/幂等恢复仍 TBD。配置记录 schema_version，网络请求集中在 ApiClient / WorkspaceConnection，未来可在此边界设计凭据提供者；当前不提供假认证字段或账号体系。AGENTS/HANDOFF/STATUS/ROADMAP 的旧“停止于 Phase 5、无 UI”属于阶段状态过期，按明确用户授权同步，不改写历史 ADR。

## 技术与模块

- `windows/RouterWorkbench.Core`：纯 .NET HTTP 客户端、只读 DTO、连接配置、连接/后台刷新生命周期、待确认请求和外部入口参数生成。不持有第二套业务状态机。
- `windows/RouterWorkbench`：Windows Forms 原生控件及异步事件处理，设备导航、Maintenance、Task/Exec、File 和 Tool/Version/Artifact 操作。只有这里依赖 WinForms；布局不使用 HTML 或 WebView。
- `windows/RouterWorkbench.Tests`：不进入产品的真实 API/受控故障/原生消息循环验证程序，测试对端的 Protocol v1 实现仅用于契约测试。
- .NET 10 LTS、C#，Windows x64 self-contained 单文件发布；不引入第三方 NuGet 包或前端构建链。提供带运行时和要求预装运行时的构建方式。

## 连接与刷新

每个 WorkspaceConnection 拥有一个 HttpClient、生命周期 CancellationToken、单一 WebSocket 接收循环、单一 HTTP 快照 worker、5 秒恢复计时器和所有在途读取/写入。WebSocket 使用框架 ClientWebSocket 的 ping/pong/close 处理，连接超时 10 秒、保活 15 秒、pong 超时 15 秒；首条必须为 resync_required。重连间隔 1/2/4/8/16/30 秒。

一次 HTTP 快照依次读取全部设备、任务摘要、资产、工具和 Maintenance，分页每页 200 项，10000 项阈值显式报错。通知合并到容量 1 的 channel；worker 查询期间发生的通知保持待刷新，保证查询后再次回查。首次/每次重连均从 HTTP 获取现状，不使用 sequence 重建业务，不依赖历史重放。只有对应连接代次的完整快照可发布；WS 断开或快照失败会标记不同步。maintenance_disabled 单独显示，不影响其它 API 页面使用。

UI 用 WinForms 消息线程更新控件。切换 Server 先撤当前引用、清理控件，再取消并 await 旧连接、请求和 worker；旧引用/选择版本的迟到结果不会刷新新页面。退出取消文件准备、网络及所有后台任务，异步等待后关闭表单。同步写操作单次准入；按钮还使用 Windows 双击时间防抖，避免快速成功响应后的第二次点击重复提交。不同 Server 的幂等请求和结果不复用。

任务详情、工具版本和兼容详情各自使用单个查询循环与一个待刷新标记；频繁通知合并，旧选择查询完成后再处理新选择，避免慢详情请求造成在途读取堆积。

## Maintenance 与外部客户端

“选设备 → 开启远程维护 → 三入口”，新 Maintenance 自动选中，旧历史仍可查看。默认请求省略 lease_ms，由 Server 应用 240 分钟；自定义分钟/毫秒换算后必须为 API 可表达的正整数毫秒。状态、Session、Released、入口全部使用 DTO。剩余时间是本机基于 expires_at 的显示估计；到期禁止打开旧入口并回查 Server，不在本地把状态改为 closed。每次打开另行 GET Maintenance 核对 Server 状态。

Web 仅把 API 入口 URL 交系统默认浏览器。SSH 默认 Windows OpenSSH，Telnet 默认 Windows Telnet Client；设置可选择用户已有 PuTTY.exe 及对应参数约定。程序路径须为本机绝对 exe 路径；主机、端口、用户名分别通过 ArgumentList 传递，禁止拼接 shell。用户名校验避免被当选项；主机/URL 验证防止入口数据变成启动选项。不自研 SSH/Telnet，不处理密码、不关闭主机密钥校验。外部窗口由用户拥有，工作台关闭不杀进程，Server 维护生命周期独立。

## 写入和错误

每个 POST/PUT 的原始 UTF-8 JSON 字节、方法、相对路径与随机幂等键绑定。文件导入流式计算 API 必需的 SHA-256 并携带长度；HTTP 发送原字节流，资产另存用临时文件并验证长度/摘要再替换。没有业务写入自动重试。网络/响应错误后保留请求，暂停其它写入；用户确认 Server 未重启后可重发同键同请求。收到 API 业务错误则展示 code/HTTP 状态并结束此次请求。非空 task_id 即保留并显示，dispatch_uncertain 不触发替代任务。

错误按操作与 code 映射，maintenance capacity_exhausted 明确可能是端口池或 Server 总容量，不能从相同 code 猜出唯一内部原因。Session 替换提示并重新拉取资料/兼容结果；客户端不计算 compatibility、不自动选择 latest、不把文件 Committed 当 Task success。窗口不缓存或展示 Tunnel token、connection_id 或 data listener。

## 验证与交付

验证映射及结果见 PHASE6_VERIFICATION；使用说明与可复现构建见 windows/README。保留 Phase 1～5 全量 Go、真实 Linux Probe、race/vet、C++ CTest/sanitizers 和 Windows/Linux Server 构建。本阶段独立 commit 推送 main 后停止等待验收，不进入 Web UI、微信小程序、MCP、AI 或新增数据面。
