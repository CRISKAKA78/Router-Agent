using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Text.Json;
using RouterWorkbench.Client;
using RouterWorkbench.Core;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private DataGrid maintenanceGrid = null!, endpointGrid = null!;
    private TextBox leaseMinutes = null!;
    private TextBlock leaseStatus = null!;
    private string maintenanceId = "";
    // A successful API response can arrive ahead of the next HTTP list snapshot.
    private Maintenance? maintenanceResult;
    private Button closeMaintenanceButton = null!;
    private Button createMaintenanceButton = null!;
    private Expander maintenanceHistory = null!;
    private TextBlock channelContext = null!;
    private Maintenance[] MaintenanceRows()
    {
        var rows = snapshot.Maintenance.Where(m => m.DeviceId == selectedDevice).ToList();
        if (maintenanceResult is { } result && result.DeviceId == selectedDevice && Device?.CurrentSession?.SessionId == result.SessionId) {
            var index = rows.FindIndex(m => m.MaintenanceId == result.MaintenanceId);
            if (index < 0) rows.Add(result);
            else if (result.Released && !rows[index].Released) rows[index] = result;
        }
        return rows.OrderByDescending(m => m.CreatedAt).ToArray();
    }
    private Maintenance? ActiveMaintenance => MaintenanceRows().FirstOrDefault(m => m.MaintenanceId == maintenanceId);
    private UIElement BuildMaintenance()
    {
        leaseMinutes = Ui.Input("240", 85); leaseMinutes.ToolTip = "维护租期，分钟。默认 240；支持自定义正租期。";
        leaseStatus = Ui.Text("请选择设备。", true); leaseStatus.Margin = new(10,0,0,0);
        maintenanceGrid = Ui.Table("维护历史", ("状态", "StateText", 75), ("创建时间", "CreatedText", 155), ("到期时间", "ExpiresText", 155), ("连接数", "Connections", 65), ("原因", "Reason", -1));
        ((DataGridTextColumn)maintenanceGrid.Columns[^1]).Binding = new Binding("Reason") {Converter=new MaintenanceReasonConverter()};
        maintenanceGrid.SelectionChanged += (_, _) => { if (refreshing) return; maintenanceId = (maintenanceGrid.SelectedItem as Maintenance)?.MaintenanceId ?? ""; UpdateEndpoints(); UpdateMaintenanceClock(); };
        endpointGrid = Ui.Table("维护通道链接", ("服务", "Service", 80), ("状态", "State", 85));
        var text = new FrameworkElementFactory(typeof(TextBlock)); text.SetValue(TextBlock.VerticalAlignmentProperty, VerticalAlignment.Center); text.SetValue(TextBlock.TextAlignmentProperty, TextAlignment.Center); text.SetValue(TextBlock.TextWrappingProperty,TextWrapping.Wrap); text.SetBinding(FrameworkElement.ToolTipProperty,new Binding("Link"));
        var link = new FrameworkElementFactory(typeof(Hyperlink)); link.SetBinding(ContentElement.IsEnabledProperty,new Binding("CanOpen"));
        var linkStyle=new Style(typeof(Hyperlink));linkStyle.Setters.Add(new Setter(Hyperlink.ForegroundProperty,new DynamicResourceExtension("Accent")));
        var unavailable=new DataTrigger {Binding=new Binding("CanOpen"),Value=false};unavailable.Setters.Add(new Setter(Hyperlink.ForegroundProperty,new DynamicResourceExtension("Muted")));unavailable.Setters.Add(new Setter(Inline.TextDecorationsProperty,null));linkStyle.Triggers.Add(unavailable);link.SetValue(FrameworkContentElement.StyleProperty,linkStyle);
        link.AddHandler(Hyperlink.ClickEvent, new RoutedEventHandler((sender, args) => { if (((Hyperlink)sender).DataContext is ChannelRow row) _ = Run("打开维护通道", () => OpenEndpoint(row.Service)); }));
        var label = new FrameworkElementFactory(typeof(System.Windows.Documents.Run)); label.SetBinding(System.Windows.Documents.Run.TextProperty, new Binding("Link") {Mode=BindingMode.OneWay}); link.AppendChild(label); text.AppendChild(link);
        endpointGrid.Columns.Add(new DataGridTemplateColumn { Header = "连接地址", Width = new(1, DataGridLengthUnitType.Star), CellTemplate = new DataTemplate { VisualTree = text }, ClipboardContentBinding=new Binding("Link"), MinWidth=200 });
        var actions=new FrameworkElementFactory(typeof(WrapPanel));actions.SetValue(FrameworkElement.HorizontalAlignmentProperty,HorizontalAlignment.Center);
        FrameworkElementFactory RowButton(string label, RoutedEventHandler handler, bool available=true) {
            var button=new FrameworkElementFactory(typeof(Button));button.SetValue(ContentControl.ContentProperty,label);button.SetResourceReference(FrameworkElement.StyleProperty,"ToolbarButton");
            if(available)button.SetBinding(UIElement.IsEnabledProperty,new Binding("CanOpen"));
            button.AddHandler(Button.ClickEvent,handler);return button;
        }
        var open=RowButton("打开",(sender,args)=>{if(((Button)sender).DataContext is ChannelRow row)_ = Run("打开维护通道",()=>OpenEndpoint(row.Service));});
        open.SetBinding(ContentControl.ContentProperty,new Binding("OpenLabel"));actions.AppendChild(open);
        actions.AppendChild(RowButton("复制链接",(sender,args)=>{endpointGrid.SelectedItem=((Button)sender).DataContext;_ = Run("复制链接",CopyEndpoint);}));
        var password=RowButton("复制密码",(_,_)=>_ = Run("复制密码",CopySshPassword),false);
        var passwordStyle=new Style(typeof(Button),(Style)FindResource("ToolbarButton"));passwordStyle.Setters.Add(new Setter(UIElement.VisibilityProperty,Visibility.Collapsed));
        var sshOnly=new DataTrigger {Binding=new Binding("Service"),Value="ssh"};sshOnly.Setters.Add(new Setter(UIElement.VisibilityProperty,Visibility.Visible));passwordStyle.Triggers.Add(sshOnly);password.SetValue(FrameworkElement.StyleProperty,passwordStyle);actions.AppendChild(password);
        endpointGrid.Columns.Add(new DataGridTemplateColumn {Header="操作",Width=new(300),MinWidth=170,CellTemplate=new DataTemplate {VisualTree=actions}});
        endpointGrid.SizeChanged+=(_,_)=>endpointGrid.Columns[^1].Width=new(endpointGrid.ActualWidth<850?190:300);
        endpointGrid.MinRowHeight=44;
        closeMaintenanceButton = Ui.Button("关闭维护…", () => _ = Run("关闭维护", CloseMaintenance));
        createMaintenanceButton=Ui.Button("开启维护", () => _ = Run("开启维护", CreateMaintenance), true);
        channelContext=Ui.Text("开启维护后显示连接入口。",true);channelContext.Margin=new(0,0,0,12);channelContext.TextWrapping=TextWrapping.Wrap;
        var channels=new Border {Padding=new(16),Child=Ui.Page(channelContext,endpointGrid)};
        maintenanceHistory=CompactWorkspace.History("维护记录（0）",maintenanceGrid);
        return Ui.Page(CompactWorkspace.Toolbar(Ui.Text("租期（分钟）  ", true),leaseMinutes,createMaintenanceButton,closeMaintenanceButton,leaseStatus,
            Ui.Button("账号与外部客户端…",OpenSettings)),CompactWorkspace.WithHistory(channels,maintenanceHistory));
    }
    private sealed class ChannelRow(string service,string state,string link) : System.ComponentModel.INotifyPropertyChanged
    {
        public string Service {get;}=service;
        public string State {get;}=state;
        public string Link {get;}=link;
        public string OpenLabel => Service=="web"?"打开 Web":"打开外部 "+(Service=="ssh"?"SSH":"Telnet");
        private bool canOpen;
        public bool CanOpen {get=>canOpen;set {if(canOpen==value)return;canOpen=value;PropertyChanged?.Invoke(this,new(nameof(CanOpen)));}}
        public event System.ComponentModel.PropertyChangedEventHandler? PropertyChanged;
    }
    private string EndpointLink(Endpoint endpoint) => endpoint.Service == "web" ? endpoint.Url ?? "" :
        new UriBuilder(endpoint.Service, endpoint.Host, endpoint.Port) { UserName = endpoint.Service == "ssh" ? profile.SshUser : "" }.Uri.AbsoluteUri;
    private void UpdateEndpoints() {
        var selected=(endpointGrid.SelectedItem as ChannelRow)?.Service;
        endpointGrid.ItemsSource=ActiveMaintenance?.Endpoints.Select(e=>new ChannelRow(e.Service,Labels.State(e.State),EndpointLink(e))).ToArray();
        endpointGrid.SelectedItem=endpointGrid.Items.OfType<ChannelRow>().FirstOrDefault(r=>r.Service==selected);
    }
    private void UpdateMaintenance()
    {
        var rows = MaintenanceRows();
        if (!rows.Any(m => m.MaintenanceId == maintenanceId)) maintenanceId = (rows.FirstOrDefault(m => !m.Released) ?? rows.FirstOrDefault())?.MaintenanceId ?? "";
        Ui.SetRows(maintenanceGrid, rows); maintenanceGrid.SelectedItem = ActiveMaintenance; UpdateEndpoints(); UpdateMaintenanceClock();
        maintenanceHistory.Header=$"维护记录（{rows.Length}）";
    }
    private void UpdateMaintenanceClock()
    {
        if (leaseStatus == null) return;
        var current = ActiveMaintenance;
        var live=MaintenanceRows().FirstOrDefault(m=>!m.Released);
        createMaintenanceButton.Content=live!=null&&live.MaintenanceId!=current?.MaintenanceId?"查看当前维护":"开启维护";
        createMaintenanceButton.IsEnabled=Writable&&Device is {Online:true,Managed:true}&&snapshot.MaintenanceError==""&&(live==null||live.MaintenanceId!=current?.MaintenanceId);
        closeMaintenanceButton.IsEnabled = Writable && current is { Released: false };
        var available=connection?.Synchronized==true&&!working&&Device is {Online:true}&&current is {Released:false,State:"ready"}&&current.ExpiresAt>DateTimeOffset.UtcNow&&current.SessionId==Device.CurrentSession?.SessionId;
        foreach(var row in endpointGrid.Items.OfType<ChannelRow>())row.CanOpen=available&&current!.Endpoints.Any(e=>e.Service==row.Service&&e.State=="ready");
        channelContext.Text=current==null?"开启维护后显示 Web、SSH 和 Telnet 入口。":current.Released?"以下为所选历史记录，入口已关闭。":"所选维护："+current.CreatedText+" · 到期 "+current.ExpiresText;
        if (current == null) { leaseStatus.Text = snapshot.MaintenanceError != "" ? snapshot.MaintenanceError : "尚未开启维护"; return; }
        var remaining = current.ExpiresAt - DateTimeOffset.UtcNow;
        leaseStatus.Text = current.Released ? "已关闭 · " + MaintenanceReasonConverter.Describe(current.Reason) : remaining <= TimeSpan.Zero ? "已到期 · 等待状态确认" : $"{current.StateText} · 剩余 {Math.Max(0,(long)remaining.TotalMinutes)}分 {Math.Max(0,remaining.Seconds)}秒";
    }
    private async Task CreateMaintenance()
    {
        var device = RequireDevice(); var c = Connected(); var lease = Lease(device.DeviceId, leaseMinutes.Text.Trim());
        var existing = await FindOpenMaintenance(c, device.DeviceId);
        if (connection != c || selectedDevice != device.DeviceId) return;
        if (existing != null) { UseExistingMaintenance(c, existing); return; }
        try {
            var data = await Write(new("开启维护", "maintenance", lease));
            SelectMaintenanceResult(c, data.Deserialize<Maintenance>(ApiJson.Options)!);
        } catch (ApiException e) when (e.Code == "conflict") {
            // Another client may have created a maintenance after the preflight read.
            existing = await FindOpenMaintenance(c, device.DeviceId);
            if (existing == null) throw;
            UseExistingMaintenance(c, existing);
        }
    }
    private Task<Maintenance?> FindOpenMaintenance(WorkspaceConnection c, string device) => c.TrackAsync(async () =>
        (await c.Api.ListAsync<Maintenance>($"maintenance?device_id={Id(device)}")).FirstOrDefault(m => !m.Released));
    private void UseExistingMaintenance(WorkspaceConnection c, Maintenance existing)
    {
        SelectMaintenanceResult(c, existing);
        if (connection != c || selectedDevice != existing.DeviceId) return;
        Log("维护", existing.State == "ready" && existing.ExpiresAt > DateTimeOffset.UtcNow
            ? "设备已有有效维护，已显示现有通道，原租期保持不变。"
            : "维护正在关闭或到期释放，请等待状态更新后再开启。");
        c.Invalidate();
    }
    private void SelectMaintenanceResult(WorkspaceConnection c, Maintenance result)
    {
        if (connection != c || closing) return;
        maintenanceResult = result;
        if (selectedDevice == result.DeviceId) maintenanceId = result.MaintenanceId;
        ApplySnapshot();
    }
    private async Task CloseMaintenance()
    {
        var current = ActiveMaintenance ?? throw new InvalidOperationException("请选择维护记录。");
        if (current.Released) { Log("维护", "所选维护已经关闭。"); return; }
        if (!Confirm("关闭维护", "关闭所选维护的 Web、SSH 和 Telnet 入口及活动连接？")) return;
        await CloseMaintenanceRecord(current);
    }
    private async Task CloseMaintenanceRecord(Maintenance current)
    {
        var c = Connected();
        var result = await Write(new("关闭维护", $"maintenance/{Id(current.MaintenanceId)}/close"));
        if (result.GetProperty("released").GetBoolean())
            SelectMaintenanceResult(c, current with { Released = true, State = "closed", Reason = "requested", Connections = 0,
                Endpoints = current.Endpoints.Select(e => e with { State = "closed" }).ToArray() });
    }
    private async Task<(Maintenance Maintenance, Endpoint Endpoint)> ResolveEndpoint(string service)
    {
        var c = Connected(); var selected = ActiveMaintenance ?? throw new InvalidOperationException("请先开启并选择维护。");
        if (!c.Synchronized) throw new InvalidOperationException("请等待快照同步。");
        var current = await c.TrackAsync(() => c.Api.GetAsync<Maintenance>($"maintenance/{Id(selected.MaintenanceId)}"));
        var device = await c.TrackAsync(() => c.Api.GetAsync<Device>($"devices/{Id(selected.DeviceId)}"));
        if (connection != c || selectedDevice != device.DeviceId || maintenanceId != current.MaintenanceId || current.Released || current.State != "ready" || current.ExpiresAt <= DateTimeOffset.UtcNow || current.SessionId != device.CurrentSession?.SessionId) throw new InvalidOperationException("维护已关闭或 Session 已变化，请刷新。");
        var endpoint = current.Endpoints.FirstOrDefault(e => e.Service == service) ?? throw new InvalidOperationException("维护入口未提供。"); ShellPolicy.ValidateEndpoint(endpoint);
        return (current, endpoint);
    }
    private async Task OpenEndpoint(string service)
    {
        var (_, endpoint) = await ResolveEndpoint(service);
        EndpointLauncher.Open(endpoint, profile); Log("维护", "已打开 " + service + " 外部入口。");
    }
    private async Task CopyEndpoint()
    {
        if (endpointGrid.SelectedItem is not ChannelRow row) throw new InvalidOperationException("请选择维护通道。");
        var (_, endpoint) = await ResolveEndpoint(row.Service); Clipboard.SetText(EndpointLink(endpoint)); Log("维护", "已复制通道链接。");
    }
}

internal sealed class MaintenanceReasonConverter : IValueConverter
{
    internal static string Describe(string reason) => reason switch {"requested"=>"手动关闭","expired"=>"租期到期","session_closed"=>"设备连接已关闭",""=>"—",_=>reason};
    public object Convert(object value,Type targetType,object parameter,System.Globalization.CultureInfo culture)=>Describe(value?.ToString()??"");
    public object ConvertBack(object value,Type targetType,object parameter,System.Globalization.CultureInfo culture)=>throw new NotSupportedException();
}
