# Phase 4 极简 TCP Maintenance Tunnel

2026-09-06 当前设计。首版从干净 Phase 3 基线 `ceaaab791850746911f965167c885789444efd4c` 实现，形成 `f92d73a0d6003835993967848f1f5fe009a0df89`。本轮从该 main 干净基线修正；采用 ADR-021 与 superseding ADR-022。

## ADR-021 / ADR-022（Accepted，用户授权范围内的设计）

取代此前未提交的 ADR-020 FRP/xfrpc 方向及其 backend 门槛。原因：用户明确放弃 FRP，要求固定三服务的自研原始 TCP Relay。放弃方向的历史摘要已归档在 DECISIONS.md 的 ADR-020；ADR-009～019 保持。此决定不引入通用端口映射、外部 API 或 Phase 5。

Maintenance Service 拥有 Create/Get/List/Close、端口池、租约、配对和 Relay。Gateway 仅提供当前 Session 绑定、受绑定约束的控制发送和状态路由。Application 组合两者。Maintenance ID、connection ID 为独立随机 128-bit 小写 hex，token 为 256-bit 随机小写 hex；均不复用、不持久化。一个维护会话固定 web/ssh/telnet，目标仅为 Probe 的 127.0.0.1:80/22/23。默认 240 分钟，调用者可自定义至少1ms的正租期；0 选择默认。每设备仅一个未释放 Maintenance，重复 Create 返回冲突，调用者查询现有会话。

端口池默认 20000～20199，监听及 advertised host 默认 127.0.0.1；data listener 默认 127.0.0.1:9001，DataHost 默认 127.0.0.1。DataHost 支持 DNS 主机名，Server 在每次 Create 用系统解析器进行最多 5 秒、受调用 context 取消的解析，优先 IPv4；本次维护固定所得 IP。并行 Create 数量受 MaxMaintenance 限制，解析不持锁，失败不分配入口。DNS 更新用于下一次维护，不做多地址故障切换或 split-horizon 自动选择；部署者确保解析所得地址从 Probe 可达。Probe 仍只处理数值 IP，无 DNS worker。公网入口返回 host+port，不建立 NAT、防火墙或证书。创建三端口是原子操作，失败回收全部临时 listener。ready 表示入口监听可用，本地服务在实际建流验证；失败变 unavailable，后续成功恢复。

端口完成本地释放后进入 PortReuseDelay 隔离，默认 24 小时。Released 不等于可立即复用；ReusableAfter 给出最早复用时间。隔离记录独立于关闭历史，不占 listener 或 goroutine，最多端口池大小；池耗尽返回 ErrCapacity，不提前使用隔离端口。默认池最多容纳 66 次三入口分配，隔离期间的使用量也占池容量，部署可按维护频率扩大池。原始 TCP 无客户端维护身份，有限隔离期只能抑制窗口内旧客户端重连，不能保证永久隔离；超过窗口或 Server 重启后可能重用旧地址。需要跨重启或永久隔离的部署必须使用不重叠的池/地址。配对 token 不认证外部客户端。

控制消息新增 TUNNEL_CONNECT 0x40、TUNNEL_CLOSE 0x41、TUNNEL_STATUS 0x42，flags=0；不使用 TASK 缓存，不重发跨 Session 命令。CONNECT 字段：session_id、maintenance_id、connection_id、service、token、data_host、data_port、timeout_ms、idle_ms。CLOSE 字段：maintenance_id，connection_id（空字符串关闭整个 Maintenance）。STATUS 字段：maintenance_id、connection_id、state（local_unavailable/data_failed/busy）。Gateway 以收到消息的 Session 绑定路由，不信任载荷指定来源。

独立数据连接固定握手：ASCII `RMT1` + 32-byte maintenance_id + 32-byte connection_id + 64-byte token，共 132 bytes，无换行；Server 精确读 132 bytes，原子核对绑定、pending、租约和 token 后一次性消费。成功回复单字节 0x01，随后全双工原始 TCP。错误/重复/迟到握手直接关闭，不影响真正 pending 流。无数据连接自动重连，无旧租约重启恢复。

默认 pending 建流总时限 10s（含发送排队），data 握手 5s；Probe CONNECT 的 timeout_ms 覆盖本地连接、data 连接和握手。活动连接默认空闲时限 24 小时，任一方向读写推进刷新整条连接期限；配置范围 1ms～24h，租约始终为绝对上限，默认 4 小时维护不会因为 5 分钟无操作断线。EOF 仅关闭对端写半部，反向可继续；异常撤销 data TCP 使用 reset，Probe 即使已见正常读 EOF 也继续检查 socket 错误。普通 EOF/HUP 保留缓冲数据排空，不将其当 reset。Server 每方向 32KiB、Probe 每方向 16KiB 缓冲，读随写背压；不累积全流。

Server 默认每维护 8、每设备 8、全局 512 条 pending+active 外部连接；未配对 data 握手另限 64；未释放 Maintenance 上限 64。Probe 默认最多 8 个 worker，配置 1～64；保持每流一个 C++11 thread，完成后控制循环 join，Session 结束取消并 join。默认 Tunnel 用户态 Relay 缓冲上限 256KiB，另有 socket、线程栈等内存；线程栈取决于目标 libc。低内存设备可调小，需要更多浏览器 TCP 并发时须同时提高 Server 与 Probe 限额。容量满直接拒绝，关闭历史默认保留 128 项；不保存流历史/token。

创建先取得 Session 生命周期句柄，再解析地址和分配资源；发布前复核，Session 替换不能继承。Gateway 同步使旧句柄失效；入口及握手复核句柄，生命周期 watcher 发起关闭。Gateway 每个支持 Tunnel 的控制 Session 只有一个发送 worker，队列最多 64 个消息；Enqueue 只做有界准入，满时立即返回 ErrCapacity。writer 写前再核对 CONNECT context 与 Session，过期消息不派发。网络写失败沿用既有控制 Session 断线语义。

Close 幂等且等待本地释放：先关闭三个 listener，再取消 pending/active；先 reset data socket 再关闭外部 socket，避免并发 Relay 抢先发送普通 FIN。等待 accept/已配对握手/relay 完成后，端口进入隔离并标记 closed/Released；不等待 Gateway 发送 worker，CLOSE 入队为尽力通知。Probe 收到 CLOSE、data reset 或控制 Session 结束取消并关闭本地/data socket。Server 关闭还清理全局 data listener 和未握手资源；Gateway 关闭 control socket 后 join 自己的发送 worker。

平台认证、加密、授权仍为既有后续主题；随机配对 token 只解决流配对，不替代平台身份或外部客户端认证。
