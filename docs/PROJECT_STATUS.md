# 项目状态

最后更新：2026-09-06。

## 当前阶段

Phase 0～5 已验收。Phase 6 Windows 客户端从首版稳定提交 `f8d099d6bb6ac4830199b122755cbf37f6a9e849` 正式重构为 C# + WinUI 3，采用 ADR-025，等待用户验收。独立重构 commit 推送 main 后停止，实际 SHA 以 Git 为准。

## 当前能力

- 中文 Fluent 工作台：设备侧栏、概览中的首要维护卡片、命令／任务、文件、工具／版本，浅色／深色／系统主题，Mica、原生导航、提示条和弹窗；不使用 WinForms 或 DataGrid。
- 连接地址与外部客户端进入设置，维护／会话编号、释放事实和关闭原因进入详情；日常路径为选择设备 → 开启维护 → 打开 Web／SSH／Telnet。
- 默认 240 分钟、自定义正租期、维护历史和关闭；设备／会话实时变化、命令结果；文件导入／另存／上传／下载／显式导入／清理／归档；工具创建／版本／兼容／投放与归档。
- 复用 Core API／WebSocket／DTO／外部启动逻辑；页面切换不重复订阅，断线重新 HTTP 全快照，旧连接及迟到响应隔离，退出取消并等待资源释放。
- 写入单次准入、同操作防抖、响应不确定保留原请求与键，明确核对后重试；不复制业务状态机、不展示隧道私有字段。
- 仅调用 Phase 5 公开 API，Server／Probe／Go 依赖／Tunnel 生产代码和业务语义无变化。

## 验证

详见 [PHASE6_VERIFICATION](PHASE6_VERIFICATION.md)。

| 项目 | 结果 |
| --- | --- |
| WinUI／Core Release 构建和自包含目录发布 | 0 警告／0 错误 |
| Core 真实 HTTP／WebSocket／故障测试 | 61 项断言 |
| 原生 WinUI 控件、弹窗、文件选择器与生命周期 | 98 项断言 |
| 正式发布程序独立启动／退出 | 原生窗口、本目录 WinUI 运行库、WM_CLOSE exit=0；包含 XBF／PRI 发布检查 |
| 浅色／深色、窗口缩放、实际 XAML 200% | 通过，含截图核对；物理多显示器矩阵未执行 |
| Linux Release C++ CTest | 4/4，4.72 秒 |
| Phase 1～5 Go／真实 Probe／Tunnel | 全包通过，集成 169.963 秒 |
| Go race + TSan 真实 Probe | 全包通过，集成 177.456 秒，无报告 |
| ASan／UBSan／LSan、TSan CTest | 各 4/4，5.95／6.92 秒，无报告 |
| ASan 真实 Probe Phase 4／5 | 通过，15.463 秒 |
| Windows Go test／vet／build、Linux vet／build | 通过 |
| Linux HTTP 启动／SIGTERM、模块校验、差异检查 | 通过 |

## 部署与边界

[Windows 使用和构建](../windows/README.md)。.NET 10 LTS，WinUI 模块化依赖；Windows x64 自包含目录携带 .NET 和所需 Windows App SDK 文件，目标机需 Visual C++ x64 运行库。必须保留整个目录，替代旧单文件包。WinUI 的传递 WebView2 接口依赖不代表应用使用网页 UI。

认证／TLS／RBAC 等仍属部署和后续设计。配置只保存非敏感连接与外观偏好。WebSocket 无重放或实时 stdout，服务器重启后任务／设备／维护／幂等清空；只有 Repository 持久化。维护固定探针 127.0.0.1:80/22/23，默认端口隔离 24 小时，不增加通用转发。

Windows 10 全版本／ARM64／x86、多物理显示器及 Probe mipsel／ARM／ARM64、uClibc／老内核实机矩阵未执行。外部登录、主机密钥和厂商网页需现场验收。

## 下一步

仅等待本轮 WinUI 3 界面验收，不进入 Web、微信、MCP 或 AI。
