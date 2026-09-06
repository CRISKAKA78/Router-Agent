# 项目状态

最后更新时间：2026-09-06

## 当前阶段

Phase 0～4已验收。Phase 5实现及全部规定自动化验证通过，等待用户验收；采用Accepted ADR-023，启动基线为 `b29aa46c81bea7f3b8f4780e9d657c1bba078897`，fetch后HEAD/main/origin/main一致、工作区干净。独立Phase 5提交推送main后停止等待验收，不进入Phase 6。提交与推送事实以Git记录为准。

## 当前可验证能力

- `/api/v1` HTTP API覆盖在线/离线Device及Session、exec/任务/结果/重发、资产导入读取/归档、上传/下载/完成导入/清理、Tool/版本/Artifact/兼容与投放、Maintenance创建/列表/查询/主动关闭和三入口。
- API只调用Management/Application和现有Service；不重写状态机。非空task_id和不确定派发保留；HTTP创建取消不撤销已成功业务对象，下载本地提交事实不伪装为最终成功。
- 统一JSON/错误、分页/过滤、异步202、输入上限和进程内有界Idempotency-Key。HTTP DTO隐藏内部本地路径、传输关联及Tunnel token/connection_id/data地址。
- WebSocket提供四类资源刷新通知，首次/重连要求HTTP同步；Device/Task/File/Repository变更计数与Maintenance有界快照驱动。多个客户端、固定队列、慢消费者断开和Server关闭join。
- 默认HTTP127.0.0.1:8080，32并行请求、64 WebSocket、128 TCP连接、64KiB JSON、1GiB原始资产流、4096幂等记录。原Service历史政策保持，API新增创建/重发受账本限额控制。
- Phase 4固定三入口、240分钟默认/正自定义租约、Session绑定、独立data TCP、half-close、背压、默认24小时端口隔离及独立撤销保持；Probe代码无变更。Phase 1～3任务/文件/Repository语义保持。

## 验证

详细命令、平台与测试映射见[PHASE5_VERIFICATION](PHASE5_VERIFICATION.md)。

| 验证 | 最终结果 |
| --- | --- |
| Linux Release C++11 / CTest | 通过4/4，4.74秒 |
| Linux Phase 1～5全部Go/真实Probe/HTTP-WebSocket | 全包通过，集成170.595秒 |
| Go race + TSan真实Probe全量回归 | 全包通过，集成177.736秒，无报告 |
| C++ TSan / ASan+UBSan+LSan CTest | 各4/4，7.12秒 / 5.97秒 |
| ASan真实Probe Phase 4/5 | 通过，15.608秒 |
| 最后HTTP校验/错误格式定向race | 通过，1.632秒 |
| Windows原生测试/vet/build/实际HTTP启动 | 全部通过；Linux Probe用例在Linux实际执行 |
| Linux vet/build/实际HTTP启动/SIGTERM | 全部通过，进程正常exit=0 |
| go mod verify / git diff --check | 通过 |


## 已知边界

- HTTP仅用于可信本机/受保护管理网络；无内置认证、TLS、RBAC、租户或完整审计。远程部署须由部署层提供相应边界。Origin校验和Tunnel配对token都不是用户认证。
- HTTP幂等账本默认4096、可配置、不淘汰，满后拒绝新键；同进程保留原响应，重启清空。WebSocket合并状态失效提示，没有历史重放或逐状态交付；客户端回查HTTP。没有实时stdout/stderr流。
- 仅Repository元数据/字节持久化；Device/Task/File/Operation/Maintenance重启不恢复。Repository单写者、本地JSON目录、归档不回收空间，没有在线备份、迁移或物理GC。原Device Inventory及任务记录的保留边界不变。
- Maintenance仅固定80/22/23，无任意端口、续租、UDP/SOCKS/VPN/P2P、HTTP改写或新数据面。ready表示入口已监听；Released是Server本地释放，端口隔离不保证超窗/重启后的永久旧地址隔离。
- Probe仍以Linux x86_64验证；mipsel/ARM/ARM64、uClibc/老内核实机矩阵未覆盖。
- 不提供UI、MCP、AI Agent、通用端口转发或大型权限体系。

## 下一步

本轮规定验证已通过；独立提交标题 `feat: complete Phase 5 HTTP and WebSocket API`，提交SHA及main推送状态以Git记录为准。推送后等待Phase 5验收。不得自行进入Phase 6。
