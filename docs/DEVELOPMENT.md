# Router-Agent 需求开发指南

本指南把 [AGENTS](../AGENTS.md) 的治理规则落实为日常工作流程。架构、业务语义与接口分别以 [ARCHITECTURE](ARCHITECTURE.md)、Accepted [DECISIONS](DECISIONS.md)、[API](API.md) 和 [PROTOCOL](PROTOCOL.md) 为准；本指南不覆盖这些规范。

## 用户描述体验，Agent 完成技术闭环

用户可以只说“任务列表增加按设备筛选”“切换设备后不要出现上一个设备的结果”或“上传失败时让我知道下一步怎么做”。无需填写需求模板或指定技术栈、目录、接口、测试清单。

Agent 先从仓库查明已有行为，将需求整理为简短的可观察结果：从哪里操作、何时生效、成功/失败如何呈现，以及与已有行为的关系。普通实现选择自行完成；只有仓库不能决定且会影响结果的关键歧义才询问用户。需求已清楚时，说明范围后直接实施，不以等待方案批准结束任务。

当前方向为在既有产品基线上持续完善，不推进 Phase 7/8。Phase 6 实机与最终产品验收仍独立保留；一次功能交付不等于整阶段验收。治理中的“Agent”指开发协作者，不代表实现 Router-Agent 的 AI Agent 产品能力。

## 需求分流

| 情况 | Agent 的处理 |
| --- | --- |
| 已有产品范围内的功能、行为调整、缺陷修复，符合 Accepted 设计 | 自主定位、实现、验证、同步文档；必要跨层改动属于同一需求 |
| 仅缺普通实现细节 | 复用现有约定，采用最小合理假设并说明；不让用户选择代码组织方式 |
| API 存在必要小缺口 | 先确认已有查询/用例不能满足；在既有业务语义和 API 扩展规则内作兼容补充，同步 Service、Adapter、客户端 DTO、契约及测试，不额外扩展协议或核心状态机 |
| 关键产品行为无法从需求和仓库判断 | 给出具体歧义、推荐行为及体验差异，提出简短问题；继续不依赖答案的部分 |
| 改变 Accepted ADR、协议/架构基线、公开兼容性、数据持久化/身份/生命周期等已确认语义 | 先记录具体问题、方案与影响；确认后新增 superseding ADR 并实施，不静默改写历史决定 |
| 涉及暂缓能力或其他大规模架构扩展 | 说明与当前边界的冲突，提出范围内替代方案或请求明确调整范围；确认前不实施依赖部分 |
| 只读审查、咨询 | 返回发现或答案；不为完成文档清单制造修改 |

暂缓项：Phase 7 MCP、Phase 8 AI Agent、微信小程序、正式公网 Web 部署、新 Tunnel 数据面、其他大规模架构扩展。认证/TLS/RBAC、租户、完整审计、跨重启恢复等未决主题不能作为普通功能的附赠实现。新协议消息、通用端口转发或任意目标端口不能包装成“最小 API 补充”。

## 执行顺序

1. 按 AGENTS 必读顺序接管，检查 `git status --short`、当前提交、相关代码和测试入口。保留已有改动、运行数据、用户进程；记录会影响任务的初始问题。
2. 从用户操作追踪到页面、公共 API、Application/Service 及必要的 Gateway/Probe，定位事实来源。列出本次完成条件、必要范围、已确认约束与会影响实施的 TBD。
3. 复用现有能力完成最小完整闭环。局部重构、调用方、错误处理、取消释放与测试调整以完成需求的实际需要为限；不要仅为将来扩展创建框架、依赖或目录。
4. 按下面的验证表选择检查，先验证直接行为，再覆盖受到影响的调用链。只在新改动、失败或未解决风险出现时扩大/重复检查。遇到环境限制，记录未通过的范围与原因，不伪造成功。
5. 检查最终差异与完成条件，同步必要文档，给出结果和验证证据。提交/推送/部署以当前授权为准；完成当前需求后停止，不自行安排下一阶段。

## 模块导航

| 需求落点 | 首先检查的现有实现 | 约束与验证入口 |
| --- | --- | --- |
| 页面、列表、表单、错误与交互 | [App.tsx](../frontend/src/App.tsx)、[ui/](../frontend/src/ui/)、[useWorkbench.ts](../frontend/src/useWorkbench.ts)、[useQuery.ts](../frontend/src/useQuery.ts) | [UI_FREEZE](UI_FREEZE.md)；生产页在 ui/，preview/ 仅作冻结参照；沿用中文 Fluent 与主题 |
| HTTP、WebSocket、连接/选择隔离 | [api.ts](../frontend/src/api.ts)、[models.ts](../frontend/src/models.ts)、[connection.ts](../frontend/src/connection.ts) | API、ADR-023/026；首连/重连 HTTP 快照，原幂等请求，切换/退出取消并等待；api/connection 测试 |
| Windows 平台、文件选择保存、终端 | [platform.ts](../frontend/src/platform.ts)、[RouterWorkbench/](../windows/RouterWorkbench/)、[RouterWorkbench.Core/](../windows/RouterWorkbench.Core/) | ADR-026/027、[PHASE6_DESIGN](PHASE6_DESIGN.md)、[Windows README](../windows/README.md)；受限 Bridge、来源校验与资源回收 |
| 服务端用例和公开接口 | [internal/api/](../internal/api/)、[internal/management/](../internal/management/)，再到对应 device/task/repository/filetransfer/tunnel 服务 | Adapter 只调用 Application/Service，不能直达连接表或存储表；各包测试与 [tests/integration/](../tests/integration/) |
| Probe、控制/文件链路、现有维护缺陷 | [probe/](../probe/)、[internal/gateway/](../internal/gateway/)、[internal/protocol/](../internal/protocol/) 及相应服务 | PROTOCOL、ADR-009～023；C++11/轻量、幂等、固定三入口与独立数据 TCP；Go 集成和 Probe CTest |

客户端不复制服务端状态机或兼容性判断。不确定派发不自动新建任务；文件完整提交/释放不等于最终 RESULT 成功。UI 没有真实数据源时如实显示未提供。新增普通交互沿用冻结视觉；改变主页面布局或视觉语言须确认，不能把重新设计作为普通功能前置条件。

## 按影响范围验证

以下为最低选择依据，跨层改动组合适用项；命令先核对仓库脚本与本机环境。测试数量是历史事实，不能硬编码成以后交付的固定目标。

| 改动范围 | 必要验证 |
| --- | --- |
| 纯文档/治理 | `git diff --check`，差异范围、相对链接/路径及规则一致性检查；无需产品构建或业务测试 |
| 文案、样式、普通 UI | `npm --prefix frontend run build`，受影响页面实际操作、错误/空态和适用主题/窗口检查；有行为逻辑时执行/补充相关前端测试，不为静态文案写镜像测试 |
| 前端状态、API Client、异步操作 | 前端 build、`npm --prefix frontend test` 与有针对性的回归；验证重复点击、错误、不确定响应、断线/重连、切换时旧结果隔离等受影响场景；涉及平台联动时加真实 WebView2 |
| Windows Shell、Bridge、文件保存、终端、发布 | 原生构建、策略/资源检查、真实 WebView2 及受影响的退出/到期/Session 替换；完整入口为 `windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify`，包括原生和正式包启动检查。终端交互变化另执行隔离真实 SSH/Telnet 往返，见 PHASE6_VERIFICATION |
| Go 服务、HTTP/WS API | 受影响包及调用方集成测试、`go vet`、适用 Server build；并发/生命周期变化加 `go test -race`；公开契约变化覆盖旧客户端兼容、DTO、错误/幂等与 HTTP/WS 闭环。跨平台路径覆盖 Windows/Linux |
| Probe、Gateway、协议实现、文件/现有 Tunnel 生命周期 | 保留并运行 Phase 1～5 全量回归、Go race/vet、C++ CTest/sanitizers、真实 Linux Probe 与 Windows/Linux Server 适用验证；已确认设计变更仍须先走确认流程 |
| 跨层发布或阶段整体验收 | 覆盖完整 Windows 产品闭环及 Phase 1～5 回归；将集成对端、隔离服务、本机发布和用户实机结果分别记录 |

Phase 1～5 完整入口：[tests/verify-phase5.sh](../tests/verify-phase5.sh)，在满足依赖的隔离 Linux 环境分别执行 `release`、`asan`、`race`，包含 CTest、Go 集成/竞态、ASan/UBSan/TSan。真实 80/22/23 服务测试使用隔离网络/devpts，不能占用用户生产服务。Windows 的 Go 通过不能替代 Linux Probe 集成，未设置真实 Probe 对端而跳过的用例不能记作通过。

Windows 的 [build.ps1](../windows/build.ps1) 会恢复前端依赖并构建验证程序；[verify-native.ps1](../windows/verify-native.ps1) 依赖这些构建产物，不能假定单独运行即完成全部准备。真实终端测试使用 [terminal-test.Dockerfile](../windows/terminal-test.Dockerfile) 与 [verify-terminal-services.ps1](../windows/verify-terminal-services.ps1)，Docker 仅为测试依赖。命令、环境和证据详见 [PHASE6_VERIFICATION](PHASE6_VERIFICATION.md)；本机曾使用 `build/dotnet/dotnet.exe`，新环境须先检查 SDK。

完整 Windows 产品闭环包括连接/断线/重连、设备实时更新/Session replacement、Maintenance 创建/关闭/默认与自定义租期/到期/Web-SSH-Telnet 三入口、Exec 最终结果、File/Tool、API 错误、重复点击/并发以及切换/退出资源释放。普通变更选择相关场景，整体验收覆盖全部；不删除既有回归要求，也不把每次文档编辑变成整阶段验收。

## 文档与交付

每次有效开发同步 [PROJECT_STATUS](PROJECT_STATUS.md) 的当前结果/验证/缺口、[HANDOFF](HANDOFF.md) 的接管要点和 [ROADMAP](ROADMAP.md) 的实际进度。状态保持简短，不累计逐轮测试流水账。

架构、API、Protocol 仅在相应边界或契约改变时更新；重要设计新增 ADR，普通实现选择不强制写 ADR。重要治理变化和已形成的用户可见/对外契约变化写 [CHANGELOG](../CHANGELOG.md)。专项验证文档记录受影响证据，保留历史范围与尚未完成的实机验收。

交付用短段落或列表说明完成行为、相关文件、实际验证、尚未解决的问题。没有遗留问题无需制造“风险”；没有提交、推送或实机测试就不声称已经完成这些动作。

## 分流示例（说明规则，不是待办或已实现能力）

- “任务列表按设备筛选”：先查 API 已有 `device_id` 过滤和现有页面；复用查询与分页，处理空态/选择变化并验证，无需用户指定接口。若已有行为已满足，指出入口并核实用户期望差异。
- “切换设备后仍出现上一个设备结果”：检查 connection、查询取消和选择隔离，修复受影响链路并覆盖迟到响应，不据此重写所有状态管理。
- “上传失败时更好理解”：沿用稳定错误码和中文提示，必要时补兼容的错误映射并验证；不因改善提示自行增加跨重启重试或自动创建替代任务。
- “维护入口支持任意端口”：触及固定三入口与暂缓范围；给出具体影响和范围内选择，确认设计变更前不实现。
- “把当前界面发布到公网”或“增加 MCP/AI 自动排障”：说明属于暂缓范围，先取得明确范围调整；不因共享 React 或仓库治理使用 AI 就开始对应产品能力。
