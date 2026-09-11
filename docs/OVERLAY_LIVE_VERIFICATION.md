# EasyTier ARM—Server 实机联调

时间：服务器本地 2026-09-12（UTC+08:00；API 服务端时间戳为 2026-09-11 UTC）。设备时钟与服务端不同，不用 TASK 时间戳计算延迟。

## 授权与基线

- 用户授权同步 main、导入 `/root` 安装包，后续明确仅测 ARM 与 Server、允许重启 Management Server；20004 不再测试、不升级 Probe。ARM 最新 SSH 入口由用户从 20007 更新为 20001，设备 ID 始终为 `FE7140555489`；维护端口随管理会话重建可能改变。
- `git fetch origin` 后 worktree 快进至本地 main `bab2325`，远端无新增提交。主目录未提交的双架构构建/文档改动未动；本轮修复留在 `codex/overlay-network-research`，未提交、合回 main 或推送。
- 已按授权备份并更新 Management；未替换设备 Probe，未改 LAN、bridge、默认路由或其他 VPN。二层不在本轮范围。

## Repository

经本机公开 Asset/Tool/版本发布 API 导入，不直接修改 Repository 存储；原 ZIP 保留，提取唯一 easytier-core ELF，而不是把 ZIP 当可执行文件。

工具 `EasyTier core`，ID `e4c1561b-6cbb-4893-833e-37b78c8127fa`。首批 `2.6.4` 为 ARM/MIPS大端；因版本不可变，补充 mipsel 后发布包集合 `2.6.4-r1`，引擎版本仍为 2.6.4。

| 架构 | asset_id | 字节数 | SHA-256 |
| --- | --- | ---: | --- |
| ARMv7 LE | ecb206e2-49ab-4d66-baaa-7d9e51e362f7 | 5980748 | 7f1040fec526ded3990663aa19dabb028fea52a6c6b06ddc1de32668829934b9 |
| MIPS BE | be767b33-add5-4ce4-b807-daa910508db0 | 6536972 | 6f54cddf4a5e680bdbab3c343d479b2cd73eaa83cc8583e6046c157a152ac465 |
| MIPS LE | 8fe4bb62-62bb-44a3-beb3-79b7b90f0b00 | 6687000 | 2ae281fb79b9026a00a4c9e5bfce6c007f155d919881d64c2a91795011d3a245 |

r1 的 artifact_id：ARM `7fb71753-fef8-4f53-b4b3-952ffefbf1cc`、MIPS BE `4fd22d48-0334-4c2e-940f-268b605cf8cf`、MIPS LE `232cf342-7b4b-44db-9a2a-d7925493decd`。

三个 ELF 均检查字节序、机器类型、无 PT_INTERP/DT_NEEDED；匹配规则为 ARM `arm/armv7/armv7l`、大端 `mips`、小端 `mipsel`，libc 为 any。这不代表所有固件已运行验收。没有把大端包标成小端兼容。导入原幂等键保存在服务器 `/root/router-agent-overlay-import-20260912/operations.json`（目录0700、文件0600）。

## 实际接线与回退

- EasyTier Web：API本机11211，TCP配置服务22020。官方源码确认默认账号存在，实际登录200；暂用默认账号，未冒称新建了专用账号。
- `/root/agent-server/easytier.json` 为0600，start.sh新增 `-easytier-config`，Repository路径不变。密码字段采用官方前端发送的MD5十六进制表示，仍按密码保护，不输出或提交。
- 旧 Server/start.sh 位于 `/root/router-agent-overlay-import-20260912/server-backup/`；最终Server SHA-256 `ae992734371e3444a730b6d385e892c0c275ef80cd6f4742c174b63c772e7f55`，最终核验PID642512（不是永久配置）。
- 服务端独立 EasyTier core 2.6.4-8428a89d：目录 `/root/router-agent-overlay-test-20260912`，machine `14bd1038-d5d1-489e-92de-68eb570c7e5f`、instance `6c47ad47-34e1-42c4-9d10-bad7fdb81860`，核验PID641992；没有设置开机自启。
- 网络 `ARM20007-Server-L3-test`（名称保留创建时端口），ID `8c67e4e2-afdc-4eea-a2e6-e1a516455cf0`；Server `ettest0=10.144.144.1/24`、ARM `tun0=10.144.144.2/24`，实际TUN MTU均1360。测试静态IP不改变产品默认DHCP。
- ARM 经真实 inspect/prepare、Repository File投放、install/start 接入 `/tmp/root/router-agent-network`；machine `11d2a27d-a8bf-45a0-85ca-61f58b82337c`、instance `211c3ad2-413e-4a9c-b69d-c12f218a7b61`。
- 引导为本机公网TCP/UDP11010，未加公共中继；无WG监听，设备配置文件存在 `routes = []`。ARM默认路由仍经192.168.5.253/br0。

## 实测结果

| 检查 | 结果和边界 |
| --- | --- |
| ARM包ABI | File校验成功，真实执行返回 easytier-core 2.6.4-8428a89d；预检临时文件已删 |
| 双向ICMP | Server→ARM 3/3、ARM→Server 5/5；平均9.103/9.128ms，均0%丢包 |
| TCP业务 | 虚拟IP读取ARM SSH banner；另由ARM发起512字节TCP echo，Server确认源10.144.144.2及内容一致 |
| UDP业务 | 96/1332字节UDP echo均完整返回；不是把UDP transport当作业务UDP验证 |
| MTU样本 | ping -M do -s 1332 对应IPv4总长1360；首次3包丢1，复测10/10，平均10.816ms。不隐瞒首次丢包，不声称全路径/长期验收 |
| 管理中断保网 | SIGTERM停止Management，确认控制9000拒绝连接；停机至少12秒，跨窗口ping30/30，平均8.983ms；EasyTier PID不变，Management恢复后ARM自动在线，网络仍运行 |
| 拓扑API | 真实TCP/UDP观测边、RTT/计数及cost=1路由；Server为外部peer，不伪装为注册Probe或双边确认；未做本轮WPF屏幕交互验收 |

最初UDP DNS53无响应，不用它判定UDP；随后用临时自包含echo校验器获得明确证据。辅助ELF经Asset/File投放，执行后设备文件删除、临时Asset归档；仅绑定虚拟IP19090/19091的监听已关闭。运行网络保留。

本地忽略证据：`build/research-sources/icmp-test.log`、`payload-echo-test.log`、`disconnect-test.log`、`final-overlay-state.log`。服务器原任务和观察快照为同导入目录的 `connectivity.json`；辅助Go源码为 `build/research-sources/overlay-payload-check.go`，不是产品或EasyTier工具版本。

## 修复与回归

1. 原安装误要求上传 Snapshot.Committed。该字段仅是Server接收下载的本地提交；真实上传的TASK_RESULT已经Gateway按transfer ID/size/SHA-256校验。现在成功RESULT后等worker Released，错误/取消保持uncertain，没有改变File/Gateway校验。
2. EasyTier异步启用后首次观察可能还未running；现在最多20秒只读轮询，绝不重发save/enable，明确错误/超时/取消仍不强行成功。
3. 增加空手动路由回读丢失开关的负面回归，不能把null偷换为true。

Windows五包 `go test ./internal/management ./internal/overlay ./internal/filetransfer ./internal/gateway ./internal/api -count=1` 通过；异步等待修改后重跑overlay/management/api通过，相关vet及Linux AMD64 build通过。WSL当前源码五包 `go test -race ... -count=1` 全通过，路径见 `build/research-sources/live-fix-linux-run.txt`。首次测试归档漏go.sum导致API依赖检查失败，补复制仓库原go.sum后全量重跑五包通过，未修改依赖。

Probe/WPF未改，本轮未重跑全量C++/桌面构建，不借历史验收冒充本轮结果。异步等待修复已部署，但没有重发原uncertain操作制造新的实机成功记录。

## 未完成与交接注意

- 原操作 `aab4d8da-8175-4f61-80a9-bf24b65749f4` 仍uncertain、applied_revision=0：原任务确实形成可通信数据面，但第一次回查过早。上游2.6.4 launcher.rs仅对非空routes回读enable_manual_routes，实际开启且空列表会返回null；不能经此API完整确认原配置。未改库、重放或伪造确认，后续需解决上游回读/额外可信证据，UI启停/回查闭环尚未完整验收。
- Management重启不实现跨进程任务恢复；不能制造旧task_id完成记录。设备/tmp安装也不等于永久安装或重启恢复保证。
- mipsel已入库，但按最新要求不测试20004、不升级其Probe。双路由器/中继、长期吞吐、完整MTU、二层/LAN仍未验收。
- 安全待确认：EasyTier Web API监听所有地址，默认账号仍有效；已询问回环绑定、修改默认密码和重启Web的许可，当前未擅自改动。不把实验接线称为安全生产部署。
