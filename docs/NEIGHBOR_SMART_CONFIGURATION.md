# 智能邻居配置、直连扫描与近期发现

本轮实施基线：Accepted [ADR-057](DECISIONS.md)，局部取代ADR-056的手填普通配置和仅最新快照边界。工作分支已核对为 `GPT6API-TEST`；本轮无提交、推送、部署，未替换用户运行的Server/Probe。下面的测试协议对端、真实Linux Probe与厂商实机严格区分。

## 用户操作

1. 生成器“模板配置 → 邻居发现”勾选“启用邻居发现 · 发现本机网络设备”。默认智能配置，连接Server后选择在线参考设备。
2. 选择“采集网络（本机IP所在接口）”，例如 `br0 · 192.168.5.0/24`；显示Probe版本/能力、应用修订及检测时间。普通检测只读取接口/地址/桥的事实，不执行Shell，也不主动扫描。可明确复制参考设备当前已应用的完整邻居配置。
3. 默认30秒刷新、“同一网络全部设备”开启；有内核端口证据时同时开启“按LAN端口筛选”。单接口生成local/lan；多接口使用稳定、可逆、唯一ID。非桥以太网、VLAN子接口、无IPv4等按真实结果展示；桥成员不是独立采集网络，不能凭接口名判断上下级。
4. 高级设置默认折叠，可编辑内部域ID、原始接口、scope、FDB端口、租约和自定义只读命令。展开不修改数据；替换现有配置、删除域、切换预设有确认。离线可导入/编辑/导出，显示“尚未设备验证”。已导入的预设也可以离线清除，保留域/端口让用户核对，不静默丢失。
5. 发布/更新只改变Server模板版本，设备应用是独立步骤，必须等配置确认。新生成器对不支持neighbor_probe的Server提前禁用发布并显示当前/所需能力；缺Probe能力时仍能保存模板，应用前明确提示更新Probe。
6. WPF两份清单默认“最近发现”，可切换“当前记录”。选择网络后自动显示规范范围、地址数与预计耗时；点“主动发现”即可。多个地址/前缀提供下拉；无IPv4、检测过期或Probe不支持时禁用并解释。
7. 高级“自定义范围”只允许当前直连网络内/24～/32。`192.168.5.222/24`会显示将规范化为`192.168.5.0/24`；外部范围在字段附近报错，不发送请求。/24有256个地址、254个主机地址，估计18秒；/31与/32保留端点语义。大于/24的直连网络默认只选设备地址所在/24，不自动扫描整个大网段。
8. 主动扫描有进度/停止及“响应N台，其中新增A台、更新B台”。刷新和30秒采集均不会再次启动主动扫描。响应不确定时沿用原始字节、幂等键和任务，禁止自动替代。

## FNR100预设与作用范围

- 核对的拓扑：br0桥成员eth0、vlan3、ath0、ath1；switch0；固定只读来源 `swconfig dev switch0 get dump_arl`。内核vlan3只表示交换机聚合路径，不能分辨机壳端口。
- PORTMAP：0x02→lan1、0x04→lan2、0x08→lan3、0x10→lan4、0x20→wan；0x01为CPU，不计外部口。未知位图/异常输出不猜测，多端口冲突不选最后一条；当前已验证br0关联VID3，其他VLAN行不混入。
- 型号相同只显示候选操作，不代表已验证或授权执行命令。用户显式点击“FNR100 物理端口只读测试”才在参考设备执行测试，需Probe能力、桥事实、switch0与输出格式全部满足。展示原始摘要与解析端口；失败不阻断内核基础发现。
- “使用FNR100预设”需要独立确认：**此设置会在所有应用该模板版本的设备上执行，不仅是当前参考设备**。运行时每台设备仍重新检查，不能只凭参考设备成功推断全体成功。失败回退内核FDB，LAN端口可能暂时未匹配，但广播域基础发现保留。
- 自定义命令与型号预设互斥；自定义命令未验证会明确提示。导入、打开页面及发布不立即执行厂商命令；应用后被动采集按显式模板执行。禁止修改VLAN/ARP/FDB、网络配置或进程的测试行为。当前没有自动登录或远端部署。
- wan名称以及当前LAN1接线不作为上级证据，所有外部口均可进入LAN筛选；继续保持ADR-056的两域重叠和“不推断上级方向”。

## 近期记录与兼容边界

- Device Service每设备最多1024条**域/IP/MAC**记录，按最后发现排序，24小时淘汰；不同域允许同一设备重复。是内存视图，不是永久资产库或在线设备数据库。
- 保存IP、MAC、端口、来源、首次/末次发现、当前标记与最近状态。最新快照消失后默认列表仍保留；仅FDB时允许MAC记录、IP未知。
- 实际ARP响应60秒的新鲜度与24小时保留独立。Probe传递响应年龄（包含扫描等待时间），Server不会被被动刷新重置新鲜度；过期后显示“最近发现于…”，不能继续称为刚刚/近期主动响应。缓存、租约、历史和current标记都不是在线保证。
- Server重启、Session替换、配置修订切换清空；离线可保留至TTL但不显示在线。UI直接说明这些边界。旧Session、旧revision、较早检测派发的迟到结果不能污染当前视图。
- 公开接口见[API](API.md) ADR-057节；新TASK、响应年龄及预设见[Protocol](PROTOCOL.md) ADR-057节。保持现有幂等/取消/配置ACK，不引入外部数据库或新数据面。
- 新Server兼容旧neighbors_v1 Probe的既有手工扫描契约（规范/24～/32检查，Probe最终直连检查）；自动检测和FNR100预设需neighbors_inspect_v1。新WPF在旧Probe上提示升级，而不是伪造可用范围。
- 新生成器的发布、更新、只读检测都使用原请求持久化/重试机制；本轮未解决跨Server重启任务恢复。错误新增field/details但保留稳定code，旧客户端可忽略新增字段。

## 代码与测试导航

- Probe：`probe/src/neighbor_networks.cpp`（只读接口与FNR100解析）、`neighbors.cpp`（采样/扫描/年龄）、TaskManager与client能力；继续C++11，无getifaddrs/arp-scan/nmap依赖。
- Server：`internal/device/neighbor_discovery.go`及`neighbors.go`持有检测与近期视图；Gateway负责TASK/RESULT/Session校验，Management组合能力，API提供公开契约；Task保存扫描派发前统计基线。邻居专用模板应用校验的遗漏也已修复。
- 客户端：`shared/NeighborNetworks.cs`仅共享CIDR/检测DTO；生成器NeighborEditor/NeighborPublishing沿用现有发布通道；WPF NeighborView经公开Client，不访问内部服务。
- 回归：Go设备/API/管理/集成邻居测试，C++neighbor_tests，生成器NeighborTests/PublishingTests，WPF NeighborChecks/TestProbe。浏览器脚本已跟随字段/高级区修改，但本轮未执行，不能当作验证证据。

## 本轮验证

Windows Go1.25.5 amd64、.NET SDK10.0.400；Linux为专用WSL RouterAgentTest（Alpine3.22.1、GCC14.2、Go1.24.13）。构建/日志在被忽略的 `build/neighbor-smart`；Linux副本地址写在其中的 `linux-path.txt`，不是用户生产目录。隔离mount/network/PID/devpts并重新挂载本命名空间sysfs，防止读取宿主桥信息。

| 范围 | 命令/入口 | 结果与证据 |
|---|---|---|
| Windows Go | `go test ./cmd/... ./internal/... ./tests/... -count=1`；同范围go vet；go build ./cmd/server | 通过，go-windows.log / go-vet.log；真实Linux Probe用例不计入Windows结果 |
| Linux原生全量 | 隔离命名空间内 `sh tests/verify-phase5.sh release` | 通过：15项CTest、全部Go/真实Probe Phase1～5、vet/build；linux-complete.log |
| Linux race | `go test -race ./cmd/... ./internal/... -count=1`；`RMP_PROBE_BIN=... go test -race ./tests/integration -run TestNativeNeighborDiscovery -count=1 -v` | 最终源码通过，核心包race与真实邻居集成race；linux-complete.log（此前结果在linux-final-race.log） |
| C++11 | CMake Release + `ctest --test-dir build/phase5-probe --output-on-failure` | 最终源码15/15通过，linux-complete.log；含完整参数身份/异常输入回归 |
| Blazor生成器 | `$env:RMP_GENERATOR_WSL='RouterAgentTest'; build/dotnet10/dotnet.exe test tests/ProbeTemplateGenerator.Tests -c Release`；`./template-generator.ps1 -BuildOnly` | 189通过、0跳过；独立构建0警告/0错误，generator-tests.log / generator-build.log |
| WPF | `build/dotnet10/dotnet.exe run --project windows/RouterWorkbench.Desktop.Tests -c Release -- build/neighbor-smart/router-server.exe build/neighbor-smart/wpf` | 572检查通过，wpf-regression.log；测试使用独立Go Server和测试协议对端，不是厂商实机 |
| WPF布局/成品 | `python windows/RouterWorkbench.Desktop.Tests/render.py build/neighbor-smart/wpf`；`windows/build-desktop.ps1 -BuildOnly -OutputName windows-desktop-smartneighbors` | 69份实际WPF XPS布局转PNG成功；已检查浅/深/窄及真实按钮流程布局，自包含目录生成成功；wpf-render.log / wpf-publish.log |

覆盖点包括：旧Server拦截且零失败发布请求；不确定只读测试原字节/键重试；桥与桥成员、多IPv4/VLAN/无IPv4；CIDR规范化/子范围/越界/上限；FNR100位图/CPU/异常/环境不匹配；两域重叠；60秒与24小时/1024容量；Session/revision及检测顺序；无IP的FDB、缓存/历史不冒充在线；Server新增更新摘要；WPF默认范围、进度、刷新不扫描和自定义字段反馈。生成器HtmlRenderer检查真实组件输出的可访问标签、字段错误关联与离线提示，**不等于浏览器键盘/主题交互验收**。

### 尚未完成的验收与限制

1. 本轮浏览器整套宽窄窗口、浅深/系统主题、键盘实际交互未执行：启动相关验证流程被执行策略阻止，未绕过。上一轮浏览器14组不作为本轮通过结果。保留现有主题与响应式样式，并有编译/组件HTML回归，但仍需浏览器验收。
2. WPF当前环境无法原生桌面栅格捕获；69份是实际WPF离屏/矢量布局，不含原生标题栏，不宣称物理鼠标、真实键盘或多屏DPI验收。
3. 未安装/替换FNR100的Probe、未对用户网络主动扫描。PORTMAP与拓扑为已提供的设备事实和自动解析回归，不是本轮新版Probe的厂商实机验收。真实ARM/uClibc/GCC5.2、老内核与厂家交换机仍需另行认证/部署许可；本轮只有GCC14 C++11本地验证。
4. ASan/UBSan编译实际失败：当前WSL缺少libasan_preinit.o、libasan、libubsan，原始日志sanitizer.log；未记为通过。没有为此升级系统或依赖。

`git diff --check`通过，10份本轮变更文档的212个本地链接目标核对通过（doc-links.log）。

构建产物仅供后续用户决定测试：`build/neighbor-smart/router-server.exe`、`build/windows-desktop-smartneighbors/win-x64/RouterWorkbench.exe`、生成器 `build/template-generator/build-only`；Linux amd64副本为 `build/neighbor-smart/router-probe-linux-x64` 与 `router-server-linux-x64`（不是ARM/uClibc产物）。运行数据与用户进程均保持，未提交、推送或发布部署。
