# 变更记录

本文件记录：

1. 已形成的用户可见产品行为变化。
2. 对客户端或开发者具有外部意义的协议或 API 契约变化。
3. 重要项目基线或治理规则变化。

本文件不记录普通内部重构、未完成计划、虚构功能或单纯开发过程流水账。

## Unreleased

### Changed

- Phase 6 产品 UI 正式迁移为共享 React / TypeScript 工作台，WinUI 3 作为 WebView2 薄 Shell；Windows 与未来 Web 共用页面、设计系统、HTTP/WebSocket 与状态管理，依据 ADR-026。
- 按确认截图实现一级导航、设备列表、维护主卡片和中文浅色/深色界面；保留 Exec、文件、工具版本/产物/兼容/投放与全部既有 API 语义。移除旧 XAML 业务页面和 C# 业务网络层。
- Windows Release 携带 production 静态资源、固定版本 WebView2、app-local VC DLL 和可选 VC++ 离线安装器；目标机不需要 Node/Vite。Native Bridge 限于明确平台能力并验证来源及输入，外部设备网页由系统浏览器打开。

- Phase 6 Windows界面以C# + WinUI 3正式替换WinForms：中文Fluent工作台、设备侧栏、突出远程维护卡片，浅色/深色/系统主题、Mica与自适应布局；服务器地址移入设置，维护/会话编号及释放原因移入详情。
- 命令、文件和工具使用列表与详情，原生弹窗和文件选择器支持桌面流程；错误主摘要中文化，原始API诊断放入详情。HTTP/WebSocket、幂等和系统浏览器/SSH/Telnet启动策略继续复用，后端契约不变。
- Windows发布改为包含.NET与所需WinUI组件的x64目录包，替代旧单文件包；需要保留整个目录及目标机Visual C++运行库。架构选择按ADR-025取代ADR-024的界面/部署部分。

### Fixed

- Phase 4：维护端口释放后默认隔离24小时，独立于关闭历史；新增 `ReusableAfter` 和 `-tunnel-port-reuse-delay`，池满拒绝提前复用。有限隔离不提供超窗或重启后的永久旧地址隔离，部署边界见ADR-022。
- `CloseMaintenance` 本地释放不再等待Probe控制发送；Gateway采用有界控制队列，撤销以data reset终止活动流，Probe在half-close后仍检测reset并回收空闲worker。
- `-tunnel-data-host`支持Server侧域名解析；每次维护固定解析所得IP，Probe保持数值IP握手。
- Probe默认Tunnel并发由64降至8、允许1～64；Server每维护/设备默认均为8。默认idle由5分钟改为24小时，并按整条连接进展计算；绝对维护租期保持默认240分钟。
- 当前Phase 4文档按ADR-022同步，FRP历史归档为仓库内摘要，接管不依赖本地临时Git保存项。

- Phase 1E：Server 严格拒绝非法 Unicode 与字符串形式 load1；Probe 忽略未知扩展字段中的合法大整数，已知 uint64 字段仍拒绝溢出。
- Probe 在 REGISTER_ACK 缩小帧上限后立即复核已缓冲 Header，防止等待超限 Payload；未确认心跳记录设为每连接最多 1024 项，满后沿用既有重连流程。
- Server 文件 worker 结束时释放残留文件块邮箱；入站 message_id 耗尽时结束连接，避免回绕为 0。
- Server 成功写出 REGISTER_ACK 后才发布可派发会话，修复注册/重连期间 TASK 抢先发送的问题。
- Server 传输写失败立即关闭并废弃连接，防止部分帧后续写入与 message_id 重用；CreateExec 对发送结果不确定的任务保留记录并返回 task_id 与 ErrDispatchUncertain。
- Probe exec 不再继承控制 socket；fd 的 close-on-exec 设置与 fork 使用统一同步。
- Probe timeout/worker 停止完整执行进程组 TERM、200 ms grace、KILL；后代持有 pipe 时有界排空并设置 truncated，防止 worker 因 EOF 等待挂住。

### Added

- Phase 6首版Windows远程维护工作台（f8d099d6，界面已由上述WinUI重构取代）：C# / .NET 10 LTS Windows Forms、自包含x64发布；Server设置、设备列表/Session、默认240分钟或正自定义租期Maintenance、三入口和主动关闭、Exec结果及文件/工具/版本/兼容产物/投放。
- Windows UI仅调用现有HTTP/WebSocket；首连/重连HTTP同步、状态失效刷新、响应不确定保留原幂等请求、重复点击防护、切换Server与退出异步释放资源。Web交系统浏览器，SSH/Telnet使用系统客户端或用户指定PuTTY，不缓存密码或Tunnel私有身份。Phase 5已验收，首版按ADR-024进入Windows UI交付，Server/Probe业务契约保持。

- Phase 5统一 `/api/v1` HTTP API：Device/Session、exec/Task及结果/原身份重发、文件资产与上传/下载导入、Tool/版本/Artifact/兼容/投放、Maintenance创建/查询/关闭和三入口。
- 独立HTTP监听默认127.0.0.1:8080；一致JSON/错误/分页、异步202及有界Idempotency-Key账本。HTTP取消不撤销成功创建的长期业务对象，不确定派发保留task_id；默认4096项不淘汰账本，满后拒绝新键。
- WebSocket `/api/v1/events`提供devices/tasks/files/maintenance状态变更通知，首连/重连重新同步；固定容量、多客户端隔离、慢消费者断开和Server关闭回收。files包括Repository目录变更。默认可信部署边界、容量和后续认证/TLS/RBAC设计点见ADR-023/API.md。
- HTTP隐藏Server本地路径与Tunnel数据面私有细节。复用Phase 4 Service及240分钟默认租约、正自定义租约和Session绑定；Probe协议与Tunnel数据面保持。

- Phase 4 新增极简自研TCP Maintenance Service：一次创建Web/SSH/Telnet临时入口，固定访问设备127.0.0.1:80/22/23；默认240分钟并允许自定义租期。独立data TCP与一次性token配对，保持原始字节、并发连接、half-close和有界背压。
- Maintenance严格绑定创建时Device Session，主动关闭、到期、替换或失联撤销入口并终止活动连接；等待Server相关资源释放后才归还端口。Server提供端口池、advertised地址、超时与分层连接限额配置，Probe保持C++11且没有新增重型依赖。
- 新增控制类型0x40～0x42与RMT1独立data握手；以Accepted ADR-021取代未交付FRP/xfrpc路线。不引入HTTP/WebSocket API或通用端口映射。

- Phase 3 新增持久化 File / Tool Repository 与内部资产查询、导入、归档、工具版本/产物发布和兼容查询。tool_id、asset_id、artifact_id 为稳定 UUID，artifact_id 全仓库唯一；版本不可变，重复内容共享 SHA-256 blob，归档保留身份与文件。
- 新增内部工具投放、资产上传和设备下载显式导入，复用既有文件任务；受限资料缺失返回 unknown，不自动投放或执行工具。保留不确定派发的任务身份，下载可分别呈现本地完整提交和最终失败结果。
- Server 支持 `-repository-dir`，默认 `./data/repository`，采用单进程目录锁与本地 JSON 元数据；Linux/Windows 无新增外部服务依赖。仓库数据跨重启保留，设备/任务/Session 不恢复。

- Phase 2 新增 Device Service / Inventory 内部 List/Get/Sessions 查询，保留 REGISTER 基础资料和 capabilities、设备在线状态、当前/最近 Session 及上线/离线/活动时间。
- 设备重连与在线 Session 替换保留稳定 device_id；替换更新 LastOnlineAt，不更新 LastOfflineAt。默认每设备保留最近 64 个已结束 Session，查询包含历史截断计数；仅进程内存储，历史淘汰不影响 Task/File 幂等与结果。

- Phase 1D 实现 upload/download、FILE_BEGIN/ACK/CHUNK/END，原始二进制分块、size/SHA-256 流式校验、同目录临时文件验证后发布和 upload.mode。
- 文件任务使用一个 active 与有界 FIFO，支持同 task_id 幂等、断线失败结果补报；内部 CreateUpload/CreateDownload 与 FileSnapshot 暴露文件能力和下载提交事实。
- 文件与控制发送在帧边界按优先级串行化，并限制内核发送缓冲目标；持续文件传输期间可处理心跳和并发 exec。

- 实现 Phase 1C：Probe 默认 4 workers 并发 exec、乱序结果关联、进程生命周期 task_id 去重；TCP 断开时已接受任务继续执行，新会话补报缓存结果。
- Server 增加内部 ResendTask，使用原 task_id 与规格重发，按 session_id/message_id/task_id 保存每次派发关联。
- 增加实际并发执行、重复任务、参数冲突、容量、断线继续执行和中继丢弃 ACK/RESULT 后补报的自动化覆盖。

- 实现 Phase 1B TASK、TASK_ACK、TASK_RESULT 与 exec 完整闭环，Management Server 可通过内部 Task Service 对在线 Probe 创建任务并等待结果。
- Probe 增加 HEARTBEAT_ACK / TASK 在线消息路由、单 worker 串行任务执行，以及 RECEIVED / QUEUED / RUNNING / SUCCESS / FAILED / TIMEOUT 基础状态流转。
- exec 支持 `/bin/sh -c`、cwd、env、独立 stdout/stderr、1 MiB 单流捕获上限、控制帧大小适配与 timeout 进程组清理。
- 增加 Phase 1B 协议、任务状态、真实进程 exec、超时、心跳不中断、unsupported task 和大输出自动化测试。
- 实现 Phase 1A 的 Go TCP Server 与 C++11 Probe 可运行闭环，包括 REGISTER、REGISTER_ACK、HEARTBEAT、HEARTBEAT_ACK、失联判断和基础退避重连。
- 实现 Protocol v1 20-byte Header、Big Endian 编解码和可处理 TCP 粘包/拆包的流式 framing。
- 增加协议、注册/心跳和真实 Probe 断线重连自动化测试。
- 初始化路由器远程运维平台的项目设计基线。
- 建立 README、AI Agent 开发规则、架构、协议、API、路线图、项目状态、接管手册和架构决策文档。
- 将 v0.2 Word 设计输入中的 TCP 控制协议整理为仓库内可持续维护的 Markdown 基线。
- 建立文档驱动的项目状态与交接治理规则。

### Changed

- Phase 1E 完成 Phase 1A～1D 整体收口验收，Phase 1 标记为完成；验收映射与完整回归、sanitizer、Windows 适用结果记录于 docs/PHASE1_VERIFICATION.md。独立 Verification commit 交付后停止等待验收，Phase 2 尚未开始。
- 按用户确认冻结 Phase 1C 互操作契约（ADR-015）：重复任务返回 queued/running/完成态 ACK，完成态随后返回原 RESULT；内容冲突使用 ERROR/INVALID_PAYLOAD，保留原任务。
- Server 接受缺 ACK 的已派发任务结果，重复 RESULT 幂等，冲突不覆盖终态；Probe 缓存有界且不淘汰已接受身份与结果，容量满后拒绝新任务。
- 更新阶段授权边界：Phase 1C 已独立提交推送，用户现已授权 Phase 1D；确认有界文件 FIFO、流式校验、同 task_id 不重复文件副作用和中断重传必须使用新 task_id/transfer_id（ADR-016）。P1-P5 已确认并写入 Accepted ADR-017：ready 省略 sha256_ok，done=true，failed=false；顺序 offset、失败收敛和最终发布/确认丢失语义为正式契约。Phase 1D 交付后停止，不进入 Phase 1E。

- Server 每条 Probe 连接的 HEARTBEAT_ACK、TASK 和 ERROR 统一串行写入，并共用单连接 Server 发送方向的 message_id 序列。
- Probe REGISTER capabilities 现在声明 `exec`；TASK_ACK 使用 RESPONSE/reply_to，TASK_RESULT 保持异步 task_id 关联且不设置 RESPONSE。
- Phase 0 形成 Git baseline `bc8d747dfc41a375c31698073005857c238ede51`，Phase 1A 完成 Linux x86_64 首轮验证。
- 明确运行事实、规范性设计、当前状态、接管入口、计划和历史设计输入的事实来源层级。
- 完成 Protocol v1 的 message_id、response correlation、flags、UUID、任务拒绝、文件传输、JSON、boot_id、幂等范围和心跳超时细化。
- 将 Phase 1 拆分为 Phase 1A 至 Phase 1E 可验证里程碑，并增加 Probe 技术栈前置决策门槛。
- 明确 Phase 0 需要用户确认和 Git baseline commit 后才能关闭。
- 确定 Probe Phase 1 技术栈为 C++11 + CMake，首轮 Linux x86_64 验证，后续使用 toolchain files 交叉编译。
- 冻结 Phase 1A REGISTER / REGISTER_ACK / HEARTBEAT / HEARTBEAT_ACK 字段契约、注册失败响应和 10-300 秒心跳范围。
