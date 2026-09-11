# 异地组网：EasyTier 三层首轮实现（ADR-064）

> 最新：用户另行授权仓库上传、ARM—Server联调和Management重启，实际接线/互通及遗留见[实机记录](OVERLAY_LIVE_VERIFICATION.md)。下文未部署/未互通是首轮历史，不覆盖新证据。

> 本次用户已授权审核并合入本地main，源提交da6c560；原分支ADR-059统一为ADR-064。下文独立工作树/未提交与旧验证数量保留为来源历史，不代替[本次集成审核](OVERLAY_INTEGRATION.md)。合并后的WPF为第七工作区（保留新增日志），不推送、不部署。

## 状态与范围

本记录属于 `codex/overlay-network-research` worktree。用户已审核并授权改造；本轮实现三层管理链路，不是厂商固件或生产组网验收。历史调研见 [OVERLAY_NETWORK_RESEARCH](OVERLAY_NETWORK_RESEARCH.md)。

- 已实现：Server 本地 EasyTier Web API 适配、网络/成员/操作持久化、仓库兼容包投放与 Probe 引导、原生 WPF 异地组网和逻辑/观测拓扑。
- 默认基线：EasyTier **2.6.4**（官方发布源码 tag，服务端 API 及 Web 默认配置均按该版本核对）。不是运行时自动跟随 latest；升级须重新验证接口及目标 ABI。
- 未完成：两台厂商设备的真实引擎安装、配置下发、虚拟 IP 双向 ICMP/TCP/UDP、直连/中继/断线恢复/MTU。当前生产 Server、Probe、网络接口未替换。
- 二层仍按已接受的顺序，在三层实机闭环之后实现。VXLAN 优先、GRETAP 按内核能力，EoIP 不等同 GRETAP；本轮没有二层 API、可用按钮、LAN/bridge 修改或空实现。

## 组件与调用链

```text
WPF → /api/v1 + WS → Management / overlay Service
                           ├─ 回环 HTTP → EasyTier Web 配置服务
                           │                 └─ 配置接入 → easytier-core
                           └─ Gateway → Probe network_agent
                                          ├─ inspect / prepare
                                          ├─ Repository + 原 File 传输 → package
                                          └─ install / start 独立进程

业务数据：设备 EasyTier TUN ↔ P2P / 中继 ↔ 其他设备 TUN
          不经过管理控制 TCP、Maintenance 或 Go Server 转发
```

`easytier-core` 普通中继的 RPC **不是**配置服务 API；同主机需部署官方 `easytier-web` 或提供等价嵌入 Web 配置服务的官方运行模式。配置服务接入端口与中继监听是两种职责：仅连上配置服务，不代表节点已获得 P2P 引导端点或虚拟网连接。

- 上游登录：`POST /api/v1/auth/login`，Cookie 会话；固定 UUID 的配置 `PUT .../networks/config/{instance_id}`，随后 `PUT .../networks/{instance_id}` 启用。运行回查使用 `GET .../networks/info/{instance_id}`。
- 不复制上游核心库；不依赖官方 GUI、WebView2、Node 或新增数据库服务。EasyTier 未配置时，原产品仍可运行，网络定义可保存，加入/启停明确返回未配置错误。
- 上游账号密码、network secret 仅存在 Server 本地受保护配置/目录；公开网络 DTO、任务 DTO、日志错误不返回它们。Probe 只收到配置接入 URL、machine UUID 和安装路径，任务公开 DTO 删除该 URL。
- 现有管理 API 认证/TLS/完整审计没有因本功能自动解决。部署必须限制管理访问和配置 API 的本机访问；设备到配置服务的接入保护、账号隔离与撤销仍需实测，不能把数据面加密等同管理面安全。

## 默认配置与用户操作

对应 v2.6.4 官方 Web 的 `DEFAULT_NETWORK_CONFIG()`：

| 项 | 本产品首次下发 |
| --- | --- |
| 监听 | `tcp://0.0.0.0:11010`、`udp://0.0.0.0:11010`；移除 WG |
| 自定义路由 | `enable_manual_routes=true`、`routes=[]` |
| 地址 | 默认 `dhcp=true`、`virtual_ipv4=""`；仅用户在添加成员时填静态 IP 才关闭该成员 DHCP |
| 其他默认 | `bind_device=true`、`multi_thread=true`，其余非业务可选字段交给上游默认 |
| 业务身份 | 固定 network UUID、成员 instance UUID、每台设备持久 machine UUID、私有随机 network secret |
| 引导/中继 | `peer_urls=[]`，由用户在网络编辑里填写真实端点；不擅自加入公共中继 |
| 网段 | 默认 `10.144.144.0/24` 用于静态地址校验；改变它不会强行重定义官方 DHCP 的地址选择策略 |

没有可达 peer 的默认空引导配置，不能当作已组成虚拟局域网。自定义路由空列表不是自动发布 LAN；开启 manual routes 还会影响上游自动代理网段路由，后续 LAN 路由需独立验证。

编辑网络增加 revision，但不自动重启成员。成员“应用 / 启动”显式下发当前 revision。状态分开显示管理在线、期望启停、引擎运行、已确认配置代次、虚拟 IP 与采样时刻；“引擎运行”不等于端到端业务可达。

首版每设备只加入一个平台网络，避免默认监听/RPC/TUN 多实例冲突；64 个网络、每网络 64 成员、最多 2048 操作。超过上限拒绝，不静默删除原操作。不是多租户/独立凭据吊销实现；移除成员记录不等于撤销其已获知的共享密钥。

## 部署接线（不含真实凭据）

Server 新增 `-easytier-config <本机JSON文件>`。该文件不要放入版本库；Linux 限制为服务账号可读，Windows 使用对应账号 ACL。示意字段：

```json
{
  "api_url": "http://127.0.0.1:11211",
  "username": "专用账号",
  "password": "官方登录接口密码表示（前端使用密码MD5十六进制），按凭据保护",
  "config_server_url": "tcp://配置服务主机:22020/专用账号",
  "tool_id": "仓库中实际工具ID",
  "tool_version": "2.6.4",
  "install_directory": "/tmp/root/router-agent-network"
}
```

`api_url` 必须是 literal loopback IP，不接受 localhost 域名、公网/内网远程 API、URL 用户信息或跳转。`config_server_url` 是路由器可达的地址，不是 Server 的 127.0.0.1。监听/防火墙/账号由部署者设置；本轮未替用户启用生产 EasyTier 服务，也未修改生产 Server 启动配置。

仓库准备：

1. 建立一个 EasyTier 工具及版本 2.6.4，不同架构/ABI 各自作为 artifact。沿用 Repository 的 OS/架构/libc/ABI 兼容规则，不由 UI 猜选。
2. **artifact 内容是已解压的单个 `easytier-core` ELF，不是 ZIP/TAR 压缩包，也不是 CLI 或 GUI。** 初版避免在旧 BusyBox 上依赖解压器或执行安装脚本。标注正确架构和 libc；MIPS 大小端必须核对。
3. 已安装则核验 `--version`；缺失时只有唯一 compatible artifact 才投放到私有 `package`。必须同时获得 File committed、released 和 Task success，才执行 install。
4. Probe 拒绝符号链接包、非 ELF、不匹配/不能执行的引擎和不安全目录；临时文件版本检查通过后原子替换。旧版本存在时不偷偷升级/覆盖。
5. 启动使用 shell-less exec、独立 session/进程、固定 machine UUID、独立 config-dir 与回环 RPC。PID + Linux 进程启动时间用于去重；已有安装但失败的启动保留错误，不假报 VPN 可用。
6. 默认目录在 `/tmp`，**只承诺管理断线不拆网，不承诺设备重启后自动恢复**。持久分区路径、空间、启动托管须结合固件另行验收。未安装 systemd/init 脚本。

## 生命周期与不确定结果

- Probe 的管理连接断开、Server 退出、WPF 断开，都不执行网络 disable、删除 TUN 或杀死已启动的独立引擎。操作本身可以因超时/管理断线变成不确定；这不是 teardown。
- 显式“停止组网”通过本机 Web API 停用该成员实例，保留配置接入进程。没有维护租约自动关闭语义。
- 入队前持久化 operation ID；每个 Probe/File task 保留原 task_id。HTTP 沿用既有 Idempotency-Key/原字节；上游超时/5xx 或“配置写入成功、后续启用失败”不自动重试写操作。
- `queued/running` 在 Server 重启后转 `uncertain`，不自动创建替代任务、重新安装或重新配置。
- 回查只读上游。原关联任务未终态/已在进程重启后丢失时，继续 uncertain；不得把节点恰好正在运行当作旧任务已结束。启用回查还要求运行状态及目标配置相符；不证明历史每一步均成功。
- 对无法回查的旧任务，目前没有“强制忽略/接管”按钮；需人工核对原设备与任务，再设计显式恢复路径。不能为解锁按钮自动重复旧命令。

## API / Probe / UI

公开路由（请求响应遵守 [API](API.md) 统一 envelope、分页及写幂等）：

| 方法 / 路径（前缀 `/api/v1`） | 用途 |
| --- | --- |
| GET `/network-settings` | 是否接线、引擎版本及默认值，不含账号/地址秘密 |
| GET / POST `/networks` | 列表 / 创建 |
| GET / PUT / DELETE `/networks/{network_id}` | 查询 / 带 revision 编辑 / 删除空网络 |
| POST `/networks/{network_id}/members` | `{device_id,virtual_ip?}` 加入，返回 202 Operation |
| POST `/networks/{network_id}/members/{device_id}/operations` | `{action:"start"|"stop"}`，返回 202 |
| DELETE `/networks/{network_id}/members/{device_id}` | 移除显式停止已确认成员 |
| GET `/networks/{network_id}/operations` | 原 task_ids、step、状态和脱敏错误 |
| GET `/networks/{network_id}/topology` | 节点、来源标记链路、观测快照 |
| POST `/network-operations/{operation_id}/reconcile` | 回查原不确定操作，不重放 |

DELETE 和无参数 POST 的请求体为 `{}`，与现有 API 一致。新增错误码：`invalid_network_request` 400、`network_not_found` 404、`network_conflict`/`network_reconcile_required` 409、`network_controller_unavailable` 502、`network_not_configured` 503。WS 新 topic `networks` 是失效通知；首连/重连仍回查 HTTP。

Probe 增加 `network_agent_v1` capability、`network_agent` typed TASK，固定 timeout 30 秒，params 限于 action/directory/machine_id/config_server，动作 inspect/prepare/install/start。没有任意 shell、任意下载 URL、stop 进程或二层命令入口。结果为标准 TASK_RESULT，stdout 是 installed/running/tun/version/machine_id。包投放复用原 File 任务，不把大文件塞入此控制帧。

WPF 全局工作区（合并后第七个）“异地组网”：成员、拓扑、链路、操作记录。旧设备无 capability 不允许加入；设置/其他六工作区保留。窄窗口一级导航保留全部文字、收起图标，宽窗口恢复，避免新增模块压缩原检查器。

拓扑只画真实上游邻接连接。每条边带来源、时间、单边/双边确认、过期标记、链路 RTT/丢包/字节计数；统计缺失为 null 而不是 0。每成员最多 128 links/routes、图最多 512 edges，返回 limited 标记；35 秒未更新变 stale。逻辑视图不画“全互联”假连线。原生缩放、平移、选择与列表替代可用；下一跳在成员详情，路径高亮/速率采样/二层视图尚未实施。

## 验证记录与实机缺口

验证日期以本轮本机日志为准（2026-09-12）。代码在独立 worktree，无 Git 提交/推送。Linux 证据目录由 `build/research-sources/verification-linux-run.txt` 记录，内含 release.log、go-final.log、race.log、vet.log 与 network-integration.log。

- Windows Go：`go test ./cmd/... ./internal/... ./tests/... -count=1` 和 `go vet` 通过（Windows 上 Linux-only Probe 用例不执行）；包含官方 API 路径/Cookie fixtures、不确定写不重放、默认值、地址约束、DTO 脱敏、持久身份、停止/重入与真实拓扑边。
- WPF：Release build 零警告/错误；本机自包含发布已生成 `build/windows-desktop-overlay/win-x64/RouterWorkbench.exe`，未替换现有客户端安装；当前 Server EXE 驱动 Desktop.Tests **567 项通过**。新增四份浅/深、空态/拓扑离屏图已检查；不冒充厂商数据或实际桌面截图。证据 `build/research-sources/desktop-tests.log`、`build/overlay-desktop-verification/`。
- Linux WSL：真实 C++ Probe Release **16 项 CTest 通过**。完整 `go test ./cmd/... ./internal/... ./tests/... -count=1`（设置真实 RMP_PROBE_BIN）通过，integration 249.576s；`go vet` 和 Linux Server build 通过。`go test -race` 覆盖 overlay/management/gateway/task/api 五包全部通过。新 `TestNetworkAgentProbeDisconnectAndReplay` 独立 verbose 复核通过：实际 Probe 接收 typed TASK、重复任务不重复启动、管理连接/Probe 退出后独立引擎替身存活；不是 EasyTier 数据面测试。首次新 API DELETE 用例误传空体，修正为契约要求的 `{}` 后全量重跑通过。
- C++ ASan/TSan：本轮链接探测实际报缺 `libasan_preinit.o/-lasan/-lubsan`、`libtsan_preinit.o/-ltsan`（Linux `sanitizer-check.log`），未进入完整 sanitizer 测试，不能声称通过。本轮 GCC5.2 厂商构建未完成：worktree 不含密码文件，自动准备凭据步骤被执行策略拒绝；未绕过。原 main 的历史 ARM 构建不能替代本轮。
- 两台指定 SSH 已只读连通：20007 为 ARMv7 / Linux 3.14.77；20004 为 MIPS / Linux 4.4.198。均发现 TUN 与 iproute2，未发现现成 EasyTier；这不是内核协议/ABI/隧道互通验收。
- 同主机检查发现 EasyTier core/cli 2.6.4 已安装，但未运行配置服务；11211/22020/11010 无相应监听。生产 Repository 工具列表为空。未创建上游账号，未向生产仓库写入包，未改测试设备管理链路。

下一步需要部署者提供：实际本机 Web API 接线与专用账号（只保存在本机）、设备可达配置接入地址/引导端点、Repository 工具 ID 和正确架构的 ELF 产物。随后按两台设备的真实 Probe/ABI 构建结果完成安装与三层 PoC，再推进二层隔离 bridge / VXLAN；不要跳过三层证据直接接 LAN。
