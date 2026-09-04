# Phase 1A Codex 执行指令

你现在进入路由器远程运维平台的 Phase 1A。

## 开始前

1. 严格按 AGENTS.md 的顺序阅读全部项目状态与设计文档。
2. 检查真实 Git 仓库状态。
3. 如果当前文档基线尚无 Phase 0 baseline commit，先提交当前文档基线；提交后把 commit ID 写入 docs/PROJECT_STATUS.md 和 docs/HANDOFF.md，并把 docs/ROADMAP.md 的 Phase 0 标记为完成、Phase 1 / Phase 1A 标记为进行中。
4. baseline commit 之外，不要先做任何其它重构。

## 已确认技术栈

- Management Server：Go。
- Probe：C++11。
- Probe 构建系统：CMake。
- 第一开发与验证平台：Linux x86_64。
- 后续交叉编译：使用 CMake toolchain files 适配 mipsel / ARM / ARM64；本阶段不要求完成全部交叉编译。
- Probe 不得使用高于 C++11 的语言特性，不引入不必要的重型运行时依赖。

## 本阶段唯一范围

只实现 Phase 1A：

- 20-byte TCP frame header encode/decode。
- Big Endian 整数处理。
- TCP stream framing（粘包/拆包）。
- Probe 主动连接 Server。
- REGISTER。
- REGISTER_ACK 成功与失败。
- HEARTBEAT。
- HEARTBEAT_ACK。
- message_id / reply_to。
- heartbeat_interval 与 3 倍失联判断。
- 基础退避重连。
- session_id 每次重新 REGISTER 更新。
- 与上述内容直接相关的日志、配置和测试。

所有字段和时序严格以 docs/PROTOCOL.md 为准。

## 明确禁止

本阶段不要实现：

- TASK / TASK_ACK / TASK_RESULT。
- exec。
- 文件上传下载。
- Process Manager。
- Tunnel。
- HTTP / WebSocket API。
- 数据库。
- Web / Windows / 微信小程序。
- MCP / AI Agent。
- VPN / FRP。
- 安全体系、TLS、权限等后续主题。

不要为了未来阶段创建大量空 package、空类或占位实现。

## 实现要求

Server 与 Probe 必须形成真实可运行闭环：

1. 启动 Server。
2. 启动 Probe。
3. Probe 主动 TCP connect。
4. Probe 发送 REGISTER。
5. Server 校验并返回 REGISTER_ACK。
6. Probe 进入 ONLINE。
7. Probe 按 heartbeat_interval 发送 HEARTBEAT。
8. Server 返回 HEARTBEAT_ACK。
9. 人为断开连接后 Probe 按既定 backoff 重连。
10. 重连后重新 REGISTER，并获得新的 session_id。

必须测试 TCP 拆包/粘包，不得假设一次 recv 得到完整帧。

## 测试要求

至少覆盖：

- Header encode/decode。
- Big Endian。
- 半包 Header。
- 半包 Payload。
- 多帧一次读取。
- bad magic。
- unsupported version。
- payload 超限。
- REGISTER 合法。
- REGISTER 缺必选字段。
- REGISTER 字段超范围。
- REGISTER_ACK success=false。
- HEARTBEAT / ACK reply_to 正确。
- 断线重连并生成新 session_id。

## 阶段交付

Phase 1A 完成后必须：

- Server 与 Probe 在 Linux x86_64 实际构建成功。
- 自动化测试通过。
- 给出最小运行命令。
- 更新 README（仅补真实构建/运行命令）。
- 更新 docs/PROJECT_STATUS.md。
- 更新 docs/HANDOFF.md。
- 更新 docs/ROADMAP.md。
- 如实现未改变协议，不修改协议语义；如发现协议阻塞问题，先记录并停止擅自改协议。

完成 Phase 1A 后停止，不要自动进入 Phase 1B。
