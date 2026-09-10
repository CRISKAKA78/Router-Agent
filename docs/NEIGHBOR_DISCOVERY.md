# LAN 与本机广播域邻居发现

2026-09-10，Accepted ADR-056。用户已授权实施，并明确取消“上级/上联IP与MAC查询”；FNR100当前LAN1的接线不构成通用产品角色配置。

## 清单语义与使用

设备详情新增“LAN 下接设备”和“本机广播域设备”。LAN清单只收录匹配所配置本机转发端口的记录；广播域清单显示所选三层接口的全部已发现记录，二者可以重叠。LAN1可以和其他插孔一样属于LAN，不依据名称、默认路由、IP前缀或某个端口的MAC数量判断上级方向。

端口表示本机到设备的转发路径，不能证明终端直接插线。无端口证据或端口证据冲突的记录留在“未匹配LAN端口”，不计入LAN清单。MAC已知但IP未知时显示“未知”；缓存、租约和FDB记录不表示当前在线。无法穿透NAT获知后方终端，也不承诺发现隔离或休眠设备。

1. 在模板生成器“模板配置 → 邻居发现”启用采集，配置实际三层接口；LAN清单同时填写本机转发端口名称。可导入[通用示例](../examples/templates/neighbors-local.json)后按设备修改，它不是自动识别或FNR100预设。
2. 发布/更新模板，再通过设备资料显式应用。需配套新版Server和支持 `neighbors_v1` 的Probe；旧Probe明确显示不支持。
3. 选择清单和广播域查看IP、MAC、端口、主机名、状态及来源；支持搜索、刷新记录。刷新回查Server最新快照，不自动扫描网络。
4. “主动发现”填写目标接口直连IPv4子网中的规范CIDR（/24～/32，每次最多256个地址）；可停止发现，取消请求完成与原任务结束分别处理。结果从原任务查询，失败原因在页面显示。

## 数据来源与边界

Probe原生通过rtnetlink读取IPv4邻居、IPv6 NDP和桥FDB；IPv4另有 `/proc/net/arp` 回退。可选按域读取dnsmasq租约，只有已与本接口记录关联或属于其当前IPv4子网的租约才使用。静态IP可以由邻居表或主动ARP发现，不依赖DHCP。IPv6仅被动读取，不遍历 /64。

内核FDB按桥、VLAN及端口关联；VLAN须指定对应三层子接口。配置桥成员时提示使用网桥；启用VLAN过滤的根桥须改为对应三层子接口。无独立物理端口的老式swconfig设备可提供只读厂商FDB命令，不能用聚合接口伪造每个外壳插孔。非网桥接口的记录可标记其本地接口路径。

厂商命令逐行输出 `域ID<TAB>MAC<TAB>端口名称`，每次最多5秒、32KiB。相同MAC的多个不同端口保留冲突，不选最后一条；无匹配或不合格式的行不作为证据。命令失败/超限、邻居表或租约不可读、接口不适用均显示原因。此命令属于现有模板采集命令，不在生成器预览中执行，也不自动套用硬件型号。

主动ARP使用原生以太网套接字，无arp-scan/nmap依赖；只绑定所选接口并校验其当前直连IPv4范围。每63ms最多发一次请求（不超过16请求/秒），发送结束再接收2秒，总期限30秒；一次仅一项扫描执行。只接受对应接口、已请求IP、目标IP/MAC及ARP头匹配的回复。扫描需要设备允许原始套接字；权限不足明确失败。当前使用接口的主IPv4地址和掩码，不自动遍历多个别名子网。

主动响应在进程内保留60秒，标为“近期主动响应”；配置或控制会话变化清空它们并取消正在执行的扫描。其后若仅有FDB，IP回到未知，不能沿用已经失效的旧扫描地址。采样时间仅指快照采样，离线或超过3个采样周期标记过期，不伪造终端最后在线时间。

## 契约与资源限制

- 可选运行模板 `neighbor_probe`：周期10～86400秒（默认30），1～8个域。域ID为小写字母开头、最多32字符；scope仅 `lan|broadcast`；interface为实际Linux接口名、最多15字符。可选绝对路径lease_file最多256字节，fdb_command最多4096字节。
- 域ID唯一；同一接口可配置一份broadcast和多份端口互不重叠的lan。lan的ports必填，最多64个非空、唯一、最多128字节的名称。broadcast不按ports过滤。
- `neighbors_v1`为新增能力；沿用CONFIG_APPLY/ACK、EVENT及TASK/ACK/RESULT，未新增传输或数据面。消息、字段与API样例分别见[协议](PROTOCOL.md#邻居发现adr-0562026-09-10)、[API](API.md#邻居发现adr-0562026-09-10)。
- 一份快照的域内行与未匹配行合计最多256条，行JSON总量最多45000字节，含元数据的EVENT不超过64KiB；达到上限标记limited。两页重叠记录分别计数。源文件读取最多128KiB，单次netlink dump最多1MiB/2048条/500ms；不完整dump报告数据源不可用。
- Server只接收当前Session、已应用revision、匹配配置的域和LAN端口，保存最新快照并深拷贝返回；重配置清空旧快照。无新数据库或邻居历史资产库。
- 扫描与取消通过公开API并沿用原幂等请求；响应不确定保持原字节、幂等键和task_id。取消类型只服务本功能，不新增通用任务取消接口。

## 本轮验证与交付

验证产物位于 `build/neighbors`；桌面自包含包位于 `build/windows-desktop-neighbors/win-x64/RouterWorkbench.exe`。测试协议对端、隔离Linux网络与厂商实机分开记录。

- Windows：`go test ./cmd/... ./internal/... ./tests/... -count=1`、`go vet`、Server build；日志 `windows-go-final.log`、`windows-vet.log`。Windows不运行真实Linux Probe用例。
- Linux：RouterAgentTest（Alpine/GCC14）隔离mount/network/PID/devpts，挂载本网络命名空间sysfs后运行 `sh tests/verify-phase5.sh release`；15项CTest、全部Go测试、真实Probe Phase 1～5、vet/build通过，日志 `linux-release.log`。最终C++解析与来源标注调整另跑neighbor CTest及原生集成race通过。
- `RMP_PROBE_BIN=... go test -race ./cmd/... ./internal/... ./tests/... -count=1` 完整通过（`linux-race.log`）；后续局部配置校验与接口测试再跑相关race通过（`neighbor-final-integration.log`）。
- 原生邻居集成使用Linux bridge、veth及另一网络命名空间的内核ARP回复，覆盖静态IP、真实FDB、两清单重叠、公开HTTP原幂等键重试、停止发现、越界/过大范围拒绝、租约子网过滤和重配置后的端口冲突归类。它不代表FNR100已安装新Probe。
- .NET10生成器176项测试通过（0跳过，`RMP_GENERATOR_WSL=RouterAgentTest`），Edge/Playwright14组浏览器场景通过，无浏览器错误，涵盖两类共用接口、编辑/工程重载/导出/发布/更新回读与宽窄/浅深布局。日志 `generator-full-final.log`、`browser.log`。
- WPF自包含发布成功，最终独立回归557项通过（`desktop-final-checks.log`），67份实际WPF矢量布局渲染通过（`wpf-render-final.log`）；已比较新页面浅/深/窄窗口，域选择器仅显示ID与接口，窄表使用水平滚动。新页覆盖重叠、未知IP、过期、切换设备及选择保持。此前执行曾遇现有剪贴板检查瞬时空读，未降低断言或修改复制逻辑，保留诊断并将独立邻居测试窗口放在原流程末尾后完整通过。实际物理输入/多屏DPI及厂商终端仍待验收。

C++ ASan/UBSan、TSan实际尝试在CMake编译器探测阶段失败：当前WSL缺少libasan_preinit.o/libasan/libubsan及libtsan_preinit.o/libtsan，日志 `linux-asan.log`、`linux-tsan.log`，未记为通过。ARM编译机 `root@10.1.1.128` 的BatchMode SSH仍返回Permission denied；用户交互认证运行`runs/20260910-203128-4121706b`已进入GCC5.2编译，并在旧uClibc未向`std`导出`snprintf`处失败。一键构建现仅在远端副本增加`<stdio.h>`并转换为`::snprintf`；Git Bash语法检查、脚本内嵌转换结果、适配后`rmp_probe_core`编译及未适配原始Probe 15项CTest通过。真实GCC5.2复跑仍需交互密码，尚未获得新版ARM/uClibc成品或厂商实机运行结果。未替换用户生产Server/Probe/UI、运行数据或提交/推送Git。Linux本机验证包为 `build/neighbors/router-probe-linux-x64` 和 `router-server-linux-x64`，不适用于该ARM设备。

### FNR100只读数据源核对

通过现有公开API读取 `FE7140555489` 的接口/ARP/桥及swconfig表，证据为 `fnr100-network-inspection.json`、`fnr100-fdb-inspection.json`。实机Linux3.14.77、br0为192.168.5.222/24，桥成员包含eth0/vlan3/无线口；switch0支持只读 `get dump_arl`，输出MAC、PORTMAP与VID。内核桥FDB主要看到vlan3聚合路径，准确插孔需要适应该固件的vendor输出。核对时未找到两个常用路径的dnsmasq租约文件。

用户说明当前所有端口为LAN、LAN1连接上一级。此信息仅用于解释该次接线，不生成上级IP/MAC页面，不自动把LAN1排除，不修改设备VLAN/桥/端口角色，也未清除任何ARP/FDB表或对生产广播域执行扫描。部署后仍需核对真实终端IP/MAC、外壳映射、隔离VLAN、NDP和取消表现。
