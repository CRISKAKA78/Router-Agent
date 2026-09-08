using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private DataGrid tasksGrid = null!, transferGrid = null!;
    private ComboBox taskScope = null!, taskState = null!;
    private TextBox taskSearch = null!, stdout = null!, stderr = null!, taskSpec = null!;
    private TextBlock taskHeading = null!, taskFooter = null!, taskEmpty = null!;
    private string taskId = "";
    private TaskDetail? task;
    private Transfer? transfer;
    private Operation? operation;
    private CancellationTokenSource detailsCancel = new();
    private bool detailsRunning, detailsDirty;
    private string lastDetailError = "";
    private UIElement BuildTransfers()
    {
        taskScope = Ui.Combo(["当前设备", "全部设备"]); taskState = Ui.Combo(["全部状态", "执行中", "成功", "失败 / 拒绝"]);
        taskSearch = Ui.Input("", 210); taskSearch.Tag = "搜索任务 / 设备"; taskSearch.ToolTip = "搜索任务 ID、设备或类型";
        taskScope.SelectionChanged += (_, _) => { if (!refreshing) UpdateTasks(); }; taskState.SelectionChanged += (_, _) => { if (!refreshing) UpdateTasks(); };
        taskSearch.TextChanged += (_, _) => { if (!refreshing) UpdateTasks(); };
        tasksGrid = Ui.Table("文件传输列表", ("时间", "CreatedText", 158), ("类型", "TypeText", 72), ("状态", "StateText", 90), ("设备", "DeviceId", 150), ("任务 ID", "TaskId", -1));
        tasksGrid.SelectionChanged += (_, _) => { if (refreshing) return; taskId = (tasksGrid.SelectedItem as TaskSummary)?.TaskId ?? ""; task = null; transfer = null; operation = null; CancelDetails(); UpdateTaskDetail(); _ = RefreshDetails(); };
        tasksGrid.MouseDoubleClick += (_, _) => { if (task != null) Inspect("任务详情", task); };
        taskEmpty = Ui.Text("当前筛选下没有任务", true); taskEmpty.HorizontalAlignment = HorizontalAlignment.Center; taskEmpty.VerticalAlignment = VerticalAlignment.Top; taskEmpty.Margin = new(10,55,10,0); taskEmpty.IsHitTestVisible = false;
        var list = new Grid(); list.Children.Add(tasksGrid); list.Children.Add(taskEmpty);
        stdout = Ui.Code("选择任务以查看最终输出。", true); stderr = Ui.Code("", true); taskSpec = Ui.Code("", true);
        transferGrid = Ui.Table("文件传输事实", ("属性", "Name", 170), ("值", "Value", -1));
        var detailTabs = new TabControl(); detailTabs.Items.Add(new TabItem { Header = "标准输出", Content = stdout }); detailTabs.Items.Add(new TabItem { Header = "标准错误", Content = stderr });
        detailTabs.Items.Add(new TabItem { Header = "任务规格", Content = taskSpec }); detailTabs.Items.Add(new TabItem { Header = "传输 / 资产", Content = transferGrid });
        taskHeading = Ui.Text("未选择任务", true); taskHeading.Margin = new(10,0,0,0);
        taskFooter = Ui.Text("202 表示已记录或派发；任务完成以最终 RESULT 为准。", true); taskFooter.Margin = new(10,7,10,7); taskFooter.TextWrapping = TextWrapping.Wrap;
        var detail = Ui.Page(Ui.Bar(
            Ui.Button("重发原任务…", () => _ = Run("重发任务", ResendTask)),
            Ui.Button("下载入库", () => _ = Run("下载入库", CompleteDownload)),
            Ui.Button("清理下载暂存…", () => _ = Run("清理暂存", CleanupDownload)), taskHeading), detailTabs, taskFooter);
        return Ui.Page(Ui.Bar(taskScope, taskState, taskSearch, Ui.Button("刷新", () => { connection?.Invalidate(); _ = RefreshDetails(); })), Ui.Split(list, detail, true, 1.1));
    }
    private void UpdateTasks()
    {
        if (tasksGrid == null) return;
        var rows = snapshot.Tasks.Where(t => (t.Type is "upload" or "download") && (taskScope.SelectedIndex == 1 || t.DeviceId == selectedDevice)
            && $"{t.TaskId} {t.DeviceId} {t.TypeText}".Contains(taskSearch.Text, StringComparison.OrdinalIgnoreCase)
            && (taskState.SelectedIndex switch { 1 => t.State is not ("success" or "succeeded" or "failed" or "rejected" or "timeout" or "timed_out" or "completed"), 2 => t.State is "success" or "succeeded" or "completed", 3 => t.State is "failed" or "rejected" or "timeout" or "timed_out", _ => true })).OrderByDescending(t => t.CreatedAt).ToArray();
        var was = refreshing; refreshing = true;
        Ui.SetRows(tasksGrid, rows); tasksGrid.SelectedItem = rows.FirstOrDefault(t => t.TaskId == taskId); refreshing = was;
        taskEmpty.Visibility = rows.Length == 0 ? Visibility.Visible : Visibility.Collapsed;
        UpdateTaskDetail();
    }
    private void ShowCreatedTask(JsonElement response)
    {
        if (!response.TryGetProperty("task_id", out var id)) return;
        taskId = id.GetString()!; task = null; transfer = null; operation = null; Navigate("files"); fileTabs.SelectedIndex = 1;
        _ = RefreshDetails();
    }
    private void UpdateTaskDetail()
    {
        if (task == null) { taskHeading.Text = taskId == "" ? "未选择任务" : "正在查询原任务…"; stdout.Text = taskId == "" ? "选择任务以查看输出。" : "正在查询任务结果…"; stderr.Clear(); taskSpec.Clear(); transferGrid.ItemsSource = null; return; }
        taskHeading.Text = $"{task.TypeText} · {task.StateText}";
        stdout.Text = task.Result?.Stdout ?? "尚无最终 RESULT"; stderr.Text = task.Result?.Stderr ?? "";
        taskSpec.Text = ApiJson.Pretty(new { task.TaskId, task.DeviceId, task.Type, task.State, task.Command, task.Cwd, task.TimeoutSeconds, task.Env, Params = task.Params.ValueKind == JsonValueKind.Undefined ? (JsonElement?)null : task.Params, task.LastSessionId, task.DispatchCount, task.CreatedAt });
        taskFooter.Text = task.Result == null ? $"原任务 {task.TaskId} · 已派发 {task.DispatchCount} 次 · 尚无最终 RESULT" : $"原任务 {task.TaskId} · 退出码 {task.Result.ExitCode} · {Labels.Time(task.Result.FinishedAt)}" + (task.Result.Truncated ? " · 输出已截断" : "");
        var facts = new List<PropertyRow>();
        if (transfer != null) facts.AddRange([new("", "完整提交 committed", transfer.Committed.ToString()), new("", "句柄释放 released", transfer.Released.ToString()), new("", "传输失败 failed", transfer.Failed.ToString()), new("", "长度", Labels.Bytes(transfer.Size)), new("", "SHA-256", transfer.Sha256)]);
        if (operation != null) facts.AddRange([new("", "资产 ID", operation.AssetId), new("", "工具 ID", operation.ToolId), new("", "版本", operation.Version), new("", "产物 ID", operation.ArtifactId)]);
        if (facts.Count == 0) facts.Add(new("", "传输", "此任务没有文件传输"));
        transferGrid.ItemsSource = facts;
    }
    private async Task ResendTask()
    {
        var id = taskId; if (id == "") throw new InvalidOperationException("请选择任务。");
        if (!Confirm("重发原任务", "确认服务器未重启且目标仍是原探针进程。使用原 task_id 重发规格，查询或补报结果；不会创建替代任务。继续？")) return;
        await Write(new("重发原任务", $"tasks/{Id(id)}/resend")); _ = RefreshDetails();
    }
    private async Task CompleteDownload()
    {
        if (task?.Type != "download" || transfer is not { Committed: true, Released: true }) throw new InvalidOperationException("请选择已完整提交且释放的下载任务。");
        var result = await Write(new("下载入库", $"downloads/{Id(task.TaskId)}/complete"));
        if (result.TryGetProperty("asset", out var asset)) Log("资产", "已入库 " + asset.GetProperty("asset_id").GetString() + "；Task RESULT 单独保留。");
        if (result.TryGetProperty("cleanup_pending", out var pending) && pending.ValueKind == JsonValueKind.True) Log("提示", "资产已成功入库，暂存清理仍待完成。");
        _ = RefreshDetails();
    }
    private async Task CleanupDownload()
    {
        if (task?.Type != "download") throw new InvalidOperationException("请选择下载任务。");
        if (!Confirm("清理下载暂存", "仅请求清理该下载已释放的服务器暂存。尚未入库的完整文件会由服务器拒绝清理。继续？")) return;
        await Write(new("清理下载暂存", $"downloads/{Id(task.TaskId)}/cleanup"));
    }
    private void CancelDetails() { detailsCancel.Cancel(); detailsCancel.Dispose(); detailsCancel = new(); detailsDirty = true; }
    private async Task RefreshDetails()
    {
        if (detailsRunning) { detailsDirty = true; return; }
        var c = connection; if (c?.Synchronized != true || closing) return;
        detailsRunning = true; detailsDirty = false;
        var cancel = detailsCancel.Token; var device = selectedDevice; var id = taskId; var page = Page;
        try {
            if (page == "overview" && device != "") {
                var rows = await c.TrackAsync(() => c.Api.ListAsync<DeviceSession>($"devices/{Id(device)}/sessions", cancel));
                if (c == connection && device == selectedDevice && !cancel.IsCancellationRequested) sessions.ItemsSource = rows;
            }
            if (page == "files" && id != "") {
                var detail = await c.TrackAsync(() => c.Api.GetAsync<TaskDetail>($"tasks/{Id(id)}", cancel));
                Transfer? file = null; Operation? op = null;
                if (detail.Type is not ("exec" or "router_config")) {
                    file = await c.TrackAsync(() => c.Api.GetAsync<Transfer>($"tasks/{Id(id)}/transfer", cancel));
                    op = await c.TrackAsync(() => c.Api.GetAsync<Operation>($"tasks/{Id(id)}/operation", cancel));
                }
                if (c == connection && id == taskId && !cancel.IsCancellationRequested) { task = detail; transfer = file; operation = op; UpdateTaskDetail(); }
            }
            if (page == "config" && configTaskId != "") {
                var configId = configTaskId;
                var detail = await c.TrackAsync(() => c.Api.GetAsync<TaskDetail>($"tasks/{Id(configId)}", cancel));
                if (c == connection && device == selectedDevice && configId == configTaskId && !cancel.IsCancellationRequested)
                    configOutput.Text = detail.Result == null ? $"{detail.StateText} · 等待最终结果 ({configId})" : $"{detail.StateText} · 退出码 {detail.Result.ExitCode}" + (detail.Result.Truncated ? " · 输出已截断" : "") + "\n" + detail.Result.Stdout + (detail.Result.Stderr == "" ? "" : "\n" + detail.Result.Stderr);
            }
            lastDetailError = "";
        } catch (OperationCanceledException) { }
        catch (Exception e) { if (c == connection && !cancel.IsCancellationRequested && lastDetailError != e.Message) { lastDetailError = e.Message; Log("错误", "详情查询：" + e.Message); } }
        finally { detailsRunning = false; if (detailsDirty && !closing) { detailsDirty = false; _ = RefreshDetails(); } }
    }
}
