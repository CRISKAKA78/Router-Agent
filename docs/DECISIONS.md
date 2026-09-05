# 架构决策记录

本文件使用轻量 ADR 记录重要设计决定与待确认草案。仅 Accepted 条目构成已确认决定；Proposed 条目不得被实现推断为已接受。状态为 Accepted 的决定不得被实现静默改变；需要变更时，应新增取代决策并说明迁移与影响。

## ADR-019 Phase 3 File and Tool Repository

- 状态：**Accepted。用户明确确认 R1～R6，并补充 artifact_id 全 Repository 唯一与稳定身份不重用规则；实际完成状态见 PROJECT_STATUS。**
- 日期：2026-09-05。
- 启动基线：HEAD、main 与 fetch 后 origin/main 同为 `b9982f5d2765546d23e09c28977c27ceb510a368`，起始工作区干净。
- 授权：用户已授权 ROADMAP Phase 3；明确要求以下长期设计先汇报推荐方案并等待确认。AGENTS/HANDOFF 的旧“不得进入 Phase 3”属于上一阶段状态与授权边界，本次同步为新授权，不修改历史 Accepted ADR。
- 启动代码事实：`internal/filetransfer` 已负责流式准备、upload/download、size/SHA-256、传输状态与下载提交事实；Gateway 提供 CreateUpload/CreateDownload/ResendTask 等适配入口。`internal/device` 保存完整 REGISTER 资料，但当前真实 Probe 只发送 device_id、probe_version、可选 hostname、arch、boot_id 和 exec/file capabilities，未发送 libc/kernel/model。启动时仓库没有 Repository、资产目录或元数据持久化。
- 问题与影响：ARCHITECTURE 对 Repository 存储、版本、兼容性和清理仍为 TBD；这些决定将影响资产引用稳定性、重启后可用性、设备投放准入和以后 API 的资源模型，不能由实现隐含确定。
- 与已接受设计关系：建议补充 ADR-003/004/006/011 的管理端能力，保持 ADR-009～018 的 wire、Device/Session、任务/传输幂等及提交事实。不把 Repository 持久化扩大为 Device/Task/Session 恢复。

### R1 文件资产持久化与目录

决定：使用 Server 本地文件系统作为持久化 Repository，无外部数据库或服务。提供 `repository-dir` 配置，默认 `./data/repository`，相对于 Server 启动工作目录解析为绝对路径并记录实际路径；路径由部署者配置，不能从资产名称或设备字段拼接。Repository 是单进程独占写入的目录，不支持多个 Server 共享写入。

目录职责为 `blobs/sha256/<前两位>/<完整摘要>` 存不可变字节、`metadata/` 存版本化目录清单、`staging/` 存导入和下载暂存。运行数据排除 Git。资产导入流式计算 size/SHA-256，完整内容先发布，再提交元数据引用；失败不能生成指向不完整文件的可用资产。目录锁与发布方式必须在 Linux/Windows 验证，启动失败不能静默降级为内存仓库。

原因与影响：重启后工具必须仍能找到实际文件，不能引用调用者随时可能修改的源路径。此方案保持单程序部署；仅支持本地文件系统，不承诺网络共享盘、多进程写入或完整灾难恢复。

### R2 元数据持久化范围

决定：资产元数据、工具身份、版本、产物与兼容约束及归档状态均持久化。第一版采用带 schema_version 的 JSON 目录快照，串行写入、临时写入并同步后完整发布；更新失败保留上一有效状态，不把内存状态先宣布成功。启动严格验证结构、唯一性和引用；损坏或不支持的格式报错，不自动创建空仓库覆盖原数据。

仅持久化 Repository；Device Inventory、Session、Task/transfer 和投放过程记录继续沿用既有进程内生命周期。Server 重启不恢复旧在线状态，不恢复或自动重发旧任务。目录备份要求停服后一并复制元数据和 blobs；在线备份、迁移工具及完整断电恢复保证后续另议。小规模目录整体快照的成本可以接受，暂不引入 SQLite 或其他数据库依赖。

### R3 工具版本唯一性与升级关系

模型：FileAsset（独立 asset_id、名称、size、SHA-256、创建时间、归档状态）；Tool（稳定 tool_id、名称、描述、归档状态）；ToolVersion（tool_id + version、创建时间、归档状态）；Artifact（独立不透明 UUID artifact_id，在整个 Repository 全局唯一；另含 asset_id、platform、兼容规则、投放 mode）。一个工具版本允许多个架构/平台产物，一个产物引用一个完整资产。

用户补充确认：tool_id、asset_id、artifact_id 均为稳定业务身份，不因归档、文件去重或存储路径变化而重用；artifact_id 使用独立不透明 UUID，在整个 Repository 范围全局唯一。

`(tool_id, version)` 唯一，version 是区分大小写的不透明标签；不推断 SemVer 排序或自动选择 latest。发布时版本、产物、资产绑定和兼容规则整体不可变；改文件或约束须发布新版本。重复发布完全相同规格返回已有记录，不同规格冲突；归档后也保留身份，不能用同版本号覆盖历史。

投放必须明确工具版本；可以明确 artifact_id，自动选择时仅允许恰好一个兼容产物，多个匹配返回歧义并由调用者选择。升级或回退均是调用者显式选择版本并发起新 upload；本阶段不建安装状态、自动升级链、包管理器、依赖解析或工具自动执行。

原因与影响：同一版本可能需要 mipsel/ARM/ARM64 多个文件；版本号与摘要各有职责。显式版本和不可变产物避免重试时暗中改变投放内容。

### R4 平台与兼容性匹配规则

决定：兼容结果为 compatible / incompatible / unknown，并返回逐项原因。不同字段之间 AND，同字段允许值之间 OR，required_capabilities 必须全部具备；缺少受限制的设备字段为 unknown，已知不匹配为 incompatible，只有全部通过才 compatible。有任何已知不匹配时总结果 incompatible，其余未满足因缺失信息时 unknown。投放仅接受 compatible，且设备必须 online 并声明 file 能力。

- platform 显式为 linux；现有 Probe 架构基线仅支持 Linux，当前不新增 OS 上报字段，也不推导 Windows Probe 支持。
- arch 必须声明允许集合或显式 any。只规范化明确等价别名 `amd64 -> x86_64`、`arm64 -> aarch64`；其余精确匹配。arm 不推断 ARM 代际、浮点 ABI；mips/mipsel 不互通。any 是发布者明确声明，不是未填写的默认值。
- libc 使用精确允许集合，或显式 any 表示该产物不依赖特定 libc；将 ASCII 大小写统一为小写。缺失 libc 不能满足 glibc/uclibc/musl 限制，不凭 arch 猜测。当前不推断 libc 版本或 ABI 兼容。
- model 可选精确允许集合，区分大小写；空集合明确表示不限制，受限而未知则 unknown。
- kernel 可选完整字符串允许集合，精确匹配；空集合表示不限制。Phase 3 不实现最低版本、范围、正则或厂商后缀推断，避免把内核版本数字当成 ABI/特性保证。
- capabilities 精确 token 集合包含判断；仍表示 Probe 声明的协议能力，不代表系统自带命令、CPU 特性或授权。兼容判断是对已声明条件的匹配，不证明文件实际能执行。

真实 Probe 当前缺少 libc/kernel/model 时，受这些条件限制的工具不能投放；明确不限制这些字段的产物可形成真实投放闭环。完整字段组合通过 Device Service 单元测试和真实 TCP 注册测试覆盖，本阶段不为 Repository 向 Probe 增加资产概念或新 wire 字段。

兼容查询可针对离线设备的最近资料返回匹配结果，但不能据此直接派发。投放检查绑定当前 session_id，实际发送前若 Session 已替换则返回错误，由调用者重新查询和判断；不会自动沿用旧资料投向新 Session。Gateway 只执行通用 Session 前置条件，不了解工具规则。

### R5 文件去重、SHA-256 与资产身份

决定：asset_id 使用独立不透明 UUID，SHA-256 标识存储内容和完整性，不作为资产业务主键。同内容允许多个资产记录（名称/来源可能不同），只共用一个不可变 blob。不同工具版本可以引用相同 asset_id 或共用同内容 blob；去重不合并工具、版本、任务或 transfer 身份。

导入计算摘要；复用已有 blob 时验证其大小和内容，冲突/损坏报错而不覆盖。投放准备须将实际内容 size/SHA-256 与资产记录核对，再复用既有 upload；已有传输过程的再次校验保留。不能因文件被外部修改就把新摘要默认为该资产的新内容。

原因与影响：资产业务身份不会因存储布局或去重改变；能复用字节而不混淆引用。SHA-256 用于既有完整性和内容寻址，不引入签名或来源认证。

### R6 删除、版本保留与清理

决定：Phase 3 的删除语义为归档（逻辑删除），保留资产/工具/版本及其 ID、引用和文件。归档项不能用于新的引用或投放，查询可显式包含归档项；已派发任务与传输继续使用固定规格，不受归档影响，不撤回设备文件。

资产被未归档版本引用时拒绝归档，先归档引用它的版本；归档工具使其全部版本不可新投放，但保留历史身份与数据。不自动清理旧版本，不重用归档版本号，不物理删除 blob，不在本阶段实现 GC 或设备端卸载。只清理本次操作自有且尚未被传输占用的暂存文件；崩溃遗留暂存/未引用 blob 保留并报告，后续显式清理策略另议。

原因与影响：避免清理与进行中传输、共享 blob、旧任务发生竞态；代价是磁盘占用增长且本阶段归档不会回收空间。

### 已确认内部投放与下载闭环

File Repository 管理资产字节和元数据，Tool Service 管理工具/版本与兼容性；Application Service 组合 Repository、Device 查询及既有文件/任务接口。Repository 不持有 Gateway 连接表、不处理 FILE 帧，Device 不依赖 Repository。

投放：明确 tool/version → 查询当前设备与兼容候选 → 固定 artifact/asset/session → 核对内容 → 既有 CreateUpload（远端路径、mode、overwrite、timeout）→ 返回并保存 task_id/transfer_id 及资产/版本关联 → 通过原 TaskSnapshot/FileSnapshot/WaitTaskResult 查询。资产 ID、工具 ID、版本、兼容规则留在 Server。非空 task_id 与 ErrDispatchUncertain 必须一起保留，绝不自动生成替代任务；ResendTask 仍只查询/补报原身份，文件中断重新传输须新 task_id/transfer_id。

下载：既有 CreateDownload 写入 Repository 分配的暂存路径；显式完成导入时检查 FileSnapshot.Committed 并重新核对 size/SHA-256，成功后登记资产。Task 最终状态与本地提交事实分别返回；允许在已提交但 done ACK 丢失导致 failed 时显式保留该完整资产，不伪造任务 success。没有完整本地提交事实则禁止导入。同一下载任务在本进程中重复完成导入返回已有资产，不重复创建身份；重启后的任务关联恢复不在本阶段。

### 验证与当前状态

验证覆盖导入/重开/损坏/失败发布、重复内容与引用、版本不可变与归档、兼容规则/未知/歧义、Session 替换、派发失败与不确定派发、真实 Probe 投放和重复任务、下载导入与提交/确认分离，并运行 Phase 1/2 全量回归、Go race/vet、C++ 构建/CTest 和 Windows 原生适用验证。用户已明确确认并授权实现；Phase 3 完成与验证事实见 PROJECT_STATUS。

## ADR-001 Probe 主动建立 TCP 长连接

- 状态：Accepted
- 日期：2026-09-05
- 决定：Probe 主动连接 Management Server，并保持 TCP 控制长连接。
- 背景与原因：路由器通常位于 NAT 或防火墙之后，由设备主动连接公网管理端更符合网络可达条件。长连接也为注册、心跳、任务和事件提供统一通道。
- 影响：Probe 负责连接、注册、心跳、退避重连和会话更新；Server 维护 device_id 到当前连接的映射。每次重连后重新 REGISTER 并生成新的 session_id。

## ADR-002 控制 TCP 与 Tunnel 数据连接分离

- 状态：Accepted
- 日期：2026-09-05
- 决定：Probe 控制 TCP 不承载 SSH、Telnet 或 Web Tunnel 的持续数据流；这些流量使用独立数据连接。
- 背景与原因：阻塞的交互会话或大流量请求不能影响设备心跳和管理任务。
- 影响：open_tunnel 和 close_tunnel 通过控制链路管理 Tunnel 生命周期，实际交互字节流由独立 Tunnel 或 Relay 连接承载。Tunnel 数据面协议仍为 TBD。

## ADR-003 Management Server 优先采用 Go

- 状态：Accepted
- 日期：2026-09-05
- 决定：Management Server 优先使用 Go，实现为可直接运行的跨平台核心程序。
- 背景与原因：平台至少需要覆盖 Linux 和 Windows，并保持基础部署简单。
- 影响：实现应避免依赖 systemd、固定 Linux 路径或只能在单一操作系统运行的组件。基础功能不强制依赖一组微服务、外部数据库或消息队列。后续可扩展 macOS。

## ADR-004 采用 API First

- 状态：Accepted
- 日期：2026-09-05
- 决定：核心能力先实现于 Application 或 Service Layer，再由 HTTP、WebSocket、Web、微信小程序、Windows UI、CLI、MCP 和 AI Agent 等 Adapter 或 Client 使用。
- 背景与原因：多个前端需要共享一致的设备控制、任务、文件和 Tunnel 语义。
- 影响：UI 和 Adapter 不得直接操作 Probe 连接注册表、存储表或内部 Manager，也不得重新实现核心设备控制逻辑。公开 API 预留 /api/v1 版本前缀。

## ADR-005 项目状态持久化在仓库文档

- 状态：Accepted
- 日期：2026-09-05
- 决定：项目状态、接管信息、路线图、协议、架构和重要决策必须维护在仓库中，不依赖 AI 会话上下文。
- 背景与原因：全新的 AI Agent 或开发者必须能在没有历史聊天的情况下确认事实并继续开发。
- 影响：PROJECT_STATUS 保存当前事实快照，HANDOFF 提供接管入口，ROADMAP 保存阶段状态。每次有效开发任务结束必须同步这些文档；代码完成但文档未同步视为交付不完整。

## ADR-006 Probe 保持轻量

- 状态：Accepted
- 日期：2026-09-05
- 决定：Probe 只提供连接、协议、执行、文件、进程、Tunnel 和设备信息等通用原语，不承担具体 AI 故障诊断逻辑。
- 背景与原因：Probe 面向 BusyBox、uClibc、多种 CPU 架构和较老 Linux 内核，需要控制体积、依赖和资源消耗。诊断知识与工具编排更适合集中在管理端。
- 影响：管理端承担工具仓库、脚本、设备能力判断和 AI 编排。Probe 不使用重型数据库，只保存必要的小规模持久状态。

## ADR-007 按阶段保持可验证基线

- 状态：Accepted
- 日期：2026-09-05
- 决定：项目按 ROADMAP 阶段推进，每个阶段优先形成可构建、可运行、可测试的闭环；不得跨多个阶段长期堆积不可验证代码。
- 背景与原因：大量空框架和并行未完成阶段会掩盖真实进度，并降低后续接管可靠性。
- 影响：未实现能力必须在 PROJECT_STATUS 和 ROADMAP 中明确标记。不得用占位代码、空类、空接口或大量空目录表示完成。进入新阶段前应确认上一阶段最低验收标准，除非 ROADMAP 明确记录例外。

## ADR-008 Markdown 是持续维护的当前设计基线

- 状态：Accepted
- 日期：2026-09-05
- 决定：Phase 0 完成后，仓库内 Markdown 文档成为项目持续维护的当前设计基线。Word v0.2 保留为 Phase 0 原始设计输入和历史参考。
- 背景与原因：项目需要在保留原始设计来源的同时，允许经过正式确认的设计继续演进。
- 影响：Word v0.2 不得覆盖后续已经写入 Markdown 或 ADR 的正式变化。文档冲突必须按 AGENTS.md 的事实来源层级分类处理。需要改变 Accepted ADR 时新增 superseding ADR，不静默改写历史决定。

## ADR-009 Protocol v1 统一传输关联与编码规则

- 状态：Accepted
- 日期：2026-09-05
- 决定：Protocol v1 统一 message_id、reply_to、Flags、transfer_id、JSON、boot_id 和默认心跳超时的互操作规则。
- 背景与原因：v0.2 已经固定帧结构和消息类型，但这些细节若由 Server 与 Probe 分别推断，会产生不兼容实现。
- 影响：
  - message_id 从 1 开始，按单连接和单发送方向独立递增，重连重置，0 保留，达到 uint64 最大值时重连且不回绕。
  - RESPONSE JSON 使用 reply_to 关联请求；TASK_RESULT 和 EVENT 不设置 RESPONSE。
  - FILE_CHUNK 设置 BINARY，JSON 消息不设置 BINARY，MORE 在 Protocol v1 中为 0。
  - transfer_id 统一为 canonical UUID string 和 RFC 4122 16-byte wire format。
  - JSON 使用 UTF-8 object 并遵循 PROTOCOL.md 的基础校验规则。
  - boot_id 优先使用系统 boot identity，退化值只代表 Probe 生命周期。
  - 默认心跳间隔为 30 秒，双方默认 90 秒未收到合法消息时判定失联或重连。

## ADR-010 Phase 1 任务拒绝与幂等边界

- 状态：Accepted
- 日期：2026-09-05
- 决定：TASK_ACK accepted=false 是最终拒绝，不再发送 TASK_RESULT；Protocol v1 Phase 1 不发送 TASK_RESULT rejected，也不实现 TASK_CANCEL。task_id 去重至少覆盖同一个 Probe 进程生命周期，包括 TCP 断开和重连。
- 背景与原因：拒绝状态、取消范围和重连后的任务重复执行需要在编码前形成单一语义。
- 影响：Server 根据 accepted=false 将业务任务记为 rejected。0x13 保留但 Phase 1 按未支持操作处理。Probe 对 RUNNING 或已缓存完成的 task_id 返回已有状态或结果，不重新执行副作用操作。Probe 重启后的恢复和持久化仍为 TBD。

## ADR-011 Phase 1 文件传输由 Server 编排

- 状态：Accepted
- 日期：2026-09-05
- 决定：upload 和 download 的 transfer_id 均由 Management Server 创建，并贯穿 TASK 与全部 FILE 消息。Probe 不接收 local_asset、tool_id 或 asset 等 Server 内部标识。一个 Probe 控制连接同时最多存在一个 active file transfer。
- 背景与原因：统一 transfer_id、文件元数据和时序可以避免 Server 资产模型泄漏到 Probe，也能防止多个文件流饿死控制消息。
- 影响：upload 和 download 使用 PROTOCOL.md 中固定的 TASK_ACK、FILE_ACK 和 TASK_RESULT 时序。写入同一 socket 必须由单一 writer 或等价机制串行化；控制消息优先于 FILE_CHUNK。Phase 1 不实现 FILE_CHUNK 单块 ACK、sliding window 或断点续传。

## ADR-012 Phase 1 使用里程碑和技术决策门槛

- 状态：Accepted
- 日期：2026-09-05
- 决定：Phase 1 保持一个总阶段，但按 Phase 1A 至 Phase 1E 的可验证里程碑推进。进入 Phase 1A 业务开发前必须确认 Probe 语言、最低语言标准、构建系统、第一验证平台和交叉编译策略。
- 背景与原因：每个里程碑需要形成可构建、可运行、可测试的小闭环，避免一次性铺开整个 Phase 1。
- 影响：上述 Probe 技术选择保持 TBD，不得由实现者自行决定。Management Server 使用 Go 的 ADR-003 保持不变。用户确认 Phase 0 并形成 Git baseline 之前不得开始 Phase 1。
- 后续状态：该技术决策门槛已由 ADR-013 完成；Phase 1A 注册与心跳互操作契约由 ADR-014 完成。Git baseline 仍需在真实仓库形成。

## ADR-013 Probe 使用 C++11 与 CMake

- 状态：Accepted
- 日期：2026-09-05
- 决定：Probe 从 Phase 1 起使用 C++11 实现，使用 CMake 作为构建系统。第一开发与验证平台为 Linux x86_64；后续通过 CMake toolchain files 支持 mipsel、ARM 和 ARM64 交叉编译。
- 背景与原因：Probe 后续需要连接管理、协议编解码、并发任务、文件和 Tunnel 等多个长期模块，C++11 能在保持嵌入式兼容性的同时提供 RAII、线程和类型封装能力；CMake 只存在于构建主机，不增加目标路由器运行时依赖。
- 影响：Probe 代码不得依赖高于 C++11 的语言特性；Phase 1A 首先在 Linux x86_64 上形成闭环，再逐步验证交叉编译。目标侧基础依赖应尽量收敛到 libc、pthread、C++ 标准运行库和明确审查过的轻量依赖。具体 mipsel、ARM、ARM64 工具链版本及最低 libc/内核兼容矩阵后续按真实设备补充。

## ADR-014 Phase 1A 注册与心跳契约冻结

- 状态：Accepted
- 日期：2026-09-05
- 决定：REGISTER、REGISTER_ACK、HEARTBEAT 和 HEARTBEAT_ACK 的必选字段、类型、允许范围、注册失败响应以及 heartbeat_interval 范围按 PROTOCOL.md 的 Phase 1A 规范执行。
- 背景与原因：Phase 1A 的 Server 与 Probe 必须能够独立实现后稳定互操作，不能只依赖 JSON 示例推断字段语义。
- 影响：REGISTER_ACK 成功与失败均使用 0x02 并设置 RESPONSE；失败后连接关闭且 Probe 不进入 ONLINE。heartbeat_interval 允许 10-300 秒，双方失联阈值统一为 3 倍 heartbeat_interval；默认 30 秒对应 90 秒。任何字段契约变化必须先更新 PROTOCOL.md 并形成正式评审。

## ADR-018 Phase 2 Device Management

- 状态：Accepted。用户明确确认 D1～D4，授权按方案实现 Phase 2；完成状态以 PROJECT_STATUS.md 为准。
- 日期：2026-09-05。
- 启动基线：main 与 fetch 后的 origin/main 均为 `3de7924353d05261e6b01faa87cbca00a0a0d0a5`，起始工作区干净。
- 问题与原因：当前 Gateway 解析完整 REGISTER，却仅在活动连接中保存 device_id/session_id；断线删除连接记录，lastSeen 是 Reader 局部变量，Events 是容量 128 的可丢通知。已有规范没有定义设备资料更新、历史保留和设备持久化边界；直接扩展会把这些长期语义隐含在 Gateway 实现中。
- 影响：Phase 2 的设备详情、当前在线状态与历史查询无法仅依赖现有连接表或 Events 实现；启动时先记录并汇报以下范围，用户随后明确确认；本 ADR 补充 Device 管理内部语义，不改变 ADR-009～017 或 Protocol v1 wire 契约。

### D1 Device 与 Session 身份及生命周期

决定：device_id 是唯一稳定设备主键；相同 ID 始终更新同一条 Inventory 记录。首次成功注册 ACK 完整写出并发布后才建设备及 Session；注册失败和 ACK 写失败不产生设备上线记录。并发注册沿用当前成功发布的串行顺序，最后发布者成为唯一当前 Session，随后关闭旧连接，不按 TCP accept 时间或 boot_id 选择胜者。

每次注册使用新 session_id；设备记录 FirstSeenAt、LastSeenAt、LastOnlineAt、LastOfflineAt、当前/最近 Session 和 online/offline 状态。Session 记录开始时间、最后活动时间、结束时间及结束原因；在线替换时旧 Session 以 replaced 结束，设备保持 online，不伪造一次设备离线。旧 Session 的迟到活动和清理不能刷新或下线新 Session；当前 Session 断开、失联、写失败或 Server 关闭后才收敛为 offline。last_seen 使用 Server 接收并通过既有校验的消息时间，沿用 3 倍心跳失联规则；不得用 Probe 时间或 boot_id 推断进程连续性、重启或任务状态。

用户补充确认的时间语义：LastOnlineAt 是最近一次成功发布当前 Session 的时间；LastOfflineAt 仅在设备整体从 online 转为 offline 时更新；Session replaced 时设备保持 online，不更新 LastOfflineAt。首次尚未离线时 LastOfflineAt 为空（Go time.Time 零值）。

REGISTER 基础字段和 capabilities 作为最近成功注册的完整快照替换；新 REGISTER 省略的可选字段清空为未知，不混入旧会话资料。保留未知 capability token 供查询，但不将声明解释为 Server 已实现该能力，也不在本阶段新增任务能力准入规则。每个保留的 Session 保存对应注册资料快照，设备详情显示最近一次资料；无需新增 get_info 或任何 wire 字段。

### D2 历史状态保留边界

决定：每台设备保留一个当前 Session，加最近 64 个已结束 Session（容量为内部配置，可调整）；旧历史按结束顺序淘汰。每条 Session 的开始/结束、last_seen、结束原因和注册快照组成最小历史查询闭环。保留设备首次/最近时间、累计 Session 数和已淘汰数量，使调用者知道历史已截断；设备离线后不删除 Inventory 记录。当前 Session 不占历史槽位。

不保存每次心跳、连续性能时序、全部协议帧或完整审计日志，不提供重放事件流。历史淘汰只影响 Device 查询，不删除 Task 派发/结果、File 身份/提交事实，也不改变 Probe 幂等缓存。设备数量在本进程内不设自动淘汰，因此总内存随设备数量增加；历史容量只限制每台设备的 Session 记录。

### D3 本阶段持久化范围

决定：Phase 2 使用进程内 Inventory，跨 TCP 断线与重连保留，Server 进程重启后清空；本阶段不引入数据库、磁盘快照或状态恢复。这样可完成当前运行周期内的设备与历史查询闭环，并保持现有 Task/File 的内存生命周期。

该建议不是对长期存储引擎的选择。Management Server 的持久化、迁移和备份仍为 TBD，后续若要求重启后保留设备及历史，须单独确定磁盘模型和启动时在线状态收敛规则，不能把旧 online 状态直接恢复为在线。

### D4 Device Service 与 Gateway / Task / File 职责

决定：新增正式 internal/device Service，由它拥有设备业务模型、Session 元数据、状态转换、历史及并发安全的查询快照。Gateway 继续拥有 socket、writer、device_id 到当前连接的传输映射、消息校验和路由，不保存第二套设备业务模型。

Gateway 在注册发布、合法活动、替换和关闭路径同步调用 Device Service，不从可丢 Events 异步重建状态；发布与清理按 session_id 校验并排序。Device Service 的锁不跨网络或磁盘 I/O，不调用 Task/File，不暴露 socket/内部 map；查询返回独立副本。内部能力为设备清单、设备详情、当前/最近 Session、保留 Session 历史查询，外部 Adapter 以后复用这些 Service 能力。

Task Service 继续拥有任务规格、ACK/RESULT 关联与幂等；File Transfer Service 继续拥有传输和本地提交事实。本阶段不把 Task/File 业务移入 Device Service，也不改变已接受的断线任务与文件行为；不新增 HTTP/WebSocket/UI/MCP 或 Phase 3 功能。既有 Gateway 任务调用入口的更大范围 Application 层整理不纳入本阶段。

### 验证与交付

覆盖首次上线、资料/capabilities、失败注册、ACK 发布顺序、断线/重连、并发替换及迟到清理、合法活动、超时、Server Close、历史截断和查询副本隔离；真实 Probe 验证设备查询与重连历史，并运行 Phase 1 全量回归、Go race/vet、C++ 构建/CTest 和 Windows Server 适用验证。全部通过后更新事实文档，形成独立 Phase 2 commit 并推送 GitHub，停止等待验收。用户确认后开始实现，实际验证结果另记。

## 当前待决策主题

以下主题尚未形成 Accepted ADR：

- Management Server 的持久化、备份与迁移方案。
- 平台和设备的认证、授权、加密与审计方案。
- Tunnel 数据面协议和 Relay 拓扑。
- Probe 重启后的 task_id 缓存、未上报结果和任务恢复策略。
- Server 重启后的任务、Session 和传输恢复策略。
- Protocol 错误严重程度与关闭连接矩阵。
- API 的正式资源模型、错误、认证和实时事件协议。

协议层的具体未决项见 [PROTOCOL.md](PROTOCOL.md) 的 Protocol Review。

### 2026-09-05 Phase 1C 接管审查补充

审查时尚未确认的重复 TASK、内容冲突和补报契约，在启动检查后由用户明确确认，现见 ADR-015。审查历史保留在 [PHASE1AB_REVIEW.md](PHASE1AB_REVIEW.md)，既有 Accepted ADR 保持原文。

## ADR-015 Phase 1C 重复任务与跨连接结果契约

- 状态：Accepted
- 日期：2026-09-05
- 确认依据：用户在 Phase 1C 启动检查汇报后明确回复“确认”，接受 PROTOCOL.md 记录的三项契约及比较、重放范围建议。
- 性质：补充 ADR-009/010 未展开的互操作细节，不取代其传输编号、最终拒绝或进程生命周期去重决定。
- 决定：重复 queued/running 返回 accepted=true 及对应 state；完成态先发送 accepted=true/state=success/failed/timeout 的 TASK_ACK，再返回原 TASK_RESULT。ACK.reply_to 关联本次 TASK，RESULT 不设置 RESPONSE。
- 执行身份：比较已解析 type、timeout、command、cwd、env；env 键顺序无关，省略 cwd/env 与空值等价；忽略 created_at 和未知扩展字段。相同 ID 内容冲突通过 ERROR/INVALID_PAYLOAD 响应本次 TASK，保留原任务，绝不重执行。
- 结果恢复：每次成功 REGISTER 后重放缓存完成结果，包括旧连接上写成功的结果；运行中 exec 继续执行，完成后通过可用连接发送。不得伪造旧 ACK，不新增 RESULT_ACK。Server 对同 device_id 的已派发 task_id 可以在缺 ACK 时接受结果。
- 幂等终态：已定义结果业务字段完全相同才是重复 RESULT；幂等成功，冲突返回 ERROR/INVALID_PAYLOAD 且不覆盖首个终态；rejected 不可被结果改写。ACK 必须匹配 session_id/message_id/task_id 派发记录。
- 资源边界：允许有界缓存，不淘汰已接受任务的身份与结果，容量不足时拒绝新任务。具体容量和 worker 数属于实现配置；Probe 与 Server 重启恢复仍为 TBD。

## ADR-016 Phase 1D 授权与已确认文件传输约束

- 状态：Accepted
- 日期：2026-09-05
- 确认依据：用户明确要求开始 Phase 1D，并列出十二项已确认规则及交付边界。
- 性质：补充 ADR-011 原先未决定的等待策略；保留 ADR-009/010/011/015 的既有决定。
- 决定：每个 Probe 控制连接最多一个 active file transfer；其他已接受文件任务使用有界 FIFO，容量是实现配置，满时拒绝新文件任务。
- 编码：Server 创建 canonical UUID transfer_id，CHUNK 使用 RFC 4122 16 bytes；内容必须为 binary FILE_CHUNK 且设置 BINARY，其余 FILE 消息为 JSON。
- 生命周期：upload/download 兼容 Phase 1C task_id 幂等，重复 ID 不重复文件副作用；中断的 transfer_id 失败，重传必须使用新的 task_id 和 transfer_id，不实现 resume。
- 流与校验：流式读写，校验 size/SHA-256；不完整或校验失败的文件不可发布为最终文件；控制消息优先，每个 CHUNK 后重查控制队列。
- 边界：不做逐块 ACK、sliding window、多文件并行流或专用数据连接；保留 Phase 1A/B/C 行为，不进入 Phase 1E 或用户明确排除的后续能力。
- 交付：完整回归、Go race/vet、C++ 与真实 Probe 集成通过后，同步文档，独立提交 Phase 1D 并推送 GitHub，停止等待验收。该 ADR 接受时仅完成启动检查；后续实现与验证状态以 PROJECT_STATUS.md 为准。

## ADR-017 Phase 1D 文件互操作补充

- 状态：Accepted
- 确认依据：用户明确回复“确认 P1～P5，按草案继续实现”，并要求 ready 省略 sha256_ok、done 固定 true、failed 固定 false。
- 日期：2026-09-05
- 基线：main `5030322b58fcbb07cae2f3256a71eb9a750a479a`。
- 问题：现有 FILE 章节主要是示例；未定义有界 FIFO 晋升时的握手、全部必选字段、CHUNK 长度口径、传输阶段失败关联、排队任务断线处理和文件提交后确认丢失的结果边界。ADR-015 的执行身份字段只覆盖 exec。
- 决定：采用 PROTOCOL.md 末尾 P1-P5 规范，复用既有消息号，不增加 ACK 类型、取消消息或恢复协议。
- 影响：P1 明确 queued upload 可登记 FILE_BEGIN 但延后 ready；P2 冻结字段与帧长度口径；P3 将 Phase 1 接收限制为顺序 offset 并明确失败处理；P4 扩展文件任务执行身份；P5 固定超时、断线和提交确认边界。
- 与历史决定关系：补充 ADR-011/015；P3 正式取代 PROTOCOL.md 原来“发送端建议顺序发送、offset 支持乱序校验”在 Phase 1 的宽松表述，改为强制连续 offset。ADR-009/010/011/015 原文保留。
- 实施：按正式协议实现与测试 Phase 1D，全部验证后独立提交推送并停止等待验收。
