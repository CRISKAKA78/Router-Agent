# AI Agent 与开发者工作规则

本文件适用于仓库中的所有 AI Agent、Codex 会话和人工开发任务。仓库文件是项目连续性的事实来源，不得依赖聊天上下文、历史会话、AI 记忆或猜测代替仓库核对。

## 开始任务前的必读顺序

任何任务开始前必须依次阅读：

1. AGENTS.md
2. docs/HANDOFF.md
3. docs/PROJECT_STATUS.md
4. docs/ARCHITECTURE.md
5. docs/ROADMAP.md
6. 当前任务相关的 docs/PROTOCOL.md、docs/API.md 和 docs/DECISIONS.md

随后检查实际文件、代码、构建和测试状态。不得仅根据当前会话上下文、以前的聊天记录、AI 记忆或猜测继续开发。

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

协议或架构基线存在问题时，先在对应文档记录问题、原因、影响和建议方案，等待明确确认。尚未决定的内容必须标记为 TBD 或待讨论，不得推断为最终设计。

## 开发边界

- Management Server 优先使用 Go，并保持 Linux 与 Windows 跨平台运行能力；不得让基础运行强制依赖一组微服务、外部数据库或消息队列。
- Probe 是独立的轻量程序，面向 BusyBox、uClibc、mipsel、ARM、ARM64 和较老 Linux 内核；不得把具体 AI 排障逻辑放入 Probe。
- 项目采用 API First。Web、微信小程序、Windows UI、CLI、MCP 和 AI Agent 都是 Client 或 Adapter，不得复制核心设备控制逻辑。
- HTTP、WebSocket 等 Adapter 只能通过 Application 或 Service Layer 使用核心能力，不得直接操作 Probe TCP 连接注册表、存储表或内部 Manager。
- Probe TCP Gateway 负责连接与协议；Device、Task、File、Tool、Tunnel 等服务负责业务语义。
- TCP 控制链路不得承载 SSH、Telnet 或 Web Tunnel 的持续交互数据流。
- 每个阶段优先完成可构建、可运行、可测试的闭环。不得同时铺开多个无法验证的阶段。
- 不得通过占位代码、空类、空接口或大量未使用目录假装功能完成。

## 当前阶段限制

Phase 0～5已验收。用户于2026-09-06明确授权从main稳定基线 `57c2b1f8f6da1069e4a2eb988224b94bafe9cf84` 进入Phase 6 Windows UI；用户随后明确要求WinUI 3正式重构（起点f8d099d6），设计按Accepted ADR-025、API.md及PHASE6_DESIGN.md。本阶段仅实现Windows桌面管理客户端，通过Phase 5 `/api/v1` 和WebSocket调用Server，不访问内部Service、数据库、Gateway或Tunnel数据面，不复制业务状态机。

保留Accepted ADR-009～023及ADR-024未被ADR-025取代的边界。Maintenance固定Web/SSH/Telnet → Probe 127.0.0.1:80/22/23，默认240分钟，自定义正租期；独立data TCP、创建Session绑定、24小时默认端口隔离、Gateway有界独立控制发送及Probe默认8条/最大64条流保持。资产持久化/身份/版本/匹配/归档/清理由Accepted ADR-019 R1～R6约束；artifact_id全Repository唯一，tool_id/asset_id/artifact_id不因归档、去重或存储路径变化重用。

当前明确使用C# + WinUI 3，中文Fluent界面、Light/Dark/系统主题，设备侧栏与首要维护卡片；地址进入设置，维护/会话编号和释放原因进入详情。可自行组织合理UI分层并复用已有API/WS/DTO/外部SSH及Telnet启动逻辑。维护默认240分钟，可自定义正租期。WebSocket首连/重连回查HTTP快照；网络请求异步，切换Server/退出取消并等待旧资源释放。必要API缺口只允许最小补充，不重构Server。认证、TLS、RBAC、租户和完整审计仍为后续边界；配置保留合理扩展入口，不构建账号体系。不得缓存或显示Tunnel token、connection_id或data私有细节。

必须完成Windows客户端连接/断线/重连、设备实时更新/Session replacement、Maintenance创建/关闭/默认与自定义租期/到期/三入口、Exec结果、File/Tool、API错误、重复点击/并发和退出资源验证；保留Phase 1～5全量测试、Go race/vet、C++ CTest/sanitizers、Linux Probe与Windows/Linux Server适用验证。同步API/ARCHITECTURE/DECISIONS/PROJECT_STATUS/HANDOFF/ROADMAP/CHANGELOG及PHASE6_VERIFICATION。全部通过后独立Phase 6 Windows UI commit推送GitHub main，停止等待验收，不进入Web UI、微信小程序、MCP、AI Agent、通用端口转发、新Tunnel数据面、自研SSH/Telnet或任意目标端口。

## 任务执行要求

1. 先确认任务属于当前 ROADMAP 阶段，并核对用户明确授权的范围。
2. 阅读相关设计和 ADR，列出会影响实现的已确认约束与 TBD。
3. 只修改完成当前任务必需的文件；保留仓库中无关的已有改动。
4. 为已经实现的行为提供与风险相称的构建、单元测试或集成测试。
5. 不把未验证、未实现或仅计划中的能力描述为已完成。
6. 若任务需要改变已确认设计，先取得明确确认，再修改实现与文档。

## 每次有效开发任务的交付要求

每次有效开发任务结束时必须：

1. 运行与本次变更相关的构建和测试，并记录结果。
2. 更新 docs/PROJECT_STATUS.md。
3. 更新 docs/HANDOFF.md。
4. 按实际进度更新 docs/ROADMAP.md。
5. 架构变化时更新 docs/ARCHITECTURE.md。
6. 协议变化时更新 docs/PROTOCOL.md。
7. API 变化时更新 docs/API.md。
8. 重要设计变化时更新 docs/DECISIONS.md。
9. 用户可见功能变化时更新 CHANGELOG.md。
10. 输出本次交付摘要，包括完成项、验证结果、遗留问题和下一步建议。

代码完成但 docs/PROJECT_STATUS.md、docs/HANDOFF.md 或 docs/ROADMAP.md 没有同步，视为本次任务未完整交付。

## 文档职责

- README.md 是项目总入口，不承载完整协议或完整架构。
- docs/PROJECT_STATUS.md 是短、具体、可验证的当前事实快照，不是历史流水账。
- docs/HANDOFF.md 是接管入口与导航，不复制专项文档。
- docs/ARCHITECTURE.md 维护系统边界与长期模块职责。
- docs/PROTOCOL.md 维护 Probe TCP 协议的规范性基线。
- docs/API.md 维护 API 的设计与实现状态。
- docs/ROADMAP.md 只按事实更新阶段和里程碑。
- docs/DECISIONS.md 使用轻量 ADR 记录重要设计决定及其原因。
- CHANGELOG.md 记录已经形成的用户可见产品行为变化、对客户端或开发者具有外部意义的协议或 API 契约变化，以及重要项目基线或治理规则变化。它不记录普通内部重构、未完成计划、虚构功能或单纯开发过程流水账。

## 状态标记

ROADMAP 使用以下标记：

- [x] 已完成并可验证
- [ ] 未完成
- [-] 暂缓或未决

PROJECT_STATUS 和 HANDOFF 中的能力描述必须与实际仓库一致。Phase 0 设计已经获得用户确认；真实仓库的 Git baseline commit 是 Phase 1A 写业务代码前的最后操作性门槛。在 baseline 实际形成前统一记录 `baseline commit: pending`，不得伪造 commit ID。
