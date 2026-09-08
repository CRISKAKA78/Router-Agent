# 服务端设备属性模板方案

状态：Accepted，2026-09-07。用户明确回复“按此方案继续实现”，已授权以下协议扩展、服务端持久化及 UI 实施；具体实现与验证状态见 PROJECT_STATUS。正式决定见 ADR-029。

## 需求与现有边界

- 当前 Gateway 要求第一帧为 REGISTER；Probe 在连接前生成注册资料，没有取得服务端模板的入口。
- ADR-014 冻结注册握手，ADR-018 以成功注册的完整快照保存设备与 Session；自定义属性不能靠未知 REGISTER 字段传递，因为 Server 会忽略它们。
- 默认设备 ID 独立于模板：未提供 `--device-id`（兼容 `--device_id`）时执行 `nvram get SN`，显式参数优先。失败不能生成随机或共享 ID。

## 推荐实施方案

1. 服务端新增独立属性模板 Service，由公开 `/api/v1/probe-templates` API 和 React 设置中的模板管理入口使用。支持列表、新建、编辑、删除；ID 永不重用、名称唯一，修改递增版本。沿用中文 Fluent 和已有表单，不重排设备主页面。
2. 模板保存于 Server 数据目录下独立 JSON 文件，服务端单写者锁、临时文件加原子替换；重启保留。模板不混入 File/Tool Repository 的资产身份或 schema。管理 API 沿用现有幂等规则，更新带版本条件，冲突拒绝覆盖。
3. Probe 增加互斥 `--template-id ID` / `--template-name NAME`，仍使用 `--server HOST:PORT`。选模板时，在同一控制 TCP 上先发送新增 TEMPLATE_GET/TEMPLATE_REPLY 消息（建议 0x05/0x06），成功收到后关闭这条准备连接，在本地采集，再建立正常 REGISTER 连接。准备连接不创建设备/Session、不开放任务或维护；响应有界且超时。这样不在现有注册 deadline 内执行采集，也不引入 Probe HTTP/TLS 客户端。
4. 模板定义属性 key、中文名称和 Shell command。首版支持现有 serial/model/firmware/hostname/kernel/libc，以及最多 32 个扩展字符串属性；device_id、arch、boot_id、probe_version、capabilities 不受模板覆盖。现有 `--hostname` 显式值优先。未选模板沿用原启动行为。
5. 每条命令通过已有 `/bin/sh -c` 执行器运行，默认 5 秒、可配置 1～30 秒，整次采集 60 秒预算；保留现有有界输出捕获和进程组回收。输出取去除首尾空白的 UTF-8 字符串；现有字段遵守原长度限制，扩展属性值最多 4096 bytes。失败、空值、超长或非法编码省略该属性并记录字段名和失败原因，不发送命令正文或 stderr。模板的命令由部署管理员维护，以 Probe 当前账号权限执行。
6. REGISTER 兼容新增模板 ID/名称/版本、扩展属性及采集失败摘要；服务端校验并作为 Device/Session 的注册快照保存，HTTP DTO 与 React 设备详情显示结果和缺失原因。没有实时遥测或心跳采集，也不覆盖现有必需字段。
7. 每次启动重新读取服务端模板；成功采集后的普通断线重连复用本进程快照，不重复执行命令。模板编辑/删除不追溯修改已有快照，下一次启动生效。名称解析一次绑定返回的 ID 和版本。模板不存在、旧 Server 不支持或模板格式非法时明确启动失败；网络暂时不可用按有界退避重试，取得模板前不上线，不静默退回无模板。
8. 旧 Probe/不选模板的 REGISTER 路径保持兼容。新模板客户端需要升级 Server；不尝试用未知字段、Exec Task 或 Heartbeat 绕过已确认注册快照语义。

## 已确认的设计变化

新增 superseding ADR-029，仅取代 ADR-014 的“所有连接首帧只能 REGISTER”限制，增加受限模板准备连接；补充 ADR-018 的注册资料字段，保留注册成功后才创建 Inventory、完整快照替换和 Session 历史规则。同步 PROTOCOL/API/ARCHITECTURE，不改写历史 ADR。

模板 ID/名称选择、服务端管理、注册前新消息、持久化方式、扩展属性及启动生效语义均已确认，继续自主完成实现与验证，不重复索取授权。

## 完成条件与验证

- 管理界面创建并编辑模板，Server 重启后仍可查询；并发编辑冲突和非法配置可见。
- 真实 Linux Probe 分别按 ID/名称选择；自定义命令结果经注册、Device Service、公开 API 到设备详情一致；模板更新、缺失、失败、超时和旧客户端兼容有回归。
- 默认 ID：真实进程通过测试 nvram 程序验证自动获取和显式覆盖，空输出、非零退出、非法编码、超长及超时拒绝；重连维持相同 ID。
- 按 DEVELOPMENT 执行 Phase 1～5 release/asan/race、Windows Go 与前端检查；厂商设备另行实测，不把测试 nvram 替身称为固件验证。
