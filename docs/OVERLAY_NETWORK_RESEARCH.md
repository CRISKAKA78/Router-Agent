# 异地组网调研与初步改造方案

> 历史调研快照。用户现已审核并授权ADR-059；当前实现、接线及未完成验收以 [OVERLAY_NETWORK](OVERLAY_NETWORK.md) 为准，下文待审核标记不再阻止已授权改造。


- 日期：2026-09-11。
- 状态：**Proposed / 待用户审核**。本轮仅创建独立 worktree、核对官方发布与源码、形成方案；没有实现、部署或启用组网，没有改动现有 Accepted ADR/API/协议。
- 分支：`codex/overlay-network-research`；worktree：`C:/Users/Administrator/Desktop/router-agent-overlay-network`。
- 起点：`f331ffb8c05fd73fbfb5a186508b476f45c4020d`，创建时原 main 工作区干净。
- 授权解释：用户明确授权研究原暂缓范围内的异地组网，不等于批准新增数据面、安全模型或持久化生命周期的实现。审核后以新 ADR 明确扩展 ADR-028 的范围；不静默改写既有维护契约。

## 1. 建议结论

**首选 EasyTier 2.6.4，作为独立三层组网进程；Router-Agent 管理网络、成员、配置与观测；二层以 VXLAN 优先，GRETAP 次选，EoIP 作为厂商兼容项。**

这不是把现有 Maintenance 扩展成 VPN，也不是把 EasyTier 源码移入 Probe。Probe 保持 C++11/GCC5.2 轻量控制端，EasyTier 使用单独的官方目标架构程序。用户数据走 EasyTier 的加密 P2P/中继连接，不进入 9000 控制链路，也不复用 9001 维护数据面。

建议产品分两步理解：

1. **设备三层互通**：在线、已纳管且通过能力检查的路由器获得稳定虚拟 IP，彼此 IP 可达；不是承诺所有在线设备都能运行组网程序。
2. **站点网络扩展**：按需开放路由器后面的指定 LAN 子网，或者建立独立二层广播域。二者不是默认自动发生的同一件事。

首版推荐 IPv4、每设备一个活动组网实例、WPF 仅作管理端；多网络并行、重叠网段隔离、Windows 管理机自动入网暂不默认开启。数据模型可以记录网络与成员关系，但不先建设通用多引擎框架。

## 2. 版本与候选比较

2026-09-11 实际查询三个官方仓库的 GitHub `releases/latest`，并下载对应 tag 源码核对；以下是该接口返回的非预发布稳定版，不把开发分支或 nightly 当稳定版。后续实施前重新核对发布，正式投放固定经过测试的版本/产物，不自动跟随 latest。[S1][S5][S7]

| 项目 | 本次官方稳定版 / UTC 发布日期 | 与本项目相关的能力 | 初步判断 |
| --- | --- | --- | --- |
| EasyTier | **v2.6.4 / 2026-05-12** | 分布式三层互联、P2P/中继、子网代理、多种外层传输；CLI JSON、节点/链路/路由及统计接口；发布 ARM/ARMv7 软硬浮点、AArch64、MIPS/MIPSel 等 Linux 包 | **首选**；与用户优先级一致，观测和多跳路由适合拓扑展示，但旧固件运行、GRE与ACL组合必须实测 |
| VNT | **v2.0.7 / 2026-09-05** | VNT 2.x 有 TUN/TAP/无网卡模式、P2P/服务端中继、CLI/IPC/HTTP 管理能力；提供相应 ARM/MIPSel musl 包 | **有效备选**，不能按 VNT 1.x 的能力评价；需单独验证匹配服务端、自建部署及资源占用 |
| ZeroTier One | **1.16.2 / 2026-05-28** | 原生虚拟以太网、网络成员控制、节点本地 JSON API；Controller 与根节点/月球节点职责独立 | 原生二层有吸引力，但不同于本次“三层底座+可选二层”的优先结构；自建与产品分发许可需单独评估 |

### 2.1 EasyTier：为什么优先

官方 README 描述了无中心组网、NAT 穿透、子网代理和中继能力；这不表示任何 NAT/防火墙组合都能直连。[S2]

固定 tag 源码已核对：[S3][S4]

- `easytier-cli --output json`，默认 RPC 地址为本机 `127.0.0.1:15888`；部署时仍须显式限制服务端RPC仅监听回环，不能由CLI默认值推定服务端绑定安全。优先以 CLI JSON 适配本地控制，不解析人类表格文本，也不把内部 RPC 当作稳定 REST/gRPC API。
- `api_instance.proto` 含 PeerInfo、Route、NodeInfo、收发字节/包数、链路延迟与 loss_rate、next_hop 等字段，可支撑详情和真实观测拓扑。
- `api_manage.proto` 含多实例配置、运行/保留/查询等模型，但首轮不引入上游 Web Console 与第二套用户体系；我们的 WPF 仍走自己的 API。
- 存在 Secure Mode、ACL 与 CredentialManageRpc（签发/吊销/列举凭据）能力；逐设备准入可以优先验证这些现成机制，但不能仅凭 schema 承诺吊销即时生效或所有模式互通。
- 2.6.4 的 Windows UDP 广播中继**不是通用二层交换**，不能据此宣称 Linux TUN 能直接桥接 Ethernet。[S1]

### 2.2 VNT：保留为真正候选

2.0.7 的 README 已明确 TUN 是三层 IPv4、TAP 透传 Ethernet；`vnt2_web` 默认本机监听，`vnt2_cli`/`vnt2_ctrl` 可无 GUI 使用。源码 HTTP 路由包含 info/peers/routes/start/stop/config，IPC 包含链路 RTT、丢包等字段。[S5][S6]

本次已核对客户端仓库，没有完成配套服务端版本、资源实测和 Router-Agent 接入验证。若 EasyTier 在目标旧固件上无法满足运行或资源条件，再进行同场景 VNT 对照；不同时实施两个引擎，也不承诺它必然更省内存。

### 2.3 许可证与自建边界

本次固定 tag 文件：EasyTier 为 LGPL-3.0，VNT 客户端仓库为 Apache-2.0；ZeroTier 1.16.2 的根 LICENSE 明确普通 Agent 代码与 `nonfree/` 采用不同许可证，Controller 位于 `nonfree/`，该目录条款明确商业使用需单独许可。[S9]

这是发布选型事实，不代替许可审查；独立进程打包也不会自动免除开源告知/分发义务。VNT 配套服务端的许可证需要另查。不要沿用记忆把当前 ZeroTier 一概写成 BSL 或把 Controller 当成与 Agent 同许可。

ZeroTier 官方协议说明区分 VL1 物理 P2P 路径和 VL2 虚拟网络；自建 Controller 不等于替换 planet，moon 也不是 Controller 或“完全脱离官方基础设施”的同义词。[S8]

## 3. 最关键的兼容性门槛

### 3.1 旧路由器不是“下载 ARM 包即可”

当前仓库 Probe 构建证据为 GCC5.2、ARMv7 小端/EABI5/uClibc；同时保留 MIPSel/ARM64 与旧内核目标。EasyTier 官方构建矩阵有 musl 目标及 ARM 软/硬浮点区分，但**静态 musl 不代表任何 uClibc 固件/老内核都能运行**。[S2][S3]

审核后的 PoC 需要先读取实际目标信息，不预设型号、内核或指令集：

- CPU 架构、端序、ARM ISA/浮点 ABI、内核版本、RAM/闪存与持久可写目录。
- TUN 内核支持与 `/dev/net/tun`；设备节点存在不等于驱动可用。
- 权限、进程创建、所需内核接口，实际创建 TUN 和配置 IP/路由是否成功。
- BusyBox 的 `ip` 不能等同完整 iproute2；逐项检测内核与用户态工具。
- 空闲 RSS、双向吞吐、CPU、掉线恢复、外网 MTU，不从压缩包大小推算 RAM 或性能。

EasyTier 自有 Rust/musl 构建链不塞进 `/root/gcc-5.2`；若官方包不兼容，再讨论独立构建目标或更换候选。现有 Probe 构建机 10.1.1.128、产物与部署规则不变。

### 3.2 真 TUN 是二层承载基础

应使用创建真实三层虚拟接口的 EasyTier 模式。仅 SOCKS、端口映射、无 TUN 或用户态 TCP/UDP 子网代理，不能直接视为等价的“任意 IP 协议承载”。源码存在 TUN 原始 IP 包与按目的 IP 转发路径，因此在其上封装 VXLAN/GRE 有技术基础；但 ACL、包处理插件、MTU、直连/中继组合仍需协议级实测。[S4]

尤其 GRETAP/EoIP 是 GRE/IP 协议号 47，不是 TCP/UDP 端口。EasyTier 当前 ACL proto 没有专用 GRE 枚举；不能误填“UDP 47”当作放通。需验证 Any 规则及设备防火墙的实际行为。[S4][S11]

## 4. 二层扩展方案

```text
站点 A 指定 LAN/VLAN                    站点 B 指定 LAN/VLAN
          │                                      │
       专用 bridge                           专用 bridge
          │                                      │
       VXLAN-A  ═══ 内层 UDP/IP 或 GRE/IP ═══ VXLAN-B
          │                                      │
     EasyTier 虚拟 IP A                  EasyTier 虚拟 IP B
          └──────── 加密 P2P / 中继 ──────────────┘
```

TUN 不直接加入以太网桥。桥接对象是二层隧道接口与用户指定 LAN/VLAN；TUN 只提供二层隧道两端的 IP 可达性。

| 类型 | 定位 | 约束 |
| --- | --- | --- |
| VXLAN | **默认优先**，更适合多个站点组成同一二层域 | 依赖内核 VXLAN、bridge/FDB 与匹配 iproute2；显式指定 UDP 4789，不能假设旧内核默认端口一致；优先单播复制/FDB，不要求 Overlay 多播 |
| GRETAP | 点对点备用，适合已有内核支持的设备 | 标准 GRE TAP、需要 GRE 47 穿过真实 TUN；多点要组合多个点对点口，必须设计防环 |
| EoIP | MikroTik 等既有环境兼容 | RouterOS 的 GRE 派生协议，不等于标准 Linux GRETAP；Linux 端需要单独兼容实现，不能承诺普通 `ip link` 即支持 |

依据：Linux 内核 VXLAN 文档、iproute2 GRETAP 手册与 MikroTik EoIP 官方说明。[S10][S11][S12]

二层网络必须单独建模为 `L2Segment`，记录关联 L3 网络、成员、隧道类型、VNI/tunnel-id、虚拟 IP 端点、bridge/LAN 接口、MTU、管理员期望与观测状态。VXLAN 段里的 VNI/端点参数需要一致，不能把三种协议作为可互通的标签随意替换。

默认策略建议：

- 新建专用 bridge/VLAN，不自动把现有 `br0` 或管理口桥出去；首轮用隔离测试 LAN。
- 同一二层段统一一种隧道方案；先两站点，再验证三站点。VXLAN 使用 split-horizon/受控复制，GRETAP 使用无环拓扑并验证 STP；不能盲目全互联再桥接。
- 明确单一 DHCP 服务端策略、广播限制及重复网关/IP 检查；同子网不是无冲突证明。
- MTU 分层计算。内层 IPv4 VXLAN 的基础开销为 IP 20 + UDP 8 + VXLAN 8 + Ethernet 14 = 50 字节（未计额外 VLAN）；桥接端 IP MTU不高于实测可用 Overlay MTU减相应开销。不要直接设 1500，也不要只靠 TCP MSS 掩盖 UDP/二层大包问题。
- 二层改动采用本地超时回滚与管理可达性检查；只删除本功能拥有的接口/路由/规则。

## 5. 与现有代码的最小集成边界

```text
WPF 组网工作区
    │ 公开 /api/v1 + WS 失效提示
HTTP Adapter → Management Application → 新增组网 Service
                                         │
                              既有 Task / File / Gateway
                                         │ 控制/状态，不传用户流量
                                  Probe 组网适配模块
                                         │ 本机受控 CLI JSON
                                  easytier-core 独立进程
                                         ║
                           其他设备 / 可选自建中继节点
```

| 层 | 已核对的实际入口 | 拟改造内容（均待审核） |
| --- | --- | --- |
| Go Server | `internal/management/server.go`、`service.go`、`neighbors.go`；`internal/api` | 独立组网 Service，网络/IP 分配、成员、配置代次、观测与操作编排；HTTP 只调用 Application，不直接碰连接注册表 |
| 资产与任务 | `internal/repository`、`internal/filetransfer`、`internal/task`、`internal/gateway` | 复用现有 artifact/兼容/文件完整提交；安装与启用显式分步，不把原“工具投放不自动执行”改成自动启动 |
| Probe | `probe/src/task_manager.cpp`、`client.cpp`、`telemetry.cpp` | 新能力声明，限定组网操作及本机进程/状态采集；argv传参、超时/输出上限/退避；不通过长期通用 exec 或模板采集脚本维持 VPN |
| .NET Client | `windows/RouterWorkbench.Client` | 新 DTO、公开 API、HTTP 快照与 WS 刷新；不连接设备 EasyTier RPC，不复制状态机 |
| WPF | `MainWindow.xaml(.cs)`、`DeviceViews.cs`、共享 Themes | 新的全局“异地组网”工作区及设备成员摘要；保留既有五个工作区、字体/主题与选择行为 |
| Blazor 生成器 | `src/ProbeTemplateGenerator` | 首轮不放网络密钥/成员/动态运行状态进模板；仅当固件能力参数确有共性，再提出必要扩展 |

Probe 部分建议暴露结构化 `network_apply`、`network_stop` 等任务与 `network_status` EVENT，名称/载荷不是本轮定稿。长生命周期由独立组网模块负责，任务只代表一次安装/应用/停止操作，不等于 VPN 的整个寿命。固定版本 CLI JSON 可先适配；无必要不在 C++ 内重写 EasyTier RPC 协议。

### 5.1 状态、持久化与失败语义

建议分开记录：

- Network：网络ID、引擎及固定版本、虚拟 CIDR、IP 分配、允许的子网与策略。
- Membership：稳定 device_id、虚拟 IP、instance 标识、desired/applied revision、启停意图。
- Observation：当前 session/generation、引擎状态、peer/route、采样时间、缺失/过期/截断标记。
- Operation：task_id、原幂等键与原字节、逐设备成功/失败/不确定，不声称多设备变更是原子事务。

网络成员/IP 分配建议原子落盘到 Server 自有目录，不新增外部数据库；配置保留和任务跨重启恢复是不同问题。重启后查询实际状态并对账，不自动重放旧 Task，不把无响应视为成功，待确认操作不自动换新 task_id。

推荐管理连接断开时，已建立的组网保持运行；UI 标记“管理离线，数据面状态未知”，不能沿用 Maintenance 的 Session 结束即撤销，也不能误报数据面仍在线。是否采用该生命周期需审核并写入新 ADR。用户停止/删除网络时必须显示未确认设备，离线节点不能承诺立即停机；成员凭据撤销与数据面隔离效果另行确认。

成员删改采用配置代次，旧结果不得覆盖新配置；固定虚拟 IP 防止重连后隧道端点漂移。网络 CIDR先做与已知 LAN/路由/维护目标冲突检查，采样不足则提示需要复核。首版不自动解决不同站点相同 LAN 网段。

### 5.2 公开接口草案

均为新资源草案，**当前没有这些 API**：

- `GET/POST /api/v1/networks`
- `GET/PATCH /api/v1/networks/{id}`
- `GET/POST /api/v1/networks/{id}/members`
- `GET /api/v1/networks/{id}/topology`
- `POST /api/v1/networks/{id}/operations`：启用、停止、应用配置；逐成员结果。
- `GET/POST /api/v1/networks/{id}/l2-segments`：二层获批后实施。
- WS新增组网topic，保持“通知失效→HTTP取快照”，首连/重连重查；大拓扑分页/限量，不无限增大控制帧。

继续遵守现有 /api/v1 兼容性与幂等规则。UI不能显示既有 Maintenance 私有 token/connection_id；EasyTier 内部 conn_id、秘密配置也在适配层过滤，前端使用稳定业务节点/边标识。

### 5.3 安全与部署：不是默认已解决

当前认证/TLS/设备身份未决，而组网将改变网络可达范围。**EasyTier数据面加密不保护现有管理API或Probe控制连接。** 自动公网下发network-secret/credential以前，必须确认管理指令身份与配置传输保护方案；不能以“以后做完整RBAC”为由先明文分发。

推荐审核时接受以下范围内最低边界：可信管理访问、受保护的组网配置分发、逐设备准入/撤销验证、本地秘密文件权限、API/任务/日志脱敏。不借本功能顺带建设完整租户/RBAC系统。可先在隔离可信环境预置凭据验证引擎，生产接入另以明确的最小安全设计为门槛。

EasyTier 普通 network-name/secret 模式可用于隔离 PoC，但共享密钥不是逐设备撤权机制；删除数据库成员不等于已阻止其重入。生产优先验证上游 Secure Mode/独立 credential（包括重复使用、过期、吊销传播与中继失联），不预先声称达成强隔离。[S4]

建议自建一个可选 EasyTier 引导/中继进程；可评估放在当前 47.119.168.150，但没有本轮部署授权。独立端口、配置、资源与日志，不复用 8888/9000/9001。它不是 Management Server 的必需微服务；直连数据不经 Go Server，中继路径受其带宽影响。公网发现/中继按测试结果限制开放范围、接入和速率，不把官方示例共享节点作为产品可用性依赖。

## 6. UI 与拓扑图初步方案

新增**全局异地组网工作区**，而不只放到某一台设备详情下；否则跨设备网络缺少独立入口。该导航扩展需随方案审核，不能把现有五工作区约束静默改写。

- 左侧/上方网络选择及创建/编辑/启停；主体保留 `概览 / 成员 / 拓扑 / 二层网络 / 操作记录`。
- 概览：CIDR、引擎/版本、期望/已应用状态、管理在线/组网运行/可达成员计数、中继与异常摘要。
- 成员：设备名、虚拟 IP、所属站点、版本、Probe在线状态、引擎状态、配置代次、直连/中继、下一跳、采样时刻、错误原因。
- 详情：TUN名、路由/发布LAN、NAT与可用地址（可获得时）、连接协议、链路RTT/丢包/收发速率、进程资源（需新增实测采集，不当作上游必有字段）。
- 二层：按L2Segment展示bridge、接入LAN/VLAN、VNI/tunnel-id、端点、MTU、配置/链路证据及冲突警告；尚未实施时不提供假可用按钮。
- Windows操作机默认仅管理，不因打开UI自动安装驱动或加入VPN；如需直接访问虚拟IP，另提供显式“本机加入”。

拓扑需要两种视图，而非一张所有节点互连的示意图：

1. **逻辑视图**：网络成员、路由子网、二层段；说明配置关系，不暗示正在直连。
2. **观测视图**：真实P2P/中继连接和路由下一跳；链路可多条、方向可不同。点击目标可高亮“从所选设备出发”的已观测下一跳路径。

合并各成员快照，不仅查询中继。边带 `reported_by`、`observed_at`、采样代次、单边/双边确认、过期/部分标记；一端信息缺失不能画成确认双向。链路 RTT 与端到端探测RTT分开，丢包显示来源与采样窗口，不把邻接链路延迟当跨多跳真实业务时延。未采集不画成零。[S4]

建议原生 WPF 绘制，先缩放/平移/选择/搜索/图例/详情联动与可访问的列表替代视图，沿用现有主题字体；按需分组折叠，不引入React/WebView2或上游GUI。本轮不选定新图形依赖、不做整体重设计。

## 7. 审核后的推进顺序与验收

以下是建议的小步交付，不是已授权开发排期，也不新建 Phase 7/8。

| 顺序 | 内容 | 必须拿到的证据 |
| --- | --- | --- |
| A 兼容性PoC | 至少两台实际目标路由器，必要时一台自建中继；固定EasyTier版本，隔离网络 | 启动/TUN、虚拟IP双向ICMP/TCP/UDP、P2P与强制中继、掉线恢复、资源与MTU；有第三节点再验证多跳 |
| B 三层产品闭环 | 网络/成员/IP分配、安装与启用分步、公开API、Probe控制、WPF详情 | 重复请求、部分失败、不确定结果、版本不兼容、管理重连、Server重启对账、禁用后的实际状态及安全准入 |
| C 观测拓扑 | 汇聚peer/route与统计，逻辑/运行双视图 | 真直连/中继切换、多跳下一跳、单边/陈旧数据、切Server与WS重连、列表可访问性 |
| D 二层PoC与产品化 | 隔离LAN首先VXLAN，再按设备能力选GRETAP/EoIP | ARP/DHCP与广播、大小包/MTU、三站点防环、GRE直连/中继、掉线/回滚、管理口不失联；不兼容时报告而不是偷换协议 |

范围判断：加入真实组网数据面与持久成员管理不是纯UI小改。A若不通过，先收敛原因再选VNT，不同时铺开三个实现；不在硬件未测时承诺性能、工期或所有协议可用。

## 8. 本次建议审核项

推荐整体方向：

1. 首选EasyTier；固定本次稳定版为PoC基线，实施前再查新发布；失败再评估VNT，ZeroTier保留参考。
2. 首先做设备本身三层互通；站点LAN路由显式启用；不自动并网所有LAN，不自动让Windows操作机入网。
3. 二层优先VXLAN，GRETAP作按需备选，EoIP列为需要独立兼容验证的选项。
4. 同意新增全局组网工作区和真实观测拓扑；不改现有维护入口的数据面与生命周期。
5. 推荐“管理离线不主动拆除已有组网”；正式下发前完成最小管理安全/凭据生命周期设计，不把未决项当已解决。

审核后先执行A，A通过再完成对应ADR和产品接入契约；涉及实际目标设备、安装目录、LAN桥接接口与中继端口时按读到的现场事实确认，不使用猜测默认值替换用户网络。

## 9. 证据与来源

外部信息来自官方发布、固定tag源码和官方文档；调研副本位于Git忽略的 `build/research-sources/`，不是项目依赖或待提交源码。

- [S1] [EasyTier v2.6.4 release](https://github.com/EasyTier/EasyTier/releases/tag/v2.6.4)，GitHub latest API实际返回发布日期2026-05-12；tag提交 `8428a89d2dabc94c97d370ec607c6ca142473626`。
- [S2] [EasyTier README](https://github.com/EasyTier/EasyTier/blob/v2.6.4/README.md)。
- [S3] [EasyTier官方构建矩阵](https://github.com/EasyTier/EasyTier/blob/v2.6.4/.github/workflows/core.yml)、[CLI源码](https://github.com/EasyTier/EasyTier/blob/v2.6.4/easytier/src/easytier-cli.rs)。
- [S4] [实例/链路/路由/凭据proto](https://github.com/EasyTier/EasyTier/blob/v2.6.4/easytier/src/proto/api_instance.proto)、[管理proto](https://github.com/EasyTier/EasyTier/blob/v2.6.4/easytier/src/proto/api_manage.proto)、[ACL proto](https://github.com/EasyTier/EasyTier/blob/v2.6.4/easytier/src/proto/acl.proto)、[TUN路径](https://github.com/EasyTier/EasyTier/blob/v2.6.4/easytier/src/instance/virtual_nic.rs)、[IP转发路径](https://github.com/EasyTier/EasyTier/blob/v2.6.4/easytier/src/peers/peer_manager.rs)。
- [S5] [VNT v2.0.7 release](https://github.com/vnt-dev/vnt/releases/tag/v2.0.7)、[对应README](https://github.com/vnt-dev/vnt/blob/v2.0.7/README.md)；发布日期2026-09-05，tag提交 `a54d72209867a595646ddd39e279a4ead18799c7`。
- [S6] [VNT HTTP API源码](https://github.com/vnt-dev/vnt/blob/v2.0.7/vnt-web/src/service_http.rs)、[IPC采集](https://github.com/vnt-dev/vnt/blob/v2.0.7/vnt-ipc/src/server.rs)。
- [S7] [ZeroTier One 1.16.2 release](https://github.com/zerotier/ZeroTierOne/releases/tag/1.16.2)、[节点本地API手册](https://github.com/zerotier/ZeroTierOne/blob/1.16.2/doc/zerotier-one.8)、[Controller说明](https://github.com/zerotier/ZeroTierOne/blob/1.16.2/nonfree/controller/README.md)；发布日期2026-05-28，tag提交 `fe29cd88886e4f58547ba7f740b2d73eb49ab222`。
- [S8] [ZeroTier协议架构](https://docs.zerotier.com/protocol/)。
- [S9] [EasyTier LICENSE](https://github.com/EasyTier/EasyTier/blob/v2.6.4/LICENSE)、[VNT LICENSE](https://github.com/vnt-dev/vnt/blob/v2.0.7/LICENSE)、[ZeroTier LICENSE](https://github.com/zerotier/ZeroTierOne/blob/1.16.2/LICENSE.txt)、[ZeroTier nonfree LICENSE](https://github.com/zerotier/ZeroTierOne/blob/1.16.2/nonfree/LICENSE.md)。
- [S10] [Linux内核VXLAN文档](https://docs.kernel.org/networking/vxlan.html)。
- [S11] [iproute2官方ip-link手册](https://github.com/iproute2/iproute2/blob/main/man/man8/ip-link.8.in)。GRE放通与EasyTier组合为待实测项，不是该手册对组合的认证。
- [S12] [MikroTik EoIP官方说明](https://help.mikrotik.com/docs/spaces/ROS/pages/24805521/EoIP)。该页标示已冻结，本文仅采用其协议身份说明；RouterOS版本专属部署参数实施前以新官方手册复核。

仓库依据：AGENTS、[ARCHITECTURE](ARCHITECTURE.md)、[API](API.md)、[PROTOCOL](PROTOCOL.md)、[DECISIONS](DECISIONS.md)、[DEVELOPMENT](DEVELOPMENT.md)、[DEPLOYMENT](DEPLOYMENT.md)、实际Go/Probe/WPF入口。未读取/使用生产凭据，没有连接设备或改动生产进程。

本轮验证限于官方发布/源码与文档一致性、工作树/差异/链接检查；没有运行产品构建或路由器PoC，不把上游功能、源码推断或既有Probe编译证据当作真实组网通过。
