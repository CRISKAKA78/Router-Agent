using System.Collections.ObjectModel;
using System.ComponentModel;
using System.IO;
using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Threading;
using Microsoft.Win32;
using RouterWorkbench.Client;
using RouterWorkbench.Core;

namespace RouterWorkbench.Desktop;

public partial class MainWindow : Window
{
    private ServerProfile profile = new() { SshUser = "admin" };
    private readonly string profilePath;
    private WorkspaceConnection? connection;
    private Snapshot snapshot = Snapshot.Empty;
    private string selectedDevice = "";
    private readonly ObservableCollection<ActivityRow> activity = [];
    private readonly ObservableCollection<QuickProperty> quickProperties = [];
    private WorkspaceConnection? reportedConnection;
    private string reportedConnectionStatus = "未连接";
    private readonly Dictionary<string, TabItem> tabs = [];
    private bool refreshing, working, closing, closed;
    private bool outputUserSet;
    private readonly DispatcherTimer clock = new() { Interval = TimeSpan.FromSeconds(1) };
    private Device? Device => snapshot.Devices.FirstOrDefault(d => d.DeviceId == selectedDevice);
    private bool Writable => connection is { Synchronized: true, Busy: false, Pending: null } && !working;
    public MainWindow() : this(ServerProfile.DefaultPath) { }
    public MainWindow(string settingsPath)
    {
        profilePath = settingsPath; SetResourceReference(StyleProperty, typeof(Window)); InitializeComponent();
        try { profile = File.Exists(profilePath) ? ServerProfile.Load(profilePath) : new() { SshUser = "admin" }; } catch (Exception e) { Log("错误", "读取配置失败：" + e.Message); }
        Theme.Apply(profile.Theme); ActivityGrid.ItemsSource = activity; QuickProperties.ItemsSource = quickProperties;
        SourceInitialized += (_, _) => Theme.ApplyCaption(this);
        BuildViews(); ApplySnapshot();
        Loaded += AutoConnect;
        PreviewKeyDown += HandleKeys; Closing += OnClosing;
        SizeChanged += (_, _) => { if (!outputUserSet) { var height = ActualHeight < 760 ? 0 : 170; OutputRow.Height = new(height); OutputSplitterRow.Height = new(height == 0 ? 0 : 4); } };
        clock.Tick += (_, _) => { UpdateMaintenanceClock(); }; clock.Start();
        SystemEvents.UserPreferenceChanged += SystemThemeChanged;
        Log("就绪", "连接管理服务器后，选择设备开始工作。F5 刷新 · Ctrl+F 搜索设备 · Ctrl+J 输出。");
    }
    private void BuildViews()
    {
        AddPage("overview", "设备", BuildOverview()); AddPage("maintenance", "维护", BuildMaintenance());
        AddPage("files", "文件", BuildFiles()); AddPage("config", "配置", BuildConfig()); AddPage("settings", "设置", BuildSettings());
    }
    private void AddPage(string key, string title, UIElement view) { var tab = new TabItem { Header = title, Tag = key, Content = view }; tabs[key] = tab; WorkspaceTabs.Items.Add(tab); }
    private string Page => (WorkspaceTabs.SelectedItem as TabItem)?.Tag?.ToString() ?? "overview";
    private void Navigate(string page) => WorkspaceTabs.SelectedItem = tabs[page];
    private void Log(string level, string message)
    {
        activity.Add(new(DateTime.Now.ToString("HH:mm:ss"), level, message));
        while (activity.Count > 300) activity.RemoveAt(0);
        StatusText.Text = message; OutputCaption.Text = $"  /  工作区活动 ({activity.Count})";
    }
    private void UpdateConnectionStatus()
    {
        var current = connection; var status = current?.Status ?? "未连接";
        ConnectionText.Text = status;
        if (reportedConnection == current && reportedConnectionStatus == status) return;
        reportedConnection = current; reportedConnectionStatus = status;
        if (!closing) Log("连接", current == null ? "已断开连接。" : status + " " + current.Api.Origin.GetLeftPart(UriPartial.Authority));
    }
    private void UpdateQuickProperties()
    {
        var device = Device;
        if (device == null) { if (quickProperties.Count != 0) quickProperties.Clear(); return; }
        if (quickProperties.Count == 0)
            foreach (var name in new[] { "设备 ID", "型号", "系统", "探针", "最近心跳" }) quickProperties.Add(new(name));
        var values = new[] { device.DeviceId, device.Registration.Model, device.Registration.Firmware, device.Registration.ProbeVersion, Labels.Time(device.LastSeenAt) };
        for (var i = 0; i < values.Length; i++) quickProperties[i].Value = string.IsNullOrEmpty(values[i]) ? "未提供" : values[i];
    }
    private async Task Run(string label, Func<Task> action)
    {
        if (working || closing) return; working = true; UpdateEnabled();
        try { await action(); }
        catch (OperationCanceledException) { if (!closing) Log("信息", label + "已取消。"); }
        catch (Exception e) { Log("错误", label + "：" + e.Message); if (OutputRow.Height.Value == 0) ToggleOutput(); }
        finally { working = false; UpdateEnabled(); }
    }
    private WorkspaceConnection Connected() => connection ?? throw new InvalidOperationException("请先连接服务器。");
    private Device RequireDevice(bool online = true)
    {
        var device = Device ?? throw new InvalidOperationException("请先选择设备。");
        if (online && (!device.Online || connection?.Synchronized != true)) throw new InvalidOperationException("设备未在线或快照尚未同步。");
        return device;
    }
    private async Task<JsonElement> Write(Mutation mutation)
    {
        var current = Connected();
        if (!current.Synchronized) { mutation.Dispose(); throw new InvalidOperationException("请等待连接同步后再操作。"); }
        var result = await current.ExecuteAsync(mutation);
        if (connection != current || closing) throw new OperationCanceledException();
        if (result.TryGetProperty("task_id", out var taskId)) {
            Log("任务", $"{mutation.Label}：已创建 {taskId.GetString()}，等待最终结果。");
            if (result.TryGetProperty("dispatch_uncertain", out var uncertain) && uncertain.ValueKind == JsonValueKind.True) Log("待核对", "派发不确定，已保留原任务；请查询原任务，不要创建替代任务。");
        } else Log("完成", mutation.Label + "完成。");
        return result;
    }
    private void Form(string title, string message, FormField[] fields, Func<Dictionary<string,string>,Task> accept) => new FormWindow(this, title, message, fields, accept).ShowDialog();
    private bool Confirm(string title, string message) => MessageBox.Show(this, message, title, MessageBoxButton.OKCancel, MessageBoxImage.Question, MessageBoxResult.Cancel) == MessageBoxResult.OK;
    private void Inspect(string title, object value) {
        var window = new Window { Owner = this, Title = title, Width = 760, Height = 600, WindowStartupLocation = WindowStartupLocation.CenterOwner, Content = Ui.Code(ApiJson.Pretty(value), true) }; window.ShowDialog();
    }
    private async void AutoConnect(object sender, RoutedEventArgs e)
    {
        Loaded -= AutoConnect;
        if (!closing && connection == null) await Run("自动连接", Connect);
    }
    private async Task Connect()
    {
        if (closing) return;
        if (connection?.Pending != null && !Confirm("切换连接", "存在响应不确定的请求。切换将放弃本地待定记录，但不会撤销服务器可能已完成的操作。继续？")) return;
        var next = profile; next.BaseUri();
        await Disconnect(); if (closing) return; await next.SaveAsync(profilePath); profile = next;
        if (closing) return;
        var current = new WorkspaceConnection(next.BaseUri()); connection = current;
        current.Changed += () => { if (!Dispatcher.HasShutdownStarted) Dispatcher.BeginInvoke(() => { if (connection == current && !closing) { snapshot = current.Snapshot; ApplySnapshot(); } }); };
        current.Start(); UpdateConnectionStatus();
    }
    private async Task Disconnect()
    {
        var old = connection; connection = null; CancelDetails();
        taskId = ""; task = null; transfer = null; operation = null; assetId = ""; maintenanceId = ""; maintenanceResult = null; ResetConfig();
        if (old != null) await old.DisposeAsync();
        snapshot = Snapshot.Empty; selectedDevice = ""; ApplySnapshot();
    }
    private void ApplySnapshot()
    {
        if (!IsInitialized || tabs.Count == 0) return;
        refreshing = true;
        try {
            if (!snapshot.Devices.Any(d => d.DeviceId == selectedDevice)) selectedDevice = snapshot.Devices.FirstOrDefault()?.DeviceId ?? "";
            ApplyDeviceFilter();
            DeviceTitle.Text = Device?.DisplayName ?? "工作区";
            DeviceSubtitle.Text = Device == null ? "选择设备以查看属性与操作" : $"{Device.StatusText}   ·   {Device.Registration.Arch} / {Device.Registration.Libc}";
            UpdateQuickProperties(); UpdateConnectionStatus();
            CountText.Text = $"设备 {snapshot.Devices.Length}   在线 {snapshot.Devices.Count(d => d.Online)}";
            SyncText.Text = snapshot.FetchedAt is { } fetched ? "同步 " + fetched.ToLocalTime().ToString("HH:mm:ss") : "";
            PendingBanner.Visibility = connection?.Pending != null && !connection.Busy ? Visibility.Visible : Visibility.Collapsed;
            PendingText.Text = $"{connection?.Pending?.Label} 响应不确定。原请求已保留，新写入已暂停。";
            UpdateOverview(); UpdateMaintenance(); UpdateTasks(); UpdateFiles(); UpdateConfig(); UpdateEnabled();
        } finally { refreshing = false; }
        _ = RefreshDetails();
    }
    private void ApplyDeviceFilter()
    {
        var query = DeviceSearch.Text.Trim();
        var devices = snapshot.Devices.Where(d => (OnlineOnly.IsChecked != true || d.Online) && $"{d.DeviceId} {d.DisplayName} {d.Registration.Model}".Contains(query, StringComparison.OrdinalIgnoreCase)).ToArray();
        Ui.SetRows(DevicesGrid, devices); DevicesGrid.SelectedItem = devices.FirstOrDefault(d => d.DeviceId == selectedDevice);
        DevicesEmpty.Text = connection == null ? "连接服务器后显示设备" : connection.Synchronized ? "没有匹配的设备" : connection.Status;
        DevicesEmpty.Visibility = devices.Length == 0 ? Visibility.Visible : Visibility.Collapsed; DeviceCountText.Text = devices.Length.ToString();
    }
    private void SearchChanged(object sender, RoutedEventArgs e) { if (tabs.Count == 0) return; refreshing = true; try { ApplyDeviceFilter(); } finally { refreshing = false; } }
    private void DeviceSelectionChanged(object sender, SelectionChangedEventArgs e)
    {
        if (refreshing || DevicesGrid.SelectedItem is not Device device || selectedDevice == device.DeviceId) return;
        selectedDevice = device.DeviceId; CancelDetails(); taskId = ""; task = null; transfer = null; operation = null;
        ResetConfig(); ApplySnapshot();
    }
    private void PageChanged(object sender, SelectionChangedEventArgs e)
    {
        if (e.Source != WorkspaceTabs || tabs.Count < 5) return;
        CancelDetails();
        _ = RefreshDetails();
    }
    private void UpdateEnabled()
    {
        ConnectButton.IsEnabled = !working; var online = Device?.Online == true && Writable;
        foreach (var button in deviceActions) button.IsEnabled = online;
        foreach (var button in writes) button.IsEnabled = Writable;
        configSubmit.IsEnabled = online && Device?.Registration.Capabilities.Contains("router_config") == true;
        UpdateMaintenanceClock();
    }
    private void SystemThemeChanged(object sender, UserPreferenceChangedEventArgs e) { if (profile.Theme == "Default") Dispatcher.BeginInvoke(() => { Theme.Apply("Default"); }); }
    private async Task SetTheme(string theme) { profile = profile with { Theme = theme }; Theme.Apply(theme); await profile.SaveAsync(profilePath); }
    private void HandleKeys(object sender, KeyEventArgs e)
    {
        if (Keyboard.Modifiers == ModifierKeys.Control) {
            switch (e.Key) {
                case Key.F: DeviceSearch.Focus(); DeviceSearch.SelectAll(); break;
                case Key.J: ToggleOutput(); break;
                case Key.O: Navigate("settings"); break;
                default: return;
            } e.Handled = true;
        } else if (e.Key == Key.F5) { connection?.Invalidate(); e.Handled = true; }
    }
    private void ToggleOutput() { outputUserSet = true; var show = OutputRow.Height.Value == 0; OutputRow.Height = new(show ? 170 : 0); OutputSplitterRow.Height = new(show ? 4 : 0); }
    private async void OnClosing(object? sender, CancelEventArgs e)
    {
        if (closed) return; e.Cancel = true; if (closing) return;
        if (connection?.Pending != null && !connection.Busy && !Confirm("退出", "存在响应不确定的请求。退出将丢弃本地待定记录，服务器可能已经执行。仍要退出？")) return;
        closing = true; clock.Stop(); SystemEvents.UserPreferenceChanged -= SystemThemeChanged;
        try { await Disconnect(); } finally { closed = true; _ = Dispatcher.BeginInvoke(Close); }
    }
    private void ConnectClick(object sender, RoutedEventArgs e) => _ = Run("连接", Connect);
    private void DisconnectClick(object sender, RoutedEventArgs e) => _ = Run("断开连接", async () => {
        if (connection?.Pending != null && !Confirm("断开连接", "存在响应不确定的请求。断开将丢弃本地待定记录，且不会撤销服务器操作。继续？")) return;
        await Disconnect();
    });
    private void RefreshClick(object sender, RoutedEventArgs e) { connection?.Invalidate(); _ = RefreshDetails(); }
    private void ServerKeyDown(object sender, KeyEventArgs e) { if (e.Key == Key.Enter) _ = Run("保存并连接", SaveAndConnect); }
    private void ThemeClick(object sender, RoutedEventArgs e) => _ = Run("切换主题", () => SetTheme((string)((MenuItem)sender).Tag));
    private void OverviewClick(object sender, RoutedEventArgs e) => Navigate("overview");
    private void FilesClick(object sender, RoutedEventArgs e) => Navigate("files");
    private void MaintenanceClick(object sender, RoutedEventArgs e) => Navigate("maintenance");
    private void ConfigClick(object sender, RoutedEventArgs e) => Navigate("config");
    private void SettingsClick(object sender, RoutedEventArgs e) => Navigate("settings");
    private void ImportClick(object sender, RoutedEventArgs e) => _ = Run("导入文件", ImportAsset);
    private void DeviceDisconnectClick(object sender, RoutedEventArgs e) => _ = Run("断开设备", DisconnectDevice);
    private void ToggleOutputClick(object sender, RoutedEventArgs e) => ToggleOutput();
    private void ClearActivityClick(object sender, RoutedEventArgs e) { activity.Clear(); OutputCaption.Text = "  /  工作区活动"; }
    private void ExitClick(object sender, RoutedEventArgs e) => Close();
    private void AboutClick(object sender, RoutedEventArgs e) => MessageBox.Show(this, "Router Workbench\n原生 C# / WPF 设备工程工作区\n\nCtrl+O 连接与设置\nCtrl+F 搜索设备\nCtrl+J 显示 / 隐藏输出\nF5 刷新快照\nCtrl+C 复制表格选中行\n拖动分隔条调整工作区；单击列标题排序。", "快捷键与关于");
    private void RetryClick(object sender, RoutedEventArgs e) => _ = Run("重试原请求", async () => {
        var c = Connected(); var pending = c.Pending; if (pending == null) return;
        if (!Confirm("重试原请求", "请先确认服务器进程没有重启，且已核对原任务/资产状态。重试将使用完全相同的请求键和字节。确认继续？")) return;
        var result = await c.ExecuteAsync(pending, true); Log("完成", "原请求已返回。" + (result.TryGetProperty("task_id", out var id) ? "任务 " + id.GetString() : ""));
    });
    private void AbandonClick(object sender, RoutedEventArgs e) { if (Confirm("放弃待定记录", "此操作只清除客户端记录，不会撤销服务器可能已完成的操作。确认已核对服务器状态后继续？")) connection?.Abandon(); }
    private static uint Positive(string value) => uint.TryParse(value, out var n) && n > 0 ? n : throw new ArgumentException("超时须为正整数秒。");
    private static string Id(string value) => ApiClient.Segment(value);
    private readonly List<Button> deviceActions = [], writes = [];
    private Button DeviceButton(string title, Action action, bool primary = false) { var button = Ui.Button(title, action, primary); deviceActions.Add(button); return button; }
    private Button WriteButton(string title, Action action) { var button = Ui.Button(title, action); writes.Add(button); return button; }
}
