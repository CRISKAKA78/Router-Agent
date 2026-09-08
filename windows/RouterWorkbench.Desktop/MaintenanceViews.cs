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
        maintenanceGrid.SelectionChanged += (_, _) => { if (refreshing) return; maintenanceId = (maintenanceGrid.SelectedItem as Maintenance)?.MaintenanceId ?? ""; UpdateEndpoints(); UpdateMaintenanceClock(); };
        endpointGrid = Ui.Table("维护通道链接", ("服务", "Service", 80), ("状态", "State", 85));
        var text = new FrameworkElementFactory(typeof(TextBlock)); text.SetValue(TextBlock.MarginProperty, new Thickness(7,4,7,4));
        var link = new FrameworkElementFactory(typeof(Hyperlink)); link.SetResourceReference(Hyperlink.ForegroundProperty, "Accent");
        link.AddHandler(Hyperlink.ClickEvent, new RoutedEventHandler((sender, args) => { if (((Hyperlink)sender).DataContext is ChannelRow row) _ = Run("打开维护通道", () => OpenEndpoint(row.Service)); }));
        var label = new FrameworkElementFactory(typeof(System.Windows.Documents.Run)); label.SetBinding(System.Windows.Documents.Run.TextProperty, new Binding("Link")); link.AppendChild(label); text.AppendChild(link);
        endpointGrid.Columns.Add(new DataGridTemplateColumn { Header = "连接链接（点击打开）", Width = new(1, DataGridLengthUnitType.Star), CellTemplate = new DataTemplate { VisualTree = text } });
        closeMaintenanceButton = Ui.Button("关闭维护…", () => _ = Run("关闭维护", CloseMaintenance));
        return Ui.Page(Ui.Bar(Ui.Text("租期（分钟）  ", true), leaseMinutes,
            DeviceButton("开启维护", () => _ = Run("开启维护", CreateMaintenance), true),
            closeMaintenanceButton, leaseStatus),
            Ui.Split(Ui.Page(Ui.Bar(Ui.Button("打开 Web", () => _ = Run("打开 Web", () => OpenEndpoint("web"))),
                Ui.Button("打开外部 SSH", () => _ = Run("打开 SSH", () => OpenEndpoint("ssh"))),
                Ui.Button("打开外部 Telnet", () => _ = Run("打开 Telnet", () => OpenEndpoint("telnet"))),
                Ui.Button("复制链接", () => _ = Run("复制链接", CopyEndpoint)),
                Ui.Button("复制 SSH 密码", () => _ = Run("复制密码", CopySshPassword)),
                Ui.Button("修改账号 / 客户端…", () => Navigate("settings"))), endpointGrid),
                Ui.Page(Ui.Heading("维护记录"), maintenanceGrid), true, 1.5),
            Ui.Note("开启维护后显示 Web / SSH / Telnet 通道链接。SSH 账号密码默认 admin，可在设置修改；外部客户端提示密码时可复制粘贴。"));
    }
    private sealed record ChannelRow(string Service, string State, string Link);
    private string EndpointLink(Endpoint endpoint) => endpoint.Service == "web" ? endpoint.Url ?? "" :
        new UriBuilder(endpoint.Service, endpoint.Host, endpoint.Port) { UserName = endpoint.Service == "ssh" ? profile.SshUser : "" }.Uri.AbsoluteUri;
    private void UpdateEndpoints() => endpointGrid.ItemsSource = ActiveMaintenance?.Endpoints.Select(e => new ChannelRow(e.Service, Labels.State(e.State), EndpointLink(e))).ToArray();
    private void UpdateMaintenance()
    {
        var rows = MaintenanceRows();
        if (!rows.Any(m => m.MaintenanceId == maintenanceId)) maintenanceId = (rows.FirstOrDefault(m => !m.Released) ?? rows.FirstOrDefault())?.MaintenanceId ?? "";
        Ui.SetRows(maintenanceGrid, rows); maintenanceGrid.SelectedItem = ActiveMaintenance; UpdateEndpoints(); UpdateMaintenanceClock();
    }
    private void UpdateMaintenanceClock()
    {
        if (leaseStatus == null) return;
        var current = ActiveMaintenance;
        closeMaintenanceButton.IsEnabled = Writable && current is { Released: false };
        if (current == null) { leaseStatus.Text = snapshot.MaintenanceError != "" ? snapshot.MaintenanceError : "尚未开启维护"; return; }
        var remaining = current.ExpiresAt - DateTimeOffset.UtcNow;
        leaseStatus.Text = current.Released ? "已关闭 · " + current.Reason : remaining <= TimeSpan.Zero ? "已到期" : $"{current.StateText} · 剩余 {Math.Max(0,(long)remaining.TotalMinutes)}分 {Math.Max(0,remaining.Seconds)}秒";
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
