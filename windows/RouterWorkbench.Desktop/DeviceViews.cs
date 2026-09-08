using System.Globalization;
using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private DataGrid properties = null!, sessions = null!;
    private TextBlock overviewHint = null!;
    private ComboBox configBackend = null!, configOperation = null!;
    private TextBox configKey = null!, configValue = null!, configTimeout = null!;
    private TextBlock configHelp = null!, configSupport = null!, configKeyLabel = null!;
    private TextBox configOutput = null!;
    private string configTaskId = "";
    private Button configSubmit = null!;
    private UIElement BuildOverview()
    {
        properties = Ui.Table("设备上报属性", ("分组", "Group", 90), ("属性", "Name", 155), ("值 / 采集状态", "Value", -1));
        var cellText = new Style(typeof(TextBlock));
        cellText.Setters.Add(new Setter(TextBlock.VerticalAlignmentProperty, VerticalAlignment.Top));
        cellText.Setters.Add(new Setter(TextBlock.TextWrappingProperty, TextWrapping.Wrap));
        cellText.Setters.Add(new Setter(TextBlock.MarginProperty, new Thickness(6,4,6,4)));
        ((DataGridTextColumn)properties.Columns[2]).ElementStyle = cellText;
        properties.RowHeight = double.NaN; properties.MinRowHeight = 28;
        sessions = Ui.Table("连接历史", ("Session", "SessionId", -1), ("开始时间", "StartedText", 157), ("结束原因", "EndReason", 100));
        overviewHint = Ui.Text("请选择设备查看属性。", true); overviewHint.Margin = new(10,8,10,8); overviewHint.TextWrapping = TextWrapping.Wrap;
        var detailTabs = new TabControl();
        detailTabs.Items.Add(new TabItem { Header = "全部属性", Content = properties });
        detailTabs.Items.Add(new TabItem { Header = "连接历史", Content = sessions });
        return Ui.Page(Ui.Bar(DeviceButton("远程维护", () => Navigate("maintenance")),
            DeviceButton("配置读写", () => Navigate("config")),
            Ui.Button("完整设备信息", () => { if (Device != null) Inspect("设备公开快照", Device); }),
            DeviceButton("断开设备…", () => _ = Run("断开设备", DisconnectDevice))), detailTabs, overviewHint);
    }
    private void UpdateOverview()
    {
        var d = Device;
        if (d == null) { properties.ItemsSource = sessions.ItemsSource = null; overviewHint.Text = "连接服务器并选择设备后查看属性。"; return; }
        Ui.SetRows(properties, DeviceProperties.Rows(d));
        overviewHint.Text = (d.Registration.Template is { } t ? $"{t.Name} · v{t.Version} · " : "未使用模板 · ") +
            $"展示该设备实际上报的属性与采集失败项。启动快照不会因模板编辑或配置写入自动更新。累计连接 {d.TotalSessions} 次。";
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
            Ui.Note("执行结果在本页显示。写入和删除不自动提交、不重启服务。"));
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
