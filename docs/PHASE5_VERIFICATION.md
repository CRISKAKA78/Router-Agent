# Phase 5 HTTP / WebSocket 验证记录

日期：2026-09-06。实现起点 `b29aa46c81bea7f3b8f4780e9d657c1bba078897`；启动时fetch后HEAD/main/origin/main一致且工作区干净。用户确认Phase 4验收并明确授权Phase 5，设计为Accepted ADR-023；全部规定验证通过，等待用户验收。正式外部契约见API.md。Probe代码与Phase 4数据面未变。

## 验收映射

| 要求 | 实际证据 |
| --- | --- |
| 在线/离线Device、当前/历史Session、未知ID、分页过滤 | TestHTTPDeviceSessionsMaintenanceAndCancellation；TestPhase5RealProbeHTTPWebSocketWorkflow；原Phase2 Device/Session全量回归 |
| Session replacement保持online、旧Session进入历史、旧Maintenance释放 | HTTP测试同时注册两个相同device_id连接，检查新session_id、LastOfflineAt=null和旧维护Released；真实Probe重启查询新Session及旧维护关闭 |
| exec创建、202、状态/结果、同身份重发 | TestPhase5RealProbeHTTPWebSocketWorkflow通过HTTP运行真实命令；同HTTP键返回原task_id；resend后文件marker仍只有一个x |
| 不确定派发保留身份及隐藏内部诊断 | TestDispatchUncertainRetainsIdentityWithoutPrivateDiagnostics检查202/Location/dispatch_uncertain；Phase1 Gateway真实不确定派发回归保留原行为 |
| 原始资产导入/读取、Range、SHA/长度、稳定身份 | TestHTTPIdempotencyConcurrencyValidationAndRepository验证二进制导入与重放、原字节、206 Range及416 JSON错误；真实Probe用240000-byte资产闭环；原Repository完整性/损坏/重启回归 |
| 文件upload/download、Operation/transfer、下载导入与清理 | TestPhase5RealProbeHTTPWebSocketWorkflow逐字节比较上传/下载、通过HTTP查询Committed/Released、显式complete/重复complete保持asset_id、cleanup；Phase1/3确认丢失与提交事实分离全量回归 |
| Tool/版本/Artifact/兼容/投放/归档 | HTTP单元集成验证12个同键并发创建只产生一个Tool、发布原身份/无效规格/引用冲突/归档；真实Probe兼容检查和显式Artifact投放后0755权限正确；原Phase3 unknown/incompatible/ambiguous与Session准入回归 |
| Maintenance默认、自定义长/短租期、查询/列表/关闭/到期 | HTTP测试默认精确240分钟、40ms到期、366天自定义租期成功、显式零/超出time.Duration表达范围拒绝，重复/未知关闭沿用Service幂等；原Phase4全量回归 |
| Maintenance真实临时入口、数据面隔离 | 真实Probe经API返回的Web入口读取设备HTTP响应；Phase4真实HTTP/SSH/Telnet、并发流、half-close、背压、控制/文件流隔离全量测试保持 |
| HTTP取消不会撤销成功创建对象 | TestHTTPCancelAfterAdmissionRetainsCreatedObjectAndReplay在Service完成创建后、HTTP响应前设置屏障，取消客户端，再继续handler；Maintenance仍ready，同键返回原ID；普通创建响应后取消同样不撤销 |
| HTTP并发不重写Service状态 | 同键12并发Tool创建、异键冲突、Session替换/维护Close；所有原Service并发测试及Go race全量通过 |
| JSON/错误/容量 | duplicate/null/非object/尾随JSON/未知字段/非法Unicode；非法分页/state；404/405/409；Idempotency冲突与满额503、请求准入503、不同Origin403；Range错误仍使用统一JSON |
| WebSocket多客户端/断线/限额/慢消费者 | TestWebSocketClientsSlowConsumerAndShutdown：两个真实WebSocket同时收到设备变化、第三客户端503、断线释放槽位；使用实际Server端socket和固定队列饱和触发同一非阻塞关闭路径，远端连接终止 |
| WebSocket Server shutdown及四类事件 | 上述测试重连后Close在1秒测试期限内join并使远端读取失败；真实Probe测试收到devices/tasks/files/maintenance四类状态通知 |
| 有界连接/队列/内存及关闭 | TestAPIAdmissionAndKeyCapacity、TestBoundedListenerAndIdleShutdown；原生Serve默认128同时TCP、32请求、64WebSocket、每客户端8槽、4096幂等记录；超额拒绝，不开无界worker/事件历史 |
| Windows/Linux Server实际适用性 | Windows原生源码测试/vet/build及启动exe后GET /api/v1/devices返回空列表；Linux全量构建/真实Probe/HTTP/WebSocket与后述进程SIGTERM检查 |

资源结论限于新增Adapter：原Device Inventory、Task/File/Operation历史及Repository目录/磁盘保留边界沿用ADR-018/019和此前阶段，不声明整个Server RSS为固定值。原Service内部或Probe控制端接入不受HTTP幂等账本的创建次数限制。WebSocket是合并失效通知，没有逐状态或重放保证；慢消费者测试使用确定队列饱和屏障，不把不稳定OS发送缓冲大小当断言。

## 环境与结果

Windows amd64 Go1.25.5；Linux x86_64 WSL隔离镜像（Alpine、GCC15.2.0、CMake4.2.3、Go1.26.3）。Linux工作目录/work，独立network/mount namespace、loopback及devpts，测试OpenSSH与busybox-extras仅为验收工具。源码来自当前工作区，不复用旧Probe测试结论。

| 验证 | 结果 |
| --- | --- |
| Linux Release C++11构建/CTest | 通过4/4，最终4.74秒 |
| Linux全量Go与真实Release Probe Phase1～5 | 最终全量通过，集成170.595秒；首轮170.646秒 |
| Go race + TSan真实Probe Phase1～5 | 全包通过，集成177.736秒，无race/TSan报告 |
| TSan CTest | 4/4通过，7.12秒 |
| ASan+UBSan+LSan CTest，detect_leaks=1 | 4/4通过，5.97秒，无报告 |
| ASan真实Probe Phase4与Phase5 HTTP工作流 | 通过，集成15.608秒，无ASan/UBSan报告 |
| Windows原生全部源码包测试 | 通过，API0.338秒；真实Linux Probe集成在Windows跳过，在Linux实际运行 |
| Windows vet / Server构建 / exe HTTP启动检查 | 均通过；GET /api/v1/devices返回200、items=[]、total=0 |
| Linux vet / Server构建 | 最终通过；Linux实际Server启动GET设备清单成功，SIGTERM正常exit=0 |
| 最终HTTP状态校验/Range错误映射的定向race | 通过，internal/api 1.632秒（覆盖最后HTTP校验与错误格式修正） |
| git diff --check | 通过 |

真实Probe由测试清理终止；LSan正常退出检查由CTest覆盖，不把强制终止Probe本身作为无泄漏证明。Phase4已有fd/thread回落与socket终止验证在本次全量回归实际执行。HTTP关闭join及连接槽位回收由Phase5测试验证。短租期/超时测试使用显式测试配置，240分钟/366天使用创建时差与期限断言，不宣称等待了这些真实时长。

## 复现命令

Linux独立网络/PTY测试环境，在仓库根执行；80/22/23须属于本测试命名空间，不能对已有生产服务运行。固定Go依赖由go.mod/go.sum解析；无外网隔离环境可先在有网环境下载并共享经过Go校验的module cache。

```sh
/bin/sh tests/verify-phase5.sh release
/bin/sh tests/verify-phase5.sh asan
/bin/sh tests/verify-phase5.sh race
```

脚本使用显式 `./cmd/... ./internal/... ./tests/...` 覆盖全部源码包；当前本地忽略的build目录含旧Go模块缓存，不能用 `go test ./...` 把缓存当项目源码。脚本为本阶段新建C++ Release/ASan/TSan构建目录，不以旧构建产物代替验收。

Windows：

```powershell
go test ./cmd/... ./internal/... ./tests/... -count=1
go vet ./cmd/... ./internal/... ./tests/...
go build -o build/server/router-server-phase5.exe ./cmd/server
```

实际进程检查：以独立临时Repository启动已构建Server，控制/data使用127.0.0.1:0、HTTP使用测试端口，GET /api/v1/devices返回200及空列表。Windows终止测试进程并等待退出；Linux发送SIGTERM并wait得到exit=0。

Windows未配置适用cgo编译器，race在Linux执行。执行中首次Linux下载依赖因隔离网络不可达失败，已通过预下载固定v1.5.3模块缓存解决；一次go mod tidy被本地旧build模块缓存干扰，未据此宣称通过，最终依赖清单仅包含直接使用的gorilla/websocket及对应go.sum。最终 `go mod verify` 返回all modules verified。上述失败尝试不计入通过结果。

## 交付与边界

最终独立提交标题：`feat: complete Phase 5 HTTP and WebSocket API`。真实SHA与GitHub main推送状态以最终Git记录为准，不在待提交文档伪造自身SHA。提交推送完成后停止等待用户Phase5验收，不进入Phase6。

未实现：内置认证/TLS/RBAC/完整审计、跨Server重启HTTP/Task幂等恢复、事件持久化/补发、实时stdout流、UI/MCP/AI、通用端口转发或新Tunnel数据面。远程部署需要受保护管理边界；仅Repository持久化，归档不物理GC。嵌入式mipsel/ARM/ARM64、uClibc/老内核实机矩阵未执行，不从Linux x86_64测试推断实机已验收。
