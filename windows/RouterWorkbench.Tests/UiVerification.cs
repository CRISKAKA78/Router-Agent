using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
using System.Runtime.InteropServices.WindowsRuntime;
using System.Text.Json;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation.Peers;
using Microsoft.UI.Xaml.Automation.Provider;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Media.Imaging;
using RouterWorkbench.Core;
using RouterWorkbench.Tests;
using Windows.Graphics;
using Windows.Graphics.Imaging;
using Windows.Storage;

namespace RouterWorkbench;

// Compiled only with VerifyUI=true. Tests run on the real WinUI dispatcher and
// invoke native controls through their automation peers. No test hook ships.
public sealed partial class MainWindow
{
    private static readonly List<string> launches = [];
    private int assertions;
    private string verificationOutput = "";
    [DllImport("user32.dll", EntryPoint = "PostMessageW")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool PostMessage(nint window, uint message, nuint wParam, nint lParam);
    private delegate bool EnumWindow(nint window, nint parameter);
    [DllImport("user32.dll")] private static extern bool EnumWindows(EnumWindow callback, nint parameter);
    [DllImport("user32.dll")] private static extern bool EnumChildWindows(nint window, EnumWindow callback, nint parameter);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] private static extern int GetClassName(nint window, StringBuilder text, int length);
    [DllImport("user32.dll")] private static extern uint GetWindowThreadProcessId(nint window, out uint process);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] private static extern int GetWindowText(nint window, StringBuilder text, int length);
    [DllImport("user32.dll")] private static extern nint GetDlgItem(nint window, int id);
    [DllImport("user32.dll")] private static extern bool SetForegroundWindow(nint window);
    [DllImport("user32.dll")] private static extern bool IsWindowVisible(nint window);
    [DllImport("user32.dll")] private static extern bool IsWindowEnabled(nint window);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] private static extern nint SendMessage(nint window, uint message, nuint parameter, string text);
    private async Task PickFileAsync(string title, string path)
    {
        nint dialog = 0;
        await UntilAsync(() => {
            EnumWindows((window, _) => {
                GetWindowThreadProcessId(window, out var process);
                var text = new StringBuilder(256); GetWindowText(window, text, text.Capacity);
                if (process == Environment.ProcessId && text.ToString() == title) dialog = window;
                return true;
            }, 0);
            return dialog != 0 && IsWindowVisible(dialog) && IsWindowEnabled(GetDlgItem(dialog, 1));
        }, "native file picker opens: " + title);
        await Task.Delay(300);
        var filename = GetDlgItem(dialog, 1148);
        if (filename == 0) EnumChildWindows(dialog, (child, _) => {
            var name = new StringBuilder(128); var value = new StringBuilder(512); GetClassName(child, name, name.Capacity); GetWindowText(child, value, value.Capacity);
            if (name.ToString() == "Edit" && value.ToString() == SelectedAsset?.Name) filename = child;
            return true;
        }, 0);
        Check(filename != 0, "file picker filename control");
        EnumChildWindows(filename, (child, _) => { var name = new StringBuilder(128); GetClassName(child, name, name.Capacity); if (name.ToString() == "Edit") filename = child; return true; }, 0);
        SendMessage(filename, 0x000C, 0, Path.GetFullPath(path));
        await Task.Delay(80);
        SetForegroundWindow(dialog);
        PostMessage(GetDlgItem(dialog, 1), 0x00F5, 0, 0);
        await UntilAsync(() => !Model.Busy, "file picker completes: " + title);
    }
    internal static void CaptureLaunch(Endpoint endpoint, ServerProfile _) => launches.Add(endpoint.Service);
    private void Check(bool value, string label)
    {
        if (!value) throw new InvalidOperationException("FAIL: " + label);
        assertions++; File.AppendAllText(Path.Combine(verificationOutput, "ui.log"), "PASS " + label + "\n");
    }
    private async Task UntilAsync(Func<bool> condition, string label, int seconds = 15)
    {
        var watch = Stopwatch.StartNew();
        while (!condition()) { if (watch.Elapsed > TimeSpan.FromSeconds(seconds)) throw new TimeoutException(label + ": " + Model.Notice); await Task.Delay(25); }
        Check(true, label);
    }
    private async Task ClickAsync(Button button)
    {
        await Task.Delay(430);
        Check(button.IsEnabled, "enabled: " + button.Content);
        ((IInvokeProvider)new ButtonAutomationPeer(button).GetPattern(PatternInterface.Invoke)).Invoke();
        await Task.Yield();
    }
    private void SecondClick(Button button)
    {
        try { ((IInvokeProvider)new ButtonAutomationPeer(button).GetPattern(PatternInterface.Invoke)).Invoke(); }
        catch (System.Runtime.InteropServices.COMException e) when (e.HResult == unchecked((int)0x80040200)) { Check(!button.IsEnabled, "WinUI rejects disabled second click"); }
    }
    private static IEnumerable<T> Descendants<T>(DependencyObject parent) where T : DependencyObject
    {
        for (var i = 0; i < VisualTreeHelper.GetChildrenCount(parent); i++) {
            var child = VisualTreeHelper.GetChild(parent, i); if (child is T value) yield return value;
            foreach (var nested in Descendants<T>(child)) yield return nested;
        }
    }
    private async Task AcceptDialogAsync(Dictionary<string, string>? fields = null, bool secondary = false)
    {
        await UntilAsync(() => activeDialog != null, "native ContentDialog opens");
        await UntilAsync(() => Descendants<Button>(activeDialog!).Any(b => b.Content as string == (secondary ? activeDialog!.SecondaryButtonText : activeDialog!.PrimaryButtonText)), "dialog template loaded");
        if (fields != null) foreach (var box in Descendants<TextBox>(activeDialog!))
            if (box.Header is string header && fields.TryGetValue(header, out var text)) box.Text = text;
        var content = secondary ? activeDialog!.SecondaryButtonText : activeDialog!.PrimaryButtonText;
        var button = Descendants<Button>(activeDialog!).First(b => b.Content as string == content);
        ((IInvokeProvider)new ButtonAutomationPeer(button).GetPattern(PatternInterface.Invoke)).Invoke();
        await UntilAsync(() => activeDialog == null && !Model.Busy, "dialog operation settles");
    }
    private async Task ScreenshotAsync(string name, int pixelWidth = 0)
    {
        // RenderTargetBitmap omits compositor-owned Mica. Use the same solid
        // theme fallback for the screenshot, then restore the live backdrop.
        var background = Root.Background;
        Root.Background = new SolidColorBrush(Root.ActualTheme == ElementTheme.Dark ? Windows.UI.Color.FromArgb(255, 32, 32, 32) : Windows.UI.Color.FromArgb(255, 243, 243, 243));
        Root.UpdateLayout(); await Task.Delay(180);
        var target = new RenderTargetBitmap();
        if (pixelWidth == 0) await target.RenderAsync(Root);
        else await target.RenderAsync(Root, pixelWidth, (int)(Root.ActualHeight * pixelWidth / Root.ActualWidth));
        var pixels = await target.GetPixelsAsync();
        var file = await StorageFile.GetFileFromPathAsync(CreateFile(Path.GetFullPath(Path.Combine(verificationOutput, name))));
        using var stream = await file.OpenAsync(FileAccessMode.ReadWrite);
        var encoder = await BitmapEncoder.CreateAsync(BitmapEncoder.PngEncoderId, stream);
        encoder.SetPixelData(BitmapPixelFormat.Bgra8, BitmapAlphaMode.Premultiplied, (uint)target.PixelWidth, (uint)target.PixelHeight, 96, 96, pixels.ToArray());
        await encoder.FlushAsync();
        Root.Background = background;
        Check(target.PixelWidth > 0 && pixels.Length > 0, $"render {name}: {target.PixelWidth}x{target.PixelHeight}, display scale {Root.XamlRoot.RasterizationScale}");
    }
    private static string CreateFile(string path) { File.WriteAllBytes(path, []); return path; }
    internal async Task VerifyAsync(string serverPath, string outputPath)
    {
        verificationOutput = Path.GetFullPath(outputPath); Directory.CreateDirectory(verificationOutput);
        Process? server = null;
        var keeper = new Window { Content = new Grid() }; keeper.Activate(); keeper.AppWindow.Hide();
        try
        {
            await UntilAsync(() => Root.XamlRoot != null && Root.ActualWidth > 0, "WinUI window loaded");
            await ScreenshotAsync("disconnected.png");
            var port = MockEvents.FreePort(); var controlPort = MockEvents.FreePort();
            var info = new ProcessStartInfo(Path.GetFullPath(serverPath)) { UseShellExecute = false, CreateNoWindow = true };
            foreach (var arg in new[] { "-listen", $"127.0.0.1:{controlPort}", "-http-listen", $"127.0.0.1:{port}", "-tunnel-data-listen", "127.0.0.1:0", "-tunnel-port-first", "32100", "-tunnel-port-last", "32129", "-tunnel-port-reuse-delay", "1ms", "-repository-dir", Path.Combine(verificationOutput, "repository-" + Guid.NewGuid().ToString("N")) }) info.ArgumentList.Add(arg);
            server = Process.Start(info)!;
            using var api = new ApiClient(new Uri($"http://127.0.0.1:{port}"));
            for (var i = 0; ; i++) { try { await api.ListAsync<Device>("devices", default); break; } catch (HttpRequestException) when (i < 50) { await Task.Delay(50); } }
            await using var probe = new TestProbe(); await probe.StartAsync(controlPort, "售后设备-001");
            await ClickAsync(Descendants<Button>(Root).First(b => Descendants<TextBlock>(b).Any(t => t.Text == "设置")));
            await AcceptDialogAsync(new() { ["管理服务器地址"] = api.BaseUri.AbsoluteUri });
            await UntilAsync(() => Model.Synchronized && DeviceList.Items.Count == 1, "settings connect and device list");
            Check(DeviceTitle.Text == "售后测试路由器" && DeviceSubtitle.Text.Contains("在线"), "device overview Chinese");
            Check(!Descendants<TextBlock>(OverviewPage).Any(t => t.Text.Contains(probe.SessionId)), "session ID absent from main surface");
            await ClickAsync(StartMaintenance);
            SecondClick(StartMaintenance);
            await UntilAsync(() => Model.Maintenance is { State: "ready" } && WebButton.IsEnabled, "maintenance ready via HTTP snapshot");
            Check(Model.Maintenance!.ExpiresAt - Model.Maintenance.CreatedAt == TimeSpan.FromMinutes(240), "default 240 minutes");
            Check(Model.Snapshot!.Maintenance.Length == 1, "double click creates one maintenance");
            foreach (var button in new[] { WebButton, SshButton, TelnetButton }) await ClickAsync(button);
            await UntilAsync(() => launches.Count == 3, "Web SSH Telnet launch boundaries after GET");
            ApplyTheme("Light"); await ScreenshotAsync("overview-light.png"); Check(Root.ActualTheme == ElementTheme.Light, "light theme applied");
            ApplyTheme("Dark"); await ScreenshotAsync("overview-dark.png"); Check(Root.ActualTheme == ElementTheme.Dark, "dark theme applied");
            AppWindow.Resize(new SizeInt32(960, 720)); await Task.Delay(150);
            await ScreenshotAsync("overview-compact.png");
            Check(StartMaintenance.ActualWidth > 0 && PageScroll.ScrollableHeight > 0 && SidebarColumn.Width.Value == 224, "compact window uses scrolling and narrower sidebar");
            await ScreenshotAsync("overview-200-percent-render.png", (int)(Root.ActualWidth * 2));
            await VerifyHighDpiAsync();
            AppWindow.Resize(new SizeInt32(1280, 920)); ApplyTheme("Light");
            await ClickAsync(StopMaintenance); await UntilAsync(() => Model.Maintenance?.Released == true, "maintenance close visible");
            await ClickAsync(LeaseButton); await UntilAsync(() => activeDialog != null, "lease dialog");
            Descendants<CheckBox>(activeDialog!).Single().IsChecked = false;
            await AcceptDialogAsync(new() { ["自定义时长（分钟，支持小数）"] = "0.00001" });
            Check(NoticeBar.IsOpen && Model.IsError && Model.Notice.Contains("租期"), "invalid lease reported in InfoBar");
            await ClickAsync(LeaseButton); await UntilAsync(() => activeDialog != null, "custom lease dialog");
            Descendants<CheckBox>(activeDialog!).Single().IsChecked = false;
            await AcceptDialogAsync(new() { ["自定义时长（分钟，支持小数）"] = "0.005" });
            await ClickAsync(StartMaintenance);
            await UntilAsync(() => Model.Snapshot!.Maintenance.Length == 2 && Model.Maintenance?.Released == true, "custom lease expiry refreshed from server");
            Check(Model.Maintenance!.ExpiresAt - Model.Maintenance.CreatedAt == TimeSpan.FromMilliseconds(300), "custom minute conversion exact");
            Sections.SelectedItem = Sections.MenuItems[1];
            await ClickAsync(ExecButton); SecondClick(ExecButton);
            await UntilAsync(() => TaskOutput.Text.Contains("中文结果"), "exec output rendered"); Check(probe.Executions == 1, "exec double click sends once");
            await ScreenshotAsync("tasks-light.png");
            var source = Path.Combine(verificationOutput, "维护脚本.sh"); await File.WriteAllTextAsync(source, "echo phase6\n");
            Sections.SelectedItem = Sections.MenuItems[2]; await ClickAsync(ImportButton); await PickFileAsync("导入文件", source);
            await UntilAsync(() => AssetList.Items.Count == 1, "file notification refreshes list"); Sections.SelectedItem = Sections.MenuItems[2];
            var savedFile = Path.Combine(verificationOutput, "saved-script.sh"); await ClickAsync(SaveButton); await PickFileAsync("另存文件", savedFile);
            Check(File.ReadAllBytes(savedFile).SequenceEqual(File.ReadAllBytes(source)), "native picker save round trip");
            await ClickAsync(UploadButton); await AcceptDialogAsync();
            await UntilAsync(() => Model.Snapshot!.Tasks.Any(t => t.Type == "upload" && t.State == "success"), "file upload from dialog");
            await ClickAsync(DownloadButton); await AcceptDialogAsync();
            await UntilAsync(() => Model.Snapshot!.Tasks.Any(t => t.Type == "download" && t.State == "success"), "file download from dialog");
            await ScreenshotAsync("files-light.png");
            Sections.SelectedItem = Sections.MenuItems[3]; await ClickAsync(CreateToolButton);
            await AcceptDialogAsync(new() { ["工具名称"] = "网络诊断", ["用途说明"] = "采集基础网络信息" });
            await UntilAsync(() => ToolList.Items.Count == 1 && PublishButton.IsEnabled, "tool created and selected");
            await ClickAsync(PublishButton); await AcceptDialogAsync(new() { ["版本名称"] = "1.0.0" });
            await UntilAsync(() => VersionList.Items.Count == 1 && DeployButton.IsEnabled, "version and server compatibility displayed");
            await ClickAsync(DeployButton); await AcceptDialogAsync();
            await UntilAsync(() => Model.Snapshot!.Tasks.Count(t => t.Type == "upload" && t.State == "success") == 2, "tool deployment completes");
            await ScreenshotAsync("tools-light.png");
            var before = Model.Device!.CurrentSession!.SessionId;
            await using var replacement = new TestProbe(); await replacement.StartAsync(controlPort, Model.DeviceId!);
            await UntilAsync(() => Model.Device!.CurrentSession!.SessionId != before && Model.Online, "session replacement reflected without offline guess");
            await api.ExecuteAsync(Mutation.Json("断开", $"devices/{Wire.Segment(Model.DeviceId!)}/disconnect", new { }), default);
            await UntilAsync(() => DeviceSubtitle.Text.Contains("离线") && !StartMaintenance.IsEnabled, "offline notification disables maintenance");
            await using var mock = new MockEvents(); var old = Model.Connection;
            await Task.Delay(450); await Model.RunAsync("切换服务器", () => Model.ConnectAsync(new() { ServerUrl = mock.Uri.AbsoluteUri }));
            await UntilAsync(() => Model.Synchronized && DeviceList.Items.Count == 1 && Model.DeviceId == "old", "server switched to fresh snapshot");
            Check(!old!.IsSynchronized && TaskOutput.Text == "" && Model.Maintenance == null && ToolList.Items.Count == 0, "old connection disposed and cross-server data cleared");
            mock.DeviceId = "恢复后的设备"; mock.Socket!.Abort();
            await UntilAsync(() => mock.Connections >= 2 && Model.DeviceId == "恢复后的设备", "WebSocket reconnect restores full HTTP snapshot");
            await ClickAsync(ConnectButton); await UntilAsync(() => DeviceList.Items.Count == 0, "disconnect clears UI");
            profile = new() { ServerUrl = mock.Uri.AbsoluteUri }; await ClickAsync(ConnectButton);
            await UntilAsync(() => Model.Synchronized, "reconnect from UI");
            await Task.Delay(450); await Model.RunAsync("业务错误", async () => {
                throw new ApiException("capacity_exhausted", "fixture", 503);
#pragma warning disable CS0162
                await Task.CompletedTask;
#pragma warning restore CS0162
            });
            Check(Model.IsError && NoticeBar.IsOpen && Model.Notice.Contains("容量不足"), "business error InfoBar");
            var last = Model.Connection; PostMessage(WinRT.Interop.WindowNative.GetWindowHandle(this), 0x10, 0, 0);
            await UntilAsync(() => closeReady, "window awaits resource shutdown"); Check(!last!.IsSynchronized, "shutdown releases HTTP and WebSocket");
            File.WriteAllText(Path.Combine(verificationOutput, "ui-result.txt"), $"PASS {assertions} WinUI assertions");
        }
        catch (Exception e) { File.WriteAllText(Path.Combine(verificationOutput, "ui-failure.txt"), e.ToString()); Environment.ExitCode = 1; activeDialog?.Hide(); await Model.DisposeAsync(); closeReady = true; Close(); }
        finally { if (server != null) { if (!server.HasExited) { server.Kill(true); await server.WaitForExitAsync(); } server.Dispose(); } keeper.Close(); }
    }
    private async Task VerifyHighDpiAsync()
    {
        // Exercise the actual XAML tree at 192 DPI without modifying the user's
        // monitor settings. The public XAML island host supplies the DPI scale.
        var host = new Window(); host.Activate();
        using var source = new Microsoft.UI.Xaml.Hosting.DesktopWindowXamlSource();
        source.Initialize(host.AppWindow.Id);
        Content = null;
        try {
            source.SiteBridge.OverrideScale = 2;
            source.SiteBridge.MoveAndResize(new RectInt32(0, 0, 1920, 1440));
            source.Content = Root; source.SiteBridge.Show();
            await UntilAsync(() => Root.XamlRoot.RasterizationScale == 2, "native XAML rasterization at 200 percent");
            await ScreenshotAsync("overview-native-200-percent.png");
            Check(WebButton.ActualWidth > 0 && StartMaintenance.ActualWidth > 0 && Root.ActualWidth >= 940, "high DPI layout retains maintenance controls");
        }
        finally { source.Content = null; Content = Root; host.Close(); }
        await UntilAsync(() => Root.XamlRoot.RasterizationScale == 1, "XAML returns to original display scale");
    }
}
