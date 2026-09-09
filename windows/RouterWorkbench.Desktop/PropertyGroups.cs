using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Display;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    internal static DisplayGroup[] PropertyGroupDefinitions(Presentation? presentation) => [
        new("builtin_system", "系统信息", 0),
        new("builtin_resources", "资源监控", 1),
        .. (presentation?.Groups ?? []).OrderBy(g => g.Order).ThenBy(g => g.Id, StringComparer.Ordinal),
        new("builtin_interfaces", "外壳端口", 1000001),
        new("other", "其他信息", 1000002)
    ];

    private static ContextMenu PropertyMenu(DataGrid grid)
    {
        var menu = new ContextMenu();
        var key = new MenuItem { Header = "复制属性标识" };
        key.Click += (_, _) => { if (grid.SelectedItem is PropertyRow row) Clipboard.SetText(row.Key); };
        var value = new MenuItem { Header = "复制完整值" };
        value.Click += (_, _) => { if (grid.SelectedItem is PropertyRow row) Clipboard.SetText(row.Value); };
        menu.Items.Add(key);
        menu.Items.Add(value);
        return menu;
    }

    internal static PropertyRow[] PresentRows(Device device, IEnumerable<PropertyRow> input)
    {
        var groups = PropertyGroupDefinitions(device.Presentation);
        var fields = device.Presentation?.Fields ?? [];
        bool Visible(PropertyRow row)
        {
            if (fields.TryGetValue(row.Key, out var field) && field.Visible is { } visible) return visible;
            if (row.Key == "session_id") return false;
            return device.Presentation?.BuiltinVisibility?.GetValueOrDefault(row.MetricGroup, row.MetricGroup is not ("network" or "switch")) ?? row.MetricGroup is not ("network" or "switch");
        }
        var rows = input.Where(Visible).ToArray();
        foreach (var row in rows)
        {
            var groupId = fields.GetValueOrDefault(row.Key)?.GroupId;
            if(groupId=="builtin_interfaces") groupId="other";
            var group = groups.FirstOrDefault(g => g.Id == groupId) ?? groups.First(g => g.Id == DevicePresentation.DefaultGroup(row.Key));
            row.GroupId = group.Id;
            row.Group = group.Name;
            row.ValueTip = string.Join("\n", new[] { row.ValueTip, "属性标识：" + row.Key }.Where(s => s.Length > 0));
        }
        return rows.OrderBy(r => Array.FindIndex(groups, g => g.Id == r.GroupId))
            .ThenBy(r => fields.GetValueOrDefault(r.Key)?.Order ?? int.MaxValue)
            .ThenBy(r => r.Key, StringComparer.Ordinal).ToArray();
    }
}
