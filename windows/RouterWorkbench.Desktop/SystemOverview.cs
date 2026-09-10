using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

internal sealed class SystemOverview : ScrollViewer
{
    private readonly Dictionary<string, SummaryBlock> blocks = [];
    internal SystemOverview()
    {
        VerticalScrollBarVisibility = ScrollBarVisibility.Auto; HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled;
        Padding = new(10, 10, 10, 4); MaxHeight = 190;
        var panel = new FlexibleSummaryPanel();
        foreach (var (key, label, minimum, priority) in new[] { ("status", "设备状态", 106d, 1d), ("uptime", "开机时长", 132d, 1d), ("firmware", "固件版本", 170d, 3d), ("ip", "出口 IP", 170d, 3d), ("isp", "运营商", 134d, 1d), ("config", "配置状态", 110d, 1d) })
        {
            var block = new SummaryBlock(label, minimum, priority, overview: true); blocks[key] = block; panel.Children.Add(block);
        }
        Content = panel;
        System.Windows.Automation.AutomationProperties.SetName(this, "设备状态概览");
    }
    internal void Update(Device? device, IpLocationResult location, string configuration)
    {
        Visibility = device == null ? Visibility.Collapsed : Visibility.Visible;
        if (device == null) return;
        blocks["status"].Update(device.StatusText, stateBrush: device.Online ? "Online" : "Muted");
        var uptime = DeviceProperties.Uptime(device); blocks["uptime"].Update(uptime.Value, uptime.ValueTip);
        var firmware = DeviceProperties.Field(device, "firmware", "固件版本", device.Registration.Firmware); blocks["firmware"].Update(firmware.Value, firmware.ValueTip);
        blocks["ip"].Update(DeviceSummary.DisplayAddress(device.SourceIp), "服务器观察到的探针连接来源 IP");
        blocks["isp"].Update(location.Isp, string.Join("\n", new[] { location.Place, location.Reason }.Where(s => s.Length > 0)));
        blocks["config"].Update(configuration, device.Profile?.ConfigurationError, configuration == "已生效" ? "Online" : "Muted");
    }
}
