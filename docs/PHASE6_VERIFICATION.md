# Phase 6 Shared React / WebView2 验证记录

> 2026-09-08 清理更新（ADR-037）：仅保留新版 WPF 主 UI 与 Blazor 生成器；旧 UI 源码、宿主与专属脚本已移除。下文各次迁移的旧路径和测试结果保留为历史证据。当前构建/测试入口见 [DEVELOPMENT](DEVELOPMENT.md)，本次清理验证见 [UI_CLEANUP](UI_CLEANUP.md)。

## UI Freeze + Production Integration（2026-09-07）

本节是当前交付证据，下方 Mock 迭代与 `404b083` 记录仅为历史。用户确认冻结实际 React 页面，随后明确默认内置 Shell、允许外部工具；实现依据 ADR-027。正式 App 使用 `ui/` 的冻结布局与既有 API/WS 层，preview 保留参照。Management Server、Probe、Protocol、Go 依赖及 Tunnel 生产代码未改动。

### 构建与业务集成

- `windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify` 全流程通过，日志 `build/ui-freeze-windows-verification.log`。后续修复终端自然退出尾部输出、任务 Operation 分类和空态文案后，重新执行前端构建、原生构建/发布、对应集成与正式包启动检查；当前生产 JS 662.27 kB、CSS 88.76 kB（未 gzip）。Vite 提示大于 500 kB 的单 chunk，不是构建失败；本轮没有为消除此提示改变冻结 UI。
- Vitest 13 项：既有 API 6、Connection 3，加目录路径引用/NUL/截断/上限 4 项，全部通过。
- 原生 27 项：来源、导航、路径、方法与参数拒绝、独立 SSH 参数、任意终端程序拒绝、尺寸边界、ConPTY UTF-8、自然退出排空第 5000 行、积压输出关闭、独立租期释放、原生保存完整性及失败清理通过。
- 真实 WebView2 31 项全部通过，最终全流程日志 `build/ui-freeze-native-final.log`，截图及对端日志 `build/react-shell/verification-20260907-034405/`。
- 31 项覆盖：正式资源与配置/HTTP/WS；危险 Bridge/导航拒绝；双击只创建一个维护；默认精确 240 分钟；三 ready 入口；页面内 SSH/Telnet 原生进程、输入与 resize；可选外部入口；目录读取及带换行文件名；离开维护关闭两个句柄；浅/深色和 960px / scale=2 无水平溢出；Exec ACK/最终中文输出；原生导入与保存字节一致；上传、下载 committed/released、显式稳定资产入库；工具/版本/Artifact/compatibility/投放以及任务中心按 Operation 归类；Session replacement、WS 重连、300ms 到期、1000 分钟租期、主动关闭、切换 Server 清空旧状态、重新连接；无 React runtime error。
- WM_CLOSE 验证时特意保持 SSH 进程活动，正常退出后确认其 PID 已不存在。正式 `VerifyUI=false` 发布窗口也在移除 Node/dotnet 的子进程 PATH 下启动并退出 0，使用随包 WinUI/WebView2/VC。最后空态文案修改后再次构建并验证正式发布启动/退出。

### 真正的终端命令往返

`docker build -t router-agent-terminal-test:local -f windows/terminal-test.Dockerfile windows` 后执行 `windows/verify-terminal-services.ps1 -Dotnet ./build/dotnet/dotnet.exe`，通过三项检查：

1. Windows 系统 OpenSSH 经 ConPTY 连接隔离 OpenSSH 服务、交互密码登录并收到真实命令输出。
2. 本地 Resize(100,40) 后远端 `stty size` 返回 `40 100`。
3. Windows Telnet 经 ConPTY 连接隔离 BusyBox Telnet 服务并收到真实命令输出。

日志 `build/ui-freeze-terminal-services.log`。密码为一次性测试值，主机密钥直接取自自身容器，使用测试目录的 known_hosts 并开启严格验证；没有自动接受未知密钥或修改用户 known_hosts。Telnet 测试服务直接启动隔离 shell。容器与测试密码退出时清理，Docker 不是产品依赖。

### Phase 1～5 回归与实际画面

- 隔离 Linux 执行 `tests/verify-phase5.sh release|asan|race` 全部退出 0。Release 包含 C++ CTest 4 项、全量 Go test/vet、Linux Server 构建；ASan/UBSan 包含 CTest 4 项及 Tunnel/Phase5 集成；race 包含 C++ TSan CTest 4 项及全量 Go race。包括实际 Linux Probe 与真实 Web/SSH/Telnet 服务的 Tunnel 测试。
- 容器采用已有 Go 1.26/CMake 构建环境，补齐 OpenSSH、busybox-extras 和 util-linux；运行使用 `--network none --security-opt seccomp=unconfined`，以隔离 80/22/23 并允许 TSan 的 personality 系统调用。三份最终日志为 `build/ui-freeze-linux-release.log`、`build/ui-freeze-linux-asan.log`、`build/ui-freeze-linux-race.log`。初次缺 sshd 和默认 seccomp 限制的环境失败已解决，不以那些失败运行作为通过证据。
- Windows 适用 `go test ./cmd/... ./internal/... ./tests/...`、`go vet` 与 Server build 通过。实际 Linux Probe 集成以 Linux 运行结果为准。
- Chrome 实际检查 2560 宽真实设备概览、1440×900 文件管理、960×850 深色；读取并查看原生维护、任务和工具截图。冻结结构、密度、配色和整体布局保持；修正空资产错误显示“已归档/已校验”为“未选择”。Vite 5173 保持运行，preview.html 为冻结参照，index 为正式数据入口。

### 验收范围

本轮验证分三层：WebView2 → 真实 Go API → Protocol 测试对端；ConPTY → 实际 SSH/Telnet 服务；Linux 真实 Probe → Tunnel → 80/22/23。三层分别有实际运行证据，不合并宣称完成用户实机路由器经 Windows/UI/Tunnel 的登录验收。用户设备凭据、厂商 Web 页面、各目标架构、物理多屏 DPI 和干净目标机仍需对应环境；已有 Sandbox Application Control 限制记录保持。本轮停止等待用户验收，不发布公网 Web、不进入下一阶段。

## Mock 视觉迭代 · 文件管理与工具仓库（2026-09-07）

- 核对 API assets/tools/versions/compatibility/uploads/downloads/deployments/complete/archive 契约、ADR-019 R1～R6、正式 FilesPage/ToolsPage 与 DTO。仅新增独立预览 RepositoryPreview/repository.css 并接入主导航和 #files/#tools；任务/概览/远程维护保持当前样式，不修改 Server、API、Probe、Tunnel 或正式业务页。
- 文件页：资产表格、搜索/分类、归档项开关、大小和引用概况、详情、导入/上传/下载表单及传输记录。示例文件导入只写本地元数据，选择真实文件只使用名称/大小，不读取字节或上传；保存到本地按钮只提示。来源/用途类别为 Mock 候选，不是现有 Asset/Tool 原生字段；引用来自演示版本聚合，大小为资产逻辑总和，不代表去重后的磁盘占用。
- 工具页：卡片目录、版本选择、多架构产物、目标设备、兼容/不兼容/信息不足、创建/发布/归档/投放。发布表单首版只新增单个产物，支持文件、Linux/架构/libc/权限；models/kernels/capabilities、多产物编辑等完整表单尚未设计。兼容函数仅用于本地示例，不替代真实服务端匹配/Session 检查；没有自动升级、latest、安装状态或工具自动执行。
- Chrome 实际查看默认宽屏、1440×900、900×850 的浅色/深色，检查表格、卡片、详情、弹窗与内外滚动；窄窗口上下排列，无页面横向溢出。收紧最小高度、修正传输表列宽、切页回到顶部，复查实际页面。
- 交互验证：从不匹配产物改选兼容产物后投放，生成本地传输记录；离线及缺少受限 libc 信息禁用投放；创建工具和 1.0.0 演示版本通过，已有/已归档标签 4.99.3 被拦截；示例导入加入资产列表；搜索空态/清除；活动版本引用的资产归档按钮禁用。下载 committed+released 的失败任务可演示导入，记录仍显示失败并保留新 asset ID。新传输只在两页共用的本地演示记录中体现，尚未和任务中心示例联动。
- npm run build（TypeScript 与 Vite）通过。未验证真实仓库导入、SHA-256、设备传输、系统保存或后端/Windows 发布，本轮没有此类改动。保持 Vite 与浏览器，未提交 Git，等待用户视觉反馈。

## Mock 视觉迭代 · 任务中心首版（2026-09-07）

- 核对 `docs/API.md`、`frontend/src/models.ts` / 正式 TasksPage、`internal/api/dto.go` / `internal/task/service.go`：任务为 exec/upload/download；工具投放通过关联 Operation 归类，底层仍为 upload。GET tasks 仅摘要；本地列表标题、命令和工具标签是详情/Operation 聚合的视觉候选，未宣称摘要接口已有这些字段。统计为已加载 Mock 数据，不代表服务端聚合统计。
- 新增独立 TasksPreview/tasks.css，并接入主导航及 `#tasks`。支持本地搜索、设备/状态/类型筛选、分页、详情切换、最终 stdout/stderr/退出码/截断标记、新建命令及 cwd/env/超时参数。没有取消、调度、实时输出流或真实请求。
- Chrome 实际查看 2560 宽默认窗口、1440×900、1100×800、900×850；桌面列表/详情伸展，较小窗口内部滚动、低矮窗口可页面滚动，窄窗口上下排布，无页面横向溢出。检查浅色/深色、新建弹窗，修正详情字段受既有样式影响而挤入两列的问题。
- 浏览器验证：搜索空结果/清除、类型筛选、翻页、拒绝状态不标记收到最终 RESULT；环境变量非字符串校验拦截，合法参数创建待确认 Mock 任务；重发保留同一编号/规格，仅增加模拟派发次数；已提交且释放的下载可模拟导入，原任务失败结果保持。
- 文件 committed/released/failed 与任务 RESULT 分开显示；无 RESULT 不显示实时输出或推断成功。任务记录仍按本次服务运行范围表达。前端 `npm run build` 通过；本轮只改预览及进度文档，不运行后端/Windows 发布验收，不代表生产能力已接入。保持 Vite 与任务中心页面，未提交 Git，等待下一轮视觉意见。

## Mock 视觉迭代 · 远程维护铺满高度（2026-09-07）

按最新反馈取消 620px 高度上限。仅维护页启用纵向弹性布局，终端/文件面板占满 Header、维护条和页脚之外的剩余高度；文件列表可在内部滚动，低矮窗口保留最小操作高度。Chrome 实际检查 2560×1283、1440×1000 和 1440×900：前两者页脚底部等于视口底部，大窗口下原有大片空白消失，低矮窗口可滚动；概览不启用该布局。`npm run build` 通过。继续 Mock、不提交、不接 API，保持开发服务。

## Mock 视觉迭代 · 远程维护参考图排版（2026-09-07）

- 仅调整 `MaintenancePreview.tsx` / `maintenance.css`。入口按钮并入深色终端顶部，第二行放连接状态和字号/清屏/设置/展开工具，底部合并命令快捷项、命令选择器与连接信息。文件区采用勾选表格、行预览、每页最多 8 项、上传拖拽区和存储条；传输记录移入文件工具菜单。桌面约 2:1，高度上限 620px。
- Chrome 实际检查 1920×1080、1440×1000（含深色）和 1100×900。首轮后将高度上限从 780px 收至 620px，文件名字号调为 12px；复查页面、终端顶栏/底栏及文件面板无横向溢出，窄窗口上下排列。已恢复默认视口并保留页面。
- 实际操作 `df -h` / `uptime` / `ip addr` / `free -m`，Telnet `pwd` 后切回 SSH 保留原输出；检查本页全选/取消、文件工具中的传输记录、新建会话菜单。新增勾选下载仍仅触发本地 Blob，不代表真实传输。
- `npm run build` 通过。未重试此前受浏览器限制的文件上传/下载落盘校验，相关未验证状态保持；未接 API、未修改生产架构、未提交 Git，等待用户视觉反馈。

## Mock 视觉迭代 · 内置终端与文件面板（2026-09-06）

- 入口 `preview.html#maintenance`，新增 `MaintenancePreview.tsx` / `maintenance.css`。概览保留，页签支持切换，概览 SSH/Telnet 入口导航到内置模拟会话。全为 React 本地状态，不导入生产 Client、Bridge 或任何终端网络库。
- Chrome 实际检查默认宽屏、1680×1050、1440×1000 深色、1100×900；桌面左右布局，窄窗口上下布局。检查页面及两个面板无横向溢出；收细滚动条并采用深色终端滚动条，连接聚焦不再自动滚走设备 Header。
- 实际操作 SSH `ip addr`、Telnet `uptime`、切回 SSH 保留独立输出；检查维护关闭后终端输入移除、连接和上传按钮禁用，重新开启可连接。已查看终端展开与恢复、目录双击进入、目录快捷按钮、文件搜索和文本预览。
- 上传使用浏览器 File 选择器/拖放加入内存目录，下载发起本地 Blob 保存并记录动作。自动选择文件时 Chrome 扩展返回 `Not allowed`（缺少文件 URL 访问权限）；下载点击后的事件等待超时，未确认文件落盘。浏览器下载页被工具 URL 策略禁止访问，没有绕过限制。上述两项不记为端到端通过，拖放也未自动验证。
- `npm run build` 通过（预览 TypeScript + 正式 production build）。未运行真实终端、设备传输、后端或 Windows 发布验收；真实接入仍为 TBD。保持 Vite 和远程维护预览，未提交 Git，等待视觉反馈。

## Mock 视觉迭代 · 概览布局（2026-09-06）

- 用户补充整页参考图，明确将设备图移到右上原配电信息区域、去掉配电信息、中间内容可先提案、拓展快速维护，字段后续再调整。
- 独立预览组件 `OverviewPreview.tsx` / `overview.css`：桌面三列，左网络/基本资料，中连接与维护概况/快速维护，右设备图/移动网络，底部运行指标；1100px 改为两列，更窄窗口单列。中间概况暂用在线时长、心跳、维护连接、今日任务和最近动态，均为待确认 Mock 字段。
- 快速维护为 Web/SSH/Telnet、命令、文件、工具六个带说明主入口，另有网络诊断、日志采集、配置备份三个辅助入口。实际浏览器点击“网络诊断”显示原型提示弹窗，不发起网络请求或设备操作。
- Chrome 实际检查 1680×1050（随后 1051）、1440×1000、1100×900 与深色模式。首轮查看后收紧卡片内部间距与操作入口高度；检查页面宽度及卡片/操作块无横向溢出。较低窗口仍需要纵向滚动查看运行指标。`npm run build` 通过；保持开发服务与预览页运行，未提交。
- 设备素材：`frontend/src/preview/assets/router-device.png`，内置 image_gen 生成的无品牌示意图，不对应真实型号。生成提示词：`Use case: product-mockup. Asset: small device image for a Chinese router administration dashboard. Generate a photorealistic 3D studio product render of a compact dark graphite rectangular industrial wired router, no antennas, metal chassis, ventilation on left side, front panel with six ethernet ports and tiny green status lights. Three-quarter view from above showing top, left side and front, like an enterprise gateway catalog photograph. Device centered, isolated on a very pale cool blue (#edf3fc) seamless backdrop, subtle contact shadow, ample space around all sides, landscape composition 3:2. No text, no logo, no watermark, no cables, no extra objects. This is a generic illustrative device, not a real branded model.`

## Mock 视觉迭代 · 设备列表收紧（2026-09-06）

依据用户新增设备列表截图，将设备项改为名称/ID 两行，去除系统/架构行；在线/离线状态改为右侧垂直居中，所有设备项保留细边框，选中项保持浅蓝背景。每项高度约 57～59px，项间距 6px。Chrome 实际查看默认宽屏与 1100×900 窗口的设备列表，六项无横向溢出、状态与文字无重叠；已恢复默认窗口并保持预览运行。`npm run build` 通过。本轮仅视觉变更，未提交，等待下一轮反馈。

## Mock 视觉迭代 · 首轮（2026-09-06）

- 开发入口：`frontend/preview.html` → `src/preview/WorkbenchPreview.tsx` / `preview.css`，本机 Vite `127.0.0.1:5173`。使用 React 本地演示状态，没有导入 API、Connection、useWorkbench 或平台 Bridge；只用于视觉和交互原型，不进入现有 production 入口。
- Chrome 实际打开页面，查看默认宽屏、1440×1000、1100×900 和 800×850 窗口。首轮修正卡片与 Header 的宽屏对齐、辅助文字字号与颜色；查看浅色/深色、导航收缩、维护弹窗及内容滚动。1440 和 800 宽度 DOM 检查无页面横向溢出，所检查的卡片/入口/设备项无内部横向溢出。
- 实际交互：搜索深圳设备、清空搜索、离线筛选返回 2 台、切换离线设备禁用开启维护；关闭演示维护后入口禁用，再开启 60 分钟租期，显示 01 小时 00 分 00 秒和 60 分钟后到期。倒计时和历史是演示数据，不代表真实业务验证。
- `npm run build` 通过（含预览 TypeScript 检查；Vite production 仍构建正式 index 入口）；`npm test` 既有 9 项测试通过。本轮没有重新执行后端或 Windows 发布验证。
- 当前状态：首轮预览，尚未取得此前参考渲染图，等待用户继续提出视觉修改意见；没有提交或推送。以下为此前正式 React / WebView2 重构验证记录。

日期：2026-09-06。起点为干净 main / origin/main `6f0ce71a5027b57d51e9a6c807794be45f7633b5`，架构依据 Accepted ADR-026。本次 Server、Probe、Go 依赖、公开 API、Protocol 与 Tunnel 生产代码未改动。旧纯 XAML 的 61/98 项历史结果不作为本轮证据。

**React Shared Frontend 是今后 Windows 与 Web 的统一产品 UI 基线。** 本轮交付可复用源码和 Windows 本地发布目录，不部署正式公网 Web。

## 构建与客户端验证

`windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify` 从锁文件恢复前端依赖，执行 TypeScript/Vite production build、Vitest、Windows Release publish、原生策略与真实 WebView2 集成。最终构建通过，0 警告、0 错误；production JS 247.14 kB、CSS 17.60 kB，未压缩原始体积。前端 9 项测试、原生策略/保存 21 项检查、实际 WebView2 26 项集成检查通过。

| 要求 | 本轮实际证据 |
| --- | --- |
| 本地资源 / SPA | 实际 WebView2 加载 production assets；受控 index 加 hash 路由、刷新后恢复；不存在 Vite 开发服务器依赖 |
| Bridge 正常与拒绝 | 实际 getProfile/连接/三入口/Windows 保存；任意 exec、非法 endpoint、脚本设置可执行路径被拒绝；策略覆盖未知方法、重复字段、外部来源/协议、路径穿越、参数注入 |
| Server 连接/切换 | React 经真实 Go HTTP/WebSocket 连接；切换另一 Server 清空旧设备/任务，再切回创建新连接所有者 |
| Device / Session replacement | 真实 Protocol v1 测试对端注册；旧连接下线与新 Session 替换后回查，界面显示新 Session |
| WebSocket 恢复 | 关闭真实 socket，断线期间替换 Session；新 socket 连入后 HTTP 全快照恢复；单元测试覆盖查询中通知不丢失、非法首事件拒绝后重连 |
| Maintenance | 实际双击仅创建一次；默认省略租期后期限精确 240 分钟；自定义 300ms 由 Server 到期、1000 分钟接受；主动关闭与三个 ready 入口 |
| Web / SSH / Telnet | 三按钮启动前回查 API，实际跨 Native Bridge 到测试捕获边界；平台测试验证默认浏览器 URL、系统 SSH 独立参数；生产保留系统客户端/PuTTY 启动实现 |
| Exec / Task | React 提交、真实 API 派发、测试对端 ACK/RESULT、页面显示最终结果；不把 202 作为任务成功 |
| File | 浏览器 File API 打开 Windows 选择器导入真实文件；Save Picker 选路径并逐字节核对输出；设备 upload 成功、download committed/released、显式 complete 后稳定资产 |
| Tool | 创建工具、版本/Artifact、查询 Server compatibility、显式投放并完成真实 API 流程 |
| 幂等 / 重复点击 | 在途写入单次准入；不确定响应保留相同键和相同 body 字节、拒绝新请求，显式重试；业务错误清除不确定状态；真实维护双击检查 |
| 网络 / 业务错误 | Vitest 分离 transport/解析不确定与结构化 API 错误；实际 WS 断线/下线/切换恢复；错误显示于页面及当前表单 |
| Light / Dark | 实际 WebView2 切换主题并截图；系统主题使用 matchMedia，本地配置保存偏好 |
| DPI / 窗口布局 | 实际 WebView2 的 960px、deviceScaleFactor=2 CDP 布局验证，无页面水平溢出；浅色/深色/任务/工具截图复核 |
| 退出 / 资源释放 | 单元测试取消阻塞 HTTP、join socket、dispose 幂等且不迟到发布；原生 WM_CLOSE 等待 JS shutdown 后退出 0；保存临时文件失败时清理且保留原目标 |
| 发布目录 | .NET/WinUI 自包含、Frontend 静态资源、Fixed Version WebView2、app-local VC DLL、可选 VC 离线安装器；正式非测试 EXE 本机启动/退出 |

集成运行日志：`build/react-shell-delivery.log`。最终 UI 截图与 fixture 日志：`build/react-shell/verification-20260906-203637/`。每次复现生成新的时间目录，构建产物与日志不提交 Git。`VerifyUI=true` 才包含 CDP 19222、测试配置入口与外部启动捕获；正式发布禁用 DevTools，不含测试 Probe。

测试对端只实现 Protocol v1 以验证 UI/API 闭环，不冒充实际 Linux Probe；实际 C++ Probe 及 Web/SSH/Telnet 通道见下面全量回归。外部客户端按钮检查不等于用户 SSH/PuTTY 登录、主机密钥或厂商设备网页验收。DPI 使用 WebView2 CDP emulation，不声称完成物理显示器 200% 或跨屏拖动矩阵。

## 本机与干净环境的验收范围

正式发布为 `build/windows-react/win-x64/`，Node/npm 仅用于构建。发布程序只加载随包前端；随包固定 Runtime 的 Microsoft 签名和 VC 安装器签名均有效。本机将完整包复制到 `build/react-shell/portable-20260906-204100/`，子进程 PATH 只含 Windows/System32，实际窗口正常显示 React 工作台；检查 .NET、WinUI 与 WebView2 进程均来自该副本，正常关闭 exit=0。证据为 `build/react-shell-portable.log` 与 `build/react-shell/portable-production.png`。这不卸载本机开发工具或改变系统 PATH。

正式 WinUI WebView2 的 UI Automation 查询在本机返回空节点；该尝试没有作为 React 加载断言。页面内容由 computer-use 实际窗口截图确认；自动 `verify-published.ps1` 检查原生窗口、随包进程/运行库及退出，最终通过，日志为 `build/react-shell-published-final.log`。React DOM 自动断言由前述实际 WebView2 集成测试承担。

实际尝试了网络关闭的 Windows Sandbox：确认没有 Node/dotnet 命令，发布目录只读映射后完整复制，离线 VC 运行准备成功；但 Windows Application Control 拒绝启动未签名 `RouterWorkbench.exe`，错误为 “An Application Control policy has blocked this file”。证据在 `build/react-shell/sandbox-20260906-203148/failure.txt`。未绕过或修改该策略，**干净 Windows 运行未验证通过**。

用户了解阻断后最终明确要求“直接在本机测试”，因此本轮按更新后的本机范围交付。不能把无开发工具 PATH、本地发布副本运行或沙盒准备成功描述为干净目标机独立运行成功。

## Phase 1～5 与 Tunnel 全量回归

本轮重新执行全部既有回归，不复用此前阶段结果。Windows 使用 Go 1.25.5、.NET SDK 10.0.400、Windows 11 build 26200；Linux 使用 WSL 隔离 Alpine、Go 1.26.3、GCC 15.2.0、CMake 4.2.3 与独立网络/mount namespace、devpts。

| 验证 | 本轮结果 |
| --- | --- |
| Linux Release C++11 / CTest | 4/4，4.72 秒 |
| Linux Phase 1～5 全量 Go + 真实 Probe | 全包通过，集成 170.462 秒 |
| C++ ASan / UBSan / LSan CTest | 4/4，6.02 秒，无报告 |
| ASan 真实 Probe Phase 4/5 | 通过，15.570 秒 |
| C++ TSan CTest | 4/4，6.96 秒，无报告 |
| Go race 全量 + TSan 真实 Probe | 全包通过，集成 177.110 秒，无报告 |
| Windows Go 全部源码包测试 | 全部通过，集成 0.144 秒；Linux 专用用例在 Linux 执行 |
| Windows / Linux Go vet 与 Server 构建 | 通过 |
| go mod verify / git diff --check | 通过 |

日志为 `build/react-shell-linux-release.log`、`build/react-shell-linux-asan.log`、`build/react-shell-linux-race.log`、`build/react-shell-windows-go.log`。后端生产差异为空。

## 复现

Windows 仓库根目录：

```powershell
./windows/build.ps1 -Verify
# 使用本轮独立 SDK：
./windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify
./windows/verify-published.ps1 -Executable ./build/windows-react/win-x64/RouterWorkbench.exe
./windows/verify-sandbox.ps1
go test ./cmd/... ./internal/... ./tests/... -count=1
go vet ./cmd/... ./internal/... ./tests/...
go mod verify
git diff --check
```

原生测试使用 loopback HTTP 18080、Probe 控制 18081、测试控制 18082、切换用 18083、CDP 19222、维护池 32200～32299，需空闲。正式程序没有这些测试入口。

Linux 隔离环境：

```sh
/bin/sh tests/verify-phase5.sh release
/bin/sh tests/verify-phase5.sh asan
/bin/sh tests/verify-phase5.sh race
```

需要 go.mod 锁定缓存、OpenSSH、busybox-extras 和 devpts；真实服务绑定 80/22/23，必须在隔离网络运行。脚本名称沿用 Phase 5，执行的仍是本轮源码。

## 已知限制与交付

- Windows x64 目录发布；干净目标机、Windows ARM64/x86、多物理显示器及所有旧 Windows 版本未完成环境矩阵。未签名应用可能被组织 Application Control 拒绝。
- 固定 WebView2/VC 版本需发布者随版本维护；尚未构建升级系统。
- 仅 Repository 持久化；WebSocket 不重放、无实时 stdout，Server 重启后幂等不保证；倒计时受本机时钟影响，API 为事实来源。
- Probe mipsel/ARM/ARM64、uClibc/老内核实机矩阵未执行，不从 Linux x86_64 推断通过。
- 正式公网 Web、认证/TLS/RBAC、微信、MCP、AI、自研 SSH/Telnet、通用转发及新数据面未实现。

本轮独立提交标题为 `refactor: adopt shared React frontend and Windows WebView2 shell`。最终 SHA 与 main 推送状态由实际 Git 记录及交付回复提供。按用户最终指定的本机范围完成验证后推送，停止等待验收。
