# Management Server API

## 组网配置更新（ADR-066，2026-09-12）

- `POST /api/v1/networks`：新增 `password`（1～128 UTF-8 字节，不允许 NUL/换行）与 `mtu`（默认1380，576～9000）；旧调用省略密码时生成随机值。新记录 `profile=2`，`peer_urls` 空时补 `tcp://47.119.168.150:11010` / `udp://47.119.168.150:11010`。`network_id` 由 Server 生成，作为稳定的 EasyTier network_name；`name` 是可编辑的显示名。新网络不接受非空网络级 routes。
- `GET /api/v1/networks/{network}/password`：返回 `data.password`；仅此按需端点公开网络密码，所有响应沿用 `Cache-Control: no-store`。不是 EasyTier Web 账号密码，不通过普通网络 GET/list、WS、操作/拓扑传播。现有 API 尚未引入账户级鉴权，不将该端点声明为管理员权限隔离。
- `POST /api/v1/networks/{network}/members/batch`：`{"members":[{"device_id":"A","virtual_ip":"10.144.144.1"},{"device_id":"B","virtual_ip":""}]}`，1～64项。结构/IP/重复项先校验，固定地址成员先受理；202 返回每个设备的 `operation` 或 `error`，允许部分受理，不回滚已经受理的其他成员。沿用公共幂等键保护整次批量请求，失败成员不自动创建替代写请求。
- `PUT /api/v1/networks/{network}/members/{device}/config`：`{"revision":1,"config":{...}}`。revision 是成员 `config_revision`，不是网络修订。config 包含 `hostname`、`virtual_ip`（空表示DHCP）、`system_forward`、`lazy_p2p`、`need_p2p`、`p2p_only`、`disable_p2p`、`proxy_cidrs`、`enable_manual_routes`、`routes`。IPv4 CIDR 列表最多32项，拒绝非规范网段、重复项；manual=false 时 routes 必须空。
- 成员新增 `config`、`config_revision`、`applied_config_revision`，操作新增 `member_revision`、`recovery`、`superseded_by`。成员配置与运行实例重建操作原子保存；停止成员不创建启动操作。旧成员无 config 时保持旧默认，显式保存后才使用新成员配置。
- `uncertain` 是历史执行结果未完全证明；`reconciled` 仅表示当前运行状态/配置已核实，不伪造任务完成。后台自动只读核实；原操作被显式停止替代时保留 uncertain 与 Task IDs，并填写 superseded_by。RemoveMember 只检查目标成员，不采用整网未知操作锁。
- 拓扑观察新增 `hostname`、`nat`（UDP/TCP枚举）、链路 `remote_url`、路由 `instance_id`。路由下一跳现有字段为 `next_hop_peer_id`（WPF 修正旧别名不匹配）；路由可见但非直接连接的节点也列入拓扑。没有观察值时返回缺失/未知，不提供虚构端到端 RTT、丢包率或流量。
- 现有单成员加入、启动/停止、移除、网络更新和操作核实端点保留；新的约束、作用域和默认值以 ADR-066 为准。

## 设备日志（ADR-061）

所有入口通过 Management/Application 层；写请求使用原有 `Idempotency-Key`，重试保留原键、请求字节和已知 task_id。新 Probe 声明 `device_logs_v1`，旧设备不下发新任务。响应仍为 `data` / `error` 信封。

| 方法与路径（前缀 `/api/v1`） | 语义 |
| --- | --- |
| GET `/devices/{id}/logs/status?session_id=...` | 四个 NVRAM 原始字符串：debuglog_enable、syslogd_enable、log_save_en、log_save_itv；读取不修改配置。 |
| GET `/devices/{id}/logs/live?session_id=...&generation=...&offset=...` | 单次有界读取；value 含 state、generation、start、offset、gap、data（原始字节的 base64）。首次从尾部最多8192字节起读；没有文件时 waiting_for_file。 |
| GET `/devices/{id}/logs/history?session_id=...&directory=...` | 空目录自动扫描 /tmp/third_party/data、/jffs，并补充 /tmp/root RAM缓存；指定目录则只查该目录。返回 files、directories、limited。 |
| POST `/devices/{id}/log-tasks` | `{ "session_id":"...", "params":{...} }`，202返回task_id；副作用通过 TASK 执行。 |
| GET `/log-assets/{id}/preview` | 读取已进入Repository的原始日志；返回 text、bytes、truncated、parser_status。完整校验输入，最多展示512KiB，解压硬限64MiB。 |
| POST `/log-assets/{id}/text` | `{}`，201返回新TXT Asset；完整校验并合并全部gzip成员后发布，失败不发布部分TXT。支持幂等重试。 |

设备查询响应为 `{ "session_id":"当前会话", "value": ... }`，必须提供当前 session_id；不自动跟随新Session。每Session最多2个并发查询，live至少间隔500ms，查询超时12秒。超额429/log_busy；设备读取失败422/log_read_failed；会话变化、离线、不支持沿用现有错误映射。损坏、截断或超限压缩文件预览/解压返回422/log_preview_failed或log_decode_failed。

`params` 所有值为字符串，拒绝无关字段：

- `{"action":"enable_live"}`：设置debuglog_enable=1、syslogd_enable=3，回读并自动commit；关闭页面不恢复。commit提交整份已暂存NVRAM。
- `{"action":"history_settings","enabled":"0|1","interval":"300","persist":"0|1"}`：间隔1～65535秒；开启同时打开debuglog总开关；只有persist=1提交，关闭历史不关闭实时输出。
- `{"action":"snapshot","path":"/jffs/FF_BKDATA_2025-09-25.txt.gz"}`：成功TASK RESULT.stdout为JSON，含remote_path/name/size/expires_in_seconds；通过既有File下载流程读取该快照。
- `{"action":"release","path":"快照路径"}`：只释放本进程创建且登记的快照；不存在幂等成功，不删除原历史文件。

文件名匹配 `FF_BKDATA_YYYY-MM-DD.txt[.gz]`。files含name/path/size/modified（Unix秒）/cached；目录状态ok/alias/missing/unavailable/unreadable。目录/文件按设备与inode去重；扫描非递归、每目录2048条、总计128文件，达到上限以limited=true告知，不把权限失败当作空目录。

`parser_status=awaiting_vendor_samples` 只表示原文可用、厂商语义解析待样本；不推导基站、覆盖时间或模组故障。Windows允许导入本地日志到既有Asset，再使用相同预览接口。原始gzip导出即使保留损坏字节也明确提示校验失败，不称作完整日志。

## 通用 AT 身份采集增量（ADR-060，2026-09-11）

- `GET /api/v1/capabilities` 在已有能力列表中增加 `cellular_identity_v1`。生成器发布含AT配置的模板前校验此能力，缺少则阻止写请求；离线编辑/导出保持可用。
- 模板输入/运行模板增加可选 `cellular_probe:{"interval_seconds":30}`，默认省略即关闭，周期10～86400、空对象默认30；未知字段拒绝。支持只含AT配置的模板。模板摘要也返回cellular_probe。设备应用时需Probe同名能力，缺少报 `unsupported_cellular`，不影响旧模板应用。
- `GET /api/v1/devices/{id}/cellular` 为只读查询，无请求体，无 Task，无需 Idempotency-Key，不发AT；未知设备沿用404。标准响应 envelope 内 `data:{"snapshot":null}` 或完整快照。
- 快照字段：`config_revision, interval_seconds, status, reason, limited, ports, sampled_at, stale`。ports含 `path, device_key, status, reason, selected, age_ms, ati, imei, sampled_at`；ati/imei为 `command,status,value`。状态枚举、长度及指令集合见[PROTOCOL](PROTOCOL.md)的ADR-060增量。
- 现有 Device DTO 增加 `cellular`，纳管设备 DTO 增加 `cellular_configuration`（已应用模板配置，未启用为null）。快照只保留当前会话观测；新Session/应用或关闭模板清空，离线/超过三倍周期标stale。sampled_at由Server接收时间减Probe年龄推导，port.age_ms在查询时更新。
- WPF通过现有HTTP快照/WS失效回查显示蜂窝模块；“刷新快照”不触发设备查询。ATI/IMEI原值可见，无新增遮罩/认证/TLS保证。所有读取经Application/Device Service，不访问Gateway注册表或存储表。

使用、限额和未完成实机验收见[CELLULAR_AT](CELLULAR_AT.md)。下方各历史能力清单仅描述当时新增项。

## 智能邻居发现增量（ADR-059，2026-09-10）

本节扩展下方 ADR-056 接口，未改变原传输、认证边界或幂等机制。所有请求通过 Application/Service；客户端不访问 Gateway、设备注册表或存储。

- `GET /api/v1/capabilities` → `data.capabilities: ["neighbor_probe", "neighbors_inspect_v1", "neighbors_recent_v1"]`。生成器把404/405或缺少 `neighbor_probe` 视为不支持，**在发送邻居模板发布/更新请求前**禁用，显示“当前Management Server不支持邻居发现模板，请更新Server后再发布”与当前/所需能力。离线保存、导入、导出不受此能力门槛限制。
- `POST /api/v1/devices/{id}/neighbor-inspections`，需 `Idempotency-Key`，JSON 为 `{"session_id":"当前Session","config_revision":1,"vendor_test":false}`。返回202与 `data.task_id`，复用任务查询；这是读取设备事实的 TASK，不修改网络。普通检测可用于未纳管的在线参考设备；需 Probe `neighbors_inspect_v1`，Session/已应用revision必须匹配（未应用时revision可为0）。`vendor_test:true` 仅用户显式点击，Server还要求注册型号为FNR100；不允许自动厂商命令探测。
- `GET /api/v1/devices/{id}/neighbor-discovery` → `data.discovery`，无结果为null。设备DTO同时添加 `neighbor_discovery`。字段为 `networks[]`、`preset`、`preset_status`、`raw_summary`（最多4096字节）、`ports[]`、`session_id`、`config_revision`、`detected_at`、`stale`；90秒、离线或Session/revision不符即过期。每个network含 `interface/bridge/vlan/master/eligible/reason/ipv4/networks/ports`；ipv4保留主机地址和前缀，networks由Server规范化，ports是内核桥成员路径。不推断上联/下联。桥成员不能作为独立采集接口；VLAN子接口按真实事实列出。普通检测不验证switch0/ARL，`preset_status=not_tested`；显式测试可能为verified/failed，不匹配回退内核FDB。
- 设备纳管DTO添加 `neighbor_configuration`（已应用的完整neighbor_probe或null），供用户明确“使用设备当前已应用配置”；包含可选高级命令，只复制，不执行。原 `neighbor_domains` 与模板摘要保持兼容。
- 原 `GET /devices/{id}/neighbors` 保留 `data.snapshot`，添加 `data.recent`、`retention_seconds:86400`、`capacity:1024`、`persistent:false`；设备DTO同时提供 `recent_neighbors`。近期条目包含原行字段及 `domain_id/scope/first_seen/last_seen/current/active_at`。每设备按域/IP/MAC去重并按最后发现淘汰，跨域可重叠；仅Server内存，重启、Session替换或配置修订变化清空。离线不立即删除历史，但不表示在线。默认客户端显示recent，用户可切换最新快照。
- 主动响应的 `responded/active_arp` 最长60秒，之后移除主动新鲜来源，近期行显示 `recent` 与最后发现时间；仅主动历史来源为 `active_arp_history`。缓存、租约、FDB以及 `current:true` 均不是在线保证。只有MAC时IP保持空/未知。
- 原扫描接口仍要求规范IPv4 CIDR `/24`～`/32`（最多256地址）。新Probe的检测结果必须新鲜且属于所选域的当前直连网络；失败400 `field:"cidr"`。为了兼容已发布的 `neighbors_v1` Probe，未声明检测能力时保持旧的规范范围校验，并由Probe最终检查直连范围；新WPF提示升级后使用自动检测。客户端接受主机地址前缀并在提交前显示规范化值；不放宽Server或Probe最终校验。
- 扫描成功后任务DTO可选 `neighbor_summary:{responses,added,updated}`；统计以扫描派发前同接口IP/MAC记录为基线，不重复计算跨域记录。实时结果立刻合入可匹配广播域历史；LAN仍等待真实端口证据。刷新查询不创建扫描。重复RESULT不重复合入；旧Session/revision的结果不污染新视图。
- 模板 `neighbor_probe` 可选 `fdb_preset:"fnr100"`，与 `fdb_command` 互斥；预设需新Probe额外能力，设备应用前显示能力不足，不把保存模板视为设备应用成功。
- 错误保持原 `error.code`（例如invalid_request），兼容增加字符串 `field`、`details`，如 `neighbor_probe.domains[0].interface`。新生成器定位字段，旧客户端可忽略新增详情。

操作、边界与本轮证据见[智能邻居配置](NEIGHBOR_SMART_CONFIGURATION.md)。

## 邻居发现（ADR-056，2026-09-10）

新增路由均通过Management Application进入Device/Task/Gateway服务，前端不访问连接表。路径前缀 `/api/v1`，响应继续使用data/error信封，写请求需要Idempotency-Key。

| 方法与路径 | 请求与结果 |
| --- | --- |
| `GET /devices/{id}/neighbors` | `data:{snapshot:对象或null}`；暂无当前有效配置样本为null，设备不存在404 |
| `POST /devices/{id}/neighbor-scans` | `{"domain_id":"local","cidr":"192.0.2.0/24","config_revision":1}`；202返回 `{task_id,dispatch_uncertain?}` |
| `POST /devices/{id}/neighbor-scans/{task_id}/cancel` | 空对象 `{}`；202返回取消任务自己的task_id，原扫描通过 `GET /tasks/{task_id}` 查询终态 |

快照包含config_revision、interval_seconds、sampled_at、stale、limited、domains和unclassified。domains内为id、scope（lan/broadcast）、interface、status、reason、limited、rows；行包含ip/mac/port/hostname/source/state，未匹配行额外含interface。空IP/端口表示未知；不把租约或缓存解释成在线。完整结构与限额见[协议](PROTOCOL.md#邻居发现adr-0562026-09-10)与[功能说明](NEIGHBOR_DISCOVERY.md)。

设备列表/详情DTO增加 `neighbors`（无样本为null），已应用模板时增加 `neighbor_domains`（id/scope/interface/ports/lease_file配置，不含可执行命令）；WPF沿用HTTP快照与WS变更通知。发布模板API增加可选neighbor_probe，当前C#工程8/草稿3可选保存该字段，新建及版本更新均保留；默认未启用不改变旧模板。

扫描要求设备已纳管、在线且声明neighbors_v1，域属于当前已应用配置、revision一致。无能力422 unsupported_capability；离线/未纳管/会话变化或配置冲突409；不存在的设备/任务404；无效域、非规范CIDR、IPv6扫描、/24以外的大网段及非法字段400。范围是否在接口当前直连IPv4子网由Probe检查，失败通过原TASK RESULT返回。取消只接受本设备的neighbor_scan任务，不得取消其他设备或其他类型。

响应不确定重试原路径、字节与Idempotency-Key，不创建替代扫描；原TASK与取消TASK各自查询终态。主动发现结果通过后续快照刷新，不把“任务已提交”解释成已发现设备。

## 来源 IP 摘要与接口导航（ADR-048，2026-09-09）

WPF所选设备“出口IP”只显示已有`source_ip`，运营商及归属地查询以该地址为目标；它仍表示Server观察到的TCP对端，不改变`egress_ipv4/egress_ipv6`的Probe探测语义，缺失时不互相替代。客户端解析结果不写入Server设备状态。

接口状态内分外壳端口/系统端口，接口采样时间通过弹窗复用原`PUT /devices/{id}/profile`及`interface_sampling`，无新路由或DTO。局部取代下方ADR-047关于两个专用同级页的说明；存储展示、归组、配置版本、幂等和文件业务契约保持。

## 客户端展示与文件工作区（ADR-047，2026-09-09）

本节取代下方ADR-044/045中存储页恒显、接口属性分组及presentation成员清单的对应说明；其余配置应用、版本冲突与幂等语义保持。

- 模板`presentation`新增可选布尔`storage_visible`。省略按true展示存储空间页，false隐藏该页；显式null返回400。模板保存、版本快照及设备已应用展示均保留该值。仅控制展示，不改变磁盘采集、effective_metrics或文件能力。磁盘属性仍由`builtin_visibility.disk`及字段`visible`独立控制。
- 客户端属性默认组为系统信息、资源监控、自定义组、其他信息；net_*、switch_*归其他信息。既有`fields[key].group_id="builtin_interfaces"`仍可保存，但客户端展示到其他信息；这个ID/旧名称继续保留为内置保留值，不可声明为自定义组。外壳端口/系统端口为专用同级页，不再承载属性表。
- 设备列表和设备发现是显示名称，API的managed/pending/ignored值保持。模板选择通过现有`GET /probe-templates`及设备profile完整PUT显式应用，选中最新版本不等于已经生效。
- 文件页使用现有exec读取有界目录清单，使用`POST /assets`准备上传内容、`POST /uploads`发送设备；下载串联`POST /downloads`、原任务/transfer查询、`POST /downloads/{task_id}/complete`及资产内容下载。服务端资产身份与保留规则保持，客户端取消等待不等于撤销设备任务。committed/released与Task RESULT分开处理，网络结果不确定复用原幂等键和请求字节。
- 仓库工具通过现有tools/versions/compatibility及deployments API查询与投放，确认框明确版本、兼容产物、目标目录（默认/tmp）与覆盖选项；投放不自动执行。Windows移除资产/工具发布入口，管理员上传通道未实现，现有公开服务能力不删除。

没有新增HTTP路由、目录协议或Probe消息。实现与验证见[客户工作区改造](CUSTOMER_WORKSPACE_VERIFICATION.md)。

## 设备工作区契约（ADR-045，2026-09-09）

本节取代下方ADR-041的任意设备采样覆盖和ADR-044的资源默认隐藏/嵌套分组说明。现有路由、请求非null校验、整份PUT、幂等键和版本冲突机制保持。

- `presentation.groups`只声明自定义组，不得使用内置ID或名称：`builtin_system`（系统信息）、`builtin_resources`（资源监控）、`builtin_interfaces`（接口信息）、`other`（其他信息）。`fields[key].group_id`可以直接引用这些内置ID，无需先声明；省略/空串使用字段默认分组。系统/资源/自定义组（order、id）/接口/其他顺序固定。CPU使用率、当前频率、memory_*、disk_*默认资源组；net_*、switch_*默认接口组；已知设备字段默认系统组，未归类模板字段默认其他组。显式字段分组优先。
- disk分类现在默认可见，network/switch仍默认隐藏明细，其他分类默认可见；session_id默认隐藏，显式visible仍可开启。专用存储表和连接历史始终独立于属性分组。所有默认/动态/模板字段均可配置group_id/order/visible。
- `PUT /devices/{id}/profile`新增可选`interface_sampling`，仅含`network_seconds`（0～86400）和`network_interfaces`（沿用模板名称校验）。成员省略表示继承模板；接口字符串空表示全部默认接口。对象省略清除此覆盖，恢复绑定模板/存量计划默认。请求内不得发送null。改名等完整PUT需带回要保留的接口覆盖。
- 旧monitoring/property_intervals仅允许原值往返，变更返回400；省略不会静默清除已有非接口覆盖。显式apply_template=true或更换模板时清理旧覆盖，并保留明确传回的interface_sampling。其他采样变化必须先发布模板，再显式应用。
- 配置内容变化增加revision；显式应用即使同版本也同时增加revision和持久template_generation。相同幂等键/字节重试不增加代次。只改接口不增加代次、不重新执行其他采样；改名不增加采集修订。
- 设备profile新增`interface_sampling`、`template_generation`、`latest_template`（绑定模板的最新元数据）。`bound_template`为期望快照，`active_template`及`applied_revision`为实际确认事实。只有当前Session ACK匹配desired_revision才显示applied；离线保存仍为waiting_dispatch。发布新模板不会自动应用。

示例：`{"version":2,"admission":"managed","name":"机房路由器","model_id":"router-a","template_id":"tpl_x","interface_sampling":{"network_seconds":3,"network_interfaces":"eth0,br0"}}`。

`GET /devices/{id}/connections`提供连续在线周期，分页items按新到旧排列。每行包含：id、state（online/offline/completed）、online_at、offline_at、reconnected_at、online_seconds、offline_seconds、end_reason、observed_at。未离线时offline_at/reconnected_at/offline_seconds为null；在线秒数累加。离线后在线秒数固定，离线秒数累加；再次上线冻结旧行离线秒数并新增一行。Session在线替换不新建周期，原/sessions接口保留技术查询。

时间和秒数由Server观测状态生成，使用同一次查询时刻；客户端以单调计时器每秒展示，快照校准，失去同步暂停。进程内保留最近history_limit个已完成周期（默认64）及一个当前周期，响应附history_limit/total_periods/evicted_periods。Server重启后仅有持久资料的设备返回空列表，不捏造离线时长；未知设备404。本接口不是持久审计或心跳时序库。

内存容量/百分比原始遥测契约保持，WPF组合为`25%（已用容量/总容量）`；缺失容量显示—，不倒算填充。验证见[DEVICE_WORKSPACE_VERIFICATION](DEVICE_WORKSPACE_VERIFICATION.md)。

## 当前属性展示契约（ADR-044，2026-09-09）

`presentation` 仅包含 `groups`、`fields`、`builtin_visibility`，接口映射 `interface_aliases` 已删除，模板写入携带该字段返回400。`fields[key].visible` 是可选布尔值：省略继承分类，true/false覆盖分类；分类仅接受 hardware/cpu/memory/disk/network/switch/egress，省略时 disk/network/switch 为false，其余true。自定义模板字段默认显示。展示过滤在客户端进行，API的effective_metrics、采集计划、专用页面数据保持完整；开关跟随设备显式应用的模板快照。

REGISTER和API registration不再包含 template/attributes/collection_errors/report_intervals；有效模板读取active_template，属性值、错误与周期读取effective_metrics。当前注册必须支持 managed_config_v1 与 telemetry_v2，不再返回unsupported热配置状态；port_counters_v1仍是独立功能能力校验。

例如：`"presentation":{"groups":[],"fields":{"device_id":{"visible":false},"net_65746830_rx_bytes":{"visible":true}},"builtin_visibility":{"disk":false,"network":false}}` 隐藏设备标识和网口明细，单独保留指定网口字节字段。字段/分类开关不更改monitoring。

本节及ADR-044取代本文下方历史接口别名、旧REGISTER快照和降级说明，完整使用与验证见 [UI_REFINEMENT_VERIFICATION](UI_REFINEMENT_VERIFICATION.md)。

## 外壳网口字节统计（ADR-043）

模板API兼容增加switch_probe.counters（backend、command或rx_field/tx_field、bits、basis），启用时1～16个显式端口。设备effective_metrics的switch组增加逐口原始字节、本次统计累计、bytes_per_sec、seconds与来源/口径/顺序；bytes使用uint64十进制字符串，客户端不能先转double再用于累计差分。没有新HTTP路由，configuration_state=failed且configuration_error=unsupported_port_counters表示Probe需升级。详见[字段与失效语义](PHYSICAL_PORT_MONITORING.md#模板与遥测契约)。

## ADR-042 物理端口可选编号

模板API的switch_probe.ports[].port可省略；显式0表示真实0号端口。以switch_id匹配时port必填；省略的编号在复制、存储、读取和下发中保持省略。旧数字字段保持兼容，没有增加HTTP路由或更改纳管状态机。生成器工程6的编辑格式不进入Server业务模型。具体规则见 [MANAGED_PROBES_DESIGN](MANAGED_PROBES_DESIGN.md)。


## 设备纳管、型号目录与动态配置（ADR-041）

本节是当前设备列表/模板分配规则，取代下文旧“客户端指定模板、重启生效”的描述。注册事实与管理员资料分离，读接口仍以公开 HTTP 快照为准；变更复用 Idempotency-Key 和版本冲突处理，WS 沿用 devices 变更提示，首连/重连回查。

| 方法与路径（均在 /api/v1） | 行为 |
| --- | --- |
| GET /devices | 仅纳管设备，仍可按 status 筛选；`admission=all` 返回含待纳管/忽略的统一管理快照 |
| GET /discoveries?admission=pending\|ignored | 默认 pending，分页待纳管池/忽略池；离线发现记录不会丢失 |
| GET /devices/{id} | 包含未纳管记录；Server 重启后仍可查询持久资料，但显示离线 |
| PUT /devices/{id}/profile | 按 version 乐观更新整份管理资料；成功后版本+1；仅配置内容变化才增加 desired_revision |
| GET /device-models?q=文本 | 分页型号目录，按名称/别名包含搜索 |
| GET /device-models/match?name=探测名称 | 去首尾空白、忽略大小写后精确匹配名称/别名；未匹配 data=null；不做模糊自动绑定 |
| PUT /device-models/{id} | `{version,name,aliases,template_id}`；新建version=0，修改用现版本；型号最多32别名，跨型号同名/别名冲突409 |

profile 更新示例：

```json
{"version":1,"admission":"managed","name":"机房路由器","model_id":"router-a","template_id":"tpl_x","template_version":3,"apply_template":true,"monitoring":{"cpu_seconds":10,"memory_seconds":10,"disk_seconds":60,"network_seconds":5,"egress_seconds":600},"property_intervals":{"signal":30}}
```

- name 为1～128 UTF-8 bytes；model_id 空表示未指定。admission=pending/ignored/managed；待纳管可忽略、恢复或纳管；本轮不提供已纳管退池/资产删除。旧安装没有登记资料的设备也先入池。被忽略设备重连仍保持忽略。
- template_id 空为内置采集。选择不同模板或 apply_template=true 从服务端解析当前版本并保存完整快照；template_version 非0时必须匹配，避免选择期间发布竞争。相同ID且不应用保留已绑定版本；发布新版、改型号默认映射不会自动更新已纳管设备。被型号或设备引用的模板删除409。
- monitoring 与 property_intervals 为设备覆盖，省略表示清除覆盖/恢复模板默认；对象成员默认周期同运行模板。interval 0～86400，未知属性覆盖拒绝400。PUT 是完整更新而非PATCH，调用方修改名称时须带回仍需保留的覆盖。模板字段移除后，WPF应用表单只保留新模板仍存在的字段覆盖。
- Device DTO增加 profile（version/admission/name/model_id/model_name/monitoring/property_intervals/bound_template/desired_revision/configuration_state/configuration_error）、applied_revision、active_template、presentation。bound_template 是绑定元数据及属性 name/interval_seconds，不含可执行命令；active_template/presentation 来自最近实际确认的 Session。Session DTO也带 applied_revision/active_template/presentation，历史快照不借当前模板改写。
- configuration_state 为 not_managed / unsupported / waiting_dispatch（离线）/ waiting_confirmation / applied / failed。只有确认当前 desired_revision 才为 applied。目录保存期望配置，重启不伪造已确认或在线状态。pending/ignored 的业务请求被服务端拒绝409 device_not_managed，不能绕过UI。
- 模板 POST/PUT/导入导出支持 presentation、switch_probe；字段结构、顺序和物理端口含义见 [MANAGED_PROBES_DESIGN](MANAGED_PROBES_DESIGN.md)。仅布局/监控模板可用 properties={}；完全空模板仍无效。旧格式字段省略兼容，生成器工程5兼容1～4。模板发布是保存定义，管理员在纳管/设备资料中应用后在线同步，离线下次连接同步。

## ADR-040 出口、统计与监控配置

设备/Session公开effective_metrics兼容增加egress_ipv4/egress_ipv6、net_<hex接口名>_rx_bytes/tx_bytes/elapsed_seconds；Metric增加可选reason。出口由Probe探测，source_ip仍由Server观察TCP连接，二者不能互相冒充。归属地/运营商查询保留在C# Client，不进入Server核心状态。

模板monitoring新增egress_seconds（省略600，0关闭，1～86400）和network_interfaces（可选逗号分隔字符串，省略沿用Probe构建默认值，显式空串选择默认全部接口，null拒绝；最多32个不重复精确名称，每项1～15个ASCII字母/数字/下划线/点/短横线，拒绝单独点和双点）。新生成器工程4兼容1/2/3；更新Server后再发布新配置。既有路由、快照、幂等与任务语义保持。详见 [TELEMETRY_DESIGN](TELEMETRY_DESIGN.md#adr-040-双栈出口与流量统计)。

ADR-035：Windows 调用方现为 `windows/RouterWorkbench.Client` 的原生 C# HTTP/WS Client；资源路径、请求与响应、幂等和业务状态均未变化。ADR-036 的 WPF 页面消费公开 DTO，维护仅打开外部客户端，主程序无内置终端；独立生成器继续使用 ADR-034 的 TemplatePublishingService。ADR-037 已移除旧 React/WinUI/Win32 UI；下文旧客户端描述仅保留为历史语境，当前 Windows 设计见 [WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md)。

本文件维护 `/api/v1` HTTP/WebSocket规范及原内部Service契约。实现入口 `internal/api`，业务来源为 `management.Server` 与 Service。ADR-029 新增服务端属性模板及注册快照字段；既有 Tunnel 数据面不变。

Phase 6 React Shared Frontend / Windows WebView2 Shell复用本文件公开契约（ADR-026/027），此前重构未补充生产 API；本轮模板扩展见下文。客户端首连/重连HTTP同步、维护与Exec/文件/工具动作、相同键显式重试及入口启动规则见[PHASE6_DESIGN](PHASE6_DESIGN.md)与[Windows使用说明](../windows/README.md)。Windows UI不能直接调用下文内部Go接口。

冻结 UI 的内置 Shell 通过本机 SSH/Telnet 客户端连接重新查询的 Maintenance 公共入口，不通过 HTTP Exec 或 WebSocket 传送持续终端字节。右侧目录使用有界单次 Exec，文件内容通过 uploads/downloads/complete；工具投放列表从 tasks/{id}/operation 的 tool_id 关联得出。ADR-039已提供CPU、内存、存储与网口遥测，4G等未上报属性仍显示未提供。

## 设备监控与连接来源（ADR-039）

GET `/api/v1/devices`、`/devices/{id}`以及Session DTO兼容增加`source_ip`（Server观察到的Probe TCP对端IP，NAT为出口地址）和`effective_metrics`（按属性标识的最新值映射）。SourceIP不从Probe自报字段读取。

每个指标包含name/value/unit/status/entity/interval_seconds/source/group/sampled_at/stale；数值在value中使用不带单位的字符串，容量byte、速率byte/s、频率MHz、百分比0～100。unknown/waiting/error不保留旧成功value；source为builtin/template，由Device Service赋值。模板同标识优先，含失败，不按更新时间抢占；实时结果不修改registration。新Session重新建立监控，离线保留最后样本，存储/网口移除时删除对应组中的旧指标。HTTP字段与范围见 [TELEMETRY_DESIGN](TELEMETRY_DESIGN.md)。

监控变化复用devices resource_changed（有界合并），客户端收到提示回查HTTP，首连/重连完整回查。runtime仍只表达既有心跳时长，不因监控事件覆盖。

模板POST/PUT与返回增加可选monitoring，属性增加可选interval_seconds；旧输入缺省保持启动采集。monitoring子字段省略采用5/5/60/5，0关闭；周期均不超过86400秒，负数/非整数/null子字段拒绝。目录持久保存，新Probe下次启动读取。

## 部署与生命周期

### 设备运行状态（ADR-038）

`GET /api/v1/devices`、`GET /api/v1/devices/{id}` 的设备 DTO，以及对应 current_session/latest_session 和 `/devices/{id}/sessions` 的 Session DTO，兼容增加 `runtime`。首个有效心跳到达前为 null，收到心跳后为：

```json
{"uptime_seconds":18372,"reported_at":"2026-09-08T12:00:00Z"}
```

uptime_seconds 为 0～9223372036854775807 的整数秒，无法获取时 null（有效的 0 不是未知）；reported_at 为 Server 接收该心跳的 UTC RFC3339 时间。缺字段的旧 Server 可由客户端按未提供处理。新版 Probe 的有效性及旧版零值兼容规则见 PROTOCOL 的 ADR-038。

每个 Session 只保存最新采样；设备顶层 runtime 与 latest_session.runtime 一致，在线取当前 Session，离线保留最近结束 Session 的最后值。新 Session 首报前不继承旧值，采集失败覆盖旧值为未知；其他合法活动不会更新 reported_at。Server 重启清空 Inventory，不增加持久化或时序查询。心跳仍不触发 devices WebSocket 事件，WPF 通过既有五秒 HTTP 回查显示最后实测值，不自行累计。

registration.arch/kernel 沿用现有字段及限制。新版 Probe 默认采集内核，显式 kernel 模板仍优先；完整内核字符串继续参与既有精确兼容匹配，不改变匹配规则。

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

`cmd/server` 默认启用 `-http-listen :8888`，控制 TCP 为 `-listen :9000`，data 为 `:9001`，维护绑定 `::`，对外维护及 data 地址为 `47.119.168.150`（ADR-057）。HTTP 包括命令执行、文件与设备断开能力，仅用于可信本机或受保护管理网络。当前 CLI 采用用户确认的全接口监听；远程访问由部署层完成 TLS、认证和网络访问限制。当前没有内置用户、租户、RBAC 或完整审计，不能把 loopback、Origin 校验或 Tunnel 配对 token 当成用户认证。

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

内部 Service 配置零值（CLI 按 ADR-057 显式覆盖）：BindHost/AdvertisedHost/DataHost均127.0.0.1；DataListen=127.0.0.1:9001；PortFirst/PortLast=20000/20199；MaxMaintenance=64；PerMaintenance=8、PerDevice=8、TotalConnections=512；Handshakes=64；History=128；PendingTimeout=10s、HandshakeTimeout=5s、IdleTimeout=24h、PortReuseDelay=24h。数值0选择默认；IdleTimeout范围1ms～24h，PortReuseDelay至少1ms，非法范围启动报错。默认200个端口在隔离窗口内最多支持66次三入口分配，按维护频率配置更大池。各限制不提供无界关闭选项。

DataHost接受IP或DNS主机名；Create在Server解析（最多5s、受ctx取消），优先IPv4，本次维护固定所得IP，下次Create更新；并行Create最多MaxMaintenance个，超额ErrCapacity。解析失败无入口，解析不阻塞Close；绑定失效在发布前复核。Probe不做DNS，仍收到数值IP。部署者保证Server解析所得地址从Probe可达；不自动判断split-horizon或逐地址故障切换。AdvertisedHost可为域名，由外部客户端解析。

Server flags：`-tunnel-bind`、`-tunnel-host`、`-tunnel-data-listen`、`-tunnel-data-host`、`-tunnel-port-first`、`-tunnel-port-last`、`-tunnel-port-reuse-delay`、`-tunnel-max-sessions`、`-tunnel-session-connections`、`-tunnel-device-connections`、`-tunnel-total-connections`、`-tunnel-handshakes`、`-tunnel-history`、`-tunnel-connect-timeout`、`-tunnel-handshake-timeout`、`-tunnel-idle-timeout`。Probe `--tunnel-connections`默认8，允许1～64，超出范围启动失败。每流一线程，低内存部署可调小；需要更多浏览器连接时同步提高双方限额。公网绑定、可达地址、NAT和防火墙由部署者配置。

固定目标为Probe的127.0.0.1:80/22/23。Maintenance/connection/token不持久化；无跨进程恢复、续租、任意端口、UDP/SOCKS/VPN/P2P、HTTP反向代理、TLS终止或通用映射管理；Phase 5增加HTTP/WebSocket，不增加UI/MCP或操作CLI。

## 异地组网 API（ADR-064，首轮实现）
实机修正：启用后最多只读观察20秒，不重复配置写请求。上传成功依赖已校验的TASK_RESULT与Released；Committed仅表示Server接收下载的本地提交。官方2.6.4对开启但为空的手动路由回读可能丢失开关，原uncertain操作不能据此强制确认；见[实机边界](OVERLAY_LIVE_VERIFICATION.md)。公开DTO/路径和幂等契约不变。

新增 `/api/v1/network-settings`、`/networks`、网络成员/操作/拓扑、`/network-operations/{id}/reconcile` 及 WS `networks` topic；方法、DTO含义、错误码与部署状态见 [OVERLAY_NETWORK §API](OVERLAY_NETWORK.md#api--probe--ui)。写操作遵循既有幂等、JSON `{}` 空对象与 envelope；接受组网返回202只表示持久操作入队，不表示已互通。账号、密码、网络密钥和配置接入URL不属于公开DTO。

本轮未提供 L2Segment API。拓扑来自采样证据，管理在线与引擎/链路状态分开，未知遥测返回null/unknown/stale，不推导成已连通。
