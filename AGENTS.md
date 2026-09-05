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

Phase 0 baseline 与 Phase 1A～1D 已形成提交，Phase 1E 已通过完整验收。用户本次授权 Phase 1E Verification：以 main 实际代码为基础，对 Protocol v1 与 Server/Probe 的 Phase 1A～1D 做完整收口验收，不新增后续功能。明确 bug 可直接修复并补测试；需要改变 Protocol v1 互操作语义的新设计缺口，先记录并汇报，等待明确确认。遵守 ADR-009～017 的已确认契约。

完成 C++、Go、race、vet、真实 Probe 集成与 Windows Server 适用验证后，同步必要文档；只有实际验收通过才能将 Phase 1 标记为完成。创建独立 Phase 1E / Verification commit 并推送 GitHub，然后停止等待验收。不得进入 Phase 2；不实现 resume、专用文件数据连接、File/Tool Repository、Process Manager、Tunnel、HTTP/WebSocket API、数据库、UI、MCP、AI Agent、TASK_CANCEL、Probe/Server 进程重启恢复或后续安全体系。

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
