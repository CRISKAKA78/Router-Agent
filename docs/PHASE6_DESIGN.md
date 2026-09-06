# Phase 6 Shared Frontend / Windows Shell 设计

2026-09-06。起点为干净的 main / origin/main `6f0ce71a5027b57d51e9a6c807794be45f7633b5`。用户明确授权本次技术调整，Accepted ADR-026 supersedes ADR-025 的产品 UI、C# 网络层与发布选择；原 ADR 保留为历史。

**React Shared Frontend 是今后 Windows 与 Web 的统一产品 UI 基线。** 本轮交付 Windows 本地应用与可复用 React 源码，不发布公网 Web 服务。

## 模块职责

| 目录 / 模块 | 职责 |
| --- | --- |
| `frontend/src/App.tsx` / `ui/` / `pages/` | 冻结工作台布局、页面导航、Device/Maintenance/Task/File/Tool；pages 保留设置与详情组件 |
| `frontend/src/useWorkbench.ts` | 选择、用户动作、详情查询与连接生命周期的 React 编排 |
| `frontend/src/ui/*.css` / `components.tsx` | 复用已确认 preview 样式与必要业务表单；不再加载旧 style.css |
| `frontend/src/api.ts` / `models.ts` | 唯一业务 HTTP Client、公开 DTO、错误映射、分页、固定请求字节与幂等键、资产完整性校验 |
| `frontend/src/connection.ts` / `useQuery.ts` | WebSocket、HTTP 全快照、恢复刷新、查询合并、在途资源与选择代次隔离 |
| `frontend/src/platform.ts` | 平台抽象：配置、连接准备、外部入口、文件保存、内置终端句柄操作 |
| `windows/RouterWorkbench` | WinUI 窗口、WebView2 生命周期、本地资源、原生保存与客户端选择器 |
| `windows/RouterWorkbench.Core` | 配置、严格 Bridge 策略、独立参数外部启动、ConPTY 与有界终端句柄；没有业务 HTTP/WS Client |

旧 WinUI 业务 XAML、WorkbenchViewModel、C# API Client 与 WorkspaceConnection 已删除。只有测试专用 TestProbe 包含 Protocol v1 对端代码，不进入产品。

## 本地内容与同源 API

现有 Phase 5 API 拒绝跨 Host Origin，不启用 CORS。本次不修改 Server，也不通过 Native Bridge 代理业务 HTTP。

Shell 把选定 Server 的 `/__workbench/index.html` 及 `/__workbench/assets/*` 请求拦截为随包静态文件。页面具有选定 Server 的 origin，TypeScript 直接 fetch `/api/v1` 并连接同源 WebSocket。拦截始终先于网络，Server 无需实际提供 `/__workbench`。离线也能显示设置与 UI。只允许本地入口文档及 hash 导航；所有脚本、样式来自本地发布目录，网络只允许 API Fetch/XHR，拒绝其他文档、frame、worker、弹出窗口和权限请求。CSP 禁止外部脚本、iframe、对象与表单导航。

路径解码后检查目录包含关系及反斜杠、冒号、点目录。Bridge 每次核对消息 Source 是当前 origin 的受控 index 文档；换 Server 后旧代次不能收到回包。API 响应从不作为 HTML 插入，React 使用文本转义。外部 Web 维护页必须由系统浏览器打开。

开发浏览器通过 Vite 同源 `/api` 代理访问 `RMP_SERVER`；开发代理移除 Origin 后使用既有非浏览器 API 入口，限本机开发监听。未来正式 Browser 部署应把静态前端和 API 放在同源反向代理下，部署与认证尚未实施。本次未关闭 WebView2 Web Security。

## 最小 Native Bridge

消息为 `{id,method,args}`，响应为 `{id,result}` 或 `{id,error}`。只接受数字请求编号、有限长度和深度、已知且不重复的字段、明确的方法与参数。配置/选择器/启动串行准入；终端读写/尺寸/关闭由独立句柄注册表串行管理，不因选择器阻塞终端输出。

- `getProfile`：读取非敏感配置。
- `saveProfile`：地址、外观、SSH 用户名；前端不能修改可执行路径或客户端类型。
- `connect`：切换到已保存的 Server origin；只改变本地页面加载上下文。
- `chooseClient`：Windows 原生文件选择器，只接受对应 ssh.exe / telnet.exe 或 putty.exe。
- `openEndpoint`：只接受 ready Web/SSH/Telnet、合法主机和端口，Web URL 必须与主机端口一致。启动既有客户端，ArgumentList 分离参数，禁止任意 shell、任意 Process.Start 接口。
- `terminalOpen`：结构化 SSH/Telnet 入口、expires_at、尺寸；按已授权客户端配置创建 ConPTY，不接受命令、程序路径或附加参数。
- `terminalRead/Write/Resize/Close/CloseAll`：仅本窗口的不透明句柄。最多 4 个会话，输入 16×16 KiB、输出 32×4 KiB，回读最多 32 KiB，尺寸 2～500 列、2～300 行。原生周期检查到期并释放，即使渲染器暂停也不保留过期进程。

文件选择通过浏览器 File API 触发原生选择器，只返回用户选中文件的 File 对象。Windows 保存使用 `beginSave(name,size,sha256)` 打开原生 Save Picker，只返回一次性不透明句柄；`saveChunk(handle,offset,base64)` 仅允许连续且至多 64 KiB 的分块，`finishSave` 复核长度/摘要并原子发布，`cancelSave` 删除临时文件。单次最多 1 GiB、最多一个保存会话，退出释放句柄，保存中拒绝页面导航。前端永远不能指定或取得本地目标路径；没有任意路径读写 Bridge。普通浏览器平台使用已校验的 Blob 下载。

## UI 与语义

### UI Freeze 与默认内置 Shell（2026-09-07）

用户确认实际页面视觉完成，并要求默认内置 Shell、允许外部客户端。Accepted ADR-027 取代 ADR-026 的仅外部启动限制，冻结清单见 UI_FREEZE。preview.html 与 preview/ 留作视觉参照；正式 App/ui 复用布局并接入唯一 API/WS 层，不使用 Mock 回退。

React xterm.js + Windows ConPTY 承载 ssh.exe / telnet.exe，连接当前 Maintenance 公共入口。SSH 禁止本地命令与转发，不读用户 ssh_config；主机密钥仍由 SSH 原生机制核验，用户名来自配置，密码在客户端提示内输入，不保存密码、终端录制或 Tunnel 私有字段。GUI PuTTY 仅用于外部入口；缺失本机命令行客户端显示依赖错误。“会话已打开”只说明本机进程启动，不宣称远端认证成功。

SSH/Telnet 各自保留本页会话；切页、切设备、失去同步、Session replacement、维护关闭/到期与退出关闭内置进程和管道。自然退出先排空尾部输出再报告结束；积压输出与退出均有界。外部程序独立于工作台生命周期。

文件面板显式刷新或切目录时，创建最多 250 项、使用 NUL 分隔的只读 Exec；路径按 shell 参数引用，截断/失败明确显示，切页取消查询等待。上传先校验入库再创建 uploads，下载创建 downloads 并在任务/传输记录中显式 complete；内容不经终端管道。API 未提供的遥测字段保留位置并标识未提供。

### 正式客户端语义

用户提供的现代工作台截图为视觉基准：一级导航、设备搜索列表、设备标题和页签、维护主卡片、设备状态、快捷操作与维护历史。浅色为基准，蓝色 Accent，统一圆角和细边框，支持深色和系统主题。中等宽度导航收为图标，维护卡片保持完整；窄内容滚动。Session、Maintenance ID、Released 与原因进入详情。

维护默认省略 lease_ms，精确为 240 分钟；自定义分钟用十进制整数换算为整数毫秒，范围 1～9223372036854，无 360 分钟上限。倒计时仅为显示估计，Server 决定状态。入口启动前重新 GET Maintenance 与设备当前 Session。

Exec 保留 cwd/env/timeout、ACK 状态与最终 RESULT；不把 202 当成功。文件 committed/released/failed 与最终 Task RESULT 独立显示，显式 complete 保持稳定 asset_id；cleanup 与 archive 使用原 API。工具使用 Server compatibility，不猜 latest、不重算兼容、不自动执行投放产物。

## 连接、并发与退出

一个 Connection 拥有 AbortController、所有请求、一个 WebSocket、一个快照 worker 和 5 秒恢复计时器。首次 resync_required 和重连触发 HTTP 全分页快照。通知在查询期间到达时保留 dirty 标记并再查一次；只有当前 socket 代次可发布同步快照。重连退避 1/2/4/8/16/30 秒。详情查询合并为单 worker，并按连接与选择版本拒绝迟到结果。

写操作单次准入，同操作短间隔去抖；网络/解析失败保留原 Mutation 的键和字节，暂停新写入。用户确认 Server 未重启后才重试原请求；业务 API 错误与网络不确定分开。Server 重启后的幂等仍不保证。切换先取消并 await 旧请求/socket，清空旧快照。退出时原生层先关闭内置终端，再通过 Runtime.evaluate await JS shutdown(true)，避免等待已经停止准入的 Bridge，最后关闭 WebView2。外部程序与 Server 已创建的任务/维护由各自生命周期管理。

## 发布与边界

.NET 10 / WinUI 模块化自包含 x64 目录 + Vite production 静态资源 + Fixed Version WebView2 Runtime 152.0.4191.62；固定运行包中的 VC DLL 同时按 app-local 方式放在 EXE 旁，不要求目标机安装开发运行环境或提权。另携带离线 Visual C++ x64 安装包作为部署选项。Node/npm 只用于构建。Fixed Runtime 与 app-local VC DLL 由发布者随版本更新，不构建自动升级系统。

保留 ADR-009～023 的全部业务与协议边界。认证、TLS、RBAC、租户、完整审计、跨 Server 重启恢复仍待后续正式设计；不进入 MCP、AI、微信、新 Tunnel 或通用端口转发。实际验证见 PHASE6_VERIFICATION。
