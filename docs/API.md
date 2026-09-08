# Management Server API

ADR-035：Windows 调用方现为 `windows/RouterWorkbench.Client` 的原生 C# HTTP/WS Client；资源路径、请求与响应、幂等和业务状态均未变化。ADR-036 的 WPF 页面消费公开 DTO，维护仅打开外部客户端，主程序无内置终端；独立生成器继续使用 ADR-034 的 TemplatePublishingService。ADR-037 已移除旧 React/WinUI/Win32 UI；下文旧客户端描述仅保留为历史语境，当前 Windows 设计见 [WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md)。

本文件维护 `/api/v1` HTTP/WebSocket规范及原内部Service契约。实现入口 `internal/api`，业务来源为 `management.Server` 与 Service。ADR-029 新增服务端属性模板及注册快照字段；既有 Tunnel 数据面不变。

Phase 6 React Shared Frontend / Windows WebView2 Shell复用本文件公开契约（ADR-026/027），此前重构未补充生产 API；本轮模板扩展见下文。客户端首连/重连HTTP同步、维护与Exec/文件/工具动作、相同键显式重试及入口启动规则见[PHASE6_DESIGN](PHASE6_DESIGN.md)与[Windows使用说明](../windows/README.md)。Windows UI不能直接调用下文内部Go接口。

冻结 UI 的内置 Shell 通过本机 SSH/Telnet 客户端连接重新查询的 Maintenance 公共入口，不通过 HTTP Exec 或 WebSocket 传送持续终端字节。右侧目录使用有界单次 Exec，文件内容通过 uploads/downloads/complete；工具投放列表从 tasks/{id}/operation 的 tool_id 关联得出。当前无 CPU、内存、磁盘、4G、端口遥测 API，界面相应字段显示未提供，不以推测数据替代。

## 部署与生命周期

### 配置任务 API（ADR-031）

`POST /api/v1/devices/{id}/config-tasks` 创建配置任务，要求 Idempotency-Key；JSON 为 `{backend,operation,key?,value?,package?,timeout_seconds?}`。backend 为 nvram/uci，operation 为 get/set/delete/commit；字段组合与值限制见 [PROTOCOL 配置任务扩展](PROTOCOL.md#配置任务扩展adr-0312026-09-08)。timeout_seconds 省略为 5，显式必须为 1～30 整数，0/null 不接受。请求示例：

```json
{"backend":"uci","operation":"set","key":"system.@system[0].hostname","value":"router-one","timeout_seconds":5}
```

202 返回 `{task_id,dispatch_uncertain}`，Location 指向原 `/api/v1/tasks/{id}`；沿用原请求字节/幂等账本和显式重发。任务列表 type 为 router_config，详情 params 为不可变结构化参数，结果 stdout/stderr/exit_code 等同 Exec。该任务没有 transfer/operation 资产关联，客户端无需请求文件信息。

非法参数 400 invalid_request，未知设备 404 not_found，离线 409 device_offline，发送前 Session 改变 409 session_changed，当前 Probe 未声明 router_config 为 422 unsupported_capability。缺固件命令在 Probe 执行后以任务 failed 表达，不等同于 HTTP 不支持能力。不确定派发仍保留原任务；重发到旧 Probe 同样返回 unsupported_capability，不自动降级或新建任务。

写入和删除不提交持久存储；commit 是独立任务，uci 必须指定 package，nvram 提交整份 NVRAM。不会重启设备/服务或刷新设备注册属性。任务参数和结果按现有可信管理网络契约可查询，配置值不应视为秘密保险库。

### 属性模板 API（ADR-029）

ADR-034 将生成器调用方迁为 C# `TemplatePublishingService`，继续通过下述公开契约访问 Go Server；HTTP/TCP schema 保持。宿主地址与管理服务器地址独立配置，不新增 Go 代理或内部调用。

独立模板生成器（ADR-032）调用同一公开 API，主工作台设置不再提供模板管理。GET `/probe-templates` 返回通常的分页结果，GET `/probe-templates/{id}` 返回完整模板；实际路径均带 `/api/v1` 前缀。生成器的虚拟属性/公式属于本地工程，发布前编译为已有来源；HTTP schema 不新增虚拟或公式字段，服务器不保存工程源文件，见 [TEMPLATE_GENERATOR](TEMPLATE_GENERATOR.md)。

| 方法 | 路径 | 请求与结果 |
| --- | --- | --- |
| POST | /probe-templates | `{name,properties}` → 201 `{template_id,name,version,properties}`；version=1 |
| PUT | /probe-templates/{id} | `{name,properties,version}` → 200 新完整模板；version 必须匹配，更新递增 |
| DELETE | /probe-templates/{id} | `{version}` → 200 `{deleted:true}`；必须匹配当前版本 |

POST/PUT/DELETE 全部要求 Idempotency-Key，沿用原请求字节与账本规则；旧版本/名称冲突返回 409 conflict，非法输入 400 invalid_request，不存在或已删除 404 not_found，存储容量满 503 capacity_exhausted。删除的同键原字节重放仍返回原成功，不同键再次删除 404。

properties 为 `{属性key:{name,command,timeout_seconds}}`，兼容增加 `{属性key:{name,source:"nvram"|"uci",key,timeout_seconds}}`；缺 source 为 command，也可显式 command。配置来源只读且不带 command，命令来源不带 key。可选字段、范围、输出与启动语义见 PROTOCOL 的“启动属性模板”与 ADR-031 扩展。名称唯一，允许编辑；ID 不重用，删除保留身份墓碑，不改已有设备/Session 快照。最多 1000 个历史身份、目录文件最多 8 MiB；命令正文仅用于模板管理和准备连接，不出现在设备采集失败摘要。旧模板无迁移，新来源需更新 Probe；旧 Server 不能读取带新来源字段的目录。

默认文件为 `repository-dir/probe-templates/catalog.json`，属于独立 Template Service，不进入 Repository 的资产/工具元数据或 schema。可用 `-probe-template-file PATH` 指定。服务端持有独立文件锁、原子替换保存，重启保留；损坏/重复身份或名称/未知 schema/已初始化但目录丢失时启动失败，不静默重建。仅适用于现有可信管理网络，未新增身份认证。

设备与 Session 的 registration 兼容增加 `template`（无模板为 null）、`attributes` 和 `collection_errors`（旧客户端可忽略；旧 Probe 可为 null/空对象）。值及字段定义见 PROTOCOL；六项已有属性仍使用原字段。模板列表在独立生成器挂载、HTTP 快照恢复和定时刷新时重查；未新增 WebSocket topic、业务状态机或遥测。

`cmd/server -http-listen 127.0.0.1:8080` 默认启用独立 HTTP listener。控制 TCP 仍为 `-listen :9000`，Maintenance data 与入口配置保持 Phase 4。HTTP 包括命令执行、文件与设备断开能力，仅用于可信本机或受保护管理网络。非 loopback 绑定由部署者显式配置；远程访问由部署层完成 TLS、认证和网络访问限制。当前没有内置用户、租户、RBAC 或完整审计，不能把 loopback、Origin 校验或 Tunnel 配对 token 当成用户认证。

API 拒绝携带不同 Host 的浏览器 Origin；无 CORS 放行配置。无 Origin 的 CLI 可以访问。HTTP/WebSocket 共用一监听，WebSocket不接收业务命令。Server shutdown 停止 API 准入、取消创建准备、关闭 HTTP 与 WebSocket socket、等待已准入工作退出，然后关闭 Maintenance/Gateway/Repository。已成功创建的对象只因其自身租期、Session或Server生命周期结束，不因创建请求断开而撤销。

## 通用约定

- 所有路径从 `/api/v1` 开始。ID 为不透明字符串，路径段须 URL encode；不得从 UUID、版本标签或字典序推断版本升级关系。
- JSON 字段使用 snake_case；成功为 `{"data":...}`，错误为 `{"error":{"code":"...","message":"..."}}`。客户端按 code 分支，message 不稳定。未知输出字段可忽略。错误不转发内部磁盘路径、socket或传输诊断。
- 时间输出为 UTC RFC3339（可带小数）；尚未发生的设备/Session时间为 null。task result 的 started_at/finished_at 同样转换为 RFC3339；Probe wire 仍用原整数秒。正超时输入为 timeout_seconds（uint32整数），Maintenance输入为 lease_ms（整数）。
- JSON请求必须为UTF-8 object，Content-Type为application/json；上限64KiB、深度32；拒绝重复成员、未知字段、null、尾随JSON、非法Unicode及错误字段类型。空动作提交 `{}`。
- 列表 `data={items:[],total,offset,limit}`；默认offset=0、limit=50，limit为1～200，offset非负。先过滤再分页。空列表为[]。设备/资产/工具/Maintenance按ID升序；版本按标签字典序；Session当前在前、结束历史从新到旧。分页不是跨请求事务，并发增删时客户端应刷新；total为本次查询的匹配条数。
- 状态码：200查询/同步动作/版本发布；201新资产/工具/Maintenance；202任务派发及尚无RESULT的结果查询。HTTP 202表示已记录/派发，不保证Probe已接受或成功。
- 400 invalid_request；403 origin_denied；404 not_found；405 method_not_allowed（Allow）；409 conflict / device_offline / session_changed / idempotency_conflict；411 length_required；413 payload_too_large；415 unsupported_media_type；416 range_not_satisfiable；422 incompatible / integrity_mismatch；503 capacity_exhausted / idempotency_capacity / server_closed / maintenance_disabled；504 operation_timeout；500 internal_error。409 session_changed也覆盖没有有效tunnel-capable Session。

## 幂等与异步语义

所有POST及PUT均要求 `Idempotency-Key`（1～128可打印ASCII、无空格）。账本作用域为API进程全部路径，默认4096项、不淘汰、不自动过期。相同键加相同方法、原始RequestURI和完全相同JSON字节返回原始响应；不同请求409。JSON字段顺序和空格变化视为不同请求。重放带 `Idempotency-Replayed: true`。Service中的版本发布/归档/下载完成幂等仍独立生效，即使用新HTTP键也不会改变这些既有业务身份。

创建准入后使用API生命周期context，默认准备/派发期限30s。取消HTTP请求不会发送TASK_CANCEL或关闭成功Maintenance。原始文件导入还受请求body生命周期约束；完整导入成功后也不回滚。已进入账本的响应（包括失败）保留，失败后确需重试须新键；语法、媒体类型和容量等准入前失败不占键。容量满拒绝新键，旧键可查询/重放。可通过 `-http-idempotency-capacity` 配置，不能靠驱逐旧键暗中允许重复执行。

任务创建返回 `{task_id,dispatch_uncertain}` 及Location `/api/v1/tasks/{id}`；文件操作同时返回operation字段。写入结果不确定也返回202、非空task_id和dispatch_uncertain=true，不自动创建替代任务。网络响应丢失用原HTTP键重试；需要向Probe查询/补报原任务，显式调用resend。文件中断重传才使用新的业务任务。没有任务取消接口；拒绝通过state=rejected且result=null表达。

账本和Task/File/Operation/Device/Maintenance均不跨Server重启恢复。禁止跨重启假定原HTTP键或task_id仍可去重；仅Repository元数据与内容持久化。原Service的进程内任务/设备历史保留政策没有被API改变；API新增创建/重发次数受有界账本约束。

## 资源与请求

下表路径省略 `/api/v1`。GET无请求体；POST/PUT均带幂等键。

| 方法和路径 | 输入 / 返回 |
| --- | --- |
| GET /devices | 分页；status可为online/offline |
| GET /devices/{id} | 最近registration、status、当前/最近Session、首次/最近时间及历史计数 |
| GET /devices/{id}/sessions | 分页；当前及保留结束Session，另含history_limit、total_sessions、evicted_sessions |
| POST /devices/{id}/disconnect | `{}`；同步请求断开当前Session；未知404、已离线disconnected=false |
| POST /devices/{id}/config-tasks | backend、operation 及对应 key/value/package；timeout_seconds 默认 5；创建 router_config |
| GET /tasks | 分页；device_id、state过滤；摘要task_id/device_id/type/state/created_at，不含输出或派发历史 |
| POST /tasks | device_id、command、timeout_seconds必填；cwd、env可选；创建exec |
| GET /tasks/{id} | 规格、state、last_session_id、dispatch_count和result；不暴露message_id/reply_to |
| GET /tasks/{id}/result | 200返回真实result或最终rejected；202返回当前state和null result |
| POST /tasks/{id}/resend | `{}`；复用旧规格/身份，返回202；不会重新执行已接受任务 |
| GET /tasks/{id}/transfer | task_id/transfer_id/size/sha256/committed/released/failed，不含LocalPath或内部Error文字 |
| GET /tasks/{id}/operation | task_id/transfer_id/device_id/session_id/tool_id/version/artifact_id/asset_id，不适用关联为空字符串 |
| POST /uploads | device_id、asset_id、remote_path、mode、timeout_seconds；overwrite默认false |
| POST /downloads | device_id、remote_path、name、timeout_seconds；目标由Repository分配，禁止传Server路径 |
| POST /deployments | device_id、tool_id、version、remote_path、timeout_seconds；可选artifact_id、overwrite=false |
| POST /downloads/{task_id}/complete | `{}`；要求committed+released，返回asset、transfer和task身份/状态；不改写最终RESULT；cleanup_pending警告保留成功asset_id |
| POST /downloads/{task_id}/cleanup | `{}`；只清理本次下载已释放的自有暂存，未导入完整提交返回409 |
| GET /assets | 分页；include_archived默认false |
| POST /assets?name={label} | 原始application/octet-stream、Content-Length、X-Content-SHA256；流式导入，返回201 Asset |
| GET /assets/{id} | Asset元数据，允许查询已归档项 |
| GET /assets/{id}/content | 原始application/octet-stream、attachment、X-Content-SHA256；支持标准HTTP Range/HEAD；归档项409 |
| POST /assets/{id}/archive | `{}`；归档，不回收字节；被活动版本引用409 |
| GET /tools | 分页；include_archived默认false |
| POST /tools | name必填、description可选；返回201 Tool |
| GET /tools/{id} | Tool元数据 |
| POST /tools/{id}/archive | `{}`；归档，保留稳定身份 |
| GET /tools/{id}/versions | 分页；include_archived默认false |
| PUT /tools/{id}/versions/{version} | `{artifacts:[{asset_id,platform,mode,rules}]}`；不可变发布，同规格返回原版本/Artifact身份，不同规格409 |
| GET /tools/{id}/versions/{version} | Version及完整Artifact列表 |
| POST /tools/{id}/versions/{version}/archive | `{}`；停止新投放/引用，不物理删除 |
| GET /tools/{id}/versions/{version}/compatibility | device_id必填、分页；每个Artifact、compatible/incompatible/unknown及checks(field/status/reason)；可查离线设备最近资料 |
| GET /maintenance | 分页；device_id及state=ready/closing/closed过滤 |
| POST /maintenance | device_id必填，lease_ms可选；省略为240分钟，显式值必须正整数；返回201及三入口 |
| GET /maintenance/{id} | Maintenance快照及当前入口，历史按Phase4配置保留 |
| POST /maintenance/{id}/close | `{}`；幂等等待本地释放；未知或已淘汰ID也200/released=true |

Asset、Tool、Version、Artifact和rules字段/限制保持ADR-019。mode为四位八进制，platform为linux，arch/libc必须有允许集合或显式 `["any"]`；models/kernels/required_capabilities可选。artifact_id仍由Repository生成且全局唯一。投放不自动执行、猜latest、降级或跨Session继承兼容判断。

原始导入默认最大1GiB，流式计算并验证声明SHA-256/长度后发布。其幂等签名使用方法、原始URI、声明长度和SHA-256；重放已有键直接返回旧Asset，不重新消费/导入body。首次完整校验保护声明与内容一致。相同内容使用新键导入仍创建新asset_id（原ADR-019语义），不能把摘要当业务身份。

Maintenance输出：maintenance_id、device_id、session_id、state、reason、created_at、expires_at、released、reusable_after、connections（数量）及endpoints。每入口含service/host/port/address/state，web另含url。SSH/Telnet使用address连接，不生成用户名/密码。固定目标保持Probe 127.0.0.1:80/22/23。没有token、connection_id、data listener地址或内部socket。自定义租期没有产品策略上限，只受现有Go time.Duration表达范围约束（最大9223372036854ms）；零、负数、浮点、溢出均400。ready只表示入口监听；租期优先于idle，端口隔离和Session替换沿用ADR-022。

## WebSocket

`GET /api/v1/events`，可选 `topics=devices,tasks,files,maintenance`，默认全部。无动态订阅命令。首次事件：

```json
{"type":"resync_required","topic":"","sequence":"1","time":"2026-09-06T00:00:00Z"}
```

后续：`{"type":"resource_changed","topic":"tasks","sequence":"2","time":"..."}`。sequence为当前连接内递增十进制字符串，避免JavaScript整数精度问题；它不是业务版本、全局游标或重放ID。

客户端先连接、收到resync_required后查询HTTP快照；查询过程中收到resource_changed再刷新对应资源。重连总是重新同步。设备通知覆盖发布/替换/结束，不推送每次心跳；任务通知覆盖规格/派发/ACK/RESULT；files通知覆盖传输创建/提交/释放/失败，以及Repository资产/工具/版本目录提交（包含导入和归档）；订阅方也刷新相关工具目录。Maintenance有界快照变化覆盖创建、关闭、到期、入口状态与连接数。事件是失效提示，可以合并、可能跳过短暂中间态，不能据此重建业务状态或完整审计。无日志/输出流、事件持久化、历史补发或事件确认协议。

单一采样worker默认250ms；每连接1 reader+1 writer、8条固定队列、5秒写期限、15秒ping、45秒pong期限、最大入站消息1024bytes。业务数据消息以1008关闭，客户端只需处理ping/pong/close。最多64客户端；升级前超额503；慢消费者队列满或写超时直接断开，其余客户端和Service不等待它。Server.Close关闭所有升级后的连接并等待reader退出。

HTTP最多32个并行handler，超额503；原生Serve监听同时最多 `MaxRequests+MaxClients+32` 个TCP连接（默认128，含idle），超额在读取HTTP前直接关闭；header上限16KiB、header读取5秒、idle60秒、body读取默认30秒。嵌入式调用者使用Server.Serve可复用这些网络限制；只挂载ServeHTTP时，外层HTTP Server负责连接数、超时和关闭自己的listener。

配置：`-http-max-requests`、`-http-max-websockets`、`-http-idempotency-capacity`、`-http-max-asset-bytes`、`-http-request-timeout`；均须正值。Go Config另可设置PollInterval、WriteTimeout。大文件准备与底层本地文件系统的不可中断I/O边界保持原实现；这不是硬实时关闭保证。

## 扩展边界

已有v1输出可增加字段和事件topic；客户端忽略未知输出。破坏资源/状态/错误语义须新版本或明确迁移。OpenAPI生成流程、认证/TLS/RBAC/完整审计、跨重启幂等账本及历史持久化为后续设计点。Phase 5不实现UI、MCP、AI、续租、任意目标端口或通用转发。


## 内部 Go Service 契约

### 当前内部任务派发接口

`gateway.Server.CreateExec(ctx, deviceID, request)` 返回 `(taskID, error)`，由 Gateway 调用 Task Service 建立与维护记录；HTTP Adapter通过management.Server调用。

- 成功派发：非空 taskID、nil error。
- 参数校验、编码、长度限制、离线或发送前失败：空 taskID、error；不保留本次未派发任务。
- 已尝试传输写入但返回错误：非空 taskID，且 `errors.Is(err, gateway.ErrDispatchUncertain)` 为 true；保留任务与 message_id，可通过 TaskSnapshot 查询。错误信息包含原 session_id；连接已关闭并废弃。

最后一种情况不能推断 Probe 未执行，不得自动创建新 task_id 重试副作用操作。调用者必须先保存返回的 taskID，再处理 error。Phase 1C 已实现同 task_id 重发与跨连接补报，不能自动创建替代任务。

正式外部契约见本文前半部分；内部Go接口不等同于外部JSON。新增或改变公开契约必须同步本文、实现、测试、PROJECT_STATUS和CHANGELOG。

## 待讨论

- 用户认证、设备认证、授权和租户模型。
- 统一错误响应格式、错误码与 HTTP 状态映射。
- 资源标识、字段命名、时间格式和分页过滤约定。
- 幂等请求、超时和长任务的异步交互方式。
- 文件上传下载方式、大小限制、校验和断点续传策略。
- WebSocket 的鉴权、事件格式、订阅、顺序、重连和补发语义。
- 平台用户对Tunnel的认证与授权；内部租约、地址和关闭语义见Phase 4章节。
- OpenAPI 是否作为正式契约及其生成和校验流程。
- API 兼容与废弃策略。
- 浏览器跨域、限流、审计和可观测性要求。

### Phase 1C 内部任务接口补充

- `gateway.Server.ResendTask(ctx, taskID) error`：从 Task Service 获取原规格，向该 device_id 当前在线会话重发同一个 task_id；记录新的 session_id/message_id。不会创建新任务或改变 command、cwd、env、timeout。未知、离线、已 rejected、上下文取消或发送前失败返回错误；传输写入后失败返回 ErrDispatchUncertain，原任务及派发记录保持。
- `TaskSnapshot` 包含 Dispatches，每项记录 SessionID、MessageID、对应 Ack；旧 MessageID 字段表示最近一次派发编号，不能单独用于跨会话关联。Snapshot 返回副本。
- `WaitTaskResult` 等待真正 RESULT 或最终拒绝；即使 ACK.state 为完成态，也继续等待 RESULT。缺 ACK 的已派发结果可完成等待；重复 ACK/RESULT 不重复完成或回退状态。具体 wire 契约以 PROTOCOL.md / ADR-015 为准。
- Server 只在内存中保留这些状态；进程重启后的恢复仍未实现。此处为Go内部契约；Phase 5外部Adapter按本文前半部分复用。

## Phase 1D 内部文件接口

- gateway.Server.CreateUpload(ctx, deviceID, filetransfer.UploadRequest) 返回 (taskID,error)。参数为 SourcePath、RemotePath、Mode（必填四位八进制）、Overwrite、Timeout。File Service 在调用者协程流式准备源元数据，不阻塞连接 Reader。
- CreateDownload(ctx, deviceID, filetransfer.DownloadRequest) 返回 (taskID,error)。参数为 RemotePath、ResultName、TargetPath（Server 本地绝对路径）、Overwrite、Timeout。本地路径与覆盖策略不传给 Probe。
- 创建、派发失败与 ErrDispatchUncertain 的返回规则沿用 CreateExec。非空 taskID 必须保存；不得因 error 自动创建替代文件任务。创建上下文只约束准备与派发，不表示 TASK_CANCEL；业务 timeout 从 Probe 晋升 active 起计算。
- ResendTask 对文件使用原 type/timeout/params，只查询或补报原任务；不会再次打开或发布文件。中断后重新传输必须创建新任务及 transfer_id。
- WaitTaskResult/TaskSnapshot 沿用任务接口。FileSnapshot 返回 TaskID、TransferID、LocalPath、Committed、Size、SHA256、Error；Committed/Size/SHA256 仅描述 Server 下载接收端已校验发布的事实。它不是 TASK_RESULT 的替代：done ACK 丢失时 Committed=true 可以与最终 failed 并存。
- 这里记录内部Go能力；Phase 5复用它们，Server/Probe重启恢复仍未实现。

## Phase 2 内部设备查询接口

`gateway.Server.Devices() device.Query` 返回 `internal/device` 的只读查询接口；调用方通过此 Service 查询，不访问 Gateway 连接表或订阅 Events 重建设备状态。

| 方法 | 返回与语义 |
| --- | --- |
| `List() []device.Snapshot` | 全部已知设备，包括离线设备；按 device_id 升序，空 Inventory 返回空切片 |
| `Get(deviceID) (device.Snapshot, error)` | 设备最近注册资料、在线状态、首次/最近时间、当前及最近 Session、累计 Session 数及已淘汰数量；未知 ID 返回 `device.ErrNotFound` |
| `Sessions(deviceID) (device.SessionHistory, error)` | 当前 Session 与保留的已结束历史；Ended 按结束操作顺序从旧到新，另返回 Limit、TotalSessions、EvictedSessions；未知 ID 返回 `device.ErrNotFound` |

`Snapshot.Registration` 保存 device_id、serial、model、firmware、probe_version、hostname、arch、kernel、libc、boot_id 和 capabilities。每次成功注册整体替换，可选字符串省略与空值统一为未知，不沿用旧值。capabilities 保留未知 token，表示 Probe 声明，不能据此推断 Server 实现了该功能；本阶段不新增任务能力准入规则。

`Snapshot.Status` 为 online/offline。CurrentSession 在线时非 nil，离线时 nil；LatestSession 在线时为当前 Session，离线时为最近结束的 Session。Session 包含 ID、对应 Registration、StartedAt、LastSeenAt、EndedAt、EndReason；当前会话 EndedAt 为 Go time.Time 零值、EndReason 为空。

时间使用 Server 观测的 `time.Time`，不使用 Probe 时钟或 boot_id 推断状态；外部JSON时间格式见前文，内部零时间映射null：

- FirstSeenAt：本 Service 生命周期内首次成功发布注册的时间，历史淘汰不改变它。
- LastSeenAt：最近 Session 的最后合法活动时间，离线后保留；单 Session 内迟到时间不会使它倒退。
- LastOnlineAt：最近一次成功发布当前 Session 的时间，replaced 也更新。
- LastOfflineAt：只在设备整体 online → offline 时更新；replaced 保持 online，不更新此值。尚未发生离线时为零值。

结束原因是内部诊断值：replaced、disconnected（对端 EOF/读错误或其他连接关闭）、heartbeat_timeout、write_error、protocol_error、server_closed、requested_disconnect。首次结束转换决定该历史记录；旧 Session 后续活动/清理不能改写历史或当前设备状态。原因不增加 wire 消息，不用于推断 Task/File 终态。

`gateway.Config.DeviceHistoryLimit` 为每设备已结束 Session 保留数：0 使用默认 64，正数自定义，负数使 New 返回错误；当前 Session 不占历史槽位。淘汰最旧历史时增加 EvictedSessions，Task/File 记录不受影响。设备清单本身不自动淘汰，没有持久化，Server 重启后为空。

每次查询在 Device Service 的读锁内取得一致快照，所有嵌套 Session 和 capabilities 都是独立副本；调用方修改返回值不会改变 Service。多次查询之间可发生状态变化，不提供跨调用事务或可重放事件流。Phase 5外部HTTP/WebSocket调用这些查询，Phase 6 Windows UI仅消费外部契约；MCP未实现。

## Phase 3 内部 Repository 与管理接口

`repository.Open(directory)` 打开单进程独占的持久目录，空配置使用 `./data/repository`，`Directory()` 返回解析后的绝对路径。`Files()` 返回 FileService，`Tools()` 返回 ToolService；`Close()` 释放目录锁。运行中的目录不能被多个 Store/Server 同时打开。`Leftovers()` 只报告暂存、未引用 blob 和遗留元数据文件，不删除数据。

### 资产与工具

| 方法 | 语义 |
| --- | --- |
| `Files().Import(ctx, name, io.Reader)` | 流式导入并返回新的 Asset；同内容共享 blob，仍产生独立 asset_id |
| `Files().Get(assetID)` / `List(includeArchived)` | 返回资产副本；Get 可查归档项，List 按 ID 排序 |
| `Files().Archive(assetID)` | 逻辑归档；被未归档版本引用时返回 ErrReferenced，不删除字节 |
| `Tools().Create(name, description)` | 创建独立 UUID tool_id；同名工具允许存在，名称不是身份 |
| `Tools().Get(toolID)` / `List(includeArchived)` | 查询工具副本，List 按 ID 排序 |
| `Tools().Publish(toolID, version, []ArtifactSpec)` | 不透明版本标签；整组发布不可变产物，自动生成全仓库唯一 UUID artifact_id |
| `Tools().Version(toolID, version)` / `Versions(toolID, includeArchived)` | 返回包含产物/约束的深拷贝；版本列表按标签字典序，不表示升级顺序 |
| `Tools().Archive(toolID)` / `ArchiveVersion(toolID, version)` | 停止新投放/新引用，保留 ID、版本和数据；工具归档不改写子版本的归档字段 |

Asset 保存 ID、Name、Size、SHA256、CreatedAt、Archived。Tool 保存 ID、Name、Description、CreatedAt、Archived。Version 保存 ToolID、Version、Artifacts、CreatedAt、Archived。Artifact 保存 ID、AssetID、Platform、Mode 和 Rules。调用方不能指定或重用 tool_id/asset_id/artifact_id。资产/工具名称 1～255 UTF-8 bytes，描述最多 4096 bytes，版本标签 1～128 bytes，均拒绝 NUL；每版 1～128 个产物。

ArtifactSpec 提交 AssetID、Platform、Mode、Rules，不包含 artifact_id。Platform 固定 linux，Mode 为四位八进制。Rules.Arch/Libc 须非空，`["any"]` 是显式不限制，不能与其他值混用；Models/Kernels/RequiredCapabilities 空集合表示不增加该类约束。每字段最多 32 个允许值；归一化、去重、排序后保存。只有 amd64/x86_64、arm64/aarch64 作架构别名；libc 转小写，其余精确匹配。完整规则见 Accepted ADR-019 R4。

Publish 比较归一化后的全部产物规格，输入产物顺序与允许集合顺序不影响重复发布。完全相同规格返回已有版本及原 artifact_id（归档后也只返回旧身份）；不同规格返回 ErrConflict。跨版本、跨工具始终生成不同 artifact_id，即便引用相同资产。归档工具不能发布新版本，归档资产不能用于新版本。重复 Archive 幂等。

### 管理端编排

`management.NewService(repo, device.Query, FileTasks)` 组合已存在的独立服务。`FileTasks` 由 `gateway.Server` 实现，只提供原文件创建、快照、等待与重发能力。`management.New(Config)` 是持久仓库与 Gateway 的程序组合入口，返回具有这些 Service 能力的 Server；Serve/Close 管理运行生命周期。既有 gateway.New/Run 和 Phase 1 内部入口继续可用。

| 方法 | 参数与返回 |
| --- | --- |
| `Compatibility(deviceID, toolID, version)` | 返回每个 Artifact 的 compatible/incompatible/unknown 及逐字段 Check；可以查询离线设备的最近资料 |
| `Deploy(ctx, DeployRequest)` | DeviceID、ToolID、Version、可选 ArtifactID、RemotePath、Overwrite、Timeout；返回 Operation |
| `Upload(ctx, UploadRequest)` | DeviceID、AssetID、RemotePath、Mode、Overwrite、Timeout；上传资产，不附加工具兼容规则 |
| `Download(ctx, DownloadRequest)` | DeviceID、RemotePath、Name、Timeout；使用 Repository 自有暂存目标，返回 Operation |
| `CompleteDownload(ctx, taskID)` | 显式导入完整本地提交；返回 Asset、FileSnapshot 与 TaskSnapshot，不修改 Task 最终状态 |
| `CleanupDownload(taskID)` | 清理本进程自有、worker 已释放的暂存；未导入的完整提交不可通过此方法丢弃 |
| `Operation(taskID)` | 查询本进程关联的 task_id、transfer_id、device_id、session_id，以及适用的工具/版本/产物/资产 ID |
| `TaskSnapshot` / `FileSnapshot` / `WaitTaskResult` / `ResendTask` | 委托既有 Service，保持 Phase 1 幂等、拒绝、超时与不确定派发语义 |

投放必须 online 且声明 file 能力；选择显式 ArtifactID 或唯一 compatible 产物。无匹配返回 ErrIncompatible，多个匹配返回 ErrAmbiguous。未知受限字段不能投放；可先用 Compatibility 获得原因。不自动推断 latest、替代版本、降级产物或执行工具。

所有文件操作返回非空 TaskID 时必须先保存 Operation，再处理 error；ErrDispatchUncertain 不代表 Probe 未接受。重发继续使用原 task_id/transfer_id；重新传文件须明确创建新任务。投放检查与派发绑定当前 Session；Gateway 在 writer 等待结束、记录派发之前复核当前 Session，变化返回 ErrSessionChanged，不重定向到新连接。派发准入之后仍可能断线或替换，继续按既有不确定派发/传输失败收敛；不保证 Session 在整个传输期间不变。

CompleteDownload 要求 `Committed=true`、`Released=true` 和登记的暂存路径一致，再重新流式核对 Size/SHA256。`Released` 是本地 worker 已关闭句柄的事实，不是 Probe TASK_RESULT。即使 Task 为 failed（如 done ACK 丢失），仍可导入已提交的完整文件；返回两种事实，不伪造 success。导入成功后清理自有暂存，FileSnapshot 的历史 LocalPath 仍为原路径。若清理失败，返回非空 Asset 及 error，资产身份保持，可调用 CleanupDownload。重复/并发 CompleteDownload 返回原 asset_id，即使后来归档，不重复导入。

`UploadRequest.ExpectedSessionID`、`DownloadRequest.ExpectedSessionID` 和 `UploadRequest.Expected *filetransfer.ContentMetadata` 是新增的可选本地前置条件，默认空值保持旧行为，不编码进 TASK/FILE。Expected 在既有 OpenSource 预读后核对 Size/SHA256，阻止仓库文件变化被当成新资产内容。`FileSnapshot.Released` 在 worker 退出并释放句柄后置 true，其余 Phase 1 字段语义不变。

Repository 与 Device/Task 查询返回副本。Repository 的 WithAsset/WithArtifact 为内部使用的准入回调：持仓库读锁完成内容校验与派发准备，阻止并发归档/关闭；回调不得重入 Repository。此锁可以跨文件准备与派发 I/O，但不是 Device/Gateway 连接表锁。Device/Gateway 锁仍不跨磁盘或网络 I/O。

Repository 元数据/字节跨进程保留；Operation、下载导入的 task_id 关联、Task/transfer、Device/Session 不持久化。Phase 5公开HTTP/WebSocket契约见前文。

## Phase 4 内部 Maintenance API

`management.Config.Tunnel *tunnel.Config` 启用维护服务；nil保留旧嵌入式调用方行为（不启动data listener）。`management.Server.Maintenance()` 返回已组合的 `*tunnel.Service`，未启用时nil。`cmd/server` 默认启用；Phase 5外部Adapter只调用Service。

| 方法 | 契约 |
| --- | --- |
| `Create(ctx, deviceID, lease time.Duration) (Snapshot,error)` | 在线且声明tunnel能力；一次原子创建web/ssh/telnet三个入口。lease=0选240分钟，正值至少1ms；同设备已有未释放Maintenance时返回冲突，不修改原租期。创建失败返回空快照、释放部分端口；成功后ctx取消不关闭Maintenance |
| `Get(maintenanceID)` / `List()` | 查询独立快照；List按ID排序，关闭历史按配置保留，淘汰后Get返回ErrNotFound |
| `CloseMaintenance(maintenanceID) error` | 并发/重复调用幂等，等待Server listener、pending/active socket、accept/已配对握手/Relay worker释放后返回，不等待Gateway控制发送worker；未知/历史已淘汰ID同样成功 |
| `Close() error` | 幂等关闭整个Service，另关闭data listener和所有未握手socket，等待全部worker；management.Server.Close会调用 |

Snapshot为ID、DeviceID、SessionID、State、Reason、CreatedAt、ExpiresAt、Released、ReusableAfter、Connections、Endpoints。State为ready/closing/closed；失败创建不产生可查询ID。ReusableAfter在释放后表示本次端口最早可复用时刻，释放前为零值。Connections统计pending+active；Endpoint为Service、Host、Port、State，Address()返回正确IPv4/IPv6 host:port。顺序固定web/ssh/telnet，状态ready/unavailable/closed；ready不承诺本地服务持续存在。失败影响本次连接，后续客户端可重试。

Close先撤监听，再reset data TCP并关闭外部流，Released是Server本地资源回收事实，没有Probe释放ACK。CLOSE通过Gateway每Session一个worker、64项有界队列尽力发送，本地释放不等待控制网络。Session失效后准入和配对同步拒绝，watcher发起关闭；新Session不继承。Released后端口默认隔离24小时；隔离记录独立于关闭历史，池满返回ErrCapacity，不提前复用。闭合历史地址不能继续使用；超出隔离期或Server重启后，旧客户端与新客户端无法由原始TCP区分。需永久隔离时部署不重叠的池/地址，见ADR-022。

配置默认值：BindHost/AdvertisedHost/DataHost均127.0.0.1；DataListen=127.0.0.1:9001；PortFirst/PortLast=20000/20199；MaxMaintenance=64；PerMaintenance=8、PerDevice=8、TotalConnections=512；Handshakes=64；History=128；PendingTimeout=10s、HandshakeTimeout=5s、IdleTimeout=24h、PortReuseDelay=24h。数值0选择默认；IdleTimeout范围1ms～24h，PortReuseDelay至少1ms，非法范围启动报错。默认200个端口在隔离窗口内最多支持66次三入口分配，按维护频率配置更大池。各限制不提供无界关闭选项。

DataHost接受IP或DNS主机名；Create在Server解析（最多5s、受ctx取消），优先IPv4，本次维护固定所得IP，下次Create更新；并行Create最多MaxMaintenance个，超额ErrCapacity。解析失败无入口，解析不阻塞Close；绑定失效在发布前复核。Probe不做DNS，仍收到数值IP。部署者保证Server解析所得地址从Probe可达；不自动判断split-horizon或逐地址故障切换。AdvertisedHost可为域名，由外部客户端解析。

Server flags：`-tunnel-bind`、`-tunnel-host`、`-tunnel-data-listen`、`-tunnel-data-host`、`-tunnel-port-first`、`-tunnel-port-last`、`-tunnel-port-reuse-delay`、`-tunnel-max-sessions`、`-tunnel-session-connections`、`-tunnel-device-connections`、`-tunnel-total-connections`、`-tunnel-handshakes`、`-tunnel-history`、`-tunnel-connect-timeout`、`-tunnel-handshake-timeout`、`-tunnel-idle-timeout`。Probe `--tunnel-connections`默认8，允许1～64，超出范围启动失败。每流一线程，低内存部署可调小；需要更多浏览器连接时同步提高双方限额。公网绑定、可达地址、NAT和防火墙由部署者配置。

固定目标为Probe的127.0.0.1:80/22/23。Maintenance/connection/token不持久化；无跨进程恢复、续租、任意端口、UDP/SOCKS/VPN/P2P、HTTP反向代理、TLS终止或通用映射管理；Phase 5增加HTTP/WebSocket，不增加UI/MCP或操作CLI。
