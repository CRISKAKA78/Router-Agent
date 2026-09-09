# 服务端模板启动修复验证

日期：2026-09-09。范围：ADR-046，Server持久目录加载和启动后模板更新；不改Probe、WPF、生成器或公开API格式。

## 问题与处理

用户日志中的`fatal=json: unknown field "interface_aliases"`发生在运行期加载设备目录，并非Go编译失败。当前`probe-templates/catalog.json`为空，但`devices/catalog.json`仍含两份旧presentation和旧reported结果字段，因此只清空模板列表不能恢复启动。另有已初始化模板目录文件缺失即退出的分支。

加载器只清理已知退役字段，先验证、备份原字节，再原子保存；模板缺失告警建空库。保留设备和型号资料、绑定模板、命令、版本及配置修订；不自动应用新模板。其他无效结构继续拒绝并标明文件路径。

## 本次验证

Windows PowerShell / Go 1.25.5 windows/amd64：

```powershell
go test ./internal/catalogupgrade ./internal/probetemplate ./internal/enrollment ./internal/management ./internal/api -count=1
go vet ./internal/catalogupgrade ./internal/probetemplate ./internal/enrollment ./internal/management ./internal/api
go build -trimpath -o build/server-template-startup-fix/router-server.exe ./cmd/server
& build/server-template-startup-fix/smoke.ps1
```

均通过。`TestStartupWithRetiredSnapshotsAndTemplateRecovery`覆盖全新、空库、已初始化后文件缺失、旧模板库四种情况；公开API查询/创建/更新、旧版本409、旧字段400、设备显式应用、重新打开后的持久结果均通过。验证原字节备份、管理员资料/型号/指令/版本保留、重启不重复升级。`TestInvalidCatalogIsNotRewrittenByUpgrade`覆盖未知字段、重复JSON键、无效版本：不改原文件，不错误生成升级备份。原缺失文件拒绝用例替换为告警及重新发布断言。

真实Windows EXE使用当前`data/server/repository`的独立副本，在随机本机HTTP/控制/data端口启动。GET模板、GET设备、POST新模板成功，备份与原设备目录字节相同；测试进程已停止。证据：`build/server-template-startup-fix/run-7bb045e5115246a7b9f99db8ce20cf8c/stdout.log`，生成了设备目录备份并继续启动。

Linux：Go 1.24.13 linux/amd64，复制当前Go源码至项目外WSL `RouterAgentTest`新目录（路径保存于`build/server-template-startup-fix/linux-run.txt`），执行：

```sh
go test -race ./internal/catalogupgrade ./internal/probetemplate ./internal/enrollment ./internal/management ./internal/api
go vet ./internal/catalogupgrade ./internal/probetemplate ./internal/enrollment ./internal/management ./internal/api
go build -o router-server ./cmd/server
```

均通过。生产数据没有清理或写入，用户Server/UI/设备Probe未重启，无Git提交/推送。本次不涉及Probe/协议/UI变更，未重跑C++/UI全量或厂商实机验收；此前实机限制不变。再次运行正常Server入口时才会升级原存量目录并保存备份。
