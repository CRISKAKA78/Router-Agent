# 异地组网改造与验证（ADR-066）

## 范围与结论

本轮在主工作区本地实现用户批准的新方案；没有提交/推送，没有上传或重启生产 Management Server、Probe、EasyTier，没有改动现有真实网络。本文日期按本机记录为2026-09-12（Asia 时区）。

1. 移除曾使用整网 busy 判断，任一成员未知操作会阻塞其他已停止成员；现改为目标成员检查。
2. 管理重启后暂态 Task 消失，被当成永远未完成；现在区分 pending 与 missing。仅当持久化步骤证明已进入配置应用/运行/配置确认阶段，才允许凭当前实际状态核实，保留原 Task IDs，不生成 RESULT。
3. 原正常路径只检查 running，而恢复路径比较配置；两者证据不一致。现正常/恢复都核实配置；新网络同时核实真实虚拟 IP 在目标网段内。
4. 官方 Web 的运行配置转换丢失 enabled-empty manual routes；使用同机官方 `ShowNodeInfo` 只读 RPC 的原始 TOML 补证。实测发现 dump 也会省略与默认值相同的 mtu/bind_device/multi_thread，按固定2.6.4的 `gen_default_flags` 语义补齐，而不是把任意缺失字段当成功。

## 已实现行为

- 新建：名称、自设/随机密码、按需显示/复制、虚拟网段、初始节点、自动ID、MTU1380。旧记录无 profile=2 时保留原默认与 UUID；显式编辑成员采用新版默认，有对应提示。
- 成员：多选在线未组网设备，首批必须指定固定地址成员，其他 DHCP 等待该成员确认。网段、主机地址/广播地址、重复IP及CIDR校验。最后固定成员不能在其他 DHCP 成员保留时删除/改 DHCP。
- 配置：成员独立修订、四种P2P标志、系统转发、代理CIDR与手动路由三态；已运行成员重建单实例，停止成员只保存。网络级修改仍按需显式应用，不自动重启整网。
- 身份与恢复：保留服务端已有 device→machine 映射和 instance ID；新映射兼容官方原始设备ID算法。成功配置过且 desired=start 的在线成员缺失引擎时，周期检查、60秒重试间隔、复用原安装/启动链路；健康实例不周期重应用，停止意图在持锁入口再次确认。持续未知操作不自动写重放。
- 操作：后台只读自动核实；停止可替代符合证据条件且工作线程已结束的未知操作，原记录保留并链接 superseded_by；旧线程迟到报告不能覆盖当前成员意图。
- WPF：左侧设备为观察设备，切换时清除旧快照并防迟到响应。保留全局网络选择、创建和添加；手工选择其他网络时显示没有该观察视角。11列成员表、中文NAT、非托管节点、实际协议/链路统计；中继端到端延迟缺失时显示未提供。
- 拓扑：路由器图标、观察设备中心布局、实际直连与路由证据的中继虚线、缩放/拖动/适应窗口。流向动画仅在采样计数递增时播放，隐藏/最小化/手动关闭时停止；不是实时流量测量。独立链路页移除，诊断详情折叠。

## 验证记录

### Go / Server

- Windows `go test ./cmd/... ./internal/... ./tests/...`、`go vet ./cmd/... ./internal/... ./tests/...`、Server build通过。Windows没有执行需要Linux真实Probe的分支；未把平台跳过当作实机成功。
- 收尾定向 `go test ./internal/overlay ./internal/management ./internal/api` 及完整 vet、Windows Server build另行执行，日志在 `build/overlay-redesign/go-targeted-final.log` / `go-windows-vet.log`。
- Linux在独立WSL `RouterAgentTest` 的 `/work-runs/overlay-redesign-20260912-051758` 源码副本中执行，不bind mount工作区。`unshare -mnpf --mount-proc` 隔离网络、PID、devpts；六包 overlay/management/gateway/task/api/filetransfer race、真实Probe组网断线/文件/仓库定向race、完整vet、Linux Server build通过（日志 `linux-race.log`，末次源码复测 `linux-race-final.log`）。Probe源码未改，复用既有同源码Linux测试Probe；本轮不重新宣称厂商双架构设备验收。
- 新回归覆盖：Rust哈希六组向量、现有UUID保留、新旧默认分离、密码不进DTO、批量固定成员、成员修订/单实例重建、停止配置不启动、缺失实例恢复与停止竞态、未知任务证据分级、其他成员不阻塞移除、迟到报告拒绝、空路由及默认旗标严格比较。

### 官方 EasyTier 真实二进制

- 官方发布ZIP：`easytier-linux-x86_64-v2.6.4.zip`，实际版本 `2.6.4-8428a89d`，与现场已核对版本一致。
- 运行 `TestOfficialEasyTierRuntimeConfig`：真实 Web + core、独立临时SQLite账号、仅测试命名空间的dummy网卡/默认路由。没有可达生产网卡；所有peer配置为空，未使用默认生产初始节点。测试自身关闭自己创建的进程。
- **通过**：`--machine-id FE7140555489` 注册为 `42a73850-8c63-39ad-a92a-63eec54941f6`；真实et0实例运行；ShowNodeInfo的selector/UUID/响应；完整新默认参数与原始配置核实；空手动路由、关闭手动路由、非空手动路由/代理/四种P2P设置的往返；实例停止确认。日志 `build/overlay-redesign/official.log` / `official-final.log`。
- 这里没有mock成功替代真实上游。最初几次失败属于隔离环境无可用物理地址、测试账号/配置目录准备不足；准备后真实比较捕获了上游“省略默认字段”的差异，已修复并新增反例回归。没有因此改生产配置。
- 上游证据：EasyTier `8428a89d` 的 `easytier/src/common/config.rs`（gen_default_flags/dump）、`easytier/src/proto/api_manage.proto`、`api_instance.proto`、`easytier-web/src/restful/rpc.rs`；操作只使用该版本公开能力。

复现可使用：

```powershell
python tests/overlay-official-seed.py build/overlay-redesign/new-test-seed.db
```

将此测试数据库和官方Linux二进制复制进独立WSL源码副本，设置绝对路径 `RMP_EASYTIER_TEST_DB`、`RMP_EASYTIER_TEST_BIN` 后执行 `sh tests/verify-overlay-official.sh`。种子仅含公开测试账号 overlay-test/admin 的Argon2哈希，不含用户凭据，拒绝覆盖已有数据库。脚本先创建独立namespace再配置dummy接口，不能对生产数据库执行。

### WPF

- `windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-networkv2`：完整**643项**通过，日志 `build/overlay-redesign/desktop-final-retry.log`。
- 一次全套运行在共享系统剪贴板的设备复制检查失败；没有删测/降低断言，完整重跑通过。该步骤与组网HTTP/引擎不在同一调用链。
- 末次界面微调（提示换行、旧成员默认显示、全局网络选择、停止替代历史入口）之后，独立 `--network-checks` **18项**通过并重发布，日志 `network-checks-final.log` / `publish-final.log`。支持该定向入口是为组网页验证，不替代上述全套记录。
- 已检查实际WPF离屏截图：浅色创建/批量成员、深色成员配置与拓扑；提示长文本换行修正。图片位于 `build/overlay-redesign/ui-network-final`。不是厂商现场屏幕/多显示器DPI最终验收。

## 产物与部署边界

- WPF：`build/windows-desktop-networkv2/win-x64/RouterWorkbench.exe`。
- Windows Server：`build/overlay-redesign/router-server.exe`。
- Linux AMD64 Server：`build/overlay-redesign/router-server-linux-amd64`（隔离构建后复制）。
- 上述均为本地构建产物，不纳入Git源码。用户原WPF进程未停止，也未替换其运行目录。
- **现网原未知操作/成员记录尚未由本轮处理**：需要另行授权部署新Server后，在既有数据备份基础上进行只读核实及真实设备验收。本轮不能声称现场已经恢复或成员已经移除。
- 仍待现场验证：两台厂商设备批量入网、掉电丢失临时引擎后的完整重新投放/恢复、真实NAT直连/中继与流量动效、MTU有效载荷及业务路由。已运行默认值不静默覆盖，二层、平台TLS/RBAC/对外发布仍不在本轮。
