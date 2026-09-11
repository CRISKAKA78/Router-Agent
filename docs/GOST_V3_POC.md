# GOST v3 有限 PoC 验证报告（A）

> 合入更新：用户已授权本工作树（含 AT 前置智能邻居）并入本地 main；当前进度、统一 ADR 编号及联合验证见 [WORKTREE_INTEGRATION](WORKTREE_INTEGRATION.md)。下文独立工作树、未提交或原目录不写入等语句记录当时事实，不限制本次授权；原始测试证据仍在对应工作树的忽略目录，不能当作本次联合测试结果。

2026-09-11；当前独立worktree/分支`codex/gostv3-device-poc`（基于5ca179a），原工作目录不写入。用户已通过ADR-063确认注册包方式并授权按讨论实施。本轮新增独立鉴权入口、最小GOST补丁及本机集成验证，尚未接入产品API/Probe/WPF、未部署；历史原版设备证据见§4.2/4.3，最新结果见§4.5。

## 结论

**SSH已改为20007并恢复实机验证。补丁版在FNR100上通过TCP、单路完整/空UDP及TCP串口首包鉴权、断线重接；run06为45项检查44通过，仍有8来源并发32000字节UDP的2路超时，不是全项通过。**

本轮定位并修复API数值metadata未生效的配置问题，以及Linux串口空闲读取阻塞Close的问题。后者以原版失败、修复后WSL和ARM各10轮通过及实际反向PTY注册链路形成验证闭环。当前GOST RSS约17.0～23.3MiB，超过16MiB候选预算；物理UART、真实LAN不同网关对端、1小时稳定性和产品API/Probe/WPF接入尚未完成。GOST保持优先，继续解决剩余压力/预算/监管，不提前部署生产。

详细结果以§4.5为准；§4.1～4.4保留当时的失败和阻塞记录，不能把旧SSH不可达或当时未实现状态当作当前状态。

## 1. 固定输入与环境

| 项目 | 实际事实 |
| --- | --- |
| 发布版本 | 官方GOST **v3.3.0**；GitHub release记录发布时间2026-08-30T10:06:08Z |
| 二进制构建来源 | `go version -m`：Go1.26.7；GOST revision `cb76f63754768c7b5d68895a0d51635b0141b80f`；go-gost/x **v0.16.0** |
| 校验 | Linux AMD64、Linux ARMv7、Windows AMD64三个下载包均与上游checksums.txt的SHA256匹配；不是独立签名认证 |
| Linux实际运行 | RouterAgentTest，Alpine3.22.1，WSL Linux6.6.87.2，AMD64；Go标准库测试器，私有mount/network/PID/devpts，无新增系统包 |
| Linux拓扑 | 第二个network namespace内实际TCP/UDP echo，目标198.18.10.2/24，Probe侧veth198.18.10.1/24；不是Mock回包 |
| 网关用例 | 目标无default路由，以及default via 198.18.10.254；后者仅配置其他网关地址，没有启动该地址的真实路由器，回包走直连网段 |
| Windows实际运行 | 当前Windows主机，官方AMD64 EXE，回环TCP/UDP及HTTP API；仅自行创建的进程/端口 |
| ARM静态检查 | ELF32/小端/ARM，GOARM=7、CGO_ENABLED=0，未见PT_INTERP；不能由此推断厂商内核/浮点指令/内存适配通过 |
| 许可证初查 | 固定GOST和x版本的LICENSE标为MIT，下载保存原文；传递依赖清单来自buildinfo。未完成全部传递依赖分发审核，未发布第三方程序 |

最终产品构建仍遵循ADR-058；本轮没有在10.1.1.128重建或覆盖Probe，也没有在47.119.168.150安装/重启Server或开公网监听。

## 2. 已测功能与明确结果

| 检查 | 结果与边界 |
| --- | --- |
| 无默认网关TCP | 57,344字节二进制原样往返通过 |
| 无默认网关UDP | 1,200字节数据报往返通过 |
| 其他默认网关TCP/UDP | 同上均通过；target.log记录实际来源198.18.10.1，不是公网访客地址 |
| 不同接口网段目标 | 无法转发，但GOST创建API仍返回200；平台必须自己完成直连目标校验和就绪语义 |
| TCP 8条同时活动 | 已建立8条实际echo连接，记录fd/thread/RSS；不是8个模拟状态 |
| UDP 8个来源 | 最终run-03八个不同源端口/不同payload均正确回到自身；run-02曾有未细分失败，最终补逐来源输出后通过。不承诺无丢包或完成长时间并发稳定性 |
| 动态新增/删除其他条目 | 原活动TCP连接继续读写 |
| 删除本条目 | Linux约301ms后旧连接EOF且新连接被拒；Windows也通过。这个结果来自Probe侧GOST API，不等同Server侧已能独立按设备撤销所有动态绑定 |
| 端口冲突 | 创建接口返回200，实际绑定是异步；不能直接将API200转换为平台ready |
| TCP/UDP目标不存在 | 实际连接失败而非假成功；不自动获得平台级错误DTO |
| Relay重启 | 原client自动重新连接并恢复映射，说明平台必须限制旧Session/旧凭据，而不能把后端重连当业务恢复策略 |
| 客户端进程被kill | 活动TCP连接终止；未模拟生产RMP Session或公网半断链 |
| TCP串口PTY | 9600/115200均完成1,024字节含NUL/高位字节/CRLF的双向精确比较，TCGETS速率位检查通过；不代表真实线速时序 |
| UDP串口PTY | 单来源下网络→PTY及PTY→来源回包通过；不代表独占/可配置组帧已经实现 |
| 关闭串口映射 | 删除后新写入未继续到PTY；尚未验证真实驱动阻塞read或USB拔插的回收上界 |
| TLS+凭据 | 受信CA/正确名称/正确凭据成功；错误名称、未受信CA、错误凭据均拒绝新通道 |
| Windows冒烟 | 32,768字节TCP、1,024字节UDP、删除终止活动流并撤监听，三项通过 |

### 2.1 UDP透明性没有通过

实际测试是对完整数据报进行长度和内容比较，没有降低断言：

| 配置/输入 | 实测输出 |
| --- | --- |
| 默认配置，1/1,200字节 | 精确往返通过 |
| 默认配置，8,192或65,000字节 | 都只收到4,096字节 |
| 默认配置，空UDP报文 | 回包超时 |
| Server `udp.bufferSize=65535` + client listener `readBufferSize=65535`，API确认更新 | 8,192字节通过；32,000及65,000字节仍只收到8,192字节；空报文仍超时 |

这证明**本次已测试的原版转发链和两项缓冲配置不足以满足完整透明性**，不宣称已穷尽GOST所有传输/参数组合。固定x源码的UDP relay默认缓冲为4096，`internal/net/pipe.go`流复制半向缓冲为32KiB，均提示需要端到端逐跳检查；不能把截断的全部原因只归于某一个配置。

不得为让测试变绿而擅自将产品UDP能力改成“最多1200/8192字节”，也不能悄悄省略空报文。若后续确定产品允许明确包长限制，应由用户确认公开契约，并在入口拒绝超长包而不是静默截断。UDP-over-TCP的公网丢包/队头阻塞影响本轮未测。

### 2.2 历史串口独占试验：UDP现已移出范围，TCP重接收缺口见§4.3

1. 原始 `rtcp/rudp → serial dialer`：第二个客户端/来源都能把`second-writer`或`serial-udp-B`写入同一PTY，默认不独占。
2. 将`climiter: exclusive`、`limits: ["$ 1"]`直接加在反向listener：第二个写入被挡住，但第一客户端随后TCP EOF或UDP回包超时。日志出现`connection limit exceeded`后重新Bind；固定x的rtcp/rudp listener在底层Accept报错时关闭并重建内部listener。这不是满足独占的成功方案。
3. 改为同一进程内 `rtcp → 本机TCP服务(climiter=1) → serial dialer`：首客户端读写、拒绝第二个写入、首客户端继续回包/再写入四项均通过。只有配置变化，不改上游代码；需要多一个本地监听和一段回环连接。
4. 相同结构用于UDP：第二来源的`intruder`仍写入PTY，虽然首客户端尚能收回包，仍然**失败**。不要以“首客户端还能读写”掩盖第二来源混写。

该轮只证明TCP原拥有者可继续工作，没有覆盖其退出后新拥有者重接收。用户现已移除串口UDP，因此UDP所有权/分包不再是当前任务；跨条目设备节点锁、非默认串口参数与TCP关闭交接仍待处理。固定串口地址解析只暴露名称/波特率/校验，不能把内部结构有字段误当作完整公开配置能力。

### 2.3 平台仍需负责的生命周期

- 关闭后端口可以立即再绑定，上游没有本项目要求的24小时隔离。
- 只杀测试父进程后，独立GOST子进程仍存活（run-04-focused）；测试随后按自己记录的PID及exe验证后终止，并由外层PID namespace退出兜底。不能声称GOST原生实现父进程死亡清理。
- 240分钟/0不限时、创建Session绑定、Server独立撤销、关闭确认、跨进程不恢复和端口池仍是平台接入职责，不通过造一个Mock Session或定时kill脚本伪称产品能力完成。
- API配置服务列表不是每一个远程bind的业务资源表。当前共享relay不能直接等同现有独立Maintenance配对模型；需要在正式选型时明确按设备撤销/凭据失效及本地门控方案。
- 未运行源CIDR白名单、RMP控制断开但数据继续可达、慢消费者/满队列、全链路5秒上界、真实主机崩溃等完整故障矩阵。

## 3. 体积与资源

| 项目 | 实测 |
| --- | --- |
| Linux AMD64解压二进制 | 49,840,290字节，47.53MiB |
| Linux ARMv7解压二进制 | 45,940,898字节，43.81MiB |
| Windows AMD64 EXE | 48,729,600字节，46.47MiB |
| AMD64无映射、仅本机API | RSS 38,656KiB（37.75MiB），7 fd；是已启用API的采样，不代表完全裁剪的核心库 |
| AMD64两个映射、无活动TCP | RSS 40,960KiB（40MiB），10 fd、17线程 |
| AMD64 1条/8条活动TCP | RSS 40/40.25MiB，11/18 fd |
| 20轮动态新增/关闭后，等待7秒 | RSS 45,824KiB（44.75MiB），9 fd、18线程 |
| 3秒短时空闲观察 | /proc utime+stime未增加；不把短采样当长期CPU指标 |

原版AMD64程序已经超过计划中16MiB的初始设备端预算；**这是AMD64证据，不是ARM实测结论**。ARM原版包也明显大于现有约0.8MB Probe，是否可放入目标设备仍需检查存储。20轮后的RSS增加可能包含Go运行时缓存，不能据此诊断泄漏；也没有通过1小时“无单调增长”验收。

首次本机PoC时，在上述透明性/独占准入失败及厂商设备未连接的情况下，当轮停止继续扩大长时间压测，没有完成1小时soak、吞吐/时延/公网丢包性能矩阵或可用RAM占比测量。当时建议先验证ARM运行与预算再决定是否值得裁剪；该运行与资源检查现已执行，结果见§4.2。两轮均未改上游源码或降级版本。

## 4. 设备访问与实机验证

以下§4开头与§4.1为先前访问阻塞的历史记录；用户更换入口后已成功登录，最新事实见§4.2。

- 两次读取 `47.119.168.150:8888/api/v1/devices` 都未获得有效HTTP响应，第二次关闭本地代理仍失败；只证明当时查询不可用，不推断Server一定停止。
- 只读SSH盘点首次尝试在本地AskPass编译路径及已有私钥权限上失败；随后按仓库既有方式准备受限临时凭据的操作被执行策略阻止。停止该路径，没有通过其他工具绕过策略。
- 因此没有取得本轮FNR100在线信息、ARM CPU/内核功能/空闲RAM/磁盘、真实串口节点及占用清单。10.1.1.128是编译机而不是FNR100，不能拿编译机信息填充厂商设备验收。
- 没有打开物理console/getty/调制解调器串口，也没有在生产路由器修改路由、SNAT、VLAN、防火墙或运行实例。

### 4.1 用户指定设备后的续测（2026-09-11）

用户指定SSH目标 `192.168.5.222`，并说明适配GOST v3压缩包已在 `/tmp/root`；本轮据此尝试设备只读盘点。此授权独立于上轮管理服务器私钥准备，不重试原被阻止的凭据操作。

- 使用Windows OpenSSH、密码AskPass（密码仅进程环境传递，结束清除），`ConnectTimeout=12`、`StrictHostKeyChecking=accept-new`、禁用公钥认证；结果为 `connect to host 192.168.5.222 port 22: Connection timed out`，未到认证阶段，不是密码错误。
- 独立 `Test-NetConnection 192.168.5.222 -Port 22` 返回 `TcpTestSucceeded: False`；`ping -n 2 -w 1200` 两次超时。ICMP无响应本身不证明设备关机；结合TCP结果只能确认当前连接路径不可达，尚不能定位设备、SSH服务或网络策略中的具体原因。
- `Find-NetRoute -RemoteIPAddress 192.168.5.222` 显示源 `10.1.1.22`，接口 `vEthernet (Br0)`，已存在路由 `192.168.5.0/24 via 10.1.1.254`。未添加或修改路由、防火墙、接口配置。
- 未执行任何远端命令，未读取/解压适配包、识别其版本/ABI或启动GOST；未触碰Probe、物理UART及其他设备进程。不能把原版v3.3.0本机结果直接当成该适配包的结果。
- 本机证据：`build/gost-device-poc-20260911/inventory.txt`（仅连接错误，不含成功盘点）及 `connectivity.txt`。后者系统时钟为 `2026-09-11T21:11:26+08:00`；与任务显示时区不混用。该目录已被现有build忽略规则排除。

恢复本机到该设备TCP22的可达性后，下一步仍是盘点真实CPU/内核/RAM/磁盘、压缩包内容和版本，在独立目录与回环监听上做ARM启动及资源/转发验证。物理串口收发需先确认非console/getty/生产业务串口及测试对端，不用盲写串口替代验收。

### 4.2 独立worktree与FNR100实机续测（2026-09-11，原版历史）

**工作区隔离**：用户将SSH入口改为 `47.119.168.150:20001`，明确后续在worktree工作。本轮从已保存的 `5ca179a` 创建 `codex/gostv3-device-poc`，工作目录 `C:/Users/Administrator/Desktop/router-agent-gostv3-poc`。所有新增源码、文档、凭据辅助程序及运行证据都位于该worktree；原工作目录不切换、不改写，不提交/合并/推送main。原目录可以由其他任务独立切换或创建worktree。SSH凭据仅通过进程环境交给AskPass，用后清除，不写入仓库。

**设备与包已确认**：SSH实际登录的是Four-Faith，`Linux 3.14.77 armv7l`，四核ARMv7具备VFP/NEON，`br0=192.168.5.222/24`、默认路由仍为 `192.168.5.253`。`MemTotal=505476 KiB`（493.63MiB），解包前 `MemAvailable=365300 KiB`。`/tmp` 是252736KiB的tmpfs，因此解压出的程序文件也会占用设备内存，不能只看进程RSS。

用户包 `/tmp/root/gost_3.3.0_linux_armv7.tar.gz` 只有普通LICENSE/README和可执行gost条目；本轮只解出gost与LICENSE到 `/tmp/root/gost-poc-20260911-a2`。后续a3/a4目录硬链接同一GOST文件，不重复分配三份二进制。实际 `gost -V` 为 `v3.3.0 (go1.26.7 linux/arm)`；文件45,940,898字节，SHA256为 `6005052bf9a050dfb7ffdee4177f877855800bfa9057c14a35a9215b0d7ecb46`，与此前校验的官方ARMv7二进制一致。因此“适配包”在本轮核对中不是一个不同的补丁构建。

**测试路径**：Windows合成客户端 → Windows GOST回环reverse监听 → SSH `-R` 承载GOST relay TCP连接 → ARM GOST → 设备回环TCP/UDP echo或PTY。`-L` 只把设备回环API带回本机；所有新监听绑定127.0.0.1。未在47.119.168.150安装程序或开放新的公网端口。此路径使用用户既有SSH入口，不代表拟议生产数据面的部署/性能；本轮没有真实LAN下接目标的无网关/SNAT复验，之前的网关证据仍只来自WSL隔离拓扑。

**设备最终主轮run-03结果**（逐字节比较，不降低断言）：

| 项目 | 实测结果 |
| --- | --- |
| TCP二进制 | 57,344字节往返精确一致 |
| TCP并发 | 8客户端各16,384字节通过；8条活动连接均能继续收发 |
| API关闭 | 删除后原TCP不能继续传输，新建连接被拒绝 |
| 反复开关 | 20轮新增/回包/删除全部通过，结束fd回到9 |
| 小UDP | 1/1,200字节往返通过；大报文之前8个不同来源各1,200字节通过 |
| 大/空UDP | 8,192、32,000、65,000、0字节均超时（不是本轮测得的4096字节截断） |
| UDP后续状态 | 大/空用例后8来源小报文全部超时；显式删除重建后小报文及8来源回包恢复；设备日志出现 `unexpected EOF` / `bad address type`。尚未把根因唯一归于某一缓冲参数或SSH层 |
| PTY9600/115200 | 每档独立PTY，网络→PTY及PTY→网络各1,024字节精确一致，termios速度位分别13/4098 |
| ARM TCP串口独占配置 | 第一个连接触发GOST全进程panic，不是正常拒绝第二客户端 |
| ARM UDP串口独占 | 因上述进程崩溃未执行，不能宣称ARM通过/失败；AMD64原有失败证据保留 |

资源为**ARM GOST本体**，不含测试helper、SSH及tmpfs文件，不等于产品集成后全部增量：

| 时点 | RSS | fd / 线程 |
| --- | --- | --- |
| API空配置、无映射 | 17,380KiB = 16.97MiB | 7 / 10 |
| 8条TCP活动连接（另有UDP来源会话） | 20,080KiB = 19.61MiB | 26 / 11 |
| 20轮开关后 | 22,052KiB = 21.54MiB | 9 / 11 |
| 30秒空闲采样后 | 22,052KiB，不变 | 9 / 11 |

空闲窗口实际30.63秒，进程增加1个user tick、0个system tick；未假定内核HZ，不声称已通过1小时/单核1%CPU验收。RSS超过计划中**Proposed**的16MiB预算，但低于该设备当时可用RAM的25%；不要静默放宽门槛或把VmSize约602MiB误当物理RSS。设备当前可以运行该包，是否接受约22MiB RSS及tmpfs程序体积仍需预算决定。

**已定位的ARM阻断**：`run-03/gost.log` 的原始堆栈为：

```text
panic: unaligned 64-bit atomic operation
internal/runtime/atomic.Xadd64(0x60c0ac4, 0x1)
github.com/go-gost/x/limiter/conn.(*llimiter).Allow
  github.com/go-gost/x@v0.16.0/limiter/conn/limiter.go:29
... limiter/conn/wrapper.(*listener).Accept
... listener/tcp.(*tcpListener).Accept
```

对照固定x v0.16.0源码，`llimiter`把 `limit int` 放在 `current int64` 前，再对current调用 `atomic.AddInt64`。本次ARM崩溃地址尾部为4，和该字段在32位布局下未按8字节对齐相符；已定位到连接限制器而非物理串口驱动。需要限定范围的对齐修复与ARM复测，不能以删掉独占限制器代替满足串口独占需求。未修改上游或替换用户包。

**失败与边界保留**：run-01未压缩helper上传时SSH被对端关闭，留下约4MiB部分文件，未执行；后续改为gzip传输并验证解压成功/字节数后才运行。run-02复用同一个PTY切换波特率，9600双向通过，但115200回传超时；旧serial会话在新会话写回附近才结束，尚不能宣称同一串口快速关闭重开安全。run-03用独立PTY确认波特率/双向基础能力，不将这一测试隔离当作串口生命周期已修好。

run-03共有27条检查（20通过、7失败）及6条资源观察；7失败包含4个UDP边界用例、后续UDP来源用例、串口owner写入和崩溃后的连接错误，并非7个独立根因。未完成UDP串口后续及final资源采样，退出码2是预期保留失败，不是全量成功。物理console为 `/dev/ttyMSM0`，多个现有进程占用；其他ttyS/ttyUSB节点的存在不证明硬件端口可用或空闲，未打开物理串口。

**清理与证据**：a2/a3/a4仅保留测试文件。run-03 cleanup记录本轮目录下无存活进程，原 `/proc/27541/exe` 仍为 `/tmp/root/router-agent`，路由与盘点一致；本轮未修改Probe或网络。Windows GOST/SSH测试子进程均已退出。设备GOST另有600秒timeout，helper自限12分钟，均为测试隔离措施，**不是GOST原生具备平台会话监管**。已因准入失败停止延长压力测试。

本worktree `build/device-poc/` 保留inventory、包与串口盘点、版本/校验、run-01/02/03 JSON、GOST日志、资源样本及cleanup；build目录仍被Git忽略。新增复现入口为 `tests/poc/gostv3/device_smoke.py`、`devicehelper/main_linux.go` 及README。helper实际由Go1.25.5交叉编译ARMv7（静态、无第三方依赖），编译与该包go vet通过；Python语法/帮助入口与差异检查通过。无产品代码变更，不重跑全产品构建。

### 4.3 串口仅TCP + 外部连接鉴权（2026-09-11，当前需求）

**已确认范围**：串口不再支持UDP；内网穿透TCP/UDP全部保留。串口外部TCP建立后必须先鉴权，失败数据不能写入串口。这里是外部使用者认证，不是Probe向GOST中继注册的后端凭据，也不是Management API认证。仍限A评估，未实现产品接口、TLS/RBAC工程或自定义鉴权网关。

在当前worktree用固定Windows GOST v3.3.0和真实TCP观察后端完成本机验证。后端发送合成欢迎字节、记录实际accept次数及全部接收字节，替代UART作为防污染观察点；所有监听为回环，只用随机测试凭据，不访问实机或真实串口。本轮不宣称已验证真正反向串口入口、TLS组合、凭据轮换或公网抗攻击。

**结果（`build/auth-poc/run-02`）**：

| 用例 | 结果 |
| --- | --- |
| 普通`tcp` handler配置`auth`，直接发原始数据 | 未鉴权就收到后端欢迎字节且垃圾数据被echo；这是不安全配置观察，不是通过项 |
| Relay缺凭据/错凭据并携带应用数据 | Unauthorized，后端新增连接为0，未泄漏欢迎字节 |
| Relay垃圾输入、半截握手、空连接 | 不连后端，格式错误即关闭；半截/空连接约1秒关闭（本测试readTimeout=1s） |
| 正确握手拆成两次send，末尾与1024字节负载同包 | 握手未完整时不连后端；认证后欢迎字节和1024字节精确返回 |
| 检查后端收到的内容 | 只有1024字节负载，没有用户名/密码/认证头 |
| 认证请求指定另一个本机目标 | 仍只连接配置的固定目标，诱饵监听未被连接 |
| 外层Relay鉴权、内层TCP bridge限1；同时存在未认证空连接 | 合法拥有者仍成功，未认证者不占串口槽位 |
| 拥有者活动时错误认证/第二个正确认证客户端 | 错认证不连接后端；第二个已认证写入被挡住，原拥有者继续收发 |
| 原拥有者退出后再来合法客户端 | **超时失败**，不能恢复为可供新拥有者使用的完整生命周期 |

共14条检查，13通过、1失败，另有1条普通TCP无鉴权观察。退出码保留失败；run-01已观察相同新拥有者超时，run-02将其精确归入独占重接收检查而非笼统harness_failure，没有改变断言。所有测试GOST子进程均已退出。

**来源与原因定位（固定源码，不用最新文档覆盖版本事实）**：

- 官方鉴权概念页说明只有协议自带身份认证时`auth`才有意义；本机原始TCP反例确认不能在tcp转发配置里加一个密码字段就认为安全。
- `x@v0.16.0/handler/relay/handler.go:215–294`：限时读完Relay请求→检查版本→`Auther.Authenticate`→才调用`handleForward`；`forward.go:52,95`选择已配置的固定后端后才Dial。本轮使用原生Relay v0.7.0二进制握手编码，不是自创产品首包协议。
- `limiter/conn/wrapper/listener.go:38–43`拒绝超额连接时返回普通error；`service/service.go`的Accept循环对非临时错误退出。run-02日志出现 `accept: connection limit exceeded`，此后原拥有者仍能完成，但新拥有者超时。源码与运行现象一致：将独占bridge移到AMD64 Server只能绕过先前ARM原子对齐panic，**不能解决停止Accept的问题**。
- Relay固定forwarder分支在命令分派前执行，网络类型仍来自请求；因此生产串口入口还需明确TCP CONNECT准入，不能把此PoC当作已证明所有BIND/UDP/异常命令都被严格拒绝。
- 参考页：`https://gost.run/en/concepts/auth/`、`https://gost.run/en/tutorials/protocols/relay/`、`https://pkg.go.dev/github.com/go-gost/relay@v0.7.0`；实现结论以固定源码和上述日志为准。

**推荐技术边界**：认证和独占准入尽量在Server串口数据入口完成，未认证不拨后端/不打开UART；业务Application仍管理映射、凭据、租期和设备Session。公网只暴露认证入口，裸rtcp/bridge保持回环或受控网络，不能另给用户一个可绕过认证的裸端口。成功取得独占且真正打开后端后才报就绪；原生Relay认证成功响应本身不能证明UART可用。

优先复用GOST Relay认证，适合外部PC能运行GOST本地适配器或客户端能实现Relay的情况；若外部设备只有简单注册包能力，建议最小新增**Server侧有界首包鉴权/独占入口**，数据面仍用GOST，不自研UART/NAT/整套隧道。两者的接入兼容性不同，最终需知道外部设备/软件及其首包能力，不能宣称裸TCP工具无需适配就能同时强制认证。

建议每条映射独立凭据，失败/超时丢弃并关闭，限制握手长度/总时长/待认证并发，认证头不得透传；半包/粘包、关闭/到期/Session变化、凭据重置与在途会话竞态均纳入验收。无效连接不得打断原拥有者。公网可复用凭据应由TLS或受保护链路承载；裸明文注册包不抵御窃听/重放。具体格式、上限（建议512字节/5秒）、轮换、成功/BUSY响应与客户端适配仍Proposed，不是已实施契约。

**对A后续工作的调整**：

1. 移除当前串口UDP测试和验收需求，保留所有LAN UDP大小/空包/关联回归；不删除旧JSON或伪造旧测试通过。
2. 小范围修复候选变为：连接限制器超限后应拒绝本连接并继续Accept，以及32位ARM计数器对齐；在Server侧统一独占仍需真实设备节点锁和关闭释放验证。不得靠去掉独占限制来掩盖缺陷。
3. 先冻结外部客户端协议能力，再确定复用Relay还是轻量首包适配；本轮只验证现成认证路径，没有先写网关或修改上游。
4. 继续验证LAN UDP、同UART重开、资源预算、真实串口/生命周期。已取消的UDP串口独占/分包不再阻止准入。

本轮新增 `tests/poc/gostv3/auth_smoke.py`；当前Linux/device诊断器移除串口UDP分支、保留LAN UDP。Python语法检查、受影响Linux runner编译/go vet及ARM helper交叉编译/go vet通过；未重跑仍有已知崩溃的设备PoC或全产品构建，旧设备结果标注为修订前证据，不冒充当前测试集合已在实机全跑。

### 4.4 首包注册鉴权实现与最小补丁（2026-09-11）

**范围**：ADR-063已接受Windows网络调试工具裸TCP连接后发送文本注册包，不要求外部GOST/Relay客户端。`internal/serialauth`及`tests/poc/gostv3/registration`是实际实现，不是空接口；当前仅由隔离PoC启动器调用，尚无产品HTTP接口或WPF页面。

#### 首包入口

- `AUTH <64位小写hex token>\r\n`（兼容LF）；通过回复`OK\r\n`后透传。注册行最多512字节、绝对期限5秒，认证行不会进入后端；半包等待、粘包保留认证后的字节。
- 认证前不拨后端、不占串口槽；认证成功取得独占后才连接固定literal loopback地址。无效、超时、超限关闭；第二个有效连接返回BUSY，不破坏原拥有者或监听。
- 每条映射32随机字节凭据，模块保留SHA256摘要，constant-time比较；token不在快照/日志内。轮换关闭活动/待鉴权/拨号中连接，context取消/租期关闭并等待桥接退出。有界待鉴权及每IP限制、连接准入限速。
- 默认租期由PoC启动器设置240分钟，支持0；Gate接收绝对ExpiresAt或上层context。产品Service的Session绑定、签发/展示和远端销毁尚未实现，不能混称已具备。
- `OK`是内部TCP连接成功，不是UART已打开信号；明文bearer注册不能防窃听和重放。客户端只需文本注册行+CRLF，之后可发送HEX二进制，不需安装适配器。

#### 固定版最小补丁

基于GOST v3.3.0和go-gost/x v0.16.0，Go1.26.7。只修改忽略的源码副本；统一补丁、before/after指纹、许可和新增回归保存在`tests/poc/gostv3/patches`，`apply_patches.py`验证源内容后应用，产品go.mod/go.sum未变。

1. `limiter/conn`把64位atomic计数放到对象首字段，避免32位ARM未对齐；增加对齐/限额释放回归。ARM仅交叉编译，待实机验证。
2. `limiter/conn/wrapper`超限关闭当前连接后继续Accept，不把拒绝返回成终止服务的普通error；新增实际TCP拒绝后再次接收测试。原单元测试随新正确语义改为验证底层监听耗尽错误，未删除限额或关闭断言。
3. `internal/util/relay`显式解析UDP-over-stream长度（包括0），避免原`gosocks5.UDPDatagram.ReadFrom`把RSV=0当普通SOCKS5无长度数据继续读取下一帧；短接收缓冲精确排空余量，按整个头+数据帧加写锁，拒绝超过16位长度。新增0/8192/32000/65000/下一帧/截断后下一帧/8来源并发帧回归。
4. `internal/net.Pipe`增加只在remote UDP路径采用的65535字节缓冲选项，TCP默认不变；PoC显式配置relay `udp.bufferSize=65535`和rudp `readBufferSize=65535`。增加整包/空包Pipe回归。未经此配置不能把原版默认缓冲当完整UDP支持。

#### 本轮实际验证

| 检查 | 命令/环境 | 结果 |
|---|---|---|
| 注册入口回归 | `go test ./internal/serialauth ./tests/poc/gostv3/registration -count=10 -timeout=60s`，Windows Go1.25.5 | 14个顶层测试组（含子用例）通过；PoC命令可编译、无命令级单元测试 |
| 静态检查 | `go vet ./internal/serialauth ./tests/poc/gostv3/registration` | 通过 |
| 并发检查 | RouterAgentTest Alpine Linux Go1.24.13/GCC，`CGO_ENABLED=1 go test -race ./internal/serialauth -count=10 -timeout=90s` | 通过；Windows本机CGO关闭，未伪称Windows race通过 |
| GOST受影响回归 | x副本内 `go test ./limiter/conn/... ./internal/util/relay ./handler/forward/remote -count=1 -timeout=60s` | 通过；此前对应包count=3也通过 |
| Pipe受影响回归 | x副本内 `go test ./internal/net -run 'TestPipe|TestPOC' -count=3 -timeout=60s` | 通过 |
| 上游完整internal/net包 | 初次运行含全部原有Transport测试 | `TestTransport_ReadError`失败，未改动的Transport在两个方向竞速中返回先结束结果；保留失败，不修改/删除无关测试，不声称全部上游通过 |
| 补丁重现 | `apply_patches.py --source build/gost-patched-src/patch-reproduction-02` | 所有before/after精确匹配；脚本显式禁用Git CRLF自动转换 |
| 补丁构建 | 独立GOST源码，`go build -trimpath -ldflags='-s -w' ./cmd/gost`，Windows AMD64及CGO=0 Linux/ARM/GOARM7 | 两平台成功；本地文件名gost-patched，非官方原包替换 |
| Windows反向TCP/UDP+注册 | `windows_smoke.py --gost build/gost-patched-src/gost-patched.exe --registration build/device-poc/registration.exe --full-udp --output build/device-poc/windows-patched-01` | **20/20通过**；TCP32768字节、UDP0～65000、8来源32000、之后小包、3轮注册后二进制/独占/重接收、删除关闭 |
| Relay历史对照 | `auth_smoke.py --gost build/gost-patched-src/gost-patched.exe --output build/device-poc/auth-patched-01` | **14/14通过**，普通TCP auth不能防裸数据仍作为观察项；不要求用户采用Relay |
| SSH续测 | 指定账号连接47.119.168.150:20001，两次只读库存命令 | 均exit255 `Connection closed by 47.119.168.150 port 20001`，在认证前失败；未远端上传/运行新补丁或新串口测试 |

测试使用合成凭据、回环真实socket/TCP sink；Windows双向GOST反向数据通道是真实进程，但sink不等于UART。当时真实设备PoC增加可选`--registration --full-udp`，脚本仅语法检查，**§4.4这一轮未实际运行设备注册分支；之后§4.5已运行，不再是当前阻塞**。本机命令退出时清理自有子进程。当前证据目录都在忽略的build下，源码补丁与测试则留仓库待审。

### 4.5 SSH20007设备续测与串口回收修复（2026-09-11，最新）

用户将SSH入口改为`admin@47.119.168.150:20007`，沿用已提供凭据；独立worktree保持，原工作目录、main、生产Probe均未写入/替换，无Git提交或推送。实际登录仍为Four-Faith FNR100 / ARMv7 / Linux3.14.77、br0=192.168.5.222/24、默认网关192.168.5.253；测试前后Probe均为PID32540，路由未变。

#### 两轮结果与修复依据

- **run05：39项检查，34通过、5失败**。原ARM atomic panic不再出现，TCP和PTY基础收发通过；API方式配置UDP缓冲未生效，32000/65000字节回包仅8192。串口旧拥有者退出后第一次注册虽收到OK，实际UART字节/独占/回传失败，进一步证明OK不能当作UART就绪确认。失败结果保留，不覆盖。
- **API配置修正（非新增数据协议）**：x v0.16.0 `metadata/util.GetInt`只处理int/string，不处理JSON解码到map后的float64。PoC通过API下发的`readBufferSize`改成字符串`"65535"`；CLI已是字符串，因此先前Windows CLI通过不能替代API配置验证。无需修改产品依赖或扩大上游通用metadata转换范围。
- **Linux串口最小修复**：原驱动通过`File.Fd()`和`SetNonblock(false)`进入阻塞文件IO，空闲读取时Close被挂住。先新增相同PTY回归：原驱动在Linux实测`Close blocked behind UART read`失败，修复后通过。改用`SyscallConn.Control`执行termios/Flush ioctl，保留非阻塞fd及Go可取消IO；用每次读取deadline保持原VTIME的0.1～25.5秒量化和超时零字节EOF语义。不自研串口驱动或UART协议，只修复现有Linux路径的生命周期；其他平台源码不变。
- 新增`registration_poc_linux_test.go`检查空闲read取消、Flush不破坏可取消性、同PTY3次打开/收发/关闭、超时后再次收发。固定Go1.26.7编译，在WSL Linux与实际ARM设备各重复10轮通过；ARM目标`go vet ./internal/util/serial`通过。物理UART未打开。
- **run06：45项检查，44通过、1失败**。串口首包鉴权全部通过；LAN单路大/空UDP和小包并发通过，新增8来源同时32000字节的压力检查仅6/8成功、2个3秒超时。未改变断言、未加自动重试、未延长等待掩盖失败；丢包位置/原因未定位，不能笼统归因于UDP本身或宣称全准入通过。

| run06检查 | 实际结果 |
|---|---|
| TCP二进制57344字节、8客户端/8条活动流 | 通过 |
| 20轮新建/删除、删除活动连接与监听 | 通过 |
| UDP 0、1、1200、8192、32000、65000字节 | 全部字节一致 |
| UDP 8来源1200字节、后续小包与重新创建 | 通过 |
| UDP 8来源同时32000字节 | **失败：6成功，2超时**，保持压力缺口 |
| PTY 9600 / 115200，1024字节双向、termios速率 | 通过 |
| 原TCP拥有者、第二连接拒绝、原拥有者继续回传 | 通过，ARM未再panic |
| 错误token/认证半包/已占用第二连接 | 拒绝或等待期间**零PTY写入** |
| 正确首包粘业务数据、注册行剥离、PTY回传 | 通过 |
| 断线重新注册、额外5轮100ms间隔重新接入双向收发 | 全部通过，无注入唤醒数据或串口重建来绕过 |
| 进程回收 | 两轮自有GOST/helper均退出，最终无a5/a6测试进程，Probe仍PID32540 |

run06资源：无映射RSS17432KiB、8活动TCP时22004KiB、20轮后23312KiB、末次23824KiB（约17.0～23.3MiB）；最终fd9、线程10。空闲采样30.94秒，用户tick+1/system+0；不是1小时稳定性证明。仍超过16MiB候选预算，未自动放宽预算。设备可用内存还受/tmp上的隔离二进制/压缩包影响，不把tmpfs占用混算成GOST RSS。

#### 产物、复现和清理

- 初版补丁隔离目录`/tmp/root/gost-poc-20260911-a5`；增加Linux串口修复后的目录`/tmp/root/gost-poc-20260911-a6`。本地ARM最终文件`build/gost-patched-src/gost-serial-fixed-arm`，设备对应`a6/gost`，SHA256=`69846f8732fd7f0cd2f2367f9b77df9d3234de22f2e4db5fc1790c340aaf8f1a`。经过分块压缩传输、大小和摘要一致检查才执行，未替换用户原包。
- 补丁/回归已加入`tests/poc/gostv3/patches/x-v0.16.0-registration-udp.patch`与manifest；新增Linux串口两文件。`apply_patches.py --source build/gost-patched-src/patch-reproduction-03`前后指纹全通过，原旧构建/证据仍保留。Windows使用前轮同一Windows补丁版，Linux专属修复不改变其平台代码。
- 最新入口：`device_smoke.py --host admin@47.119.168.150 --port 20007 --remote-dir /tmp/root/gost-poc-20260911-a6 --helper build/device-poc/device-helper --gost build/gost-patched-src/gost-patched.exe --askpass build/device-poc/askpass.exe --registration build/device-poc/registration.exe --full-udp --output build/device-poc/run06-serial-fixed`。目录已运行，复跑必须准备新的隔离目录/输出，不能直接覆盖本次证据。
- 本地证据：`build/device-poc/run05-patched-registration`、`run06-serial-fixed`、`a6-arm-serial-tests.stdout`、`port20007-inventory.stdout`、`port20007-final-cleanup.stdout`；Linux原版失败/修复日志在`build/gost-patched-src/serial-*-test.log`。所有凭据只经当前进程环境传递，不写入仓库。
- 所有自有进程已停止。仅删除本轮a5/a6上传中间`gost.gz`、`helper.gz`及a6的`serial.test.gz`，保留二进制和日志及用户原始GOST压缩包。/tmp剩余从13676KiB恢复到56216KiB；未清理用户文件或历史测试目录。
- 本轮仅续测和最小补丁/测试配置修复：公开API、Probe控制、WPF和旧Maintenance无变化，无需借此重跑全产品UI/Phase1～5。实际硬件UART、LAN不同网关设备、长期稳定性、并发大UDP与预算仍需后续验证；不得将本机/PTY结果冒充整套产品上线。

## 5. 可复现入口、证据与检查

测试源码：[tests/poc/gostv3](../tests/poc/gostv3/README.md)。仅使用标准库，未更改go.mod/go.sum或产品构建入口。

实际主要命令：

```text
go build -o build/gost-poc-20260911/runner tests/poc/gostv3/main_linux.go
GOOS=linux GOARCH=amd64 go vet ./tests/poc/gostv3
python tests/poc/gostv3/windows_smoke.py --gost build/gost-poc-20260911/windows-amd64/gost.exe --output build/gost-poc-20260911/windows-smoke
wsl -d RouterAgentTest -u root -- unshare -mnpf --mount-proc sh -c '<private mount/lo/devpts setup>; GOST_BIN=.../gost .../runner .../run-03'
wsl -d RouterAgentTest -u root -- unshare -mnpf --mount-proc sh -c '<same isolation>; GOST_POC_FOCUS=1 GOST_BIN=.../gost .../runner .../run-04-focused'
```

精确隔离启动模板见测试README，不把上面的省略写法当作可直接复制的命令。下载与校验的固定版本复现脚本是`fetch_release.py`，本轮最初使用同等urllib下载/上游SHA核对流程。

- 本机目录 `build/gost-poc-20260911`：release.json、checksums、三平台包/解压文件、构建信息、ARM ELF检查、来源源码与LICENSE、Windows日志。
- Linux目录 `/work-runs/gost-poc-20260911/run-03` 和 `/work-runs/gost-poc-20260911/run-04-focused`：结果JSON、GOST配置、日志、目标来源IP及短时测试证书。
- 两组Linux证据已复制到本机 `build/gost-poc-20260911/linux-evidence/`，同时保留`linux-evidence.tar.gz`。这些属于忽略的运行证据，不混入源码提交。
- run-03有57条检查/观察记录，包含资源样本和已知不满足项，**不是57项业务测试全通过**。run-04有12条记录；Windows3项功能检查通过。
- 早期调试证据保留在Linux运行目录：初次串口CLI配置错误不计为后端失败；run-02的115200回包问题在消除“主动探测监听造成额外串口reader”后通过。run-03父进程测试监听端口碰撞不计为上游失败，run-04使用不在临时源端口区的测试端口后证实子进程残留。
- 编译、受影响PoC `go vet`、Python脚本语法检查通过；无产品代码变更，不重跑产品全量构建、WPF布局或Phase1～5业务测试。
- 测试进程清理后WSL进程列表未发现GOST；Windows冒烟只终止自己创建的进程。工作区出现的WPF测试文件改动不属于本轮PoC，保持原样。

## 6. 下一步（按新授权推进，实际阻断不跳过）

1. 继续使用用户指定的SSH20007。定位8来源32000字节突发UDP的丢失位置，增加两端socket/队列和收发计数证据；不以放宽超时、自动重试或删除压力检查代替定位。单路65000和空报文已通过，不再误记为仍被8192截断。
2. 在明确的非生产UART及测试对端上验证真实串口、电气参数与热插拔，并验证实际LAN不同网关对端。当前同PTY首包/独占/快速重开已通过，不再重复列为未验证；硬件与跨控制Session回收不能由它替代。
3. 评估实测RSS与16MiB候选预算及1小时稳定性，再按准入结果接入Service/API/Probe辅助进程监管/WPF。客户端注册包选择已确认，不需重复询问；不替换生产进程或自行放宽资源门槛。

固定来源：GitHub `go-gost/gost` release/tag v3.3.0、二进制buildinfo、`github.com/go-gost/x@v0.16.0`模块源码及两者LICENSE。原调研文档中的master/最新文档只是背景，本轮行为结论由上述固定二进制和测试结果给出。
