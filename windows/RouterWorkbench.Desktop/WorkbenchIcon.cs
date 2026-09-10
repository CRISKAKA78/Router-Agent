using System.Globalization;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Media;

namespace RouterWorkbench.Desktop;

// Published monochrome Fluent icons, independent of the user's text font.
public sealed class WorkbenchIcon : Control
{
    public static readonly DependencyProperty KindProperty = DependencyProperty.Register(nameof(Kind), typeof(string), typeof(WorkbenchIcon), new FrameworkPropertyMetadata("details", FrameworkPropertyMetadataOptions.AffectsRender));
    public string Kind { get => (string)GetValue(KindProperty); set => SetValue(KindProperty, value); }
    static WorkbenchIcon()
    {
        WidthProperty.OverrideMetadata(typeof(WorkbenchIcon), new FrameworkPropertyMetadata(16d));
        HeightProperty.OverrideMetadata(typeof(WorkbenchIcon), new FrameworkPropertyMetadata(16d));
        IsHitTestVisibleProperty.OverrideMetadata(typeof(WorkbenchIcon), new FrameworkPropertyMetadata(false));
        FocusableProperty.OverrideMetadata(typeof(WorkbenchIcon), new FrameworkPropertyMetadata(false));
    }
    protected override void OnRender(DrawingContext dc)
    {
        base.OnRender(dc);
        // Published Fluent System Icons paths, unchanged; see Assets/Fluent/sources.json.
        var geometry = TryFindResource("Icon." + Kind) as Geometry ?? TryFindResource("Icon.details") as Geometry;
        if (geometry == null) return;
        var units = Kind is "caret-down" or "chevron-down" or "chevron-right" or "checkmark" or "subtract" or "memory" ? 16d : 20d;
        var scale = Math.Min(ActualWidth, ActualHeight) / units;
        dc.PushTransform(new TranslateTransform((ActualWidth - units * scale) / 2, (ActualHeight - units * scale) / 2));
        dc.PushTransform(new ScaleTransform(scale, scale));
        dc.DrawGeometry(Foreground, null, geometry);
        dc.Pop(); dc.Pop();
    }
    public static string ForSection(string name) => name switch {
        "硬件平台" or "系统与硬件" or "CPU" => "cpu", "内存" => "memory", "存储" or "存储空间" or "文件系统 · 使用情况" => "storage",
        "运行状态" or "运行与连接" => "runtime", "探针能力" or "固件与版本" => "probe", "连接历史" => "history", "基础身份" => "device",
        _ when name.Contains("网络") || name.Contains("接口") || name.Contains("路由") || name.Contains("DNS") || name.Contains("出口") => "network",
        _ when name.Contains("4G") || name.Contains("5G") || name.Contains("小区") || name.Contains("射频") || name.Contains("注册") => "signal",
        _ => "details"
    };
}

public sealed class SectionIconConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture) => WorkbenchIcon.ForSection(SectionTitleConverter.Title(value));
    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture) => throw new NotSupportedException();
}

public sealed class SectionTitleConverter : IValueConverter
{
    internal static string Title(object? value) => (value?.ToString() ?? "").Split('\u001f')[0];
    public object Convert(object value, Type targetType, object parameter, CultureInfo culture) => Title(value);
    public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture) => throw new NotSupportedException();
}
