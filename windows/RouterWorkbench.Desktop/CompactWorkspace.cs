using System.Windows;
using System.Windows.Controls;
using System.Windows.Automation;

namespace RouterWorkbench.Desktop;

// The same native, keyboard-accessible disclosure for file and maintenance history.
internal static class CompactWorkspace
{
    public static Expander History(string title, UIElement content)
    {
        var expander = new Expander { Header = title, Content = content, IsExpanded = false };
        expander.SetResourceReference(FrameworkElement.StyleProperty, "CompactHistory");
        expander.SetBinding(AutomationProperties.NameProperty,new System.Windows.Data.Binding("Header") {Source=expander});
        return expander;
    }

    public static DockPanel WithHistory(UIElement main, Expander history)
    {
        var panel = new DockPanel();
        DockPanel.SetDock(history, Dock.Bottom); panel.Children.Add(history); panel.Children.Add(main);
        panel.SizeChanged += (_, _) => {
            if (history.Content is FrameworkElement content)
                content.Height = Math.Max(110, Math.Min(280, panel.ActualHeight * .45));
        };
        return panel;
    }

    public static Button Action(string title, string icon, Action action, bool primary = false)
    {
        var button = Ui.Button(title, action, primary); ControlChrome.SetIcon(button, icon);
        AutomationProperties.SetName(button, title); return button;
    }

    public static Border Toolbar(params UIElement[] children)
    {
        var bar=Ui.Bar(children);bar.Padding=new(16,8,12,8);return bar;
    }
}
