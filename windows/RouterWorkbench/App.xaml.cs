using Microsoft.UI.Xaml;

namespace RouterWorkbench;

public partial class App : Application
{
    private Window? window;
    public App()
    {
        InitializeComponent();
        UnhandledException += (_, e) => {
            // Unexpected UI failures need a local diagnostic even when the
            // window could not be created. Expected API errors stay in InfoBar.
            try {
                var directory = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "RouterWorkbench");
                Directory.CreateDirectory(directory);
                File.WriteAllText(Path.Combine(directory, "last-error.log"), DateTimeOffset.Now + "\n" + e.Exception);
            }
            catch (IOException) { }
            catch (UnauthorizedAccessException) { }
        };
    }
    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
#if VERIFY_UI
        var arguments = Environment.GetCommandLineArgs();
        UnhandledException += (_, e) => { File.WriteAllText(Path.Combine(arguments[2], "ui-failure.txt"), e.Exception.ToString()); Environment.Exit(1); };
        var verification = new MainWindow(Path.Combine(arguments[2], "profile.json"), MainWindow.CaptureLaunch);
        window = verification;
        verification.Activate();
        verification.DispatcherQueue.TryEnqueue(async () => await verification.VerifyAsync(arguments[1], arguments[2]));
#else
        window = new MainWindow();
        window.Activate();
#endif
    }
}
