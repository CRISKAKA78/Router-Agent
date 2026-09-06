# 变更记录

本文件记录：

1. 已形成的用户可见产品行为变化。
2. 对客户端或开发者具有外部意义的协议或 API 契约变化。
3. 重要项目基线或治理规则变化。

本文件不记录普通内部重构、未完成计划、虚构功能或单纯开发过程流水账。

## Unreleased

### Fixed

- Phase 1E：Server 严格拒绝非法 Unicode 与字符串形式 load1；Probe 忽略未知扩展字段中的合法大整数，已知 uint64 字段仍拒绝溢出。
- Probe 在 REGISTER_ACK 缩小帧上限后立即复核已缓冲 Header，防止等待超限 Payload；未确认心跳记录设为每连接最多 1024 项，满后沿用既有重连流程。
- Server 文件 worker 结束时释放残留文件块邮箱；入站 message_id 耗尽时结束连接，避免回绕为 0。
- Server 成功写出 REGISTER_ACK 后才发布可派发会话，修复注册/重连期间 TASK 抢先发送的问题。
- Server 传输写失败立即关闭并废弃连接，防止部分帧后续写入与 message_id 重用；CreateExec 对发送结果不确定的任务保留记录并返回 task_id 与 ErrDispatchUncertain。
- Probe exec 不再继承控制 socket；fd 的 close-on-exec 设置与 fork 使用统一同步。
- Probe timeout/worker 停止完整执行进程组 TERM、200 ms grace、KILL；后代持有 pipe 时有界排空并设置 truncated，防止 worker 因 EOF 等待挂住。

### Added

- Phase 4 新增极简自研TCP Maintenance Service：一次创建Web/SSH/Telnet临时入口，固定访问设备127.0.0.1:80/22/23；默认240分钟并允许自定义租期。独立data TCP与一次性token配对，保持原始字节、并发连接、half-close和有界背压。
- Maintenance严格绑定创建时Device Session，主动关闭、到期、替换或失联撤销入口并终止活动连接；等待Server相关资源释放后才归还端口。Server提供端口池、advertised地址、超时与分层连接限额配置，Probe保持C++11且没有新增重型依赖。
- 新增控制类型0x40～0x42与RMT1独立data握手；以Accepted ADR-021取代未交付FRP/xfrpc路线。不引入HTTP/WebSocket API或通用端口映射。

- Phase 3 新增持久化 File / Tool Repository 与内部资产查询、导入、归档、工具版本/产物发布和兼容查询。tool_id、asset_id、artifact_id 为稳定 UUID，artifact_id 全仓库唯一；版本不可变，重复内容共享 SHA-256 blob，归档保留身份与文件。
- 新增内部工具投放、资产上传和设备下载显式导入，复用既有文件任务；受限资料缺失返回 unknown，不自动投放或执行工具。保留不确定派发的任务身份，下载可分别呈现本地完整提交和最终失败结果。
- Server 支持 `-repository-dir`，默认 `./data/repository`，采用单进程目录锁与本地 JSON 元数据；Linux/Windows 无新增外部服务依赖。仓库数据跨重启保留，设备/任务/Session 不恢复。

- Phase 2 新增 Device Service / Inventory 内部 List/Get/Sessions 查询，保留 REGISTER 基础资料和 capabilities、设备在线状态、当前/最近 Session 及上线/离线/活动时间。
- 设备重连与在线 Session 替换保留稳定 device_id；替换更新 LastOnlineAt，不更新 LastOfflineAt。默认每设备保留最近 64 个已结束 Session，查询包含历史截断计数；仅进程内存储，历史淘汰不影响 Task/File 幂等与结果。

- Phase 1D 实现 upload/download、FILE_BEGIN/ACK/CHUNK/END，原始二进制分块、size/SHA-256 流式校验、同目录临时文件验证后发布和 upload.mode。
- 文件任务使用一个 active 与有界 FIFO，支持同 task_id 幂等、断线失败结果补报；内部 CreateUpload/CreateDownload 与 FileSnapshot 暴露文件能力和下载提交事实。
- 文件与控制发送在帧边界按优先级串行化，并限制内核发送缓冲目标；持续文件传输期间可处理心跳和并发 exec。

- 实现 Phase 1C：Probe 默认 4 workers 并发 exec、乱序结果关联、进程生命周期 task_id 去重；TCP 断开时已接受任务继续执行，新会话补报缓存结果。
- Server 增加内部 ResendTask，使用原 task_id 与规格重发，按 session_id/message_id/task_id 保存每次派发关联。
- 增加实际并发执行、重复任务、参数冲突、容量、断线继续执行和中继丢弃 ACK/RESULT 后补报的自动化覆盖。

- 实现 Phase 1B TASK、TASK_ACK、TASK_RESULT 与 exec 完整闭环，Management Server 可通过内部 Task Service 对在线 Probe 创建任务并等待结果。
- Probe 增加 HEARTBEAT_ACK / TASK 在线消息路由、单 worker 串行任务执行，以及 RECEIVED / QUEUED / RUNNING / SUCCESS / FAILED / TIMEOUT 基础状态流转。
- exec 支持 `/bin/sh -c`、cwd、env、独立 stdout/stderr、1 MiB 单流捕获上限、控制帧大小适配与 timeout 进程组清理。
- 增加 Phase 1B 协议、任务状态、真实进程 exec、超时、心跳不中断、unsupported task 和大输出自动化测试。
- 实现 Phase 1A 的 Go TCP Server 与 C++11 Probe 可运行闭环，包括 REGISTER、REGISTER_ACK、HEARTBEAT、HEARTBEAT_ACK、失联判断和基础退避重连。
- 实现 Protocol v1 20-byte Header、Big Endian 编解码和可处理 TCP 粘包/拆包的流式 framing。
- 增加协议、注册/心跳和真实 Probe 断线重连自动化测试。
- 初始化路由器远程运维平台的项目设计基线。
- 建立 README、AI Agent 开发规则、架构、协议、API、路线图、项目状态、接管手册和架构决策文档。
- 将 v0.2 Word 设计输入中的 TCP 控制协议整理为仓库内可持续维护的 Markdown 基线。
- 建立文档驱动的项目状态与交接治理规则。

### Changed

- Phase 1E 完成 Phase 1A～1D 整体收口验收，Phase 1 标记为完成；验收映射与完整回归、sanitizer、Windows 适用结果记录于 docs/PHASE1_VERIFICATION.md。独立 Verification commit 交付后停止等待验收，Phase 2 尚未开始。
- 按用户确认冻结 Phase 1C 互操作契约（ADR-015）：重复任务返回 queued/running/完成态 ACK，完成态随后返回原 RESULT；内容冲突使用 ERROR/INVALID_PAYLOAD，保留原任务。
- Server 接受缺 ACK 的已派发任务结果，重复 RESULT 幂等，冲突不覆盖终态；Probe 缓存有界且不淘汰已接受身份与结果，容量满后拒绝新任务。
- 更新阶段授权边界：Phase 1C 已独立提交推送，用户现已授权 Phase 1D；确认有界文件 FIFO、流式校验、同 task_id 不重复文件副作用和中断重传必须使用新 task_id/transfer_id（ADR-016）。P1-P5 已确认并写入 Accepted ADR-017：ready 省略 sha256_ok，done=true，failed=false；顺序 offset、失败收敛和最终发布/确认丢失语义为正式契约。Phase 1D 交付后停止，不进入 Phase 1E。

- Server 每条 Probe 连接的 HEARTBEAT_ACK、TASK 和 ERROR 统一串行写入，并共用单连接 Server 发送方向的 message_id 序列。
- Probe REGISTER capabilities 现在声明 `exec`；TASK_ACK 使用 RESPONSE/reply_to，TASK_RESULT 保持异步 task_id 关联且不设置 RESPONSE。
- Phase 0 形成 Git baseline `bc8d747dfc41a375c31698073005857c238ede51`，Phase 1A 完成 Linux x86_64 首轮验证。
- 明确运行事实、规范性设计、当前状态、接管入口、计划和历史设计输入的事实来源层级。
- 完成 Protocol v1 的 message_id、response correlation、flags、UUID、任务拒绝、文件传输、JSON、boot_id、幂等范围和心跳超时细化。
- 将 Phase 1 拆分为 Phase 1A 至 Phase 1E 可验证里程碑，并增加 Probe 技术栈前置决策门槛。
- 明确 Phase 0 需要用户确认和 Git baseline commit 后才能关闭。
- 确定 Probe Phase 1 技术栈为 C++11 + CMake，首轮 Linux x86_64 验证，后续使用 toolchain files 交叉编译。
- 冻结 Phase 1A REGISTER / REGISTER_ACK / HEARTBEAT / HEARTBEAT_ACK 字段契约、注册失败响应和 10-300 秒心跳范围。
