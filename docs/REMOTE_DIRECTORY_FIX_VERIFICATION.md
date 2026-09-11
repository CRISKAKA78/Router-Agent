# FTV300 远程目录读取修复验证

2026-09-12。用户明确授权排查SSH维护入口47.119.168.150:20004的设备文件读取失败，及服务器47.119.168.150:22。凭据仅由本地辅助程序读取，不记录密码、私钥或文件正文。

## 原因与范围

- 设备为FTV300/FJB130161591，mipsel/Linux4.4.198，SSH实际身份uid=0(admin)。`/tmp/root`存在且可列出；Probe在线，Server进程正常，不是SSH密码、目录权限或维护链路断开。
- 原失败任务`0d019e39-1ad0-4037-b3f8-e3f437fd5851`执行`RemoteDirectory.Command`旧脚本；TASK RESULT为failed、exit_code=2、stdout/stderr为空。实机`command -v stat`及`stat -c`均返回127，`busybox stat`返回1/applet not found，固件没有该命令。
- 旧客户端直接依赖`stat -c '%s %Y'`，且失败文案只拼stderr，形成截图中的空错误信息。
- 本轮Windows编译还复现raw string携带CRLF而导致Shell解析失败（任务`a62b36a7-17f3-45b6-a4be-8c5b20c3dd0a`）；最终只对脚本文本归一LF，保留目录路径本身的CR/LF与引号。

## 实现

- `windows/RouterWorkbench.Client/RemoteDirectory.cs`：有stat保持原精确元数据路径；无stat用`ls -ldn`仅取前置元数据中的大小，绝不从ls文本重建文件名、不读取文件内容。名称始终来自原Shell遍历且以NUL分隔；250项上限、目录/符号链接逻辑保持。
- 缺修改时间或无法识别的特殊文件大小作为可空值，在界面显示“未提供”，不伪造0字节或1970时间；真实0字节/epoch仍有效。
- 失败优先保留stderr；为空/空白时报告状态、退出码、原task_id，不自动补发新任务。cd/stat/ls失败还增加明确的脚本stderr。
- 只修改客户端和相关测试；API、TCP协议、Server、Probe、默认目录与原布局不变，无需新增ADR或固件工具安装。

## 验证命令与结果

### Windows

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-filefix
```

.NET10自包含发布、测试Server构建、631项桌面检查通过；产生81份WPF XPS布局。新增检查覆盖可空元数据、错误/任务身份、不重放、LF脚本及路径字节保持。日志：`build/file-directory-fix/desktop-verification.log`。本轮未逐张视觉复审81份布局，不将离屏输出描述为物理屏幕验收。

### Linux Shell

从编译后的真实C#方法导出脚本（输出用UTF-8/LF保存为文件，不额外做行尾转换）：

```powershell
build/dotnet10/dotnet.exe run --project windows/RouterWorkbench.Desktop.Tests -c Release -- --directory-shell-command /fixture
```

将导出的`command.sh`和`tests/remote-directory-shell-test.sh`复制到既有WSL RouterAgentTest的`/work-runs/file-directory-fix`后：

```sh
sh /work-runs/file-directory-fix/test.sh /work-runs/file-directory-fix/command.sh
```

8组通过：空目录、stat精确元数据、无stat的空格/引号/换行/中文/隐藏/通配符/LIMIT名称、悬空链接/FIFO/目录、250项上限、stat空错误、ls错误和目录不存在。测试用独立PATH屏蔽stat、临时目录自动清理，FIFO以5秒超时保护；不新增WSL运行时依赖。日志：`build/file-directory-fix/shell-verification.log`。

### 真实设备与当前服务

本机临时C#验证程序直接引用修复后的Client项目，经公开API使用`RemoteDirectory.ReadAsync`及`FileExchange`；不是Mock或仅SSH替代验证。

| 检查 | 真实结果 |
| --- | --- |
| FTV300 `/tmp/root` | 任务`275be041-e109-4fe0-825b-77429ddec018` success/0、未截断；4项含.ssh、data.txt和2个接口状态文件；修改时间未知 |
| FTV300 `/tmp/root/data.txt` | 下载任务`3c8b8f83-5d89-462a-a64f-93e8b994320b` success/0，29字节；File API完整性校验成功，另与直接SSH读取字节完全一致 |
| FNR100 `/tmp/root` | 任务`7aae3241-85d2-40ed-b5f1-a52f645454f2` success/0、未截断；原stat大小/修改时间路径正常 |

本机精简证据：`build/file-directory-fix/live-verification.json`；下载验证文件仅保留在忽略的build目录，不记录正文到文档。诊断生成少量任务记录，并按既有下载完成流程创建一份服务器Asset；未更改设备文件。

## 交付与边界

新程序为`build/windows-desktop-filefix/win-x64/RouterWorkbench.exe`。保留正在运行的`build/windows-desktop/win-x64/RouterWorkbench.exe`，用户关闭旧窗口后运行新包才生效；未主动终止或替换用户进程。

未重启/部署Server或Probe、未更改固件、未测试上传写入；未重跑Go/Probe全量及sanitizers（本轮未修改这些实现）。未提交或推送，原双架构构建改动保留。Phase6最终产品验收、双架构Probe部署与异地组网里程碑保持独立。
