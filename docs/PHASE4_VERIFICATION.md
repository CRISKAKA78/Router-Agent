# Phase 4 极简TCP Maintenance验收记录

日期：2026-09-06。起点 `ceaaab791850746911f965167c885789444efd4c`（Phase 3完成）；git fetch后HEAD/main/origin/main一致。起始工作区包含未提交FRP PoC文档和测试，已完整保存至Git stash `preserve previous FRP Phase 4 PoC before minimal TCP Tunnel`，随后确认干净基线。没有复用该PoC源码、补丁或验证结论。

**本轮自研TCP Maintenance实现及规定验收通过。**设计为Accepted ADR-021，实施前说明见PHASE4_DESIGN，正式wire见PROTOCOL，内部用例/配置见API。交付提交标题 `feat: complete Phase 4 TCP maintenance tunnels`，真实SHA与推送结果由Git记录提供；推送main后停止等待验收，不进入Phase 5。

## 验收映射

| 要求 | 证据 |
| --- | --- |
| 一键三入口、默认240分钟、自定义租期 | TestMaintenanceDefaultCustomConcurrentHalfCloseAndIsolation；TestTunnelRealProbeLifecycleConcurrencyAndControlLoad |
| Web TCP80与浏览器多并发模式 | TestTunnelRealHTTPSSHAndTelnet：HTTP Server监听127.0.0.1:80，8个并发独立HTTP客户端连接，经真实Probe返回device-web-ok |
| SSH TCP22真实登录/命令 | 同上：临时独立OpenSSH sshd监听22，临时Ed25519 host/client key，ssh通过临时入口公钥登录root并执行printf，返回real-ssh-command-ok |
| Telnet TCP23双向交互 | 同上：BusyBox telnetd监听23，PTY交互shell，通过入口输入printf命令并收到独立输出real-telnet-command-ok |
| 三服务同时、多个Web连接、多设备 | TestTunnelRealProbeLifecycleConcurrencyAndControlLoad：两真实Probe，A的9条流分布三服务，B已有流保持；实际协议测试同时执行HTTP/SSH/Telnet |
| 客户端断开隔离 | 上述真实测试与Service测试：一条EOF/断开后其他流继续双向工作 |
| half-close与反向延迟响应 | TestTunnelRealBackpressureHalfCloseAndRevoke：外部CloseWrite后，本地读到EOF才返回half-close-after-eof；Service测试额外覆盖反向half-close |
| 主动关闭、并发Close幂等、自动到期 | Service 8个并发Close；真实200ms租约、活动流主动关闭、反复创建/撤销；连接读取得到实际EOF/错误 |
| Session replacement、旧清理隔离 | TestTunnelBindingReplacementAndWriterRevalidation；真实第二Probe使用相同device_id注册替换，旧Maintenance/活动流关闭，新入口绑定新session_id |
| Probe断线/进程退出 | 真实Probe Kill关闭控制Session，旧流和入口释放，重启新Session不继承 |
| 错误/重复token、迟到data | TestPairTokensTimeoutUnavailableAndRevocation：错误token不消费真token，重复拒绝，过期pending及撤销身份不能配对 |
| Server data连接失败、本地目标未监听 | TestTunnelRealProbeMissingLocalAndDataFailure：错误data IP连接失败、未监听23报告unavailable；后续exec继续成功；Service测试验证其他通道不变 |
| 控制writer阻塞时pending总期限 | TestPendingDeadlineRevokesSocketWhileControlWriterBlocked：控制发送尚未返回，40ms期限仍实际关闭外部socket，不等待控制发送超时 |
| 慢读写、背压下关闭 | TestExpiryLimitsHandshakesBackpressureAndCleanup；真实本地服务向慢客户端持续写64MiB，控制exec仍完成，撤销使本地写实际终止 |
| 大流量下heartbeat/exec/upload/download | 两向连续data流运行超过11秒，同时exec、200000-byte上传/下载并逐字节一致；loaded设备LastSeenAt独立确认心跳持续推进 |
| listener释放/端口复用/部分分配回滚 | Service与真实连续5轮维护，只有Released后下一次复用同端口；不足三端口时整个分配回滚 |
| 有界连接/握手/历史 | TestExpiryLimitsHandshakesBackpressureAndCleanup；配置校验；C++tunnel_tests容量满拒绝、在途重复不新增worker |
| goroutine/thread/socket回收 | Service关闭后WaitGroup全部完成、total/ports/handshakes归零；真实Probe多轮关闭及背压撤销后/proc/PID/fd与task线程数回到初始值；C++析构cancel/join |
| Server关闭/重启组合与幂等 | TestManagementTunnelCompositionAndRestart：未配对data socket实际关闭，重开无维护状态，历史未知ID Close幂等成功 |
| SIGPIPE与旧exec隔离 | C++tunnel_tests确认signal disposition保持SIG_DFL；实际背压hard revoke后Probe仍可运行；socket CLOEXEC受原ExecForkMutex保护 |

Server/Probe协议只携带控制指令与状态，所有维护业务字节走独立data TCP。没有HTTP解析、重写或TLS终止；真实SSH/Telnet测试的协议实现来自测试服务端，未被加入Probe。测试SSH/Telnet服务、密钥和工具仅用于验收。

## 环境与结果

Windows amd64 Go1.25.5；Linux x86_64既有WSL隔离镜像、Alpine、GCC15.2.0、CMake4.2.3、Go1.26.3。源码在`/work`，测试使用独立网络/挂载命名空间，lo启用并为Telnet提供独立devpts，不占用宿主TCP22。Linux安装OpenSSH与busybox-extras作为测试工具，不是产品运行依赖。

| 验证 | 实际结果 |
| --- | --- |
| Release C++11构建/CTest | 通过，4/4，4.71秒 |
| Linux全量Go、真实Release Probe，Phase 1～4 | 全部通过，集成167.713秒 |
| 新增真实背压/明确loaded设备心跳断言后的Phase 4补充 | 通过，12.768秒 |
| ASan+UBSan+LSan CTest，detect_leaks=1 | 通过，4/4，5.91秒，无报告 |
| ASan真实Probe Phase 4全部用例 | 通过，12.943秒，无ASan/UBSan报告 |
| TSan CTest，halt_on_error=1 | 通过，4/4，6.84秒，无报告 |
| 全量Go race + TSan真实Probe，Phase 1～4 | 全包通过，集成174.726秒，无race/TSan报告 |
| Windows原生所有源码包测试、vet、Server构建 | 通过；Linux Probe用例在Windows跳过，在上述Linux实际执行 |
| Linux vet、Server构建 | 通过 |
| Close历史幂等与组合Server重启补充 | Windows普通与Linux race通过 |
| git diff --check | 通过 |

真实Probe由测试清理终止；LSan的正常进程退出检查由CTest覆盖。数据worker的实际释放另以/proc fd/thread回落及本地/远端socket终止验证，不把进程Kill本身当成无泄漏证据。

## 复现命令

以下在可绑定固定80/22/23且具备PTY的隔离Linux环境中，于仓库根运行；需要OpenSSH的sshd/ssh/ssh-keygen及busybox-extras。不要让测试接入已有的生产服务。现有工作区build目录有上轮遗留Go模块缓存，`./...`会把缓存误当源码；使用以下显式源码根覆盖仓库所有Go包，不执行build产物。

~~~sh
mkdir -p build/phase4-tcp-go/cache build/phase4-tcp-go/tmp
export GOCACHE="$PWD/build/phase4-tcp-go/cache"
export GOTMPDIR="$PWD/build/phase4-tcp-go/tmp"
cmake -S probe -B build/phase4-tcp-probe -DCMAKE_BUILD_TYPE=Release
cmake --build build/phase4-tcp-probe --parallel 2
ctest --test-dir build/phase4-tcp-probe --output-on-failure
RMP_PROBE_BIN="$PWD/build/phase4-tcp-probe/router-probe" \
  go test ./cmd/... ./internal/... ./tests/... -count=1
cmake -S probe -B build/phase4-tcp-asan -DCMAKE_BUILD_TYPE=Debug \
  -DCMAKE_CXX_FLAGS=-fsanitize=address,undefined -DCMAKE_EXE_LINKER_FLAGS=-fsanitize=address,undefined
cmake --build build/phase4-tcp-asan --parallel 2
ASAN_OPTIONS=detect_leaks=1 ctest --test-dir build/phase4-tcp-asan --output-on-failure
ASAN_OPTIONS=detect_leaks=1 RMP_PROBE_BIN="$PWD/build/phase4-tcp-asan/router-probe" \
  go test ./tests/integration -run TestTunnel -count=1
cmake -S probe -B build/phase4-tcp-tsan -DCMAKE_BUILD_TYPE=Debug \
  -DCMAKE_CXX_FLAGS=-fsanitize=thread -DCMAKE_EXE_LINKER_FLAGS=-fsanitize=thread
cmake --build build/phase4-tcp-tsan --parallel 2
TSAN_OPTIONS=halt_on_error=1 ctest --test-dir build/phase4-tcp-tsan --output-on-failure
TSAN_OPTIONS=halt_on_error=1 RMP_PROBE_BIN="$PWD/build/phase4-tcp-tsan/router-probe" \
  go test -race ./cmd/... ./internal/... ./tests/... -count=1
go vet ./cmd/... ./internal/... ./tests/...
go build -o build/server/router-server-phase4-linux ./cmd/server
~~~

Windows原生：

~~~powershell
go test ./cmd/... ./internal/... ./tests/... -count=1
go vet ./cmd/... ./internal/... ./tests/...
go build -o build/server/router-server-phase4.exe ./cmd/server
~~~

Windows环境未配置cgo C编译器，Go race在Linux完整执行。首次运行另遇共享环境22端口占用和镜像空间不足，已通过独立网络命名空间、workspace Go cache/tmp解决；失败运行未作为通过证据。

## 交付边界

仅内部Go API，默认loopback绑定，由部署者配置公网监听及advertised地址。不配置NAT/防火墙/证书；不提供任意端口、通用转发规则、UDP/SOCKS/VPN/P2P、HTTP反向代理/改写、TLS终止、多级转发或重启恢复。ready表示入口监听，并非持续本地服务探测；Released为Server本地资源释放，Probe取消不依赖跨网络释放ACK。

认证、TLS、权限和审计保持原后续主题；mipsel/ARM/ARM64、uClibc、老Linux实机矩阵尚未执行，不从x86_64构建推断实机验证通过。Phase 5未开始。
