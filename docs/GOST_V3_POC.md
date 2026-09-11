# GOST v3 有限 PoC 验证报告（A）

2026-09-11；分支 `codex/lan-serial-tunnel-plan`。用户已批准先执行A、优先GOST v3。本轮未修改产品业务、上游源码或生产部署。

## 结论

**已完成本机隔离PoC并取得可复现结果，但A的完整准入条件未通过，不能进入B正式接入。GOST v3继续作为优先候选，不把“优先”解释成无条件采用。**

- TCP LAN、常用小报文UDP、TCP/UDP与PTY双向字节流及TLS校验已有实际证据。
- TCP串口独占可用**同一GOST进程内增加本地转发服务**的配置级方案实现；不需要为了这一点立刻开发串口字节转发器或添加ser2net。
- 串口UDP独占仍失败；大/空UDP报文的透明性、设备端包大小/内存、会话和父子进程生命周期，仍阻止“原版配置直接接入”。
- ARMv7只完成官方包/ELF/构建信息检查，未运行于FNR100；真实UART、电气层、USB拔插和1小时稳定性未验证。不能把AMD64/PTY结果作为这些项目的通过证据。
- 当前建议继续完成A的设备端适配判断，暂不增加frp/ser2net/rathole依赖，不做GOST fork，不实现B～D。

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

### 2.2 串口独占：TCP有配置级路径，UDP没有通过

1. 原始 `rtcp/rudp → serial dialer`：第二个客户端/来源都能把`second-writer`或`serial-udp-B`写入同一PTY，默认不独占。
2. 将`climiter: exclusive`、`limits: ["$ 1"]`直接加在反向listener：第二个写入被挡住，但第一客户端随后TCP EOF或UDP回包超时。日志出现`connection limit exceeded`后重新Bind；固定x的rtcp/rudp listener在底层Accept报错时关闭并重建内部listener。这不是满足独占的成功方案。
3. 改为同一进程内 `rtcp → 本机TCP服务(climiter=1) → serial dialer`：首客户端读写、拒绝第二个写入、首客户端继续回包/再写入四项均通过。只有配置变化，不改上游代码；需要多一个本地监听和一段回环连接。
4. 相同结构用于UDP：第二来源的`intruder`仍写入PTY，虽然首客户端尚能收回包，仍然**失败**。不要以“首客户端还能读写”掩盖第二来源混写。

因此暂不需要为TCP串口另引入ser2net；但UDP所有权、串口分包、超时交接、跨条目设备节点锁和非默认数据位/停止位/流控仍要处理。固定串口地址解析只暴露名称/波特率/校验，不能把内部结构有字段误当作完整公开配置能力。

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

在上述透明性/独占准入失败及厂商设备未连接的情况下，本轮停止继续扩大长时间压测，没有完成1小时soak、吞吐/时延/公网丢包性能矩阵或可用RAM占比测量。后续如用户维持GOST优先，应先验证ARM运行与预算，再决定是否值得评估裁剪构建；本轮未改上游源码或降级版本。

## 4. 真实设备与环境阻塞

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

## 6. 下一步（仍属于A，不自动进入B）

1. 恢复获准测试的真实Probe设备可达性，并指定一个不承载生产业务的UART/USB串口或测试串口线；再测ARM启动、常驻资源和真实收发。
2. 保持GOST优先，但最终选型前解决/明确：UDP大及空报文、UDP串口独占/组帧、设备内存/体积、Server可独立撤销与父子进程监管。
3. 已通过的TCP串口配置方案优先保留；若需要补丁或裁剪，先提交准确影响和验证计划，不因原版不满足就擅自自研整个数据面，也不悄悄删减UDP需求。
4. A通过后再确认最终后端及Accepted ADR，然后才实施Forwarding Service/API/Probe/WPF。当前ADR-059继续Proposed。

固定来源：GitHub `go-gost/gost` release/tag v3.3.0、二进制buildinfo、`github.com/go-gost/x@v0.16.0`模块源码及两者LICENSE。原调研文档中的master/最新文档只是背景，本轮行为结论由上述固定二进制和测试结果给出。
