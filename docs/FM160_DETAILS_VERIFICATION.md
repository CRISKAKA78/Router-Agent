# FM160 详细采集与锁定配置验证

日期：2026-09-12（本机日志UTC+08）。工作树：`C:/Users/Administrator/Desktop/router-agent-at-module-adaptation`，分支`codex/at-module-adaptation`。本轮追加范围见ADR-068；基础遥测/趋势首轮证据保留于[原验证](FM160_TELEMETRY_VERIFICATION.md)。没有合入、提交、推送或生产部署。

## 1. 范围与完成条件

用户要求尽可能完整展示FM160信息，并增加锁频段、锁频点、锁小区。完成的链路为模板 → Probe固定查询 → Gateway能力/配置/白名单 → Device Service解析 → 公开API/WS → WPF。不会因为同一型号路由器或相同模板而假定模组相同；当前精确匹配FM160-CN，其他型号仍只查身份。

| 类别 | 详情输出 | 不作出的假设 |
| --- | --- | --- |
| 基本信息/SIM | 厂家、型号、固件、SVN、IMEI、SIM状态、ICCID、IMSI | 没有额外鉴权/脱敏设计承诺 |
| 注册/信号 | 运营商、MCC/MNC、制式、EPS/5GS注册、可返回TAC/Cell ID、附着、RSSI、LTE/NR RSRP/RSRQ、NR及LTE SINR | 未查询到不沿用旧值，255不作为物理信号 |
| 温度/供电 | 模块、基带、RF单次温度及电压 | MTSM?本身不是温度；原周期上报不修改 |
| 服务小区/邻区 | LTE/NR/EN-DC报告、TAC、Cell ID、ARFCN、PCI、驻留Band、带宽、信号、接收电平原始编码 | 当前报告不等于新发起的全网扫描，不保证邻区测量新鲜度 |
| 载波 | PCC/SCC频段、原始PCI/ARFCN、DL/UL带宽、辅载波配置/激活、UL CA、MIMO、调制、原始RSRP | 无SCC报告不能证明永远不支持CA；不猜未知编码 |
| 射频测量 | 开关已开启且返回的CQI、Power、Rank、MCS、SSB Beam ID、QCI | 不自动开启GTCELLINFO；未定义Power单位不写成dBm |
| PDP/连接 | CID、配置APN/PDP类型、激活状态、动态APN、IPv4/IPv6、掩码前缀、网关、DNS、P-CSCF | 模块IP不等于公网出口；未激活地址可能是上次记录 |
| 锁定配置 | 锁频段、锁频点、锁小区；允许频段/制式、优先制式及返回的SCS/NR Band | 驻留频段不是锁频段；不存在锁定/解锁写操作 |

`cellular_probe.details=true`依赖telemetry=true，默认不启用。通过`cellular_details_v1`与独立EVENT隔离旧版，固定24项包含前8项基础查询；精确白名单、大小/时间限制见PROTOCOL。界面保持自动口隐藏、仅查询字段和当前路径、搜索/滚动原位更新、右侧RSRP/SINR/RSRQ三图。name/group/source由Server提供，不在Client复制解析。

## 2. 手册核对与保守处理

来源是用户提供的《Fibocom_FM160&FG160_AT命令用户手册_V1.3 (1).pdf》，264页。下列均为PDF文件页索引（从1开始），不是页脚编号：

- CBC §3.3和MTSM §3.4（约34～36页）：温度=1/6/7为单次模块/BBIC/RF测量。实现先查当前报告模式；报告2/3、未知或读失败时不调用单次命令，保留原报告配置，不发送停止/恢复命令。
- GTACT §5.14（约80～83页）：按当前与支持频段集合比较，LTE101→B1，NR501/5010/5078→n1/n10/n78。仅比较当前实际报告的制式家族，制式选择与频段限制分开。支持集取设备查询值，不硬编码所有FM160硬件支持Band。
- GTCCINFO §5.15（83～92页）：服务行14字段；邻区LTE/NR表定义12字段。兼容92页LTE邻区示例中额外空保留槽，不接受任意移位。EN-DC标题下用rat区分LTE/NR。实际NR rat=9虽缺于枚举，必须有明确NR/EN-DC标题才解析，且记录为固件实测变体。
- GTCCINFO的TAC/Cell ID/ARFCN/PCI按表中十六进制范围及示例解释，同时显示十进制与原始十六进制。NR与LTE分别限制合理位数；不把GTCAINFO未经确认的数值进制套进来。
- LTE RSSNR编码-100～100映射固定图轴[-50,50]dB；量化区间中点标≈，-100显示≤-50。NR SS-RSRP/RSRQ/SINR沿基础CESQ的ETSI TS127007 V16.9.0 §8.69映射，并与NR报告字段顺序配套。同制式同指标优先CESQ，GTCC作为补充，不覆盖有效CESQ；不可识别值保留缺测。
- GTCELLLOCK §5.16（92～94页）：只读`?`。mode0→三锁中的频点/小区均否；type1→ARFCN，type0→ARFCN+PCI。支持查询失败、越界PCI、未知mode/type的回归，不把失败变成否。
- GTCAINFO §5.17（94～97页）：PCC9字段、SCC12字段，Band可推导LTE/NR。实际频点/PCI包含十六进制字符，而表格写整数；保留标记原始值。实际NR带宽500不在手册枚举，不猜成500MHz；RSRP表仅给编码范围，亦保留编码。
- GTCELLINFO §5.20（101～104页）：只读已配置的模式/数据。实机返回0；开启需要配置且可能涉及重启，本轮没有执行。缺失Power等明确显示未提供，不推算。
- CGACT/CGDCONT/CGPADDR/CGCONTRDP按SIM/PDP章节解析。IPv4八段地址+掩码、IPv6三十二段地址+掩码分开处理；IPv6十六段十进制地址转冒号表示，零地址未分配。

手册提到GTCCINFO前置GTSCANSTAT，但未给出该命令定义，**没有发送GTSCANSTAT或任何扫描启动命令**。只读报告的采样接收时间不证明其中邻区的实际测量时刻。

锁频段为“否”表示本次可验证的允许Band配置没有限制已报告制式家族；不是承诺不存在其他未查询的网络约束。查询失败或无法比较时显示未提供/失败原因，而不是“否”。

## 3. 20004实机：当前生产采集核心一次性查询

目标为用户授权的SSH端口20004，设备FTV300、模块FM160-CN、固件89641.1000.00.01.04.18。复用当前`SampleCellular(..., telemetry=true, details=true)`核心编译MIPS一次性helper，不替换现有Probe。凭据仅本地使用，未写入仓库。

本机05:50左右运行，24项查询均`ok`，选中`/dev/ttyUSB1`。Server解析输出127个字段、3个NR信号项；**127是此次展开的展示字段数，包含缺测及多CID/邻区重复维度，不是127个成功硬件测量能力**。

| 本次项目 | 真实返回/解析 |
| --- | --- |
| 锁频段 / 锁频点 / 锁小区 | 否 / 否 / 否 |
| 模块 / 基带 / 射频温度 | 40 / 39 / 39 °C |
| 模块供电 | 3952 mV |
| 驻留频段 / 服务小区带宽 | n78 / 100 MHz |
| 本次报告邻区 | 3条NR记录，多个身份字段模块未提供 |
| 载波 | 1个PCC，未报告SCC；原始带宽编码500未识别 |
| 射频详细测量 | GTCELLINFO=0，发射功率等未提供 |
| 数据连接 | 返回多个CID、APN、激活、IPv4/IPv6和动态DNS等；不在文档暴露真实地址或SIM标识 |

前后PID26713（原Probe）、PID1419（redial）的`/proc/PID/stat`启动字段一致，路由表文本一致。未执行锁定/解锁、重启、拨号或APN写入；helper和自身临时目录已清理。该检查只是进程/路由有限证据，**不声称完整业务数据面连续性验收，也不等于新Probe与生产Server端到端部署通过**。

原始完整标识/查询、归一化、前后记录仅位于Git忽略的`build/fm160-details/full-live.json`、`full-normalized.json`、`full-before.log`、`full-after.log`；`test-full-live.py`和`full-compile.log`记录一次性helper过程。前期preflight样本与最终24项样本分开保存，不混用瞬时温度/电压值。

## 4. 本轮自动化与构建证据

### Go / 原生 / 跨层

- Windows `go test ./internal/device ./internal/gateway ./internal/probetemplate ./internal/api`通过，最终记录go-final.log；`go vet`同四包无输出成功，`go build -o build/fm160-details/router-server.exe ./cmd/server`成功。
- WSL RouterAgentTest独立目录`/work-runs/fm160-details-68a546d7`，私有mount/net/pid namespace、sysfs与devpts；`sh tests/verify-phase5.sh release`完整通过：CTest **18/18**，Go所有包与真实Probe集成（integration约378秒）。没有跳过失败断言。记录linux-full.log。
- `go test -race ./internal/device ./internal/gateway ./internal/probetemplate`通过。最终补齐手册EN-DC/邻区空槽与rxlev字段，并拆分C++警告行后，同目录同步相关源文件，再跑上述race及`cmake --build build/phase5-probe --target cellular_tests -j2` / `ctest --test-dir build/phase5-probe -R '^cellular_tests$' --output-on-failure`通过；linux-final-targeted.log。最后这两项局部修正后未重复全部378秒集成，不把早一轮全量说成最后逐字节源码全量。
- 新回归包括：详细开关依赖和兼容、24项顺序/白名单、拒绝写锁和周期温度命令、周期温度模式跳过、服务/邻区/EN-DC、LTE SINR、CA/Power、IPv4/IPv6、未知/非法值与锁定否/实际值/查询失败。真实C++Probe—PTY—Server集成增加details模式，覆盖端口改名/占用/撤销配置及HTTP详情。
- 脱敏夹具`internal/device/testdata/fm160-details.json`使用人工SIM标识、TEST-NET IPv4和2001:db8::/32 IPv6。夹具中的锁定开启、LTE、SCC和射频详细测量并非本次实机实际状态；测试未为此改变实机。

### 双架构旧工具链

`./probe-build.ps1 -RemoteRoot /root/router-agent-at-module -PasswordFile <主工作区password.txt>`在已授权10.1.1.128编译机运行，隔离run为`/root/router-agent-at-module/runs/20260912-054551-7cb0e075`。GCC5.2 ARMv7/uClibc与GCC5.4 MIPS小端/uClibc均编译、链接、ELF校验成功（965584/1232352字节），记录cross-build.log。未覆盖正式`/root/router-agent`产物，未安装或启动新Probe。最终仅C++同语义换行去警告未重复远端双编译；本机Linux已重编译验证。

### C# / WPF

- 使用现有.NET10 SDK，`RMP_GENERATOR_WSL=RouterAgentTest`，`dotnet test ProbeTemplateGenerator.sln -c Release` **198/198通过**（generator-full.log）。之前只测试一个csproj未设置WSL的167通过/10跳过不是最终结果。
- 使用脱敏Server归一化port样本，设置`RMP_CELLULAR_DETAIL_SAMPLE=<build/fm160-details/ui-fixture.json绝对路径>`，运行`dotnet run --project windows/RouterWorkbench.Desktop.Tests -c Release -- build/fm160-details/router-server.exe build/fm160-details/wpf-qa --cellular-checks`，**243项通过**（wpf-cellular-final.log），包含大于100字段展示、锁定名称/分组/否/实际值、长页原位刷新滚动及原信号曲线/端口行为。初次失败是能力错误提示未带能力名称，已修复产品提示并保留断言后复跑通过。
- `dotnet publish windows/RouterWorkbench.Desktop/RouterWorkbench.Desktop.csproj -c Release -r win-x64 --self-contained true -o build/fm160-details/windows-publish`成功，wpf-publish.log。没有替换用户运行的客户端。
- WPF真实控件导出XPS后PyMuPDF渲染，目视检查浅色未锁、深色已锁及详情首屏；`wpf-qa/cellular-details-*.review.png`。内容为**脱敏/人工状态测试夹具，不是实机持续采样截图或物理屏幕验收**。
- 完整WPF首轮在系统剪贴板断言失败、ASan/UBSan缺运行库，原始原因/命令见基础遥测验证。本轮不重跑无变化环境的这两项，也不宣称已通过。生成器新增开关单元/构建通过，未额外做浏览器人工交互验收。

## 5. 接管与剩余工作

1. 生产Server/Probe/模板未更新；客户真实页面和持续采样需明确部署授权后验收。锁定写功能不在此轮。
2. FM150/FM650/RM500U/RM520N/RM500Q没有套用本规则；不同FM160固件的回包变体仍需真实样本验证。
3. 已保留未启用射频测量、未知CA带宽与原始编码、邻区新鲜度限制；不以展示完整为由编造数值或擅自改配置。
4. main/其他工作树未被本次修改，没有Git提交/推送/合入；相关状态、API/协议、架构、ADR和CHANGELOG已同步。后续合并须检查并发ADR编号，不静默覆盖其他任务决定。
