using System.Windows;
using System.Windows.Media;
using RouterWorkbench.Core;

namespace RouterWorkbench.Desktop;

public static class Typography
{
    public static void Apply(string family, double size)
    {
        if (!double.IsFinite(size) || size is < 10 or > 24) size = ServerProfile.DefaultUiFontSize;
        var font = Fonts.SystemFontFamilies.FirstOrDefault(f => string.Equals(f.Source, family, StringComparison.OrdinalIgnoreCase))
            ?? new FontFamily(ServerProfile.DefaultUiFontFamily);
        var resources = Application.Current.Resources;
        resources["UiFontFamily"] = font;
        resources["UiFontSize"] = size;
        resources["UiControlFontSize"] = size + 1;
        resources["UiControlHeight"] = Math.Max(32, size + 18);
        resources["UiSmallFontSize"] = size - 1;
        resources["UiTitleFontSize"] = size + 2;
        resources["UiNavigationFontSize"] = size + 1;
        resources["UiInspectorFontSize"] = size + 2;
        resources["UiCodeLineHeight"] = (size + 2) * 1.6;
        resources["UiInspectorTitleFontSize"] = size + 5;
        resources["UiWorkspaceFontSize"] = size + 3;
        resources["UiSectionFontSize"] = size;
        resources["UiPlotFontSize"] = size - 2;
    }
}
