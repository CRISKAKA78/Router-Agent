# Management Server API 设计基线

当前阶段尚未实现任何 HTTP API 或 WebSocket。本文件维护 API First、职责边界、能力分类和已实现的内部 Go Service 查询/操作契约；外部字段、认证模型和具体 endpoint 行为在后续 API 设计阶段确定。

## API First

设备控制能力必须先成为 Management Server 的 Application 或 Service 能力，再由 HTTP、WebSocket、CLI、MCP 或其他 Adapter 暴露。Web、微信小程序、Windows UI、CLI、MCP 和 AI Agent 共用同一套核心业务，不得分别实现设备、任务、文件或 Tunnel 逻辑。

公开 HTTP API 从第一版起使用版本前缀：

~~~text
/api/v1
~~~

版本前缀已经确认；资源路径、请求响应字段和兼容策略仍为 TBD。

## HTTP API 职责

HTTP API 负责请求响应型操作，例如：

- 查询设备、设备状态和设备详情。
- 创建和查询任务；取消能力属于后续设计。
- 管理文件资产和工具。
- 发起上传、下载或工具投放。
- 创建、查询和关闭 Tunnel。
- 查询当前设备会话或平台会话。

HTTP Handler 只负责协议层工作：解析参数，获取认证与请求上下文，调用 Application 或 Service，并转换响应。它不得直接访问 Probe TCP connection registry，不得直接操作存储表，也不得拼装控制协议帧。

## WebSocket 职责

WebSocket 或等价实时接口负责向客户端推送：

- 设备上线与离线。
- 任务状态变化和最终结果。
- 经明确设计允许的日志或进度流。
- 文件传输状态变化。
- Tunnel 状态变化。

事件名称、订阅模型、重连、补发、顺序和背压规则均为 TBD。WebSocket 不替代 HTTP 的资源管理接口，也不形成另一套业务逻辑。

## 能力分类

以下分类用于后续设计导航，不是已经冻结的 endpoint 清单：

| 分类 | 预期职责 | 当前状态 |
| --- | --- | --- |
| devices | 设备清单、详情、在线状态和能力 | Phase 2 内部 Service 已实现；HTTP 未设计 |
| sessions | Probe 会话与连接状态查询 | Phase 2 内部 Service 已实现；HTTP 未设计 |
| tasks | 创建、查询、取消任务及读取结果 | 未设计 |
| files | 文件资产、上传、下载与传输状态 | 未设计 |
| tools | 工具元数据、版本、兼容性与投放 | 未设计 |
| tunnels | 创建、查询和关闭临时 Tunnel | 未设计 |

这些分类可以在正式设计中调整。任何调整都必须保持 API First 和 Service 复用原则。

Protocol v1 的 Phase 1 不实现 TASK_CANCEL。未来 API 是否提供任务取消，以及其语义如何映射到 Probe 协议，仍为 TBD。

## 客户端边界

| 客户端 | 接入原则 |
| --- | --- |
| Web | 使用公开 HTTP API 与 WebSocket |
| 微信小程序 | 使用同一公开 API，不依赖 Server 内部实现 |
| Windows UI | 使用同一 API；即使内嵌 Web UI，也不复制核心业务 |
| CLI | 作为薄客户端使用公开 API 或稳定 Service |
| MCP | 复用 Service 或 API，不直接控制 Probe 连接 |
| AI Agent | 通过 API、MCP 或受控的临时通道使用平台能力 |

## 与 Probe 协议的关系

外部 API 表达用户和平台业务语义，Probe TCP 协议表达 Server 与设备之间的控制语义。二者不能简单等同：

- API Adapter 不直接拼装 TCP 帧。
- Service 将 API 用例转换为设备、任务、文件或 Tunnel 操作。
- Probe Gateway 负责把内部请求适配为 [PROTOCOL.md](PROTOCOL.md) 定义的消息。
- task_id、transfer_id 和 session_id 的业务含义必须在两层间保持一致。

## 状态与兼容

### 当前内部任务派发接口

`gateway.Server.CreateExec(ctx, deviceID, request)` 返回 `(taskID, error)`，由 Gateway 调用 Task Service 建立与维护记录，尚无 HTTP/CLI 入口。

- 成功派发：非空 taskID、nil error。
- 参数校验、编码、长度限制、离线或发送前失败：空 taskID、error；不保留本次未派发任务。
- 已尝试传输写入但返回错误：非空 taskID，且 `errors.Is(err, gateway.ErrDispatchUncertain)` 为 true；保留任务与 message_id，可通过 TaskSnapshot 查询。错误信息包含原 session_id；连接已关闭并废弃。

最后一种情况不能推断 Probe 未执行，不得自动创建新 task_id 重试副作用操作。调用者必须先保存返回的 taskID，再处理 error。Phase 1C 已实现同 task_id 重发与跨连接补报，不能自动创建替代任务。

当前没有已发布 API，也没有客户端兼容承诺。正式 API 设计后，新增或改变资源、事件或错误行为时必须同步更新本文件、实现、测试、PROJECT_STATUS 和 CHANGELOG。

## 待讨论

- 用户认证、设备认证、授权和租户模型。
- 统一错误响应格式、错误码与 HTTP 状态映射。
- 资源标识、字段命名、时间格式和分页过滤约定。
- 幂等请求、超时和长任务的异步交互方式。
- 文件上传下载方式、大小限制、校验和断点续传策略。
- WebSocket 的鉴权、事件格式、订阅、顺序、重连和补发语义。
- Tunnel 的访问凭证、租约、暴露地址和关闭语义。
- OpenAPI 是否作为正式契约及其生成和校验流程。
- API 兼容与废弃策略。
- 浏览器跨域、限流、审计和可观测性要求。

### Phase 1C 内部任务接口补充

- `gateway.Server.ResendTask(ctx, taskID) error`：从 Task Service 获取原规格，向该 device_id 当前在线会话重发同一个 task_id；记录新的 session_id/message_id。不会创建新任务或改变 command、cwd、env、timeout。未知、离线、已 rejected、上下文取消或发送前失败返回错误；传输写入后失败返回 ErrDispatchUncertain，原任务及派发记录保持。
- `TaskSnapshot` 包含 Dispatches，每项记录 SessionID、MessageID、对应 Ack；旧 MessageID 字段表示最近一次派发编号，不能单独用于跨会话关联。Snapshot 返回副本。
- `WaitTaskResult` 等待真正 RESULT 或最终拒绝；即使 ACK.state 为完成态，也继续等待 RESULT。缺 ACK 的已派发结果可完成等待；重复 ACK/RESULT 不重复完成或回退状态。具体 wire 契约以 PROTOCOL.md / ADR-015 为准。
- Server 只在内存中保留这些状态；进程重启后的恢复仍未实现。此处均为 Go 内部能力，没有新增 HTTP、WebSocket、CLI、MCP 或 AI Adapter。

## Phase 1D 内部文件接口

- gateway.Server.CreateUpload(ctx, deviceID, filetransfer.UploadRequest) 返回 (taskID,error)。参数为 SourcePath、RemotePath、Mode（必填四位八进制）、Overwrite、Timeout。File Service 在调用者协程流式准备源元数据，不阻塞连接 Reader。
- CreateDownload(ctx, deviceID, filetransfer.DownloadRequest) 返回 (taskID,error)。参数为 RemotePath、ResultName、TargetPath（Server 本地绝对路径）、Overwrite、Timeout。本地路径与覆盖策略不传给 Probe。
- 创建、派发失败与 ErrDispatchUncertain 的返回规则沿用 CreateExec。非空 taskID 必须保存；不得因 error 自动创建替代文件任务。创建上下文只约束准备与派发，不表示 TASK_CANCEL；业务 timeout 从 Probe 晋升 active 起计算。
- ResendTask 对文件使用原 type/timeout/params，只查询或补报原任务；不会再次打开或发布文件。中断后重新传输必须创建新任务及 transfer_id。
- WaitTaskResult/TaskSnapshot 沿用任务接口。FileSnapshot 返回 TaskID、TransferID、LocalPath、Committed、Size、SHA256、Error；Committed/Size/SHA256 仅描述 Server 下载接收端已校验发布的事实。它不是 TASK_RESULT 的替代：done ACK 丢失时 Committed=true 可以与最终 failed 并存。
- 所有接口均为内部 Go 能力，没有新增 HTTP、WebSocket、CLI、UI、MCP 或 AI Adapter；Server/Probe 重启恢复不在本阶段实现。

## Phase 2 内部设备查询接口

`gateway.Server.Devices() device.Query` 返回 `internal/device` 的只读查询接口；调用方通过此 Service 查询，不访问 Gateway 连接表或订阅 Events 重建设备状态。

| 方法 | 返回与语义 |
| --- | --- |
| `List() []device.Snapshot` | 全部已知设备，包括离线设备；按 device_id 升序，空 Inventory 返回空切片 |
| `Get(deviceID) (device.Snapshot, error)` | 设备最近注册资料、在线状态、首次/最近时间、当前及最近 Session、累计 Session 数及已淘汰数量；未知 ID 返回 `device.ErrNotFound` |
| `Sessions(deviceID) (device.SessionHistory, error)` | 当前 Session 与保留的已结束历史；Ended 按结束操作顺序从旧到新，另返回 Limit、TotalSessions、EvictedSessions；未知 ID 返回 `device.ErrNotFound` |

`Snapshot.Registration` 保存 device_id、serial、model、firmware、probe_version、hostname、arch、kernel、libc、boot_id 和 capabilities。每次成功注册整体替换，可选字符串省略与空值统一为未知，不沿用旧值。capabilities 保留未知 token，表示 Probe 声明，不能据此推断 Server 实现了该功能；本阶段不新增任务能力准入规则。

`Snapshot.Status` 为 online/offline。CurrentSession 在线时非 nil，离线时 nil；LatestSession 在线时为当前 Session，离线时为最近结束的 Session。Session 包含 ID、对应 Registration、StartedAt、LastSeenAt、EndedAt、EndReason；当前会话 EndedAt 为 Go time.Time 零值、EndReason 为空。

时间使用 Server 观测的 `time.Time`，不使用 Probe 时钟或 boot_id 推断状态；外部 JSON 时间格式尚未设计：

- FirstSeenAt：本 Service 生命周期内首次成功发布注册的时间，历史淘汰不改变它。
- LastSeenAt：最近 Session 的最后合法活动时间，离线后保留；单 Session 内迟到时间不会使它倒退。
- LastOnlineAt：最近一次成功发布当前 Session 的时间，replaced 也更新。
- LastOfflineAt：只在设备整体 online → offline 时更新；replaced 保持 online，不更新此值。尚未发生离线时为零值。

结束原因是内部诊断值：replaced、disconnected（对端 EOF/读错误或其他连接关闭）、heartbeat_timeout、write_error、protocol_error、server_closed、requested_disconnect。首次结束转换决定该历史记录；旧 Session 后续活动/清理不能改写历史或当前设备状态。原因不增加 wire 消息，不用于推断 Task/File 终态。

`gateway.Config.DeviceHistoryLimit` 为每设备已结束 Session 保留数：0 使用默认 64，正数自定义，负数使 New 返回错误；当前 Session 不占历史槽位。淘汰最旧历史时增加 EvictedSessions，Task/File 记录不受影响。设备清单本身不自动淘汰，没有持久化，Server 重启后为空。

每次查询在 Device Service 的读锁内取得一致快照，所有嵌套 Session 和 capabilities 都是独立副本；调用方修改返回值不会改变 Service。多次查询之间可发生状态变化，不提供跨调用事务或可重放事件流。没有实现 HTTP/WebSocket、UI、MCP 或其他外部 Adapter。
