using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private DataGrid tasksGrid = null!, transferGrid = null!;
    private ComboBox taskScope = null!, taskState = null!;
    private TextBox taskSearch = null!;
    private TextBlock taskHeading = null!, taskFooter = null!, taskEmpty = null!;
    private string taskId = "";
    private TaskDetail? task;
    private Transfer? transfer;
    private Operation? operation;
    private CancellationTokenSource detailsCancel = new();
    private bool detailsRunning, detailsDirty;
    private string lastDetailError = "";
    private Button saveDownloadButton = null!;
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
        transferGrid = PropertySheet.Create("传输详情");
        taskHeading = Ui.Text("未选择传输", true);taskFooter=Ui.Text("",true);taskFooter.TextWrapping=TextWrapping.Wrap;
        saveDownloadButton=Ui.Button("保存已下载文件…",()=>_ = Run("保存下载",SaveCompletedDownload));
        var detail=Ui.Page(Ui.Bar(saveDownloadButton,taskHeading),transferGrid,Ui.Note(""));
        var footer=(DockPanel)detail;footer.Children.RemoveAt(1);DockPanel.SetDock(taskFooter,Dock.Bottom);footer.Children.Insert(1,taskFooter);
        return Ui.Page(Ui.Bar(taskScope,taskState,taskSearch,Ui.Button("刷新",()=>{connection?.Invalidate();_ = RefreshDetails();})),Ui.Split(list,detail));
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
        if(transferHistory!=null) transferHistory.Header=$"传输记录（{rows.Length}）";
        UpdateTaskDetail();
    }
    private void ShowCreatedTask(JsonElement response)
    {
        if (!response.TryGetProperty("task_id", out var id)) return;
        taskId = id.GetString()!; task = null; transfer = null; operation = null; UpdateTasks();
        _ = RefreshDetails();
    }
    private void UpdateTaskDetail()
    {
        if(saveDownloadButton==null)return;
        saveDownloadButton.IsEnabled=Writable && Device?.DeviceId==task?.DeviceId && task?.Type=="download" && transfer is {Committed:true,Released:true};
        saveDownloadButton.ToolTip=saveDownloadButton.IsEnabled?"将所选已完整接收的文件保存到本机":"请选择当前设备已完整接收的下载记录";
        if(task==null){taskHeading.Text=taskId==""?"未选择传输":"正在查询原传输…";taskFooter.Text="";transferGrid.ItemsSource=null;return;}
        taskHeading.Text=task.TypeText+" · "+task.StateText;
        taskFooter.Text=task.Result==null?"等待设备确认结果":task.Result.Status=="success"?"设备已确认完成":task.StateText+" · "+task.Result.Stderr;
        var facts=new List<PropertyRow> {new("","设备",task.DeviceId),new("","操作",task.TypeText),new("","状态",task.StateText)};
        if(transfer!=null){facts.Add(new("","文件大小",Labels.Bytes(transfer.Size)));facts.Add(new("","文件接收",transfer.Committed?"完整文件已提交":transfer.Failed?"传输失败":"等待完整文件"));}
        if(operation is {ToolId.Length:>0}){facts.Add(new("","工具",snapshot.Tools.FirstOrDefault(t=>t.ToolId==operation.ToolId)?.Name??operation.ToolId));facts.Add(new("","版本",operation.Version));}
        if(exchange?.TaskId==task.TaskId){facts.Add(new("","设备路径",exchange.RemotePath));facts.Add(new("","本地路径",exchange.LocalPath));facts.Add(new("","本地状态",exchange.Status));}
        transferGrid.ItemsSource=facts;TableBehavior.FitPropertyColumns(transferGrid,facts);
    }
    private async Task SaveCompletedDownload()
    {
        var selected=task;if(selected?.Type!="download"||transfer is not {Committed:true,Released:true})throw new InvalidOperationException("请选择已完整接收的下载记录。");
        var device=RequireDevice(false);if(selected.DeviceId!=device.DeviceId)throw new InvalidOperationException("请先选择这条记录对应的设备。");
        var picker=new Microsoft.Win32.SaveFileDialog {Title="保存已下载文件",FileName=exchange?.TaskId==selected.TaskId?System.IO.Path.GetFileName(exchange.LocalPath):"download.bin",OverwritePrompt=true};
        if(picker.ShowDialog(this)!=true)return;
        exchange=FileExchange.ExistingDownload(Connected(),selected.DeviceId,selected.TaskId,picker.FileName);
        await RunExchange(exchange);
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
                var rows = await c.TrackAsync(() => c.Api.ListAsync<ConnectionPeriod>($"devices/{Id(device)}/connections", cancel));
                if (c == connection && device == selectedDevice && !cancel.IsCancellationRequested) SetConnectionHistory(rows);
            }
            if ((page == "files" || page == "tools") && id != "") {
                var detail = await c.TrackAsync(() => c.Api.GetAsync<TaskDetail>($"tasks/{Id(id)}", cancel));
                Transfer? file = null; Operation? op = null;
                if (detail.Type is not ("exec" or "router_config")) {
                    file = await c.TrackAsync(() => c.Api.GetAsync<Transfer>($"tasks/{Id(id)}/transfer", cancel));
                    op = await c.TrackAsync(() => c.Api.GetAsync<Operation>($"tasks/{Id(id)}/operation", cancel));
                }
                if (c == connection && id == taskId && !cancel.IsCancellationRequested) { task = detail; transfer = file; operation = op; UpdateTaskDetail(); if(page=="tools")toolStatus.Text=detail.Result==null?"投放中 · "+detail.StateText:detail.Result.Status=="success"?"工具已投放到设备。":"投放"+detail.StateText+" · "+detail.Result.Stderr; }
            }
            if (page == "config" && configTaskId != "") {
                var configId = configTaskId;
                var detail = await c.TrackAsync(() => c.Api.GetAsync<TaskDetail>($"tasks/{Id(configId)}", cancel));
                if (c == connection && device == selectedDevice && configId == configTaskId && !cancel.IsCancellationRequested)
                    ShowConfigResult(detail);
            }
            lastDetailError = "";
        } catch (OperationCanceledException) { }
        catch (Exception e) { if (c == connection && !cancel.IsCancellationRequested && lastDetailError != e.Message) { lastDetailError = e.Message; Log("错误", "详情查询：" + e.Message); } }
        finally { detailsRunning = false; if (detailsDirty && !closing) { detailsDirty = false; _ = RefreshDetails(); } }
    }
}
