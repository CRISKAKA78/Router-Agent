# 属性模板验证与工作区恢复记录

日期：2026-09-07；本次新增代码从远端基线 `f73853f29fcbc3b49a84b1ad694de596b95373a6` 恢复后执行验证。功能依据 ADR-029，未提交或推送。

## 工作区误删事件

Agent 在清理首次失败的 Linux 测试 rootfs 时，工作区 bind mount 卸载失败后仍执行递归删除，导致源码目录（含 .git）、已有未跟踪运行文件和构建产物被删除。这是 Agent 操作错误，非用户操作或产品缺陷。

已从 GitHub main `f73853f` 重新克隆已提交项目，并从当前任务的原始补丁记录逐项恢复本次及上一轮默认 ID 改动；恢复后重跑下述检查。旧 .git 的本地 reflog/额外 refs 无法由远端恢复；不声称保留了原始本地 Git 元数据。前端依赖、当前 Server/Probe 构建产物已重新生成。

开始时已知未跟踪 `cmd/server/1.txt`、`cmd/server/data/`、`cmd/server/router-server.exe`；前两项没有找到恢复来源，不能伪造空数据或声称已恢复。已检查 Windows 卷影副本 HarddiskVolumeShadowCopy16（2026-09-07 09:13:52），其中没有这两项数据；已从该副本恢复 `build/windows-react`，但不代表误删前最新状态，也不包含本次新功能的发布验证。原 Server EXE 未按原字节恢复，当前版本已另行构建。已向用户说明事件并询问备份路径；有备份后应按实际内容恢复，不能用新模板测试数据替代。所有后续 Linux 验证使用源码 tar 副本和独立磁盘镜像，不再 bind mount 工作区，也未再次执行目录清理。

## 已完成验证

- Windows / PowerShell：`go test ./cmd/... ./internal/... ./tests/... -count=1`、`go vet ./cmd/... ./internal/... ./tests/...`、`go build -o build/template-check/router-server.exe ./cmd/server`。Linux 专用 Probe 集成在 Windows 跳过，不计入真实 Probe 通过。
- React：`npm --prefix frontend run build`、`npm --prefix frontend test`（13 项）。构建保留既有 >500 KiB bundle 提示，未改依赖版本或拆分无关模块。
- 浏览器：`node frontend/tests/templates.mjs`，Windows 本机 Edge headless + 实际 Go API/Vite，隔离测试目录和 18573/18580～18582 端口。覆盖创建两项模板、编辑名称、并发修改后旧版本拒绝且保留编辑内容、删除、深浅主题，以及注册快照中的自定义值与失败原因展示。截图在 `build/template-check/templates-light.png`、`templates-dark.png`、`collected-properties.png`；设备展示使用协议测试对端，另由下述真实 C++ Probe 集成覆盖采集。
- Linux x86_64 / WSL kernel 6.6.87.2 / Alpine 3.22 / GCC 14.2.0 / Go 1.24.13：独立 ext4 rootfs 内使用 `unshare -mnpf` 的 mount/PID/network namespace，挂载该 namespace 的 proc/devpts 并只开启 loopback，执行源码副本的 `/bin/sh tests/verify-phase5.sh release`，退出 0。包括 6 项 CTest、Phase 1～5 全量 Go 测试、真实 Probe 与隔离 Web/SSH/Telnet 维护、Go vet 和 Linux Server 构建；完整集成耗时约 169 秒。
- 新增 `TestProbeTemplateStartupSnapshot` 真实 C++ Probe + Template API/Device API：按名称获取和按 ID 获取、nvram 测试替身返回自动 ID、显式 ID/hostname 优先、命令取值/失败摘要、HTTP 输出、编辑模板后重连不重采集、Session 历史保留原版本、新进程采用新版本、不存在模板明确退出。独立执行通过。
- 新增 Service 测试覆盖单写者、重开恢复、并发预期版本只有一个成功、名称冲突、输入/查询副本隔离、删除墓碑、损坏/缺失目录拒绝、保存失败不发布内存；Device 测试覆盖注册输入、查询和历史快照隔离；API 测试覆盖 CRUD、原键重放、失败/非法选择和准备连接不建设备。
- Linux 全量 Go race：同样隔离 mount/PID/network/devpts，在 Go 源码副本执行 `RMP_PROBE_BIN=/work/build/phase5-probe/router-probe go test -race ./cmd/... ./internal/... ./tests/... -count=1` 退出 0；包括新增模板集成，完整集成约 171 秒。C++ Probe 使用已通过的 Release 构建，该结果不代替 C++ TSan。
- 全量检查后补充服务端控制报文上限下发和注册前大小校验：重新构建 Probe，6 项 CTest 全部通过；`go test -race ./internal/gateway ./internal/api ./tests/integration -run Template -count=1` 通过（Gateway 无匹配测试），模板启动快照及新增 `TestProbeTemplateControlLimit` 两项真实 Probe 集成通过。后者验证 1024 字节上限下过大采集结果明确退出且不发布设备。Windows 受影响 Go 测试、vet、Server 构建再次通过。

## 检查限制

- `/bin/sh tests/verify-phase5.sh asan`、`race` 的 C++ 阶段尝试失败：当前 Alpine GCC 不带 `libasan_preinit.o/-lasan/-lubsan` 或 `libtsan_preinit.o/-ltsan`，工具链无法通过 CMake 编译器探测。未降低断言或删除测试；不能计为 ASan/UBSan/TSan 通过。
- Windows Go race 因 CGO 未启用无法运行；Linux 全量 Go race 已通过，未把平台限制记为 Windows race 通过。
- 初次 CTest 因跨 WSL PID namespace 的旧 proc 挂载无法解析 `/proc/self/exe`，修正为每次创建 PID namespace 并挂载 proc 后，6 项全部通过；未修改既有 helper 断言。
- 未执行厂商路由器的 nvram/自定义指令、mipsel/ARM/ARM64/uClibc/老内核实机矩阵，未重新发布 WinUI 安装目录或执行 WebView2 原生发布验证；本轮未改 Shell/Bridge。Phase 6 最终实机验收不因本功能通过。
