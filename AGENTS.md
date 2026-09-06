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

Phase 0～3 已交付。Phase 4 首版提交为 `f92d73a0d6003835993967848f1f5fe009a0df89`，本轮从该 main 干净基线继续修正。当前范围以 Accepted ADR-021 / ADR-022、PROTOCOL.md、PHASE4_DESIGN.md 及用户指令为准。维护会话固定 Web/SSH/Telnet → 127.0.0.1:80/22/23，默认 240 分钟，可自定义正租期；数据使用独立 TCP，与创建时 Device Session 严格绑定。保持自研路线，不恢复 FRP/xfrpc。

端口本地释放后默认隔离24小时；控制发送由Gateway有界队列独立拥有，不得阻塞Maintenance本地释放；域名在Server解析，Probe仍只接收IP；Probe默认8条流、最大64，每流一线程；默认整连接idle24小时，租期优先。完整语义与有限端口隔离边界见ADR-022。正式历史必须在提交的仓库中，不依赖开发者本地临时保存项。

保留 Accepted ADR-009～018、Phase 1/2 的连接、设备、Session、任务、文件和幂等行为。具体资产模型、版本模型、目录结构和内部接口可自行设计；涉及资产持久化和存储目录、元数据持久化、版本唯一性与升级关系、arch/libc/kernel/model 匹配、去重/SHA-256/资产身份、删除/版本保留/清理的长期设计，先记录问题、原因、影响和推荐方案，等待用户明确确认。ADR-019 R1～R6 已获用户明确确认，状态为 Accepted。artifact_id 为全 Repository 唯一的不透明 UUID；tool_id、asset_id、artifact_id 均为稳定业务身份，不因归档、去重或存储路径变化重用。

Phase 4 必须验证撤销全部入口与活动数据连接、一次性配对、half-close、背压、有界资源、Session 替换/断线、租约、端口释放复用，以及真实 HTTP/SSH/Telnet 与控制/文件流量隔离。运行 Phase 1～3 全量回归、Go race/vet、C++ CTest/sanitizers、Linux Probe 和 Windows/Linux Management Server 适用验证，更新全部必要文档后才能标记完成。形成独立 Phase 4 commit 推送 GitHub main 后停止等待验收，不进入 Phase 5、HTTP/WebSocket API、UI、MCP、AI Agent、任意目标端口或通用端口映射。

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
