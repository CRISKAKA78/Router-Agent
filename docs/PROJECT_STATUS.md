# 项目状态

最后更新时间：2026-09-06

## 当前阶段

Phase 0～5已验收。Phase 6 Windows UI实现和规定验证通过，等待用户验收；启动稳定基线为 `57c2b1f8f6da1069e4a2eb988224b94bafe9cf84`，fetch后HEAD/main/origin/main一致，初始工作区干净。采用Accepted ADR-024。独立Phase 6 Windows UI commit推送main后停止；真实提交SHA与推送事实以Git记录为准。

## 当前可验证能力

- 原生C# / .NET 10 LTS Windows Forms工作台，自包含Windows x64单文件发布，无WebView、Node或第三方NuGet依赖。入口与构建：[windows/README](../windows/README.md)。
- Server连接设置、在线/离线设备列表与过滤、设备资料、当前/历史Session；Maintenance默认240分钟、自定义正租期、三入口、剩余时间、历史和主动关闭。
- Web交系统浏览器，SSH/Telnet交系统客户端或用户指定PuTTY；独立参数传递，缺失客户端有设置提示，不自研协议或处理密码。
- 基础Exec、Task状态/真实RESULT/输出/原身份重发；文件资产导入/保存/上传/下载/显式导入/清理/归档；工具创建/版本发布/产物规则/Server兼容判断/显式投放与归档。
- WebSocket首连/重连HTTP同步、合并刷新与5秒恢复刷新；当前连接/选择版本检查拒绝迟到结果，任务/版本/兼容详情查询分别串行合并；切换Server与退出取消并等待HTTP/WebSocket及后台工作。
- 写入单次准入及双击防护；响应丢失保留原请求字节和幂等键，核对Server未重启后显式重试；保留非空task_id和dispatch_uncertain。下载Committed/Released与最终RESULT分开显示。
- Windows客户端仅消费Phase 5 `/api/v1` 和WebSocket。Server、Probe、Go依赖、业务状态机和Tunnel数据面生产代码均无变化，既有Phase 1～5能力保持。

## 验证

详细映射、环境、命令见[PHASE6_VERIFICATION](PHASE6_VERIFICATION.md)。

| 验证 | 结果 |
| --- | --- |
| Windows客户端编译/自包含发布 | 0警告、0错误；SDK10.0.400 / Runtime10.0.11 |
| 客户端真实HTTP/WS及原生WinForms | 84项断言通过；设备/Session/维护/Exec/File/Tool/断线/重复点击/关闭 |
| 外部入口启动 | Web系统Shell分派验证；OpenSSH/Telnet/PuTTY模式实际argv捕获进程通过 |
| 自包含EXE启动 | 实际Windows启动并完成消息循环初始化；隐藏测试进程随后终止 |
| Linux Release C++11/CTest | 4/4通过，4.70秒 |
| Linux Phase 1～5全量Go/真实Probe | 全包通过，集成169.961秒 |
| Go race + TSan真实Probe全量 | 全包通过，集成176.819秒，无报告 |
| TSan / ASan+UBSan+LSan CTest | 各4/4，6.83秒 / 5.92秒，无报告 |
| ASan真实Probe Phase 4/5 | 通过，15.549秒 |
| Windows原生Go测试/vet/build | 全部通过；真实Linux用例在Linux执行 |
| Linux vet/build/实际HTTP启动/SIGTERM | 全部通过，正常退出0 |
| go mod verify / git diff --check | 通过 |

## 已知边界

- API用于可信本机或受保护管理网络；内置认证/TLS/RBAC/租户/完整审计仍未实现。配置不保存密码/业务快照/Tunnel身份；远程安全边界由部署层提供。
- 当前发布Windows x64，带运行时EXE约111 MiB；其它Windows架构/实机DPI矩阵未验收。SSH/Telnet/PuTTY须由用户已有环境提供；外部登录与主机密钥交互由该客户端负责。
- HTTP/WebSocket无历史重放、实时stdout流或跨Server重启幂等保证。仅Repository持久化；任务/设备/Operation/Maintenance与HTTP账本重启清空。归档不物理GC。
- 客户端显示倒计时受本机时钟影响，业务到期以Server查询为准；列表超过10000项显式失败，文件导入上限1GiB并受Server限制。capacity_exhausted不能区分端口池与其它Server容量。
- Maintenance仅固定Probe127.0.0.1:80/22/23，默认24小时端口隔离不保证超窗/重启后的永久旧地址隔离。未增加通用转发、新数据面或任意目标端口。
- Probe仍以Linux x86_64验证；mipsel/ARM/ARM64、uClibc/老内核实机矩阵未执行。
- Web UI、微信小程序、MCP和AI Agent未实现、未获本轮授权。

## 下一步

形成独立 `feat: add Phase 6 Windows maintenance workbench` commit并推送main后停止，等待Windows UI验收。不得自行开展Web、微信、MCP或AI。
