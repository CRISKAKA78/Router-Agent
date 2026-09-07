# Windows UI Freeze 与正式接入

2026-09-07 用户已明确确认当前 React 实际页面的视觉设计完成，进入 UI Freeze + Production Integration。此指令取代此前继续视觉设计、只做 Mock 的工作阶段；不主动改布局、配色、字体、圆角、密度或视觉语言。

后续工作方向按 ADR-028 持续完善当前产品。本文件继续约束视觉，普通功能可按下述规则补充必要业务交互；历史接入完成/等待验收不阻止已授权的范围内改进。改变冻结布局或视觉语言仍须明确确认，不自动进入新阶段。

## 冻结基线

仓库：Router-Agent（origin: https://github.com/CRISKAKA78/Router-Agent.git），现有已提交业务基线 `404b083`。冻结对象是当前工作区 `frontend/src/preview/` 中的实际页面及 `frontend/preview.html`，不是此前旧生产 App 的样式。

| 页面 | 冻结源码 |
| --- | --- |
| 外壳、导航、设备列表/Header | WorkbenchPreview.tsx、preview.css |
| 设备概览 | OverviewPreview.tsx、overview.css、assets/router-device.png |
| 远程维护 | MaintenancePreview.tsx、maintenance.css |
| 任务中心 | TasksPreview.tsx、tasks.css |
| 文件管理、工具仓库 | RepositoryPreview.tsx、repository.css |

这些文件保留为只读视觉参照；生产实现复用已确认结构、样式与资产，改动仅为真实数据绑定、状态/错误/空态、可访问性及必要业务表单。缺少数据来源的字段保留位置并显示未提供，不虚构在线、容量、CPU、4G、接口或任务结果。新增功能不得以重设计主页面作为前置条件。

## 接入规则与状态

- [x] 用户确认视觉冻结；仓库与冻结源码已核对。
- [x] 正式入口使用冻结 UI，与 WinUI/WebView2 发布链路连接。
- [x] 设备/Session、连接配置、HTTP/WS 恢复与生命周期接入。
- [x] Maintenance 默认/自定义租期、创建/关闭/到期与入口接入。
- [x] 任务列表/详情/最终输出/原身份重发接入。
- [x] 文件资产导入/保存/上传/下载/显式入库/归档/暂存清理接入。
- [x] 工具/版本/产物、服务端兼容判断、归档与投放接入。
- [x] 内置终端适配实现与验证；方案已按 ADR-027 确认。
- [x] 浏览器/真实 WebView2、业务集成、异常/并发/资源释放与构建验证。

复用 api.ts、connection.ts、useQuery.ts、useWorkbench.ts 与 platform.ts，保持单一 API Client、同源策略、原始幂等键/字节、HTTP 快照及取消释放机制。文件 committed/released 与 Task RESULT 分开；工具不自动执行、不猜 latest、兼容由服务端判定。Management Server、Probe、控制协议及 Tunnel 数据面不因视觉迁移重构。

## 已确认终端方案

用户明确要求默认内置 Shell，同时允许调用外部 Shell 工具；按 Accepted ADR-027 扩展平台适配职责。ConPTY 承载本机 SSH/Telnet 命令行客户端，React xterm.js 呈现；保留外部客户端入口。不改写 ADR-026 历史内容。

远程目录采用有界单次只读 Exec；文件内容仍通过现有 File API。设备遥测未提供的字段保留位置并标识未提供。

实现及本机/隔离环境验证已通过，详细证据和范围见 PHASE6_VERIFICATION。等待用户验收；不以测试对端或隔离服务替代真实厂商路由器验收，不进入下一阶段或恢复视觉迭代。
