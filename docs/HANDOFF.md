# 项目接管手册

Phase 0～4已验收。Phase 5从main稳定基线 `b29aa46c81bea7f3b8f4780e9d657c1bba078897` 继续，用户明确授权HTTP/WebSocket API。实现及全部规定自动化验证已通过，等待用户验收；事实以[PROJECT_STATUS](PROJECT_STATUS.md)和[PHASE5_VERIFICATION](PHASE5_VERIFICATION.md)为准。交付独立Phase 5 commit并推送main后停止等待验收，不进入Phase 6。

## 接管顺序

依次阅读AGENTS、本文件、PROJECT_STATUS、ARCHITECTURE、ROADMAP及相关API/PROTOCOL/DECISIONS，随后核对代码、Git、构建与测试。不得依赖聊天或开发者本地临时保存项。已接受设计有变化须新增superseding ADR，不能静默改写历史决定。

## 当前入口

- 正式HTTP/WebSocket契约：[API.md](API.md)。设计理由：[ADR-023](DECISIONS.md#adr-023-phase-5-http--websocket-adapter)、[PHASE5_DESIGN](PHASE5_DESIGN.md)。
- `cmd/server`使用`api.Run`同时组合HTTP listener与原Management控制入口；默认HTTP127.0.0.1:8080，控制TCP :9000，Maintenance data/入口仍按Phase4 flags。API仅供可信本机/受保护管理网络，远程认证/TLS由部署层负责。
- `internal/api`只做路由、DTO、校验、分页、错误、HTTP幂等账本与WebSocket通知。业务调用management.Server及已有Service，不访问连接表/存储表，不重写业务状态机。
- `internal/management`拥有兼容投放、下载导入、Operation编排和API所需exec/设备断开/任务摘要查询入口。`internal/device`、`task`、`filetransfer`、`repository`继续拥有各自事实。新增Revision只用于合并刷新；files事件也覆盖Repository目录提交。
- `internal/tunnel`、`internal/gateway/tunnel.go`、`probe/src/tunnel.cpp`继续执行ADR-021/022。HTTP直接调用Maintenance Service，不修改Probe或数据面，DTO不返回token/connection_id/data私有地址。

## 使用要点

所有POST/PUT带Idempotency-Key，默认4096项不淘汰账本，满后拒绝新键；同键同请求返回旧响应，冲突409。HTTP取消不撤销成功创建的任务/Maintenance。任务非空ID与dispatch_uncertain必须一起保存；不得自动创建替代任务。文件下载Committed+Released与最终RESULT分开呈现，显式complete导入后保留同一asset_id。

WebSocket `/api/v1/events`首次/重连resync_required后回查HTTP；resource_changed通知devices/tasks/files/maintenance刷新，可以合并、没有重放/审计语义。默认64客户端、8槽队列及写期限，慢客户端断开。原生API.Serve负责TCP限额/超时；嵌入仅ServeHTTP时外层Server负责自己的网络资源。

Repository默认`./data/repository`，本地单写者JSON+不可变blob，整体停服备份。归档不物理GC。仅Repository持久化；Device/Session/Task/File/Operation/Maintenance及HTTP账本重启清空。任务/设备原保留政策、Session历史默认64、Probe128身份/8MiB预算保持。

Maintenance默认240分钟，自定义正整数ms无产品租期上限（仅time.Duration表示范围）；固定Probe127.0.0.1:80/22/23。关闭/到期/Session替换或断线撤销三入口及活动流。端口Released后默认隔离24小时，ReusableAfter只是最早复用；超窗/Server重启不保证旧地址永久隔离。细节见[PHASE4_DESIGN](PHASE4_DESIGN.md)和ADR-022。

## 验证与历史

最终命令、结果和验收映射统一见[PHASE5_VERIFICATION](PHASE5_VERIFICATION.md)；Linux脚本[tests/verify-phase5.sh](../tests/verify-phase5.sh)。真实SSH/Telnet/HTTP回归要求隔离网络与devpts、OpenSSH/busybox-extras，不能占用真实服务80/22/23。Windows运行原生源码测试/vet/build；真实Linux Probe和race在Linux执行。

- 当前Phase 5起点/Phase 4稳定提交：`b29aa46c81bea7f3b8f4780e9d657c1bba078897`。
- Phase 4首版：`f92d73a0d6003835993967848f1f5fe009a0df89`。
- Phase 3：`ceaaab791850746911f965167c885789444efd4c`。
- 原baseline：`bc8d747dfc41a375c31698073005857c238ede51`。
- 历史验证：[Phase 1](PHASE1_VERIFICATION.md)、[Phase 2](PHASE2_VERIFICATION.md)、[Phase 3](PHASE3_VERIFICATION.md)、[Phase 4](PHASE4_VERIFICATION.md)。FRP放弃方向只保留ADR-020归档摘要。

当前阶段不包括Web/Windows/微信UI、MCP、AI Agent、通用转发、新数据面或大型用户权限架构。下一步仅等待Phase 5验收。
