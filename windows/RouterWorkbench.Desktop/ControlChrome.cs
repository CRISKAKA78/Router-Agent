using System.Windows;

namespace RouterWorkbench.Desktop;

// Visual options shared by native control templates; no input or business behavior.
public static class ControlChrome
{
    public static readonly DependencyProperty IconProperty = DependencyProperty.RegisterAttached("Icon", typeof(string), typeof(ControlChrome), new PropertyMetadata(""));
    public static string GetIcon(DependencyObject target) => (string)target.GetValue(IconProperty);
    public static void SetIcon(DependencyObject target, string value) => target.SetValue(IconProperty, value);
    public static readonly DependencyProperty IsSearchProperty = DependencyProperty.RegisterAttached("IsSearch", typeof(bool), typeof(ControlChrome), new PropertyMetadata(false));
    public static bool GetIsSearch(DependencyObject target) => (bool)target.GetValue(IsSearchProperty);
    public static void SetIsSearch(DependencyObject target, bool value) => target.SetValue(IsSearchProperty, value);
}
