# 三工作树合入与联合验证

## 范围与来源

本次用户明确授权三个工作树全部合入本地main，AT前置的智能邻居也一并保留；只解决集成冲突，不改变既有功能逻辑。GOST以已完成的PoC源码、独立鉴权模块和补丁合入，后续实机测试由用户另行执行。本次不推送、不远程构建、不上传/部署、不替换用户运行实例。

| 来源 | 本次保存提交 | 基线/包含范围 |
| --- | --- | --- |
| main | f331ffb | 默认服务器及原Probe构建入口 |
| codex/at-discovery | 57e04db | 包含7671b5f智能邻居，以及AT/GCC5.4入口 |
| codex/device-logs | 613b622 | 日志完整未提交源码及测试 |
| codex/gostv3-device-poc | 3d6ba21 | 包含5ca179a及后续鉴权/补丁/设备PoC记录 |

已在codex/integrate-at-logs-gost集成：3792410（AT/邻居）、e47d668（日志）、ff51e74（GOST）。联合验证及最终提交完成后，已以fast-forward合回本地main；最终提交与可达关系以Git为准。原工作树和忽略的凭据、运行数据、构建包全部保留，不纳入提交。

## 冲突处理

- Probe REGISTER保留neighbors_v1、neighbors_inspect_v1、cellular_identity_v1、device_logs_v1；TASK准入/分发并列保留neighbor_inspect与device_logs，CMake同时纳入两套新增测试。
- Gateway并列处理cellular、device_log_reply及既有EVENT，不让一方覆盖另一方。HTTP错误响应保留field/details与自定义message，并增加组合回归。
- WPF/测试对端保留日志导航、AT页、智能邻居扫描/近期记录及各套回归；邻居页采用新智能操作控件但不恢复main已移除的旧重复概述。生成器保留智能/高级配置及AT编辑。
- main的47.119.168.150默认值、全接口监听、无SSH凭据配置、维护失效隐藏地址、/tmp/root目录和GCC5.2默认入口保持，GCC5.4新增入口独立存在。
- ADR编号仅消除碰撞：main057/058保留，智能邻居059、AT060、日志061、GOST提案062/鉴权063；原分支编号可在上述来源提交追溯。
- GOST补丁文本保留原始内容与LF；.gitattributes只对该目录的统一差异文件关闭普通文本空白告警（补丁上下文行本身具有必要空白），不修改补丁内容或放宽产品测试。
- 127个只有单一来源修改的实现/脚本文件与来源内容一致；额外变化仅为新增的集成回归。多来源冲突按上述功能并集处理。

## 本次验证

环境：Windows、仓库内.NET10 SDK；Linux复用RouterAgentTest WSL，当前源码复制到`/work-runs/integration-at-logs-gost-20260911-235932`，在独立mount/network/PID/devpts namespace内运行，并重新挂载sysfs。不触碰生产80/22/23端口或主机设备节点。

证据根：`build/integration-at-logs-gost/`（忽略目录）。

| 命令/入口 | 本次结果 | 证据 |
| --- | --- | --- |
| go test ./cmd/... ./internal/... ./tests/... -count=1（Windows） | 通过；Linux专属测试按平台不运行 | windows-go.log |
| go vet ./cmd/... ./internal/... ./tests/...（Windows） | 通过 | windows-vet.log |
| windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-integrated | 自包含发布通过；首轮测试在上下文复制剪贴板断言失败，未改代码或断言重跑通过605项 | windows-desktop.log、desktop-retry.log |
| dotnet run --project windows/RouterWorkbench.Desktop.Tests -c Release -- <当前Server> <隔离输出> | 605项通过；AT、邻居、日志均纳入同一轮 | desktop-retry.log、desktop-retry/ |
| python windows/RouterWorkbench.Desktop.Tests/render.py <隔离输出> | 77份实际WPF矢量布局转换/内容检查通过；抽查AT/日志/邻居画面，非物理DPI验收 | wpf-render.log |
| RMP_GENERATOR_WSL=RouterAgentTest dotnet test ProbeTemplateGenerator.sln -c Release | 196通过、0跳过 | generator-tests.log、integrated-generator.trx |
| template-generator.ps1 -BuildOnly | 通过，0警告/0错误 | generator-build.log |
| python tests/probe_build_gcc54_test.py | 4项通过；不登录编译机 | gcc54-script-tests.log |
| node --check tests/template-generator-browser.mjs | 通过；真实浏览器启动命令被执行策略拒绝，未运行场景 | 不声称浏览器通过 |
| sh tests/verify-phase5.sh release（WSL隔离） | 17项CTest、真实Linux Probe全量（291.801秒）、vet及Linux Server构建通过 | linux-release-race-final.log |
| RMP_PROBE_BIN=<本次Probe> go test -race ./cmd/... ./internal/... ./tests/... -count=1 | 全范围通过，真实Probe集成293.660秒 | linux-release-race-final.log |

新增真实Probe回归在AT自动采集期间，通过公开HTTP请求历史日志，检查相同Session的EVENT往返、所有新增能力及后续AT重编号。首轮新增测试误按data.files解包，而既有契约是data.value.files；已仅修正测试解包并增加Session检查，不更改产品信封或降低断言。首轮记录保留在linux-release-race.log。

最终核对：63个ADR标题唯一；新增12个相对文档链接均存在；11个main默认值/构建/维护关键文件与f331ffb完全一致；git diff --check通过。三个来源提交及AT前置7671b5f均可从集成结果追溯。

## 产物与未完成项

- Windows：`build/windows-desktop-integrated/win-x64/RouterWorkbench.exe`。Windows测试Server：`build/windows-desktop-integrated/verification/router-server.exe`。
- Linux验证产物已复制到`build/integration-at-logs-gost/router-server-linux-amd64`及`router-probe-linux-amd64`；为动态链接musl的Linux AMD64验证产物（解释器/lib/ld-musl-x86_64.so.1），不当作通用发行包或MIPS/ARM厂商设备产物。
- GOST本次只随Go模块编译/单元/race回归，不运行设备PoC。原45项44通过、并发大UDP/资源预算、真实UART/不同网关LAN/长稳缺口保持；原证据仍在原GOST工作树，见GOST_V3_POC。
- 厂商设备新组合版本、MIPS/uClibc构建、物理DPI、生成器真实浏览器及C++sanitizer未在本次验收；既有缺库约束不因Go race通过而消失。Phase6最终产品验收不变。
