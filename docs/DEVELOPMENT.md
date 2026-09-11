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

ADR-037 之后只维护新版 WPF 主工作台与 Blazor 模板生成器，旧 React/WinUI/Win32 UI、Bridge/ConPTY 与专属脚本已经移除。旧设计文档仅用于回溯，不作为当前构建入口。

| 需求落点 | 首先检查的实现 | 约束与验证入口 |
| --- | --- | --- |
| Windows 页面、列表、表单与生命周期 | [RouterWorkbench.Desktop](../windows/RouterWorkbench.Desktop)，DeviceViews/DeviceProperties、MaintenanceViews、FileTransfers、RepositoryViews、SettingsView、MainWindow 与 Themes | 原生 WPF，中文浅深/系统主题；[WINDOWS_DESKTOP_MIGRATION](WINDOWS_DESKTOP_MIGRATION.md) |
| HTTP/WS、快照、幂等与取消 | [RouterWorkbench.Client](../windows/RouterWorkbench.Client)，ApiClient、Models、WorkspaceConnection | 只使用公开 API；首连/重连回查 HTTP，原请求键与字节，切换/退出取消并等待 |
| 本机偏好、外部 SSH/Telnet、文件选择 | [RouterWorkbench.Core](../windows/RouterWorkbench.Core)、Desktop 的 SettingsView/RepositoryViews | 无 WebView2 Bridge 或内置终端；端点复核，不保存 SSH 凭据，外部客户端由用户管理 |
| 生成器编辑、公式、规则、导入导出与发布 | [ProbeTemplateGenerator](../src/ProbeTemplateGenerator)，Features、Models、Services、Persistence | 强类型 C#，原模板 API，版本 1/2 与旧草稿兼容；[TEMPLATE_GENERATOR_MIGRATION](TEMPLATE_GENERATOR_MIGRATION.md) |
| 服务端用例和公开接口 | [internal/api](../internal/api)、[internal/management](../internal/management) 及对应服务 | Adapter 只调用 Application/Service；各包测试与 [tests/integration](../tests/integration) |
| Probe、控制/文件链路与维护 | [probe](../probe)、[gateway](../internal/gateway)、[protocol](../internal/protocol) | C++11、既有幂等、固定三入口、独立数据 TCP；PROTOCOL 与 Phase 1～5 验证 |

客户端不复制 Server 状态机或兼容性算法。文件 committed/released 与最终 RESULT 分开，未知遥测显示未提供。普通功能沿用当前 WPF/Blazor 视觉，不把整体重设计作为前置条件。

## 按影响范围验证

命令中的 dotnet 需要 .NET 10 SDK；本机可用 `build/dotnet10/dotnet.exe`，入口脚本也支持 `-Dotnet`。测试数量是各次验证事实，不能固定成未来交付目标。

| 改动范围 | 必要验证 |
| --- | --- |
| 纯文档/治理 | `git diff --check`，差异、相对链接与规则一致性，无需产品构建 |
| WPF 文案、样式与普通 UI | `windows/build-desktop.ps1 -BuildOnly`；受影响页面、错误/空态、主题/窗口检查；有行为时加相关回归，不为静态文案写镜像测试 |
| C# Client、Core、WPF 状态、文件保存或发布 | `windows/build-desktop.ps1 -BuildOnly -Verify`，构建当前 Go Server 并运行 Desktop.Tests；覆盖受影响的错误、不确定响应、断线/重连、重复操作、切换/退出与发布 EXE 启停 |
| Blazor 编辑、编译、持久化或发布 | `template-generator.ps1 -BuildOnly`、`dotnet test ProbeTemplateGenerator.sln -c Release`；命令兼容测试设置 `RMP_GENERATOR_WSL=RouterAgentTest`。UI/发布链路变化增加下述浏览器流程 |
| Go 服务、HTTP/WS API | 受影响包及调用方集成、`go vet` 与 Server build；并发/生命周期变化加 race，公开契约覆盖兼容/DTO/错误/幂等 |
| Probe、Gateway、协议、文件/Tunnel 生命周期 | Phase 1～5 全量回归、Go race/vet、C++ CTest/sanitizers、真实 Linux Probe 及 Windows/Linux Server；未提供真实对端而跳过的用例不能记为通过 |
| 跨层发布或阶段验收 | 完整当前 Windows 产品闭环及 Phase 1～5 回归，区分测试协议对端、隔离服务、本机发布与厂商实机 |

生成器浏览器验证依赖独立保存在 [tests/package.json](../tests/package.json) 和锁文件，先执行 `npm.cmd --prefix tests ci`，只安装已有 Playwright 测试依赖。启动本机 Blazor 后，设置 `GENERATOR_URL`、`RMP_SERVER_BIN`（当前 Go Server EXE），执行 `npm.cmd --prefix tests run test:generator`。默认使用本机 Edge，可通过 `RMP_BROWSER_CHANNEL` 改变已安装的浏览器；`RMP_GENERATOR_OUTPUT` 可设置隔离结果目录。Node 不是两套产品的构建或运行依赖。

Desktop.Tests 的 TestProbe 为测试专用协议对端，不进入产品。测试输出 WPF XPS；`windows/RouterWorkbench.Desktop.Tests/render.py` 使用 Python/PyMuPDF 转为 PNG，属于实际控件矢量布局，不等于物理屏幕/DPI 验收。

共享控件迭代可运行 `build/dotnet10/dotnet.exe run --project windows/RouterWorkbench.Desktop.Tests -c Release -- --components build/component-qa` 打开真实WPF样板；将 `--components` 换为 `--component-checks` 可执行输入/选择/弹出层/字体几何专项检查。完整桌面回归也包含这些检查。对验证目录运行 `python windows/RouterWorkbench.Desktop.Tests/component-density.py <验证目录>` 输出100/125/150/200%离屏密度图；它只检查真实WPF矢量在不同栅格密度的表现，不能代替物理显示器或跨屏DPI验收。组件视觉必须以[方案3](design/property-inspector-target.png)和实际程序截图联合比较，规范见[COMPONENT_VISUAL_SPEC](COMPONENT_VISUAL_SPEC.md)。

Linux 测试复用项目外 WSL 2 `RouterAgentTest`，复制当前源码，在 network/devpts namespace 隔离运行；见 [WSL_TEST_ENVIRONMENT](WSL_TEST_ENVIRONMENT.md)。不将工作区 bind mount 进测试 rootfs。完整入口 [tests/verify-phase5.sh](../tests/verify-phase5.sh) 支持 release/asan/race，真实 80/22/23 服务不能占用用户生产端口；缺失运行库如实记录，不降低断言。

当前 WPF 完整闭环包括连接/断线/重连、设备属性与 Session replacement、维护创建/关闭/默认和自定义租期/到期/外部三入口、文件传输与配置结果、API 错误和不确定请求、重复操作与切换/退出释放。通用任务/工具入口已按 ADR-036 移除，其公开 API 与业务集成仍保留。

## 文档与交付

每次有效开发同步 [PROJECT_STATUS](PROJECT_STATUS.md) 的当前结果/验证/缺口、[HANDOFF](HANDOFF.md) 的接管要点和 [ROADMAP](ROADMAP.md) 的实际进度。状态保持简短，不累计逐轮测试流水账。

架构、API、Protocol 仅在相应边界或契约改变时更新；重要设计新增 ADR，普通实现选择不强制写 ADR。重要治理变化和已形成的用户可见/对外契约变化写 [CHANGELOG](../CHANGELOG.md)。专项验证文档记录受影响证据，保留历史范围与尚未完成的实机验收。

交付用短段落或列表说明完成行为、相关文件、实际验证、尚未解决的问题。没有遗留问题无需制造“风险”；没有提交、推送或实机测试就不声称已经完成这些动作。

## 分流示例（说明规则，不是待办或已实现能力）

- “任务列表按设备筛选”：先查 API 已有 `device_id` 过滤和现有页面；复用查询与分页，处理空态/选择变化并验证，无需用户指定接口。若已有行为已满足，指出入口并核实用户期望差异。
- “切换设备后仍出现上一个设备结果”：检查 connection、查询取消和选择隔离，修复受影响链路并覆盖迟到响应，不据此重写所有状态管理。
- “上传失败时更好理解”：沿用稳定错误码和中文提示，必要时补兼容的错误映射并验证；不因改善提示自行增加跨重启重试或自动创建替代任务。
- “维护入口支持任意端口”：触及固定三入口与暂缓范围；给出具体影响和范围内选择，确认设计变更前不实现。
- “把当前界面发布到公网”或“增加 MCP/AI 自动排障”：说明属于暂缓范围，先取得明确范围调整；不因本机 Blazor 工作区或仓库治理使用 AI 就开始对应产品能力。
