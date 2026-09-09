# 内置系统信息与开机时长验证

2026-09-08，起点 `bdf9f9b`，依据已确认 ADR-038。当前任务为默认 CPU 架构/内核和动态系统开机时长，用户补充年月日时分秒及省略前导空单位。本次没有提交/推送授权。

## 当前实现与验收口径

- Probe 使用 C++11 系统接口，保留编译目标和端序及 --arch 覆盖；uname release 默认上报，显式 kernel 模板优先，失败不会被默认值遮盖，重连复用启动快照。
- sysinfo 系统秒数，失败回退 `/proc/uptime`，整数安全解析；有效零值与失败分开，注册成功立即首报，之后默认30秒（按协商间隔）。服务端原子保存当前 Session 的最新值及采样时间，旧会话不能覆盖；其他消息只刷新 last_seen，runtime 不保存心跳时序。
- 公开设备和 Session 增加 runtime，首报前 null，采集失败的 uptime_seconds 为 null，离线保留最后采样。旧版无有效性标记时正数兼容、零值未知；新会话首报前不沿用旧值。
- WPF 既有五秒 HTTP 刷新，全部属性集合不变时仅按值通知，保留选中行。格式如 `59秒`、`1分 0秒`、`1时 0分 0秒`、`1年 1月 2日 3时 4分 5秒`；固定年365日/月30日，最高有效单位到秒连续显示，中间零保留，离线不累计。

## 环境与结果

Windows Go、.NET 10 / WPF；Linux 使用项目外 WSL 2 RouterAgentTest（Alpine、GCC14、CMake3.31、Go1.24）。仅复制当前源码到 `/work-runs/systeminfo-e1eb551ee6f64ebda6d66bb091949ba3`，控制/维护测试在独立 network/mount/PID/devpts namespace，未将工作区挂载到 Linux 测试根目录。

| 验证 | 命令/结果 |
| --- | --- |
| C++ Release | `sh tests/verify-phase5.sh release` 的构建及 CTest：8/8 通过，含 system_info_tests 与模板默认/覆盖/失败 |
| 真实 Linux 专项 | 设置 `RMP_PROBE_BIN` 后 `go test ./tests/integration -run 'TestProbeRejectsUnsupportedTask\|TestProbeInvalidProtocolReconnect\|TestRepositoryRealProbeCompatibilityAndBinaryAsset\|TestSystemInfoRealProbeAPI' -count=1 -v`：通过；真实 uname 和 /proc/uptime 对照、默认/模板、立即/周期上报、断线重连和进程重启、内核精确匹配均覆盖 |
| Phase 1～5 全量 Release | 隔离环境 `sh tests/verify-phase5.sh release` 通过：8/8 CTest、所有 Go 包和真实 Probe 集成（182.678秒）、go vet、Linux Server build |
| Go race | 同一隔离方式设置 `RMP_PROBE_BIN`，`go test -race ./cmd/... ./internal/... ./tests/... -count=1` 全部通过，真实 Probe 集成183.919秒；使用 Release Probe，不声称 C++ TSan 通过 |
| Windows Go | `go test ./...`、`go vet ./...` 通过；Windows 不具备 Linux Probe 的集成用例由 Linux 单独覆盖，不把跳过算成通过 |
| WPF | `windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-systeminfo`：Release 自包含发布与 120 项检查通过，包含当前 Go HTTP/WS、零值/失败/离线、五秒更新与选择保持 |
| 实际控件布局 | `python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-systeminfo/verification`：12 份 WPF XPS 转图通过；检查浅/深设备页，CPU、内核、时长和采样时间完整可读 |
| 发布程序启停 | 当前任务自包含 EXE 启动至主窗口，CloseMainWindow 与 WaitForExit 成功；仅关闭本次启动的 PID，未停止用户进程。重新获取的进程句柄未提供退出码，不记录为 exit=0 |
| C++ ASan/UBSan/TSan | 本次重新尝试 CMake 配置，工具链缺 libasan_preinit.o、libtsan_preinit.o、-lasan/-lubsan/-ltsan，编译器检查失败；未执行 sanitizer，不能以 Go race 替代 |

初轮全量发现旧测试假定注册后无立即心跳及默认 kernel 为空，已按新契约修正测试对端，并保持逐帧序号、ACK reply_to、非法协议断线断言；新增实际内核精确匹配/不匹配断言。WPF 初轮暴露全部属性表重建行的问题，已改为稳定值绑定并由选择保持检查验证。

产物：`build/windows-desktop-systeminfo/win-x64/RouterWorkbench.exe`；对应当前 Windows Server 在该目录兄弟 `verification/router-server.exe`。Linux x86_64 Probe/Server 已复制到 `build/systeminfo/router-probe-linux-x86_64` / `build/systeminfo/router-server-linux-x86_64`，原产物仍保留在上述 WSL 本次目录。查看新功能需配套更新 Probe、Server 与 WPF；这些 Linux x86_64 产物不用于 MIPS/ARM 路由器。日志为 `build/systeminfo-*.log`（最终 Linux 全量/race 使用 `*-final.log`），本地构建与测试数据不进入 Git。

## 真实遗留

尚无 mipsel/ARM/ARM64 厂商路由器实测或可宣称通用的交叉工具链。架构/端序函数测试不是实机验证；WPF 矢量布局不是物理 DPI/鼠标验收。当前 C++ sanitizer 环境缺口如上。此前 `cmd/server/1.txt`、`cmd/server/data/` 未恢复和 Phase 6 最终验收状态保持，见 PROJECT_STATUS，不属于本次新问题。
