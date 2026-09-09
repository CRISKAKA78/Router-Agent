# 设备监控与连接来源（ADR-039）

## 原生出口与接口重配（ADR-045）

出口探测已改为Probe原生HTTPS socket/Mbed TLS客户端，固定双栈端点、证书校验及有界取消见[PROTOCOL](PROTOCOL.md)。不再依赖curl/wget。接口动态重配保留其他worker及采样时间，显式应用代次才重建模板采集；下方ADR-040外部工具实现和ADR-041全监控设备覆盖已被取代。

内存百分比原统计口径（MemAvailable或旧内核可回收估算）保持；WPF展示实际memory_used_bytes与memory_total_bytes的容量，不改变Probe原始数值。


## ADR-040 双栈出口与流量统计

2026-09-09用户确认。下述条目局部取代ADR-039的字段/展示限制，其余生命周期保持。验证见 [MONITORING_V2_VERIFICATION](MONITORING_V2_VERIFICATION.md)。

- 新Probe同时声明telemetry_v1、telemetry_v2；ACK只对支持v2的Probe确认v2。未确认时关闭出口采集，剔除新的内置型号/固件状态、网口累计字节及统计秒数，保留v1速率。旧Server不能返回新数据，WPF显示横杠与未提供原因；旧Probe仍正常连接新版Server。
- `monitoring.network_interfaces`为可选逗号分隔精确名称字符串。省略沿用构建值，空串选择默认全部（默认排除lo；显式选择lo可以采集）；最多32项，大小写敏感、不重复，每项1～15个ASCII字母/数字/`_`/`.`/`-`，拒绝`.`与`..`。CLI `--network-interfaces` > 模板 > CMake `RMP_NETWORK_INTERFACES`。不存在的指定接口上报unknown和interface_missing，不回退全部。过滤在Probe读取详细信息/生成报文前完成。
- network新增 `net_<UTF8 hex接口名>_rx_bytes` / `_tx_bytes`（bytes）和 `_elapsed_seconds`（seconds）。累计值是首次有效采样的内核计数差，首次0，时长从同一单调时钟基线开始。每接口分别计算，不能累加桥/虚拟/物理接口作为设备总量。Probe重启、接口消失/ifindex变化、计数回退时重置；普通控制Session重连复用进程内SystemSampler，不重置。Server仅保存最新观察值。
- 内置网口沿用最多48接口的有界枚举，新版完整组仍最多256项及64KiB/协商载荷上限；超限用collection_status错误替代整组并携带payload_limit原因，不能发送半个组或静默保留旧值。配置较小白名单可减少载荷。统计完整性只覆盖已枚举/选中的接口。
- egress组仅含 `egress_ipv4` / `egress_ipv6`，text，值必须为对应协议地址；按键增量，两种协议分别保存成功/失败与采样时间，不用一个协议更新清除另一个。`monitoring.egress_seconds`省略600，0关闭，1～86400；CLI `--egress-interval`优先，重启配置生效。关闭时v2报告collection_disabled。
- 独立有界worker在注册后首轮及每周期，使用设备已有curl（强制-4/-6）或wget（协议专用域名）访问固定HTTPS `api.ipify.org` / `api6.ipify.org`。清除代理环境变量，命令最长8秒并支持退出取消，不自行实现HTTP/TLS、不安装设备依赖。响应长度/IP家族校验，缺命令、网络失败、超时、非法结果均清除旧值并报告原因，不以WPF电脑IP或source_ip补造结果。[ipify官方接口](https://www.ipify.org/)提供纯文本IPv4及IPv6专用端点；实际网络成功与协议替身测试分开记录。
- Metric可选reason为最多128字节非null字符串；模板命令原因保留command_failed/timeout/empty/invalid_output/budget_exhausted，内置增加model_prefix_missing/interface_missing/http_tool_missing/egress_request_failed/invalid_ip/collection_disabled/payload_limit。unknown/error/waiting值为空；seconds只接受非负整数。新字段仍由Service决定source、sampled_at和模板优先。
- 默认型号/固件用有界只读 `nvram get softver` 启动采集；全文去首尾空白后存firmware，首个小写v前去首尾空白存model。无v则firmware保留、model未知；命令失败两项未知，超过既有128 bytes上限按无效处理，不截断。模板独立覆盖两字段，显式模板失败不回退；普通重连复用注册快照。
- WPF对设备上报公网出口分别查询归属地与运营商，查询失败保留IP，已知与未知部分独立显示；缓存、限流、超时、切换/退出取消保持。值列不拼来源/采样时间，未知统一“—”并悬停原因。连接来源仍单列保留原含义。最近心跳用runtime.reported_at，时长单位年/月/天/小时/分钟/秒且无空格。网口表最后一列更新时间，累计收发流量与统计时长独立列；双击打开收发曲线，仅窗口打开后保留10分钟，离线/过期/接口消失/失败/Session变化断点，关闭释放。
- 生成器工程4兼容1/2/3与旧草稿，完整保留接口配置、出口周期及逐属性周期；新运行模板需更新Server后发布，旧Server会拒绝不认识的新配置字段。


2026-09-08 用户确认 CPU 型号/频率/完整架构、CPU/内存/存储/网口独立周期、模板优先和 WPF 展示方案，并追加连接来源 IP 与公网归属地。实现与验证结果在 PROJECT_STATUS 和 TELEMETRY_VERIFICATION 中维护。

## 契约

- 保留 registration.arch 的工具匹配含义。CPU 详情在 hardware 监控组；区分硬件能力、内核运行架构和 Probe 位数；未暴露信息为 unknown。
- Probe 声明 telemetry_v1，Server 的控制载荷上限至少8 KiB且 REGISTER_ACK 返回 telemetry_v1=true 才可发送 EVENT。旧 Server 不支持时保留注册与心跳，明确日志提示监控不可用，不发送未知消息。
- EVENT flags=0，payload 为 `{event:"telemetry",group:"hardware|cpu|memory|disk|network|template",values:{key:{name,value,unit,status,entity,interval_seconds,age_ms}}}`。值为字符串，unit 为 text/percent/bytes/bytes_per_sec/mhz/bits；status 为 ok/unknown/waiting/error；age_ms 为采集结束到发送的单调时钟年龄，最大一天。单事件最多 64 KiB、256 项；模板最多38项，值最多4096 UTF-8 bytes，其余最多512。name最多128，entity最多512。组报文为完整组快照，template 为按 key 增量。模板按载荷上限拆分，单项过大上报error；内置组超过载荷上限时整组替换为collection_status错误，不能静默保留旧成功值。
- 各组值独立保留采样时间和周期，Server 接收时间减 age_ms 得到 sampled_at；不依赖设备墙钟。来源由 Server 根据事件组赋值。失败清除旧成功值，新 Session 不继承旧实时采样。
- 固定指标标识：cpu_model/cpu_soc/cpu_arch/cpu_hardware_bits/kernel_arch/kernel_bits/probe_bits/cpu_max_mhz/cpu_usage/cpu_current_mhz，memory_usage/memory_used_bytes/memory_total_bytes/memory_available_bytes/memory_method。网口标识 net_<接口名UTF8的hex>_<rx_bytes_per_sec|tx_bytes_per_sec|state|kind>。存储使用 disk_<挂载路径FNV1a64>_<usage|used_bytes|total_bytes|available_bytes|kind>，entity 保存完整路径；稳定摘要仅用于满足64字符标识限制，同轮碰撞不合并。界面显示可复制标识供模板覆盖。
- 内置 CPU/内存/网口默认5秒、存储60秒；0关闭，1～86400秒。CLI --cpu-interval/--memory-interval/--disk-interval/--network-interval 优先于模板 monitoring 设置；未选模板仍默认启用。硬件信息每会话首报一次。
- 模板 monitoring 可选，字段 cpu_seconds/memory_seconds/disk_seconds/network_seconds，缺省采用默认。属性 interval_seconds 缺省或0仅启动采集，1～86400周期采集。生成器工程升级版本3并兼容1/2。配置及模板版本在 Probe 重启时生效。
- REGISTER 可选 report_intervals 保存所选模板每项周期，与模板注册成功/失败键一致。模板实时更新只允许这些键；仅启动属性使用注册结果（采样时间为注册接收时间）；周期属性新Session先waiting、立即触发重新采集，不把旧注册值标为新采样。显式 hostname 仍优先。
- CPU忙碌比例排除idle/iowait，guest不重复计入总量；CPU与网口差分首次等待，计数回退重置；memory 使用 total-available，旧内核回退 free+buffers+cached+reclaimable-shmem（有界估算并标注）。存储按文件系统容量、过滤虚拟文件系统，不累计桥/虚拟网口或重复挂载。百分比0～100，容量非负；硬件当前/最大频率分别显示。
- Device Service 生成 effective_metrics：模板同键始终优先（含失败），不按谁更新更快决定。内置数值键的模板输出按内置单位校验，非法显示 error，不回退。过期阈值 max(3*interval,15秒)；仅启动属性不按周期过期，离线另行标记。现有 registration 不被周期改写，工具匹配不变。
- 仅保存每 Session 最新组与最终属性；复用 devices revision/WS合并通知、HTTP快照恢复。不新增历史数据库或数据面。

## 来源 IP 与归属地

Server 由 TCP RemoteAddr 记录 source_ip，忽略 Probe 自报来源，API 在 Session/Device 返回。地址表示 Server 观察到的连接对端，NAT/代理场景为出口/代理地址。

ADR-040当前WPF为所选设备Probe上报的公网出口IPv4/IPv6异步查询 `https://ipwho.is/<IP>?lang=zh-CN&fields=ip,success,country,region,city,connection.isp`，只发送IP。私网、CGNAT、链路本地、回环、多播、文档和保留地址不查询。查询有超时、响应大小限制、有限缓存和失败退避；切换设备/Server及退出取消，迟到响应不能覆盖选择。属性分别显示归属地与运营商；界面不再附加查询来源说明，归属地仍不代表设备精确位置。服务不可用时保留IP、显示横杠、悬停失败原因，不影响设备控制。

接口依据：[ipwhois.io documentation](https://ipwhois.io/documentation)，2026-09-08核对支持HTTPS、IPv4/IPv6、免费端点无需密钥、商业使用及每日限额；程序不自动购买额度。
