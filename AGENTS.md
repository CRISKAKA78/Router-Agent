# AI Agent 与开发者工作规则

本文件适用于仓库中的所有 AI Agent、Codex 会话和人工开发任务。仓库文件是项目连续性的事实来源，不得依赖聊天上下文、历史会话、AI 记忆或猜测代替仓库核对。用户当前明确指令决定本次任务范围；已确认的新授权应同步到仓库，不能让历史阶段停止语句阻止当前已授权工作。

用户只需描述想增加的功能、想修改的行为、待解决的问题或最终体验，不必提供技术方案、文件清单、测试命令或重复粘贴本文件。Agent 负责依据仓库完成需求到实现、验证与必要文档的闭环。具体流程、模块导航、验证选择与示例见 [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)。

## 开始任务前的必读顺序

任何任务开始前必须依次阅读：

1. AGENTS.md
2. docs/HANDOFF.md
3. docs/PROJECT_STATUS.md
4. docs/ARCHITECTURE.md
5. docs/ROADMAP.md
6. 当前任务相关的 docs/PROTOCOL.md、docs/API.md 和 docs/DECISIONS.md

随后阅读 docs/DEVELOPMENT.md，并检查 Git 状态、任务涉及的实际文件、代码、构建入口与测试证据。UI 任务补读 docs/UI_FREEZE.md、docs/PHASE6_DESIGN.md。按影响范围深入相关章节，不要求每次重读所有历史验证记录或重跑全量构建；历史测试通过不等于本次验证通过。

## 项目事实来源与文档优先级

按以下层级理解仓库信息。不同层级承担不同职责，不能用低层级材料覆盖高层级事实或规范：

1. 运行事实：实际代码、自动化测试结果、实际构建结果和实际运行结果。
2. 规范性设计：docs/DECISIONS.md、docs/PROTOCOL.md、docs/ARCHITECTURE.md 和 docs/API.md。
3. 当前状态：docs/PROJECT_STATUS.md。
4. 接管入口：docs/HANDOFF.md。
5. 计划：docs/ROADMAP.md。
6. 历史设计输入：路由器探针_TCP长连接控制协议设计_v0.2.docx。

Phase 0 完成后，仓库内 Markdown 文档成为项目持续维护的当前设计基线。Word v0.2 继续保留为 Phase 0 原始设计输入和历史参考，但不得覆盖后续经过正式确认并写入 Markdown 或 ADR 的设计变化。

文档之间发生冲突时，不得自行选择、猜测或静默修正。必须先判断冲突属于：

- 实现偏离设计。
- 状态文档过期。
- 两份规范性设计文档不一致。
- 已接受设计需要正式变更。

确定冲突类别后，修正对应事实或发起设计变更。需要改变 Accepted ADR 时，新增 superseding ADR，说明被取代的 ADR 和影响，不得静默改写历史决定。

实现偏离明确设计时，在当前需求范围内修复并补充回归；状态文档过期时，按可验证事实直接同步。两份规范冲突且没有明确取代关系，或需要改变已接受的协议/架构基线时，先记录问题、原因、影响和建议方案，等待明确确认后实施依赖该决定的部分。尚未决定的内容标记为 Proposed、TBD 或待讨论，不得推断为最终设计。历史 TBD 已被后续 Accepted ADR 明确解决时，按该决定处理，不要求用户重新确认。

## 开发边界

- Management Server 优先使用 Go，并保持 Linux 与 Windows 跨平台运行能力；不得让基础运行强制依赖一组微服务、外部数据库或消息队列。
- Probe 是独立的轻量程序，面向 BusyBox、uClibc、mipsel、ARM、ARM64 和较老 Linux 内核；不得把具体 AI 排障逻辑放入 Probe。
- 项目采用 API First。Web、微信小程序、Windows UI、CLI、MCP 和 AI Agent 都是 Client 或 Adapter，不得复制核心设备控制逻辑。
- HTTP、WebSocket 等 Adapter 只能通过 Application 或 Service Layer 使用核心能力，不得直接操作 Probe TCP 连接注册表、存储表或内部 Manager。
- Probe TCP Gateway 负责连接与协议；Device、Task、File、Tool、Tunnel 等服务负责业务语义。
- TCP 控制链路不得承载 SSH、Telnet 或 Web Tunnel 的持续交互数据流。
- 每个阶段优先完成可构建、可运行、可测试的闭环。不得同时铺开多个无法验证的阶段。
- 不得通过占位代码、空类、空接口或大量未使用目录假装功能完成。

## 当前产品方向与阶段边界

2026-09-07 用户明确要求继续完善当前 Router-Agent 产品，暂不进入后续阶段（ADR-028）。Phase 0～5 已验收；Phase 6 已有实现与验证，用户实机与最终产品验收状态以 PROJECT_STATUS 为准。当前在既有产品基线上持续迭代，不新建阶段，不把本次治理改造当作 Phase 6 产品验收。

- 当前产品需求可涉及新版 C# / WPF 客户端、C# / Blazor 模板生成器、Management Server、Probe、设备、维护、任务、文件与工具；Agent 按真实调用链判断必要改动，不因文件属于早期 Phase 而拒绝修复，也不借普通功能重构整个 Server。
- 暂缓 Phase 7 MCP、Phase 8 AI Agent、微信小程序、正式公网 Web 部署、新 Tunnel 数据面及其他大规模架构扩展。路线图或长期架构中提到这些方向不构成实施授权。Blazor 生成器的本机浏览器访问及设备 Web 维护入口不等于正式公网 Web 部署；旧 React UI 已按 ADR-037 移除。
- 认证、TLS、RBAC、租户、完整审计、跨进程任务恢复等未决设计继续保持未决；通用端口转发、任意目标端口、自研 SSH/Telnet 不纳入普通功能的隐含范围。
- 本次 Agent Governance / Repository Guidance 改造仅修改治理与必要状态文档，不修改产品业务逻辑，也不实现上述暂缓能力。此限制针对本次交付，不阻止以后用户明确提出的范围内产品需求。

## 持续适用的已确认约束

- ADR-048：工作区名称为设备详情、远程维护、文件管理、配置管理、仓库工具。接口状态内并排外壳端口/系统端口，末尾接口采样时间只打开配置弹窗；所选设备出口IP只显示Server的source_ip，运营商及归属地异步解析该IP。局部取代ADR-045/047对应摘要与导航，公开契约保持。
- ADR-037：当前只保留 .NET 10 / WPF 主 UI 和 .NET 10 / Blazor / Fluent UI 探针模板生成器。React 前端、WinUI/Win32/WebView2 旧宿主及专属构建、测试、Bridge/ConPTY 已移除，不恢复旧 UI。当前启动入口为 `ui-windows.cmd` 与 `template-generator.cmd`，构建与验证见 docs/DEVELOPMENT.md；历史 ADR/验收文档不作为现存代码导航。运行数据和既有发布包不纳入源码清理或 Git 提交。
- ADR-036/047：服务器配置统一顶部设置弹窗，启动自动连接上次保存地址；维护展示 Web/SSH/Telnet 公共链接并打开外部客户端，无默认内置 Shell 或通用任务入口。SSH 新配置默认 admin/admin，可修改，密码以 Windows 当前用户 DPAPI 加密保存、显式复制给外部客户端；不自动登录或接收主机密钥。文件页负责设备目录与本地上传下载，工作区末项仓库工具仅查询、搜索和显式确认投放，不发布或自动执行。管理员上传通道后续另行实施。属性及存储页按已应用模板展示设置过滤，分类编辑顺序独立于展示归组；ADR-047取代相关旧导航/接口分组/存储恒显规则，Server/Probe文件业务契约保持。
- ADR-035：Windows 主程序是原生 C# 工程工作区，页面在 `windows/RouterWorkbench.Desktop`，公开 API/WS Client 在 `windows/RouterWorkbench.Client`，Core 保留配置、外部启动与端点校验。当前规范与验证见 `docs/WINDOWS_DESKTOP_MIGRATION.md`；历史宿主验证不能替代 WPF 验证。
- ADR-034：独立生成器在 `src/ProbeTemplateGenerator`，采用强类型 C# 模型、状态、编译与发布逻辑，按 Feature 聚合，通过本机浏览器使用。工程8/草稿3与当前运行模板，公式/规则保留，旧格式和接口映射按ADR-044删除，当前规范见 `docs/TEMPLATE_GENERATOR_MIGRATION.md`。
- 保留 Accepted ADR-009～036 未被后续 ADR 明确取代的业务约束；ADR-025/026/027/030 的旧 UI 技术、旧布局冻结与内置终端不再适用。
- 客户端通过公开 `/api/v1` 和 WebSocket 使用 Server；不访问内部 Service、数据库、Gateway 或 Tunnel 私有数据面，不复制业务状态机。API 缺口先查已有能力，只有符合已确认契约的必要最小补充可自主实施，并同步 API、调用方与测试。
- UI 沿用当前 WPF 与 Blazor 的中文、浅色/深色/系统主题及各自页面基线；普通功能完成必要状态、错误、空态和可访问性交互，不主动整体重设计。docs/UI_FREEZE.md 的 React 冻结清单仅作历史参考。
- WebSocket 首连/重连回查 HTTP 快照；网络请求异步，切换 Server/退出取消并等待旧资源释放。响应不确定保留原幂等键、字节与 task_id，不自动创建替代任务；下载 committed/released 与 Task RESULT 分开。不得缓存或显示 Tunnel token、connection_id 或 data 私有细节。
- Maintenance 固定 Web/SSH/Telnet → Probe 127.0.0.1:80/22/23，默认 240 分钟，允许自定义正租期；独立 data TCP、创建 Session 绑定、默认 24 小时端口隔离、Gateway 有界独立控制发送及 Probe 默认 8 条/最大 64 条流保持。
- SSH/Telnet 使用本机外部客户端，不自研协议；文件内容继续走 File API，未提供遥测如实显示未提供。
- 资产持久化、身份、版本、匹配、归档与清理由 ADR-019 R1～R6 约束；artifact_id 全 Repository 唯一，tool_id/asset_id/artifact_id 不因归档、去重或路径变化重用。

## 任务执行要求

1. 将用户需求转成可观察的完成条件，核对当前产品范围、相关 ADR、API/Protocol、实现和测试；说明影响实现的约束与未决点，不要求用户代写技术规格。
2. 对已授权的普通功能，自主选择模块、局部实现、必要调用方/DTO 调整与回归测试，并完成工作；不为命名、文件拆分、既有组件复用等普通技术选择反复确认。
3. 信息不足时，先依据既有行为和设计采用最小合理假设并说明。仅当歧义会实质改变用户体验、数据语义、兼容性或任务范围且无法由仓库确定时，提出简短具体的问题；等待期间继续不依赖答案的工作。
4. 需要变更 Accepted ADR、协议/架构基线、破坏公开兼容性或进入暂缓能力时，先给出具体方案、依据、影响与推荐选择；已有明确确认不重复索取，尚未确认的依赖实现不先落地。普通功能授权不是扩大架构范围的授权。
5. 只修改完成需求必需的文件；保留无关的已有改动、运行数据和用户进程。不要顺手升级依赖、全仓格式化、创建空框架或实现后续能力。
6. 根据 docs/DEVELOPMENT.md 的风险范围运行构建、测试与必要实际交互；失败先判断是否由本次改动引起。不得删测、降断言或用 Mock 成功掩盖失败；无关问题记录，不无限扩大修复范围。
7. 不把未验证、未实现、仅计划或测试对端的能力描述为真实设备已通过。达到完成条件后交付并停止，不自动领取下一项功能。
8. Git 提交、推送、发布按当前任务中明确授权执行；历史 Phase 的一次性“推送 main”要求不是以后每项任务的默认指令。不得把无关改动和运行数据混入提交，不擅自重置或清理工作区。

## 每次有效开发任务的交付要求

每次有效开发任务结束时必须：

1. 完成需求及必要调用链，按影响范围验证并记录命令、环境与结果；纯文档任务检查差异、链接和规则一致性即可，不要求重跑产品构建与全量业务测试。
2. 更新 docs/PROJECT_STATUS.md。
3. 更新 docs/HANDOFF.md。
4. 按实际进度更新 docs/ROADMAP.md。
5. 架构变化时更新 docs/ARCHITECTURE.md。
6. 协议变化时更新 docs/PROTOCOL.md。
7. API 变化时更新 docs/API.md。
8. 重要设计变化时更新 docs/DECISIONS.md。
9. 用户可见功能、对外契约或重要治理规则变化时更新 CHANGELOG.md；不写普通内部过程流水账。
10. 输出简短交付摘要：完成项、验证结果、真实遗留问题及必要下一步；存在未运行检查或验收阻塞时说明原因，不伪造全量通过。

代码完成但 docs/PROJECT_STATUS.md、docs/HANDOFF.md 或 docs/ROADMAP.md 没有同步，视为本次任务未完整交付。

三个状态入口保持短、具体、可验证；记录本次结果与接管影响，不复制实现细节和历史日志。只读咨询/审查没有改变仓库状态时无需为满足清单制造文档修改。专项验证事实按受影响部分维护，不覆盖历史证据。

## 文档职责

- README.md 是项目总入口，不承载完整协议或完整架构。
- docs/PROJECT_STATUS.md 是短、具体、可验证的当前事实快照，不是历史流水账。
- docs/HANDOFF.md 是接管入口与导航，不复制专项文档。
- docs/ARCHITECTURE.md 维护系统边界与长期模块职责。
- docs/PROTOCOL.md 维护 Probe TCP 协议的规范性基线。
- docs/API.md 维护 API 的设计与实现状态。
- docs/ROADMAP.md 只按事实更新阶段和里程碑。
- docs/DECISIONS.md 使用轻量 ADR 记录重要设计决定及其原因。
- docs/DEVELOPMENT.md 提供需求分流、模块导航、验证入口与交付示例；不另建一套架构或 API/Protocol 规范。
- CHANGELOG.md 记录已经形成的用户可见产品行为变化、对客户端或开发者具有外部意义的协议或 API 契约变化，以及重要项目基线或治理规则变化。它不记录普通内部重构、未完成计划、虚构功能或单纯开发过程流水账。

## 状态标记

ROADMAP 使用以下标记：

- [x] 已完成并可验证
- [ ] 未完成
- [-] 暂缓或未决

PROJECT_STATUS 和 HANDOFF 中的能力描述必须与实际仓库一致。Phase 0 设计已经获得用户确认；真实仓库的 Git baseline commit 是 Phase 1A 写业务代码前的最后操作性门槛。在 baseline 实际形成前统一记录 `baseline commit: pending`，不得伪造 commit ID。
