# Phase 6 Windows UI 验证记录

日期：2026-09-06。起点 main `57c2b1f8f6da1069e4a2eb988224b94bafe9cf84`；fetch后HEAD/main/origin/main一致、工作区干净。用户已验收Phase 5并明确授权Windows UI。设计为Accepted ADR-024 / PHASE6_DESIGN；产品只消费既有Phase 5 HTTP/WebSocket，没有Server/Probe生产代码、Go依赖或Tunnel数据面修改。

## 验收映射

自动化入口为 `windows/RouterWorkbench.Tests/Program.cs`。不依赖测试框架包：断言失败抛异常，进程exit=1；最终84项断言全部通过，产品和测试项目均以warnings-as-errors构建。

| 要求 | 实际证据 |
| --- | --- |
| Server连接、断开、重新连接 | CoreTestsAsync首条resync后获取真实API设备；UiTests操作“连接/切换”“断开”，清空再恢复列表；LifecycleTestsAsync测试不可达提示 |
| 设备列表实时刷新 | 真实Go Server登记测试设备，经原生WebSocket通知读取HTTP快照；断开设备后UI/Core获得offline |
| Session replacement | 同device_id新TCP注册，Core断言Session ID变化、设备保持online、LastOfflineAt=null、原维护Released；UiTests核对详情显示新Session |
| Maintenance创建/主动关闭 | 真实POST创建三入口、GET快照、POST close及失效通知回查，UI直接点击创建/关闭；重复创建API conflict |
| 默认240分钟/自定义正租期 | 创建时UTC期限差精确240分钟；150ms自定义期限与释放；UI分钟/毫秒切换，零租期错误且不创建对象 |
| 三入口打开 | Core验证Web系统默认Shell URL；OpenSSH、Windows Telnet及PuTTY两种模式分别启动真实argv捕获进程，核对独立主机/端口/用户名参数；UI点击三个按钮并经过GET Maintenance后调用启动边界 |
| Maintenance到期/历史/自动选中 | Core等待短租期Server释放后快照反映；UI新建150ms维护，确认第二条出现、两条均Released且自动选中新项 |
| Exec创建/状态/结果 | 真实API创建Task，测试对端ACK/RESULT，经GET读取stdout、stderr、中文、退出码；UI结果文本实际包含中文输出；同HTTP键/原Task重发未产生第二次执行 |
| File基本操作 | 原始HTTP导入/另存逐字节一致、上传设备、下载、Committed+Released、显式complete重复保持asset_id、cleanup、归档；UI资产列表实际显示 |
| Tool/版本/兼容/投放 | 真实API创建工具、不可变版本/Artifact、Server compatible结果、显式产物投放到测试对端、归档版本/工具；files通知更新目录，UI工具列表显示 |
| WebSocket断线恢复 | MockEvents关闭真实WebSocket，客户端自动重连并从HTTP读取断线期间变化，不依赖历史事件；首个HTTP查询屏障期间再通知，验证随后二次查询取得新快照 |
| API/操作错误 | 真实维护conflict和device_offline；session_changed、capacity_exhausted、internal_error、incompatible映射含业务code；不可达、缺客户端、无效租期有不同提示 |
| 重复点击/并发 | LostResponseHandler屏障期间第二次Execute拒绝；UI双击创建维护/Exec只创建一个对象，并验证快速返回后的额外点击防抖；响应丢失后禁止新写入，同键同字节显式重试保留original-task及dispatch_uncertain |
| 切换Server资源与迟到结果 | UiTests从真实Server切到独立MockEvents，只显示新设备；旧WorkspaceConnection不再同步，旧Task/Maintenance视图清空 |
| 应用关闭资源释放 | Core Dispose幂等join、关闭后无新更新；卡住的HTTP body在2秒测试期限内取消；UiTests在独立原生消息循环await表单关闭，断言最终connection释放 |
| 不暴露/持久化内部数据 | 实际Maintenance响应无token/connection_id；产品DTO不定义这些字段；配置只保存Server地址、exe路径、SSH用户名及schema_version；无业务快照持久化 |
| 实际界面/打包 | 原生WinForms构建和消息循环通过；Maintenance与工具页DrawToBitmap截图检查；自包含EXE实际启动并完成消息循环初始化，隐藏启动测试随后终止进程 |

真实API客户端测试启动仓库当前 `cmd/server` Windows可执行文件，Repository使用每轮独立临时目录。`TestProbe.cs`是测试专用Protocol v1对端，返回可核对的Exec结果并完成上传/下载；它不进入产品，不被称为真实Linux Probe。实际C++ Probe及真实SSH/Telnet/HTTP通道由下述全量回归验证。

## 平台与最终结果

Windows amd64：Go1.25.5、.NET SDK10.0.400、.NET/Windows Desktop Runtime10.0.11；现有系统OpenSSH为9.5p2，Windows Telnet Client程序存在。Linux x86_64使用WSL隔离Alpine构建环境，Go1.26.3、GCC15.2.0、CMake4.2.3，独立network/mount namespace和devpts。

| 验证 | 最终结果 |
| --- | --- |
| Windows UI/Core/Tests Release构建 | 通过，0警告/0错误 |
| 客户端真实API/WebSocket及原生控件验证 | 84项断言，exit=0 |
| Windows self-contained win-x64单文件发布 | 通过；RouterWorkbench.exe约111 MiB，附使用说明 |
| 自包含EXE实际启动 | 消息循环初始化成功；隐藏测试进程终止；原生表单正常关闭另由UiTests覆盖 |
| Linux Release C++11构建/CTest | 4/4通过，4.70秒 |
| Linux Phase 1～5全量Go+真实Release Probe | 全包通过，tests/integration 169.961秒 |
| Go race + TSan真实Probe全量 | 全包通过，tests/integration 176.819秒，无race/TSan报告 |
| C++ TSan CTest | 4/4通过，6.83秒，无报告 |
| C++ ASan+UBSan+LSan CTest | 4/4通过，5.92秒，detect_leaks=1，无报告 |
| ASan真实Probe Phase 4/5 | 通过，tests/integration 15.549秒 |
| Windows全部Go源码包测试 | 全包通过；API0.307秒，集成0.160秒（Linux专用例在Windows跳过，在Linux实际执行） |
| Windows Go vet / Server build | 通过；客户端测试实际启动当前Windows Server并调用HTTP/WebSocket |
| Linux Go vet / Server build / 进程检查 | 通过；实际GET设备返回空列表，SIGTERM exit=0 |
| go mod verify / git diff --check | 通过 |

最后一次客户端改动为合并任务/版本/兼容详情刷新，防止频繁通知叠加在途详情查询；再次执行完整Windows构建、84项断言和单文件发布通过。Go/Probe生产源码未变，无需将既有通过的Linux回归重复计算为新结果。

开发中的首次C#编译报错已修正；受控HttpListener服务的关闭Abort错误已修正；按钮防抖后，UI测试对明确的新操作等待系统双击窗口再点击。上述失败不计入通过结果。Linux进程检查首次连接发生在listener就绪前，启动轮询随后成功并正常SIGTERM退出。

## 复现

仓库根目录，Windows已安装.NET10 SDK与Go：

```powershell
./windows/build.ps1 -Verify
# 或指定独立SDK（本轮使用）：
./windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify
go test ./cmd/... ./internal/... ./tests/... -count=1
go vet ./cmd/... ./internal/... ./tests/...
go mod verify
git diff --check
```

脚本发布 `build/windows-ui/win-x64/RouterWorkbench.exe`；客户端验证日志、独立测试Repository、argv记录和截图位于 `build/phase6-tests`。仅临时文件和构建产物被Git忽略，测试源码/命令全部提交。测试HTTP监听使用loopback随机端口，维护池使用32000～32029，须为空闲端口。

Linux完整继承Phase 5回归脚本，从本轮源码运行；Go源码包路径显式限定，避免被本地忽略build目录中的旧Go缓存干扰：

```sh
/bin/sh tests/verify-phase5.sh release
/bin/sh tests/verify-phase5.sh asan
/bin/sh tests/verify-phase5.sh race
```

真实SSH/Telnet/HTTP测试在独立网络及devpts环境运行，需OpenSSH、busybox-extras等测试程序。不要在有真实服务占用80/22/23的环境执行。无外网隔离时先准备go.mod锁定的模块缓存。脚本仍使用phase5命名的build目录和Linux Server输出名，表示重用完整回归命令，不表示复用旧测试结果。本轮实际重新运行所有用例。

Linux进程检查用脚本构建的Server、独立空Repository、控制/data loopback随机端口及测试HTTP端口启动，GET `/api/v1/devices`成功后发送SIGTERM并wait得到0。Windows客户端测试通过相同方式启动当前Windows Server；Windows环境无适用cgo编译器，Go race在Linux实际执行。

## 限制与交付

- 当前验证/发布Windows x64；未执行Windows ARM64/x86和多种DPI/真实嵌入式设备矩阵。Probe实机mipsel/ARM/ARM64、uClibc/老内核仍不从Linux x86_64结果推断已通过。
- 入口启动测试验证系统浏览器分派边界和实际进程参数，没有代替用户浏览器/SSH/PuTTY里的登录、密码、主机密钥或厂商Web页面测试；真实通道字节流由Phase 4/5全量回归覆盖。
- API业务状态完全由Server决定。倒计时使用客户端时钟，只作估计；WS通知不重放，无实时stdout流；Server重启后的幂等不保证，只有Repository持久化。外部客户端由用户拥有，退出工作台不撤销Server对象。
- 认证/TLS/RBAC、Web/微信UI、MCP/AI、通用转发、自研SSH/Telnet、新Tunnel数据面和任意目标端口未实现。本轮不修改这些边界。

最终独立提交标题：`feat: add Phase 6 Windows maintenance workbench`。提交SHA和GitHub main推送状态由实际Git记录/交付回复提供，不在待提交文档伪造自身SHA。推送后停止等待Phase 6 Windows UI验收。
