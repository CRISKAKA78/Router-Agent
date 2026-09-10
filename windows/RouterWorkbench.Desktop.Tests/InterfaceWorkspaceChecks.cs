using System.IO;
using System.Net;
using System.Net.Http;
using System.Reflection;
using System.Text.Json;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;

internal static partial class Program
{
    private static async Task SourceSummaryChecks()
    {
        var late = new TaskCompletionSource<HttpResponseMessage>();
        var requests = new List<string>();
        using var lookup = new IpLocation(new LocationHandler(async (request, token) => {
            var ip = Uri.UnescapeDataString(request.RequestUri!.AbsolutePath.TrimStart('/'));
            requests.Add(ip);
            if (ip == "8.8.8.8") return await late.Task;
            return Json(JsonSerializer.Serialize(new { success = true, ip, country = "测试国家", region = "测试地区", connection = new { isp = "测试运营商" } }));
        }));
        var window = (MainWindow)Activator.CreateInstance(typeof(MainWindow), BindingFlags.Instance | BindingFlags.NonPublic,
            null, [Path.Combine(output, "source-summary-profile.json"), lookup], null)!;
        var device = new Device("source-fixture", new("source-fixture", "来源测试", "", "", "", "test", "x64", "", "", "", []),
            "online", null, null, null, null, null, null, 0, 0, SourceIp: "8.8.8.8",
            EffectiveMetrics: new() {
                ["egress_ipv4"] = new("IPv4", "192.0.2.10", "text", "ok", "", 600, "builtin", "egress", DateTimeOffset.UtcNow, false),
                ["egress_ipv6"] = new("IPv6", "2001:db8::10", "text", "ok", "", 600, "builtin", "egress", DateTimeOffset.UtcNow, false)
            });
        void Select(Device next)
        {
            window.GetType().GetField("snapshot", BindingFlags.Instance | BindingFlags.NonPublic)!.SetValue(window, Snapshot.Empty with { Devices = [next] });
            window.GetType().GetField("selectedDevice", BindingFlags.Instance | BindingFlags.NonPublic)!.SetValue(window, next.DeviceId);
            Invoke(window, "UpdateOverview");
        }
        string Value(string name, string property = "Value")
        {
            var value = SummaryValue(window, name == "出口IP" ? "ip" : name == "归属地" ? "place" : "isp");
            return property == "Value" ? value.Text : value.ToolTip?.ToString() ?? "";
        }
        try
        {
            Select(device);
            Check(Value("出口IP") == "8.8.8.8" && Value("运营商及归属地") == "—", "summary immediately shows only server source while lookup is pending despite two reported egress addresses");
            await Eventually(() => Task.FromResult(requests.SequenceEqual(new[] { "8.8.8.8" })), "summary looks up server source first");
            Select(device with { DeviceId = "new-source", SourceIp = "8.8.4.4" });
            late.SetResult(Json("{\"success\":true,\"ip\":\"8.8.8.8\",\"country\":\"过期归属地\",\"connection\":{\"isp\":\"旧运营商\"}}"));
            await Field<Task>(window, "locationWork");
            Check(Value("出口IP") == "8.8.4.4" && Value("运营商") == "测试运营商" && Value("归属地") == "测试国家·测试地区", "late lookup cannot replace current source and ISP/place display as independent items");
            Select(device with { SourceIp = "2001:4860:4860::8888" });
            await Field<Task>(window, "locationWork");
            Check(Value("出口IP") == "2001:4860:4860::8888" && Value("运营商及归属地").StartsWith("测试运营商"), "IPv6 server source also produces a single IP and operator/location row");
            var count = requests.Count;
            Select(device with { SourceIp = "192.168.1.5" });
            await Field<Task>(window, "locationWork");
            Check(Value("出口IP") == "192.168.1.5" && Value("运营商及归属地") == "—" && Value("运营商及归属地", "ValueTip").Contains("内网") && requests.Count == count, "private source is displayed without public lookup or invented location");
            Select(device with { SourceIp = null });
            await Field<Task>(window, "locationWork");
            Check(Value("出口IP") == "—" && Value("运营商及归属地") == "—" && requests.Count == count, "missing source does not fall back to either Probe egress address");
        }
        finally { late.TrySetResult(new(HttpStatusCode.OK)); await InvokeAsync(window, "Disconnect"); window.Close(); }
    }

    private static async Task SamplingDialogChecks(MainWindow window, ApiClient api, WorkspaceConnection connection)
    {
        var overview = Field<TabControl>(window, "overviewTabs");
        overview.SelectedItem = overview.Items.Cast<TabItem>().Single(t => t.Header?.ToString() == "接口状态");
        var ports = Field<TabControl>(window, "interfaceTabs");
        ports.SelectedIndex = 1; window.UpdateLayout();
        var button = Field<Button>(window, "samplingButton");
        Check(ports.Items.Count == 2 && ports.Items.Cast<TabItem>().Select(t => t.Header?.ToString()).SequenceEqual(new[] { "外壳端口", "系统端口" }) && button.IsVisible, "interface status exposes two selectable port groups and a visible sampling action");
        var header = ports.Items.Cast<TabItem>().Last();
        Check(button.TranslatePoint(new(), window).X >= header.TranslatePoint(new(header.ActualWidth, 0), window).X && Math.Abs(button.TranslatePoint(new(), window).Y - header.TranslatePoint(new(), window).Y) < 10, "sampling action sits after both port groups on the same header row");
        async Task<Window> Open()
        {
            _ = window.Dispatcher.BeginInvoke(new Action(() => button.RaiseEvent(new RoutedEventArgs(Button.ClickEvent))));
            await Eventually(() => Task.FromResult(Application.Current.Windows.Cast<Window>().Any(w => w.Owner == window && w.Title == "接口采样时间")), "sampling action opens modal dialog");
            var dialog = Application.Current.Windows.Cast<Window>().Single(w => w.Owner == window && w.Title == "接口采样时间");
            dialog.UpdateLayout(); return dialog;
        }
        TextBox Seconds(Window dialog) => Visuals<TextBox>(dialog).Single(t => AutomationProperties.GetName(t).StartsWith("采样周期"));
        void Save(Window dialog) => Visuals<Button>(dialog).Single(b => b.Content?.ToString() == "保存").RaiseEvent(new RoutedEventArgs(Button.ClickEvent));
        async Task Saved(Window dialog)
        {
            await Eventually(() => Task.FromResult(!dialog.IsVisible && !Field<bool>(window, "working")), "sampling dialog saves and closes");
            var expected = (await api.GetAsync<Device>("devices/managed-ui")).Profile!.Version;
            await Eventually(() => Task.FromResult(connection.Snapshot.Devices.Single(d => d.DeviceId == "managed-ui").Profile!.Version == expected), "saved interface profile reaches HTTP snapshot");
        }
        var before = (await api.GetAsync<Device>("devices/managed-ui")).Profile!;
        var dialog = await Open(); Seconds(dialog).Text = "15"; dialog.Close();
        await Eventually(() => Task.FromResult(!Field<bool>(window, "working")), "cancel releases sampling dialog");
        Check((await api.GetAsync<Device>("devices/managed-ui")).Profile!.Version == before.Version && ports.SelectedIndex == 1, "cancel changes no server configuration and retains selected port group");
        dialog = await Open(); Seconds(dialog).Text = "86401"; Save(dialog);
        await Eventually(() => Task.FromResult(Visuals<TextBlock>(dialog).Any(t => t.Text.Contains("须为0～86400"))), "invalid sampling interval stays in dialog with error");
        Check((await api.GetAsync<Device>("devices/managed-ui")).Profile!.Version == before.Version, "invalid interval sends no profile mutation");
        Seconds(dialog).Text = "4";
        var scope = Visuals<ComboBox>(dialog).Single(); scope.SelectedIndex = 2; Save(dialog);
        await Eventually(() => Task.FromResult(Visuals<TextBlock>(dialog).Any(t => t.Text.Contains("请填写需要采样"))), "empty selected-interface scope is rejected within dialog");
        Visuals<TextBox>(dialog).Single(t => AutomationProperties.GetName(t).StartsWith("接口名称")).Text = "eth0,br0";
        // Render writes Window.Content; include the Window backdrop in this otherwise transparent panel.
        ((Panel)dialog.Content).SetResourceReference(Panel.BackgroundProperty, "Surface");
        Render(dialog, "interface-sampling-light.png"); Theme.Apply("Dark"); Render(dialog, "interface-sampling-dark.png"); Theme.Apply("Light");
        Save(dialog); await Saved(dialog);
        var saved = (await api.GetAsync<Device>("devices/managed-ui")).Profile!;
        Check(saved.InterfaceSampling is { NetworkSeconds: 4, NetworkInterfaces: "eth0,br0" } && saved.TemplateGeneration == before.TemplateGeneration, "dialog saves interface-only offline override without reapplying template");
        dialog = await Open(); Visuals<CheckBox>(dialog).Single().IsChecked = true; Save(dialog); await Saved(dialog);
        Check((await api.GetAsync<Device>("devices/managed-ui")).Profile!.InterfaceSampling == null, "restore template defaults clears only interface override");
        dialog = await Open(); Seconds(dialog).Text = "4"; Save(dialog); await Saved(dialog);
        Check(ports.SelectedIndex == 1 && overview.SelectedItem is TabItem { Tag: "builtin_interfaces" }, "saving sampling leaves current interface group selected without a configuration page");
    }
}
