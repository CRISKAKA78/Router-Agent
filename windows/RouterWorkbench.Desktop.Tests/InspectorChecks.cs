using System.Reflection;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Input;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static async Task InspectorChecks(MainWindow window)
    {
        var arrange = typeof(MainWindow).Assembly.GetType("RouterWorkbench.Desktop.PropertySheet")!
            .GetMethod("Arrange", BindingFlags.Static | BindingFlags.NonPublic)!;
        var input = new[] { "source_ip", "kernel_arch", "hostname", "configuration_state", "cpu_model", "firmware" }
            .Select(key => new PropertyRow("系统信息", key, key) { Key = key, GroupId = "builtin_system" }).ToArray();
        (PropertyRow[] Rows, Dictionary<string, string> Sections) Layout(Presentation? presentation)
        {
            var result = arrange.Invoke(null, [input, presentation, "builtin_system", "系统信息"])!;
            return ((PropertyRow[])result.GetType().GetProperty("Rows")!.GetValue(result)!,
                (Dictionary<string, string>)result.GetType().GetProperty("Sections")!.GetValue(result)!);
        }
        var defaults = Layout(null);
        Check(defaults.Rows.Select(r => r.Key).SequenceEqual(new[] { "hostname", "cpu_model", "kernel_arch", "source_ip", "configuration_state", "firmware" }) && defaults.Sections.Values.Distinct().SequenceEqual(new[] { "基础身份", "系统与硬件", "运行与连接", "固件与版本" }),
            "default system properties form identity, hardware, runtime and firmware groups");
        Check(defaults.Rows.All(input.Contains) && defaults.Rows.All(r => r.GroupId == "builtin_system"),
            "visual sections retain original row objects and business group identity");
        var configured = Layout(new([], new() { ["source_ip"] = new("builtin_system", 0) }));
        Check(configured.Rows.SequenceEqual(input) && configured.Sections.Values.Distinct().Count() == input.Length && configured.Sections.Values.All(s => !((string)new SectionTitleConverter().Convert(s, typeof(string), null!, System.Globalization.CultureInfo.InvariantCulture)).Contains('\u001f')),
            "interleaved explicit field order retains contiguous semantic groups without exposing segment identifiers");

        Invoke(window, "Navigate", "overview");
        var tabs = Field<TabControl>(window, "overviewTabs"); tabs.SelectedIndex = 0;
        await Task.Delay(80); window.UpdateLayout();
        var grid = Field<DataGrid>(window, "properties");
        var source = grid.ItemsSource; var selected = grid.Items.Cast<PropertyRow>().First(); grid.SelectedItem = selected;
        Invoke(window, "UpdateOverview"); window.UpdateLayout();
        Check(ReferenceEquals(source, grid.ItemsSource) && ReferenceEquals(grid.SelectedItem, selected),
            "Inspector snapshot refresh retains grouped view and selected property object");
        Check(grid.Items.Groups == null && grid.Columns.Count == 2 && grid.HeadersVisibility == DataGridHeadersVisibility.Column,
            "actual default device uses the continuous inspector table and two property columns");
        var handle = Visuals<PropertyColumnThumb>(grid).First(t => TableBehavior.Ancestor<DataGridCell>(t)?.Column == grid.Columns[0]);
        var initialWidth = grid.Columns[0].ActualWidth;
        handle.RaiseEvent(new KeyEventArgs(Keyboard.PrimaryDevice, PresentationSource.FromVisual(window), 0, Key.Right) { RoutedEvent = Keyboard.KeyDownEvent });
        grid.UpdateLayout();
        Check(Math.Abs(grid.Columns[0].ActualWidth - initialWidth - 10) < 1,
            "headerless Inspector column resize is keyboard accessible");
        grid.SelectedItem = null;
        Visuals<ScrollViewer>(grid).First().ScrollToTop(); grid.UpdateLayout();
        foreach (var theme in new[] { "Light", "Dark" }) {
            Theme.Apply(theme); await Task.Delay(70); Render(window, $"inspector-{theme.ToLowerInvariant()}.png");
        }
        var width = window.Width; var height = window.Height;
        window.Width = 960; window.Height = 640; await Task.Delay(100); window.UpdateLayout();
        var workspace = (TabControl)window.FindName("WorkspaceTabs"); var client = (FrameworkElement)window.Content;
        Check(Visuals<DataGridRow>(grid).Any(r => r.IsVisible) && workspace.ActualWidth >= 640 && workspace.TranslatePoint(new(workspace.ActualWidth, 0), client).X <= client.ActualWidth + 1,
            "minimum window retains visible Inspector rows and a workspace that fits inside the client area");
        Render(window, "inspector-minimum.png");
        window.Width = width; window.Height = height;
    }
}
