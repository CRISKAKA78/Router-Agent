using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Windowing;
using Microsoft.UI.Dispatching;
using RouterWorkbench.Core;
using System.Text.Json;
using Windows.Graphics;

namespace RouterWorkbench;
public sealed partial class MainWindow : Window
{
    internal readonly WorkbenchViewModel Model;
    private readonly string profilePath;
    private readonly Action<Endpoint, ServerProfile> launch;
    private readonly DispatcherQueueTimer timer;
    private ServerProfile profile = new();
    private Snapshot? renderedSnapshot;
    private string? renderedDevice;
    private string renderedNotice = "";
    private bool binding;
    private bool closeReady;
    private ContentDialog? activeDialog;
    private long? leaseMs;
    private TaskDetail? taskDetail;
    private readonly Dictionary<string, (bool Dirty, Func<Task> Read)> reads = [];
    private readonly HashSet<Task> readWorkers = [];
    private int selectionRevision;
    internal MainWindow(string? path = null, Action<Endpoint, ServerProfile>? open = null)
    {
        InitializeComponent();
        profilePath = path ?? ServerProfile.DefaultPath;
        launch = open ?? EndpointLauncher.Open;
        Title = "远程维护工作台";
        SystemBackdrop = new MicaBackdrop();
        ExtendsContentIntoTitleBar = true; SetTitleBar(TitleRegion);
        Root.ActualThemeChanged += (_, _) => UpdateTitleBar();
        AppWindow.Resize(new SizeInt32(1280, 920));
        if (AppWindow.Presenter is OverlappedPresenter presenter) presenter.PreferredMinimumWidth = 960;
        Model = new(action => DispatcherQueue.TryEnqueue(() => action()));
        Model.Changed += Render;
        timer = DispatcherQueue.CreateTimer(); timer.Interval = TimeSpan.FromSeconds(1);
        timer.Tick += (_, _) => Model.Tick(); timer.Start();
        AppWindow.Closing += async (_, e) => {
            if (closeReady) return;
            e.Cancel = true;
            if (Model.Closing) return;
            activeDialog?.Hide(); timer.Stop();
            await Model.DisposeAsync();
            await Task.WhenAll(readWorkers.ToArray());
            closeReady = true; Close();
        };
        Root.SizeChanged += (_, _) => SidebarColumn.Width = new GridLength(Root.ActualWidth < 1100 ? 224 : 272);
        Root.Loaded += (_, _) => {
            var scale = Root.XamlRoot.RasterizationScale;
            void SizeLimits() { if (AppWindow.Presenter is OverlappedPresenter p) { p.PreferredMinimumWidth = (int)(960 * Root.XamlRoot.RasterizationScale); p.PreferredMinimumHeight = (int)(480 * Root.XamlRoot.RasterizationScale); } }
            SizeLimits(); Root.XamlRoot.Changed += (_, _) => SizeLimits();
            var area = DisplayArea.GetFromWindowId(AppWindow.Id, DisplayAreaFallback.Primary).WorkArea;
            AppWindow.Resize(new SizeInt32(Math.Min(area.Width, (int)(1280 * scale)), Math.Min(area.Height, (int)(920 * scale))));
        };
        var template = (DataTemplate)Microsoft.UI.Xaml.Markup.XamlReader.Load("<DataTemplate xmlns='http://schemas.microsoft.com/winfx/2006/xaml/presentation'><StackPanel Padding='4,10' Spacing='5'><TextBlock Text='{Binding Title}' FontWeight='SemiBold' TextTrimming='CharacterEllipsis'/><TextBlock Text='{Binding Subtitle}' Opacity='0.65' FontSize='12' TextWrapping='Wrap'/></StackPanel></DataTemplate>");
        foreach (var list in new[] { TaskList, AssetList, ToolList, ArtifactList }) list.ItemTemplate = template;
        try { profile = ServerProfile.Load(profilePath); ApplyTheme(profile.Theme); }
        catch (Exception e) { Model.SetNotice(Errors.Describe(e, "读取设置"), true); }
        Render();
    }
    internal void ApplyTheme(string theme) => Root.RequestedTheme = theme switch { "Light" => ElementTheme.Light, "Dark" => ElementTheme.Dark, _ => ElementTheme.Default };
    private void UpdateTitleBar()
    {
        AppWindow.TitleBar.ButtonBackgroundColor = Microsoft.UI.Colors.Transparent;
        AppWindow.TitleBar.ButtonInactiveBackgroundColor = Microsoft.UI.Colors.Transparent;
        AppWindow.TitleBar.ButtonForegroundColor = Root.ActualTheme == ElementTheme.Dark ? Microsoft.UI.Colors.White : Microsoft.UI.Colors.Black;
        AppWindow.TitleBar.ButtonInactiveForegroundColor = Root.ActualTheme == ElementTheme.Dark ? Microsoft.UI.Colors.LightGray : Microsoft.UI.Colors.DimGray;
    }
    private static Visibility ToVisibility(bool value) => value ? Visibility.Visible : Visibility.Collapsed;
    private string? TaskId => (TaskList.SelectedItem as ListItem)?.Id;
    private Asset? SelectedAsset => (AssetList.SelectedItem as ListItem)?.Value as Asset;
    private Tool? SelectedTool => (ToolList.SelectedItem as ListItem)?.Value as Tool;
    private ToolVersion? SelectedVersion => VersionList.SelectedItem as ToolVersion;
    private Compatibility? SelectedArtifact => (ArtifactList.SelectedItem as ListItem)?.Value as Compatibility;
    private void BindList(ListView list, ListItem[] rows)
    {
        var id = (list.SelectedItem as ListItem)?.Id;
        list.ItemsSource = rows; list.SelectedItem = rows.FirstOrDefault(r => r.Id == id) ?? rows.FirstOrDefault();
    }
    private void Render()
    {
        if (Model.Closing) return;
        binding = true;
        try
        {
            ConnectionLabel.Text = Model.Synchronized ? "已连接 · 实时同步" : Model.Connection == null ? "未连接" : "连接恢复中";
            ToolTipService.SetToolTip(ConnectionLabel, Model.ConnectionStatus);
            ConnectionDot.Fill = (Brush)Application.Current.Resources[Model.Synchronized ? "SystemFillColorSuccessBrush" : "TextFillColorTertiaryBrush"];
            ConnectButton.Content = Model.Connection == null ? "连接服务器" : "断开连接"; ConnectButton.IsEnabled = !Model.Busy;
            ConnectionBar.IsOpen = Model.Connection != null && !Model.Synchronized;
            ConnectionBar.Message = Model.ConnectionStatus + (Model.Snapshot == null ? "" : " 当前信息可能过期，请等待快照同步。");
            if (renderedNotice != Model.Notice) { renderedNotice = Model.Notice; NoticeBar.IsOpen = Model.Notice.Length > 0; }
            NoticeBar.Message = Model.Notice;
            NoticeBar.Severity = Model.IsError ? InfoBarSeverity.Error : InfoBarSeverity.Informational;
            ErrorDetailsButton.Visibility = ToVisibility(Model.IsError && Model.ErrorDetails.Length > 0);
            PendingBar.IsOpen = Model.Connection?.Pending != null; BusyBar.Visibility = ToVisibility(Model.Busy);
            if (!ReferenceEquals(renderedSnapshot, Model.Snapshot) || renderedDevice != Model.DeviceId)
            {
                var changedDevice = renderedDevice != Model.DeviceId;
                renderedSnapshot = Model.Snapshot; renderedDevice = Model.DeviceId;
                RenderDevices();
                BindList(TaskList, (Model.Snapshot?.Tasks ?? []).Where(t => t.DeviceId == Model.DeviceId).OrderByDescending(t => t.CreatedAt)
                    .Select(t => new ListItem(t.TaskId, $"{Display.State(t.Type)} · {Display.State(t.State)}", t.CreatedAt.LocalDateTime.ToString("MM-dd HH:mm:ss"), t)).ToArray());
                BindList(AssetList, (Model.Snapshot?.Assets ?? []).Select(a => new ListItem(a.AssetId, a.Name, $"{Display.Size(a.Size)} · {(a.Archived ? "已归档" : "可用")}", a)).ToArray());
                BindList(ToolList, (Model.Snapshot?.Tools ?? []).Select(t => new ListItem(t.ToolId, t.Name, $"{(t.Archived ? "已归档 · " : "")}{t.Description}", t)).ToArray());
                if (changedDevice) { selectionRevision++; TaskOutput.Text = ""; taskDetail = null; VersionList.ItemsSource = null; ArtifactList.ItemsSource = null; }
                QueueRead("task", ReadTaskAsync); QueueRead("tools", ReadToolsAsync);
            }
            var device = Model.Device;
            DeviceTitle.Text = device == null ? "准备好连接设备" : string.IsNullOrWhiteSpace(device.Registration.Hostname) ? device.DeviceId : device.Registration.Hostname;
            DeviceSubtitle.Text = device == null ? "连接管理服务器，选择一台设备开始维护。" : Display.Join(Display.State(device.Status), device.Registration.Model, device.DeviceId);
            ModelLabel.Text = device == null ? "—" : Display.Join(device.Registration.Model, device.Registration.Hostname);
            FirmwareLabel.Text = device == null ? "—" : Display.Join(device.Registration.Firmware, device.Registration.ProbeVersion);
            ArchitectureLabel.Text = device == null ? "—" : Display.Join(device.Registration.Arch, device.Registration.Libc, device.Registration.Kernel);
            LastSeenLabel.Text = device?.LastSeenAt?.LocalDateTime.ToString("yyyy-MM-dd HH:mm:ss") ?? "—";
            var maintenance = Model.Maintenance;
            MaintenanceState.Text = Model.Snapshot?.MaintenanceError ?? (maintenance == null ? "尚未开启" : Display.State(maintenance.State));
            RemainingLabel.Text = Display.Remaining(maintenance);
            foreach (var (service, address, button) in new[] { ("web", WebAddress, WebButton), ("ssh", SshAddress, SshButton), ("telnet", TelnetAddress, TelnetButton) })
            {
                var entry = maintenance?.Endpoints.FirstOrDefault(e => e.Service == service);
                address.Text = maintenance is { State: not "closed" } && entry != null ? entry.Url ?? entry.Address : "开启后显示临时地址";
                ToolTipService.SetToolTip(address, address.Text); button.IsEnabled = Model.CanOpen && entry is { State: not "closed" };
            }
            StartMaintenance.IsEnabled = Model.CanCreate;
            StopMaintenance.IsEnabled = Model.CanWrite && maintenance is { State: not "closed", Released: false };
            MaintenanceDetailsButton.IsEnabled = maintenance != null;
            LeaseButton.IsEnabled = !Model.Busy;
            ExecButton.IsEnabled = DownloadButton.IsEnabled = Model.CanWrite && Model.Online;
            ImportButton.IsEnabled = CreateToolButton.IsEnabled = Model.CanWrite;
            ResendButton.IsEnabled = Model.CanWrite && TaskId != null;
            CompleteDownloadButton.Visibility = CleanupDownloadButton.Visibility = ToVisibility(taskDetail?.Type == "download");
            CompleteDownloadButton.IsEnabled = CleanupDownloadButton.IsEnabled = Model.CanWrite;
            RenderAsset(); RenderArtifact();
            PublishButton.IsEnabled = ArchiveToolButton.IsEnabled = Model.CanWrite && SelectedTool != null;
            ArchiveVersionButton.IsEnabled = Model.CanWrite && SelectedVersion != null;
            DeviceList.IsEnabled = !Model.Busy;
        }
        finally { binding = false; }
    }
    private void RenderDevices()
    {
        var filter = Search.Text.Trim();
        var rows = (Model.Snapshot?.Devices ?? []).Where(d => $"{d.DeviceId} {d.Registration.Hostname} {d.Registration.Model}".Contains(filter, StringComparison.OrdinalIgnoreCase))
            .Select(d => new DeviceItem(d, (Model.Snapshot!.Maintenance.Where(m => m.DeviceId == d.DeviceId).OrderByDescending(m => m.CreatedAt).FirstOrDefault()) is { State: not "closed" } m ? "远程维护 · " + Display.State(m.State) : "未开启维护")).ToArray();
        DeviceList.ItemsSource = rows; DeviceList.SelectedItem = rows.FirstOrDefault(d => d.Id == Model.DeviceId);
        DeviceCount.Text = $"{rows.Length} 台";
    }
    private void Search_Changed(AutoSuggestBox sender, AutoSuggestBoxTextChangedEventArgs e) { if (Model == null) return; binding = true; RenderDevices(); binding = false; }
    private void Device_Changed(object sender, SelectionChangedEventArgs e) { if (!binding) Model.SelectDevice((DeviceList.SelectedItem as DeviceItem)?.Id); }
    private void Section_Changed(NavigationView sender, NavigationViewSelectionChangedEventArgs args)
    {
        if (OverviewPage == null) return;
        var tag = (args.SelectedItem as NavigationViewItem)?.Tag as string;
        OverviewPage.Visibility = ToVisibility(tag == "overview"); TasksPage.Visibility = ToVisibility(tag == "tasks"); FilesPage.Visibility = ToVisibility(tag == "files"); ToolsPage.Visibility = ToVisibility(tag == "tools");
        PageScroll.ChangeView(null, 0, null);
    }
    private void QueueRead(string key, Func<Task> read)
    {
        if (Model.Closing) return;
        if (reads.ContainsKey(key)) { reads[key] = (true, read); return; }
        reads[key] = (false, read);
        var worker = ReadLoopAsync(key); readWorkers.Add(worker);
        _ = worker.ContinueWith(_ => DispatcherQueue.TryEnqueue(() => readWorkers.Remove(worker)), TaskScheduler.Default);
    }
    private async Task ReadLoopAsync(string key)
    {
        // Defer beyond the binding pass; otherwise synchronous no-selection reads
        // can reenter rendering while a list is being replaced.
        await Task.Yield();
        var owner = Model.Connection;
        try {
            do { var item = reads[key]; reads[key] = (false, item.Read); await item.Read(); }
            while (!Model.Closing && reads[key].Dirty);
        }
        catch (Exception e) { if (!Model.Closing && ReferenceEquals(owner, Model.Connection)) Model.SetError(e, "刷新详情"); }
        finally { reads.Remove(key); }
    }
    private async void Connect_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("连接服务器", async () => {
        if (Model.Connection == null) await Model.ConnectAsync(profile); else await Model.DisconnectAsync();
    });
    private async void StartMaintenance_Click(object sender, RoutedEventArgs e)
    {
        if (!Model.CanCreate) return;
        await Model.RunAsync("开启远程维护", async () => {
            object body = leaseMs is { } ms ? new { device_id = Model.RequireDevice(), lease_ms = ms } : new { device_id = Model.RequireDevice() };
            await Model.ExecuteAsync(Mutation.Json("开启远程维护", "maintenance", body));
        });
    }
    private async void StopMaintenance_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("关闭远程维护", async () => {
        var current = Model.Maintenance ?? throw new InvalidOperationException("请选择维护中的设备。");
        await Model.ExecuteAsync(Mutation.Json("关闭远程维护", $"maintenance/{Wire.Segment(current.MaintenanceId)}/close", new { }));
    });
    private async void Endpoint_Click(object sender, RoutedEventArgs e)
    {
        if (!Model.CanOpen) return;
        var service = (string)((Button)sender).Tag;
        await Model.RunAsync("打开 " + service, async () => {
            var owner = Model.RequireConnection(); var current = Model.Maintenance!;
            var fresh = await owner.ReadAsync((a, ct) => a.GetAsync<Maintenance>($"maintenance/{Wire.Segment(current.MaintenanceId)}", ct));
            if (!ReferenceEquals(owner, Model.Connection) || current.MaintenanceId != Model.Maintenance?.MaintenanceId || Model.Closing) return;
            if (fresh.State != "ready" || fresh.Released || fresh.SessionId != Model.Device?.CurrentSession?.SessionId) throw new InvalidOperationException("维护入口不可用，请等待设备状态刷新。");
            launch(fresh.Endpoints.Single(e => e.Service == service), profile);
            Model.SetNotice(service == "web" ? "网页已交给默认浏览器打开。" : $"已发起 {service.ToUpperInvariant()} 连接，请在外部客户端完成登录。");
            owner.Invalidate();
        });
    }
    private async void Exec_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("执行命令", async () => {
        var data = await Model.ExecuteAsync(Mutation.Json("执行命令", "tasks", new { device_id = Model.RequireDevice(), command = Required(CommandBox.Text, "命令"), timeout_seconds = PositiveTimeout(TimeoutBox.Value.ToString(System.Globalization.CultureInfo.InvariantCulture)) }));
        var id = data.GetProperty("task_id").GetString();
        QueueRead("task", async () => await ReadTaskAsync(id));
    });
    private void Task_Changed(object sender, SelectionChangedEventArgs e) { if (!binding) { selectionRevision++; TaskOutput.Text = ""; QueueRead("task", ReadTaskAsync); } }
    private async Task ReadTaskAsync() => await ReadTaskAsync(null);
    private async Task ReadTaskAsync(string? requestedId)
    {
        var id = requestedId ?? TaskId; var owner = Model.Connection; var revision = selectionRevision; var device = Model.DeviceId;
        if (id == null || owner == null) { TaskHeading.Text = "选择任务查看结果"; TaskOutput.Text = TaskDetails.Text = TaskSummaryLabel.Text = ""; taskDetail = null; return; }
        var detail = await owner.ReadAsync((a, ct) => a.GetAsync<TaskDetail>($"tasks/{Wire.Segment(id)}", ct));
        Transfer? transfer = null;
        if (detail.Type is "upload" or "download") transfer = await owner.ReadAsync((a, ct) => a.GetAsync<Transfer>($"tasks/{Wire.Segment(id)}/transfer", ct));
        if (Model.Closing || !ReferenceEquals(owner, Model.Connection) || revision != selectionRevision || device != Model.DeviceId) return;
        if (requestedId != null) { binding = true; TaskList.SelectedItem = (TaskList.ItemsSource as ListItem[])?.FirstOrDefault(t => t.Id == id); binding = false; }
        taskDetail = detail; TaskHeading.Text = $"{Display.State(detail.Type)} · {Display.State(detail.State)}";
        TaskSummaryLabel.Text = detail.Result is { } r ? $"{Display.State(r.Status)} · 退出码 {r.ExitCode} · {(r.Truncated ? "输出已截断" : "完整输出")}" : "等待服务器返回最终结果";
        TaskOutput.Text = detail.Result is { } result ? $"标准输出\n{result.Stdout}\n\n标准错误\n{result.Stderr}" : "尚无最终结果。";
        TaskDetails.Text = $"任务编号：{detail.TaskId}\n会话编号：{detail.LastSessionId}\n派发次数：{detail.DispatchCount}\n命令：{detail.Command}\n超时：{detail.TimeoutSeconds} 秒" +
            (transfer == null ? "" : $"\n文件已提交：{Display.Yes(transfer.Committed)}\n资源已释放：{Display.Yes(transfer.Released)}\n传输失败：{Display.Yes(transfer.Failed)}\n大小：{Display.Size(transfer.Size)}\nSHA-256：{transfer.Sha256}");
        Render();
    }
    private void RefreshTask_Click(object sender, RoutedEventArgs e) => QueueRead("task", ReadTaskAsync);
    private async void Resend_Click(object sender, RoutedEventArgs e) => await TaskMutationAsync("重发原任务", $"tasks/{Wire.Segment(TaskId ?? "")}/resend");
    private async void CompleteDownload_Click(object sender, RoutedEventArgs e) => await TaskMutationAsync("导入下载", $"downloads/{Wire.Segment(TaskId ?? "")}/complete");
    private async void CleanupDownload_Click(object sender, RoutedEventArgs e) => await TaskMutationAsync("清理下载暂存", $"downloads/{Wire.Segment(TaskId ?? "")}/cleanup");
    private Task TaskMutationAsync(string label, string path) => Model.RunAsync(label, async () => { if (TaskId == null) throw new InvalidOperationException("请选择任务。"); await Model.ExecuteAsync(Mutation.Json(label, path, new { })); QueueRead("task", ReadTaskAsync); });
    private static string Required(string text, string label) => string.IsNullOrWhiteSpace(text) ? throw new ArgumentException(label + "不能为空。") : text.Trim();
    private static uint PositiveTimeout(string text) => uint.TryParse(text, out var n) && n > 0 ? n : throw new ArgumentException("超时必须为正整数秒。");
}
