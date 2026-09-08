# 路由器探针 TCP 长连接控制协议

协议版本：Protocol v1  
基线来源：v0.2 Word 设计输入  
日期：2026-09-07

状态：已确认的互操作设计；Phase 1～5 已验收，ADR-029 新增启动属性模板准备连接与注册快照扩展。

## 配置任务扩展（ADR-031，2026-09-08）

新增 `router_config` capability 和同名 TASK type，沿用 TASK / TASK_ACK / TASK_RESULT、原任务状态及有界缓存，不增加消息类型。capability 表示支持协议，不保证设备安装 nvram 和 uci；执行时缺少命令返回 failed。Server 对未声明能力的当前 Session 不派发，不降级为 Exec。

```json
{"task_id":"config-1","type":"router_config","timeout":5,"params":{"backend":"uci","operation":"set","key":"system.@system[0].hostname","value":"router-one"}}
```

timeout 必须为 1～30 秒。params 是严格字段组合，所有成员为无 NUL UTF-8 字符串，不接受 null、未知字段或操作无关字段；value 允许空字符串且 set 必须显式提供：

| backend / operation | params 附加必选字段 | 设备调用 |
| --- | --- | --- |
| nvram / get | key | nvram get KEY |
| nvram / set | key、value | nvram set KEY=VALUE |
| nvram / delete | key | nvram unset KEY |
| nvram / commit | 无 | nvram commit |
| uci / get | key | uci get KEY |
| uci / set | key、value | uci set KEY=VALUE |
| uci / delete | key | uci delete KEY |
| uci / commit | package | uci commit PACKAGE |

nvram key 最多 128 bytes，字符 `[A-Za-z0-9_./:-]+` 且首字符非 `-`。UCI key 最多 256 bytes，为 `package.section.option`，package/option/命名 section 为 `[A-Za-z0-9_]+`；匿名 section 为 `@type[index]`，type 同上述字符集，index 为可带负号的十进制整数。首版仅操作 option。commit 的 package 最多 256 bytes，同上述字符集。value 最多 4096 bytes，无 shell 展开，保留空格、引号、换行和空字符串。

Probe 按运行环境 PATH 查找固件程序，缺省 PATH 使用 `/usr/sbin:/usr/bin:/sbin:/bin`；以独立 argv 执行，不拼 shell、不使用 libuci 或厂商库。复用 Exec 有界输出/清理，返回原 status、exit_code、stdout、stderr、truncated、时间及空 result object；输出 UTF-8 编码与截断规则同 Exec。成功空输出不推断为缺失键。非零退出、缺命令或超时如实报告，失败不自动回滚。

Probe 配置任务按首次接受顺序串行运行，复用 TaskManager 容量和 worker；其他 Exec 可独立执行。timeout 从实际执行开始计时，排队不计；配置任务之间不提供事务，外部 SSH/LuCI/原 Exec 不受队列约束。写入/删除不自动 commit、重启服务或设备。nvram commit 提交整份 NVRAM，uci commit 提交指定包，可能包含其他程序已暂存的修改。

同 task_id 的身份比较增加全部上述配置参数及其存在性，参数顺序无关；缺 value 与空 value 不等价。重复排队/执行/完成任务和冲突沿用 ADR-015，不重复副作用。断线不取消已接受配置任务，重连补报缓存结果；超时或不确定派发不证明配置未变，不自动创建替代任务。Probe/Server 跨进程恢复仍未实现。

### 模板配置来源

ADR-031 扩展下文 ADR-029：原 command 模板不变；可显式 `source:"command"`，或使用 `{name,source:"nvram"|"uci",key,timeout_seconds}`。配置 source 只执行 get，key 校验与配置任务相同，command 与配置 key 互斥。字段限制、默认每项 5 秒、整次 60 秒、空值失败摘要、显式 hostname 优先和重连不重采集均保持；REGISTER 不新增字段。

旧 Probe 可继续使用原 command 模板；选择新 source 模板会解析失败，须先更新 Probe。配置任务修改不刷新启动属性；即时读值使用配置读取任务。默认 `nvram get SN` 设备 ID 逻辑保持，OpenWrt 无该值时须显式传稳定 device-id。

本文是 [路由器探针_TCP长连接控制协议设计_v0.2.docx](../路由器探针_TCP长连接控制协议设计_v0.2.docx) 中 TCP 协议部分的仓库内维护版本，并包含 Phase 0 最终确认的 Protocol v1 互操作细化。这些规则不改变 TCP 长连接、20-byte Header、JSON Control 和 Binary FILE_CHUNK 的核心设计。

Phase 0 完成后，本文是持续维护的当前协议基线。Word v0.2 保留为原始设计输入和历史参考，不覆盖后续经过正式确认并写入本文或 ADR 的协议变化。

## 设计目标与范围

本协议用于 Probe 主动连接 Management Server，并在一条稳定的 TCP 长连接上完成设备注册、心跳、任务下发、结果回传、事件通知和文件控制。

本阶段采用固定二进制包头、JSON 控制载荷和二进制文件块的混合协议。TCP 控制连接只承担控制面和必要文件传输；SSH、Telnet、Web Tunnel 的持续数据流使用独立数据连接。

### 本阶段包含

- Probe 主动建立 TCP 长连接、注册、心跳与断线重连。
- 统一消息帧、消息类型、字段和错误处理规则。
- 通用任务模型：exec、upload、download、start_process、stop_process、open_tunnel、close_tunnel、get_info。
- TASK_ACK 与 TASK_RESULT 分离，支持多任务并发和乱序返回。
- 文件传输控制流程和 FILE_CHUNK 二进制分块格式。
- Probe 与 Server 状态机和协议实现边界。

### 本阶段不展开

- Web 管理界面和用户权限体系。
- AI Agent 的具体推理流程。
- MCP 工具的最终接口清单。
- 通用Tunnel；固定SSH、Telnet、Web的数据面细节见Phase 4章节。
- 完整的安全、认证、加密和审计策略。

## 设计原则

- Probe 保持轻量，只提供执行、文件、进程、Tunnel 等通用原语。
- 工具仓库、诊断脚本、设备能力判断和 AI 编排放在管理端。
- 控制面与数据面分离，控制 TCP 不承载 SSH、Telnet、Web 的持续交互流量。
- message_id 用于传输追踪，task_id 用于业务任务，transfer_id 用于文件传输。
- 控制载荷优先使用 JSON UTF-8，便于抓包、日志和早期开发。
- 文件块传输原始二进制数据，不使用 Base64。

## TCP 连接生命周期

~~~text
Probe START
   |
   +-- load config and collect basic info
   |
   +-- TCP CONNECT
   |
   +-- REGISTER ------------------> Server
   |                           <-- REGISTER_ACK
   |
   +-- ONLINE
   |    +-- HEARTBEAT <--> HEARTBEAT_ACK
   |    +-- TASK <---------- Server
   |    +-- TASK_ACK ------> Server
   |    +-- TASK_RESULT ---> Server
   |
   +-- disconnected -> backoff -> reconnect -> REGISTER again
~~~

### 建议连接参数

| 项目 | 建议值 | 说明 |
| --- | --- | --- |
| 连接方向 | Probe -> Server | 路由器主动向公网管理端发起连接 |
| TCP Keepalive | 开启 | 辅助发现半开连接；业务在线状态仍以应用层心跳为准 |
| 心跳间隔 | 30 s | 由 REGISTER_ACK 下发，可配置 |
| 离线判定 | 默认 90 s 无有效消息 | 任何合法消息均刷新 last_seen |
| 重连策略 | 1/2/5/10/30 s，之后固定 30 s | 成功连接后重置退避 |
| 连接状态 | CONNECTING / REGISTERING / ONLINE / RECONNECTING | Probe 和 Server 都应显式维护状态 |

## TCP 消息帧格式

TCP 是字节流，协议必须自行定义消息边界。每个消息统一使用 20 字节固定包头。所有整数使用网络字节序 Big Endian。

| 偏移 | 长度 | 字段 | 类型 | 说明 |
| ---: | ---: | --- | --- | --- |
| 0 | 4 | magic | char[4] | 固定 ASCII：RMP1，表示 Router Management Protocol v1 |
| 4 | 1 | version | uint8 | 协议主版本，当前为 1 |
| 5 | 1 | type | uint8 | 消息类型 |
| 6 | 2 | flags | uint16 | 消息标志位 |
| 8 | 4 | payload_len | uint32 | Payload 字节长度，不包含 20 字节包头 |
| 12 | 8 | message_id | uint64 | 单 TCP Session 内的传输追踪和响应关联编号 |

~~~text
0                   4   5   6       8              12                  20
+-------------------+---+---+-------+----------------+-----------------------+
| magic = "RMP1"    |ver|typ| flags | payload_len    | message_id            |
+-------------------+---+---+-------+----------------+-----------------------+
| Payload ...                                                            |
+------------------------------------------------------------------------+
~~~

### message_id

- message_id 类型为 uint64，0 保留不用。
- 每个 TCP 连接内，每个发送方向维护独立计数器。
- 每个方向的第一条发送消息使用 message_id = 1，此后单调加 1。
- TCP 重连后，新连接的两个方向分别重新从 1 开始。
- message_id 只用于单 TCP Session 内的日志、传输追踪和 response correlation。
- message_id 不用于任务业务幂等。
- 接收端不需要使用 message_id 做消息去重。
- 计数器理论上达到 uint64 最大值时必须重新建立连接，不允许回绕后继续使用。

### Flags

| Bit | 名称 | 含义 |
| ---: | --- | --- |
| 0 | RESPONSE | 该消息是对另一帧的直接响应；JSON Payload 必须包含 reply_to |
| 1 | BINARY | Payload 为二进制结构而不是 JSON |
| 2 | MORE | Protocol v1 必须为 0，仅保留未来用途 |
| 3-15 | Reserved | 当前必须置 0 |

所有 JSON Response 类型统一包含 reply_to，类型为 uint64。RESPONSE=1 时，reply_to 必须存在并等于被响应帧的 message_id。

Protocol v1 至少对以下消息设置 RESPONSE=1：

- REGISTER_ACK。
- HEARTBEAT_ACK。
- TASK_ACK。
- FILE_ACK。
- 作为某个具体请求直接响应产生的 ERROR。

TASK_RESULT 是通过 task_id 关联的异步业务最终结果，不设置 RESPONSE。EVENT 不设置 RESPONSE。

FILE_CHUNK 必须设置 BINARY=1。当前所有 JSON 消息必须设置 BINARY=0。如果 message type 与 BINARY flag 不一致，接收端返回协议错误或关闭连接，具体取决于错误严重程度。严重程度与关闭连接规则仍为 TBD。

解析时先累积至少 20 字节包头，校验 magic 和 version，再按 payload_len 精确读取 Payload。单次 recv() 绝不能假设得到完整消息。

### 长度限制

| 类别 | 默认上限 | 处理方式 |
| --- | --- | --- |
| 普通 JSON 控制消息 | 1 MiB | 超过直接拒绝并返回 ERROR |
| FILE_CHUNK | 64 KiB 默认；512 KiB 硬上限 | 使用二进制块传输 |
| TASK_RESULT stdout | 1 MiB 建议上限 | 超出时截断并标记 truncated=true；大输出改为文件回传 |
| TASK_RESULT stderr | 1 MiB 建议上限 | 超出时截断并标记 truncated=true；大输出改为文件回传 |

### JSON 基础规则

Protocol v1 的 JSON Payload 遵循以下规则：

- 编码必须为 UTF-8。
- 顶层必须是 JSON object。
- 接收端应忽略未识别的额外字段，以支持前向兼容。
- 缺少必选字段时返回 INVALID_PAYLOAD。
- 字段类型错误时返回 INVALID_PAYLOAD。
- 发送端不得产生重复 JSON key。
- null 只有在字段规范明确允许时才合法。
- Unix 时间统一使用整数秒。
- 整数字段必须以 JSON integer 表达，不使用浮点表示。
- 字符串必须是合法 Unicode 和 UTF-8。
- Protocol v1 不引入 canonical JSON。

## 消息类型

Phase 4 新增下表后的 0x40/0x41/0x42，完整规范见文末“Phase 4 Maintenance 控制与数据协议”；历史任务示例中的 open_tunnel/close_tunnel 未作为 TASK 实现，正式维护生命周期不进入 Task 缓存。

| Type | 名称 | 方向 | Payload | 用途 |
| --- | --- | --- | --- | --- |
| 0x01 | REGISTER | Probe -> Server | JSON | 连接建立后的设备注册 |
| 0x02 | REGISTER_ACK | Server -> Probe | JSON | 确认注册并返回会话参数 |
| 0x03 | HEARTBEAT | Probe -> Server | JSON | 应用层心跳和轻量运行状态 |
| 0x04 | HEARTBEAT_ACK | Server -> Probe | JSON | 心跳确认 |
| 0x05 | TEMPLATE_GET | Probe -> Server | JSON | 启动时按 ID/名称查询属性模板，仅准备连接首帧 |
| 0x06 | TEMPLATE_REPLY | Server -> Probe | JSON RESPONSE | 模板或明确错误，发送后关闭准备连接 |
| 0x10 | TASK | Server -> Probe | JSON | 下发通用任务 |
| 0x11 | TASK_ACK | Probe -> Server | JSON | 确认已接收或拒绝任务 |
| 0x12 | TASK_RESULT | Probe -> Server | JSON | 任务最终结果 |
| 0x13 | TASK_CANCEL | Server -> Probe | 保留 | Protocol v1 Phase 1 不支持，消息号留作后续扩展 |
| 0x20 | EVENT | Probe -> Server | JSON | Probe 主动上报异步事件 |
| 0x30 | FILE_BEGIN | 双向 | JSON | 开始文件传输 |
| 0x31 | FILE_CHUNK | 双向 | Binary | 文件数据块 |
| 0x32 | FILE_END | 双向 | JSON | 文件发送完毕 |
| 0x33 | FILE_ACK | 双向 | JSON | 文件接收状态或最终确认 |
| 0x40 | TUNNEL_CONNECT | Server -> Probe | JSON | 为一个外部客户端建立独立数据流 |
| 0x41 | TUNNEL_CLOSE | Server -> Probe | JSON | 取消一条流或整个维护会话的流 |
| 0x42 | TUNNEL_STATUS | Probe -> Server | JSON | 当前Session建流失败状态 |
| 0xFE | ERROR | 双向 | JSON | 协议级或通用错误 |

## 注册与设备会话

### 启动属性模板（ADR-029）

不选模板的 Probe 继续直接 REGISTER。选择 `--template-id` 或 `--template-name` 时，先连接同一控制端口，首帧发送 TEMPLATE_GET（message_id=1、flags=0），收到 TEMPLATE_REPLY 后关闭准备连接，执行采集，再用新 TCP 连接正常 REGISTER。准备连接不发布 Inventory/Session，不接收任务/维护；REGISTER 成功才上线。此扩展仅取代旧的“所有连接首帧必须 REGISTER”限制，不改变正式控制会话。

请求 `{"template_id":"…"}` 或 `{"name":"…"}`，两者必选其一；UTF-8 非空、最多 128 bytes，请求最多 1024 bytes。TEMPLATE_REPLY 的 flags=RESPONSE、message_id=1、reply_to=1：

```json
{"reply_to":1,"success":true,"max_control_payload":1048576,"template":{"template_id":"…","name":"路由器","version":1,"properties":{"model":{"name":"型号","command":"nvram get model","timeout_seconds":5},"signal":{"name":"信号强度","command":"printf 90","timeout_seconds":5}}}}
```

失败为 `{"reply_to":1,"success":false,"error_code":"TEMPLATE_NOT_FOUND"}`。错误码还有 INVALID_TEMPLATE_REQUEST、TEMPLATES_UNAVAILABLE、TEMPLATE_TOO_LARGE。旧 Server 返回 ERROR/未知回复时 Probe 明确退出；不存在/格式非法不退回无模板。暂时连接或读写失败按 1/2/5/10/30 秒、之后固定 30 秒重试。模板回复硬上限 64 KiB，Server 同时遵守配置的 MaxControlPayload；成功回复携带 max_control_payload（1024～1048576；省略按 1 MiB），采集后的 REGISTER 超过它则明确退出，不反复发送超长注册。响应接收总期限 10 秒，连接每个候选地址等待最多 10 秒，主机名解析沿用系统 resolver。

属性是按 key 的 object，按 key 字典序采集。支持 serial/model/firmware/hostname/kernel/libc 六项现有可选属性及最多 32 项扩展字符串属性，总计最多 38 项。key 为 `[a-z][a-z0-9_]{0,63}`，device_id/arch/boot_id/probe_version/capabilities/template/attributes/collection_errors 保留；属性显示名称最多 128 bytes，指令最多 4096 bytes。服务端完整模板输入序列化最多 48 KiB。timeout_seconds 默认 5（管理输入省略或 0 归一化为 5），下发值为 1～30。

Probe 用 `/bin/sh -c` 执行，逐项复用有界执行器；整次采集 60 秒预算（超时后另有最多约 200ms 的 TERM/KILL 清理）。去除输出首尾空格、Tab、CR/LF；已有字段保持下表限制，扩展值 1～4096 bytes UTF-8、不能含 NUL；libc 继续 ASCII。非零退出、空值、截断/非法输出、超时、预算耗尽均不填值，记录固定原因。显式 `--hostname` 优先并跳过对应命令；选模板后未选中的可选属性省略，必需字段由原探针逻辑维护。命令只在启动采集一次，普通重连复用快照；模板修改/删除在下一次 Probe 启动生效。

REGISTER 可选增加以下字段，旧 Probe 可省略；附加结果必须同时包含 template：

```json
{"template":{"template_id":"…","name":"路由器","version":1},"attributes":{"signal":{"name":"信号强度","value":"90"}},"collection_errors":{"model":{"name":"型号","reason":"empty"}}}
```

template 包含非空 ID/名称（各最多 128 bytes UTF-8）与正 uint64 version。attributes 最多 32 项，只放扩展属性，值带显示名称和 value；六项已有属性继续放 REGISTER 原字段。collection_errors 最多 38 项，原因仅 command_failed/timeout/empty/invalid_output/budget_exhausted，不包含命令或 stderr；同一 key 不可同时成功和失败，扩展成功+失败总数最多 32。所有集合/成员遵守非 null、UTF-8、重复字段和类型校验。设备与 Session 保存独立完整快照，不按当前模板版本重新解释历史值。

启动未显式传 `--device-id`（别名 `--device_id`）时执行一次 `nvram get SN`，5 秒超时，去首尾空白后校验非空、单行、最多 128 bytes UTF-8，失败直接退出。显式 ID 优先且空值报错；不回退随机 ID。该默认值来源不改变 REGISTER 必选 device_id 或稳定主键语义。

### REGISTER

~~~json
{
  "device_id": "F3X36-20260905-001",
  "serial": "ABC123456",
  "model": "F3X36",
  "firmware": "v2.8.1",
  "probe_version": "1.0.0",
  "hostname": "Four-Faith",
  "arch": "mipsel",
  "kernel": "3.10.14",
  "libc": "uclibc",
  "boot_id": "7b5e...",
  "capabilities": ["exec", "file", "process", "tunnel", "info"]
}
~~~

#### REGISTER 字段契约

| 字段 | 必选 | 类型 | Protocol v1 约束 |
| --- | --- | --- | --- |
| device_id | 是 | string | 1-128 bytes UTF-8；设备稳定主键，不因重启、IP 或 TCP 重连改变 |
| serial | 否 | string | 0-128 bytes UTF-8；设备无序列号时省略，不使用 null 占位 |
| model | 否 | string | 0-128 bytes UTF-8；未知时省略 |
| firmware | 否 | string | 0-128 bytes UTF-8；未知时省略 |
| probe_version | 是 | string | 1-64 bytes UTF-8；Probe 软件版本标识 |
| hostname | 否 | string | 0-255 bytes UTF-8；未知时省略 |
| arch | 是 | string | 1-32 bytes ASCII；例如 x86_64、mipsel、arm、aarch64 |
| kernel | 否 | string | 0-128 bytes UTF-8；未知时省略 |
| libc | 否 | string | 0-64 bytes ASCII；例如 glibc、uclibc、musl；未知时省略 |
| boot_id | 是 | string | 1-128 bytes ASCII；优先使用系统 boot identity，无法取得时使用 Probe 生命周期 ID |
| capabilities | 是 | array[string] | 0-32 项；每项 1-32 bytes ASCII，建议使用小写 token；未知 capability 由 Server 忽略但可记录 |

补充规则：

- device_id 是管理平台中的稳定设备主键。Probe 必须从稳定配置或设备标识中获取，不得在每次启动时随机生成。
- boot_id 应优先使用系统真实 boot identity。在 Linux 上，如果 `/proc/sys/kernel/random/boot_id` 存在且可读，Probe 优先使用该值。
- 无法获取系统 boot identity 时，可以在 Probe 启动时生成生命周期 ID。该退化值只能标识 Probe 进程生命周期，不能严格证明整个设备发生了 reboot。
- capabilities 声明当前 Probe 版本支持的协议能力，不等同于系统自带命令列表。Protocol v1 已知 token 包括 `exec`、`file`、`process`、`tunnel`、`info`；Server 必须容忍未来未知 token。
- REGISTER 顶层未知字段按 JSON 前向兼容规则忽略。

### REGISTER_ACK 成功响应

~~~json
{
  "reply_to": 1,
  "success": true,
  "session_id": "sess_01J...",
  "heartbeat_interval": 30,
  "server_time": 1788571380,
  "max_control_payload": 1048576,
  "file_chunk_size": 65536
}
~~~

| 字段 | 必选 | 类型 | Protocol v1 约束 |
| --- | --- | --- | --- |
| reply_to | 是 | uint64 | 必须等于 REGISTER 帧的 message_id |
| success | 是 | boolean | 成功响应固定为 true |
| session_id | 是 | string | 1-128 bytes ASCII/UTF-8；Server 生成的不透明会话 ID |
| heartbeat_interval | 是 | integer | 10-300 秒；默认 30 秒 |
| server_time | 是 | integer | Unix seconds，>= 0 |
| max_control_payload | 是 | integer | 1024-1048576；不得高于 Protocol v1 普通 JSON 硬上限 1 MiB |
| file_chunk_size | 是 | integer | 1024-524288；默认 65536；不得高于 FILE_CHUNK 512 KiB 硬上限 |

每次 TCP 重连并重新 REGISTER 都生成新的 session_id。task_id 可以跨连接关联，但某个任务是否仍在执行必须由任务恢复策略明确决定，不能只靠旧 TCP 会话推断。

### REGISTER_ACK 失败响应

Server 拒绝注册时仍使用 REGISTER_ACK，设置 RESPONSE=1，并返回：

~~~json
{
  "reply_to": 1,
  "success": false,
  "error_code": "INVALID_REGISTER",
  "message": "device_id is required",
  "retry_after": 30
}
~~~

| 字段 | 必选 | 类型 | Protocol v1 约束 |
| --- | --- | --- | --- |
| reply_to | 是 | uint64 | 必须等于 REGISTER 帧的 message_id |
| success | 是 | boolean | 失败响应固定为 false |
| error_code | 是 | string | Protocol v1 已知值见下表 |
| message | 否 | string | 0-512 bytes UTF-8；用于诊断，不用于程序分支 |
| retry_after | 否 | integer | 1-3600 秒；省略时 Probe 使用自身重连退避 |

Protocol v1 注册失败码：

| error_code | 含义 | Probe 行为 |
| --- | --- | --- |
| INVALID_REGISTER | REGISTER 缺少必选字段、类型错误或字段超范围 | 关闭当前连接；修正配置前重复重试可能仍失败 |
| DEVICE_REJECTED | Server 明确拒绝该 device_id | 关闭当前连接；按 retry_after 或本地退避重试 |
| UNSUPPORTED_PROBE | Probe 版本或能力不满足 Server 最低要求 | 关闭当前连接；等待升级或配置变更 |
| SERVER_BUSY | Server 暂时无法接纳会话 | 关闭当前连接；按 retry_after 或本地退避重试 |

Server 发送失败 REGISTER_ACK 后应主动关闭当前 TCP 连接。Probe 收到失败响应后不得进入 ONLINE 状态。

## 心跳与在线状态

### HEARTBEAT

~~~json
{
  "uptime": 18372,
  "load1": 0.21,
  "free_memory": 12390400,
  "running_tasks": 2
}
~~~

| 字段 | 必选 | 类型 | Protocol v1 约束 |
| --- | --- | --- | --- |
| uptime | 是 | integer | >= 0，单位秒，优先表示系统 uptime |
| running_tasks | 是 | integer | 0-65535，当前正在执行的任务数 |
| load1 | 否 | number | >= 0；无法获取时省略 |
| free_memory | 否 | integer | >= 0，单位 byte；无法获取时省略 |

HEARTBEAT 只携带轻量状态，不承担完整设备资产上报。完整设备信息后续由 get_info 任务按需获取。

### HEARTBEAT_ACK

~~~json
{
  "reply_to": 27,
  "server_time": 1788571410
}
~~~

| 字段 | 必选 | 类型 | Protocol v1 约束 |
| --- | --- | --- | --- |
| reply_to | 是 | uint64 | 必须等于 HEARTBEAT 帧的 message_id |
| server_time | 是 | integer | Unix seconds，>= 0 |

### 心跳间隔与失联判定

- 任意通过校验的消息都刷新对应方向的 last_seen，心跳不是唯一活跃信号。
- 默认 heartbeat_interval 为 30 秒。
- REGISTER_ACK 可将 heartbeat_interval 设置为 10-300 秒的整数。
- Server 的 Session 失联阈值为 `3 * heartbeat_interval`。
- Probe 对 Server 的失联阈值同样为 `3 * heartbeat_interval`。
- 因此默认 30 秒心跳对应默认 90 秒失联阈值。
- Probe 达到失联阈值后主动关闭旧 socket，进入既定退避重连流程并重新 REGISTER。
- Server 达到失联阈值后将旧 session 标记为 disconnected；新连接必须重新 REGISTER 并生成新 session_id。

## 通用任务模型

### TASK

~~~json
{
  "task_id": "task_10001",
  "type": "exec",
  "created_at": 1788571420,
  "timeout": 30,
  "params": {
    "command": "uname -a",
    "cwd": "/tmp",
    "env": {}
  }
}
~~~

| 字段 | 必选 | 说明 |
| --- | --- | --- |
| task_id | 是 | 业务任务唯一标识，建议 UUID 或 ULID；重发同一任务时保持不变 |
| type | 是 | 任务类型 |
| created_at | 否 | Server 创建时间，Unix 秒 |
| timeout | 是 | 任务执行超时，单位秒；不建议使用 0 |
| params | 是 | 各任务类型自己的参数对象 |

### TASK_ACK

~~~json
{
  "reply_to": 101,
  "task_id": "task_10001",
  "accepted": true,
  "state": "queued"
}
~~~

TASK_ACK 只表示 Probe 已收到并接受任务，不表示执行成功。建议收到 TASK 后立即 ACK，再异步执行。

TASK_ACK 中 accepted=false 表示任务已经被 Probe 最终拒绝。此时 Probe 不再发送 TASK_RESULT，Server 将该业务任务状态记录为 rejected。

~~~json
{
  "reply_to": 101,
  "task_id": "task_10001",
  "accepted": false,
  "state": "rejected"
}
~~~

### TASK_RESULT

~~~json
{
  "task_id": "task_10001",
  "status": "success",
  "started_at": 1788571421,
  "finished_at": 1788571421,
  "exit_code": 0,
  "stdout": "Linux Four-Faith ...\n",
  "stderr": "",
  "truncated": false,
  "result": {}
}
~~~

| status | 含义 |
| --- | --- |
| success | 任务完成且业务执行成功 |
| failed | 任务已执行但失败 |
| timeout | 超过任务 timeout，Probe 已尝试结束相关执行 |

Protocol v1 Phase 1 禁止发送 status=rejected。拒绝由 TASK_ACK accepted=false 完整表达。Phase 1 不实现 TASK_CANCEL，因此也不发送 status=cancelled。

### 第一版任务类型

| type | 作用 | 关键 params |
| --- | --- | --- |
| exec | 执行一次非交互命令 | command, cwd, env |
| upload | Server 向设备发送文件 | transfer_id, remote_path, size, sha256, mode, overwrite |
| download | 从设备拉取文件 | transfer_id, remote_path, result_name |
| start_process | 启动长期进程 | command, args, cwd, env, pid_file 可选 |
| stop_process | 停止由平台启动或已知的进程 | process_id，或 pid 加 signal |
| open_tunnel | 创建临时远程数据通道 | kind, local_host, local_port, tunnel_server params |
| close_tunnel | 关闭临时通道 | tunnel_id |
| get_info | 获取详细设备或系统信息 | sections，例如 network、process、storage、tools |

exec 用于一次性非交互命令，不是交互式 Shell。复杂交互应通过 open_tunnel 建立 SSH 或 Telnet 独立会话。

### upload TASK 参数

local_asset、tool_id 和 asset 等 Management Server 内部资产概念不得传给 Probe。Server Service 负责根据内部资产标识找到实际文件，生成 transfer_id，再创建设备 upload TASK。

Probe 接收的 upload params 至少包含：

~~~json
{
  "transfer_id": "96d3e6a3-78d2-4e94-a5f0-bf0bdfd42b61",
  "remote_path": "/tmp/ai-tools/tcpdump",
  "size": 842112,
  "sha256": "...",
  "mode": "0755",
  "overwrite": true
}
~~~

upload TASK 与后续 FILE_BEGIN 中重复出现的文件元数据必须使用相同字段名和相同值，不能形成两套文件元数据。

### TASK_CANCEL

0x13 保留为后续协议扩展消息号。Protocol v1 Phase 1 不实现 TASK_CANCEL，不设计 Cancel ACK、取消状态机或进程终止语义。

Phase 1 实现如果收到 0x13，应按当前版本未支持的操作处理，不得报告取消已经受理或完成。后续如需任务取消，必须单独评审并设计。

## 并发 幂等与任务状态

~~~text
TASK received
    |
    +-- invalid or unsupported --> TASK_ACK accepted=false
    |
    +-- accepted
         |
       QUEUED
         |
       RUNNING
         |
         +-- success ------------> SUCCESS
         +-- error --------------> FAILED
         +-- timeout ------------> TIMEOUT
~~~

- TCP Reader 只负责读帧和解析，不在 Reader 线程或协程中执行耗时任务。
- Task Dispatcher 将任务投递给 worker；不同 task_id 的结果允许乱序返回。
- 建议设置最大并发数，例如默认 4；文件传输和 Tunnel 创建可以单独设置并发额度。
- 在同一个 Probe 进程生命周期内，即使 TCP 断开并重新连接，同一个已经接受的 task_id 也不得再次执行副作用操作。
- 对 RUNNING task_id，Probe 返回当前状态；对已完成且仍在本地幂等缓存范围内的 task_id，Probe 返回已有结果，不重新执行。
- 幂等缓存容量属于实现配置；Phase 1C 保留全部已接受身份与结果，容量不足时拒绝新任务，具体边界见本文末尾。
- Probe 自身重启后的 task_id 缓存、任务恢复和未上报结果持久化仍为 TBD。
- message_id 只解决一条 TCP 会话中的传输追踪；task_id 才是业务幂等键。

## 文件传输协议

文件内容不放入 JSON，也不使用 Base64。控制消息使用 FILE_BEGIN、FILE_END 和 FILE_ACK，数据使用 FILE_CHUNK 二进制帧。

upload 和 download 的 transfer_id 均由 Management Server 在创建 TASK 时生成，并在 TASK 阶段确定。同一个文件任务后续的 FILE_BEGIN、FILE_CHUNK、FILE_END 和 FILE_ACK 必须使用同一个 transfer_id。

Protocol v1 的 transfer_id 统一使用 UUID。JSON 中使用标准 canonical UUID string；FILE_CHUNK 中使用对应 UUID 的 RFC 4122 16-byte 原始表示。禁止使用 Windows GUID mixed-endian 内存布局作为 wire format，也不得同时支持 UUID 和 ULID 两套 wire encoding。

### FILE_BEGIN

~~~json
{
  "transfer_id": "96d3e6a3-78d2-4e94-a5f0-bf0bdfd42b61",
  "task_id": "task_10020",
  "direction": "server_to_device",
  "name": "tcpdump",
  "remote_path": "/tmp/ai-tools/tcpdump",
  "size": 842112,
  "sha256": "...",
  "chunk_size": 65536,
  "mode": "0755",
  "overwrite": true
}
~~~

### FILE_CHUNK 二进制 Payload

| 偏移 | 长度 | 字段 | 说明 |
| ---: | ---: | --- | --- |
| 0 | 16 | transfer_id | canonical UUID 对应的 RFC 4122 16-byte 原始表示 |
| 16 | 8 | offset | 该块在文件中的绝对偏移 uint64 |
| 24 | 4 | data_len | 本块实际数据长度 uint32 |
| 28 | N | data | 原始文件字节 |

### upload 成功时序

~~~text
Server -> TASK upload
Probe  -> TASK_ACK accepted

Server -> FILE_BEGIN
Probe  -> FILE_ACK ready

Server -> FILE_CHUNK ...
Server -> FILE_END

Probe  -> FILE_ACK done
Probe  -> TASK_RESULT success
~~~

如果 Probe 拒绝 FILE_BEGIN，应返回 FILE_ACK failed，随后返回 TASK_RESULT failed。文件传输中途断开时，旧 transfer_id 失败；第一版重新发起新的 task_id 和 transfer_id，不实现 resume。

### download 成功时序

~~~text
Server -> TASK download
Probe  -> TASK_ACK accepted

Probe  -> FILE_BEGIN
Server -> FILE_ACK ready

Probe  -> FILE_CHUNK ...
Probe  -> FILE_END

Server -> FILE_ACK done
Probe  -> TASK_RESULT success
~~~

如果 Server 无法接收文件，应返回 FILE_ACK failed，Probe 最终返回 TASK_RESULT failed。

### FILE_END 与 FILE_ACK 示例

~~~json
{
  "transfer_id": "96d3e6a3-78d2-4e94-a5f0-bf0bdfd42b61",
  "size": 842112,
  "sha256": "..."
}
~~~

上例为 FILE_END。

~~~json
{
  "reply_to": 208,
  "transfer_id": "96d3e6a3-78d2-4e94-a5f0-bf0bdfd42b61",
  "status": "done",
  "received": 842112,
  "sha256_ok": true
}
~~~

上例为 FILE_ACK。

- 小文件可以直接走控制连接。
- 大文件或持续 pcap 可以在后续版本扩展为专用文件数据连接，不改变 TASK 业务模型。
- FILE_CHUNK 在 Phase 1 必须使用连续 offset；不接受乱序、重叠、重复块或空洞（ADR-017 取代原建议顺序发送的表述）。
- download 与 upload 复用同一套 FILE 消息，方向相反。

### 文件并发与背压

Protocol v1 Phase 1 规定一个 Probe 控制连接同时最多只有一个 active file transfer。其他已接受文件任务采用有界 FIFO，队列容量属于实现配置，满时拒绝新文件任务（用户在 Phase 1D 启动时明确确认，见 ADR-016）。不得并行向同一控制连接写入多个文件流。队列晋升的规范性 wire 时序见本文末尾 P1-P5。

文件传输期间，REGISTER、HEARTBEAT、TASK、TASK_ACK、TASK_RESULT、EVENT 和 ERROR 等控制消息必须具有高于 FILE_CHUNK 的发送优先级。

实现必须使用单一 socket writer 或等价的序列化机制，禁止多个 worker 无序同时写同一个 socket。建议设置 High Priority Control Queue 和 Low Priority File Queue；Writer 每发送一个 FILE_CHUNK 后必须重新检查高优先级控制队列。

TCP 自身负责字节流背压。Protocol v1 Phase 1 不设计 FILE_CHUNK 单块 ACK 或复杂 sliding window。

## Tunnel 与控制链路

~~~text
Server -- TASK open_tunnel --> Probe
                                 |
                       separate data TCP connection
                                 |
                         Tunnel or Relay Server
                                 |
                      SSH / Telnet / Web traffic

Control TCP: registration, heartbeat, task, event, error and necessary file transfer
Tunnel TCP : actual remote interactive traffic
~~~

不得把 SSH、Telnet、Web 的持续字节流封装成 TASK 或塞入主控制 TCP。一个卡住的会话或大流量请求不能影响设备心跳和管理命令。

## 断线重连与任务恢复

| 场景 | Probe 行为 | Server 行为 |
| --- | --- | --- |
| TCP 短暂断开 | 关闭旧 socket，进入退避重连；同一 Probe 进程内继续维护已接受 task_id 的状态和去重 | 将旧 session 标记 disconnected，等待新 REGISTER |
| 重连成功 | 重新 REGISTER，获得新 session_id；上报 running_tasks 摘要 | 将新连接绑定到同一 device_id |
| 一次性 exec 正在运行 | 同一 Probe 进程内继续执行并缓存结果，连接恢复后补报 RESULT | 按 task_id 接受迟到结果 |
| 文件传输中断 | 第一版将旧 transfer_id 标记 failed，重新发起新的 task_id 和 transfer_id | 结束旧 transfer_id；第一版不实现 resume |
| Tunnel 数据连接断开 | 结束本条流，不自动重连，不影响控制TCP或其他流 | 维护有效时下次客户端连接创建新流 |
| Probe 进程重启 | task_id 缓存、未上报结果和任务恢复规则仍为 TBD | 不得仅根据旧 TCP Session 推断任务状态 |

## EVENT 与 ERROR

### EVENT

~~~json
{
  "event": "process_exit",
  "time": 1788571500,
  "data": {
    "process_id": "proc_1003",
    "exit_code": 1
  }
}
~~~

EVENT 用于不属于某个立即请求响应链的异步信息，例如长期进程退出、Tunnel 断开和磁盘空间不足。

### ERROR

~~~json
{
  "reply_to": 10231,
  "code": "INVALID_PAYLOAD",
  "message": "field task_id is required"
}
~~~

上例是对具体请求的直接 ERROR 响应，因此设置 RESPONSE=1，并用 reply_to 关联被响应帧。不是具体请求直接响应的 ERROR 不设置 RESPONSE，也不包含 reply_to。

| 错误码 | 典型含义 |
| --- | --- |
| BAD_MAGIC | 包头 magic 不正确，通常直接关闭连接 |
| UNSUPPORTED_VERSION | 协议版本不支持 |
| PAYLOAD_TOO_LARGE | payload_len 超过本端上限 |
| INVALID_JSON | 控制 Payload 不是合法 JSON |
| INVALID_PAYLOAD | 字段缺失或类型错误 |
| UNSUPPORTED_TYPE | 未知消息类型 |
| TASK_NOT_FOUND | 取消或查询的 task_id 不存在 |
| TRANSFER_ERROR | 文件传输失败 |

## 协议实现边界

Probe 的建议职责：

~~~text
Probe
├─ ConnectionManager
│  ├─ connect / reconnect / heartbeat
│  └─ session state
├─ ProtocolCodec
│  ├─ frame encode / decode
│  └─ JSON / binary payload
├─ MessageRouter
├─ TaskManager
│  ├─ task registry / idempotency
│  └─ worker pool / timeout / cancel future
├─ ExecManager
├─ FileManager
├─ ProcessManager
├─ TunnelManager
├─ DeviceInfoCollector
└─ LocalStateStore
~~~

ConnectionManager 不处理 exec 或 Tunnel 的具体业务。TaskManager 负责任务生命周期、并发、timeout 和 task_id 去重。任务Manager通过统一结果对象返回TASK_RESULT；Phase 4 TunnelManager独立管理控制Session内的数据worker，通过TUNNEL_STATUS报告建流失败，不进入Task缓存。LocalStateStore只允许保存必要的小规模持久状态。

device_id、Server 地址和基础配置是本地持久化的基本候选。未上报 TASK_RESULT、已完成 task_id、正在执行的任务和文件传输状态是否持久化仍为 TBD。

Server 的建议职责：

~~~text
Management Server
├─ TCP Gateway
│  ├─ connection registry: device_id -> connection
│  └─ frame codec
├─ Device Session Manager
├─ Task Service
│  ├─ create / dispatch / wait / query
│  └─ task result store
├─ File / Tool Service
├─ Tunnel Service
├─ Device Inventory
└─ API / MCP Adapter 后续阶段
~~~

第一版 Management Server 可以是模块化单体，但应保留上述职责边界。

## 关键时序

### 设备上线并执行命令

~~~text
Probe                         Server
  |---- TCP CONNECT ------------>|
  |---- REGISTER --------------->|
  |<--- REGISTER_ACK ------------|
  |                              |
  |<--- TASK exec ---------------|
  |---- TASK_ACK ---------------->|
  |       execute                |
  |---- TASK_RESULT ------------>|
  |                              |
  |---- HEARTBEAT -------------->|
  |<--- HEARTBEAT_ACK -----------|
~~~

### 上传工具并抓包

该时序的工具投放和文件回收已由 Phase 3 管理端内部 Service 复用既有 upload/download 实现；exec 仍须调用方单独发起。AI 编排和 start_process 仍为后续能力，不能从此示意推断已实现。Repository 资产、版本、产物和兼容规则不进入 wire。

~~~text
Management or AI
   | create upload task
   v
Server -- TASK upload --------------------> Probe
Server <- TASK_ACK accepted --------------- Probe
Server -- FILE_BEGIN ---------------------> Probe
Server <- FILE_ACK ready ------------------ Probe
Server -- FILE_CHUNK / FILE_END ----------> Probe
Server <- FILE_ACK done ------------------- Probe
Server <- TASK_RESULT success ------------- Probe
   |
   +-- TASK exec or start_process --------> Probe
   |
   +-- TASK download ---------------------> Probe
      <- TASK_ACK accepted ---------------- Probe
      <- FILE_BEGIN ----------------------- Probe
      -- FILE_ACK ready ------------------> Probe
      <- FILE_CHUNK / FILE_END ------------ Probe
      -- FILE_ACK done -------------------> Probe
      <- TASK_RESULT success -------------- Probe
~~~

## Phase 1 验收基线

1. Probe 能主动连接 Server 并完成 REGISTER 与 REGISTER_ACK。
2. 拔网线或断开 TCP 后，Probe 能自动重连并形成新的 session_id。
3. Server 能对在线设备下发 exec，收到 TASK_ACK 和 TASK_RESULT。
4. 同时下发至少 3 个不同任务，允许乱序完成且结果不串 task_id。
5. 重复发送同一 task_id 不会重复执行副作用任务。
6. Server 能上传一个二进制文件到 /tmp，并校验 size 和 sha256。
7. Server 能从设备下载一个文件，并校验 size 和 sha256。
8. 文件传输或任务执行期间，心跳和其他控制消息仍能正常处理。
9. 非法 magic、超大 payload 和非法 JSON 不会导致 Probe 崩溃。
10. 协议日志能打印 message_id、task_id 和 transfer_id。
11. message_id、reply_to、RESPONSE、BINARY 和 MORE 的编码符合 Protocol v1 规则。
12. transfer_id 的 JSON 和 FILE_CHUNK 表示符合 canonical UUID 与 RFC 4122 wire format。
13. 同一个 Probe 进程生命周期内，TCP 重连后重复 task_id 不会重新执行副作用操作。
14. 一个控制连接最多存在一个 active file transfer，且 FILE_CHUNK 不会饿死高优先级控制消息。

## 已确认的协议决策

| 决策 | 结论 |
| --- | --- |
| 底层通信 | TCP 长连接，Probe 主动连接 |
| 消息边界 | 20-byte 固定包头加 payload_len |
| 控制载荷 | JSON UTF-8 |
| 文件数据 | FILE_CHUNK 原始二进制，不使用 Base64 |
| 任务确认 | TASK_ACK 与 TASK_RESULT 分离 |
| 并发模型 | task_id 独立，多任务可并发，结果可乱序 |
| 幂等 | task_id 作为业务幂等键 |
| Tunnel 流量 | 独立数据连接，不进入控制 TCP |
| Probe 职责 | 通用执行、文件、进程、Tunnel 原语，不承载 AI 排障逻辑 |

## Protocol Review 状态

Phase 0 最终细化已经解决 message_id、reply_to 与 RESPONSE、BINARY 与 MORE、transfer_id wire format、TASK 拒绝、TASK_CANCEL 的 Phase 1 范围、upload 与 download 时序、文件背压、JSON 基础规则、boot_id、Probe 进程生命周期内幂等、心跳超时，以及 REGISTER / REGISTER_ACK / HEARTBEAT / HEARTBEAT_ACK 的正式字段契约和注册失败响应。以上内容已经移入本文规范性章节，不再属于 TBD。

### 仍为 TBD

| 主题 | 当前边界 |
| --- | --- |
| 用户与 Probe 身份认证、TLS、链路加密、权限、租户、密钥轮换和审计 | 本轮不设计；生产部署前必须形成正式安全设计 |
| Probe 重启后的任务恢复 | task_id 缓存、未上报结果、运行中任务和文件状态是否持久化尚未决定 |
| Server 重启后的任务恢复 | 任务、Session 和传输状态的恢复策略尚未决定 |
| BINARY/type 不一致等协议错误的严重程度 | 返回 ERROR 还是直接关闭连接的逐错误矩阵尚未决定 |
| Probe 进程重启后的缓存 | Phase 1C 进程内有界且不淘汰；跨进程持久化仍未决定 |

固定三服务Tunnel数据面由Phase 4章节和ADR-021/022定义。Phase 5只增加外部HTTP/WebSocket Adapter（API.md/ADR-023），不新增或改变任何本文件wire字段和时序；HTTP事件不是Probe EVENT重放。数据库存储、OpenAPI生成流程与通用Tunnel仍为后续边界。

### Phase 1C 重复任务与跨连接结果契约（2026-09-05）

状态：**已由用户明确确认，规范性契约；决策见 ADR-015。** 启动检查曾发现以下缺口，用户随后确认建议方案。历史原因与影响见 [PHASE1AB_REVIEW.md](PHASE1AB_REVIEW.md)。

| 收到的 TASK | Probe 响应 |
| --- | --- |
| 首次接受 | 登记 task_id 后发送 accepted=true/state=queued 的 TASK_ACK，异步执行；ACK 写失败也保留已接受任务 |
| 重复 QUEUED | accepted=true/state=queued 的 TASK_ACK，不再次入队 |
| 重复 RUNNING | accepted=true/state=running 的 TASK_ACK，不再次执行 |
| 重复已完成任务 | 先发送 accepted=true/state=success/failed/timeout 的 TASK_ACK，再发送原缓存 TASK_RESULT |
| 同 task_id 的执行内容冲突 | ERROR/INVALID_PAYLOAD 响应本次 TASK；保留原任务，绝不重执行，不能用 accepted=false 拒绝原业务任务 |
| 新任务无效、不支持或容量不足 | accepted=false/state=rejected 的 TASK_ACK；不再发送该任务的 RESULT |

每个 ACK 的 reply_to 都引用本次 TASK 帧，设置 RESPONSE；RESULT 始终不设 RESPONSE。新任务一经接受，ACK 的网络发送是否成功都不改变接受事实。

执行身份比较采用已解析的 type、timeout、params.command、params.cwd、params.env。env 键顺序无关，省略 cwd/env 与空字符串/空对象等价；created_at 与未知扩展字段不参与比较。Protocol v1 不引入 canonical JSON。

每次新会话 REGISTER_ACK 成功后，Probe 补报全部缓存完成结果，包括旧连接上写成功但没有业务接收确认的结果；排队与运行中 exec 不因 TCP 断开终止，完成后通过可用连接发送。补报不伪造旧 TASK_ACK，不增加 RESULT_ACK 消息。running_tasks 使用既有 HEARTBEAT，不新增 REGISTER 字段。

Server 按 device_id/task_id 接受已派发任务的迟到结果，包括缺少 ACK 或派发结果不确定的任务。未知 task_id、错误 device_id、未派发或已 rejected 的任务不能被 RESULT 改写。重复 RESULT 要求已定义结果字段全部相同（包括 result 对象），不能仅比较 status；对象键顺序无关。相同结果幂等成功，冲突返回 ERROR/INVALID_PAYLOAD，保留首个终态。迟到或重复 ACK 不得把 running 回退为 queued，也不得回退已有终态；完成态 ACK 本身不替代 RESULT。

每次重发 TASK 必须保持原执行内容，并分别记录 session_id/message_id/task_id。ACK 必须匹配这组三元组，不能跨连接比较裸 message_id。Server 不自动生成替代 task_id，也不根据 boot_id 推断同一 Probe 进程（boot_id 可能属于整机启动）。

缓存采用有界容量并保留全部已接受身份与结果，容量不足时拒绝新任务，不通过淘汰允许重执行。容量和并发数是实现配置。

### Phase 1C 当前实现配置与长度限制

当前默认 4 workers、最多 128 个已接受任务、8 MiB 身份/结果计费预算。接受时为结果预留当前协商 max_control_payload，并按两倍输入字节计费身份；完成后释放未使用的结果预留。该预算不包含线程栈、容器节点及运行时临时输出，不是进程 RSS 上限。

缓存 RESULT 按任务接受时的协商上限编码一次。后续会话若把 max_control_payload 降至该缓存帧以下，Probe 保留原结果并延后补报，不改写或发送超限帧；收到该任务的重复 TASK 时在完成态 ACK 后返回 ERROR/PAYLOAD_TOO_LARGE。恢复足够大的协商上限后继续补报。这个长度限制不能被解释为可以重新执行任务。Probe / Server 进程重启后的恢复与持久化仍为 TBD。

### Phase 1D 文件互操作规范（2026-09-05）

**P1-P5 已由用户明确确认，为规范性契约（Accepted ADR-017）。** 用户另明确：ready 必须省略 sha256_ok，done 固定 true，failed 固定 false。启动检查基线为 main `5030322b58fcbb07cae2f3256a71eb9a750a479a`。以下问题说明保留设计原因，决定正文具有规范效力。

#### P1：FIFO 的归属、晋升与握手

问题：TASK_ACK accepted/queued 仅表达接收，未说明 upload 何时可 BEGIN、queued upload 如何获知轮到自己。现有 Task Service 拒绝同一派发的冲突 ACK，不能额外发送 running ACK 来暗示晋升。

决定：Probe 按首次接受文件 TASK 的顺序维护 upload/download 共用的有界 FIFO。重复任务不占新槽位；队列满只拒绝新任务；正在执行的文件任务不占等待槽位。不同 exec 不受该 FIFO 限制。

- upload：Server 收到首次 accepted ACK 后可发送一次 FILE_BEGIN；Probe 对排队任务仅登记并验证有界元数据，不打开目标、不发送 ready。成为队首 active 后才准备临时文件并回复 FILE_ACK ready，其 reply_to 引用原 FILE_BEGIN。未收到 ready，Server 不发送 CHUNK/END。
- download：Probe 仅在任务成为队首 active 后准备源文件、计算元数据并发送 FILE_BEGIN；Server 回复 ready 后才发送 CHUNK/END。
- active 从队首任务开始准备文件时算起，包括等待 BEGIN/ACK、计算摘要、传输和最终确认。Probe 排入当前任务终态 RESULT 后，才晋升下一文件任务。各方向 writer 保持该终态通知和后续文件控制消息的因果顺序。
- TASK 的每次派发仍只返回一个 ACK；查询状态用既有同 task_id 重发，不新增晋升消息。未在相同连接接受的任务不能靠 FILE_BEGIN 创建任务；重复 BEGIN 不是重传或重开文件入口。

影响：沿用既有成功时序和消息号；明确 queued BEGIN 不等于 active transfer，不需要同一 TASK 的第二个 ACK。

#### P2：正式字段、单位与校验口径

问题：download TASK、ready/failed ACK 缺少字段表；file_chunk_size 是否包含 28-byte 前缀及其与 max_control_payload 的关系尚不明确。

共同字段规则：transfer_id 为小写 canonical UUID；size/received 为 0..INT64_MAX 的 JSON integer；sha256 为 64 个小写十六进制字符；字符串遵循现有 UTF-8/NUL 校验。remote_path 为非空绝对路径，最长 4096 UTF-8 bytes；name/result_name 为 1..255 bytes 的文件名标签，不含斜杠、反斜杠、NUL，且不等于 `.` 或 `..`。名称不用于让 Probe 推导 Server 本地路径。

| 消息 | 必选字段与含义 |
| --- | --- |
| upload TASK.params | transfer_id、remote_path、size、sha256、mode、overwrite；mode 固定四位八进制 `0[0-7]{3}`，overwrite 为 boolean，不给省略值隐式默认 |
| download TASK.params | transfer_id、remote_path、result_name；不含 Server 本地目标路径 |
| FILE_BEGIN 共用字段 | transfer_id、task_id、direction、name、remote_path、size、sha256、chunk_size |
| upload FILE_BEGIN | direction=server_to_device，另须 mode/overwrite；与 TASK 重复字段值完全一致 |
| download FILE_BEGIN | direction=device_to_server；name=TASK.result_name，remote_path 与 TASK 一致；size/sha256 由 Probe 流式预读源文件获得；不携带 mode/overwrite |
| FILE_END | transfer_id、size、sha256；与 BEGIN 完全一致 |
| FILE_ACK ready | reply_to、transfer_id、status=ready、received=0；回复 BEGIN，必须省略 sha256_ok（包括 false/null 也不得出现） |
| FILE_ACK done | reply_to、transfer_id、status=done、received、sha256_ok=true；回复 END，received=size，仅在接收文件校验并发布后发送 |
| FILE_ACK failed | reply_to、transfer_id、status=failed、received、sha256_ok=false；可附 0..512 bytes message，诊断文字不作为分支条件 |

FILE_BEGIN/END flags=0；ACK 仅 RESPONSE；CHUNK 仅 BINARY。file_chunk_size、BEGIN.chunk_size 与 512 KiB 上限均指原始 data 字节数，CHUNK.payload_len=28+data_len；BEGIN.chunk_size 为 1..协商 file_chunk_size。JSON 受 max_control_payload 限制，CHUNK 独立受 28+file_chunk_size 限制。

文件 TASK_RESULT 保留现有全部通用字段：success 时 exit_code=0，failed/timeout 时 exit_code=-1；stdout 为空，stderr 可为有界诊断文字，truncated=false。result 必含 transfer_id；成功时另含校验过的 size、sha256。started_at 为晋升 active 时间；未晋升即因断线失败时 started_at=finished_at（终止时间）。终态编码一次缓存并按 ADR-015 重放。

影响：两端可独立校验；允许空文件；既有 JSON 帧上限不会错误限制二进制文件块。download 源文件预读摘要仍使用固定大小缓冲，不整文件入内存；发送期间再次流式计算摘要，源内容变化时失败。

#### P3：offset 与传输阶段失败

问题：当前“建议顺序发送”不能决定接收端是否必须实现随机写、空洞、重叠和重复块。中途磁盘失败、发送方读取失败没有确定的通知和收敛方式。

决定：Phase 1 强制 offset 等于已连续收到的字节数，data_len>0，精确匹配 payload 剩余长度，不超过 chunk_size 或剩余 size。空文件不发送 CHUNK。不接受乱序、重叠、重复块或空洞。

- 可关联且格式正确的 BEGIN 被业务拒绝：FILE_ACK failed 回复 BEGIN，Probe 形成 TASK_RESULT failed。END 的 size/摘要校验失败：FILE_ACK failed 回复 END，清理临时文件并形成失败任务。
- CHUNK 阶段出现非法 offset、写失败、发送端读失败、错误 transfer_id、非法 FILE 状态/格式或传输超时：可关联触发帧时尽力发送 RESPONSE ERROR（INVALID_PAYLOAD 或 TRANSFER_ERROR，reply_to 引用触发帧）；无触发帧的本地错误发送无 RESPONSE 的 ERROR/TRANSFER_ERROR；随后关闭当前 TCP，按 P5 统一失败收敛。不依赖 ERROR 一定送达。
- 不为成功 CHUNK 发送 ACK；不新增 abort 消息；中途错误不在原连接继续发送下一文件，避免在途 CHUNK 与下一传输交错。保持原 Phase 1A/B/C 的错误处理分支不变。

影响：已确认的严格顺序限制取代本文件旧的宽松 offset 表述。以断连处理传输中途失败会使排队文件任务一并失败，但 exec 保持 Phase 1C 的跨 TCP 执行与补报行为。

#### P4：文件身份与重复副作用

问题：ADR-015 身份比较仅列 exec 字段，未包含文件元数据；不同 task_id 重用同一 transfer_id 也未定义。

决定：文件身份比较已解析的 type、timeout 和 P2 对应 TASK.params 全部字段；created_at 和未知扩展字段不参与比较。省略必选文件字段是无效请求。旧 ID 同内容沿用 queued/running/终态 ACK 与缓存 RESULT，不再次 BEGIN、不重开源或目标；同 ID 不同内容返回 ERROR/INVALID_PAYLOAD，保留旧身份。

同一 Probe 进程内已接受的 transfer_id 不得绑定另一个 task_id；新任务如此重用时 TASK_ACK rejected，原任务不变。身份/终态和 transfer_id 绑定均有界保留、不淘汰，满后拒绝新任务。Server 的同 ID 重发同样不得重新覆盖已提交的下载文件。

影响：扩展 ADR-015 的文件任务比较字段，保持其 exec 比较及结果重放语义；不会因本地源文件后来改变而重新执行旧任务。

#### P5：排队断线、timeout 与最终发布边界

问题：旧协议仅提“当前 transfer 失败”，没有说明已接受但排队中的任务是否跨连接继续；文件已发布但 done ACK 丢失时，不能简单把“failed”解释为目标文件一定不存在。

决定：文件 TASK.timeout 是晋升 active 起的总时限，包括源摘要预读、等待 BEGIN/ACK、收发、校验和发布，排队时间不计入。Probe 掌握业务 timeout；Server 可有独立的本地 I/O 等待上限，到期断连，不能伪造 Probe RESULT。

- TCP 断开时，同连接已接受、尚未形成终态的 active 与 queued 文件任务全部终止。active 超过任务时限记 timeout；其余因断线终止记 failed。Probe 保留身份与 RESULT，在新会话按 ADR-015 补报；同 task_id 重发只查询原任务，重新传文件须使用新 task_id 和 transfer_id。
- 接收端先写目标同目录的独占临时文件，校验 size/SHA-256 后才发布。upload 应用指定 mode；overwrite=false 时目标已存在必须失败，发布不得覆盖并发新建的目标；overwrite=true 只在完整校验后替换目标。不自动创建父目录。download 的 Server 目标路径与覆盖策略是本地 Service 参数，不传给 Probe。
- 上传以 Probe 成功发布为不可逆提交点：在同一本地完成流程中记录 success，再通知 FILE_ACK done 和缓存 TASK_RESULT。发布后网络发送失败仍保留成功结果，重连补报；不删除完整目标，不重新执行。
- 下载以 Server 成功发布为本地提交点；Probe 收到 done 后形成 success。若 Server 已提交，但 done 丢失导致连接断开，Probe 缓存 failed，Server 保留已校验的完整文件和本地提交事实，接受 failed RESULT；不能把它伪造成 success，也不能回滚/重复下载。文件完整性成功与任务确认失败必须可区分。

影响：明确 no-resume 的排队范围和失败副作用边界；不引入两阶段提交、RESULT_ACK、持久化或进程重启恢复。控制优先在完整帧边界生效，已经写出的 CHUNK 字节不能被后来到达的控制帧抢占；每块后重查控制队列，Reader 与文件 I/O 分离。

### Phase 1D 当前实现配置

Probe ClientConfig.file_queue_capacity 默认 8（等待槽位，不含一个 active），并继续受 TaskManager 全局 128 身份、8 MiB 计费预算限制。较大缓存 RESULT 仍按 Phase 1C 规则延后补报。文件任务接受时检查终态元数据可装入协商 JSON 上限；文件内容不计入身份缓存。

文件 I/O 在独立 worker 执行；每条连接接收文件邮箱最多 16 帧。chunk_size 默认 64 KiB，硬上限 512 KiB（不含 28-byte 前缀）。两端发送缓冲目标 64 KiB，OS 可调整实际容量；控制优先只在完整帧边界生效，不承诺抢占 TCP 已排入字节或不可中断的内核 I/O。Probe deadline watcher 负责超时断开 socket。

无覆盖发布要求目标文件系统支持同目录 hard link；不支持时任务失败，不退化为覆盖或复制不完整文件。Windows Server 使用本地路径，Probe remote_path 使用 Linux 绝对路径。进程崩溃后的临时文件清理、状态恢复与持久化仍未实现。

## Phase 4 Maintenance 控制与数据协议

状态：Accepted ADR-021 / ADR-022。固定三服务的本节取代前文有关 Tunnel 数据面尚未展开的描述；通用 Tunnel、平台认证和加密仍不在范围内。REGISTER.capabilities 新增 `tunnel`；不声明该能力时 Server 拒绝新 Maintenance。

0x40 CONNECT、0x41 CLOSE、0x42 STATUS 均为 UTF-8 JSON object，flags=0（无 RESPONSE/reply_to），沿用控制连接双向 message_id 顺序；不是 TASK，不存 task_id，不跨 Session 补报/重发。CONNECT/CLOSE 最多4096bytes，仍受协商控制载荷上限限制。

| CONNECT必选字段 | 类型与约束 |
| --- | --- |
| session_id | 必须精确等于本控制Session REGISTER_ACK的session_id |
| maintenance_id / connection_id | 各32个小写hex，Server分别以128-bit安全随机值生成，不重用 |
| service | 仅web、ssh、telnet，Probe内部固定映射127.0.0.1:80/22/23；wire不得指定target host/port |
| token | 64个小写hex，256-bit安全随机值，只用于本条流的一次配对 |
| data_host / data_port | 从Probe可达的数值IPv4/IPv6字符串 / 1～65535整数；配置域名由Server在Create解析，Probe不做DNS |
| timeout_ms | 1～60000整数，总建流期限，包括本地连接、data连接和握手，默认10000 |
| idle_ms | 1～86400000整数，整条连接无读写进展的空闲期限，默认86400000；绝对租期优先 |

Probe先连接本地目标，再主动连接data_host:data_port；使用独立socket，不在控制帧承载数据。本地连接失败上报local_unavailable，data连接/握手失败上报data_failed，worker配额满上报busy；STATUS必选maintenance_id、connection_id、state。Server按当前来源Session与pending流校验；已关闭、迟到、不属于该Session的状态不改变当前连接或其他服务。错误state/缺少身份为协议错误。

CLOSE必选maintenance_id、connection_id；后者为空字符串表示取消整个Maintenance，否则仅取消指定流。未知或重复CLOSE幂等无操作。Probe立即标记取消，worker退出前关闭本地及data socket；控制Session结束也取消并join全部Tunnel worker。既有exec跨TCP继续、文件断线失败的语义保持。CLOSE不依赖ACK完成Server本地撤销；Released是Server资源释放事实，不声称收到Probe释放ACK。

独立data TCP不使用RMP1帧。第一段精确132bytes：`RMT1`（4 ASCII bytes）+ maintenance_id（32 ASCII bytes）+ connection_id（32 ASCII bytes）+ token（64 ASCII bytes）。无分隔符、换行或JSON；Server精确读取132bytes，不吞掉后续业务字节。Server在配对锁内校验maintenance仍有效、创建时Session未撤销、connection处于pending且未超时、token完全匹配，再原子消费token；成功回复一个字节0x01，其后即双向原始TCP。非法/重复/错配/迟到连接直接关闭，错误token不得消费真正pending token。

Server未配对握手默认5s；外部accept起pending总时限默认10s，包含控制派发排队；到期关闭外部socket并使该token永久失效，后续建立须新connection_id。Probe不自动重连data流；重启不恢复任何Maintenance。Server租期默认240分钟、0选默认，自定义至少1ms；到期始终撤销全部listener和pending/active流。

任意业务字节保持原样，不解析HTTP/SSH/Telnet，不改Host/Location/Cookie，不终止TLS。正常EOF在已读字节写完后传播写半关闭，反向持续至EOF/错误/关闭/空闲时限。Server每方向32KiB、Probe每方向16KiB；两端均由任一方向实际读写进展刷新整条连接idle期限。读取服从写入背压，不保存完整流或无界队列。

主动关闭/租期/Session撤销先撤listener，再以reset终止data TCP，最后关闭外部TCP；正常EOF仍用半关闭。Probe读EOF后继续检查socket错误，确保CLOSE延迟或未送达时reset也能退出空闲worker。Gateway每Session一发送worker、64项队列，CONNECT写前复核context和Session；本地Released不等待控制writer。端口Released后还受默认24小时隔离约束，窗口及重启边界见API.md/ADR-022；wire身份和132-byte握手不变。

服务状态ready表示Server入口已监听，本地可达性在建流时才确定；某次local_unavailable使该服务unavailable，其他服务保持原状态，后续配对成功恢复ready。关闭全部closed。错误data建流只失败本客户端，不关闭控制TCP或其他通道。配额、端口池及详细生命周期为内部API配置，见API.md；Phase 5公开Maintenance endpoint仅复用该Service，不改变本节数据面。

### Phase 1E 验收与本地资源配置

Phase 1 完整验收已通过，14 项基线与测试映射见 [PHASE1_VERIFICATION.md](PHASE1_VERIFICATION.md)。本轮修复实现偏离，不修改上述字段、时序或 Accepted ADR。

Probe 当前每连接最多保留 1024 个未确认 HEARTBEAT 的 message_id。若一直存在其他合法流量但对端不确认心跳，容量耗尽时在发送下一心跳之前关闭 TCP，并沿用既有重连与任务处理规则。该本地资源上限不增加 wire 字段或 ACK 超时：在线会话中不淘汰未确认关联，乱序和迟到 ACK 仍按原 reply_to 校验；任意合法消息仍刷新 last_seen。

协商上限缩小时，Probe 立即复核同批 REGISTER_ACK 后已经缓冲的下一 Header，不等待超限 Payload 到齐。Server 文件 worker 退出后释放接收邮箱，任务身份与最终快照保留；文件块不因终态记录而继续驻留。
