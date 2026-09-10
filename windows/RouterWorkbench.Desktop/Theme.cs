using System.Windows;
using System.Windows.Media;
using Microsoft.Win32;
using System.Runtime.InteropServices;
using System.Windows.Interop;

namespace RouterWorkbench.Desktop;

public static class Theme
{
    public static bool IsDark { get; private set; }
    public static void Apply(string choice)
    {
        var systemDark = (int?)Registry.GetValue(@"HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize", "AppsUseLightTheme", 1) == 0;
        IsDark = choice == "Dark" || (choice == "Default" && systemDark);
        string[] colors = IsDark
            ? ["#1E1F22", "#25262A", "#2B2D31", "#35373D", "#DCDDDF", "#A7ABB4", "#34363C", "#383E48", "#263B53", "#60A5FA", "#102036", "#D78787", "#7DBB98"]
            : ["#FFFFFF", "#F4F7FA", "#F4F7FB", "#D7E1EC", "#223650", "#687D96", "#E3EAF2", "#EEF4FB", "#E2F0FF", "#006FE6", "#FFFFFF", "#B43C43", "#149653"];
        string[] names = ["Surface", "Panel", "Chrome", "InputBorder", "Text", "Muted", "Line", "Hover", "Selected", "Accent", "AccentText", "Error", "Online"];
        for (var i = 0; i < names.Length; i++) {
            var brush = new SolidColorBrush((Color)ColorConverter.ConvertFromString(colors[i])); brush.Freeze(); Application.Current.Resources[names[i]] = brush;
        }
        Application.Current.Resources[SystemColors.WindowBrushKey] = Application.Current.Resources["Surface"];
        Application.Current.Resources[SystemColors.WindowTextBrushKey] = Application.Current.Resources["Text"];
        Application.Current.Resources[SystemColors.ControlBrushKey] = Application.Current.Resources["Panel"];
        Application.Current.Resources[SystemColors.ControlTextBrushKey] = Application.Current.Resources["Text"];
        Application.Current.Resources[SystemColors.HighlightBrushKey] = Application.Current.Resources["Selected"];
        Application.Current.Resources[SystemColors.HighlightTextBrushKey] = Application.Current.Resources["Text"];
        foreach (Window window in Application.Current.Windows) ApplyCaption(window);
    }
    public static void ApplyCaption(Window window) {
        var handle = new WindowInteropHelper(window).Handle; if (handle == 0) return;
        var dark = IsDark ? 1 : 0; _ = DwmSetWindowAttribute(handle, 20, ref dark, sizeof(int));
    }
    [DllImport("dwmapi.dll")] private static extern int DwmSetWindowAttribute(nint hwnd, int attribute, ref int value, int size);
}
