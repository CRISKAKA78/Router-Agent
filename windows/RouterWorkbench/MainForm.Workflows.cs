using System.Globalization;
using System.Text;
using RouterWorkbench.Core;

namespace RouterWorkbench;

public sealed partial class MainForm
{
    private void BuildMaintenance()
    {
        var panel = Layout((SizeType.Absolute, 45), (SizeType.Absolute, 125), (SizeType.Absolute, 62), (SizeType.Percent, 100));
        var controls = Row(); leaseUnit.Items.AddRange(["分钟", "毫秒"]); leaseUnit.SelectedIndex = 0;
        controls.Controls.AddRange([defaultLease, lease, leaseUnit, ActionButton("开启远程维护", CreateMaintenanceAsync, true),
            ActionButton("关闭所选维护", CloseMaintenanceAsync, true)]);
        panel.Controls.Add(controls, 0, 0);
        var entries = new TableLayoutPanel { Dock = DockStyle.Fill, ColumnCount = 3, RowCount = 3 };
        entries.ColumnStyles.Add(new(SizeType.Absolute, 85)); entries.ColumnStyles.Add(new(SizeType.Percent, 100)); entries.ColumnStyles.Add(new(SizeType.Absolute, 130));
        foreach (var service in new[] { "web", "ssh", "telnet" })
        {
            var box = new TextBox { ReadOnly = true, Dock = DockStyle.Fill, AccessibleName = service + " 入口" }; addresses.Add(service, box);
            var launch = ActionButton("打开 " + service.ToUpperInvariant(), () => OpenEndpointAsync(service)); launchButtons.Add(service, launch);
            entries.Controls.Add(new Label { Text = service.ToUpperInvariant(), AutoSize = true, Padding = new Padding(4, 6, 0, 0) }); entries.Controls.Add(box); entries.Controls.Add(launch);
        }
        panel.Controls.Add(entries, 0, 1); panel.Controls.Add(maintenanceInfo, 0, 2); panel.Controls.Add(maintenance, 0, 3);
        Page("远程维护", panel);
    }
    private void BuildDevice()
    {
        var panel = Layout((SizeType.Absolute, 38), (SizeType.Percent, 100));
        var row = Row(); row.Controls.Add(ActionButton("查看 Session 历史", async () => {
            var owner = RequireConnection(); var id = DeviceId ?? throw new InvalidOperationException("请选择设备");
            var sessions = await owner.ReadAsync((a, ct) => a.ListAsync<Session>($"devices/{Wire.Segment(id)}/sessions", ct));
            if (!ReferenceEquals(owner, connection) || id != DeviceId) return;
            using var dialog = new Form { Text = "Session 历史 · " + id, Size = new Size(950, 480), StartPosition = FormStartPosition.CenterParent };
            var grid = Grid(("SessionId", "Session", 280), ("StartedAt", "开始", 190), ("EndedAt", "结束", 190), ("EndReason", "结束原因", 160));
            grid.DataSource = sessions; dialog.Controls.Add(grid); dialog.ShowDialog(this);
        }));
        panel.Controls.Add(row, 0, 0); panel.Controls.Add(deviceDetails, 0, 1); Page("设备详情", panel);
    }
    private async Task CreateMaintenanceAsync()
    {
        var id = RequireDevice();
        object body;
        if (defaultLease.Checked) body = new { device_id = id };
        else
        {
            if (!decimal.TryParse(lease.Text, NumberStyles.Number, CultureInfo.InvariantCulture, out var value)) throw new ArgumentException("请输入正租期，使用小数点表示小数。");
            var ms = value * (leaseUnit.SelectedIndex == 0 ? 60000m : 1m);
            if (ms < 1 || ms > 9223372036854m || ms != decimal.Truncate(ms)) throw new ArgumentException("租期必须可表示为 1～9223372036854 的整数毫秒。");
            body = new { device_id = id, lease_ms = (long)ms };
        }
        await ExecuteAsync(Mutation.Json("创建远程维护", "maintenance", body));
    }
    private Task CloseMaintenanceAsync()
    {
        var current = CurrentMaintenance ?? throw new InvalidOperationException("请选择 Maintenance");
        return ExecuteAsync(Mutation.Json("关闭远程维护", $"maintenance/{Wire.Segment(current.MaintenanceId)}/close", new { }));
    }
    private void RenderMaintenance()
    {
        var current = CurrentMaintenance;
        foreach (var service in addresses.Keys)
        {
            var endpoint = current?.Endpoints.FirstOrDefault(e => e.Service == service);
            addresses[service].Text = endpoint == null ? "" : (endpoint.Url ?? endpoint.Address) + "   [" + endpoint.State + "]";
            launchButtons[service].Enabled = !busy && !closing && connection is { IsSynchronized: true } &&
                current is { State: "ready", Released: false } && endpoint is { State: not "closed" } &&
                CurrentDevice?.CurrentSession?.SessionId == current.SessionId && current.ExpiresAt > DateTimeOffset.UtcNow;
        }
        if (current == null) { maintenanceInfo.Text = snapshot?.MaintenanceError ?? "选设备 → 开启远程维护 → 打开三个临时入口。"; return; }
        var remaining = current.ExpiresAt - DateTimeOffset.UtcNow;
        var timeText = remaining > TimeSpan.Zero ? $"约 {Math.Floor(remaining.TotalHours):0} 小时 {remaining.Minutes:00} 分 {remaining.Seconds:00} 秒" : "已到显示期限，正在回查 Server";
        maintenanceInfo.Text = $"Server 状态：{current.State}  |  剩余：{timeText}  |  连接数：{current.Connections}\nSession：{current.SessionId}  |  到期：{current.ExpiresAt.LocalDateTime:yyyy-MM-dd HH:mm:ss}  |  释放：{current.Released} {current.Reason}";
        if (remaining <= TimeSpan.Zero && current.State != "closed") connection?.Invalidate();
    }
    private async Task OpenEndpointAsync(string service)
    {
        if (busy) return;
        var owner = RequireConnection(); var selected = CurrentMaintenance ?? throw new InvalidOperationException("请选择维护");
        busy = true; UpdateActions();
        try
        {
            var fresh = await owner.ReadAsync((a, ct) => a.GetAsync<Maintenance>($"maintenance/{Wire.Segment(selected.MaintenanceId)}", ct));
            if (!ReferenceEquals(owner, connection) || selected.MaintenanceId != CurrentMaintenance?.MaintenanceId || closing) return;
            if (fresh.State != "ready" || fresh.Released) throw new InvalidOperationException("Server 已关闭维护，请刷新。");
            openEndpoint(fresh.Endpoints.Single(e => e.Service == service), profile);
            errorStatus.Text = service.ToUpperInvariant() + " 已交给外部客户端；登录与主机密钥验证在该客户端完成。";
        }
        finally { busy = false; owner.Invalidate(); if (!closing) UpdateActions(); }
    }
    private void BuildTasks()
    {
        var panel = Layout((SizeType.Absolute, 70), (SizeType.Percent, 40), (SizeType.Absolute, 38), (SizeType.Percent, 60));
        var row = Row(); row.Controls.AddRange([command, new Label { Text = "超时(秒)", AutoSize = true, Padding = new Padding(0, 5, 0, 0) }, timeout,
            ActionButton("执行 Exec", () => ExecuteAsync(Mutation.Json("创建 Exec", "tasks", new { device_id = RequireDevice(), command = Required(command.Text, "命令"), timeout_seconds = (uint)timeout.Value })), true)]);
        panel.Controls.Add(row, 0, 0); panel.Controls.Add(tasks, 0, 1);
        var actions = Row();
        actions.Controls.Add(ActionButton("刷新结果", ReadTaskAsync));
        actions.Controls.Add(ActionButton("重发原 Task", () => ExecuteAsync(Mutation.Json("重发原 Task", $"tasks/{Wire.Segment(RequireTask().TaskId)}/resend", new { })), true));
        actions.Controls.Add(ActionButton("导入已完成下载", () => ExecuteAsync(Mutation.Json("导入下载", $"downloads/{Wire.Segment(RequireTask().TaskId)}/complete", new { })), true));
        actions.Controls.Add(ActionButton("清理下载暂存", () => ExecuteAsync(Mutation.Json("清理下载暂存", $"downloads/{Wire.Segment(RequireTask().TaskId)}/cleanup", new { })), true));
        panel.Controls.Add(actions, 0, 2); panel.Controls.Add(taskOutput, 0, 3); Page("Task / Exec", panel);
    }
    private static string Required(string value, string label) => string.IsNullOrWhiteSpace(value) ? throw new ArgumentException(label + "不能为空") : value;
    private TaskSummary RequireTask() => CurrentTask ?? throw new InvalidOperationException("请选择 Task");
    private Task ReadTaskAsync()
    {
        taskRevision++;
        return CoalesceReadAsync("task", () => ReadTaskCoreAsync(taskRevision));
    }
    private async Task ReadTaskCoreAsync(int revision)
    {
        var selected = CurrentTask; var owner = connection;
        if (selected == null || owner == null) { taskOutput.Clear(); return; }
        try
        {
            var detail = await owner.ReadAsync((a, ct) => a.GetAsync<TaskDetail>($"tasks/{Wire.Segment(selected.TaskId)}", ct));
            Transfer? transfer = null;
            if (detail.Type is "upload" or "download")
                transfer = await owner.ReadAsync((a, ct) => a.GetAsync<Transfer>($"tasks/{Wire.Segment(selected.TaskId)}/transfer", ct));
            if (!ReferenceEquals(owner, connection) || revision != taskRevision || closing) return;
            var text = new StringBuilder($"Task：{detail.TaskId}\n状态：{detail.State}  派发次数：{detail.DispatchCount}\n最近派发 Session：{detail.LastSessionId}\n命令：{detail.Command}\n超时：{detail.TimeoutSeconds} 秒\n");
            if (transfer != null) text.Append($"\n文件本地事实：Committed={transfer.Committed}  Released={transfer.Released}  Failed={transfer.Failed}\n字节：{transfer.Size}  SHA-256：{transfer.Sha256}\n");
            if (detail.Result is { } result)
                text.Append($"\n最终 RESULT：{result.Status}  ExitCode={result.ExitCode}  截断={result.Truncated}\n开始：{result.StartedAt}\n结束：{result.FinishedAt}\n\nstdout:\n{result.Stdout}\n\nstderr:\n{result.Stderr}");
            else text.Append(detail.State == "rejected" ? "\nProbe 已最终拒绝；没有 RESULT。" : "\n尚无最终 RESULT；以上状态来自 Server。" );
            taskOutput.Text = text.ToString();
        }
        catch (Exception e) { if (ReferenceEquals(owner, connection) && revision == taskRevision && !closing) taskOutput.Text = Errors.Describe(e, "查询 Task / 文件状态"); }
    }
    private void BuildFiles()
    {
        var panel = Layout((SizeType.Absolute, 76), (SizeType.Percent, 100));
        var row = Row();
        row.Controls.Add(ActionButton("导入本地文件", ImportAssetAsync, true));
        row.Controls.Add(ActionButton("另存到本机", SaveAssetAsync));
        row.Controls.Add(ActionButton("上传到设备", UploadAssetAsync, true));
        row.Controls.Add(ActionButton("从设备下载", DownloadAsync, true));
        row.Controls.Add(ActionButton("归档所选资产", () => ExecuteAsync(Mutation.Json("归档资产", $"assets/{Wire.Segment((CurrentAsset ?? throw new InvalidOperationException("请选择资产")).AssetId)}/archive", new { })), true));
        row.Controls.Add(new Label { Text = "导入保存到 Server 仓库；从设备下载完成后，请在 Task 页显式导入。归档保留文件与身份。", AutoSize = true });
        panel.Controls.Add(row, 0, 0); panel.Controls.Add(assets, 0, 1); Page("文件资产", panel);
    }
    private async Task ImportAssetAsync()
    {
        if (busy) return;
        using var dialog = new OpenFileDialog { CheckFileExists = true, Title = "导入 Server 文件仓库" };
        if (dialog.ShowDialog(this) != DialogResult.OK) return;
        var owner = RequireConnection(); busy = true; UpdateActions();
        Mutation request;
        try { request = await Task.Run(() => Mutation.ImportAsync(dialog.FileName, lifetime.Token)); }
        finally { busy = false; if (!closing) UpdateActions(); }
        if (ReferenceEquals(owner, connection) && !closing) await ExecuteAsync(request);
    }
    private async Task SaveAssetAsync()
    {
        if (busy) return;
        var asset = CurrentAsset ?? throw new InvalidOperationException("请选择资产"); var owner = RequireConnection();
        using var dialog = new SaveFileDialog { FileName = Path.GetFileName(asset.Name), Title = "另存资产到本机", OverwritePrompt = true };
        if (dialog.ShowDialog(this) != DialogResult.OK) return;
        busy = true; UpdateActions();
        try { await owner.SaveAssetAsync(asset, dialog.FileName); if (ReferenceEquals(owner, connection)) errorStatus.Text = "文件已保存，长度与 SHA-256 校验通过。"; }
        finally { busy = false; if (!closing) UpdateActions(); }
    }
    private async Task UploadAssetAsync()
    {
        var id = RequireDevice(); var asset = CurrentAsset ?? throw new InvalidOperationException("请选择资产");
        using var dialog = new InputDialog("上传到设备 · " + id, ("path", "设备绝对路径", "/tmp/" + Path.GetFileName(asset.Name)), ("mode", "权限(四位八进制)", "0644"), ("overwrite", "覆盖现有文件(true/false)", "false"), ("timeout", "超时(秒)", "60"));
        if (dialog.ShowDialog(this) != DialogResult.OK) return;
        await ExecuteAsync(Mutation.Json("上传文件", "uploads", new { device_id = id, asset_id = asset.AssetId, remote_path = dialog["path"], mode = dialog["mode"], overwrite = bool.Parse(dialog["overwrite"]), timeout_seconds = PositiveTimeout(dialog["timeout"]) }));
    }
    private async Task DownloadAsync()
    {
        var id = RequireDevice();
        using var dialog = new InputDialog("从设备下载 · " + id, ("path", "设备绝对路径", "/tmp/diagnostic.txt"), ("name", "资产名称", "diagnostic.txt"), ("timeout", "超时(秒)", "60"));
        if (dialog.ShowDialog(this) != DialogResult.OK) return;
        await ExecuteAsync(Mutation.Json("从设备下载", "downloads", new { device_id = id, remote_path = dialog["path"], name = dialog["name"], timeout_seconds = PositiveTimeout(dialog["timeout"]) }));
    }
    private static uint PositiveTimeout(string text) => uint.TryParse(text, out var value) && value > 0 ? value : throw new ArgumentException("超时必须为正整数秒");
    private void BuildTools()
    {
        var panel = Layout((SizeType.Absolute, 38), (SizeType.Percent, 30), (SizeType.Absolute, 45), (SizeType.Percent, 35), (SizeType.Percent, 35));
        var row = Row();
        row.Controls.Add(ActionButton("创建工具", async () => {
            using var d = new InputDialog("创建工具", ("name", "名称", ""), ("description", "说明", ""));
            if (d.ShowDialog(this) == DialogResult.OK) await ExecuteAsync(Mutation.Json("创建工具", "tools", new { name = Required(d["name"], "名称"), description = d["description"] }));
        }, true));
        row.Controls.Add(ActionButton("发布版本…", PublishAsync, true));
        row.Controls.Add(ActionButton("归档工具", () => ExecuteAsync(Mutation.Json("归档工具", $"tools/{Wire.Segment(RequireTool().ToolId)}/archive", new { })), true));
        panel.Controls.Add(row, 0, 0); panel.Controls.Add(toolsGrid, 0, 1);
        var versionRow = Row(); versionRow.Controls.Add(new Label { Text = "版本", AutoSize = true, Padding = new Padding(0, 5, 0, 0) }); versionRow.Controls.Add(versions);
        versionRow.Controls.Add(ActionButton("刷新兼容性", ReadCompatibilityAsync));
        versionRow.Controls.Add(ActionButton("投放所选产物", DeployAsync, true));
        versionRow.Controls.Add(ActionButton("归档版本", () => ExecuteAsync(Mutation.Json("归档版本", VersionPath() + "/archive", new { })), true));
        panel.Controls.Add(versionRow, 0, 2); panel.Controls.Add(artifacts, 0, 3); panel.Controls.Add(compatibilityDetails, 0, 4); Page("工具 / 版本", panel);
    }
    private Tool RequireTool() => CurrentTool ?? throw new InvalidOperationException("请选择工具");
    private string VersionPath() => $"tools/{Wire.Segment(RequireTool().ToolId)}/versions/{Wire.Segment((CurrentVersion ?? throw new InvalidOperationException("请选择版本")).Version)}";
    private Task ReadToolsAsync()
    {
        toolRevision++;
        return CoalesceReadAsync("tools", () => ReadToolsCoreAsync(toolRevision));
    }
    private async Task ReadToolsCoreAsync(int revision)
    {
        var tool = CurrentTool; var owner = connection;
        if (loadedToolId != tool?.ToolId)
        {
            loadedToolId = tool?.ToolId;
            binding = true; versions.DataSource = null; artifacts.DataSource = null; binding = false;
            compatibilityDetails.Clear();
        }
        if (tool == null || owner == null) { binding = true; versions.DataSource = null; artifacts.DataSource = null; binding = false; compatibilityDetails.Clear(); return; }
        try
        {
            var values = await owner.ReadAsync((a, ct) => a.ListAsync<ToolVersion>($"tools/{Wire.Segment(tool.ToolId)}/versions?include_archived=true", ct));
            if (revision != toolRevision || !ReferenceEquals(owner, connection) || closing) return;
            var previous = CurrentVersion?.Version;
            binding = true; versions.DisplayMember = "Version"; versions.DataSource = values;
            if (previous != null) versions.SelectedItem = values.FirstOrDefault(v => v.Version == previous) ?? values.FirstOrDefault();
            binding = false; await ReadCompatibilityAsync();
        }
        catch (Exception e) { if (revision == toolRevision && ReferenceEquals(owner, connection) && !closing) compatibilityDetails.Text = Errors.Describe(e, "查询工具版本"); }
    }
    private string? loadedToolId;
    private int compatibilityRevision;
    private string? compatibilityKey;
    private sealed record ArtifactRow(string ArtifactId, string AssetId, string Status, string Mode, Artifact Artifact, MatchCheck[] Checks);
    private Task ReadCompatibilityAsync()
    {
        compatibilityRevision++;
        return CoalesceReadAsync("compatibility", () => ReadCompatibilityCoreAsync(compatibilityRevision));
    }
    private async Task ReadCompatibilityCoreAsync(int revision)
    {
        var version = CurrentVersion; var tool = CurrentTool; var device = DeviceId; var owner = connection;
        var key = $"{tool?.ToolId}\n{version?.Version}\n{device}\n{CurrentDevice?.CurrentSession?.SessionId}";
        if (compatibilityKey != key) { compatibilityKey = key; artifacts.DataSource = null; compatibilityDetails.Clear(); }
        if (version == null || tool == null || owner == null) { artifacts.DataSource = null; compatibilityDetails.Clear(); return; }
        try
        {
            ArtifactRow[] rows;
            if (device == null) rows = version.Artifacts.Select(a => new ArtifactRow(a.ArtifactId, a.AssetId, "请选择设备", a.Mode, a, [])).ToArray();
            else
            {
                var matches = await owner.ReadAsync((a, ct) => a.ListAsync<Compatibility>($"tools/{Wire.Segment(tool.ToolId)}/versions/{Wire.Segment(version.Version)}/compatibility?device_id={Wire.Segment(device)}", ct));
                rows = matches.Select(m => new ArtifactRow(m.Artifact.ArtifactId, m.Artifact.AssetId, m.Status, m.Artifact.Mode, m.Artifact, m.Checks)).ToArray();
            }
            if (revision != compatibilityRevision || !ReferenceEquals(owner, connection) || closing) return;
            Bind(artifacts, rows, a => a.ArtifactId); ShowCompatibility();
        }
        catch (Exception e) { if (revision == compatibilityRevision && ReferenceEquals(owner, connection) && !closing) { artifacts.DataSource = null; compatibilityDetails.Text = Errors.Describe(e, "查询兼容产物"); } }
    }
    private void ShowCompatibility()
    {
        if (artifacts.CurrentRow?.DataBoundItem is not ArtifactRow row) { compatibilityDetails.Clear(); return; }
        var rules = row.Artifact.Rules;
        compatibilityDetails.Text = $"平台：{row.Artifact.Platform}  权限：{row.Mode}  兼容：{row.Status}\n架构：{string.Join(", ", rules.Arch)}\nlibc：{string.Join(", ", rules.Libc)}\n型号：{string.Join(", ", rules.Models ?? [])}\n内核：{string.Join(", ", rules.Kernels ?? [])}\n能力：{string.Join(", ", rules.RequiredCapabilities ?? [])}\n" +
            string.Join("\n", row.Checks.Select(c => $"{c.Field}: {c.Status} — {c.Reason}"));
    }
    private async Task DeployAsync()
    {
        var device = RequireDevice(); var tool = RequireTool(); var version = CurrentVersion ?? throw new InvalidOperationException("请选择版本");
        var artifact = artifacts.CurrentRow?.DataBoundItem as ArtifactRow ?? throw new InvalidOperationException("请选择产物");
        if (artifact.Status != "compatible") throw new InvalidOperationException("请选择 Server 判定为 compatible 的产物。");
        using var d = new InputDialog("投放工具（仅上传，不自动执行）", ("path", "设备绝对路径", "/tmp/tool"), ("overwrite", "覆盖现有文件(true/false)", "false"), ("timeout", "超时(秒)", "60"));
        if (d.ShowDialog(this) != DialogResult.OK) return;
        await ExecuteAsync(Mutation.Json("投放工具", "deployments", new { device_id = device, tool_id = tool.ToolId, version = version.Version, artifact_id = artifact.ArtifactId, remote_path = d["path"], overwrite = bool.Parse(d["overwrite"]), timeout_seconds = PositiveTimeout(d["timeout"]) }));
    }
    private async Task PublishAsync()
    {
        var tool = RequireTool();
        using var dialog = new PublishDialog(snapshot?.Assets.Where(a => !a.Archived).ToArray() ?? []);
        if (dialog.ShowDialog(this) != DialogResult.OK) return;
        await ExecuteAsync(Mutation.Json("发布工具版本", $"tools/{Wire.Segment(tool.ToolId)}/versions/{Wire.Segment(dialog.Version)}", new { artifacts = dialog.Specifications }, "PUT"));
    }
}
