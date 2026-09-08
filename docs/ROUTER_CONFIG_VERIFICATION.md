# nvram / uci 实现与验证

2026-09-08，Accepted ADR-031。用户回复“按此方案继续实现”，授权实现及相关回归；此前 Win32 宿主迁移停止测试要求不作为本次功能验证的阻塞。

## 实现

Probe 直接以 argv 调用固件 nvram/uci，复用超时、输出限制、进程清理和原任务缓存；配置任务以原 worker/容量按接受顺序串行，Exec 可并行。Server 提供共享参数校验、不可变任务规格、Session capability 检查、公开 config-tasks API 和原身份重发。模板新增 nvram/uci 只读来源，兼容 command、持久化及启动快照。React 设置提供来源选择，设备“更多设备操作”提供配置表单，任务中心显示配置分类/参数/输出，不查询文件关联。未增加 Windows Bridge、业务网络层或依赖。

## 环境与命令

Windows 使用当前仓库、Go、Node/npm、Edge headless；Linux 使用独立 WSL 2 RouterAgentTest（Alpine 3.22.1、GCC 14.2.0、CMake 3.31.7、Go 1.24.13）。源码复制到 `/work-runs/router-config-b360364ae95d4556a50987c656b3184c`；完整回归使用独立 network/devpts namespace，不挂载 Windows 工作区，不占用用户真实 80/22/23 服务。

| 检查 | 命令 / 入口 | 结果 |
| --- | --- | --- |
| Linux Release 全量 | `sh tests/verify-phase5.sh release`，隔离 namespace | 7 项 CTest、全部 Go/真实 Probe/Phase 1～5 集成、vet、Server build 通过；集成包约 173 秒 |
| Linux 全量 Go race | 设置 `RMP_PROBE_BIN="$PWD/build/phase5-probe/router-probe"`，`go test -race ./cmd/... ./internal/... ./tests/... -count=1`，隔离 namespace | 全部通过，使用真实 Linux Probe；集成包约 175 秒 |
| 最后模板互斥字段与不可变规格回归 | `go test -race ./internal/probetemplate ./internal/api ./internal/task -count=1`；同一 RMP_PROBE_BIN 下 `go test -race ./tests/integration -run 'TestRouterConfig\|TestProbeTemplate' -count=1 -v` | 全部通过，明确执行 TestProbeTemplateControlLimit、TestProbeTemplateStartupSnapshot、TestRouterConfigProbeAPI；再次 vet 受影响包并构建 Linux Server |
| Windows Go | `go test ./cmd/... ./internal/... ./tests/... -count=1`、`go vet ./cmd/... ./internal/... ./tests/...`、`go build -o build/router-config/router-server.exe ./cmd/server` | 适用检查通过；需 Linux Probe 的用例在 Windows 跳过。最终模板改动另测 probetemplate/api/task 并 vet/build |
| 前端 | `npm --prefix frontend run build`、`npm --prefix frontend test` | 构建及 13 项测试通过；Vite 保留包体积超过 500 kB 提示，未改依赖或分包策略 |
| 新配置 UI | `node frontend/tests/router-config.mjs` | 真实 Edge + Go API：source 保存/回填、旧 Probe 禁用、原样/空值写入、读取输出、显式 commit、任务分类、浅深主题通过；截图已核对 |
| 旧模板 UI | 设置 RMP_SERVER_BIN 为本次 Windows Server，`node frontend/tests/templates.mjs` | command 模板 CRUD、版本冲突、主题与设备属性展示通过 |
| Windows 单 EXE | `powershell -NoProfile -ExecutionPolicy Bypass -File windows/build-native.ps1 -SkipFrontend -OutputName windows-native-config` | 使用本次前端构建，编译通过；未启动新 EXE 或重验整个 Win32/ConPTY 业务 |
| 文档 | `git -c core.safecrlf=false diff --check`、相关 Markdown 本地链接核对 | 通过；保留原换行配置，不全仓转换 CRLF/LF |

首轮新 UI 脚本连续动作过快，触发既有 350ms 同名动作去抖；脚本按既有模板测试方式等待 450ms 后通过，未放宽产品去抖。截图发现新表单缺少共享输入样式，已补齐、重建并重新验证，不把首次失败记为首次全通过。

## 覆盖与限制

C++ 新测试覆盖非法参数/超时、匿名 UCI 路径、特殊字符原样 argv、空 value、缺程序、非零退出、超时、配置顺序、Exec 独立执行、容量、重复/冲突和重连缓存。Go 覆盖参数边界、HTTP 严格字段/能力错误、模板互斥/持久化及不可变规格。

真实 Probe 集成经 HTTP API 完成两种 backend 的 set/get/delete/commit、特殊字符/空值往返、显式 commit、HTTP 幂等重放/冲突、原任务重发不重复写、运行中写任务断线补报、重连保持启动快照、缺命令和超时。nvram/uci 使用隔离替身程序验证 argv 与副作用次数；浏览器使用 Protocol 测试对端。两者均不代表厂商固件验收。

`sh tests/verify-phase5.sh asan` 和 `race` 的 C++ 阶段实际尝试后，在 CMake 编译器检查失败：缺 `libasan_preinit.o`、`-lasan`、`-lubsan`、`libtsan_preinit.o`、`-ltsan`。C++ ASan/UBSan/TSan 未运行，不以 Go race 代替。准确日志为上述 Linux 目录中的 `release.log`、`go-race.log`、`asan.log`、`tsan.log`、`final-checks.log`。

未交叉编译到 mipsel/ARM/ARM64/uClibc，未操作用户设备、运行数据或已有进程。厂商键名、命令权限/可用性、commit 后服务生效仍需目标固件实测；OpenWrt 缺 nvram SN 时继续显式指定稳定 device-id。

## 本机产物

- Windows 客户端：`build/windows-native-config/win-x64/RouterWorkbench.exe`，嵌入本次 React 页面，客户安装 WebView2。
- Windows Server：`build/router-config/router-server.exe`。
- Linux x86_64 Probe：上述 Linux 目录 `build/phase5-probe/router-probe`，为 Alpine 测试构建，不是其他架构/固件通用包。
- UI 截图：`build/router-config/template-sources.png`、`config-form-light.png`、`config-form-dark.png`、`config-task-result.png`。

未提交或推送 Git。任务开始已有大量改动与未跟踪文件，本次保留。此前误删文件的历史遗留仍见 [PROBE_TEMPLATES_VERIFICATION](PROBE_TEMPLATES_VERIFICATION.md)，本次不改写恢复状态。
