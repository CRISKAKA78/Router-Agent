using System.ComponentModel;
using System.Reflection;
using System.Windows;
using System.Windows.Automation.Peers;
using System.Windows.Automation.Provider;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Media;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static TextBlock SummaryValue(MainWindow window, string key) => Field<Dictionary<string, TextBlock>>((DeviceSummary)window.FindName("Summary"), "values")[key];

    private static async Task DetailsLayoutChecks(MainWindow window)
    {
        Invoke(window, "Navigate", "overview");
        var tabs = Field<TabControl>(window, "overviewTabs"); tabs.SelectedIndex = 0;
        var devices = (DeviceList)window.FindName("DevicesGrid");
        var search = (TextBox)window.FindName("DeviceSearch"); var online = (CheckBox)window.FindName("OnlineOnly");
        var original = Field<Snapshot>(window, "snapshot"); var selected = Field<string>(window, "selectedDevice");
        var template = original.Devices.Single(d => d.DeviceId == selected);
        var width = window.Width; var height = window.Height;
        var outputHeight = Field<double>(window, "outputHeight");
        var outputRowHeight = ((RowDefinition)window.FindName("OutputRow")).Height;
        var outputSplitterHeight = ((RowDefinition)window.FindName("OutputSplitterRow")).Height;
        void Snapshot(params Device[] rows)
        {
            typeof(MainWindow).GetField("snapshot", BindingFlags.NonPublic | BindingFlags.Instance)!.SetValue(window, original with { Devices = rows });
            Invoke(window, "ApplySnapshot"); window.UpdateLayout();
        }
        void Sort(string path)
        {
            var column = devices.Columns.Single(c => c.SortMemberPath == path);
            var header = Visuals<DataGridColumnHeader>(devices).Single(h => h.Column == column);
            Check(column.CanUserSort && devices.CanUserSortColumns, "device column supports sorting: " + path);
            typeof(DataGridColumnHeader).GetMethod("OnClick", BindingFlags.Instance | BindingFlags.NonPublic)!.Invoke(header, null);
        }
        void Numbers() => Check(Visuals<DataGridRow>(devices).Where(r => r.IsVisible).All(r => DeviceList.GetRowNumber(r) == devices.Items.IndexOf(r.Item) + 1), "device row numbers track the current visible order");
        try
        {
            var fixtures = Enumerable.Range(1, 60).Select(i => template with { DeviceId = "sort-" + i.ToString("D2"), Status = i % 2 == 0 ? "online" : "offline", Profile = template.Profile! with { Name = "设备 " + (61-i).ToString("D2") } }).ToArray();
            Snapshot(fixtures);
            foreach (var path in new[] { "DeviceName", "DeviceId", "StatusText" })
            {
                for (var i = 0; i < 2; i++)
                {
                    Sort(path); await Task.Delay(60); window.UpdateLayout();
                    var direction = devices.Columns.Single(c => c.SortMemberPath == path).SortDirection;
                    var rows = devices.Items.Cast<Device>().ToArray();
                    string Value(Device d) => path == "DeviceName" ? d.DeviceName : path == "DeviceId" ? d.DeviceId : d.StatusText;
                    var expected = i == 0 ? rows.OrderBy(Value, StringComparer.Create(devices.Items.Culture ?? System.Globalization.CultureInfo.InvariantCulture, false)) : rows.OrderByDescending(Value, StringComparer.Create(devices.Items.Culture ?? System.Globalization.CultureInfo.InvariantCulture, false));
                    Check(direction == (i == 0 ? ListSortDirection.Ascending : ListSortDirection.Descending) && rows.SequenceEqual(expected), "header click toggles sort and its direction indicator: " + path + " / " + direction + " / " + (devices.Items.Culture?.Name ?? "invariant"));
                    Numbers();
                }
            }
            online.IsChecked = true; search.Text = "设备 0"; window.UpdateLayout();
            Check(devices.Items.Cast<Device>().All(d => d.Online && d.DeviceName.Contains("设备 0")) && devices.Items.Count > 0, "sorting composes with search and online filtering"); Numbers();
            search.Clear(); online.IsChecked = false; window.UpdateLayout();
            devices.ScrollIntoView(devices.Items[^1]); window.UpdateLayout(); Numbers();
            Check(Visuals<DataGridRow>(devices).Any(r => r.IsVisible && DeviceList.GetRowNumber(r) > 40), "recycled list rows retain absolute visual numbering after scrolling");
            var chosen = devices.Items[10]; devices.SelectedItem = chosen; var chosenId = ((Device)chosen).DeviceId;
            Sort("DeviceName"); await Task.Delay(60); window.UpdateLayout();
            Check(((Device)devices.SelectedItem).DeviceId == chosenId, "sorting retains the selected device identity");

            Snapshot(template); typeof(MainWindow).GetField("selectedDevice", BindingFlags.NonPublic | BindingFlags.Instance)!.SetValue(window, selected); Invoke(window, "ApplySnapshot");
            var longDevice = template with {
                DeviceId = "FE7140555489-very-long-device-identifier-for-layout-verification",
                SourceIp = "240e:3b6:d051:ffff:1234:5678:90ab:1af6",
                Registration = template.Registration with { Firmware = "v1.1.20260909-enterprise-router-firmware-long-release-description", BootId = "aabbccdd-1122-3344-5566-778899aabbcc", Capabilities = ["exec", "file", "tunnel", "router_config", "telemetry_v2", "managed_config_v1", "port_counters_v1", "future_long_capability_token_for_layout"] },
                EffectiveMetrics = template.EffectiveMetrics?.Where(p => p.Key != "firmware").ToDictionary(), Presentation = null
            };
            Snapshot(longDevice); tabs.SelectedIndex = 0;
            foreach (var size in new[] { (1480d, 920d), (1920d, 1080d), (960d, 640d) })
            {
                window.Width = size.Item1; window.Height = size.Item2; await Task.Delay(90);
                // A live HTTP refresh may replace the temporary layout fixture during the delay.
                Snapshot(longDevice); window.UpdateLayout();
                foreach (var theme in new[] { "Light", "Dark" })
                {
                    Theme.Apply(theme); window.UpdateLayout();
                    foreach (var sheet in Visuals<DataGrid>(tabs).Where(g => g.IsVisible)) { sheet.SelectedItem = null; Visuals<ScrollViewer>(sheet).First().ScrollToTop(); }
                    window.UpdateLayout();
                    var grids = Visuals<DataGrid>(tabs).Where(g => g.IsVisible && g.Items.OfType<PropertyRow>().Any()).ToArray();
                    Check(grids.Length == 1, "system sheet adapts to width: " + size.Item1 + " " + theme);
                    var shown = grids.SelectMany(g => g.Items.OfType<PropertyRow>()).ToArray();
                    Check(shown.Select(r => r.Key).Distinct().Count() == shown.Length && shown.Any(r => r.Key == "boot_id") && shown.Any(r => r.Key == "capabilities"), "responsive system sheet retains each property exactly once");
                    Check(grids.All(g => g.Items.Groups == null && g.HeadersVisibility == DataGridHeadersVisibility.Column), "inspector uses one continuous property table with explicit column headings");
                    foreach (var grid in grids)
                        Check(grid.Columns.Sum(c => c.ActualWidth) <= grid.ActualWidth + 1, "automatic property columns stay inside their sheet: " + grid.ActualWidth + " / " + string.Join(",", grid.Columns.Select(c => c.ActualWidth + ":" + c.Width)));
                    var valueCells = grids.SelectMany(Visuals<DataGridCell>).Where(c => c.Column.DisplayIndex == 1).ToArray();
                    Check(valueCells.Length > 0 && valueCells.SelectMany(Visuals<TextBlock>).All(t => t.TextWrapping == TextWrapping.Wrap), "system property values wrap fully by default at every supported width");
                    var critical = new[] { "status", "configuration_state", "capabilities" };
                    Check(valueCells.All(c => Visuals<TextBlock>(c).All(t => t.FontWeight == (c.DataContext is PropertyRow row && critical.Contains(row.Key) ? FontWeights.SemiBold : FontWeights.Normal))), "Inspector emphasizes status and available actions while firmware and ordinary values stay regular");
                    var ip = SummaryValue(window, "ip");
                    Check(ip.Text == longDevice.SourceIp && ip.TextTrimming == TextTrimming.CharacterEllipsis && ip.ToolTip.ToString()!.Contains(longDevice.SourceIp), "long IPv6 stays intact and is trimmed only by actual layout with full tooltip");
                    var summary = (DeviceSummary)window.FindName("Summary");
                    Check(summary.ActualHeight <= (size.Item1 == 960 ? 170 : 130), "device summary stays compact at supported widths");
                    var selector = InspectorControl<ComboBox>(window, "PageSelector");
                    Check(selector.Items.Count == tabs.Items.Count && selector.SelectedValue?.ToString() == (tabs.SelectedItem as TabItem)?.Tag?.ToString(), "page selector preserves every page and the current selection");
                    Check(!Visuals<SummaryBlock>(tabs).Any(), "system page does not repeat the persistent summary");
                    Check(!Visuals<TextBlock>(window).Any(t => t.Text == "所选设备"), "left explorer has no duplicate selected-device section");
                    Render(window, $"details-{size.Item1}-{theme.ToLowerInvariant()}.png");
                }
            }
            Check(DeviceSummary.DisplayAddress("::ffff:192.0.2.1") == "192.0.2.1" && DeviceSummary.DisplayAddress(null) == "—", "mapped IPv4 display and unknown source preserve address semantics");
            var workspace = (TabControl)window.FindName("WorkspaceTabs");
            var primary = (TabItem)workspace.Items[0];
            Check(primary.FontWeight == FontWeights.SemiBold && Visuals<WorkbenchIcon>(primary).Any() && InspectorControl<ComboBox>(window, "PageSelector").IsVisible, "primary modules and subordinate page selector are distinct and reachable");
            Check(Visuals<DataGridCell>(tabs).Where(c => c.Column.DisplayIndex == 0).SelectMany(Visuals<TextBlock>).All(t => t.FontWeight == FontWeights.Normal), "selected navigation weight does not leak into muted property labels");
            Invoke(window, "ToggleOutput"); window.UpdateLayout();
            var outputRow = (RowDefinition)window.FindName("OutputRow"); outputRow.Height = new(176); window.UpdateLayout();
            Invoke(window, "OutputResized", window, new DragCompletedEventArgs(0, 64, false));
            Invoke(window, "ToggleOutput"); Invoke(window, "ToggleOutput"); window.UpdateLayout();
            Check(Math.Abs(outputRow.ActualHeight - 176) < 1, "output hiding and showing preserves a user-resized height");
            Check(Visuals<DataGridCell>((DataGrid)window.FindName("ActivityGrid")).Where(c => c.Column.DisplayIndex == 2).SelectMany(Visuals<TextBlock>).All(t => t.TextAlignment == TextAlignment.Left), "output message text starts at the content edge");
        }
        finally
        {
            search.Clear(); online.IsChecked = false;
            devices.Items.SortDescriptions.Clear(); foreach (var column in devices.Columns) column.SortDirection = null;
            typeof(MainWindow).GetField("selectedDevice", BindingFlags.NonPublic | BindingFlags.Instance)!.SetValue(window, selected);
            typeof(MainWindow).GetField("snapshot", BindingFlags.NonPublic | BindingFlags.Instance)!.SetValue(window, original); Invoke(window, "ApplySnapshot");
            window.Width = width; window.Height = height; Theme.Apply("Light"); await Task.Delay(80);
            typeof(MainWindow).GetField("outputHeight", BindingFlags.NonPublic | BindingFlags.Instance)!.SetValue(window, outputHeight);
            ((RowDefinition)window.FindName("OutputRow")).Height = outputRowHeight;
            ((RowDefinition)window.FindName("OutputSplitterRow")).Height = outputSplitterHeight;
        }
    }
}
