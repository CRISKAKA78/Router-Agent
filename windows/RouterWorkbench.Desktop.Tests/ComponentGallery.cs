using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;

internal static partial class Program
{
    private static Window CreateComponentGallery()
    {
        var window = new Window { Title = "Router Workbench · 组件校准", Width = 1100, Height = 850 };
        window.SetResourceReference(FrameworkElement.StyleProperty, typeof(Window));
        var body = new StackPanel { Margin = new(24) };
        body.Children.Add(new TextBlock { Text = "方案 3 · 字体与组件样板", FontSize = 20, FontWeight = FontWeights.SemiBold, Margin = new(0, 0, 0, 16) });
        var toolbar = new WrapPanel { Margin = new(0, 0, 0, 16) };
        foreach (var theme in new[] { "Light", "Dark" }) {
            var button = new Button { Content = theme == "Light" ? "浅色" : "深色" };
            button.Click += (_, _) => Theme.Apply(theme); toolbar.Children.Add(button);
        }
        foreach (var size in new[] { 13d, 18d, 24d }) {
            var button = new Button { Content = $"字号 {size}" };
            button.Click += (_, _) => Typography.Apply(RouterWorkbench.Core.ServerProfile.DefaultUiFontFamily, size); toolbar.Children.Add(button);
        }
        body.Children.Add(toolbar);
        foreach (var family in new[] { "Segoe UI, Microsoft YaHei UI", "Microsoft YaHei UI", "Microsoft YaHei", "DengXian" }) {
            var row = new Grid { Margin = new(0, 0, 0, 10) };
            row.ColumnDefinitions.Add(new() { Width = new(260) }); row.ColumnDefinitions.Add(new());
            row.Children.Add(new TextBlock { Text = family, FontSize = 13 });
            var sample = new TextBlock { Text = "系统信息　属性 / 当前值　FNR100　716 MHz　在线", FontFamily = new(family), FontSize = 15 };
            Grid.SetColumn(sample, 1); row.Children.Add(sample); body.Children.Add(row);
        }
        var controls = new WrapPanel { Margin = new(0, 18, 0, 18) };
        controls.Children.Add(new ComboBox { Width = 140, ItemsSource = new[] { "系统信息", "资源监控", "设备信息", "网络信息" }, SelectedIndex = 0, Margin = new(0, 0, 12, 0) });
        var search = new TextBox { Tag = "搜索属性名或值", Width = 250, Margin = new(0, 0, 12, 0) }; ControlChrome.SetIsSearch(search, true); controls.Children.Add(search);
        var copy = new Button { Content = "复制完整值" }; ControlChrome.SetIcon(copy, "copy"); controls.Children.Add(copy);
        var primary = new Button { Content = "应用配置" }; primary.SetResourceReference(FrameworkElement.StyleProperty, "PrimaryButton"); controls.Children.Add(primary);
        controls.Children.Add(new Button { Content = "不可用", IsEnabled = false }); body.Children.Add(controls);
        var editable = new ComboBox { IsEditable = true, Width = 310, HorizontalAlignment = HorizontalAlignment.Left, ItemsSource = new[] { "默认模板", "FNR100 · 系统信息", "长名称模板 · 测试内容换行与省略" }, Text = "默认模板", Margin = new(0, 0, 0, 16) };
        body.Children.Add(editable);
        var checks = new WrapPanel { Margin = new(0, 0, 0, 18) };
        foreach (var value in new bool?[] { false, true, null }) checks.Children.Add(new CheckBox { Content = "仅显示在线设备", IsChecked = value, IsThreeState = true });
        checks.Children.Add(new CheckBox { Content = "不可用", IsEnabled = false }); body.Children.Add(checks);
        var grid = new DataGrid { Height = 195, AutoGenerateColumns = false, IsReadOnly = true, ItemsSource = new[] { new { Name = "CPU架构", Value = "ARMv7-A" }, new { Name = "固件版本", Value = "FNR100 v1.1 (Jan 7 2026 11:51:01) std" }, new { Name = "支持操作", Value = "7 项 · 查看完整值" } } };
        grid.Columns.Add(new DataGridTextColumn { Header = "属性", Binding = new System.Windows.Data.Binding("Name"), Width = 240 });
        grid.Columns.Add(new DataGridTextColumn { Header = "当前值", Binding = new System.Windows.Data.Binding("Value"), Width = new(1, DataGridLengthUnitType.Star) });
        grid.SetResourceReference(FrameworkElement.StyleProperty, "InspectorTable");
        grid.SetResourceReference(DataGrid.CellStyleProperty, "InspectorTableCell");
        grid.SetResourceReference(DataGrid.ColumnHeaderStyleProperty, "InspectorColumnHeader");
        grid.SelectedIndex = 2; body.Children.Add(grid);
        var menu = new ContextMenu();
        menu.Items.Add(new MenuItem { Header = "复制完整值", InputGestureText = "Ctrl+C" });
        var more = new MenuItem { Header = "查看方式" }; more.Items.Add(new MenuItem { Header = "自动换行", IsCheckable = true, IsChecked = true }); more.Items.Add(new MenuItem { Header = "单行显示", IsCheckable = true }); menu.Items.Add(more);
        menu.Items.Add(new Separator()); menu.Items.Add(new MenuItem { Header = "不可用操作", IsEnabled = false }); grid.ContextMenu = menu;
        var note = new TextBlock { Text = "用鼠标与键盘检查：下拉展开、可编辑输入、复选状态、焦点、禁用和表格选择。", Margin = new(0, 16, 0, 0), TextWrapping = TextWrapping.Wrap };
        note.SetResourceReference(TextBlock.ForegroundProperty, "Muted"); body.Children.Add(note);
        window.Content = new ScrollViewer { Content = body, VerticalScrollBarVisibility = ScrollBarVisibility.Auto, HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled };
        window.Loaded += (_, _) => {
            foreach (var family in new[] { "Segoe UI", "Microsoft YaHei UI", "Microsoft YaHei", "DengXian" }) {
                var typeface = new Typeface(new FontFamily(family), FontStyles.Normal, FontWeights.Normal, FontStretches.Normal);
                if (typeface.TryGetGlyphTypeface(out var glyph)) Console.WriteLine($"FONT {family}: {glyph.FontUri}; 中={glyph.CharacterToGlyphMap.ContainsKey('中')}; A={glyph.CharacterToGlyphMap.ContainsKey('A')}");
            }
            Console.WriteLine($"WINDOW DPI {VisualTreeHelper.GetDpi(window).PixelsPerInchX}");
        };
        return window;
    }
}
