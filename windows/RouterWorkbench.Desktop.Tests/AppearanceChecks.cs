using System.IO;
using System.Globalization;
using System.Reflection;
using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Input;
using System.Windows.Media;
using RouterWorkbench.Client;
using RouterWorkbench.Core;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    // Drive WPF's mouse-over propagation without moving the user's physical cursor or sending clicks.
    private static void Hover(IInputElement? element) => typeof(MouseDevice).GetMethod("ChangeMouseOver", BindingFlags.Instance | BindingFlags.NonPublic)!.Invoke(Mouse.PrimaryDevice, [element, Environment.TickCount]);
    private static void FocusWithin(DependencyObject? element) => typeof(KeyboardDevice).GetMethod("ChangeFocus", BindingFlags.Instance | BindingFlags.NonPublic)!.Invoke(Keyboard.PrimaryDevice, [element, Environment.TickCount]);
    private static Color ColorOf(Brush? brush) => (brush as SolidColorBrush)?.Color ?? Colors.Transparent;

    private static async Task AppearanceChecks(MainWindow window, string profilePath)
    {
        var legacyPath = Path.Combine(output, "legacy-font-profile.json");
        await File.WriteAllTextAsync(legacyPath, "{\"schema_version\":1,\"server_url\":\"http://127.0.0.1:8080\",\"ssh_user\":\"operator\"}");
        var legacy = ServerProfile.Load(legacyPath);
        Check(legacy.UiFontFamily == ServerProfile.DefaultUiFontFamily && legacy.UiFontSize == 13 && legacy.ServerUrl == "http://127.0.0.1:8080", "old profiles discard SSH account and retain connection and typography");
        await legacy.SaveAsync(legacyPath);
        Check(!File.ReadAllText(legacyPath).Contains("ssh_user"), "saving legacy profile removes retired SSH account");
        await File.WriteAllTextAsync(legacyPath, JsonSerializer.Serialize(legacy with { UiFontSize = 999, UiFontFamily = "" }, Wire.Json));
        Check(ServerProfile.Load(legacyPath).UiFontSize == 13 && ServerProfile.Load(legacyPath).UiFontFamily == ServerProfile.DefaultUiFontFamily, "invalid stored typography falls back without discarding the profile");

        Invoke(window, "Navigate", "overview"); var overview = Field<TabControl>(window, "overviewTabs"); overview.SelectedIndex = 0;
        var grid = Field<DataGrid>(window, "properties"); window.UpdateLayout();
        Visuals<ScrollViewer>(grid).First().ScrollToTop(); window.UpdateLayout();
        var rows = Visuals<DataGridRow>(grid).Where(r => r.IsVisible).Take(3).ToArray();
        var firstCell = Visuals<DataGridCell>(rows[0]).First();
        InputFeedback.SetKeyboardMode(window, false); FocusWithin(firstCell); grid.SelectedItem = null; window.UpdateLayout();
        Check(firstCell.IsKeyboardFocusWithin, "WPF focus propagation reproduces the first-cell focus state");
        foreach (var theme in new[] { "Light", "Dark" }) {
            Theme.Apply(theme); window.UpdateLayout();
            rows = Visuals<DataGridRow>(grid).Where(r => r.IsVisible).Take(3).ToArray(); firstCell = Visuals<DataGridCell>(rows[0]).First();
            FocusWithin(firstCell); Hover(Visuals<DataGridCell>(rows[1]).First()); window.UpdateLayout();
            var selectedColor = ColorOf((Brush)window.FindResource("Selected"));
            Check(rows[1].IsMouseOver && grid.SelectedItems.Count == 0 && ColorOf(rows[1].Background) != selectedColor && ColorOf(firstCell.Background) == Colors.Transparent,
                $"{theme} hover and first-cell focus do not paint a false selection");
            Check(firstCell.FocusVisualStyle == null && ((Border)firstCell.Template.FindName("KeyboardOutline", firstCell)).Visibility == Visibility.Collapsed,
                $"{theme} pointer mode has no automatic dotted cell focus box");
            grid.SelectedItem = rows[2].Item; Hover(Visuals<DataGridCell>(rows[1]).First()); window.UpdateLayout();
            Check(grid.SelectedItems.Count == 1 && rows[2].IsSelected && !rows[1].IsSelected && ColorOf(rows[2].Background) == selectedColor && ColorOf(rows[1].Background) != selectedColor,
                $"{theme} only the actual selected row retains selection fill while another row is hovered");
            Render(window, $"selection-{theme.ToLowerInvariant()}.png"); grid.SelectedItem = null;
        }
        window.RaiseEvent(new KeyEventArgs(Keyboard.PrimaryDevice, PresentationSource.FromVisual(window), 0, Key.Tab) { RoutedEvent = Keyboard.PreviewKeyDownEvent });
        FocusWithin(firstCell); grid.SelectedItem = null; window.UpdateLayout();
        Check(InputFeedback.GetKeyboardMode(firstCell) && ((Border)firstCell.Template.FindName("KeyboardOutline", firstCell)).Visibility == Visibility.Visible && ColorOf(firstCell.Background) == Colors.Transparent,
            "keyboard navigation has a focus outline without falsifying selection");
        window.RaiseEvent(new MouseButtonEventArgs(Mouse.PrimaryDevice, Environment.TickCount, MouseButton.Left) { RoutedEvent = Mouse.PreviewMouseDownEvent });
        Check(!InputFeedback.GetKeyboardMode(firstCell), "pointer input exits keyboard-only focus feedback");
        foreach (var tabs in new[] { (TabControl)window.FindName("WorkspaceTabs") }) {
            var current = tabs.SelectedItem; var other = tabs.Items.Cast<TabItem>().First(t => !t.IsSelected);
            Hover(other); window.UpdateLayout();
            var border = (Border)other.Template.FindName("Tab", other);
            Check(other.IsMouseOver && ReferenceEquals(tabs.SelectedItem, current) && tabs.Items.Cast<TabItem>().Count(t => t.IsSelected) == 1 && ColorOf(border.Background) == (ReferenceEquals(tabs, overview) ? ColorOf((Brush)window.FindResource("Hover")) : Colors.Transparent),
                "hovering a different tab preserves the active page and does not use selection fill");
        }
        Hover(null); FocusWithin(null); Theme.Apply("Light");
        var workspace = (TabControl)window.FindName("WorkspaceTabs"); var originalPage = workspace.SelectedItem;
        foreach (TabItem tab in workspace.Items) {
            workspace.SelectedItem = tab; window.UpdateLayout();
            Check(tab.Content is FrameworkElement page && ColorOf(System.Windows.Documents.TextElement.GetForeground(page)) == ColorOf((Brush)window.FindResource("Text")), "workspace body does not inherit the active navigation color: " + tab.Tag);
        }
        workspace.SelectedItem = originalPage; window.UpdateLayout();
        var devices = (DataGrid)window.FindName("DevicesGrid");
        var originalDevice = devices.SelectedItem;
        var otherDevice = Visuals<DataGridRow>(devices).First(r => r.IsVisible && !r.IsSelected);
        var targetDevice = otherDevice.Item;
        Visuals<DataGridCell>(otherDevice).First().RaiseEvent(new MouseButtonEventArgs(Mouse.PrimaryDevice, Environment.TickCount, MouseButton.Right) { RoutedEvent = Mouse.PreviewMouseDownEvent });
        Check(ReferenceEquals(devices.SelectedItem, targetDevice) && devices.SelectedItems.Count == 1, "right-click selects the target device without mutating a single-selection collection");
        devices.SelectedItem = originalDevice; window.UpdateLayout();
        overview.SelectedIndex = 0; grid = Field<DataGrid>(window, "properties");

        var settingsDone = new TaskCompletionSource();
        _ = window.Dispatcher.BeginInvoke(new Action(() => { Invoke(window, "OpenSettings"); settingsDone.SetResult(); }));
        await Eventually(() => Task.FromResult(Application.Current.Windows.Cast<Window>().Any(w => w.Title == "设置")), "font settings opens in existing settings dialog");
        var dialog = Application.Current.Windows.Cast<Window>().Single(w => w.Title == "设置");
        var font = Field<ComboBox>(window, "uiFont"); var size = Field<ComboBox>(window, "uiFontSize");
        try {
            font.IsDropDownOpen = true; dialog.UpdateLayout();
            var candidate = (ComboBoxItem)font.ItemContainerGenerator.ContainerFromIndex(1);
            typeof(ComboBoxItem).GetProperty(nameof(ComboBoxItem.IsHighlighted))!.SetValue(candidate, true);
            var border = (Border)candidate.Template.FindName("B", candidate);
            Check(!candidate.IsSelected && ColorOf(border.Background) == ColorOf((Brush)window.FindResource("Hover")) && ColorOf(border.BorderBrush) == Colors.Transparent,
                "highlighted dropdown candidate is visually distinct from its selected value");
            font.IsDropDownOpen = false;
            string Family(object choice) => (string)choice.GetType().GetProperty("Family")!.GetValue(choice)!;
            var choice = font.Items.Cast<object>().FirstOrDefault(f => Family(f) == "Microsoft YaHei UI") ?? font.Items[1];
            font.SelectedItem = choice; var family = Family(choice); size.Text = "18";
            var source = grid.ItemsSource; var row = grid.Items[0]; grid.SelectedItem = row;
            await InvokeAsync(window, "SaveTypography", false); dialog.UpdateLayout(); window.UpdateLayout();
            Check(window.FontFamily.Source == family && dialog.FontFamily.Source == family && window.FontSize == 18 && grid.FontSize == 20,
                "chosen font and size apply to existing workspace, table and modal window");
            Check(Visuals<Menu>(window).All(m => m.FontFamily.Source == family && m.FontSize == 18), "main menu follows the chosen typography");
            Check(ReferenceEquals(grid.SelectedItem, row) && grid.Items.Contains(row), "font reflow preserves the selected property object");
            Check(ServerProfile.Load(profilePath) is { UiFontSize: 18 } stored && stored.UiFontFamily == family, "font preferences persist in the existing local profile");
            Check(Visuals<DataGridCell>(grid).SelectMany(Visuals<TextBlock>).All(t => t.TextAlignment == TextAlignment.Left), "Inspector labels and values retain aligned reading starts after typography changes");
            Render(dialog, "font-settings-18.png"); Render(window, "font-workspace-18.png");
            var before = File.ReadAllText(profilePath); size.Text = "NaN"; await InvokeAsync(window, "SaveTypography", false);
            Check(File.ReadAllText(profilePath) == before && window.FontSize == 18 && Field<TextBlock>(window, "fontStatus").Text.Contains("10～24"), "invalid font size stays in settings and changes neither saved nor live preferences");
            size.Text = "24"; await InvokeAsync(window, "SaveTypography", false); Theme.Apply("Dark"); window.UpdateLayout(); dialog.UpdateLayout();
            Check(Visuals<DataGridRow>(grid).Where(r => r.IsVisible).All(r => r.ActualHeight >= grid.FontSize + 6), "large typography grows row heights instead of clipping text");
            var activity = (DataGrid)window.FindName("ActivityGrid");
            var outputLabels = Visuals<DataGridCell>(activity).Where(c => c.IsVisible && c.Column.DisplayIndex < 2).SelectMany(Visuals<TextBlock>).ToArray();
            Render(window, "font-workspace-24-dark.png");
            Check(outputLabels.Length > 0 && outputLabels.All(t => t.ActualWidth + 1 >= new FormattedText(t.Text, CultureInfo.CurrentCulture, t.FlowDirection,
                new Typeface(t.FontFamily, t.FontStyle, t.FontWeight, t.FontStretch), t.FontSize, Brushes.Black, VisualTreeHelper.GetDpi(t).PixelsPerDip).WidthIncludingTrailingWhitespace),
                "large typography keeps output timestamps and levels fully visible");
            var chart = new NetworkRateWindow(window, "test", "字体预览") { Width = 650 }; chart.Show(); chart.UpdateLayout(); Render(chart, "font-chart-24-dark.png"); chart.Close();
            await InvokeAsync(window, "SaveTypography", true); Theme.Apply("Light");
            Check(window.FontSize == 13 && ServerProfile.Load(profilePath).UiFontFamily == ServerProfile.DefaultUiFontFamily, "restore default typography is applied and saved");
        }
        finally { Hover(null); font.IsDropDownOpen = false; dialog.Close(); await settingsDone.Task; }
    }
}
