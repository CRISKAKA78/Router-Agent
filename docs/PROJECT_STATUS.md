# 项目状态

最后更新时间：2026-09-06

## 当前阶段

Phase 0～3已交付。Phase 4极简自研TCP Maintenance已完成实现与规定验收，采用Accepted ADR-021，放弃FRP/xfrpc。基线为用户确认的 `ceaaab791850746911f965167c885789444efd4c`；启动时fetch后HEAD/main/origin/main一致，旧未提交FRP PoC完整stash后工作区干净。

交付提交标题：`feat: complete Phase 4 TCP maintenance tunnels`；实际SHA和推送状态以Git记录为准。独立提交推送GitHub main后停止等待验收，不进入Phase 5。

## 当前可验证能力

- `internal/tunnel.Service`一次创建web/ssh/telnet三个临时入口，固定Probe的127.0.0.1:80/22/23；默认240分钟，支持自定义正租期。
- 每外部客户端独立Probe data TCP；128-bit Maintenance/connection身份、256-bit一次性token，RMT1精确握手后双向原始TCP Relay。Web多连接、三通道同时工作、多设备隔离、真实HTTP/SSH登录命令/Telnet交互通过。
- Maintenance严格绑定创建时Device Session。关闭、到期、替换、失联撤销入口并终止pending/active流；Close幂等等待Server相关资源释放，随后归还端口。新Session与Server/Probe重启不继承旧Maintenance。
- 支持half-close/EOF传播、固定缓冲、背压；有每维护/设备/Server连接限额、未配对握手限额、建立/空闲超时、有界关闭历史。Probe默认64个有界worker，控制Session退出取消和join，C++11、无新增第三方运行依赖。
- `management.Server.Maintenance()`暴露内部Create/Get/List/CloseMaintenance；Server的`-tunnel-*`与Probe的`--tunnel-connections`可配置。默认loopback绑定，部署者显式配置公网bind和可达地址。
- Phase 1任务/文件幂等、控制优先、上传下载与提交/确认分离；Phase 2 Device/Session历史；Phase 3持久Repository、版本/兼容/归档及工具投放保持，全量回归通过。

## 验证结果

完整映射、命令及环境见[PHASE4_VERIFICATION](PHASE4_VERIFICATION.md)。历史记录保留于PHASE1/2/3_VERIFICATION。

| 验证 | 结果 |
| --- | --- |
| Linux Release C++11 / CTest | 通过，4/4，4.71秒 |
| Linux完整Go与真实Probe Phase 1～4回归 | 通过，集成167.713秒；新增背压/心跳断言补充Phase 4为12.768秒 |
| ASan/UBSan/LSan CTest | 通过，4/4，5.91秒 |
| ASan真实Probe Phase 4 | 通过，12.943秒 |
| TSan CTest | 通过，4/4，6.84秒 |
| Go race + TSan真实Probe完整回归 | 通过，集成174.726秒 |
| Windows原生测试 / vet / Server构建 | 通过；真实Linux Probe集成在Linux执行 |
| Linux vet / Server构建 | 通过 |

## 已知边界

- 仅固定TCP 80/22/23；无任意端口、UDP、SOCKS、VPN、P2P、HTTP反向代理/改写、TLS终止、多级映射或重启恢复。
- ready表示Server入口已监听，本地目标在实际建流时验证；失败只影响本服务/本连接，后续连接可重试。Released表示Server本地资源释放，没有Probe跨网络释放ACK；Probe取消由CLOSE或控制Session结束执行。关闭历史端口可能已复用，须检查State。
- API仍为内部Go Service，未实现HTTP/WebSocket、UI、外部操作CLI、MCP或AI Agent。认证、TLS、权限与完整审计仍为既有后续主题；data token仅用于流配对。
- Probe保持Linux/C++11；mipsel/ARM/ARM64、uClibc/最低内核实机矩阵尚未覆盖，不能从Linux x86_64测试推断实机已验收。
- Repository仍为本地单写者JSON目录与不可变blob；归档不回收磁盘，无在线备份/迁移或物理GC。仅Repository跨重启保留；Device/Task/File/Operation仍为进程内状态，原有限额、hard-link发布等限制不变。

## 下一步

等待用户验收Phase 4；不得自行进入Phase 5。
