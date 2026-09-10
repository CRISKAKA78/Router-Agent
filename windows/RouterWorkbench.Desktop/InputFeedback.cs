using System.Windows;
using System.Windows.Input;

namespace RouterWorkbench.Desktop;

// Focus is not selection. Show a cell focus outline only for deliberate keyboard navigation.
public static class InputFeedback
{
    public static readonly DependencyProperty EnabledProperty = DependencyProperty.RegisterAttached("Enabled", typeof(bool), typeof(InputFeedback), new PropertyMetadata(false, Changed));
    public static void SetEnabled(DependencyObject target, bool value) => target.SetValue(EnabledProperty, value);
    public static bool GetEnabled(DependencyObject target) => (bool)target.GetValue(EnabledProperty);
    public static readonly DependencyProperty KeyboardModeProperty = DependencyProperty.RegisterAttached("KeyboardMode", typeof(bool), typeof(InputFeedback), new FrameworkPropertyMetadata(false, FrameworkPropertyMetadataOptions.Inherits));
    public static void SetKeyboardMode(DependencyObject target, bool value) => target.SetValue(KeyboardModeProperty, value);
    public static bool GetKeyboardMode(DependencyObject target) => (bool)target.GetValue(KeyboardModeProperty);
    private static void Changed(DependencyObject target, DependencyPropertyChangedEventArgs e)
    {
        if (target is not Window window) return;
        if (e.NewValue is true) { window.PreviewKeyDown += KeyDown; window.PreviewMouseDown += MouseDown; }
        else { window.PreviewKeyDown -= KeyDown; window.PreviewMouseDown -= MouseDown; }
    }
    private static void KeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key is Key.Tab or Key.Left or Key.Right or Key.Up or Key.Down or Key.Home or Key.End or Key.PageUp or Key.PageDown)
            SetKeyboardMode((DependencyObject)sender, true);
    }
    private static void MouseDown(object sender, MouseButtonEventArgs e) => SetKeyboardMode((DependencyObject)sender, false);
}
