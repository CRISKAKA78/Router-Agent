using System.ComponentModel;
using System.Globalization;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Data;
using System.Windows.Input;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

internal static class PropertySheet
{
    internal sealed record Layout(PropertyRow[] Rows, Dictionary<string, string> Sections);

    internal static DataGrid Create(string name, bool system = false, bool inspectable = false)
    {
        var table = Ui.Table(name, ("属性", "Name", 164), ("值", "Value", 300));
        TableBehavior.SetCompactProperties(table, true);
        TableBehavior.SetPropertyInset(table, inspectable ? 2 : system ? 18 : 0);
        TableBehavior.SetInspectorTable(table, inspectable);
        table.SetResourceReference(FrameworkElement.StyleProperty, "PropertySheet");
        // A local CellStyle wins WPF's implicit DataGridCell style during property transfer.
        table.SetResourceReference(DataGrid.CellStyleProperty, "PropertyCell");
        ((DataGridTextColumn)table.Columns[0]).ElementStyle = (Style)Application.Current.FindResource("InspectorLabel");
        var valueStyle = new Style(typeof(TextBlock), Ui.CellTextStyle("ValueTip"));
        valueStyle.Setters.Add(new Setter(TextBlock.TextAlignmentProperty, TextAlignment.Left));
        var capabilities = new DataTrigger { Binding = new Binding("Key"), Value = "capabilities" };
        capabilities.Setters.Add(new Setter(TextBlock.TextWrappingProperty, TextWrapping.Wrap));
        if (inspectable) {
            capabilities.Setters.Add(new Setter(TextBlock.ForegroundProperty, new DynamicResourceExtension("Accent")));
            capabilities.Setters.Add(new Setter(TextBlock.FontWeightProperty, FontWeights.SemiBold));
        }
        valueStyle.Triggers.Add(capabilities);
        if (system)
        {
            var cell = new Style(typeof(DataGridCell), (Style)Application.Current.FindResource("PropertyCell"));
            cell.Setters.Add(new Setter(TableBehavior.WrapModeProperty, TextWrapping.Wrap));
            table.Columns[1].CellStyle = cell;
            foreach (var key in new[] { "status", "uptime", "firmware", "source_ip", "egress_ipv4", "egress_ipv6", "configuration_state", "probe_version", "template_version" }) {
                if (inspectable && key is not ("status" or "configuration_state")) continue;
                var important = new DataTrigger { Binding = new Binding("Key"), Value = key };
                important.Setters.Add(new Setter(TextBlock.FontWeightProperty, FontWeights.SemiBold)); valueStyle.Triggers.Add(important);
            }
        }
        ((DataGridTextColumn)table.Columns[1]).ElementStyle = valueStyle;
        if (inspectable) {
            table.SetResourceReference(FrameworkElement.StyleProperty, "InspectorTable");
            table.SetResourceReference(DataGrid.CellStyleProperty, "InspectorTableCell");
            table.SetResourceReference(DataGrid.ColumnHeaderStyleProperty, "InspectorColumnHeader");
            table.SetResourceReference(Control.FontSizeProperty, "UiInspectorFontSize");
            table.Columns[0].Header = "属性"; table.Columns[1].Header = "当前值";
            if (system) {
                var cell = new Style(typeof(DataGridCell), (Style)Application.Current.FindResource("InspectorTableCell"));
                cell.Setters.Add(new Setter(TableBehavior.WrapModeProperty, TextWrapping.Wrap));
                table.Columns[1].CellStyle = cell;
            }
            var labelStyle = new Style(typeof(TextBlock), (Style)Application.Current.FindResource("InspectorLabel"));
            labelStyle.Setters.Add(new Setter(TextBlock.ForegroundProperty, new DynamicResourceExtension("Text")));
            var selectedLabel = new DataTrigger { Binding = new Binding("IsSelected") { RelativeSource = new RelativeSource(RelativeSourceMode.FindAncestor, typeof(DataGridRow), 1) }, Value = true };
            selectedLabel.Setters.Add(new Setter(TextBlock.FontWeightProperty, FontWeights.SemiBold)); labelStyle.Triggers.Add(selectedLabel);
            ((DataGridTextColumn)table.Columns[0]).ElementStyle = labelStyle;
        } else table.GroupStyle.Add((GroupStyle)Application.Current.FindResource(system ? "SystemPropertyCard" : "PropertySection"));
        table.SizeChanged += (_, _) => TableBehavior.FitPropertyColumns(table, table.Items.OfType<PropertyRow>());
        return table;
    }

    // Explicit presentation is already ordered by PresentRows. Visual sections never reorder it.
    internal static Layout Arrange(PropertyRow[] rows, Presentation? presentation, string pageId, string pageName)
    {
        var explicitLayout = rows.Any(row => presentation?.Fields.ContainsKey(row.Key) == true);
        (int Order, string Name) Section(PropertyRow row)
        {
            if (explicitLayout && pageId != "builtin_system") return (0, pageName);
            if (pageId == "builtin_system") return row.Key switch {
                "managed_name" or "hostname" or "device_id" or "serial" or "model" or "managed_model" or "boot_id" => (0, "基础身份"),
                "kernel" or "kernel_arch" or "kernel_bits" or "arch" or "libc" or "probe_bits" => (1, "系统与硬件"),
                _ when row.Key.StartsWith("cpu_", StringComparison.Ordinal) => (1, "系统与硬件"),
                "firmware" or "probe_version" or "capabilities" => (3, "固件与版本"),
                _ when row.Key.StartsWith("template_", StringComparison.Ordinal) => (3, "固件与版本"),
                _ => (2, "运行与连接")
            };
            if (pageId == "builtin_resources") return row.Key switch {
                _ when row.Key.StartsWith("cpu_", StringComparison.Ordinal) => (0, "CPU"),
                _ when row.Key.StartsWith("memory_", StringComparison.Ordinal) => (1, "内存"),
                _ when row.Key.StartsWith("disk_", StringComparison.Ordinal) => (2, "存储"),
                _ when row.Key.StartsWith("net_", StringComparison.Ordinal) => (3, "网络接口"),
                _ => (3, pageName)
            };
            return (0, pageName);
        }
        var readingOrder = new[] { "managed_name", "device_id", "model", "managed_model", "serial", "hostname", "boot_id",
            "cpu_model", "cpu_arch", "cpu_hardware_bits", "kernel", "kernel_arch", "kernel_bits", "arch", "libc", "probe_bits",
            "status", "uptime", "source_ip", "configuration_state", "last_heartbeat", "last_online_at", "first_seen_at", "last_offline_at",
            "firmware", "probe_version", "template_name", "template_version", "template_id", "capabilities" };
        int Rank(PropertyRow row) { var index = Array.IndexOf(readingOrder, row.Key); return pageId == "builtin_system" && index >= 0 ? index : int.MaxValue; }
        var ordered = explicitLayout ? rows : rows.OrderBy(row => Section(row).Order).ThenBy(Rank).ToArray();
        var sections = new Dictionary<string, string>();
        string? previous = null; var segment = 0;
        foreach (var row in ordered)
        {
            var title = Section(row).Name;
            if (title != previous) { segment++; previous = title; }
            // Unique contiguous sections keep explicit order even when a template interleaves semantics.
            sections[row.Key] = explicitLayout && pageId == "builtin_system" ? title + '\u001f' + segment : title;
        }
        return new(ordered, sections);
    }

    internal static void Bind(DataGrid grid, PropertyRow[] rows, Dictionary<string, string> sections)
    {
        var view = new ListCollectionView(rows);
        if (!TableBehavior.GetInspectorTable(grid)) view.GroupDescriptions.Add(new SectionDescription(sections));
        grid.ItemsSource = view;
    }

    private sealed class SectionDescription(Dictionary<string, string> sections) : GroupDescription
    {
        public override object GroupNameFromItem(object item, int level, CultureInfo culture) =>
            item is PropertyRow row ? sections.GetValueOrDefault(row.Key, row.Group) : "";
    }
}

// Two continuous sheets share the existing row objects. Narrow windows reunite them.
internal sealed class PropertySheetColumns : Grid
{
    internal DataGrid First { get; }
    internal DataGrid Second { get; }
    private PropertyRow[] rows = [];
    private Dictionary<string, string> sections = [];
    private bool? wide;
    internal PropertySheetColumns(DataGrid first, string name)
    {
        First = first; Second = PropertySheet.Create(name + " · 右侧", system: true);
        ColumnDefinitions.Add(new() { Width = new(1, GridUnitType.Star) });
        ColumnDefinitions.Add(new() { Width = new(1, GridUnitType.Star) });
        Children.Add(First); SetColumn(Second, 1); Children.Add(Second);
        First.SelectionChanged += (_, e) => { if (e.AddedItems.Count > 0) Second.SelectedItem = null; };
        Second.SelectionChanged += (_, e) => { if (e.AddedItems.Count > 0) First.SelectedItem = null; };
        SizeChanged += (_, _) => Reflow(false);
    }
    protected override void OnPropertyChanged(DependencyPropertyChangedEventArgs e)
    {
        base.OnPropertyChanged(e);
        if (e.Property == Control.FontSizeProperty && First != null) Dispatcher.BeginInvoke(() => Reflow(false));
    }
    internal void Bind(PropertyRow[] next, Dictionary<string, string> groups)
    {
        rows = next; sections = groups; Reflow(true);
    }
    private void Reflow(bool force)
    {
        var useTwo = ActualWidth >= 820 * First.FontSize / 13 && rows.Length > 1;
        if (!force && wide == useTwo) return;
        var selected = First.SelectedItem as PropertyRow ?? Second.SelectedItem as PropertyRow;
        var anchor = TableBehavior.Capture(First);
        wide = useTwo;
        var left = rows; PropertyRow[] right = [];
        if (useTwo)
        {
            var identityAndRuntime = rows.Where(r => sections.GetValueOrDefault(r.Key) is "基础身份" or "运行与连接").ToArray();
            if (identityAndRuntime.Length > 0 && identityAndRuntime.Length < rows.Length) { left = identityAndRuntime; right = rows.Except(identityAndRuntime).ToArray(); }
            else
            {
                // Template order remains authoritative. Prefer a whole-section boundary.
                var breaks = Enumerable.Range(1, rows.Length - 1).Where(i => sections.GetValueOrDefault(rows[i-1].Key) != sections.GetValueOrDefault(rows[i].Key)).ToArray();
                var split = breaks.Length > 0 ? breaks.MinBy(i => Math.Abs(i - rows.Length / 2)) : (rows.Length + 1) / 2;
                left = rows[..split]; right = rows[split..];
            }
        }
        Second.Visibility = useTwo ? Visibility.Visible : Visibility.Collapsed;
        SetColumnSpan(First, useTwo ? 1 : 2);
        PropertySheet.Bind(First, left, sections); PropertySheet.Bind(Second, right, sections);
        First.SelectedItem = left.FirstOrDefault(r => r.Key == selected?.Key);
        Second.SelectedItem = right.FirstOrDefault(r => r.Key == selected?.Key);
        TableBehavior.Restore(First, anchor);
        TableBehavior.FitPropertyColumns(First, left); TableBehavior.FitPropertyColumns(Second, right);
    }
}

// A real resize handle in every property cell replaces the removed column heading.
// Keeping the row and column containers also preserves selection, copying and virtualization.
public sealed class PropertyColumnThumb : Thumb
{
    private double initialWidth, delta;
    public PropertyColumnThumb()
    {
        DragStarted += (_, _) => { initialWidth = Cell?.Column.ActualWidth ?? 0; delta = 0; };
        DragDelta += (_, e) => { delta += e.HorizontalChange; Resize(initialWidth + delta); };
    }
    private DataGridCell? Cell => TableBehavior.Ancestor<DataGridCell>(this);
    private void Resize(double width)
    {
        if (Cell is { } cell && TableBehavior.Ancestor<DataGrid>(cell) is { } grid)
            TableBehavior.ResizeColumn(grid, cell.Column, width);
    }
    protected override void OnKeyDown(KeyEventArgs e)
    {
        if (e.Key is Key.Left or Key.Right && Cell is { } cell)
        {
            Resize(cell.Column.ActualWidth + (e.Key == Key.Left ? -1 : 1) * (Keyboard.Modifiers.HasFlag(ModifierKeys.Shift) ? 1 : 10));
            e.Handled = true;
        }
        base.OnKeyDown(e);
    }
}
