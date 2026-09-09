# 外壳网口监控

2026-09-09，ADR-043 Accepted。用户确认逐外壳端口方案，以192.168.5.222的FNR100固件为首个实机，并授权Telnet验证、备份/停止/覆盖/重启Probe。没有Git提交/推送授权。

## 本轮完成条件

- WPF单一“网口”页同时展示端口链路、协商速率、逐口收发速率、累计流量、统计时长和同页曲线；逻辑接口与内部口在折叠区，不能将聚合接口流量冒充外壳口。
- 模板保留现有链路backend，增加独立counters配置；swconfig_mib按端口读取命名计数器，command接收稳定端口ID及原始十进制RX/TX字节的三列TSV。采集命令不计算速率。当前只接受累计、非清零的字节口径。
- 首个配置按现场已确认LAN1～4→switch0:1～4、WAN→switch0:5，CPU→switch0:0；不展示无有效路径的Port6。型号与固件预设需要显式应用，不能凭SoC名称自动绑定。
- Probe使用精确整数解析计数、单调时间算差分；每个端口独立采样时间。统计来源或映射变更重建基线，重连保留同来源基线。未知/失败/回退不造零、不退回eth流量。计数回退重建基线；32位可能多次回绕或重置无法区分时中断统计，不猜测。
- Server通过Probe能力port_counters_v1保护新配置，旧Probe明确失败，不能误报已应用流量采集。现有CONFIG_APPLY、EVENT及公开设备快照复用。

## 首个固件事实

本轮Telnet读取确认：设备SN FE7140555489，固件FNR100 v1.1 (Jan 7 2026 11:51:01) std，原Probe位于/tmp/root/router-probe，以--server pcv6.criskaka.com:9000运行。MIB状态ENABLE，Port1和Port5的get mib成功；Port1 RxGoodByte=9307599594、TxByte=8118199201，已超过32位；Port5当前计数0。这些是采集时快照，不是流量压力验收。

RX为网线进入交换端口的好帧字节，TX为端口发往网线的TxByte口径。FCS/前导码/帧间隙是否计入未由厂商确认；不是应用有效载荷或计费流量。LAN间转发会同时计入入口RX与出口TX，不提供误导性的全口“上网总流量”。

## 模板与遥测契约

运行模板增加可选`switch_probe.counters`：

```json
{"backend":"swconfig_mib","rx_field":"RxGoodByte","tx_field":"TxByte","bits":64,"basis":"RX好帧字节 / TX字节；FCS等帧开销未确认"}
```

- backend为`swconfig_mib`或`command`；bits必填32/64；basis为1～128 UTF-8字节。MIB模式必须配置不同的rx_field/tx_field，各1～64字节；按各ports的switch_id/port查询，交换机实例仅允许1～32个ASCII字母数字/下划线/点/短横线，拒绝单独点与双点。没有任意寄存器访问或MIB启用/清零操作。
- command模式使用1～4096字节command，不设置MIB字段名。每行`端口ID<TAB>累计RX字节<TAB>累计TX字节`，要求ID属于ports且无重复，十进制非负整数；不接受包数/估算值/读取后清零计数。非法行使本轮命令计数失败，未提供的端口独立失败；每次5秒/32KiB，最多64行。
- 启用counters时配置1～16个ports，按数组顺序展示；采集仅保留映射清单，其他driver枚举槽位不当作外壳插孔。外部口role=external，内部口role=cpu；未核验用途放高级区域。链路backend和counters.backend分别工作，支持不同命令来源。
- MIB逐口命令各最多5秒，整轮计数读取启动预算10秒；预算用尽时剩余端口显示失败。命令返回前后单调时间的中点为样本时间；计数读取的耗时不假装为固定周期。32位需要已知链路速率并确保采样间隔不可能完整回绕；不满足或计数回退则重建基线，不能排除重置时不猜测回绕量。速率显著超过已知链路上限或累计溢出也重建基线。
- 原始计数采用uint64，先做整数差分再转速率；API字节值允许uint64十进制字符串。原有10项switch指标继续保留，新增`rx_raw_bytes/tx_raw_bytes`（原始累计）、`rx_bytes/tx_bytes`（本次统计累计）、`rx_bytes_per_sec/tx_bytes_per_sec`、`elapsed_seconds`、`counter_source/counter_basis/order`。单位依次为bytes、bytes_per_sec、seconds及text；group仍为switch，完整替换，有界1024指标/64KiB，过大沿用payload_limit，不截成错误完整快照。
- 键仍为`switch_<稳定id>_<字段>`；source仍由Server赋builtin/template，counter_source才描述采集后端，不能混淆。原始值和统计累计不同，不声称原始计数是开机/出厂/本月流量。
- 每个端口拥有独立基线。普通控制重连保留同一进程的基线；更换映射/流量来源/位宽/口径、关闭后重新开启、计数失败或重置后重新建立统计，不补算未知缺口。链路拔插本身不主动清零。停止采集线程的取消不会抹掉重连基线。
- 新Probe注册声明`port_counters_v1`。新Server在派发含counters配置前检查能力，缺失则配置状态failed/error=unsupported_port_counters，并保留原已应用计划；不发新消息类型。先升级Server和Probe，再发布/应用新模板。

## 使用与模块

- WPF设备详情只有一个“网口”页；主表为已确认外部口。选中行在同页下方显示最近10分钟收发曲线（Mbps），统计累计和时长保留；离线/失败/过期断点。原始整数与采集口径位于所选口详情，系统接口原统计与CPU/待核验口位于折叠区域。切换设备/服务器隔离曲线，卸载时停止计时器。
- 生成器“模板配置→物理端口采集”可载入FNR100预设，分别编辑链路和字节来源，粘贴原始输出进行精确整数预览。预览不执行命令；修改统计配置不会修改已有属性。工程7兼容1～6和原运行模板/草稿。通用模板的发布/型号引用/显式应用语义不变。
- [独立运行模板示例](../examples/templates/fnr100-physical-ports.json)与[真实MIB格式样本](../examples/templates/fnr100-port1-mib.txt)可导入/预览；它们只适用于已核对的板卡。已有设备应保留其属性与监控设置再增加switch_probe，不覆盖其他型号的通用模板。
- Probe：`port_counters.cpp`负责原始整数解析、来源关联、失败与基线；SystemSampler在进程内持有它，原switch_probe仍采链路。Go的probetemplate/device/gateway校验并传递配置和指标；WPF的PortRatePanel/ManagedViews消费公开快照，生成器的PhysicalPortProfiles维护预设及样本预览。

## 本轮验证与部署状态

2026-09-09 Windows x64/.NET10.0.400/Go1.25.5/Node24.12.0/Edge；WSL RouterAgentTest、GCC14.2、Go1.24.13。独立Linux副本`/work-runs/physical-ports-20260909`，完整业务用例使用network/pid/mount/devpts隔离。

| 检查 | 结果与证据 |
| --- | --- |
| C++ | `sh tests/verify-phase5.sh release`：13组CTest通过；新增真实MIB字段、uint64极值/超过double精度、非法/包计数、独立来源、零流量、重置/32位不确定、取消保留基线检查 |
| Linux完整Release | 同脚本完整Go包/真实Probe Phase1～5、vet、Server构建通过；integration224.852s；`build/physical-ports/linux-release.log` |
| Linux完整Go race | 配套真实Linux Probe，`go test -race ./cmd/... ./internal/... ./tests/... -count=1`全部通过，integration223.674s；`linux-race.log` |
| 重连专项 | `RMP_PROBE_BIN=... go test -race ./tests/integration -run TestManagedPhysicalPortCounters -count=1`通过，真实Linux Probe断开重连后raw-total基线保持；`reconnect.log` |
| Windows Go | 完整`go test ./cmd/... ./internal/... ./tests/... -count=1`及vet通过；Windows上的真实Linux用例跳过，独立Linux结果如上；`windows-go.log` |
| WPF | `windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-ports`：202项通过；同页速率/累计/原始整数/错误不回退/稳定行/旧Probe明确失败；浅深主题实际WPF布局已查看。`desktop-verify-final.log`及`build/windows-desktop-ports/verification/` |
| 生成器 | `RMP_GENERATOR_WSL=RouterAgentTest`，`.NET10 test ProbeTemplateGenerator.sln -c Release --no-restore`：167通过、0跳过；`generator-tests.log` |
| 真实浏览器/API | 独立5197生成器与测试Go Server，`npm.cmd --prefix tests run test:generator`：11组通过，无浏览器错误；精确样本解析、包计数拒绝、预设/工程/发布往返及1920浅色/900深色已查看；`browser-final.log`与`browser/` |
| C++ sanitizer | 已尝试asan/ubsan/tsan，环境缺libasan/libubsan/libtsan和preinit链接文件，未通过；`asan.log`、`tsan.log` |
| 厂商实机只读 | 本轮Telnet确认MIB ENABLE、Port0/1/5可读、五口链路；LAN1/2 up 1000/full，LAN3/4/WAN down。不是端到端流量压力或拔插验收 |

初轮生成器测试因版本由6升7导致8项旧期望失败，已只更新版本期望并全量复验。初轮WPF布局导航仍查旧页签名称，已更新并完整复验。初轮新增浏览器发布用了已存在的模板名，被既有唯一性校验拒绝；改为明确的新示例名称后11组通过。保留失败记录，不声称全部首轮成功。

当前实机升级待完成：编译机`root@10.1.1.128`免密SSH认证失败，已向用户请求密码或密钥就绪通知。没有用旧ARM产物冒充本轮二进制，未替换运行中的Server/UI/Probe。已保存本机`build/physical-ports/device-before.json`、`template-before.json`，候选`template-candidate.json`保留该设备既有7项属性并增加FNR100网口配置，待配套ARM构建后部署。原设备`/tmp/root/router-probe --server pcv6.criskaka.com:9000`继续运行。

可用新Windows包：`build/windows-desktop-ports/win-x64/RouterWorkbench.exe`；新Server：`build/physical-ports/router-server.exe`；生成器入口`template-generator.cmd`。所有这些都是当前源码产物，但未声称已替换用户正在运行的实例。逐口终端到终端传输、WAN实收发、物理拔插仍需后续实机验证。没有提交/推送。
