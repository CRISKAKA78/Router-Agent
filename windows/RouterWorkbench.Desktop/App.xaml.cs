using System.Windows;

namespace RouterWorkbench.Desktop;
public partial class App : Application
{
    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);
        Theme.Apply("Default");
        MainWindow = new MainWindow(); MainWindow.Show();
    }
}
