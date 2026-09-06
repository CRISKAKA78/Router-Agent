# 项目状态

最后更新时间：2026-09-06

## 当前阶段

Phase 0～4已完成实现与规定验收。当前Phase 4修正采用Accepted ADR-021 / ADR-022，保留自研TCP Maintenance。修正基线为 `f92d73a0d6003835993967848f1f5fe009a0df89`；启动时fetch后HEAD/main/origin/main一致，工作区干净。

修正提交标题：`fix: isolate Phase 4 ports and decouple maintenance revocation`；实际SHA和推送状态以Git记录为准。独立提交推送GitHub main后停止等待验收，不进入Phase 5。

## 当前可验证能力

- `internal/tunnel.Service`一次创建web/ssh/telnet三个临时入口，固定Probe的127.0.0.1:80/22/23；默认240分钟，支持自定义正租期。
- 每外部客户端独立Probe data TCP；128-bit Maintenance/connection身份、256-bit一次性token，RMT1精确握手后双向原始TCP Relay。Web多连接、三通道同时工作、多设备隔离、真实HTTP/SSH登录命令/Telnet交互通过。
- Maintenance严格绑定创建时Device Session。关闭、到期、替换、失联先撤入口再终止pending/active流；Close幂等等待本地释放，不等待控制writer。Gateway每Session一发送worker和64项队列；data reset也能回收已读EOF的空闲Probe流。新Session与Server/Probe重启不继承旧Maintenance。
- 端口本地释放后默认隔离24小时，Released与ReusableAfter分别表达释放和最早复用；隔离独立于关闭历史，满池拒绝提前复用。DataHost域名由Server在每次Create解析并固定IP，Probe无DNS逻辑。
- 支持half-close/EOF排空、固定缓冲、背压；默认整连接idle24小时，租期优先。Probe默认8个worker、允许1～64，每流一线程，控制Session退出取消和join，C++11、无新增第三方运行依赖；Server每维护/设备默认8条流。
- `management.Server.Maintenance()`暴露内部Create/Get/List/CloseMaintenance；Server的`-tunnel-*`与Probe的`--tunnel-connections`可配置。默认loopback绑定，部署者显式配置公网bind和可达地址。
- Phase 1任务/文件幂等、控制优先、上传下载与提交/确认分离；Phase 2 Device/Session历史；Phase 3持久Repository、版本/兼容/归档及工具投放保持，全量回归通过。

## 验证结果

完整映射、命令及环境见[PHASE4_VERIFICATION](PHASE4_VERIFICATION.md)。历史记录保留于PHASE1/2/3_VERIFICATION。

| 验证 | 结果 |
| --- | --- |
| Linux Release C++11 / CTest | 通过，4/4，4.71秒 |
| Linux完整Go与真实Probe Phase 1～4回归 | 通过，集成169.774秒 |
| ASan/UBSan/LSan CTest | 通过，4/4，5.97秒 |
| ASan真实Probe Phase 4 | 通过，14.905秒 |
| TSan CTest | 通过，4/4，6.85秒 |
| Go race + TSan真实Probe完整回归 | 通过，集成177.103秒 |
| Windows原生测试 / vet / Server构建 | 通过；真实Linux Probe集成在Linux执行 |
| Linux vet / Server构建 | 通过 |

## 已知边界

- 仅固定TCP 80/22/23；无任意端口、UDP、SOCKS、VPN、P2P、HTTP反向代理/改写、TLS终止、多级映射或重启恢复。
- ready表示Server入口已监听，本地目标在实际建流时验证；失败只影响本服务/本连接，后续可重试。Released无Probe跨网络释放ACK。原始TCP不识别外部客户端维护身份；隔离窗口后或Server重启后不保证旧地址永久隔离，永久隔离需不重叠池/地址。默认200端口池在隔离窗口内最多66次三入口创建，按频率配置容量。
- API仍为内部Go Service，未实现HTTP/WebSocket、UI、外部操作CLI、MCP或AI Agent。认证、TLS、权限与完整审计仍为既有后续主题；data token仅用于流配对。
- Probe保持Linux/C++11；mipsel/ARM/ARM64、uClibc/最低内核实机矩阵尚未覆盖，不能从Linux x86_64测试推断实机已验收。
- Repository仍为本地单写者JSON目录与不可变blob；归档不回收磁盘，无在线备份/迁移或物理GC。仅Repository跨重启保留；Device/Task/File/Operation仍为进程内状态，原有限额、hard-link发布等限制不变。

## 下一步

等待用户验收Phase 4；不得自行进入Phase 5。
