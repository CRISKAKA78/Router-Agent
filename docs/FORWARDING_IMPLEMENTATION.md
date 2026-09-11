# GOST 产品接入（2026-09-12）

用户明确要求暂停UDP丢包问题定位（网络/程序原因均不阻塞）、暂停RSS门槛，继续完成功能和UI。此授权取代计划中A-only及完整压力准入后才能接入的停止语句，不表示已知UDP问题修复或批准生产进程替换。串口仅TCP+首包注册；LAN TCP/UDP；默认240分钟，0不限时但设备Session结束即关闭。

实施方案：Application Forwarding服务 + 独立GOST进程数据面 + Probe会话持有的可选router-forwarding-agent辅助程序 + WPF两页。新控制0x50/0x51仅传管理JSON，能力forwarding_v1；旧Maintenance/RMT1不变。设备辅助程序检查接口直连目标、源地址与串口白名单，并监管GOST；会话关闭/父进程死亡撤销，不恢复旧通道。Server每条独立Relay/随机内部凭据/独立TLS证书，设备校验证书；内部凭据不进入公开快照。串口外部明文注册是用户选择，不宣称防窃听。端口与注册入口关闭，租期0仍受会话监管。网络代理使用本机源IP访问直连目标，无需目标默认网关指向本机，不修改系统NAT/路由。API/状态机为唯一业务入口，UI不访问GOST API或内部控制。

当前在独立worktree实施；完成与验证结果更新于此，不复用PoC测试冒充产品验收。GOST历史分支ADR059/060对应main统一062/063；本轮设计作为用户明确批准的产品接入补充，不重排其他工作树ADR编号。


## 产品实现

- `internal/forwarding.Service`是Application业务入口；API/WS不直接操作Gateway。Gateway提供当前Session有界命令运输。Probe C++ ForwardingManager用私有管道监管Linux Go侧车，侧车做目标校验并管理GOST，不把Go运行时编入Probe本体。
- LAN仅IPv4直连目标，校验所选接口/网段，排除所匹配接口自身地址及网段/广播地址，严格绑定该接口源IP访问目标。这是应用代理的SNAT等效行为，不保留外部源IP、不改iptables/路由，不是透明网桥。
- 串口枚举实际字符设备并排除控制台；波特率与校验可选，当前GOST后端固定8数据位/1停止位。模块间串口排他锁加每条映射单一认证拥有者；其他程序占用UART不能只靠本模块锁保证发现。
- WPF“内网穿透”“串口透传”两页支持候选IPv4/手输、协议/端口、串口参数、有效期、创建/刷新/关闭、连接地址复制、详情、注册包复制及鉴权统计。候选合并LAN与广播域，缺IP/非IPv4不显示；LAN字节统计如实为未提供。
- creating需设备子进程启动且Server实际BIND成功才变active；**active不表示物理UART已打开**。关闭响应证明Server释放，device_released单独表示设备确认。Session结束/失联/替换/进程死亡撤销，0租期也不恢复。Windows子GOST使用kill-on-close Job，Linux子进程使用父进程死亡信号；辅助失败不主动中断基本Probe管理。
- 限制：全局32条活跃、每设备8条、160条历史、Server队列32、设备队列16、辅助JSON行64KiB；查询8秒、创建20秒超时。每映射占范围内两个端口，关闭持久隔离24小时；崩溃遗留活跃占位重启转换为新24小时隔离。

## 构建与交付

以下相对路径均在当前独立工作树；不操作main，不自动部署。

```powershell
python scripts/build-forwarding.py --gost-source build/gost-patched-src/gost-3.3.0
powershell -NoProfile -File probe-build.ps1
powershell -NoProfile -File windows/build-desktop.ps1 -BuildOnly -OutputName windows-desktop-forwarding -Dotnet <dotnet.exe>
```

build-forwarding.py要求已有固定GOST3.3.0源码及相邻x0.16.0，本仓库PoC补丁manifest逐文件校验通过才构建；不自动下载或升级依赖。GOST用Go1.26.7，设备侧车CGO_ENABLED=0/GOARM=7。Windows/Linux Server及设备后端产物在：

- `build/forwarding-release/server-windows-amd64/`：router-server.exe、gost.exe。
- `build/forwarding-release/server-linux-amd64/`：router-server、gost。
- `build/forwarding-release/device-linux-armv7/`：router-forwarding-agent、gost；本轮已另下载匹配的厂商router-agent及带入许可证。
- `build/windows-desktop-forwarding/win-x64/RouterWorkbench.exe`：自包含WPF，最终用户无需.NET SDK或GOST客户端。

Probe真实构建run：`root@10.1.1.128:/root/router-agent/runs/20260912-023855-59364f2a`，GCC5.2、ARMv7/EABI/uClibc，解释器`/lib/ld-uClibc.so.0`。本轮从该run的output下载产物，没有拿并行任务可能覆盖的root最新文件。其他架构/更老内核不视为已验证。

## 部署说明（本轮未替换生产）

1. 保留原Server数据目录/参数，Server与匹配补丁GOST同目录。新增参数：`-forwarding-gost`（缺省优先同目录gost，再PATH）、`-forwarding-bind 0.0.0.0`、`-forwarding-host 47.119.168.150`、`-forwarding-port-first 22000`、`-forwarding-port-last 22399`。Host须同时让设备连接Relay和外部客户端连接公开端口。
2. 防火墙需允许范围内TCP，LAN UDP公开端口还需UDP。每映射有独立动态TLS Relay TCP端口，不能只开放公开入口。API8888/控制9000/旧数据9001保持。
3. 设备新版router-agent、router-forwarding-agent与ARMv7 gost同目录并可执行；可用绝对路径环境变量RMP_FORWARDING_AGENT覆盖侧车路径，侧车默认找自身同目录gost。侧车由Probe启动，不自行常驻。安装后Probe新Session上报forwarding_v1；缺能力/缺后端在UI明确提示，不影响基本管理。
4. 升级前核对空间、架构、权限及运行文件占用。本轮不向FNR100有限/tmp自动投放大后端，不停止用户Server/Probe。RSS门槛暂停不代表内存不足风险消失。
5. Windows网络工具选择TCP，连接页面地址，发送页面注册包ASCII `AUTH <64位小写hex>`，末尾CRLF；等待OK后发业务字节。无欢迎包；重连重新注册，丢失注册包关闭重建，不通过列表找回秘密。

安全边界：外部串口按用户选择为明文注册，防误连写入而不防窃听/重放；LAN公开入口不新增认证。内部Relay固定证书TLS只保护数据段，配置凭据仍经现有Probe控制链路；本轮未升级全局控制TLS/API认证/RBAC/租户。不能据此宣称完成公网安全加固。

## 本轮独立验证证据

| 验证 | 结果与日志 |
| --- | --- |
| Windows定向Go / vet | forwarding、api、gateway、management、protocol、serialauth通过；build/forwarding-go-final.log，vet无诊断 |
| 实际GOST生命周期 | 实际TCP/UDP/串口Gate→测试TCP后端、秘密不进快照、关闭/持久隔离通过；build/forwarding-gost-tests-03.log（非UART测试） |
| 实际1分钟租期 | build/forwarding-expiry-tests.log：expiry60.10秒通过，原因expired；同轮TCP/UDP/串口与崩溃端口隔离回归通过 |
| Linux既有release | tests/verify-phase5.sh release在隔离mount/net/PID命名空间通过；build/forwarding-linux-tests.log。新可选用例无环境时跳过，另行执行见下行 |
| 完整Probe新链路 | build/forwarding-product-integration-final.log：最终发布版GOST且开启Go race，真实C++ Probe→侧车→GOST TLS→隔离接口TCP/UDP echo，校验源IP/空UDP/关闭；串口真实PTY字符设备，错注册零写入、正确行剥离、双向、关闭回收、0租期断Session撤销 |
| 最新CTest/race | 15项CTest通过（build/forwarding-linux-final-race.log的CTest段），7个Go包race通过（build/forwarding-linux-final-race-02.log） |
| WPF | build/forwarding-ui-tests-delivery.log：574项通过；build/forwarding-ui-qa-delivery含新两页浅/深/窄窗口6张实际WPF布局图。图中为显式测试fixture，不是实机在线证明 |
| 发布/厂商编译 | build/forwarding-release-build.log、forwarding-wpf-publish.log、forwarding-vendor-build.log均成功；无生产替换 |

保留失败：首次race命令GOST路径展开错误，绝对路径重跑通过；实际GOST就绪解析曾漏TCP /tcp后缀，修复并回归；早期WPF窄布局失败，修复后05通过；02曾出现剪贴板检查失败，未修改无关业务或降低断言。

### 完整链路复跑示例（隔离Linux）

在Linux工作副本，构建C++至build/probe、Go侧车至build/backend/router-forwarding-agent，把已固定补丁GOST置build/backend/gost；运行以下命令前，ROOT设为**Linux副本绝对路径**，不要将真实设备或宿主/dev绑定为测试目标：

```sh
unshare -mnpf --mount-proc sh -c '
set -eu
mount --make-rprivate /
ip link set lo up
mount -t devpts devpts /dev/pts -o newinstance,ptmxmode=0666,mode=0620
cd "$ROOT"
RMP_PROBE_BIN="$ROOT/build/probe/router-probe" \
RMP_FORWARDING_AGENT="$ROOT/build/backend/router-forwarding-agent" \
RMP_FORWARDING_GOST="$ROOT/build/backend/gost" RMP_FORWARDING_TEST_NET=1 \
go test ./tests/integration -run TestForwardingProduct -count=1 -v -timeout 90s
'
```

ROOT需export。用例自行创建隔离dummy接口并将分配的PTY绑定到测试UART名，不操作物理UART。Windows实际GOST租期回归设置RMP_FORWARDING_GOST为绝对exe路径、RMP_FORWARDING_LONG_TESTS=1，执行`go test ./internal/forwarding -count=1 -v -timeout 100s`。

## 真实遗留

UDP历史丢包/长延迟原因未修复，RSS预算暂停；用户明确不阻塞本轮功能。尚未部署新产品到生产Server/FNR100，未物理UART双向、真实不同默认网关终端或1小时产品长稳验收。隔离dummy接口/PTY不替代硬件证据。无提交/推送/合main；独立worktree内交付，不改变原工作树和用户进程。
