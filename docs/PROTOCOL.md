# 路由器探针 TCP 长连接控制协议

协议版本：Protocol v1  
基线来源：v0.2 Word 设计输入  
日期：2026-09-05  
状态：已确认的互操作设计；Phase 1A 子集已实现

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
- SSH、Telnet、Web Tunnel 的数据面协议细节。
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

| Type | 名称 | 方向 | Payload | 用途 |
| --- | --- | --- | --- | --- |
| 0x01 | REGISTER | Probe -> Server | JSON | 连接建立后的设备注册 |
| 0x02 | REGISTER_ACK | Server -> Probe | JSON | 确认注册并返回会话参数 |
| 0x03 | HEARTBEAT | Probe -> Server | JSON | 应用层心跳和轻量运行状态 |
| 0x04 | HEARTBEAT_ACK | Server -> Probe | JSON | 心跳确认 |
| 0x10 | TASK | Server -> Probe | JSON | 下发通用任务 |
| 0x11 | TASK_ACK | Probe -> Server | JSON | 确认已接收或拒绝任务 |
| 0x12 | TASK_RESULT | Probe -> Server | JSON | 任务最终结果 |
| 0x13 | TASK_CANCEL | Server -> Probe | 保留 | Protocol v1 Phase 1 不支持，消息号留作后续扩展 |
| 0x20 | EVENT | Probe -> Server | JSON | Probe 主动上报异步事件 |
| 0x30 | FILE_BEGIN | 双向 | JSON | 开始文件传输 |
| 0x31 | FILE_CHUNK | 双向 | Binary | 文件数据块 |
| 0x32 | FILE_END | 双向 | JSON | 文件发送完毕 |
| 0x33 | FILE_ACK | 双向 | JSON | 文件接收状态或最终确认 |
| 0xFE | ERROR | 双向 | JSON | 协议级或通用错误 |

## 注册与设备会话

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
- 幂等缓存容量和淘汰策略属于实现配置，后续确定。
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

如果 Probe 拒绝 FILE_BEGIN，应返回 FILE_ACK failed，随后返回 TASK_RESULT failed。文件传输中途断开时，旧 transfer_id 失败；第一版重新发起新的文件任务或新的 transfer，不实现 resume。

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
- FILE_CHUNK 通过 offset 支持乱序校验和未来断点续传；第一版发送端仍建议顺序发送。
- download 与 upload 复用同一套 FILE 消息，方向相反。

### 文件并发与背压

Protocol v1 Phase 1 规定一个 Probe 控制连接同时最多只有一个 active file transfer。其他文件任务可以排队，或者按后续明确策略拒绝；具体选择仍为 TBD。不得并行向同一控制连接写入多个文件流。

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
| 文件传输中断 | 第一版将旧 transfer_id 标记 failed，重新发起新的文件任务或 transfer | 结束旧 transfer_id；第一版不实现 resume |
| Tunnel 数据连接断开 | Tunnel Manager 单独重连或结束 Tunnel，不影响控制 TCP | 根据 Tunnel 状态决定是否重新创建 |
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

ConnectionManager 不处理 exec 或 Tunnel 的具体业务。TaskManager 负责生命周期、并发、timeout 和 task_id 去重。各 Manager 通过统一结果对象返回给 TaskManager，再由协议层序列化 TASK_RESULT。LocalStateStore 只允许保存必要的小规模持久状态。

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

该时序描述未来上层调用，不表示 AI 或工具仓库已经实现。

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
| 等待中的文件任务 | 一个连接只允许一个 active file transfer；其他文件任务排队还是拒绝尚未决定 |
| BINARY/type 不一致等协议错误的严重程度 | 返回 ERROR 还是直接关闭连接的逐错误矩阵尚未决定 |
| 幂等缓存配置 | 同一 Probe 进程生命周期内必须去重；缓存容量和淘汰策略尚未决定 |

Tunnel 数据面、Relay 架构、数据库存储、OpenAPI 正式资源模型和 WebSocket 事件协议不在本协议中设计，分别继续由 ARCHITECTURE 和 API 文档标记为 TBD。
