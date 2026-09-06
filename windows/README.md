# Windows 远程维护工作台

Windows x64、C# / .NET 10 LTS Windows Forms。发布包带运行时，无需单独安装 .NET；开发需要 .NET 10 SDK。无 NuGet 第三方依赖，无 Node、WebView 或浏览器前端工程。

## 启动和连接

运行发布目录的 `RouterWorkbench.exe`，输入 `http://主机:端口` 或 `https://主机:端口`，点击“连接 / 切换”。默认 `http://127.0.0.1:8080`。这里填写 API 地址，不是 Probe 控制 TCP 或 Tunnel data 地址；不含 `/api/v1`。界面显示“HTTP 快照已同步”后可操作。

API 当前用于可信本机或受保护管理网络；TLS 和访问限制由部署层负责。客户端使用系统证书验证，不提供忽略证书错误开关。连接配置保存到 `%LOCALAPPDATA%\RouterWorkbench\profile.json`，仅含地址、外部客户端路径和 SSH 用户名；不保存密码、业务快照、Tunnel token 或内部数据面标识。配置 `schema_version` 为未来扩展入口，认证方案需后续正式设计。

## 日常操作

1. 在左侧选择在线设备，“设备详情”查看注册资料和当前 / 历史 Session。
2. “远程维护”点击“开启远程维护”，默认 240 分钟。取消默认勾选可输入分钟或毫秒；值须对应正整数毫秒，范围与 API 一致。
3. 创建成功后自动选中新 Maintenance，直接看到 Web / SSH / Telnet 入口和剩余时间。点击相应按钮打开，或复制地址。列表保留 Server 可查询的历史；可主动关闭所选维护。
4. Web 使用系统默认浏览器；SSH 默认使用 Windows OpenSSH（系统目录 `OpenSSH\ssh.exe`），Telnet 默认使用 Windows Telnet Client。未安装时在“设置”选择已有 PuTTY.exe 并勾选对应 PuTTY 选项，也可指定相同命令行约定的系统客户端。用户名默认 root，可修改；密码和主机密钥验证留在外部客户端，不禁用 SSH 主机密钥检查。
5. “Task / Exec”发起基础命令，查看状态、真实 RESULT、退出码、stdout/stderr 和截断标记。重发原 Task 使用原业务身份，不新建替代任务。没有实时输出流或任务取消。
6. “文件资产”可导入本机文件到 Server、另存资产、上传到设备、从设备下载和归档。下载 Task 的 Committed / Released 与最终 RESULT 分别呈现；“导入已完成下载”显式生成资产，“清理下载暂存”调用 Server 现有清理能力。
7. “工具 / 版本”创建工具、发布含多个产物的不可变版本、查看 Server 兼容结果、选择 compatible 产物投放及归档。架构和 libc 必须显式填写集合或 any；可填写型号、内核和必需能力。投放只上传文件，执行须另行发起 Exec。

外部浏览器、SSH/PuTTY/Telnet 由用户拥有，关闭工作台不会杀死这些程序，也不会主动关闭 Server 已创建的 Maintenance 或撤销任务；维护仍由 Server 的租期、Session 或显式关闭操作控制。

## 刷新和错误

WebSocket 只作为刷新通知。首连/重连收到 resync_required 后获取 HTTP 快照；刷新期间发生变化会再次查询。连接中断自动按 1/2/4/8/16/30 秒重试，另每 5 秒刷新快照用于恢复漏失通知和显示最新状态。断线或快照失败禁用新操作，显示可能过期提示。倒计时基于本机时钟，仅作估算；到期状态以 Server 回查结果为准，打开入口前另查一次 Maintenance。

所有 POST/PUT 带随机幂等键。网络响应丢失时保留原请求和键，暂停新写操作；核对当前快照后，仅在确认 Server **未重启** 时点击“重试原请求”。若 Server 已重启，不能用旧键假定去重，应先核对实际副作用并点击“已核对不确定操作”。切换 Server 或退出清除进程内未决请求。已返回的 task_id 与 dispatch_uncertain 一起显示，不自动生成替代任务。

错误包含操作名称、业务 code 和 HTTP 状态；分别提示 Server 不可达、设备离线、Session 变化、维护冲突/容量不足及工具文件失败。API 的 capacity_exhausted 共用于端口池和其他 Server 容量，因此维护提示为“维护端口池或 Server 容量不足”，不猜测具体耗尽的内部资源。

列表自动读取每页 200 项，超过 10000 项显式报错，不把截断清单当完整快照。适用于技术售后的小规模工作台；资产导入客户端上限 1 GiB，Server 还可设置更小限制。HTTP 请求超时 40 秒，资产保存体超时 5 分钟；实际大文件准备仍受 Server API 配置约束。

## 构建与验证

在仓库根目录运行：

```powershell
./windows/build.ps1 -Verify
# 自定义 SDK 路径：
./windows/build.ps1 -Dotnet ./build/dotnet/dotnet.exe -Verify
```

输出 `build/windows-ui/win-x64/RouterWorkbench.exe` 和本说明。自包含包较大，换取无需预装运行时；原生库会按 .NET 单文件机制解压到当前用户临时目录。也可用 `dotnet publish windows/RouterWorkbench -c Release --self-contained false` 制作要求已安装 .NET 10 Desktop Runtime 的较小目录包。

自动化验证程序启动真正的 Windows Go Server；用测试专用 Protocol v1 对端覆盖客户端请求与响应，再用可控 WebSocket 服务测试故障。原生 Windows Forms 消息循环验证控件操作、入口分派和关闭，截图写到 `build/phase6-tests/maintenance.png`。测试中外部客户端使用 argv 捕获进程，不需要登录真实设备或弹出用户的终端。真实 Linux Probe 和 SSH/Telnet/HTTP 通道由 Phase 1～5 全量回归覆盖，见 `docs/PHASE6_VERIFICATION.md`。

构建依据：[Windows Forms](https://learn.microsoft.com/en-us/dotnet/desktop/winforms/overview/)、[.NET 支持策略](https://dotnet.microsoft.com/en-us/platform/support/policy)、[单文件发布](https://learn.microsoft.com/en-us/dotnet/core/deploying/single-file/overview)。
