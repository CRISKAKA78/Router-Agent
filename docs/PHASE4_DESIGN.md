# Phase 4 极简 TCP Maintenance Tunnel

2026-09-06，实施前设计。基线 HEAD/main/fetch 后 origin/main 均为 `ceaaab791850746911f965167c885789444efd4c`。启动发现上轮 FRP PoC 未提交材料，已完整保存至 Git stash `preserve previous FRP Phase 4 PoC before minimal TCP Tunnel`；stash 后工作区干净。不复用该 PoC 源码或验收结论。

## ADR-021（Accepted，本次用户授权范围内的设计）

取代此前未提交的 ADR-020 FRP/xfrpc 方向及其 backend 门槛。原因：用户明确放弃 FRP，要求固定三服务的自研原始 TCP Relay。旧 ADR 与失败证据完整保存在上述 stash；ADR-009～019 保持。此决定不引入通用端口映射、外部 API 或 Phase 5。

Maintenance Service 拥有 Create/Get/List/Close、端口池、租约、配对和 Relay。Gateway 仅提供当前 Session 绑定、受绑定约束的控制发送和状态路由。Application 组合两者。Maintenance ID、connection ID 为独立随机 128-bit 小写 hex，token 为 256-bit 随机小写 hex；均不复用、不持久化。一个维护会话固定 web/ssh/telnet，目标仅为 Probe 的 127.0.0.1:80/22/23。默认 240 分钟，调用者可自定义至少1ms的正租期；0 选择默认。每设备仅一个未释放 Maintenance，重复 Create 返回冲突，调用者查询现有会话。

端口池默认 20000～20199，监听默认 127.0.0.1，advertised host 默认 127.0.0.1；data listener 默认 127.0.0.1:9001，data advertised IP 默认为 127.0.0.1。部署者显式配置公网绑定和地址；data IP 使用数值 IPv4/IPv6，避免 Probe worker 内不可取消 DNS。公网入口返回 host+port，不建立 NAT、防火墙或证书。创建三端口是原子操作，失败回收全部临时 listener。ready 表示入口监听可用，本地服务可达性在实际建流时验证；失败变 unavailable，后续成功变 ready，其他服务不变。

控制消息新增 TUNNEL_CONNECT 0x40、TUNNEL_CLOSE 0x41、TUNNEL_STATUS 0x42，flags=0；不使用 TASK 缓存，不重发跨 Session 命令。CONNECT 字段：session_id、maintenance_id、connection_id、service、token、data_host、data_port、timeout_ms、idle_ms。CLOSE 字段：maintenance_id，connection_id（空字符串关闭整个 Maintenance）。STATUS 字段：maintenance_id、connection_id、state（local_unavailable/data_failed/busy）。Gateway 以收到消息的 Session 绑定路由，不信任载荷指定来源。

独立数据连接固定握手：ASCII `RMT1` + 32-byte maintenance_id + 32-byte connection_id + 64-byte token，共 132 bytes，无换行；Server 精确读 132 bytes，原子核对绑定、pending、租约和 token 后一次性消费。成功回复单字节 0x01，随后全双工原始 TCP。错误/重复/迟到握手直接关闭，不影响真正 pending 流。无数据连接自动重连，无旧租约重启恢复。

默认 pending 建流总时限 10s，data 握手 5s；Probe CONNECT 的 timeout_ms 覆盖本地连接、data 连接和握手。活动连接默认每方向 I/O 空闲时限 5 分钟，可配置；租约始终为绝对上限。EOF 仅关闭对端写半部，反向数据可继续；错误/关闭/到期终止两端。Server 每方向 32KiB、Probe 每方向 16KiB 缓冲，读随写背压；不累积全流。

Server 默认每维护 32、每设备 64、全局 512 条 pending+active 外部连接；未配对 data 握手另限 64；Maintenance 数量受端口池和配置上限 64 约束。Probe 默认最多 64 个 worker，完成 worker 在控制循环回收，关闭控制 Session 时取消并 join。容量满直接拒绝，无无界等待队列。关闭记录按配置保留最近 128 个；不保存流历史/token。

创建先取得 Session 生命周期句柄，再分配资源；发布前复核句柄，Session 替换不能继承。Gateway 同步使旧句柄失效；所有入口准入和握手复核句柄，生命周期 watcher 发起关闭。Close 幂等且等待资源释放：先关闭三个 listener，再取消 pending/active 并关闭所有 Server socket，等待 accept/dispatch/relay workers 完成，最后归还池端口并标记 closed/released。Probe 收到 CLOSE 或控制 Session 结束，取消目标 worker 并关闭本地/data socket。Server 关闭后 data listener、未握手 socket 与其 worker 也全部回收。

平台认证、加密、授权仍为既有后续主题；随机配对 token 只解决流配对，不替代平台身份或外部客户端认证。
