using System.Globalization;
using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private TabControl overviewTabs = null!;
    private DataGrid properties = null!, sessions = null!;
    private PropertyRow[] propertyRows = [];
    private string propertyDeviceId = "";

    private ComboBox configBackend = null!, configOperation = null!;
    private TextBox configKey = null!, configValue = null!, configTimeout = null!;
    private TextBlock configHelp = null!, configSupport = null!, configKeyLabel = null!;
    private TextBox configOutput = null!;
    private string configTaskId = "";
    private Button configSubmit = null!;
    private sealed class PropertyPage(string id, DataGrid table, TabItem tab)
    {
        public string Id { get; } = id;
        public DataGrid Table { get; } = table;
        public TabItem Tab { get; } = tab;
        public PropertyRow[] Rows { get; set; } = [];
        public TextBlock? Empty { get; set; }
    }
    private readonly Dictionary<string, PropertyPage> propertyPages = [];
    private UIElement interfaceView = null!, discoveryDetails = null!;
    private DataGrid discoveryProperties = null!;
    private TabItem storageTab = null!, historyTab = null!;
    private TabControl interfaceTabs = null!;
    private Button samplingButton = null!;

    private UIElement BuildOverview()
    {
        sessions = Ui.Table("连接历史", ("状态", "StateText", 90), ("上线时间", "OnlineText", 160), ("下线时间", "OfflineText", 160), ("在线时长", "OnlineDuration", 180), ("离线时长", "OfflineDuration", 180), ("下线原因", "ReasonText", 140));
        sessions.CanUserSortColumns = false;
        storageMetrics = Ui.Table("存储空间", ("挂载点", "Entity", 160), ("使用率", "A", 110), ("已用 / 总量", "B", 180), ("可用", "C", 110), ("文件系统", "D", 155), ("更新时间", "E", -1));
        networkMetrics = Ui.Table("接口实时速率", ("接口", "Entity", 85), ("接收", "A", 100), ("发送", "B", 100), ("状态", "C", 65), ("接口类型", "D", 90), ("累计接收", "F", 100), ("累计发送", "G", 100), ("统计时长", "H", 145), ("更新时间", "E", 160));
        foreach (var grid in new[] { networkMetrics, storageMetrics })
            foreach (var column in grid.Columns.OfType<DataGridTextColumn>()) column.ElementStyle = Ui.CellTextStyle(column.Binding is System.Windows.Data.Binding binding ? binding.Path.Path + "Tip" : null);
        networkMetrics.MouseDoubleClick += (_, e) => { if (e.OriginalSource is DependencyObject source && ItemsControl.ContainerFromElement(networkMetrics, source) is DataGridRow row && row.Item is MonitorTableRow selected) OpenNetworkChart(selected); };
        samplingButton = Ui.Button("接口采样时间", () => _ = Run("接口采样时间", OpenSampling));
        samplingButton.Margin = new(10, 3, 0, 3);
        System.Windows.Automation.AutomationProperties.SetName(samplingButton, "接口采样时间");
        interfaceTabs = new TabControl { Style = (Style)FindResource("PropertyTabs"), Tag = samplingButton };
        interfaceTabs.Items.Add(new TabItem { Header="外壳端口", Tag="physical_ports", Content=BuildSwitchTable() });
        interfaceTabs.Items.Add(new TabItem { Header="系统端口", Tag="system_ports", Content=BuildSystemPorts() });
        interfaceTabs.SelectedIndex = 0;
        interfaceView = interfaceTabs;
        overviewTabs = new TabControl { Style = (Style)FindResource("PropertyTabs") };
        overviewTabs.SelectionChanged += (_, e) => {
            if (e.Source == overviewTabs && overviewTabs.SelectedItem is TabItem { Tag: string id } && propertyPages.TryGetValue(id, out var page)) properties = page.Table;
        };
        storageTab = new TabItem { Header = "存储空间", Tag = "storage", Content = storageMetrics };
        historyStatus = Ui.Text("暂无连接记录", true);
        historyTab = new TabItem { Header = "连接历史", Tag = "history", Content = Ui.Page(Ui.Bar(historyStatus), sessions) };
        UpdatePropertyPages(null);
        discoveryProperties = Ui.Table("发现资料", ("属性", "Name", 155), ("值", "Value", -1));
        discoveryDetails = Ui.Page(Ui.Bar(Ui.Button("纳管设备", () => _ = Run("纳管设备", async () => { if (Device is { } device) await EnrollDevice(device); }), true)), discoveryProperties);
        var root = new Grid(); root.Children.Add(overviewTabs); root.Children.Add(discoveryDetails);
        discoveryDetails.Visibility = Visibility.Collapsed;
        return root;
    }

    private void UpdatePropertyPages(Device? device)
    {
        var definitions = PropertyGroupDefinitions(device?.Presentation);
        var selected = (overviewTabs.SelectedItem as TabItem)?.Tag?.ToString();
        var current = overviewTabs.Items.Cast<TabItem>().Select(t => t.Tag?.ToString()).ToArray();
        var expected = definitions.Select(g => g.Id).Concat(device?.Presentation?.StorageVisible==false?new[]{"history"}:new[]{"storage","history"}).ToArray();
        foreach (var definition in definitions)
        {
            if (!propertyPages.TryGetValue(definition.Id, out var page))
            {
                var table = Ui.Table(definition.Name, ("属性", "Name", 155), ("值", "Value", 300));
                table.CanUserSortColumns = false;
                ((DataGridTextColumn)table.Columns[1]).ElementStyle = Ui.CellTextStyle("ValueTip");
                table.ContextMenu = PropertyMenu(table);
                var empty = Ui.Text("此分组暂无显示属性", true); empty.HorizontalAlignment = HorizontalAlignment.Center; empty.VerticalAlignment = VerticalAlignment.Top; empty.Margin = new(10, 55, 10, 10); empty.IsHitTestVisible = false;
                var contents = new Grid(); contents.Children.Add(table); contents.Children.Add(empty);
                var tab = new TabItem { Header = definition.Name, Tag = definition.Id, Content = contents };
                if (definition.Id == "builtin_interfaces")
                {
                    tab.Content = interfaceView;
                }
                page = new PropertyPage(definition.Id, table, tab) { Empty = definition.Id == "builtin_interfaces" ? null : empty }; propertyPages.Add(definition.Id, page);
            }
            page.Tab.Header = definition.Name;
        }
        if (!current.SequenceEqual(expected))
        {
            overviewTabs.Items.Clear();
            foreach (var definition in definitions) overviewTabs.Items.Add(propertyPages[definition.Id].Tab);
            if(device?.Presentation?.StorageVisible!=false) overviewTabs.Items.Add(storageTab);
            overviewTabs.Items.Add(historyTab);
            foreach (var id in propertyPages.Keys.Except(definitions.Select(g => g.Id)).ToArray()) propertyPages.Remove(id);
            overviewTabs.SelectedItem = overviewTabs.Items.Cast<TabItem>().FirstOrDefault(t => t.Tag?.ToString() == selected) ?? overviewTabs.Items[0];
        }
        properties = propertyPages.GetValueOrDefault(selected ?? "")?.Table ?? propertyPages["builtin_system"].Table;
    }

    private void UpdateOverview()
    {
        var device = Device; UpdateLocation(device); UpdateMonitorTables(device);
        UpdatePropertyPages(device);
        var pending = device is { Managed: false };
        discoveryDetails.Visibility = pending ? Visibility.Visible : Visibility.Collapsed;
        overviewTabs.Visibility = pending ? Visibility.Collapsed : Visibility.Visible;
        if (pending) { var rows=DeviceProperties.Rows(device!);discoveryProperties.ItemsSource=rows;TableBehavior.FitPropertyColumns(discoveryProperties,rows); }
        var incoming = device == null ? [] : PresentRows(device, DeviceProperties.Rows(device).Concat(LocationRows(device)).Concat(new[] {
            new PropertyRow("", "管理型号", device.Profile?.ModelName ?? "—") { Key = "managed_model" },
            new PropertyRow("", "配置状态", ConfigState(device.Profile?.ConfigurationState), device.Profile?.ConfigurationError ?? "") { Key = "configuration_state" }
        }));
        foreach (var page in propertyPages.Values)
        {
            var rows = incoming.Where(r => r.GroupId == page.Id).ToArray();
            if (page.Empty != null) page.Empty.Visibility = rows.Length == 0 ? Visibility.Visible : Visibility.Collapsed;
            if (propertyDeviceId == device?.DeviceId && page.Rows.Select(r => (r.Key, r.Name)).SequenceEqual(rows.Select(r => (r.Key, r.Name))))
            {
                for (var i = 0; i < rows.Length; i++) { page.Rows[i].Value = rows[i].Value; page.Rows[i].ValueTip = rows[i].ValueTip; }
            }
            else
            {
                var anchor = TableBehavior.Capture(page.Table);
                var selected = (page.Table.SelectedItem as PropertyRow)?.Key;
                page.Rows = rows; page.Table.ItemsSource = rows;
                page.Table.SelectedItem = rows.FirstOrDefault(r => r.Key == selected);
                if (propertyDeviceId == device?.DeviceId) TableBehavior.Restore(page.Table, anchor);
            }
            TableBehavior.FitPropertyColumns(page.Table, rows);
        }
        if (propertyDeviceId != device?.DeviceId) ClearConnectionHistory();
        propertyRows = incoming; propertyDeviceId = device?.DeviceId ?? "";
        UpdateQuickProperties();
    }

    private async Task DisconnectDevice()
    {
        var device = RequireDevice();
        if (!Confirm("断开设备", $"断开 {device.DisplayName} 的当前 Session？关联维护入口会被关闭，探针可能自动重连。")) return;
        await Write(new("断开设备", $"devices/{Id(device.DeviceId)}/disconnect"));
    }
    private UIElement BuildConfig()
    {
        configBackend = Ui.Combo(["nvram", "uci"]); configOperation = Ui.Combo(["读取", "写入", "删除", "提交到持久存储"], 0, 170);
        configKey = Ui.Input(); configValue = Ui.Code(); configValue.Height = 110; configValue.BorderThickness = new(1);
        configTimeout = Ui.Input("5", 85); configKeyLabel = Ui.Text("nvram 键名", true);
        configHelp = Ui.Text("", true); configHelp.TextWrapping = TextWrapping.Wrap;
        configSupport = Ui.Text("请选择设备。", true); configSupport.TextWrapping = TextWrapping.Wrap;
        configSubmit = Ui.Button("读取配置", () => _ = Run("配置读写", SubmitConfig), true);
        configBackend.SelectionChanged += (_, _) => { if (configKey == null) return; configKey.Clear(); configValue.Clear(); UpdateConfig(); };
        configOperation.SelectionChanged += (_, _) => { configKey.Clear(); configValue.Clear(); UpdateConfig(); };
        var form = new StackPanel { Margin = new(18), MaxWidth = 760, HorizontalAlignment = HorizontalAlignment.Left };
        form.Children.Add(configSupport); var line = new WrapPanel { Margin = new(0,16,0,6) };
        line.Children.Add(Ui.Labeled("配置系统", configBackend)); line.Children.Add(Ui.Labeled("操作", configOperation)); line.Children.Add(Ui.Labeled("超时（1～30 秒）", configTimeout)); form.Children.Add(line);
        form.Children.Add(configKeyLabel); configKey.Margin = new(0,5,0,14); form.Children.Add(configKey);
        form.Children.Add(Ui.Labeled("写入值（允许空字符串）", configValue));
        configHelp.Margin = new(0,5,0,18); form.Children.Add(configHelp); configSubmit.HorizontalAlignment = HorizontalAlignment.Left; form.Children.Add(configSubmit);
        configOutput = Ui.Code("操作结果将在此显示。", true);
        return Ui.Page(Ui.Heading("设备配置 · NVRAM / UCI"), Ui.Split(new ScrollViewer { Content = form, VerticalScrollBarVisibility = ScrollBarVisibility.Auto }, Ui.Page(Ui.Heading("配置操作结果"), configOutput), true, 2),
            Ui.Note("写入和删除不自动提交、不重启服务。"));
    }
    private void UpdateConfig()
    {
        var backend = configBackend.SelectedItem?.ToString() ?? "nvram"; var action = configOperation.SelectedIndex;
        configSupport.Text = Device == null ? "请选择设备。" : Device.Registration.Capabilities.Contains("router_config") ? "目标设备：" + Device.DisplayName : "当前探针未声明配置读写能力，请先更新探针。";
        configKey.IsEnabled = action != 3 || backend == "uci"; configValue.IsEnabled = action == 1;
        configKeyLabel.Text = action == 3 ? "UCI 配置包（nvram 提交整份配置，无需填写）" : backend == "uci" ? "UCI 路径，例如 system.@system[0].hostname" : "nvram 键名，例如 SN";
        configSubmit.Content = new[] { "读取配置", "写入配置", "删除配置", "提交配置" }[Math.Max(0, action)];
        configHelp.Text = action == 3 ? "提交可能同时持久化其他程序已暂存的修改。" : "读取和修改通过固件原生命令执行；修改后生效方式由固件和服务决定。";
    }
    private void ResetConfig() { configTaskId = ""; configOutput.Text = "操作结果将在此显示。"; configBackend.SelectedIndex = configOperation.SelectedIndex = 0; configKey.Clear(); configValue.Clear(); configTimeout.Text = "5"; }
    private async Task SubmitConfig()
    {
        var device = RequireDevice(); var current = Connected(); var backend = configBackend.SelectedItem.ToString()!; var action = new[] { "get", "set", "delete", "commit" }[configOperation.SelectedIndex];
        var timeout = Positive(configTimeout.Text); if (timeout > 30) throw new ArgumentException("配置超时须为 1～30 秒。");
        var body = new Dictionary<string,object> { ["backend"] = backend, ["operation"] = action, ["timeout_seconds"] = timeout };
        if (action != "commit") body["key"] = configKey.Text; else if (backend == "uci") body["package"] = configKey.Text;
        if (action == "set") body["value"] = configValue.Text;
        if (action is "delete" or "commit" && !Confirm("确认配置操作", $"在 {device.DisplayName} 上执行{configOperation.SelectedItem}？\n" + configHelp.Text)) return;
        var result = await Write(new("配置读写", $"devices/{Id(device.DeviceId)}/config-tasks", body)); configValue.Clear();
        if (connection == current && selectedDevice == device.DeviceId) {
            configTaskId = result.GetProperty("task_id").GetString()!; configOutput.Text = "正在等待配置操作结果…";
            _ = RefreshDetails();
        }
    }
    private static object Lease(string device, string minutes)
    {
        if (!decimal.TryParse(minutes, NumberStyles.AllowDecimalPoint, CultureInfo.InvariantCulture, out var n) || n <= 0 || n > 9223372036854m / 60000m) throw new ArgumentException("请输入有效的正租期分钟数。");
        var ms = n * 60000m; if (ms != decimal.Truncate(ms)) throw new ArgumentException("租期必须精确到整数毫秒。");
        return n == 240 ? new { device_id = device } : new { device_id = device, lease_ms = (long)ms };
    }
}
