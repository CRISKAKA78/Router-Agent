# 服务端纳管与动态配置验证

日期：2026-09-09；对应 Accepted ADR-041。需求/操作/升级说明见 [MANAGED_PROBES_DESIGN](MANAGED_PROBES_DESIGN.md)。这是本轮实现证据，不代表厂商实机验收或 Phase 6 最终验收。

## 环境与命令

- Windows amd64：Go 1.25.5、.NET SDK 10.0.400、Node 24.12.0、现有 Edge/Playwright。保留任务开始前已有改动、用户数据和用户进程，未Git提交/推送。
- WSL `RouterAgentTest`：Alpine、Go 1.24.13、GCC 14.2.0、CMake 3.31.7，独立副本 `/work-runs/managed-20260909`。未将用户工作区挂入Linux测试环境。
- Phase1～5真实服务用例在独立network/pid/mount namespace中运行，loopback与devpts由测试创建，避免占用用户80/22/23端口。

| 检查 | 命令/入口 | 结果 |
| --- | --- | --- |
| Probe C++ Release | `sh tests/verify-phase5.sh release` 中的cmake/build/ctest | 12组全部通过；含CPU两次、热配置、DSA/sysfs、swconfig、厂商输出格式、板卡型号回退 |
| 真实Linux完整业务 | 同入口，`RMP_PROBE_BIN=.../router-probe go test ./cmd/... ./internal/... ./tests/... -count=1` | 全部通过，integration 221.772s；实际Linux Probe与隔离真实维护服务，未跳过Probe用例 |
| 完整Go race | 同隔离namespace，`RMP_PROBE_BIN=.../router-probe go test -race ./cmd/... ./internal/... ./tests/... -count=1` | 全部通过，integration 220.048s |
| 最终受影响补丁 | CTest全部12项；`go test -race` 的API/Gateway/Management/真实Probe纳管、配置确认、模板、配置任务专项；随后端口/CPU2项与目录/管理race | 全部通过；最后型号无别名归一化、端口探测元数据回填及编译警告修正另验证 |
| Go静态检查与构建 | Linux/Windows `go vet ./cmd/... ./internal/... ./tests/...`、`go build ./cmd/server`；Windows完整Go测试 | 通过。Windows不提供Linux Probe，真实Probe证据使用上述Linux结果，不把Windows跳过当作执行 |
| WPF | `windows/build-desktop.ps1 -BuildOnly -Verify` | 自包含Release发布成功，195项检查通过；使用当前Go Server和测试专用TCP协议对端 |
| Blazor生成器 | `RMP_GENERATOR_WSL=RouterAgentTest`，`dotnet test ProbeTemplateGenerator.sln -c Release --no-restore`；`template-generator.ps1 -BuildOnly` | 158项全部通过、无跳过；BuildOnly 0警告/错误 |
| 浏览器真实流程 | 本轮独立Blazor 127.0.0.1:5194；当前Go API；`npm.cmd --prefix tests run test:generator` | 9组场景通过，无浏览器错误；最终版本包含新布局样式和真实模板版本文字 |
| 实际WPF布局 | Desktop.Tests的XPS/PNG，`python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop/verification` | 本轮纳管表单、分组、端口表与原有浅深主题页面已渲染检查；目录中的旧tasks/tools图不是本轮产品页面 |

初次验证发现的仅采一次字段首报被waiting时间戳挡住、分组表格虚拟化异常、型号对象匹配及可编辑ComboBox缺少文本部件、型号写入草稿校验未接受新路径，均已修复并由本轮对应用例通过验证。未删测试或降低业务断言。

## 新能力覆盖

- 未纳管不进设备列表，忽略/恢复、重复幂等请求、版本冲突、重连不丢管理员决定；Server拒绝未纳管Exec/维护等业务派发。
- 型号名称/别名精确匹配、冲突拒绝、默认模板、引用删除保护；离线资料与绑定快照跨Server重启保留，不伪造在线/已生效。
- CONFIG_ACK丢失后原载荷重发、旧关联ACK不生效、确认前不冒充成功、旧修订遥测隔离；新周期和模板在同Session热更新，重连恢复并确认。
- 模板发布新版本不自动应用；显式应用后切换。原始REGISTER不变，interval=0首采和重连缓存都保留，设备覆盖不修改模板默认。
- CPU静态检测进程内只有两次，反复启动collector不增加次数且保留采样年龄；使用率/频率/其他监控仍周期运行。
- DSA/sysfs与swconfig测试夹具确认上联/端口/状态独立，内部CPU口单独标识；厂商命令失败不伪造DOWN。夹具不是某款真实交换机兼容性证明。
- WPF实际纳管表单改名、自动型号/模板、输入搜索、配置确认、组与字段顺序、默认其他信息、全部折叠/刷新保持、端口别名和物理/管理状态分离、管理员型号优先摘要；离线设备可打开采样配置、保存新周期并保持等待下发。
- 生成器工程/运行格式往返、旧工程兼容、仅内置布局无需占位命令、分组/排序/别名/物理口发布、型号目录保存与精确匹配、不确定请求原字节恢复契约。

## 产物与日志

所有产物/测试仓库均在Git忽略的build内，既有发布目录未清空：

- WPF：`build/windows-desktop/win-x64/RouterWorkbench.exe`；入口 `ui-windows.cmd`。
- Windows Server：`build/managed-router-server.exe`。
- Linux实验产物：`build/managed-probes/router-probe-linux-x86_64`、`router-server-linux-x86_64`。Probe为x86-64动态musl ELF，供对应实验环境验证，不能当作ARM/uClibc路由器产物。
- 生成器：入口 `template-generator.cmd`；本轮BuildOnly输出 `build/template-generator/build-only/`。已有运行实例仍按原入口规则复用，使用新代码需正常退出旧实例再启动。
- 本轮Go/C++日志：`build/managed-probes/release-final.log`、`go-race.log`、`final-checks.log`、`final-build.log`；Windows `build/managed-go-windows.log`、`managed-go-final.log`、`managed-catalog-final.log`。
- WPF：`build/managed-wpf.log`、`build/windows-desktop/verification/admission-form.png`、`managed-groups.png`、`managed-switch-ports.png`。
- 生成器：`build/managed-generator-tests.log`、`managed-generator-build.log`、`managed-browser.log`、`build/managed-generator-browser/browser-results.json`及布局PNG。

## 未通过/待实机范围

1. ASan/UBSan/TSan已实际尝试，CMake编译器链接检查因缺 `libasan_preinit.o/-lasan/-lubsan`、`libtsan_preinit.o/-ltsan` 失败。见 `build/managed-probes/asan.log`、`tsan.log`。未声称sanitizer通过，未擅自安装或升级系统依赖。
2. 10.1.1.128 GCC5.2编译机本轮 `ssh -o BatchMode=yes -o ConnectTimeout=5 root@10.1.1.128 ...` 返回 `Permission denied (publickey,password)`。未获得新版ARM构建/运行证据；旧ADR-040 ARM成品保持不变，不能替代新版验证。
3. 各厂商固件、私有命令、交换机芯片号与机壳WAN/LAN丝印、内部CPU口、逐口插拔/链路管理状态，以及真实DPI均待实机验收。自动探测与可配置命令通道已实现，具体型号映射需以真实硬件确认。
4. 此前工作区事故中 `cmd/server/1.txt` 与 `cmd/server/data/` 未恢复的问题仍按原接管记录保留，本轮未声称恢复这些数据。
