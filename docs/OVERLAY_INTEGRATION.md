# 异地组网工作树审核与主分支集成

## 完成结论

审核发现已作必要局部修复，合并后验证通过；本地main已形成包含原主分支及来源分支两个父提交的合并提交（确切ID以Git为准）。工作区干净，未推送或部署。

## 范围与来源

用户明确要求审核 `router-agent-overlay-network` 并接入主分支。来源工作树 `codex/overlay-network-research` 原有46项源码/文档变更已保存为 `da6c560`，基于 `f331ffb`；目标基线为已包含AT、智能邻居、日志和GOST PoC的 `c93235d`。本次仅本地集成，不推送、远端构建、上传、配置上游账号或替换运行实例。原工作树与忽略的验证/凭据文件保留。

## 审核结果与修复

- 完成范围是EasyTier三层首轮产品管理链路：公开API → Management/overlay Service → 本机配置服务；Probe沿用Repository/File安装并启动独立引擎，业务数据不进入原管理控制通道；WPF全局工作区提供成员/拓扑/链路/操作记录。未配置EasyTier不阻止原Server和其他工作区使用。
- 回查配置确认后未更新成员 `AppliedRevision`：已补同步，操作状态仍为reconciled而非伪造历史成功。
- 已回查确认停止的成员仍被 `RemoveMember` 拒绝：允许succeeded或reconciled的stop操作移除，未确认/仍运行仍拒绝。两项缺陷先以回归复现失败，再修复通过；回查未重发Apply/Stop，未终态旧任务仍阻止回查放行。
- Probe `network_agent` 已受理时ACK误附带unsupported消息：将其加入消息判断白名单，不改变任务调度/容量/幂等。
- 13处文件内容冲突按能力并集处理：API路由、Server初始化、Probe任务解析/调度/REGISTER、WPF导航/图标/测试和状态文档。保留AT/邻居/日志全部能力及GOST独立PoC边界，不采用整文件覆盖。
- EasyTier原分支ADR-059统一为ADR-064，避免与智能邻居冲突；原059～063含义及Accepted内容保持。历史“未提交/独立工作树”授权说明加本次合入注记。
- 既有默认管理服务器/端口、GCC5.2与GCC5.4构建入口、生成器、维护页面业务及凭据规则没有被回退。

## 界面与产物

新程序：`build/windows-desktop-overlaymerged/win-x64/RouterWorkbench.exe`；同目录的兄弟目录 `verification/router-server.exe` 为本次桌面测试Server。未改旧快捷方式或正在运行的程序。

“异地组网”为第七个全局工作区，日志保留；入口不按设备能力隐藏。添加设备要求在线/已纳管、`network_agent_v1`、不在其他网络及配置服务接线。新增真实Server/WPF检查覆盖未配置仍能启动、读取settings、禁止加入和共享连接回查网络定义。

## 本次验证

证据目录：`build/integration-overlay/`。以最终日志为准，来源工作树历史通过不替代本轮结果。

- Windows：`go test ./cmd/... ./internal/... ./tests/... -count=1`及`go vet`通过；回查缺陷红/绿证据为reconcile-before.log、stop-reconcile-before.log、reconcile-after.log。
- WPF：自包含publish通过；合并首轮Desktop.Tests 612项通过，81份XPS布局用render.py验证并渲染。新增4种组网布局使用内容/节点/连线断言，不降低既有页面的通用断言。已查看窄窗口七项导航与组网空态。
- 增补5项真实Server/WPF接线检查后，两次全量复跑分别在原有属性复制和设备右键复制的共享剪贴板断言失败；未修改/跳过或降低其断言。相同代码/断言的最终复核617项全部通过（desktop-confirm.log），对应81份布局渲染通过（render-confirm.log）；保留前述失败证据，不宣称未发生波动。
- Linux：隔离RouterAgentTest WSL mount/network/PID/devpts/sysfs，18项CTest及完整Go/真实Probe集成通过（integration 292.038秒），vet及Server构建通过。脚本与Linux副本位置记录在linux-run.sh、linux-path.txt；EasyTier独立进程用例使用明确标注的引擎替身，不是VPN数据面验收。
- 最终Probe ACK小修复与新增network_agent_v1共存断言同步后，重新构建并通过18项CTest；overlay/management/gateway/task/api五包完整race通过；真实Probe AT/日志/邻居/组网能力共存及独立引擎重放/断线用例race通过（44.605秒，无跳过）。最终vet再次覆盖cmd/internal/tests通过，见linux-final.log、linux-vet-final.log。临时脚本末尾CR曾使一次vet的tests通配失配，已修正LF并单独重跑完整vet，不将该警告当通过。

## 真实遗留与未运行范围

- 上游官方配置服务/专用账号、设备可达配置入口和引导端点、Repository正确架构ELF包仍需部署接线。
- 本轮不执行GCC5.2/GCC5.4厂商构建或两台路由器VPN安装/互通、直连/中继/恢复/MTU测试，不自动启用接口、改LAN桥接或默认路由。
- 二层VXLAN/GRETAP/EoIP尚未实现，必须按已接受顺序先完成三层实机；真实物理DPI和sanitizer本轮未追加。
- 生成器代码/构建入口未改，本轮不重跑其全套测试；GOST仍保持原PoC状态与原压力/预算缺口，不借本次EasyTier接入扩充。
