using System.Net;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public sealed class DeviceSummary : Grid
{
    private readonly TextBlock title = Ui.Text("工作区"), identity = Ui.Text("选择设备以查看属性与操作", true);
    private readonly FlexibleSummaryPanel metadata = new();
    private readonly Dictionary<string, TextBlock> values = [];
    private readonly Dictionary<string, SummaryBlock> blocks = [];
    private readonly TextBlock status = Ui.Text("—");
    private readonly StackPanel who;
    public Button TemplateButton { get; } = new() { Content = new WorkbenchIcon { Kind = "refresh" }, ToolTip = "更新模板", MinHeight = 24, Padding = new(4), Margin = new(6, 0, 0, 0) };
    public DeviceSummary()
    {
        Margin = new(18, 16, 12, 16);
        ColumnDefinitions.Add(new() { Width = new(200) }); ColumnDefinitions.Add(new() { Width = new(1, GridUnitType.Star) });
        RowDefinitions.Add(new() { Height = GridLength.Auto }); RowDefinitions.Add(new() { Height = GridLength.Auto });
        title.SetResourceReference(TextBlock.FontSizeProperty, "UiInspectorTitleFontSize"); title.FontWeight = FontWeights.SemiBold;
        title.TextTrimming = identity.TextTrimming = TextTrimming.CharacterEllipsis;
        identity.SetResourceReference(TextBlock.FontSizeProperty, "UiSmallFontSize"); identity.Margin = new(24, 3, 0, 0);
        var name = new DockPanel(); var icon = new WorkbenchIcon { Kind = "device", Margin = new(0, 0, 8, 0) };
        DockPanel.SetDock(icon, Dock.Left); name.Children.Add(icon); DockPanel.SetDock(TemplateButton, Dock.Right); name.Children.Add(TemplateButton); name.Children.Add(title);
        status.Margin = new(14, 0, 0, 0); status.FontWeight = FontWeights.SemiBold; DockPanel.SetDock(status, Dock.Right); name.Children.Insert(2, status);
        values["status"] = status;
        System.Windows.Automation.AutomationProperties.SetName(status, "在线状态");
        who = new StackPanel { Margin = new(0, 3, 16, 3), VerticalAlignment = VerticalAlignment.Center, Children = { name, identity } }; Children.Add(who);
        System.Windows.Automation.AutomationProperties.SetName(TemplateButton, "更新模板");
        foreach (var (key, label, min, priority) in new[] { ("uptime", "开机时长", 106d, 1d), ("firmware", "固件版本", 120d, 3d), ("ip", "出口 IP（服务器观察）", 140d, 3d), ("isp", "运营商", 70d, 1d), ("place", "归属地", 70d, 1d) })
        {
            var block = new SummaryBlock(label, min, priority); blocks[key] = block; values[key] = block.Value; metadata.Children.Add(block);
        }
        SetColumn(metadata, 1); Children.Add(metadata);
        SizeChanged += (_, _) => Reflow();
    }
    private void Reflow()
    {
        if (ActualWidth <= 0) return;
        var scale = (double)FindResource("UiFontSize") / 13;
        var stacked = ActualWidth < 920 * scale;
        SetRow(metadata, stacked ? 1 : 0); SetColumn(metadata, stacked ? 0 : 1); SetColumnSpan(metadata, stacked ? 2 : 1);
        SetColumnSpan(who, stacked ? 2 : 1);
        if (!stacked)
        {
            who.Measure(new(double.PositiveInfinity, double.PositiveInfinity));
            ColumnDefinitions[0].Width = new(Math.Clamp(who.DesiredSize.Width, 180 * scale, Math.Max(180 * scale, ActualWidth * .26)));
        }
    }
    public void Update(Device? device, IpLocationResult location, bool hasUpdate, bool canUpdate)
    {
        title.Text = device?.DisplayName ?? "工作区"; title.ToolTip = title.Text;
        identity.Text = device?.DeviceId ?? "选择设备以查看属性与操作"; identity.ToolTip = identity.Text;
        metadata.Visibility = device == null ? Visibility.Collapsed : Visibility.Visible;
        status.Visibility = device == null ? Visibility.Collapsed : Visibility.Visible;
        TemplateButton.Visibility = hasUpdate ? Visibility.Visible : Visibility.Collapsed; TemplateButton.IsEnabled = canUpdate;
        if (device == null) return;
        TemplateButton.ToolTip = "更新模板 · 当前生效：" + (device.ActiveTemplate is { } active ? active.Name + " · v" + active.Version : "尚未确认");
        status.Text = device.StatusText; status.SetResourceReference(TextBlock.ForegroundProperty, device.Online ? "Online" : "Muted");
        var firmware = DeviceProperties.Field(device, "firmware", "固件版本", device.Registration.Firmware);
        blocks["firmware"].Update(firmware.Value, firmware.ValueTip);
        var uptime = DeviceProperties.Uptime(device); blocks["uptime"].Update(uptime.Value, uptime.ValueTip);
        blocks["ip"].Update(DisplayAddress(device.SourceIp), string.IsNullOrEmpty(device.SourceIp) ? "服务器未提供连接来源 IP" : device.SourceIp + "\n服务器观察到的探针连接来源 IP");
        blocks["isp"].Update(location.Isp, location.Reason); blocks["place"].Update(location.Place, location.Reason);
        Reflow();
    }
    public static string DisplayAddress(string? address)
    {
        if (string.IsNullOrEmpty(address)) return "—";
        if (IPAddress.TryParse(address, out var ip) && ip.IsIPv4MappedToIPv6) return ip.MapToIPv4().ToString();
        return address;
    }
}
