# nvram / uci 读写与属性采集方案

状态：Accepted，2026-09-08。用户明确回复“按此方案继续实现”，授权实现及相关回归测试；对应 ADR-031。以下为确认方案，实现和验证状态见 PROJECT_STATUS。

## 仓库核对与设计边界

- Probe `task.cpp` / `task_manager.cpp` 当前只执行 exec、upload、download；exec 使用 `/bin/sh -c`，已可以运行系统 nvram/uci 命令，但没有结构化配置任务。
- `collection.cpp` 与 `internal/probetemplate` 当前只支持 command 属性来源；模板已能填写 `nvram get SN` 或 `uci get system.@system[0].hostname`。
- `identity.cpp` 默认一次 `nvram get SN` 解析设备 ID，显式 ID 优先。设备 ID 属于稳定身份，不从任意 UCI 属性猜测替代值。
- 问题类别是已接受设计需要正式扩展，不是实现偏离设计。新增任务参数需补充 ADR-015；新增模板来源需补充 ADR-029。既有协议帧头、消息类型和任务状态不变。

## 已确认用户体验

1. 设置 → 探针属性模板：每项选择“命令 / nvram / uci”。命令保留原输入；nvram 填键名，例如 `SN`；uci 填路径，例如 `system.@system[0].hostname`。来源只执行读取，沿用显示名、超时和启动快照。
2. 设备详情增加沿用现有样式的“配置读写”操作表单：选择 nvram/uci、读取/写入/删除/提交，填写键或路径及写入值。操作进入现有任务中心查看 ACK 和最终结果，不把 HTTP 202 当作成功。
3. 写入与提交分开。不自动 commit、重启服务或设备；用户可以先读取核对，再明确提交。UCI 提交必须指定 package；nvram 提交明确表示提交整份 NVRAM。
4. 修改配置不会刷新启动注册属性，避免将启动快照伪装成实时值；即时核对使用读取任务，模板更新仍在下次启动生效。

## Probe 与任务契约

复用 TASK / TASK_ACK / TASK_RESULT，增加 `type="router_config"`，参数为 `backend`、`operation`、`key`、`value`、`package` 的受限组合：

| backend / operation | 必填参数 | 设备程序调用 |
| --- | --- | --- |
| nvram / get | key | `nvram get KEY` |
| nvram / set | key、value | `nvram set KEY=VALUE` |
| nvram / delete | key | `nvram unset KEY` |
| nvram / commit | 无 | `nvram commit` |
| uci / get | key | `uci get PACKAGE.SECTION.OPTION` |
| uci / set | key、value | `uci set PACKAGE.SECTION.OPTION=VALUE` |
| uci / delete | key | `uci delete PACKAGE.SECTION.OPTION` |
| uci / commit | package | `uci commit PACKAGE` |

- Probe 保持 C++11，调用固件提供的命令程序，复用现有超时、输出限制、TERM/KILL、文件描述符隔离和结果回报。参数独立传给程序，不拼接 shell 命令；不链接厂商 NVRAM 库或要求安装 libuci 开发库。
- nvram key 限 128 bytes、字符 `[A-Za-z0-9_./:-]+`，首字符非 `-`；UCI 路径限 256 bytes，使用命名 section 或 `@type[index]`（含负索引），首版操作 option，不删除整个 package/section。package 为 `[A-Za-z0-9_]+`。value 为最多 4096 bytes 的无 NUL UTF-8 字符串，允许空字符串；拒绝操作无关参数。
- 默认任务超时 5 秒，允许 1～30 秒；结果沿用 status、exit_code、stdout、stderr、truncated，不另建任务状态。非零退出或缺命令如实失败；读取空值保留为空，不伪造“键不存在”（不同 nvram 实现可能无法据退出码区分）。
- 新 capability `router_config` 表示实现该任务协议，不保证固件安装两个命令。Server 对旧 Probe 返回不支持，禁止静默降级为写 shell 任务；现有 Exec 和旧 Probe 正常使用。
- 同 task_id 比较 type、timeout 及全部已定义配置参数；冲突拒绝，已接受任务不重执行，重连补报原结果。保持进程内幂等，不新增跨 Probe/Server 重启恢复或自动重试。
- Probe 内结构化配置任务按接受顺序串行执行，排队仍用既有容量控制；读取、写入、删除、commit 相互不穿插。此序列不约束外部 SSH、LuCI 或原始 Exec，多个任务之间不承诺事务隔离或回滚；操作方等待前一任务结果再执行依赖动作。
- commit 可能提交其他程序已暂存的修改；超时/断连不说明设备配置未改变，先查询原任务与当前值，不自动重发新的写入身份。

## 属性模板与 API

- 兼容保留 `{name,command,timeout_seconds}`；新增 `{name,source:"nvram"|"uci",key,timeout_seconds}`。缺 source 按旧 command；命令字段与配置 key 互斥，配置来源只允许 get。
- Probe 模板读取与配置任务共享参数校验和直接程序调用。模板依然逐项、每项 1～30 秒、整体 60 秒；去首尾空白、空值/失败摘要及字段限制继续按 ADR-029。模板来源不修改 REGISTER 结果结构。
- 旧模板无需迁移；旧 Probe 选择含新来源的模板会明确解析失败，部署需先更新 Probe。旧 Server 不支持新任务和模板字段，不自动回退或改写配置。
- 新增 `POST /api/v1/devices/{id}/config-tasks`，请求包含上述参数及可选 `timeout_seconds`；沿用 Idempotency-Key、202 `{task_id,dispatch_uncertain}` 和现有查询/重发接口。GET tasks 的规格公开结构化配置参数。
- HTTP Adapter 只调用 Management 用例，由 Task Service 记录不可变规格、Gateway 发送；Probe 执行。React 只传结构化 DTO，Windows Shell 不新增业务逻辑或 Bridge。
- 不硬编码不同型号的属性映射，不自动创建/覆盖用户模板或执行真实设备写入。OpenWrt 没有可用 `nvram get SN` 时继续用 `--device-id` 指定稳定 ID；自动身份来源切换不在本方案内。

## 实施与验收

ADR-031 已由用户确认；Probe、Task/Management/Gateway/API、模板来源与 React 调用链已实现，正式契约见 API / PROTOCOL。

验证覆盖特殊字符作为原样参数、空值/不存在/命令缺失/非零退出/超时、顺序与容量、重复和冲突 task_id、断线结果补报、旧 Probe/模板兼容、模板只读和注册快照、API 幂等及前端原请求保留。按 DEVELOPMENT 运行适用回归；此前停止测试要求针对 Win32 宿主迁移，本次用户已确认实现及相关回归测试，不将历史结果替代本次结果。厂商固件实测另记，替身程序不是 DD-WRT/OpenWrt 验收。

前次方案交付仅修改设计与状态文档；本次实现及测试范围见 [ROUTER_CONFIG_VERIFICATION](ROUTER_CONFIG_VERIFICATION.md)。未操作用户设备或运行数据。

参考：[OpenWrt UCI 技术说明](https://openwrt.org/docs/techref/uci)说明暂存与 commit；[官方 UCI CLI 源码](https://lxr.openwrt.org/source/uci/cli.c)可核对参数解析。DD-WRT Wiki 当前请求被访问保护拦截，其命令行为仍须在目标固件验证。
