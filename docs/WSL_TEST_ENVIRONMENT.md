# 本机 WSL 测试环境

2026-09-07 按用户要求，将原 `build/template-check/linux.ext4` 的 Alpine 文件系统导出并导入为独立 WSL 2 发行版 **RouterAgentTest**。安装目录为 `C:\Users\Administrator\WSL\RouterAgentTest`，运行磁盘为其中的 `ext4.vhdx`。以后本机 Linux 测试复用此发行版，不再创建项目内 rootfs 镜像，不依赖 docker-desktop 提供测试系统。Windows/WinUI 与路由器实机验证仍在对应平台执行。

原镜像已卸载，loop 设备已释放；原文件留给用户移除。删除项目中的 `build/template-check/linux.ext4` 不影响此发行版。不要删除安装目录中的 `ext4.vhdx`，也不要执行 `wsl --unregister RouterAgentTest`，后者会删除发行版数据。

## 进入与复制当前源码

```powershell
wsl -d RouterAgentTest -u root --cd /work
```

`/work` 是迁移时的源码和构建副本，不会自动同步 Windows 仓库。已通过 `/etc/wsl.conf` 关闭该发行版的 Windows 磁盘自动挂载及 Windows 程序互操作；保持此设置，不把工作区 bind mount 到测试根目录。每次验证复制当前源码到新的 Linux 目录，避免旧副本混入结果。在项目根目录执行以下 PowerShell：

```powershell
$testRunName = 'router-' + [guid]::NewGuid().ToString('N')
$testArchive = Join-Path $env:TEMP ($testRunName + '.tar')
tar -cf $testArchive go.mod cmd internal probe tests
if ($LASTEXITCODE -ne 0) { throw 'Source archive failed' }
wsl -d RouterAgentTest -u root --cd / -- mkdir -p /work-runs
if ($LASTEXITCODE -ne 0) { throw 'WSL preparation failed' }
Copy-Item -LiteralPath $testArchive -Destination "\\wsl.localhost\RouterAgentTest\work-runs\$testRunName.tar" -ErrorAction Stop
$testLinuxPath = '/work-runs/' + $testRunName
wsl -d RouterAgentTest -u root --cd / -- mkdir $testLinuxPath
if ($LASTEXITCODE -ne 0) { throw 'Test directory creation failed' }
wsl -d RouterAgentTest -u root --cd /work-runs -- tar -xf ($testRunName + '.tar') -C $testLinuxPath
if ($LASTEXITCODE -ne 0) { throw 'Source extraction failed' }
```

上述清单针对 Go/Probe；如任务新增构建输入，应按实际入口补充。归档包含未提交的当前源码。测试输出保存在本次 Linux 目录，后续只清理明确核对过的测试文件；不能跨 shell 拼接递归删除，也不能在卸载失败后继续清理挂载目录。

## 隔离运行

完整 Release 验证（接上面的 PowerShell 变量）：

```powershell
wsl -d RouterAgentTest -u root --cd $testLinuxPath -- unshare -mnpf --mount-proc sh -c 'set -eu; mount --make-rprivate /; ip link set lo up; mount -t devpts devpts /dev/pts -o newinstance,ptmxmode=0666,mode=0620; sh tests/verify-phase5.sh release'
if ($LASTEXITCODE -ne 0) { throw 'Linux verification failed' }
```

真实维护测试绑定 80/22/23，必须保留 network/devpts 隔离。按 DEVELOPMENT 选择本次需要的测试，不要求每次重跑全量。当前 Alpine 缺少 C++ sanitizer 开发运行库，既有 `asan`/`race` 脚本的 C++ 阶段仍不可用；不能把 Go race 通过当作 C++ TSan 通过。

## 本次迁移验证

- WSL 2 注册信息确认 BasePath 在项目外；终止并重新启动成功，Windows 磁盘未自动挂载，Windows UNC 可读 Linux 文件。
- Alpine 3.22.1、GCC 14.2.0、CMake 3.31.7、Go 1.24.13 保留。
- 在 `/work` 的隔离 namespace 内，`ctest --test-dir build/phase5-probe --output-on-failure`：6 项全部通过。
- 同一隔离环境执行 `RMP_PROBE_BIN=/work/build/phase5-probe/router-probe go test ./tests/integration -run TestProbeTemplate -count=1`：通过。
- 本次为环境迁移检查，没有重新执行全量产品测试、C++ sanitizers 或厂商固件验收。此前误删数据的未恢复状态仍见 [恢复记录](PROBE_TEMPLATES_VERIFICATION.md)。
