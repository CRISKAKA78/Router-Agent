using System.ComponentModel;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Input;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

// A presentation-only view over the existing, ordered PropertyRow objects.
internal sealed class PropertyInspectorWorkspace : Grid
{
    private sealed record PageChoice(string Id, string Title, TabItem Page) { public override string ToString() => Title; }
    internal ComboBox PageSelector { get; } = new() { MinWidth = 120, MaxWidth = 220, DisplayMemberPath = "Title", SelectedValuePath = "Id" };
    internal TextBox Search { get; } = Ui.Input();
    internal Border Inspector { get; }
    internal TextBox FullValue { get; } = Ui.Code("", true);
    internal Button CopyValue { get; }
    private readonly TextBlock title = Ui.Text("属性详情"), category = Ui.Text(""), reason = Ui.Text("", true), order = Ui.Text("模板顺序 · 与设备保持一致", true);
    private readonly Button open;
    private readonly Button compactCopy;
    private readonly DockPanel heading, valueHeading;
    private readonly StackPanel body;
    private readonly Border band;
    private readonly Grid tableRegion = new();
    private readonly TextBlock empty = Ui.Text("", true);
    private readonly TabControl tabs;
    private DataGrid? table;
    private PropertyRow? selected;
    private Dictionary<string, string> sections = [];
    private bool changing, panelOpen;

    internal PropertyInspectorWorkspace(TabControl pages)
    {
        tabs = pages;
        SetResourceReference(System.Windows.Documents.TextElement.ForegroundProperty, "Text");
        SetResourceReference(System.Windows.Documents.TextElement.FontSizeProperty, "UiInspectorFontSize");
        Margin = new(8, 12, 12, 8);
        ColumnDefinitions.Add(new() { Width = new(1, GridUnitType.Star) });
        ColumnDefinitions.Add(new() { Width = new(0) });
        ColumnDefinitions.Add(new() { Width = new(0) });
        RowDefinitions.Add(new() { Height = new(1, GridUnitType.Star) });
        RowDefinitions.Add(new() { Height = new(0) });
        var toolbar = new Grid { Margin = new(0, 0, 0, 10) };
        toolbar.ColumnDefinitions.Add(new() { Width = GridLength.Auto });
        toolbar.ColumnDefinitions.Add(new() { Width = new(1, GridUnitType.Star) });
        toolbar.ColumnDefinitions.Add(new() { Width = GridLength.Auto });
        PageSelector.Margin = new(0, 0, 10, 0); PageSelector.MinHeight = 36;
        PageSelector.FontWeight = FontWeights.SemiBold;
        PageSelector.SetResourceReference(Control.FontSizeProperty, "UiWorkspaceFontSize");
        PageSelector.Padding = new(16, 3, 0, 3);
        PageSelector.SetResourceReference(Control.BackgroundProperty, "Panel");
        AutomationProperties.SetName(PageSelector, "设备详情页面");
        toolbar.Children.Add(PageSelector);
        Search.Tag = "搜索属性名或值"; Search.ToolTip = "筛选当前页面的属性名、标识或完整值 (Ctrl+Shift+F)";
        Search.MinHeight = 36; Search.MaxWidth = 390; Search.HorizontalAlignment = HorizontalAlignment.Stretch;
        ControlChrome.SetIsSearch(Search, true);
        AutomationProperties.SetName(Search, "搜索当前页属性");
        SetColumn(Search, 1); toolbar.Children.Add(Search);
        open = Ui.Button("完整值", () => { if (table?.SelectedItem is PropertyRow row) Select(row); });
        open.MinHeight = 34; open.Margin = new(10, 0, 0, 0); open.IsEnabled = false;
        order.Margin = new(16, 0, 0, 0); order.SetResourceReference(TextBlock.FontSizeProperty, "UiSmallFontSize");
        var hint = new StackPanel { Orientation = Orientation.Horizontal, Children = { order, open } };
        SetColumn(hint, 2); toolbar.Children.Add(hint);
        tableRegion.Children.Add(tabs);
        empty.HorizontalAlignment = HorizontalAlignment.Center; empty.VerticalAlignment = VerticalAlignment.Top;
        empty.Margin = new(12, 60, 12, 12); empty.IsHitTestVisible = false; empty.Visibility = Visibility.Collapsed;
        tableRegion.Children.Add(empty);
        Children.Add(Ui.Page(toolbar, tableRegion));

        var panel = new DockPanel();
        var close = Ui.Button("关闭", CloseInspector); close.ToolTip = "关闭属性详情 (Esc)";
        close.SetResourceReference(FrameworkElement.StyleProperty, "IconButton"); ControlChrome.SetIcon(close, "dismiss"); close.Margin = new(8, 0, 0, 0);
        AutomationProperties.SetName(close, "关闭属性详情");
        title.SetResourceReference(TextBlock.FontSizeProperty, "UiInspectorTitleFontSize"); title.FontWeight = FontWeights.SemiBold;
        title.TextWrapping = TextWrapping.Wrap;
        heading = new DockPanel { Margin = new(14) };
        compactCopy = Ui.Button("复制完整值", () => Copy());
        ControlChrome.SetIcon(compactCopy, "copy");
        compactCopy.Padding = new(8, 3, 8, 3); compactCopy.SetResourceReference(Control.ForegroundProperty, "Accent");
        DockPanel.SetDock(compactCopy, Dock.Right); heading.Children.Add(compactCopy);
        DockPanel.SetDock(close, Dock.Right); heading.Children.Add(close); heading.Children.Add(title);
        DockPanel.SetDock(heading, Dock.Top); panel.Children.Add(heading);
        category.Margin = new(14, 8, 14, 8); category.FontWeight = FontWeights.SemiBold; category.TextWrapping = TextWrapping.Wrap;
        band = new Border { Child = category }; band.SetResourceReference(Border.BackgroundProperty, "Panel");
        DockPanel.SetDock(band, Dock.Top); panel.Children.Add(band);
        body = new StackPanel { Margin = new(14, 20, 14, 14) };
        CopyValue = Ui.Button("复制完整值", () => Copy()); CopyValue.MinHeight = 36;
        ControlChrome.SetIcon(CopyValue, "copy");
        CopyValue.SetResourceReference(Control.ForegroundProperty, "Accent");
        valueHeading = new DockPanel { Margin = new(0, 0, 0, 12) };
        DockPanel.SetDock(CopyValue, Dock.Right); valueHeading.Children.Add(CopyValue);
        var label = Ui.Text("原始值"); label.FontWeight = FontWeights.SemiBold; valueHeading.Children.Add(label);
        DockPanel.SetDock(valueHeading, Dock.Top); body.Children.Add(valueHeading);
        reason.TextWrapping = TextWrapping.Wrap; reason.Margin = new(0, 0, 0, 10);
        reason.SetResourceReference(TextBlock.FontSizeProperty, "UiSmallFontSize");
        DockPanel.SetDock(reason, Dock.Top); body.Children.Add(reason);
        FullValue.TextWrapping = TextWrapping.Wrap; FullValue.AcceptsTab = false;
        FullValue.HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled; FullValue.Padding = new(12);
        FullValue.VerticalAlignment = VerticalAlignment.Top;
        FullValue.FontFamily = new System.Windows.Media.FontFamily("Consolas, Microsoft YaHei UI");
        FullValue.SetResourceReference(Control.BackgroundProperty, "Panel");
        FullValue.SetResourceReference(Control.FontSizeProperty, "UiInspectorFontSize");
        FullValue.SetResourceReference(System.Windows.Documents.Block.LineHeightProperty, "UiCodeLineHeight");
        FullValue.SetValue(System.Windows.Documents.Block.LineStackingStrategyProperty, LineStackingStrategy.BlockLineHeight);
        AutomationProperties.SetName(FullValue, "属性完整原始值");
        body.Children.Add(FullValue);
        panel.Children.Add(new ScrollViewer { Content = body, VerticalScrollBarVisibility = ScrollBarVisibility.Auto, HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled });
        Inspector = new Border { Child = panel, BorderThickness = new(1), Visibility = Visibility.Collapsed };
        Inspector.SetResourceReference(Border.BorderBrushProperty, "Line");
        Inspector.SetResourceReference(Border.BackgroundProperty, "Surface");
        Children.Add(Inspector);
        Search.TextChanged += (_, _) => ApplyFilter();
        PageSelector.SelectionChanged += (_, _) => { if (!changing && PageSelector.SelectedItem is PageChoice choice) tabs.SelectedItem = choice.Page; };
        SizeChanged += (_, _) => Reflow();
        PreviewKeyDown += (_, e) => { if (e.Key == Key.Escape && panelOpen) { CloseInspector(); e.Handled = true; } };
        PreviewKeyDown += (_, e) => {
            if (Keyboard.Modifiers == ModifierKeys.Control && e.Key is Key.PageUp or Key.PageDown && tabs.Items.Count > 0) {
                tabs.SelectedIndex = (tabs.SelectedIndex + (e.Key == Key.PageDown ? 1 : tabs.Items.Count - 1)) % tabs.Items.Count;
                e.Handled = true;
            }
        };
    }

    internal void SyncPages()
    {
        changing = true;
        var pages = tabs.Items.Cast<TabItem>().ToArray();
        if (PageSelector.ItemsSource is not PageChoice[] old || !old.Select(p => (p.Page, p.Title)).SequenceEqual(pages.Select(p => (p, p.Header?.ToString() ?? ""))))
            PageSelector.ItemsSource = pages.Select(p => new PageChoice(p.Tag?.ToString() ?? "", p.Header?.ToString() ?? "", p)).ToArray();
        PageSelector.SelectedValue = (tabs.SelectedItem as TabItem)?.Tag?.ToString();
        changing = false;
    }

    internal void SetPage(DataGrid? next, Dictionary<string, string>? groups = null)
    {
        SyncPages(); sections = groups ?? [];
        if (table != next)
        {
            if (table != null) table.SelectionChanged -= SelectionChanged;
            ClearSelection(); table = next; Search.Clear();
            if (table != null) table.SelectionChanged += SelectionChanged;
        }
        Search.Visibility = order.Visibility = open.Visibility = table == null ? Visibility.Collapsed : Visibility.Visible;
        if (table == null) { ClearSelection(); empty.Visibility = Visibility.Collapsed; }
        else { ApplyFilter(); if (table.SelectedItem is PropertyRow row) Select(row); }
        Reflow();
    }

    internal void RowsUpdated(Dictionary<string, string> groups, bool deviceChanged)
    {
        sections = groups;
        if (deviceChanged) { if (table != null) table.SelectedItem = null; Search.Clear(); ClearSelection(); }
        ApplyFilter();
        if (selected != null) UpdateValue();
    }

    private void ApplyFilter()
    {
        if (table?.ItemsSource == null) return;
        var query = Search.Text.Trim();
        var view = CollectionViewSource.GetDefaultView(table.ItemsSource);
        var anchor = TableBehavior.Capture(table);
        var prior = table.SelectedItem;
        view.Filter = query.Length == 0 ? null : item => item is PropertyRow row &&
            (row.Name.Contains(query, StringComparison.OrdinalIgnoreCase) || row.Key.Contains(query, StringComparison.OrdinalIgnoreCase) || row.Value.Contains(query, StringComparison.OrdinalIgnoreCase));
        if (prior != null && view.Contains(prior)) table.SelectedItem = prior;
        else if (prior != null) { table.SelectedItem = null; ClearSelection(); }
        empty.Text = query.Length == 0 ? "此分组暂无显示属性" : "没有匹配的属性，请修改或清空搜索。";
        empty.Visibility = view.IsEmpty ? Visibility.Visible : Visibility.Collapsed;
        if (query.Length == 0) TableBehavior.Restore(table, anchor);
    }

    private void SelectionChanged(object sender, SelectionChangedEventArgs e)
    {
        if (table?.SelectedItem is PropertyRow row) Select(row); else ClearSelection();
    }
    private void Select(PropertyRow row)
    {
        if (!ReferenceEquals(selected, row))
        {
            if (selected != null) selected.PropertyChanged -= ValueChanged;
            selected = row; selected.PropertyChanged += ValueChanged;
        }
        panelOpen = true; open.IsEnabled = true; UpdateValue(); Reflow();
    }
    private void ValueChanged(object? sender, PropertyChangedEventArgs e) => UpdateValue();
    private void UpdateValue()
    {
        if (selected == null) return;
        category.Text = SectionTitleConverter.Title(sections.GetValueOrDefault(selected.Key, selected.Group));
        title.Text = band.Visibility == Visibility.Visible ? selected.Name : selected.Name + " · " + category.Text;
        var value = selected.Key == "capabilities" && selected.Value != "—"
            ? string.Join(Environment.NewLine, selected.Value.Split(',', StringSplitOptions.TrimEntries | StringSplitOptions.RemoveEmptyEntries)) : selected.Value;
        if (FullValue.Text != value) FullValue.Text = value;
        reason.Text = selected.Key == "capabilities" ? "设备声明支持的操作能力列表。" : selected.ValueTip;
        reason.Visibility = string.IsNullOrEmpty(reason.Text) ? Visibility.Collapsed : Visibility.Visible;
        CopyValue.Content = "复制完整值";
        compactCopy.Content = "复制完整值";
    }
    internal void Copy() { if (selected != null) { Clipboard.SetText(selected.Value); CopyValue.Content = compactCopy.Content = "已复制"; } }
    internal void Reset() { Search.Clear(); if (table != null) table.SelectedItem = null; ClearSelection(); }
    private void ClearSelection()
    {
        if (selected != null) selected.PropertyChanged -= ValueChanged;
        selected = null; panelOpen = false; FullValue.Clear(); open.IsEnabled = false; Reflow();
    }
    private void CloseInspector() { panelOpen = false; Reflow(); table?.Focus(); }
    private void Reflow()
    {
        var scale = (double)FindResource("UiFontSize") / 13;
        order.Visibility = table != null && ActualWidth >= 950 * scale ? Visibility.Visible : Visibility.Collapsed;
        var side = ActualWidth >= 940 * scale;
        band.Visibility = valueHeading.Visibility = side ? Visibility.Visible : Visibility.Collapsed;
        compactCopy.Visibility = side ? Visibility.Collapsed : Visibility.Visible;
        heading.Margin = side ? new(14) : new(8);
        body.Margin = side ? new(14, 20, 14, 14) : new(8, 0, 8, 8);
        title.SetResourceReference(TextBlock.FontSizeProperty, side ? "UiInspectorTitleFontSize" : "UiInspectorFontSize");
        if (selected != null) title.Text = side ? selected.Name : selected.Name + " · " + category.Text;
        Inspector.Visibility = panelOpen ? Visibility.Visible : Visibility.Collapsed;
        ColumnDefinitions[1].Width = new(panelOpen && side ? 14 : 0);
        ColumnDefinitions[2].Width = new(panelOpen && side ? Math.Clamp(ActualWidth * .34, 280 * scale, 390 * scale) : 0);
        RowDefinitions[1].Height = new(panelOpen && !side ? Math.Min(270 * scale, ActualHeight * .45) : 0);
        SetColumn(Inspector, side ? 2 : 0); SetRow(Inspector, side ? 0 : 1);
        SetColumnSpan(Inspector, side ? 1 : 3);
        Inspector.Margin = new(0, !side && panelOpen ? 10 : 0, 0, 0);
    }
}
