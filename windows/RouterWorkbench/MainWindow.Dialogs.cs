using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RouterWorkbench.Core;
using System.Globalization;

namespace RouterWorkbench;
public sealed partial class MainWindow
{
    private async void ErrorDetails_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("查看错误详情", async () => {
        await DialogAsync("错误详细信息", Text(Model.ErrorDetails), "", "关闭");
    });
    private async Task<ContentDialogResult> DialogAsync(string title, UIElement content, string primary = "确定", string close = "取消", string secondary = "")
    {
        if (Model.Closing) return ContentDialogResult.None;
        var dialog = new ContentDialog { Title = title, Content = content, PrimaryButtonText = primary,
            CloseButtonText = close, SecondaryButtonText = secondary, DefaultButton = ContentDialogButton.Primary,
            XamlRoot = Root.XamlRoot, RequestedTheme = Root.ActualTheme };
        activeDialog = dialog;
        try { return await dialog.ShowAsync(); }
        finally { activeDialog = null; }
    }
    private static TextBlock Text(string value) => new() { Text = value, TextWrapping = TextWrapping.Wrap, IsTextSelectionEnabled = true };
    private async Task<Dictionary<string, string>?> FieldsAsync(string title, params (string Key, string Label, string Value)[] fields)
    {
        var panel = new StackPanel { Spacing = 14, MinWidth = 360 };
        var boxes = new Dictionary<string, TextBox>();
        foreach (var (key, label, value) in fields) { var box = new TextBox { Header = label, Text = value }; boxes[key] = box; panel.Children.Add(box); }
        if (await DialogAsync(title, new ScrollViewer { Content = panel, MaxHeight = 480 }) != ContentDialogResult.Primary) return null;
        return boxes.ToDictionary(p => p.Key, p => p.Value.Text);
    }
    private async void Settings_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("保存连接设置", async () => {
        var panel = new StackPanel { Spacing = 16, MinWidth = 420 };
        var address = new TextBox { Header = "管理服务器地址", Text = profile.ServerUrl, PlaceholderText = "http://主机:端口" };
        var theme = new ComboBox { Header = "外观", ItemsSource = new[] { "跟随系统", "浅色", "深色" }, SelectedIndex = profile.Theme == "Light" ? 1 : profile.Theme == "Dark" ? 2 : 0, HorizontalAlignment = HorizontalAlignment.Stretch };
        var ssh = new TextBox { Header = "SSH 客户端路径（留空使用系统 OpenSSH）", Text = profile.SshExecutable };
        var sshPutty = new CheckBox { Content = "SSH 使用 PuTTY", IsChecked = profile.SshUsePutty };
        var user = new TextBox { Header = "SSH 用户名", Text = profile.SshUser };
        var telnet = new TextBox { Header = "Telnet 客户端路径（留空使用系统客户端）", Text = profile.TelnetExecutable };
        var telnetPutty = new CheckBox { Content = "Telnet 使用 PuTTY", IsChecked = profile.TelnetUsePutty };
        foreach (var item in new UIElement[] { address, theme, ssh, sshPutty, user, telnet, telnetPutty }) panel.Children.Add(item);
        panel.Children.Add(Text("连接配置仅保存地址、外观及外部客户端偏好。SSH 与 Telnet 登录在外部客户端完成。"));
        var choice = await DialogAsync("设置", new ScrollViewer { Content = panel, MaxHeight = 520 }, "保存并连接", "取消", "仅保存");
        if (choice == ContentDialogResult.None) return;
        var next = profile with { ServerUrl = address.Text, Theme = theme.SelectedIndex == 1 ? "Light" : theme.SelectedIndex == 2 ? "Dark" : "Default",
            SshExecutable = ssh.Text.Trim(), TelnetExecutable = telnet.Text.Trim(), SshUsePutty = sshPutty.IsChecked == true,
            TelnetUsePutty = telnetPutty.IsChecked == true, SshUser = user.Text.Trim() };
        next.BaseUri(); await next.SaveAsync(profilePath); profile = next; ApplyTheme(next.Theme);
        if (choice == ContentDialogResult.Primary) await Model.ConnectAsync(next);
        else Model.SetNotice("设置已保存。新的服务器地址将在下次连接时生效。");
    });
    private async void Lease_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("设置维护时长", async () => {
        var panel = new StackPanel { Spacing = 16, MinWidth = 360 };
        var useDefault = new CheckBox { Content = "使用默认时长（240 分钟）", IsChecked = leaseMs == null };
        var minutes = new TextBox { Header = "自定义时长（分钟，支持小数）", Text = (leaseMs is { } ms ? ms / 60000m : 240m).ToString(CultureInfo.InvariantCulture), IsEnabled = leaseMs != null };
        useDefault.Checked += (_, _) => minutes.IsEnabled = false; useDefault.Unchecked += (_, _) => minutes.IsEnabled = true;
        panel.Children.Add(useDefault); panel.Children.Add(minutes); panel.Children.Add(Text("时长必须为正数。到期后以服务器返回的关闭状态为准。"));
        if (await DialogAsync("维护时长", panel) != ContentDialogResult.Primary) return;
        if (useDefault.IsChecked == true) leaseMs = null;
        else {
            if (!decimal.TryParse(minutes.Text, NumberStyles.Number, CultureInfo.InvariantCulture, out var value)) throw new ArgumentException("请输入有效的维护时长。");
            var duration = value * 60000m;
            if (duration < 1 || duration > 9223372036854m || duration != decimal.Truncate(duration)) throw new ArgumentException("维护租期必须可表示为 1～9223372036854 的整数毫秒。");
            leaseMs = (long)duration;
        }
        LeaseButton.Content = $"{(leaseMs is { } custom ? custom / 60000m : 240m):0.########} 分钟 · 调整";
    });
    private async void MaintenanceDetails_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("维护详细信息", async () => {
        var panel = new StackPanel { Spacing = 16, MinWidth = 380 };
        foreach (var m in (Model.Snapshot?.Maintenance ?? []).Where(m => m.DeviceId == Model.DeviceId).OrderByDescending(m => m.CreatedAt))
            panel.Children.Add(Text($"{Display.State(m.State)} · {m.CreatedAt.LocalDateTime:MM-dd HH:mm}\n维护编号：{m.MaintenanceId}\n会话编号：{m.SessionId}\n到期时间：{m.ExpiresAt.LocalDateTime:yyyy-MM-dd HH:mm:ss}\n资源已释放：{Display.Yes(m.Released)}\n关闭原因：{Display.State(m.Reason)}\n当前连接数：{m.Connections}"));
        await DialogAsync("维护记录与详细信息", new ScrollViewer { Content = panel, MaxHeight = 480 }, "", "关闭");
    });
    private async void DeviceDetails_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("设备详细信息", async () => {
        var device = Model.Device ?? throw new InvalidOperationException("请选择设备。"); var owner = Model.RequireConnection();
        var sessions = await owner.ReadAsync((a, ct) => a.ListAsync<Session>($"devices/{Wire.Segment(device.DeviceId)}/sessions", ct));
        if (Model.Closing || !ReferenceEquals(owner, Model.Connection)) return;
        var panel = new StackPanel { Spacing = 16, MinWidth = 380 };
        panel.Children.Add(Text($"设备编号：{device.DeviceId}\n序列号：{device.Registration.Serial}\n启动编号：{device.Registration.BootId}\n能力：{string.Join(", ", device.Registration.Capabilities)}"));
        foreach (var s in sessions) panel.Children.Add(Text($"会话编号：{s.SessionId}\n开始：{s.StartedAt.LocalDateTime:yyyy-MM-dd HH:mm:ss}\n结束：{s.EndedAt?.LocalDateTime.ToString("yyyy-MM-dd HH:mm:ss") ?? "当前会话"}\n结束原因：{Display.State(s.EndReason)}"));
        await DialogAsync("设备与会话详情", new ScrollViewer { Content = panel, MaxHeight = 480 }, "", "关闭");
    });
    private async void Pending_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("处理未确认请求", async () => {
        var owner = Model.RequireConnection(); var pending = owner.Pending; if (pending == null) return;
        var result = await DialogAsync("核对未确认请求", Text($"操作：{pending.Label}\n请先查看服务器当前状态。重试仅适用于同一服务器进程，会复用原请求编号和内容。\n如果服务器已重启，请核对结果后解除请求保护。"), "重试原请求", "返回", "已核对，解除保护");
        if (result == ContentDialogResult.Primary) await Model.ExecuteAsync(pending, true);
        else if (result == ContentDialogResult.Secondary) { owner.AbandonPending(); Model.SetNotice("请求保护已解除，请根据当前服务器状态继续操作。"); }
    });
}
