# Phase 5 HTTP / WebSocket 设计

授权基线：2026-09-06 用户确认 Phase 4 验收并授权实现 Phase 5；main / origin/main 为 `b29aa46c81bea7f3b8f4780e9d657c1bba078897`，工作区干净。

已确认约束：API First、复用 Service；保留 ADR-009～022、固定三入口与默认240分钟租约、自定义正租约、Session绑定、文件提交与最终结果分离、稳定Repository身份。HTTP不访问连接表或存储表，不修改Tunnel数据面。

ADR-023 补充此前未决的外部资源/事件契约。认证、TLS、RBAC、完整审计、跨进程幂等恢复、历史清理仍为 TBD；本阶段采用独立loopback HTTP监听及明确可信部署边界。

实现范围：devices及sessions；tasks（exec、结果、原身份重发）；assets原始流导入/读取/归档；tools/versions/artifacts/compatibility；uploads/downloads/deployments及Operation/transfer查询、下载完成导入/清理；maintenance创建/查询/列表/关闭/入口；WebSocket状态失效通知。

所有业务状态由既有Service提供。设备/任务/文件/Repository Service只新增变更计数与任务列表查询；实时层以250ms采样计数、Maintenance有界快照比较，合并通知四类资源，不保存业务镜像或事件历史。客户端连接后及重连后重新查询HTTP；不承诺每个中间状态或事件重放。每客户端固定队列和写期限，满队列关闭，Server关闭等待reader/writer退出。

API创建使用有界进程内Idempotency-Key账本，不淘汰已接受键，容量满拒绝新建。相同键/请求返回原响应，冲突409。键保护HTTP重试；task_id继续保护设备重发，两者不混淆。JSON创建在验证并准入后使用API生命周期context；HTTP取消不撤销已创建业务对象。原始文件导入保持流式及请求取消语义，以声明SHA-256/长度参与幂等签名并完整校验；同键返回原资产，新键保留独立资产身份。

验证事实写入 PHASE5_VERIFICATION.md；全部通过后独立commit推送main并停止等待验收，不进入Phase 6。
