using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Media;
using System.Collections;

namespace RouterWorkbench.Desktop;

internal static class Ui
{
    public static TextBlock Text(string text, bool muted = false) {
        var block = new TextBlock { Text = text }; if (muted) block.SetResourceReference(TextBlock.ForegroundProperty, "Muted"); return block;
    }
    public static TextBox Input(string value = "", double width = double.NaN) => new() { Text = value, Width = width };
    public static TextBox Code(string value = "", bool readOnly = false) {
        var input = new TextBox {
        Text = value, IsReadOnly = readOnly, AcceptsReturn = true, AcceptsTab = true,
        VerticalScrollBarVisibility = ScrollBarVisibility.Auto, HorizontalScrollBarVisibility = ScrollBarVisibility.Auto,
        VerticalContentAlignment = VerticalAlignment.Top, TextWrapping = TextWrapping.NoWrap, BorderThickness = new(0)
        };
        input.SetResourceReference(Control.FontSizeProperty, "UiSmallFontSize"); return input;
    }
    public static Button Button(string title, Action action, bool primary = false) {
        var button = new Button { Content = title }; if (primary) button.SetResourceReference(FrameworkElement.StyleProperty, "PrimaryButton");
        button.Click += (_, _) => action(); return button;
    }
    public static ComboBox Combo(string[] values, int index = 0, double width = 120) => new() { ItemsSource = values, SelectedIndex = index, Width = width, Margin = new(0,0,8,0) };
    public static Border Bar(params UIElement[] children) {
        var panel = new WrapPanel { Orientation = Orientation.Horizontal };
        foreach (var child in children) { if (child is Button button && button.ReadLocalValue(FrameworkElement.StyleProperty) == DependencyProperty.UnsetValue) button.SetResourceReference(FrameworkElement.StyleProperty, "ToolbarButton"); panel.Children.Add(child); }
        var border = new Border { Child = panel, Padding = new(8,4,4,4), BorderThickness = new(0,0,0,1) };
        border.SetResourceReference(Border.BackgroundProperty, "Panel"); border.SetResourceReference(Border.BorderBrushProperty, "Line"); return border;
    }
    public static WrapPanel FormActions(params UIElement[] children) {
        var panel = new WrapPanel { Margin = new(164,0,0,8) };
        foreach (var child in children) panel.Children.Add(child);
        return panel;
    }
    public static Border Heading(string title) {
        var border = new Border { Child = IconLabel(WorkbenchIcon.ForSection(title), title) };
        border.SetResourceReference(FrameworkElement.StyleProperty, "SectionHeading"); return border;
    }
    public static FrameworkElement IconLabel(string icon, string title) => new StackPanel { Orientation = Orientation.Horizontal,
        Children = { new WorkbenchIcon { Kind = icon, Margin = new(0,0,7,0) }, new TextBlock { Text = title } } };
    public static FrameworkElement NavigationHeader(string key, string title) {
        var header = (StackPanel)IconLabel(key switch { "overview" => "details", "maintenance" => "network", "networks" => "network", "files" => "files", "config" => "config", "logs" => "files", _ => "tools" }, title);
        ((WorkbenchIcon)header.Children[0]).Width = ((WorkbenchIcon)header.Children[0]).Height = 20;
        return header;
    }
    public static Style CellTextStyle(string? tip = null) {
        var style = new Style(typeof(TextBlock));
        style.Setters.Add(new Setter(TextBlock.TextTrimmingProperty, TextTrimming.CharacterEllipsis));
        style.Setters.Add(new Setter(TextBlock.TextAlignmentProperty, TextAlignment.Center));
        style.Setters.Add(new Setter(TextBlock.VerticalAlignmentProperty, VerticalAlignment.Center));
        style.Setters.Add(new Setter(TextBlock.TextWrappingProperty, new Binding { RelativeSource = new(RelativeSourceMode.Self), Path = new PropertyPath(TableBehavior.WrapModeProperty) }));
        if(tip != null) style.Setters.Add(new Setter(FrameworkElement.ToolTipProperty, new Binding(tip)));
        return style;
    }
    public static DataGrid Table(string name, params (string Title, string Property, double Width)[] columns) {
        var grid = new DataGrid(); AutomationProperties.SetName(grid, name);
        foreach (var col in columns) {
            grid.Columns.Add(new DataGridTextColumn { Header = col.Title, Binding = new Binding(col.Property), ElementStyle = CellTextStyle(), Width = col.Width < 0 ? new DataGridLength(-col.Width, DataGridLengthUnitType.Star) : new DataGridLength(col.Width), MinWidth = 45 });
        }
        return grid;
    }
    public static void SetRows(DataGrid grid, IEnumerable? rows) {
        var sorting = grid.Items.SortDescriptions.ToArray();
        grid.ItemsSource = rows;
        using (grid.Items.DeferRefresh()) { grid.Items.SortDescriptions.Clear(); foreach (var sort in sorting) grid.Items.SortDescriptions.Add(sort); }
    }
    public static DockPanel Page(UIElement toolbar, UIElement content, UIElement? footnote = null) {
        var panel = new DockPanel(); DockPanel.SetDock(toolbar, Dock.Top); panel.Children.Add(toolbar);
        if (footnote != null) { DockPanel.SetDock(footnote, Dock.Bottom); panel.Children.Add(footnote); }
        panel.Children.Add(content); return panel;
    }
    public static Border Note(string text) {
        var block = Text(text, true); block.TextWrapping = TextWrapping.Wrap;
        return new Border { Padding = new(10,7,10,7), Child = block };
    }
    public static Grid Split(UIElement first, UIElement second, bool vertical = false, double ratio = 1) {
        var grid = new Grid();
        if (vertical) {
            grid.RowDefinitions.Add(new() { Height = new(ratio, GridUnitType.Star), MinHeight = 90 });
            grid.RowDefinitions.Add(new() { Height = new(4) }); grid.RowDefinitions.Add(new() { Height = new(1, GridUnitType.Star), MinHeight = 100 });
            Grid.SetRow(second, 2);
        } else {
            grid.ColumnDefinitions.Add(new() { Width = new(ratio, GridUnitType.Star), MinWidth = 200 });
            grid.ColumnDefinitions.Add(new() { Width = new(4) }); grid.ColumnDefinitions.Add(new() { Width = new(1, GridUnitType.Star), MinWidth = 220 }); Grid.SetColumn(second, 2);
        }
        var splitter = new GridSplitter { HorizontalAlignment = HorizontalAlignment.Stretch, VerticalAlignment = VerticalAlignment.Stretch, ResizeDirection = vertical ? GridResizeDirection.Rows : GridResizeDirection.Columns };
        if (vertical) Grid.SetRow(splitter, 1); else Grid.SetColumn(splitter, 1);
        grid.Children.Add(first); grid.Children.Add(splitter); grid.Children.Add(second); return grid;
    }
    public static FrameworkElement Labeled(string label, FrameworkElement control, double labelWidth = 164) {
        var panel = new Grid { Margin = new(0,0,0,8) };
        panel.ColumnDefinitions.Add(new() { Width = new(labelWidth) }); panel.ColumnDefinitions.Add(new() { Width = new(1, GridUnitType.Star) });
        var text = Text(label, true); text.TextWrapping = TextWrapping.Wrap; text.Margin = new(0,4,12,0); text.VerticalAlignment = VerticalAlignment.Top;
        Grid.SetColumn(control, 1); panel.Children.Add(text); panel.Children.Add(control);
        AutomationProperties.SetName(control, label); return panel;
    }
}

internal sealed record FormField(string Key, string Label, string Value = "", bool Multiline = false, string[]? Choices = null);
internal sealed class FormWindow : Window
{
    public FormWindow(Window owner, string title, string message, FormField[] fields, Func<Dictionary<string,string>, Task> accept) {
        SetResourceReference(StyleProperty, typeof(Window));
        Owner = owner; Title = title; Width = 610; MaxHeight = Math.Max(500, SystemParameters.WorkArea.Height - 100);
        SizeToContent = SizeToContent.Height; ResizeMode = ResizeMode.CanResize; WindowStartupLocation = WindowStartupLocation.CenterOwner;
        var root = new DockPanel { Margin = new(16) }; Content = root;
        var footer = new StackPanel(); var footerBand = new Border { Child = footer, Margin = new(0,12,0,0), Padding = new(0,12,0,0), BorderThickness = new(0,1,0,0) };
        footerBand.SetResourceReference(Border.BorderBrushProperty, "Line"); DockPanel.SetDock(footerBand, Dock.Bottom); root.Children.Add(footerBand);
        var error = Ui.Text(""); error.TextWrapping = TextWrapping.Wrap; error.SetResourceReference(TextBlock.ForegroundProperty, "Error"); footer.Children.Add(error);
        var actions = new StackPanel { Orientation = Orientation.Horizontal, HorizontalAlignment = HorizontalAlignment.Right, Margin = new(0,10,0,0) }; footer.Children.Add(actions);
        var ok = new Button { Content = "确定", IsDefault = true, MinWidth = 78 }; ok.SetResourceReference(StyleProperty, "PrimaryButton");
        var cancel = new Button { Content = "取消", IsCancel = true, MinWidth = 78 }; actions.Children.Add(ok); actions.Children.Add(cancel);
        var body = new StackPanel(); root.Children.Add(new ScrollViewer { Content = body, VerticalScrollBarVisibility = ScrollBarVisibility.Auto, MaxHeight = Math.Max(330, SystemParameters.WorkArea.Height - 270) });
        if (message != "") { var note = Ui.Text(message, true); note.TextWrapping = TextWrapping.Wrap; note.Margin = new(0,0,0,14); body.Children.Add(note); }
        var readers = new Dictionary<string,Func<string>>();
        foreach (var field in fields) {
            FrameworkElement control;
            if (field.Choices != null) {
                var combo = Ui.Combo(field.Choices, Math.Max(0, Array.IndexOf(field.Choices, field.Value)), double.NaN); control = combo; readers[field.Key] = () => combo.SelectedItem?.ToString() ?? "";
            } else {
                var input = field.Multiline ? Ui.Code(field.Value) : Ui.Input(field.Value); if (field.Multiline) { input.Height = 115; input.BorderThickness = new(1); } control = input; readers[field.Key] = () => input.Text;
            }
            body.Children.Add(Ui.Labeled(field.Label, control));
        }
        var working = false;
        Closing += (_, e) => { if (working) e.Cancel = true; };
        ok.Click += async (_, _) => {
            if (working) return; working = true; ok.IsEnabled = cancel.IsEnabled = body.IsEnabled = false; error.Text = "";
            try { await accept(readers.ToDictionary(p => p.Key, p => p.Value())); working = false; DialogResult = true; }
            catch (Exception e) { error.Text = e.Message; }
            finally { working = false; ok.IsEnabled = cancel.IsEnabled = body.IsEnabled = true; }
        };
    }
}
