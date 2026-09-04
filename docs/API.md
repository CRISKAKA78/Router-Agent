# Management Server API 设计基线

当前阶段尚未实现任何 HTTP API 或 WebSocket。本文件只固定 API First、职责边界和能力分类；字段、认证模型和具体接口行为在后续 API 设计阶段确定。

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
| devices | 设备清单、详情、在线状态和能力 | 未设计 |
| sessions | Probe 会话与连接状态查询 | 未设计 |
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
