# 项目接管手册

Phase 0～5已验收。Phase 6 Windows UI从main稳定基线 `57c2b1f8f6da1069e4a2eb988224b94bafe9cf84` 开始，首版提交为f8d099d6；本轮WinUI 3重构从该稳定提交开始，采用Accepted ADR-025；实现及规定验证通过，独立commit推送main后等待用户验收。当前事实见[PROJECT_STATUS](PROJECT_STATUS.md)和[PHASE6_VERIFICATION](PHASE6_VERIFICATION.md)，不得自行进入Web/微信/MCP/AI。

## 接管顺序

依次阅读AGENTS、本文件、PROJECT_STATUS、ARCHITECTURE、ROADMAP及相关API/PROTOCOL/DECISIONS，再核对实际代码、Git、构建与测试。不以历史聊天代替事实，Accepted ADR变更须新增superseding ADR。

## 当前入口

- Windows使用、部署与构建：[windows/README](../windows/README.md)和[windows/build.ps1](../windows/build.ps1)。设计：[PHASE6_DESIGN](PHASE6_DESIGN.md)、ADR-025。最终验证：[PHASE6_VERIFICATION](PHASE6_VERIFICATION.md)。
- `windows/RouterWorkbench`：原生WinUI 3中文Fluent工作台，设备侧栏和首要维护卡片，支持浅色/深色/系统主题；`RouterWorkbench.Core`：公开HTTP DTO、ApiClient、WorkspaceConnection、配置与外部入口启动。无Server内部引用；测试专用Protocol v1对端源码只在Tests目录，WinUI验证构建按条件链接，正式产物不包含。
- UI选择设备后默认维护页创建Maintenance，新项自动选中；默认240分钟、自定义正租期，显示三个入口、剩余时间和Server状态。Web使用系统浏览器，SSH/Telnet支持系统客户端或已有PuTTY。
- 所有网络异步；WS仅作失效通知，首连/重连重新HTTP同步，容量1通知合并及5秒恢复刷新。切换/退出取消并await旧连接、请求和worker；UI按连接/选择版本隔离迟到返回，任务/版本/兼容详情查询串行合并。
- POST/PUT使用固定请求字节和幂等键。响应不确定保留原请求，确认Server未重启后才显式重试；不能自动新建替代Task。下载Committed/Released和最终RESULT独立呈现，显式complete保留同一asset_id。
- Server正式契约：[API](API.md)、ADR-023、[PHASE5_DESIGN](PHASE5_DESIGN.md)。`cmd/server`默认HTTP127.0.0.1:8080、控制TCP :9000；HTTP仅可信本机/受保护管理网络，远程认证/TLS由部署层负责。
- `internal/api`复用management/Application和原Service；Phase 6没有改Server/Probe生产代码、协议或Tunnel数据面。Maintenance继续执行ADR-021/022，Repository继续执行ADR-019。

## 保持的运行边界

HTTP幂等账本默认4096项、不淘汰、满后拒绝新键；Server重启清空。WebSocket不提供事件重放/审计或实时stdout流。只有Repository元数据/字节持久化；Device/Session/Task/File/Operation/Maintenance均为进程内。Repository默认 `./data/repository`，单写者，停服整体备份，归档不物理GC。

Maintenance固定Probe127.0.0.1:80/22/23，默认240分钟、自定义正整数ms；关闭/到期/Session替换或断线撤销三入口与流。Released后端口默认隔离24小时，ReusableAfter只是最早复用，超窗/Server重启无永久旧地址隔离保证。配置和详细语义见API/ADR-022。UI倒计时只是本机显示，不能替代Server状态。

Windows设置保存到 `%LOCALAPPDATA%\RouterWorkbench\profile.json`，仅地址/外观/客户端路径/用户名；未来认证设计从该配置及集中网络层扩展。外部窗口由用户拥有；工作台退出不杀这些程序、不主动撤销Server已有任务或维护。无密码或Tunnel内部token持久化。

## 验证与历史

Windows `./windows/build.ps1 -Verify`执行产品构建、真实API和原生控件测试并发布；需要.NET10 SDK及Go。Phase 1～5Linux全量回归继续用 `tests/verify-phase5.sh release|asan|race`；真实服务测试须隔离网络和devpts，不占真实80/22/23。详见PHASE6_VERIFICATION中的结果与命令。

- 本轮WinUI重构起点/首版Windows UI：`f8d099d6bb6ac4830199b122755cbf37f6a9e849`。
- Phase 5稳定提交：`57c2b1f8f6da1069e4a2eb988224b94bafe9cf84`。
- Phase 4稳定提交：`b29aa46c81bea7f3b8f4780e9d657c1bba078897`。
- Phase 3：`ceaaab791850746911f965167c885789444efd4c`。
- 原baseline：`bc8d747dfc41a375c31698073005857c238ede51`。
- 历史验证：PHASE1_VERIFICATION～PHASE5_VERIFICATION。FRP放弃方向仅为ADR-020历史摘要，不作为当前构建输入。

下一步仅等待Phase 6 Windows UI验收。Web/微信技术栈与实施仍TBD。
