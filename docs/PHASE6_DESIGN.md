# Phase 6 Windows UI 设计

2026-09-06。重构起点 `f8d099d6bb6ac4830199b122755cbf37f6a9e849`，核对 HEAD/main/origin/main 一致、初始工作区干净。用户明确要求 C# + WinUI 3 取代首版 WinForms，依据 ADR-025；ADR-024 的网络、幂等和外部客户端边界保留。

## 已确认边界与 TBD

Phase 0～5 已验收。只使用 `/api/v1` 与 `/api/v1/events`，Server／Probe／Tunnel 生产代码及公开 API 无变化。不访问内部服务、数据库、Gateway 或数据面，不复制状态机。维护固定三入口、默认 240 分钟、正租期、会话绑定与端口隔离均由服务器处理，保留 ADR-009～023。

认证、TLS、RBAC、租户、完整审计及跨重启幂等仍 TBD。配置 schema_version 和集中网络层保留扩展位置，不构建假认证字段或账号体系。不进入 Web、微信、MCP、AI 或新 Tunnel。

## 技术和组织

- Core 保留 DTO、ApiClient、WorkspaceConnection、配置和外部入口启动；新增外观偏好与中文错误摘要，不改网络契约。
- WinUI XAML 负责控件和主题；WorkbenchViewModel 持有当前连接、服务器快照、选择、提示和交互准入；窗口负责调度、展示、弹窗与文件选择。无额外 MVVM 框架。
- 独立 Core 测试和仅在验证构建中链接的 WinUI 控件测试；测试协议对端不进入正式程序。
- .NET 10 LTS；Windows App SDK 2.4 稳定发行对应的 WinUI 2.3.6、InteractiveExperiences 2.1.6，BuildTools 10.0.26100.9169。模块化引用避免无关 AI／ML／Widgets；WinUI 自身传递依赖 WebView2 接口，客户端不使用 WebView。
- Windows x64、非 MSIX、自包含目录部署，包含 .NET 与所需 Windows App SDK 文件，目标机需要 Visual C++ x64 运行库。取代旧单文件部署选择。

## 信息层级

左侧设备搜索与列表；右侧设备标题及概览、命令与任务、文件、工具与版本。使用 NavigationView、Mica、主题卡片、Fluent 字体图标、InfoBar、ContentDialog。浅色／深色／系统主题，侧栏随有效宽度收缩，内容垂直滚动，最小窗口尺寸按 DPI 换算，不使用 DataGrid。

维护卡片优先展示开启状态、剩余时间、三个地址和直接操作。默认省略 lease_ms，自定义分钟转为 API 范围内的正整数毫秒。服务器地址在设置；维护记录、编号、会话、释放事实和原因在详情中。可翻译状态和操作使用中文，专业名称、命令、架构值和原始诊断字段保留原值。

## 生命周期、并发和事实来源

每个 WorkspaceConnection 独立拥有 HttpClient、ClientWebSocket、取消源、单快照 worker、容量 1 刷新通道、5 秒恢复计时器和在途网络操作。首连／重连收到 resync_required 后获取 HTTP 全量分页快照，退避 1/2/4/8/16/30 秒。查询中的通知排入下一次回查；连接代次不符的快照不发布，不依赖重放。

窗口只有一个协调器，页面切换不订阅网络。DispatcherQueue 应用变化；任务、版本、兼容查询各自合并，迟到响应校验连接和选择。切换先清空显示并 await 旧连接释放。AppWindow.Closing 取消首次关闭，取消弹窗、文件选择、文件准备和网络，等待协调器及详情查询后关闭。外部程序归用户所有。

交互准入覆盖弹窗与文件选择器，在途时禁用重复写入，同一操作 400ms 防抖，不阻挡已完成后的不同操作。幂等和待确认请求复用 Core；响应不确定时暂停新写入，明确处理原请求，不自动重发或创建替代任务。

设备、任务、维护、文件提交和兼容性均来自 API。倒计时只是 expires_at 的本机显示估计，过期回查，不伪造 closed。打开入口前 GET 维护并确认服务器状态和当前会话。Web 使用默认浏览器；SSH／Telnet 保持系统／PuTTY 和 ArgumentList 参数策略，不保存密码、不自研协议、不关闭主机密钥验证。

## 验证与停止点

见 PHASE6_VERIFICATION、windows/README 和 windows/build.ps1。保留 Phase 1～5 全量 Go／真实 Linux Probe／Tunnel、race／vet、C++ CTest／sanitizers 与 Windows／Linux 构建。独立 WinUI 重构提交推送 main 后停止等待验收。
