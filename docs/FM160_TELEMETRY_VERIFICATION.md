# FM160只读遥测与蜂窝UI验证

本轮本机时间：2026-09-12（UTC+08，实际shell与文件时间）。工作树`codex/at-module-adaptation`；所有记录对应未提交源码，不伪造版本SHA。规范见ADR-066，首轮计划见[AT_MODULE_ADAPTATION_PLAN](AT_MODULE_ADAPTATION_PLAN.md)。

## 1. 资料与坐标

用户资料：`C:/Users/Administrator/Desktop/Fibocom_FM160&FG160_AT命令用户手册_V1.3 (1).pdf`，264页；已文本提取并查看CESQ表格的实际PDF页面。手册是数据资料，不执行其中配置/拨号/重启示例。

- §2.10/2.12：CIMI/CCID；CPIN查询用于SIM状态。
- §5.1/5.2：CSQ/CESQ，后者手册示例实际为9项，但响应语法仅列6项、SS-RSRP换算表缺失。
- §5.6/5.7/5.8：CEREG/C5GREG/COPS只读查询。
- GTCCINFO前置的GTSCANSTAT没有完整定义，本轮不发送GTCCINFO/GTCELLSCAN，不自行推测前置配置。

缺失映射补充依据：ETSI TS 127 007 V16.9.0 (2021-08)，3GPP TS27.007 §8.69，印刷197～200页；官方PDF已下载并实际提取、渲染199/200页，不仅依据搜索摘要。

`https://www.etsi.org/deliver/etsi_ts/127000_127099/127007/16.09.00_60/ts_127007v160900p.pdf`

CESQ顺序：rxlev,ber,rscp,ecno,rsrq,rsrp,ss_rsrq,ss_rsrp,ss_sinr。6项视为无NR扩展，9项按此顺序。所有信号255为未知，不是零。

| 制式/指标 | 编码 | 固定图轴 | 普通区间中点 |
| --- | --- | --- | --- |
| LTE RSRP | 0～97 | -140～-44 dBm | -140+(n-1)+0.5 |
| LTE RSRQ | 0～34 | -19.5～-3 dB | -19.5+(n-1)×0.5+0.25 |
| NR SS-RSRP | 0～126 | -156～-31 dBm | -156+(n-1)+0.5 |
| NR SS-RSRQ | 0～126 | -43～20 dB | -43+(n-1)×0.5+0.25 |
| NR SS-SINR | 0～127 | -23～40 dB | -23+(n-1)×0.5+0.25 |

编码0表示低于轴下界，值放边界并保留`<`；最高编码表示高于/等于轴上界时保留`≥`，唯NR RSRQ的126仍是[19.5,20)区间。普通值显示`≈`，不把中点当精确测量。图轴是编码表达的有限绘图范围，不是信号物理极限。LTE CESQ不含SINR，不从RSSI/RSRQ反推SINR。原始AT仍在API查询记录中。

## 2. 实机只读验证

目标：用户提供的SSH 47.119.168.150:20004，FTV300内FM160-CN，固件89641.1000.00.01.04.18 / SVN18。凭据仅本地使用，不写入源码或本记录。

- 现有生产Probe PID26713，redial PID1419；二者未停止、替换或重启。
- 在编译机GCC5.4中用本次`cellular.cpp/json.cpp`构建一次性`SampleCellular(...,telemetry=true)` helper，复用产品端口发现、占用复核、flock/TIOCEXCL及预算，而不是另写无锁串口脚本。
- 上传到独立`/tmp/router-at-readonly-*`，执行一轮最长15秒；完成后删除该helper及空目录。未发送设置AT、未应用模板、未启动第二个长期Probe。
- 2026-09-12 05:10:10（UTC+08）保存响应：ttyUSB0在该轮busy而跳过，ttyUSB1身份成功且匹配`fibocom-fm160-v1`；同USB后续ttyUSB2/3为alternate，未重复采集。
- CPIN/CCID/CIMI/COPS/CEREG/C5GREG/CSQ/CESQ共8项都返回ok。CSQ为99,99，因此RSSI为未提供；不能把命令ok当作该指标可用。

| 项目 | 此轮结果 |
| --- | --- |
| 当前路径 | /dev/ttyUSB1 |
| SIM | READY，就绪 |
| ICCID / IMSI | 均成功解析；完整标识仅本地忽略目录保留 |
| PLMN | 46011，MCC460 / MNC11 |
| 接入制式 | COPS AcT=11，NR / 5GC |
| EPS / NR注册 | 均stat=1，本地注册 |
| CESQ原值 | 99,99,255,255,255,255,65,66,81 |
| NR RSRP | 编码66，[-91,-90) dBm，中点≈-90.5 dBm |
| NR SINR | 编码81，[17,17.5) dB，中点≈17.25 dB |
| NR RSRQ | 编码65，[-11,-10.5) dB，中点≈-10.75 dB |

执行前后对比PID与/proc stat的进程启动tick相同，路由表逐行相同。以上是只读查询前后证据，不宣称排除任何瞬态网络抖动，也不是新Probe生产持续运行验收。该固件返回9项CESQ与标准布局一致；其他固件尚未实测。

本地证据均在Git忽略目录`build/at-module-research/`：sample-live.json、sample-normalized.json、sample-before.log、sample-after.log、normalized-summary.log、sample-live.py、sample-once.cpp、sample-compile.log；原始身份/SIM标识不进入提交。设备临时helper已经清理，编译机隔离runs保留。

## 3. 构建与回归

### Probe / Go

- WSL RouterAgentTest，隔离副本`/work-runs/at-module-caf9e598`，不挂载用户工作树；当前Probe Release构建与CTest **18/18通过**（含新增FM160命令/URC/未知型号/配置测试），日志linux-release.log。
- Windows `go test ./internal/device ./internal/gateway ./internal/probetemplate ./internal/api`通过；新增CESQ普通/边界/255/异常/timeout、真实脱敏样本、深拷贝、v1/v2隔离和白名单校验。`go build -o build/at-module-research/router-server.exe ./cmd/server`成功。
- Linux蜂窝实际Probe集成：`RMP_PROBE_BIN=.../build/phase5-probe/router-probe go test ./tests/integration -run TestCellularIdentityRealProbeAutomaticRenumber -count=1 -v`，v1和v2两分支均通过（86.257秒）。包含真实PTY、占用不抢口、自动重枚举、配置关闭清空、日志/心跳并存、Server归一化和公开HTTP；不是厂商硬件Mock冒充。
- `go test -race ./internal/device ./internal/gateway ./internal/probetemplate`、`go vet ./...`、`go build ./cmd/...`在Linux通过，日志linux-cellular.log。
- Linux全量第一次缺go.sum（复制清单遗漏）已纠正；随后全量仅TestNativeNeighborDiscovery失败，证据是/sys/class/net仍显示宿主网卡。已修正隔离命令为同时挂载私有sysfs，重新运行Go全量全部通过，其中tests/integration为335.056秒，结果记录在linux-full-corrected.log。修改的是测试环境命令，不删测试、不降低断言；通用WSL文档同步go.sum/sysfs要求。
- ASan/UBSan尝试被环境阻塞：CMake编译器自检无法链接libasan_preinit.o、-lasan、-lubsan。未安装/升级环境依赖；不宣称sanitizer通过。C++ TSan本轮未运行。

规范隔离入口：

```sh
unshare -mnpf --mount-proc sh -c 'set -eu; mount --make-rprivate /; ip link set lo up; mount -t sysfs sysfs /sys; mount -t devpts devpts /dev/pts -o newinstance,ptmxmode=0666,mode=0620; RMP_PROBE_BIN="$PWD/build/phase5-probe/router-probe" go test ./cmd/... ./internal/... ./tests/... -count=1'
```

### 旧工具链

本工作树运行：

```powershell
./probe-build.ps1 -RemoteRoot /root/router-agent-at-module -PasswordFile 'C:/Users/Administrator/Desktop/路由器探针平台/password.txt'
```

两架构均构建及ELF校验成功；隔离运行目录`/root/router-agent-at-module/runs/20260912-050705-b08a6d77`。ARMv7 GCC5.2/uClibc，965240字节；MIPS小端 GCC5.4/uClibc，1231504字节。产物为隔离目录下router-agent-armv7/router-agent-mipsel，**未覆盖正式/root/router-agent产物**。cross-build.log记录过程，原SDK/正式产物不动。

### C# / WPF

- 使用主工作区现有.NET10 SDK，仅构建本工作树代码。`dotnet test ProbeTemplateGenerator.sln -c Release`：**197/197通过**，包含telemetry复制、工程/运行模板序列化和旧默认省略；RMP_GENERATOR_WSL=RouterAgentTest。
- WPF Release编译成功，`dotnet publish windows/RouterWorkbench.Desktop/RouterWorkbench.Desktop.csproj -c Release -r win-x64 --self-contained true -o build/at-module-research/windows-publish`成功；未替换正在运行的用户客户端。
- 新增可选测试入口（不移除完整套件断言）：`dotnet run --project windows/RouterWorkbench.Desktop.Tests -c Release -- build/at-module-research/router-server.exe build/at-module-research/wpf-qa --cellular-checks`。蜂窝专项验证自动口隐藏、诊断行移除、35字段下20次快刷新保持同一行对象及滚动、搜索下刷新、三图顺序/范围/去重/缺测、180点上限、端口改名/设备替换/禁用清空及旧Probe门禁。最新专项及共用前置检查共235项通过（wpf-cellular.log）。
- 全量WPF尝试在原PropertyInspectorChecks的系统剪贴板断言失败：期望复制能力完整值，实际剪贴板为空；本轮未更改复制实现/断言。蜂窝专项全部执行不代表完整WPF通过。日志wpf-tests.log，仍需在可用交互式剪贴板环境补验。
- 实际WPF控件导出XPS并用PyMuPDF渲染，已目视查看Light/Dark/窄窗口蜂窝页面；曲线和字段均来自明确测试夹具，**不是连续实机采样截图或物理显示器验收**。图像在wpf-qa/cellular-*.review.png。当前用户完整值面板保留，关闭后可扩大表格/图表区域。
- 生成器本轮完成构建/单元链路，未做新的浏览器人工交互验收。

## 4. 接管与限制

1. 新Server、Probe、WPF代码与可选生成器开关已形成；旧生产程序/模板未变，需用户明确部署授权后安排上线和持续采样验收。
2. 只有FM160-CN这个profile已实现并在上述固件实测。FM150/FM650/RM500U/RM520N/RM500Q不套用；仍只有通用身份。
3. 当前三图跟随所示首个模块；多个模块的属性分别分组，但不同时展示多组三图。未自动选口时的下拉只切换已有候选观测，不强占串口。
4. 未含详细小区/频段/CA、LTE SINR专有查询、持久化历史、规则热更新。断点、未知状态与真实覆盖范围必须保留。
5. 没有Git提交/推送/合入，本次没有修改主工作树；收尾时主目录有其他并行任务的组网相关未提交变更，全部保留，不纳入本工作树交付。手册全文、凭据、构建产物、原始SIM/设备标识不纳入Git。
