# 属性展示与UI改造交付（ADR-044）

2026-09-09。用户审核七项UI/模板改造及“删除旧版兼容”补充后确认实施。当前代码、构建和本机验证已完成；无Git提交/推送，未替换用户正在运行的Server、UI或Probe，未清空运行数据。

## 已交付行为

1. **删除接口映射**：移除接口显示名页签、编辑器、模型字段、编译/导入导出、Server存储/DTO和WPF别名应用。`interface_aliases`不再是有效字段。物理端口本身的display_name、系统接口匹配和上联关系是现行采集功能，继续保留。
2. **文字与列宽**：所有WPF DataGrid自动行高，默认单行，去掉维护链接重复上下边距，保留字体下沿。原数据换行仅在单行显示时压为间隔，原文复制不变；超长值可悬停查看。用户手动调窄列后该列换行且行高增长。Blazor数据表也默认单行，可拖列头右缘或用左右键调整宽度，调窄允许换行。说明文字/代码编辑器按用途保留多行。
3. **连续滚动**：WPF统一按像素滚动并保留分组虚拟化；高于窗口的组/行中部可达。水平滚动条改为正确的左右翻页命令。属性刷新不重新设置列宽，结构变化尽量保留原字段位置。
4. **内置展示选择**：生成器“模板配置→分组与排序”提供分类开关与逐字段显示覆盖。disk/network/switch明细默认隐藏，其他分类显示；字段可覆盖分类或恢复默认。开关跟随模板显式应用，预览同样过滤，空组不显示。采集计划、API指标、摘要、存储/网口专页和曲线不受影响。复制属性同时复制显示设置。
5. **名称与标识**：属性名称不附技术key；字段key继续用于关联、悬停提示、右键“复制属性标识”和“复制完整值”。
6. **模板顺序**：属性表删除重复分组列，仅保留属性/值；关闭列头排序，组与字段按模板数字顺序排列，其他信息最后。不会因双击分组改变数据顺序。
7. **整行折叠**：分组标题横跨表格，轻背景、上下边界与留白区分数据；整个标题和键盘可操作，双击不重复翻转。当前窗口按Server/设备/稳定组ID记忆折叠状态，提供全部展开/折叠。网口页的两个高级分组沿用同一标题样式。

## 唯一版本基线

- 工程仅接受schema_version=8，浏览器工作区仅接受workspace_version=3，storage key为router-template-generator.workspace.v3。删除旧项目/草稿读取与迁移，不自动读取或删除旧浏览器数据。当前运行模板仍可导入；发布模板版本不等于工程格式版本。
- 删除TEMPLATE_GET/REPLY处理和Probe的--template-id/--template-name，不再把旧参数当作忽略项。模板只通过CONFIG_APPLY执行，删除同步启动采集及其REGISTER结果封装。
- REGISTER不再接受template/attributes/collection_errors/report_intervals；API registration也不再返回它们。有效模板和样本使用active_template/effective_metrics。设备配置建立waiting字段，随后由当前修订EVENT更新，不回填旧注册样本。
- Server注册要求managed_config_v1与telemetry_v2，成功ACK须确认两者，控制上限配置至少64KiB；Probe不接受旧ACK或遥测降级。未配置时不启动遥测collector。HEARTBEAT的uptime_valid必填，不推断缺失值。
- 保留实际设备平台适配：BusyBox、uClibc、旧Linux内核、GCC5.2目标；保留当前可选能力、未知/错误、修订号、幂等及Session生命周期保护。
- 配套升级当前程序。包含已删除字段的旧模板/目录或旧工程需重新生成；本次没有提供迁移层，也没有改写用户运行数据。

规范见 [ADR-044](DECISIONS.md#adr-044-最新产品基线与属性展示工作区)、[API](API.md)、[PROTOCOL](PROTOCOL.md)。

## 验证证据

环境：Windows x64，.NET10.0.400、Go1.25.5、Node24.12.0、Edge；WSL RouterAgentTest，GCC14.2/musl、Go1.24.13。独立Linux副本为`/work-runs/ui-refinement-20260909`，完整业务测试使用network/pid/mount/devpts隔离。Windows证据在`build/ui-refinement`。

| 检查 | 命令/结果 |
| --- | --- |
| Windows Go | `go test ./cmd/... ./internal/... ./tests/... -timeout 90s`通过；`go vet`同包通过；Windows Server构建通过。Windows上的真实Linux测试跳过，不能替代下行实证。`go-current.log`、`windows-vet.log` |
| 真实Linux Release | `RMP_PROBE_BIN=... go test ./cmd/... ./internal/... ./tests/... -count=1 -timeout 420s`全包通过，integration220.940s；`linux-final.log` |
| 真实Linux Go race | 同环境`go test -race ... -count=1 -timeout 480s`全包通过，integration222.767s；`race-final.log`。Linux vet/Server构建通过 |
| C++ | 最终`cmake --build build/probe -j4`、`ctest --test-dir build/probe --output-on-failure`：13/13通过，17.66s。旧启动采集断言迁入当前TelemetryCollector测试；增加无配置不发遥测、旧成功ACK缺能力拒绝。`ctest-final.log` |
| 最终Probe专项 | 清理最后一个未用准备字段后，以最终二进制执行`TestManagedProbeAdmissionHotConfigurationAndReconnect`通过，3.777s；`managed-final.log` |
| WPF | `.NET10 run --project windows/RouterWorkbench.Desktop.Tests -c Release -- <隔离Server> <证据目录>`：224项通过。新增100项长组、13/37像素偏移、中部可达、整行可访问折叠、手动调窄/自动行高/原文复制、展示覆盖及专页保留检查。`wpf-final.log` |
| WPF图像 | render.py从实际WPF XPS生成24幅布局，含浅/深色、1000/1480/1920窗口、长组和维护链接；已检查维护链接下沿、多行文字、整行分组和网口布局。导出直接使用当前已加载布局，避免卸载控件导致虚拟化列遗漏；此项不是厂商物理屏幕/DPI验收 |
| 生成器 | `RMP_GENERATOR_WSL=RouterAgentTest` + `dotnet test ProbeTemplateGenerator.sln -c Release`：169通过，0跳过，包含BusyBox公式/规则；`generator-final.log` |
| 浏览器/API | 隔离5191生成器/18680测试API，`node tests/template-generator-browser.mjs`：12组通过，无浏览器错误。含鼠标/键盘列宽、单行/换行、分类/字段开关、旧格式拒绝、复制/删除、导入导出/发布以及900～3440宽度浅深色；`browser-final.log`和`browser/browser-results.json` |
| 构建 | `template-generator.ps1 -BuildOnly`通过，0警告/错误；`windows/build-desktop.ps1 -BuildOnly -OutputName windows-desktop-refined`成功发布自包含EXE |
| C++ sanitizer | 已尝试ASan/UBSan和TSan；CMake链接检查缺libasan_preinit.o、libtsan_preinit.o及asan/ubsan/tsan库，未通过。保留asan.log/tsan.log，未安装或升级环境依赖 |

前几轮暴露的旧测试参数、旧REGISTER断言和旧边界值已经转为当前协议；旧采集成功断言迁入当前collector，未通过降低断言掩盖失败。浏览器脚本等待Blazor选择/新增渲染完成，并在鼠标操作前滚动到列头。此前失败日志保留在build目录，以上为最终结果。

## 产物与使用

- 新Windows客户端：`build/windows-desktop-refined/win-x64/RouterWorkbench.exe`，自包含；本次未启动它替换用户实例。
- Windows Server：`build/ui-refinement/router-server.exe`；Linux Server/Probe：`build/ui-refinement/linux/`，为本机musl x86_64验证产物，不能当成ARM成品。
- 生成器入口仍为`template-generator.cmd`。源码已更新；若旧实例正在运行，需正常退出原窗口后重启加载，或者选空闲端口启动。测试5191实例在验证结束后关闭。
- 展示开关保存后先发布模板，再在设备资料显式应用。仅发布不自动替换已绑定设备快照。

本次未构建或部署厂商ARM版本，未进行物理DPI/触控板/逐口业务流量验收；历史ARM成品和设备验证不作为本次证据。现有用户进程、已有改动与运行数据保留，未Git提交/推送。
