# 设备日志：使用、实现与验证

> 合入更新：用户已授权本工作树（含 AT 前置智能邻居）并入本地 main；当前进度、统一 ADR 编号及联合验证见 [WORKTREE_INTEGRATION](WORKTREE_INTEGRATION.md)。下文独立工作树、未提交或原目录不写入等语句记录当时事实，不限制本次授权；原始测试证据仍在对应工作树的忽略目录，不能当作本次联合测试结果。

依据ADR-061。工作树 `C:/Users/Administrator/Desktop/router-agent-device-logs`，分支 `codex/device-logs`，基线f331ffb；没有提交、推送或远程部署。原始SSH只读调查属于前一轮，不能当作新功能实机验证。

## 使用

1. 使用本轮Server和Windows客户端，设备需由本轮Probe注册 `device_logs_v1`。旧Probe会显示不支持，不发送新命令。
2. 设备工作区进入**日志 → 实时日志**，Probe设置debuglog_enable=1、syslogd_enable=3并自动nvram commit，然后每秒读取/tmp/.systemlog。停止采集/切页只停读取，开关保持开启。注意commit提交整份已暂存NVRAM，不仅这两个键。
3. 实时支持关键词过滤、暂停显示/跟随滚动、清空显示、导出已采集原始文本。客户端最多保留512KiB，初次从设备尾部最多8192字节开始；缺段/缓存截断明确标识，不把它当作全历史导出。
4. **历史日志**自动检查/tmp/third_party/data与/jffs（别名去重），补充/tmp/root的RAM缓存并单独标识；其他路径可手填。关闭历史仍能导出已有文件；未开启且没有发现文件给出提示，无法读取目录不被描述成确定不存在。
5. 历史保存勾选框独立设置log_save_en，间隔log_save_itv默认300秒，可填1～65535；开启同时打开debuglog总开关。勾选“保存到设备配置”才commit，应用前确认；关闭历史不影响实时输出。不重启服务。
6. 历史支持多选导出原始gzip或解压TXT；下载前建立有界快照，经原有File协议确认完整文件后释放探针临时副本。相同文件名不覆盖彼此，已有目标由用户确认。
7. **分析 / 原文**查看已导出或本地导入的txt/gz。Server读取并校验所有拼接gzip成员，解压上限64MiB，最多显示512KiB；完整TXT可导出。当前明确显示“厂商解析器待样本”，不生成基站/时间范围/故障判断。原始gzip损坏时仍可保存原始字节，但会提示校验未通过。
8. 响应不确定时使用“开始/继续”“继续原设置”“继续原导出”或现有全局重试入口；原请求键、字节、task_id保持。切设备/Server不把旧结果贴到新设备。

## 代码入口

- Probe：probe/include/rmp/device_logs.h、probe/src/device_logs.cpp、client.cpp、task.cpp、task_manager.cpp。
- Go：internal/devicelog、internal/gateway/device_logs.go、internal/task/device_logs.go、internal/management/device_logs.go、internal/api/device_logs.go。
- Windows：RouterWorkbench.Client/DeviceLogs.cs、RouterWorkbench.Desktop/LogWorkspace.cs；原FileExchange继续负责File状态机与下载完整性校验。
- 契约见API、PROTOCOL；主要设计见ADR-061。没有引入新Tunnel、数据库或依赖包。

## 本地验证记录

证据目录 `build/device-logs-verification/`（忽略文件，不提交）。

- Windows：`go test ./...` 通过，见go-tests.log；包括新参数/查询、gzip多成员与损坏、API幂等、真实TCP Gateway会话/限流/断连及无轮询Task记录测试。Windows上的Linux Probe集成跳过不计作MIPS验证。
- Linux：复用RouterAgentTest WSL，当前源码独立复制到 `/work-runs/device-logs-516d2f8b65144c11bc8d7cfe9b6978b8`，未复制/编译Probe。`go test -race ./internal/devicelog ./internal/gateway ./internal/api ./internal/management`、`go vet ./...`、`go build -o agent-server ./cmd/server`通过，见linux-go.log。第一次归档遗漏go.sum导致API setup失败，补齐原仓库go.sum后重新通过，没有改依赖。
- WPF：使用本机原仓库build/dotnet10/dotnet.exe (.NET10)，`dotnet run --project windows/RouterWorkbench.Desktop.Tests/RouterWorkbench.Desktop.Tests.csproj -c Release -- <本轮router-server.exe> <证据目录>`；最终 **580项检查通过**，生成72份WPF渲染图；日志实时/历史/原文三页已查看布局，见desktop-tests.log及device-logs-*.png。新检查使用真实Go Server+可控C# TestProbe，不是真设备。
- WPF验证覆盖打开实时自动持久开启、停止读取不恢复、历史关闭仍发现文件、File+TXT多成员导出、快照释放、显式历史持久开关、split UTF8/缓存/轮转、响应不确定原请求重试、原文切页保留。现有导航断言同步增加logs页，不降低断言。
- 发布命令：`windows/build-desktop.ps1 -BuildOnly -Dotnet <原仓库build/dotnet10/dotnet.exe> -OutputName windows-desktop-logs`；发布成功，见desktop-publish.log；单独输出、不启动或替换用户正在运行的程序。

## 交付产物

- Windows：`build/windows-desktop-logs/win-x64/RouterWorkbench.exe`。
- 本轮Windows Server：`build/device-logs-verification/router-server.exe`。
- 本轮Linux amd64 Server：`build/device-logs-verification/agent-server`。
- 本轮不提供Probe编译产物，探针源码与回归测试入口已更新。

## 用户侧待验收与限制

- 按用户指令，Probe编译和设备功能测试由用户完成；本轮未运行C++编译、device_logs_tests、目标MIPS/uClibc构建、C++ sanitizer或设备开关/落盘/断线测试。源码检查不能替代这些验证。
- 用户在自己的MIPS工具链构建；若环境支持运行测试，可执行新增 `device_logs_tests` CTest。不要以既有ARMv7包或前期只读SSH结果代替。
- 真机重点：从两个开关关闭开始打开实时页，回读两个值及commit后的持久性；生成/轮转.systemlog；历史开关/间隔、两条默认路径与自定义路径；多段gzip、导出期间文件变化、取消/断线恢复、关闭客户端不关开关。
- 快照单份32MiB、总64MiB、最多8份、超时25秒；正常导出自动释放，取消/失败遗留在本进程内1小时清理，进程崩溃遗留不擅自扫描删除。超过这些边界明确报错；目录扫描有界且不递归。
- 发布EXE原生手工交互、物理DPI/辅助技术及目标设备性能待用户验收；WPF渲染属于实际控件离屏布局证据。
- 厂商AT解析、时间范围、基站去重/切换及其他分析字段需用户提供样本后另行实现；当前不宣称已支持。
