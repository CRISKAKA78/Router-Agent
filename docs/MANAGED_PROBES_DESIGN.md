# 服务端纳管与动态探针配置

## 当前设备配置边界（ADR-045）

设备级动态配置限制为interface_sampling（接口周期与接口集合）；CPU/内存/磁盘/出口及模板属性周期均通过模板发布/明确应用修改。存量非接口覆盖只读保留到显式应用，应用确认时提示清理。CONFIG_APPLY新增持久template_generation，区别主动重新应用与接口调整/网络重试/重连；详见[API](API.md)、[PROTOCOL](PROTOCOL.md)。

WPF属性改为平行默认/自定义组，默认系统/资源在前、接口/其他在后，存储和连接历史独立。待纳管迁入左侧，模板更新迁入摘要图标和设备右键，移除旧设备资料/全采样弹窗入口。下文任意设备覆盖、全部属性嵌套折叠等为旧设计，最新依据[ADR-045](DECISIONS.md#adr-045-设备分组工作区接口采样与连接周期)。

> 当前基线为ADR-044：只支持工程8/草稿3，删除接口别名及旧Probe降级，属性展示增加字段/分类开关；REGISTER只含身份平台事实。下面ADR-041设计中被明确取代的语义以 [API](API.md)、[PROTOCOL](PROTOCOL.md) 和 [ADR-044](DECISIONS.md#adr-044-最新产品基线与属性展示工作区) 为准。

2026-09-09：用户审核七项改造计划后回复“确认，开始改造”；Accepted ADR-041。本文件定义本轮契约，实施状态以 PROJECT_STATUS 为准。

## 已确认行为

- 首次接入进入 pending，人工纳管为 managed，忽略为 ignored，可恢复 pending。纳管与 online/offline 独立；旧设备没有纳管记录时也进入待纳管。设备 ID 不变，管理员名称/型号不改注册事实。
- 独立设备目录持久保存纳管决定、最后发现资料、型号及其精确别名、默认模板、设备绑定模板快照和配置覆盖。重启不恢复在线、任务或维护。
- 型号单一默认模板、模板可关联多个型号；人工选型/模板优先。发布模板不自动应用；设备绑定保存具体版本。引用中的模板不能删除。
- 有效配置：设备覆盖 > 服务端模板 > 内置默认。新版 Probe 启动参数仅为初始值；模板由服务端选择。新能力须协商，旧 Probe 可发现但不能假装已应用。
- CONFIG_APPLY 0x07 / CONFIG_ACK 0x08，能力 managed_config_v1，通过原控制 TCP 下发完整版本化配置。相同版本相同字节幂等；拒绝旧版本或同版本冲突。ACK 仅表示采集计划已切换，不表示采样成功。离线保留目标配置，重连重新确认。采样携带配置版本，旧版本不能覆盖新版本。
- CPU 静态详情每 Probe 进程采集两次，首次启动、下一 CPU 周期（关闭时5秒）；重连不增加次数。CPU 使用率/当前频率仍周期采集。静态缓存重报保留年龄，不按周期过期。
- presentation 包含 groups（id/name/order）、fields（属性标识→group_id/order）、interface_aliases（精确系统名或switch标识:端口→显示名）。所有未分组字段进入末尾其他信息，排序相同时按稳定标识。展示元数据绑定实际配置快照，不修改历史注册。
- switch_probe 包含 backend（auto/dsa/swconfig/command）、command（厂商只读命令）、ports（id/switch_id/port/system_name/uplink/display_name/role）。标准化物理链路与管理状态分开，未知不伪造DOWN；物理口映射与显示别名独立。设备支持情况决定自动采集能力，厂商标签须真机逐口验收。
- 工程6兼容1～5（ADR-042），WPF纳管/动态设置/分组/接口展示，Blazor模板/型号映射维护。保持C++11、API First、可信管理网络与现有数据面，不扩展认证/RBAC。

## 验证

覆盖目录重启、冲突、模板引用、未纳管业务拒绝、配置ACK丢失/重连/版本隔离、CPU两次与缓存、端口解析和未知状态、工程兼容与两套UI。真实厂商端口与CPU固件采集单独验收。

## 操作入口

1. 先升级Server与Probe，再启动 `ui-windows.cmd` 和 `template-generator.cmd`。已运行的旧程序需由管理员正常退出后重新启动；不自动覆盖用户进程。
2. 生成器“模板配置”中的四个页签分别维护内置监控、分组与排序、接口显示名和物理端口采集；单个展示属性可在“属性与公式”直接选择分组和组内顺序。“导出与发布”发布模板并维护型号/精确别名/默认模板。
3. WPF“待纳管”选择设备，点“添加到设备列表”。探测型号匹配型号目录时自动带出型号和默认模板；可更改名称、型号，也可输入模板名称或ID筛选并选择。批量纳管使用各设备的探测名称与默认匹配，逐台操作成功独立保存。
4. 已纳管设备在“全部属性 → 设备资料与模板”选择并显式应用模板版本；“探针采集配置”修改周期/接口范围/属性周期或恢复模板默认，离线设备也可保存并在重连时下发。发布模板本身不会立即修改所有设备。
5. “全部属性”支持逐组/全部展开折叠；“物理端口”显示机壳名、系统接口、芯片端口、上联和状态；“网口速率”展示接口别名但保留系统名与真实统计身份。

## 模板结构示例

展示配置与采集命令分离，同一份字段表也能描述内置属性。模板顶层示例（发布时由Server生成template_id/version，CONFIG_APPLY下发时带齐）：

```json
{
  "name": "路由器示例",
  "properties": {},
  "monitoring": {"cpu_seconds":5,"memory_seconds":5,"disk_seconds":60,"network_seconds":5,"egress_seconds":600},
  "presentation": {
    "groups": [{"id":"basic","name":"基本信息","order":10},{"id":"network","name":"网络信息","order":20},{"id":"cellular","name":"4G/5G信息","order":30}],
    "fields": {"device_id":{"group_id":"basic","order":10},"cpu_model":{"group_id":"basic","order":20},"egress_ipv4":{"group_id":"network","order":10}},
    "interface_aliases": {"vlan3":"LAN1","switch0:1":"LAN1"}
  },
  "switch_probe": {
    "backend": "swconfig",
    "ports": [
      {"id":"lan1","switch_id":"switch0","port":1,"system_name":"vlan3","uplink":"eth0","display_name":"LAN1","role":"external"},
      {"id":"cpu","switch_id":"switch0","port":6,"system_name":"eth0","uplink":"eth0","display_name":"CPU上联","role":"cpu"}
    ]
  }
}
```

示例端口号只说明格式，不能直接当成某个机型的接线事实。CPU口6、LAN1口1等映射必须按真实板卡确认。

- groups最多64项，ID为小写字母开头、字母/数字/下划线、最多64字符；ID与名称唯一，保留other/其他信息。fields最多1024项，group_id空为其他；order为0～1000000，升序，同值按稳定组ID/字段key。未设字段顺序放组内末尾，未分组永远在其他信息末组。
- 内置字段标识可直接选择，例如device_id、managed_name、managed_model、cpu_model、cpu_usage、egress_ipv4、source_ip；动态网口/存储字段使用“全部属性”中显示的实际key。删除组后字段回其他信息；不修改属性采集身份。
- interface_aliases最多128项，键和值各最多128 UTF-8 bytes，精确匹配系统名或交换机:端口，不做通配/正则。别名不把vlan3变成一个物理口，也不改变其计数或速率。
- 端口映射最多64项，稳定id最多32字符，port可省略，填写时为0～255（0是真实编号，不表示未知）；switch_id最多64 bytes、system_name/uplink最多15、display_name最多128。role=external/cpu/空（未知）。同一已知物理来源不能重复映射；多口共用eth0必须以不同switch_id/port标识，不能复制eth0链路推断四口状态。

## 交换机适配与厂商采集

| 后端 | 只读探测方式 | 可确认的内容 |
| --- | --- | --- |
| dsa | sysfs物理网卡/交换机标识、phys_port_name、carrier、flags、iflink/ifindex、speed/duplex | 对应可见物理口、芯片/端口、上联关系及物理/管理状态；不列纯逻辑bridge/VLAN为独立物理口 |
| swconfig | `swconfig list` 后对合法芯片名执行 `swconfig dev NAME show` | Port与link/speed/duplex；机壳编号、上联和内部CPU用途通过模板补充，未提供管理状态保持未知 |
| auto | 先尝试swconfig实际芯片输出，没有可识别结果时回退DSA/sysfs | 固件能提供多少物理事实就展示多少；无法识别时保留未知和原因 |
| command | 管理员按固件编写有界只读命令，转换为统一TSV | 可接入厂商工具输出，不预置无法用真机验证的芯片私有命令 |

ADR-042物理口填写规则：

- 内部标识id由生成器自动生成，改变显示名称时保持不变。厂商命令按记录ID匹配时，须与输出第一列一致。
- switch_id是固件/工具报告的交换机实例，如switch0，不是交换机芯片型号；与port组成真实匹配来源。芯片编号和机壳LAN编号没有默认对应关系。
- 系统物理接口匹配（DSA，或auto回退）允许省略port，使用sysfs探测编号；缺失仍为未知，不写成0。旧模板显式数字继续保留。填写switch_id时必须同时填写port。
- command模式无芯片匹配条件时按输出id关联；system_name只作关联展示，不重定向到sysfs。上联uplink描述通往本机CPU的接口，不替代每个插孔的链路状态。
- 用途role为外部网口/CPU内部上联/未知，不配置WAN/LAN工作模式；display_name只影响名称。同一switch_id:port的interface_aliases优先覆盖显示名称，编辑器提示最终别名。
- 省略端口号的模板需配套更新后的Server/Probe；旧Probe会拒绝此配置，不能声称已应用。生成器按后端校验匹配字段，旧工程数字映射保持；不完整工程保留为可编辑草稿。

厂商命令单次最多5秒、32KiB成功输出；每行9列，使用Tab分隔：`id switch_id port system_name uplink state admin speed duplex`。未知字段填`-`，state/admin用up/down/unknown，speed用Mbps数值或`-`，duplex用full/half/unknown；最多64端口。失败、超时、截断、非法格式均不产生假的DOWN。命令以探针账号权限执行，应当只查询状态，不执行网口配置操作。

DSA/sysfs接口名保持不变；`p1`形式的phys_port_name规范为端口1。模板补充的空标签/上联/芯片字段保留已有探测事实。自动结果不会猜测机壳丝印顺序、WAN/LAN角色或芯片CPU口；确认后以模板保存。

## 升级与边界

- Server目录新增 `devices/catalog.json` 和独占锁文件，必须和同一Repository一同保留；没有记录的老设备也进入pending。旧任务/维护不恢复，管理员名称和绑定配置跨重启保留。
- 新版不再使用客户端template-id/template-name选型；旧参数仅兼容解析并明确提示忽略。新旧双方没有协商managed_config_v1时，不发送新控制消息；旧Probe配置显示需升级。旧启动式模板流程已被正式取代，完整七项功能需配套升级。
- 模板默认版本固定；同名型号可以共用模板，别名精确匹配，手工选定优先。模板切换时WPF保留仍存在的属性覆盖，清除已移除属性覆盖；内置周期覆盖可独立恢复默认。
- 折叠记忆限当前WPF窗口，按Server/设备/组ID隔离；当前没有服务器端个人偏好、账号/RBAC或布局共享。
- 厂商逐口插拔、固件特定命令和新版ARM/uClibc运行仍需实机验收。本机/隔离测试对端不代表这些设备已验证。

验证命令、产物与本轮限制见 [MANAGED_PROBES_VERIFICATION](MANAGED_PROBES_VERIFICATION.md)。
