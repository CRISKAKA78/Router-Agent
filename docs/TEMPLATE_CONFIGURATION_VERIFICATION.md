# 模板配置工作区验证

2026-09-09，Accepted ADR-042。用户批准页面拆分、属性归组联动和物理口说明后实施；操作说明见 [TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md) 与 [MANAGED_PROBES_DESIGN](MANAGED_PROBES_DESIGN.md)。

## 结果与环境

Windows x64 / .NET 10.0.400 / Go 1.25.5 / Node 24.12.0 / Edge；Linux使用独立WSL RouterAgentTest、GCC14.2.0、Go1.24.13、CMake3.31.7。当前源码独立副本 `/work-runs/template-config-20260909`；完整真实服务用例使用network/pid/mount隔离及loopback/devpts，不占用用户80/22/23。

| 验证 | 命令 / 实际结果 |
| --- | --- |
| 生成器单元与BusyBox兼容 | `RMP_GENERATOR_WSL=RouterAgentTest`，`build/dotnet10/dotnet.exe test ProbeTemplateGenerator.sln -c Release --no-restore`：164通过、0失败、0跳过 |
| 生成器发布入口构建 | `template-generator.ps1 -BuildOnly`：0警告、0错误；同源码独立5196实例用于浏览器检查 |
| 真实浏览器与Go API | `GENERATOR_URL=http://127.0.0.1:5196`、当前 `RMP_SERVER_BIN=build/template-config-server.exe`，`npm.cmd --prefix tests run test:generator`：10组全部通过、无浏览器错误；最终补丁已重建宿主并复跑 |
| C++ Release | `sh tests/verify-phase5.sh release` 的build/ctest：12组通过，包含新增省略端口、真实0号、未匹配未知和厂商记录ID关联 |
| Linux真实业务 | 同脚本的完整integration通过，218.059s；该次API慢消费者测试失败，不能将本次release命令称为全绿，原因和修正见下文 |
| Linux完整Go race | 配套实际Probe，`go test -race ./cmd/... ./internal/... ./tests/... -count=1`：全部通过，integration220.892s，含新增 `TestManagedProbeOptionalPhysicalPort`；随后Linuxvet和Server构建通过 |
| 最终API测试修正 | `go test -race ./internal/api -count=1`：通过（12.921s） |
| Windows Go | 最终 `go test ./cmd/... ./internal/... ./tests/... -count=1`、`go vet`、Server构建通过；真实Probe用例按Windows环境跳过，Linux证据如上 |
| 文档 | UTF-8/相对链接及 `git diff --check` 通过 |

## 行为覆盖

- 配置页独立四分类、属性页无全局配置、仅属性页显示属性导航；浅深主题与1920/2560/3440/900宽度原有页面及1920/900配置页检查。
- 同一presentation数据支持统一字段列表与属性编辑器，搜索中文名称/标识，内置字段自动列出，虚拟属性不作为独立结果；重命名、复制继承组并置末尾、删除与删除分组、上移/下移稳定排序。
- 分组预览保持最终顺序；模拟结果与待设备采集区分，属性公式、规则、来源、超时和监控周期的原功能回归。
- 工程6保存/重开及旧工程5/1、原草稿导入；旧数字port仍保留，新的DSA无编号工程在编译、HTTP发布、存储、重读中保持省略；0与未知不混淆。
- 物理口按后端提示匹配条件，swconfig缺编号报错；厂商输出按记录ID匹配而不错误重定向sysfs；真实Probe收到省略编号的厂商配置，确认并上报正确链路和未知编号。
- 型号目录/模板绑定、发布与版本冲突、原请求恢复等既有行为保持。

## 验证中发现并处理的问题

初次Linux测试副本缺go.sum，补齐当前仓库文件后重新执行。工程版本测试原来把6视为不支持，已更新为999并保留不支持版本断言。

Windows与Linux完整回归均出现原有 `TestWebSocketClientsSlowConsumerAndShutdown` 偶发失败：注册/纳管的多条通知可能在连接关闭前留在缓冲区，旧测试最多读取两条便判失败。修正仅针对测试，使用1秒截止时间排空通知，并要求明确的关闭/EOF；超时仍失败。没有降低断言或修改业务广播机制。修正后Windows完整测试与Linux API race通过；之前的完整Linux race也通过。保留失败日志，不声称所有首次尝试成功。

## 产物与限制

- 生成器入口 `template-generator.cmd`；BuildOnly结果位于 `build/template-generator/build-only/`。旧实例不会自动加载新代码：在其启动窗口Ctrl+C退出后重新启动。
- Windows Server：`build/template-config-server.exe`。
- Linux实验产物：`build/template-config-probes/router-probe-linux-x86_64`、`router-server-linux-x86_64`；不是ARM路由器成品。
- 日志：`build/template-config-tests.log`、`template-config-build.log`、`template-config-browser-final.log`、`template-config-linux.log`、`template-config-race.log`、`template-config-api-final.log`、`template-config-go-windows-final.log`。
- 截图与浏览器结果：`build/template-config-browser/`，包含分组配置、物理口表单及深色窄窗口。已实际查看对应图片。
- C++ ASan/UBSan/TSan已尝试，仍因缺少libasan/libubsan/libtsan与preinit链接文件无法运行；日志 `build/template-config-asan.log`、`template-config-tsan.log`。Go race通过不代表C++ sanitizer通过。
- 本轮未构建/部署新版ARM，未执行厂商逐口实机验收；原ARM产物、用户进程和数据保留。WPF源码本轮未改，未重跑195项旧WPF检查，也不将旧证据当本轮结果。
- 保留任务开始前已有改动；未Git提交/推送。此前工作区事故的数据恢复问题不属于本次已完成范围。
