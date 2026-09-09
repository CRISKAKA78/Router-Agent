using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private Task OpenSampling()
    {
        var device = RequireDevice(false);
        if (!device.Managed || device.Profile is not { } profile) throw new InvalidOperationException("请先纳管设备。");
        var owner = Connected();
        var defaults = profile.Monitoring ?? profile.BoundTemplate?.Monitoring ?? new();
        var sampling = profile.InterfaceSampling;
        var seconds = Ui.Input((sampling?.NetworkSeconds ?? defaults.NetworkSeconds).ToString());
        var names = Ui.Input(sampling?.NetworkInterfaces ?? "");
        var scope = Ui.Combo(["模板默认", "全部接口", "指定接口"], sampling?.NetworkInterfaces is null ? 0 : sampling.NetworkInterfaces.Length == 0 ? 1 : 2, double.NaN);
        var inherit = new CheckBox { Content = "恢复模板默认", Margin = new(0, 0, 0, 12) };
        var fields = new StackPanel();
        fields.Children.Add(Ui.Labeled("采样周期（秒，0关闭）", seconds));
        fields.Children.Add(Ui.Labeled("采样接口", scope));
        fields.Children.Add(Ui.Labeled("接口名称（逗号分隔）", names));
        names.IsEnabled = scope.SelectedIndex == 2;
        scope.SelectionChanged += (_, _) => names.IsEnabled = scope.SelectedIndex == 2;
        inherit.Checked += (_, _) => fields.IsEnabled = false;
        inherit.Unchecked += (_, _) => fields.IsEnabled = true;
        var note = Ui.Text($"设备：{device.DisplayName}\n" + ConfigState(profile.ConfigurationState) +
            (string.IsNullOrEmpty(profile.ConfigurationError) ? "" : " · " + profile.ConfigurationError) +
            (device.Online ? "" : " · 保存后等待设备上线应用"), true);
        note.TextWrapping = TextWrapping.Wrap; note.Margin = new(0, 0, 0, 16);
        var panel = new StackPanel { Children = { note, inherit, fields } };
        new ActionWindow(this, "接口采样时间", new ScrollViewer { Content = panel, VerticalScrollBarVisibility = ScrollBarVisibility.Auto }, "保存", async () => {
            if (owner != connection || closing || selectedDevice != device.DeviceId) throw new OperationCanceledException();
            InterfaceSampling? value = null;
            if (inherit.IsChecked != true)
            {
                if (!uint.TryParse(seconds.Text, out var interval) || interval > 86400) throw new InvalidOperationException("采样周期须为0～86400的整数。");
                var interfaces = scope.SelectedIndex == 0 ? null : scope.SelectedIndex == 1 ? "" : names.Text.Trim();
                if (scope.SelectedIndex == 2 && string.IsNullOrEmpty(interfaces)) throw new InvalidOperationException("请填写需要采样的接口名称。");
                value = new(interval, interfaces);
            }
            await SaveProfile(device, "managed", profile.Name, profile.ModelId, profile.BoundTemplate?.TemplateId ?? "", 0, false,
                profile.Monitoring, profile.PropertyIntervals, value, true);
        }) { Width = 570, Height = 450 }.ShowDialog();
        return Task.CompletedTask;
    }
}
