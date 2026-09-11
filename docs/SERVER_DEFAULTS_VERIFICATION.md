# 默认服务器与 Linux AMD64 交付验证

后续纠正（ADR-058）：Probe已恢复默认在10.1.1.128通过password.txt账号密码认证，并成功生成/root/router-agent/router-agent。下文“没有ARM成品/需覆盖主机”是上一轮验证范围；最新证据见[部署§5.3](DEPLOYMENT.md#53-mipsel--arm--arm64-交叉编译)。

2026-09-11，ADR-057。六项需求涉及 Server CLI/启动脚本、Probe 默认地址与采集接口、WPF/Core、Blazor 默认连接及必要文档；没有新数据面或业务 API。

## 交付

- `server-linux-amd64.cmd` → `server-linux-amd64.ps1`：Go `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`，本地产物 `build/server-linux-amd64/router-server` 与 `start.sh`；默认通过 `scripts/upload-server.ps1` 的 OpenSSH SFTP 上传到 `/root/agent-server`。`-BuildOnly` 可跳过上传。
- 远端启动 `/root/agent-server/start.sh`，持久数据为该目录下 `data/repository`；末尾可追加 CLI 覆盖。上传采用临时文件后重命名，不改变正在运行的进程。凭据不随产物上传，根目录两份凭据已纳入 Git 忽略。
- WPF 自包含产物 `build/windows-desktop-serverdefaults/win-x64/RouterWorkbench.exe`。旧 profile 可读；再次保存移除 `ssh_user`，不读取或修改旧 `.ssh-password` 文件。默认地址仅用于新配置，已有保存地址继续生效。
- 关闭/到期/会话失效后的维护地址为空，按钮不可用；文件页 `/tmp/root` 不存在时显示原目录错误，不创建或改写设备目录。新 Probe 编译默认接口经主程序保留到模板配置回退；显式模板/CLI 规则保持。

## 本轮证据

环境：Windows PowerShell 5.1、Go、项目内 .NET 10 SDK；WSL 2 RouterAgentTest；远端 CentOS Stream 9，Linux 5.14 x86_64。

- `powershell.exe -NoProfile -ExecutionPolicy Bypass -File server-linux-amd64.ps1`：构建及真实 SFTP 上传通过，随后重复上传覆盖同名产物通过；远端 ELF 为静态 x86-64。远端原目录为空，未替换运行中的 Server。
- 远端短时执行 `start.sh -repository-dir /tmp/rmp-server-check-7lgoNl/repository`：`ss` 确认 `*:8888`、`*:9000`、`*:9001`；IPv4 `127.0.0.1` 与 IPv6 `::1` 的 `/api/v1/devices` 均返回合法空分页。测试进程经 SIGINT 正常退出，日志留在该临时目录；部署目录没有自动常驻服务。
- `windows/build-desktop.ps1 -BuildOnly -Verify -OutputName windows-desktop-serverdefaults`：自包含发布与 560 项检查通过，包括关闭/重开及迟到快照、链接清空、无账号外部 SSH 参数、旧 profile 保存、默认文件目录、切换/退出与字体偏好。发布 EXE 实际启动出现 Router Workbench 窗口并正常关闭。
- `python windows/RouterWorkbench.Desktop.Tests/render.py build/windows-desktop-serverdefaults/verification`：67 份实际 WPF 布局渲染通过；检查维护窄窗口、设置、LAN 清单和文件页，未冒充物理屏幕/DPI验收。
- `go test ./cmd/server ./internal/api ./internal/management ./internal/tunnel` 与对应 `go vet`：Windows 通过。
- `RMP_GENERATOR_WSL=RouterAgentTest`，`build/dotnet10/dotnet.exe test ProbeTemplateGenerator.sln -c Release`：176 项通过；`template-generator.ps1 -BuildOnly` 成功，零警告/错误。
- WSL 当前源码副本 `/work-runs/server-defaults-20260911`：Release Probe 构建、15 项 CTest、完整 Go/真实 Probe 集成、go vet 和 Linux Server 构建通过（集成249.367秒）。
- `GENERATOR_URL=http://127.0.0.1:18588`，`RMP_SERVER_BIN` 指向当前构建，`npm.cmd --prefix tests run test:generator`：15组Edge浏览器场景通过、无浏览器错误，包括首次默认服务器地址、草稿恢复、显式修改地址后真实API发布及邻居配置。测试用本机生成器进程已退出。
- 新增/修改PowerShell入口语法检查、`git diff --check`通过；未改业务并发或协议字节，本轮不重复既有缺库的C++sanitizer，也未重跑Go race。

远端没有 Go、CMake 或 `/root/gcc-5.2`。Server 本机构建不依赖它们；ARM 构建需要用 `-SshTarget root@10.1.1.128` 指定原有工具链主机或由用户准备新工具链，本轮没有 ARM 成品/固件验收。已有 C++ sanitizer 缺库、物理输入/DPI及厂商实机缺口保持。没有 Git 提交或推送。

自动审批拒绝清理前期环境检查创建的 `build/remote-inspect/id_rsa`（仅当前用户可读的加密私钥副本）与 `%TEMP%/rmp-server-askpass.exe`（不含口令的辅助程序），仅返回 `blocked by policy`，没有更具体原因；这两份临时文件留待用户手动删除。根目录原始私钥与口令未修改。正式SFTP脚本的两次运行均正常完成其自身临时目录清理。
