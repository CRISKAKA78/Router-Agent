using System.Reflection;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static T InspectorControl<T>(MainWindow window, string name) => (T)Field<object>(window, "inspectorWorkspace").GetType()
        .GetProperty(name, BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic)!.GetValue(Field<object>(window, "inspectorWorkspace"))!;

    private static async Task PropertyInspectorChecks(MainWindow window)
    {
        Invoke(window, "Navigate", "overview");
        var tabs = Field<TabControl>(window, "overviewTabs"); tabs.SelectedIndex = 0; window.UpdateLayout();
        var grid = Field<DataGrid>(window, "properties");
        var search = InspectorControl<TextBox>(window, "Search");
        var selector = InspectorControl<ComboBox>(window, "PageSelector");
        var inspector = InspectorControl<Border>(window, "Inspector");
        var full = InspectorControl<TextBox>(window, "FullValue");
        var rows = grid.Items.OfType<PropertyRow>().ToArray();
        var capabilities = rows.Single(r => r.Key == "capabilities");
        var priorClipboard = Clipboard.ContainsText() ? Clipboard.GetText() : null;
        try
        {
            Check(selector.Items.Cast<object>().Select(p => p.GetType().GetProperty("Id")!.GetValue(p)?.ToString()).SequenceEqual(tabs.Items.Cast<TabItem>().Select(t => t.Tag?.ToString())), "page selector preserves dynamic page order and all specialized pages");
            Check(!selector.Items.OfType<UIElement>().Any() && selector.ActualHeight < 60 && grid.ActualWidth > 400, "selector owns labels only and cannot reparent the live page contents");
            Check(((RowDefinition)window.FindName("OutputRow")).Height.Value == 0 && ((TextBlock)window.FindName("OutputCaption")).IsVisible, "activity drawer starts collapsed with a visible re-open control");
            search.Text = "CAPABILITIES"; window.UpdateLayout();
            Check(grid.Items.Count == 1 && ReferenceEquals(grid.Items[0], capabilities), "property search matches identifiers case-insensitively without replacing rows");
            grid.SelectedItem = capabilities; window.UpdateLayout();
            Check(inspector.IsVisible && full.Text.Split(Environment.NewLine).SequenceEqual(capabilities.Value.Split(',', StringSplitOptions.TrimEntries | StringSplitOptions.RemoveEmptyEntries)), "selected capability summary opens the complete token list");
            Check(Visuals<TextBlock>(grid).Any(t => t.Text.EndsWith("项 · 查看完整值")), "capability row has a compact preview without replacing its source value");
            InspectorControl<Button>(window, "CopyValue").RaiseEvent(new RoutedEventArgs(Button.ClickEvent));
            var copied = Clipboard.GetText();
            Check(copied == capabilities.Value, "copy complete value retains the exact original separators and bytes of text" + (copied == capabilities.Value ? "" : $"; expected={System.Text.Json.JsonSerializer.Serialize(capabilities.Value)}, actual={System.Text.Json.JsonSerializer.Serialize(copied)}"));
            var originalValue = capabilities.Value; capabilities.Value = "exec, file\n中文"; window.UpdateLayout();
            Check(full.Text.Contains("中文"), "inspector follows value notifications from the same selected row");
            capabilities.Value = originalValue;
            search.Text = "__no_property_matches__"; window.UpdateLayout();
            Check(grid.Items.Count == 0 && !inspector.IsVisible && full.Text == "" && Visuals<TextBlock>(window).Any(t => t.Text.Contains("没有匹配的属性")), "unmatched search shows an empty state and clears the hidden selection");
            search.Clear(); window.UpdateLayout();
            Check(grid.Items.Cast<PropertyRow>().SequenceEqual(rows), "clearing search restores the exact template order and row identity");
            search.Text = capabilities.Name; window.UpdateLayout();
            Check(grid.Items.Contains(capabilities), "search matches displayed property names");
            search.Text = "tunnel"; window.UpdateLayout();
            Check(grid.Items.Contains(capabilities), "search matches the full raw value behind a compact preview");
            search.Clear(); grid.SelectedItem = capabilities; window.UpdateLayout();
            Invoke(window, "UpdateOverview"); window.UpdateLayout();
            Check(ReferenceEquals(grid.SelectedItem, capabilities) && inspector.IsVisible, "snapshot refresh retains the selected field and open inspector");
            foreach (var theme in new[] { "Light", "Dark" }) { Theme.Apply(theme); Render(window, "property-inspector-" + theme.ToLowerInvariant() + ".png"); }
            selector.SelectedValue = "history"; window.UpdateLayout();
            Check((tabs.SelectedItem as TabItem)?.Tag?.ToString() == "history" && !search.IsVisible && !inspector.IsVisible, "history remains reachable and never shows stale property tools");
            selector.SelectedIndex = 0; window.UpdateLayout(); grid.SelectedItem = capabilities;
            var devices = (DataGrid)window.FindName("DevicesGrid"); var device = devices.SelectedItem;
            devices.SelectedItem = devices.Items.Cast<Device>().First(d => !ReferenceEquals(d, device)); window.UpdateLayout();
            Check(!inspector.IsVisible && full.Text.Length == 0 && search.Text.Length == 0, "switching devices clears the prior device field selection and search");
            devices.SelectedItem = device;
            var width = window.Width; var height = window.Height;
            window.Width = 960; window.Height = 640; await Task.Delay(100); window.UpdateLayout();
            grid = Field<DataGrid>(window, "properties"); grid.SelectedItem = grid.Items.OfType<PropertyRow>().First(); window.UpdateLayout();
            var root = (FrameworkElement)Field<object>(window, "inspectorWorkspace");
            Check(inspector.IsVisible && Grid.GetRow(inspector) == 1 && inspector.TranslatePoint(new(inspector.ActualWidth, inspector.ActualHeight), root).Y <= root.ActualHeight + 1 && grid.ActualHeight > 100, "narrow workspace docks the inspector below the table without hiding navigation or fields");
            Check(full.ActualHeight > 15 && full.TranslatePoint(new(0, 15), inspector).Y <= inspector.ActualHeight && Visuals<Button>(inspector).Any(b => b.IsVisible && b.Content?.ToString() == "复制完整值" && b.TranslatePoint(new(0, b.ActualHeight), inspector).Y <= inspector.ActualHeight), "narrow inspector keeps the raw value and copy action inside its visible bounds");
            var workspaceTabs = (TabControl)window.FindName("WorkspaceTabs");
            Check(workspaceTabs.Items.Cast<TabItem>().All(t => t.TranslatePoint(new(t.ActualWidth, 0), workspaceTabs).X <= workspaceTabs.ActualWidth + 1), "narrow primary navigation keeps every module within the workspace width");
            Render(window, "property-inspector-narrow.png");
            window.Width = width; window.Height = height;
        }
        finally
        {
            search.Clear(); tabs.SelectedIndex = 0; grid.SelectedItem = null;
            if (priorClipboard != null) Clipboard.SetText(priorClipboard);
            Theme.Apply("Light"); window.UpdateLayout();
        }
    }
}
