using System.Globalization;
using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private TabControl overviewTabs = null!;
    private PropertyInspectorWorkspace inspectorWorkspace = null!;
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
        public Dictionary<string, string> Sections { get; set; } = [];
        public TextBlock? Empty { get; set; }
    }
    private readonly Dictionary<string, PropertyPage> propertyPages = [];
    private UIElement interfaceView = null!, discoveryDetails = null!;
    private DataGrid discoveryProperties = null!;
    private TabItem storageTab = null!, historyTab = null!,lanNeighborsTab=null!,broadcastNeighborsTab=null!;
    private NeighborView lanNeighbors=null!,broadcastNeighbors=null!;
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
        samplingButton.Margin = new(10, 2, 4, 2); samplingButton.MinHeight = 24; samplingButton.SetResourceReference(Control.FontSizeProperty, "UiSmallFontSize");
        System.Windows.Automation.AutomationProperties.SetName(samplingButton, "接口采样时间");
        interfaceTabs = new TabControl { Style = (Style)FindResource("InterfaceTabs"), Tag = samplingButton };
        interfaceTabs.Items.Add(new TabItem { Header="外壳端口", Tag="physical_ports", Content=BuildSwitchTable() });
        interfaceTabs.Items.Add(new TabItem { Header="系统端口", Tag="system_ports", Content=BuildSystemPorts() });
        interfaceTabs.SelectedIndex = 0;
        interfaceView = interfaceTabs;
        overviewTabs = new TabControl { Style = (Style)FindResource("InspectorPages") };
        inspectorWorkspace = new PropertyInspectorWorkspace(overviewTabs);
        overviewTabs.SelectionChanged += (_, e) => {
            if (e.Source != overviewTabs) return;
            if (overviewTabs.SelectedItem is TabItem { Tag: string id } && propertyPages.TryGetValue(id, out var page) && id != "builtin_interfaces") {
                properties = page.Table; inspectorWorkspace.SetPage(page.Table, page.Sections);
            } else inspectorWorkspace.SetPage(null);
        };
        storageTab = new TabItem { Header = "存储空间", Tag = "storage", Content = Ui.Page(Ui.Heading("文件系统 · 使用情况"), storageMetrics) };
        historyStatus = Ui.Text("暂无连接记录", true);
        historyTab = new TabItem { Header = "连接历史", Tag = "history", Content = Ui.Page(Ui.Heading("连接历史"), Ui.Page(Ui.Bar(historyStatus), sessions)) };
        lanNeighbors=new NeighborView("lan",()=>connection,()=>Device);broadcastNeighbors=new NeighborView("broadcast",()=>connection,()=>Device);
        lanNeighborsTab=new TabItem{Header="LAN 下接设备",Tag="lan_neighbors",Content=lanNeighbors};broadcastNeighborsTab=new TabItem{Header="本机广播域设备",Tag="broadcast_neighbors",Content=broadcastNeighbors};
        UpdatePropertyPages(null);
        discoveryProperties = PropertySheet.Create("发现资料");
        discoveryProperties.ContextMenu = PropertyMenu(discoveryProperties);
        discoveryDetails = Ui.Page(Ui.Bar(Ui.Button("纳管设备", () => _ = Run("纳管设备", async () => { if (Device is { } device) await EnrollDevice(device); }), true)), discoveryProperties);
        var root = new Grid(); root.Children.Add(inspectorWorkspace); root.Children.Add(discoveryDetails);
        discoveryDetails.Visibility = Visibility.Collapsed;
        return root;
    }

    private void UpdatePropertyPages(Device? device)
    {
        var definitions = PropertyGroupDefinitions(device?.Presentation);
        var selected = (overviewTabs.SelectedItem as TabItem)?.Tag?.ToString();
        var current = overviewTabs.Items.Cast<TabItem>().Select(t => t.Tag?.ToString()).ToArray();
        var expected = definitions.Select(g => g.Id).Concat(new[]{"lan_neighbors","broadcast_neighbors"}).Concat(device?.Presentation?.StorageVisible==false?new[]{"history"}:new[]{"storage","history"}).ToArray();
        foreach (var definition in definitions)
        {
            if (!propertyPages.TryGetValue(definition.Id, out var page))
            {
                var table = PropertySheet.Create(definition.Name, system: definition.Id == "builtin_system", inspectable: true);
                table.ContextMenu = PropertyMenu(table);
                UIElement body = table;
                var tab = new TabItem { Header = definition.Name, Tag = definition.Id, Content = body };
                System.Windows.Documents.TextElement.SetFontWeight(body, FontWeights.Normal);
                if (definition.Id == "builtin_interfaces")
                {
                    tab.Content = interfaceView;
                }
                page = new PropertyPage(definition.Id, table, tab); propertyPages.Add(definition.Id, page);
            }
            page.Tab.Header = definition.Name;
        }
        if (!current.SequenceEqual(expected))
        {
            overviewTabs.Items.Clear();
            foreach (var definition in definitions) overviewTabs.Items.Add(propertyPages[definition.Id].Tab);
            overviewTabs.Items.Add(lanNeighborsTab);overviewTabs.Items.Add(broadcastNeighborsTab);
            if(device?.Presentation?.StorageVisible!=false) overviewTabs.Items.Add(storageTab);
            overviewTabs.Items.Add(historyTab);
            foreach (var id in propertyPages.Keys.Except(definitions.Select(g => g.Id)).ToArray()) propertyPages.Remove(id);
            overviewTabs.SelectedItem = overviewTabs.Items.Cast<TabItem>().FirstOrDefault(t => t.Tag?.ToString() == selected) ?? overviewTabs.Items[0];
        }
        properties = propertyPages.GetValueOrDefault(selected ?? "")?.Table ?? propertyPages["builtin_system"].Table;
        inspectorWorkspace.SyncPages();
    }

    private void UpdateOverview()
    {
        var device = Device; UpdateLocation(device); UpdateMonitorTables(device);
        UpdatePropertyPages(device);
        lanNeighbors.Update();broadcastNeighbors.Update();
        var pending = device is { Managed: false };
        discoveryDetails.Visibility = pending ? Visibility.Visible : Visibility.Collapsed;
        inspectorWorkspace.Visibility = pending ? Visibility.Collapsed : Visibility.Visible;
        if (pending) { var layout=PropertySheet.Arrange(DeviceProperties.Rows(device!),null,"builtin_system","发现资料");PropertySheet.Bind(discoveryProperties,layout.Rows,layout.Sections);TableBehavior.FitPropertyColumns(discoveryProperties,layout.Rows); }
        var incoming = device == null ? [] : PresentRows(device, DeviceProperties.Rows(device).Concat(LocationRows(device)).Concat(new[] {
            new PropertyRow("", "管理型号", DeviceProperties.Text(device.Profile?.ModelName)) { Key = "managed_model" },
            new PropertyRow("", "配置状态", ConfigState(device.Profile?.ConfigurationState), device.Profile?.ConfigurationError ?? "") { Key = "configuration_state" }
        }));
        foreach (var page in propertyPages.Values)
        {
            var layout = PropertySheet.Arrange(incoming.Where(r => r.GroupId == page.Id).ToArray(), device?.Presentation, page.Id, page.Tab.Header?.ToString() ?? "属性");
            var rows = layout.Rows;
            if (page.Empty != null) page.Empty.Visibility = rows.Length == 0 ? Visibility.Visible : Visibility.Collapsed;
            if (propertyDeviceId == device?.DeviceId && page.Rows.Select(r => (r.Key, r.Name)).SequenceEqual(rows.Select(r => (r.Key, r.Name))) && page.Sections.SequenceEqual(layout.Sections))
            {
                for (var i = 0; i < rows.Length; i++) { page.Rows[i].Value = rows[i].Value; page.Rows[i].ValueTip = rows[i].ValueTip; }
            }
            else
            {
                var anchor = TableBehavior.Capture(page.Table);
                var selected = (page.Table.SelectedItem as PropertyRow)?.Key;
                page.Rows = rows; page.Sections = layout.Sections;
                PropertySheet.Bind(page.Table, rows, layout.Sections);
                page.Table.SelectedItem = propertyDeviceId == device?.DeviceId ? rows.FirstOrDefault(r => r.Key == selected) : null;
                if (propertyDeviceId == device?.DeviceId) TableBehavior.Restore(page.Table, anchor);
            }
            TableBehavior.FitPropertyColumns(page.Table, rows);
        }
        if (overviewTabs.SelectedItem is TabItem { Tag: string active } && propertyPages.TryGetValue(active, out var activePage) && active != "builtin_interfaces")
            inspectorWorkspace.RowsUpdated(activePage.Sections, propertyDeviceId != device?.DeviceId);
        else if (propertyDeviceId != device?.DeviceId) inspectorWorkspace.Reset();
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
    private FrameworkElement configValueField=null!,configKeyField=null!;
    private TextBlock configContext=null!;
    private string configResultReported="";
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
        var form = new StackPanel { Margin = new(16,12,16,12) };
        form.Children.Add(configSupport); var line = new WrapPanel { Margin = new(0,12,0,0) };
        StackPanel Field(TextBlock label,FrameworkElement input,double width) {
            var field=new StackPanel {Width=width,Margin=new(0,0,12,8)};
            label.Margin=new(0,0,0,6);label.TextWrapping=TextWrapping.Wrap;input.Width=double.NaN;input.Margin=new(0);field.Children.Add(label);field.Children.Add(input);System.Windows.Automation.AutomationProperties.SetName(input,label.Text);return field;
        }
        line.Children.Add(Field(Ui.Text("配置系统",true),configBackend,120));line.Children.Add(Field(Ui.Text("操作",true),configOperation,180));
        configKeyField=Field(configKeyLabel,configKey,290);line.Children.Add(configKeyField);
        configTimeout.ToolTip="1～30 秒";line.Children.Add(Field(Ui.Text("超时（秒）",true),configTimeout,85));
        configSubmit.Margin=new(0,22,0,8);line.Children.Add(configSubmit);form.Children.Add(line);
        configValueField=Ui.Labeled("写入值（允许空字符串）",configValue);configValueField.MaxWidth=760;configValueField.HorizontalAlignment=HorizontalAlignment.Left;configValueField.Width=760;form.Children.Add(configValueField);
        form.SizeChanged+=(_,_)=>configValueField.Width=Math.Max(120,Math.Min(760,form.ActualWidth));
        configHelp.Margin = new(0,4,0,0); form.Children.Add(configHelp);
        configOutput = Ui.Code("操作结果将在此显示。", true);
        configContext=Ui.Text("读取或修改配置后，结果将在下方显示。",true);configContext.TextWrapping=TextWrapping.Wrap;configContext.Margin=new(16,10,16,10);
        System.Windows.Automation.AutomationProperties.SetLiveSetting(configContext,System.Windows.Automation.AutomationLiveSetting.Polite);
        var resultTop=new StackPanel {Children={Ui.Heading("配置操作结果"),configContext}};
        var formScroll=new ScrollViewer {Content=form,VerticalScrollBarVisibility=ScrollBarVisibility.Auto};
        var page=Ui.Page(formScroll,Ui.Page(resultTop,configOutput));
        page.SizeChanged+=(_,_)=>formScroll.MaxHeight=Math.Max(150,page.ActualHeight*.65);
        return page;
    }
    private void UpdateConfig()
    {
        var backend = configBackend.SelectedItem?.ToString() ?? "nvram"; var action = configOperation.SelectedIndex;
        configSupport.Text = Device == null ? "请选择设备。" : Device.Registration.Capabilities.Contains("router_config") ? "目标设备：" + Device.DisplayName : "当前探针未声明配置读写能力，请先更新探针。";
        configKey.IsEnabled = action != 3 || backend == "uci"; configValue.IsEnabled = action == 1;
        configValueField.Visibility=action==1?Visibility.Visible:Visibility.Collapsed;
        configKeyField.Visibility=action==3&&backend=="nvram"?Visibility.Collapsed:Visibility.Visible;
        configKeyLabel.Text = action == 3 ? "UCI 配置包" : backend == "uci" ? "UCI 路径" : "nvram 键名";
        configKey.ToolTip=backend=="uci"?"例如 system.@system[0].hostname；提交时填写配置包":"例如 SN";System.Windows.Automation.AutomationProperties.SetName(configKey,configKeyLabel.Text);
        configSubmit.Content = new[] { "读取配置", "写入配置", "删除配置", "提交配置" }[Math.Max(0, action)];
        configHelp.Text = action == 3 ? (backend=="nvram"?"提交整份 NVRAM。":"提交指定 UCI 配置包。")+"可能同时持久化其他程序已暂存的修改；不自动重启服务。" : action is 1 or 2 ? "写入和删除不自动提交、不重启服务；生效方式由固件和服务决定。" : "读取指定配置键；返回值为空时不推断键不存在。";
    }
    private void ResetConfig() { configTaskId = ""; configResultReported="";configContext.Text="读取或修改配置后，结果将在下方显示。";configOutput.Text = "操作结果将在此显示。"; configBackend.SelectedIndex = configOperation.SelectedIndex = 0; configKey.Clear(); configValue.Clear(); configTimeout.Text = "5"; }
    private async Task SubmitConfig()
    {
        var device = RequireDevice(); var current = Connected(); var backend = configBackend.SelectedItem.ToString()!; var action = new[] { "get", "set", "delete", "commit" }[configOperation.SelectedIndex];
        var timeout = Positive(configTimeout.Text); if (timeout > 30) throw new ArgumentException("配置超时须为 1～30 秒。");
        var body = new Dictionary<string,object> { ["backend"] = backend, ["operation"] = action, ["timeout_seconds"] = timeout };
        if (action != "commit") body["key"] = configKey.Text; else if (backend == "uci") body["package"] = configKey.Text;
        if (action == "set") body["value"] = configValue.Text;
        if (action is "delete" or "commit" && !Confirm("确认配置操作", $"在 {device.DisplayName} 上执行{configOperation.SelectedItem}？\n" + configHelp.Text)) return;
        var context=$"{device.DisplayName}（{device.DeviceId}） · {backend} · {configOperation.SelectedItem} {configKey.Text}";
        var result = await Write(new("配置读写", $"devices/{Id(device.DeviceId)}/config-tasks", body));
        if (connection == current && selectedDevice == device.DeviceId) {
            configValue.Clear();
            configTaskId = result.GetProperty("task_id").GetString()!; configOutput.Text = "正在等待配置操作结果…";
            configResultReported="";configContext.Text=context;
            _ = RefreshDetails();
        }
    }
    private void ShowConfigResult(TaskDetail detail)
    {
        var parameters=detail.Params;
        string Value(string key)=>parameters.ValueKind==JsonValueKind.Object&&parameters.TryGetProperty(key,out var value)?value.GetString()??"":"";
        var operation=Value("operation") switch {"get"=>"读取","set"=>"写入","delete"=>"删除","commit"=>"提交",_=>"配置操作"};
        var key=Value("key");if(key=="")key=Value("package");
        configContext.Text=$"{snapshot.Devices.FirstOrDefault(d=>d.DeviceId==detail.DeviceId)?.DisplayName??detail.DeviceId}（{detail.DeviceId}） · {Value("backend")} · {operation} {key} · {Labels.Time(detail.CreatedAt)}";
        configOutput.Text=detail.Result==null?$"{detail.StateText} · 等待最终结果 ({detail.TaskId})":$"{detail.StateText} · 退出码 {detail.Result.ExitCode}"+(detail.Result.Truncated?" · 输出已截断":"")+"\n"+detail.Result.Stdout+(detail.Result.Stderr==""?"":"\n"+detail.Result.Stderr);
        if(detail.Result!=null&&configResultReported!=detail.TaskId){configResultReported=detail.TaskId;Log("配置",$"{operation} {key}：{detail.StateText}（{detail.DeviceId}）");}
    }
    private static object Lease(string device, string minutes)
    {
        if (!decimal.TryParse(minutes, NumberStyles.AllowDecimalPoint, CultureInfo.InvariantCulture, out var n) || n <= 0 || n > 9223372036854m / 60000m) throw new ArgumentException("请输入有效的正租期分钟数。");
        var ms = n * 60000m; if (ms != decimal.Truncate(ms)) throw new ArgumentException("租期必须精确到整数毫秒。");
        return n == 240 ? new { device_id = device } : new { device_id = device, lease_ms = (long)ms };
    }
}
