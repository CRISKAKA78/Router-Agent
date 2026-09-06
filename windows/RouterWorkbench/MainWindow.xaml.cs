using Microsoft.UI.Xaml;
using Microsoft.UI.Windowing;
using Microsoft.Web.WebView2.Core;
using Microsoft.Windows.Storage.Pickers;
using RouterWorkbench.Core;
using System.Text;
using System.Text.Json;
using Windows.Graphics;

namespace RouterWorkbench;
public sealed partial class MainWindow : Window
{
 private readonly CancellationTokenSource lifetime = new();
 private readonly HashSet<Task> active = [];
 private readonly string frontend = Path.Combine(AppContext.BaseDirectory, "Frontend");
 private readonly string profilePath;
 private ServerProfile profile = new();
 private Uri origin;
 private bool closing, closeReady, bridgeBusy;
 private int navigation;
 private PickedFileSave? saving;
 private Task? initialization;
 internal MainWindow(string? configurationPath = null) {
  InitializeComponent(); Title = "远程维护工作台";
  profilePath = configurationPath ?? ServerProfile.DefaultPath;
  try { profile = ServerProfile.Load(profilePath); } catch (Exception e) { StartupMessage.Text = "配置读取失败：" + e.Message; }
  origin = profile.BaseUri(); AppWindow.Resize(new SizeInt32(1536, 1024));
  if (AppWindow.Presenter is OverlappedPresenter presenter) { presenter.PreferredMinimumWidth = 800; presenter.PreferredMinimumHeight = 600; }
  AppWindow.Closing += async (_, e) => {
   if (closeReady) return; e.Cancel = true; if (closing) return;
   closing = true; lifetime.Cancel();
   try {
    if(initialization!=null)await initialization;
    // Runtime.evaluate awaits promises; ExecuteScriptAsync does not await JS teardown.
    if (Web.CoreWebView2 != null) await Web.CoreWebView2.CallDevToolsProtocolMethodAsync("Runtime.evaluate", "{\"expression\":\"window.workbenchShutdown?.()\",\"awaitPromise\":true}");
    await Task.WhenAll(active.ToArray());
    if(saving!=null){await saving.DisposeAsync();saving=null;}
   } catch (Exception e2) { Log(e2); }
   Web.Close(); lifetime.Dispose(); closeReady = true; Close();
  };
  Root.Loaded += async (_, _) => { initialization ??= InitializeAsync(); await initialization; };
 }
 private async Task InitializeAsync() {
  try {
   if (!File.Exists(Path.Combine(frontend, "index.html"))) throw new FileNotFoundException("缺少 Frontend/index.html，请保留完整发布目录。");
   var options = new CoreWebView2EnvironmentOptions { Language = "zh-CN", AreBrowserExtensionsEnabled = false };
#if VERIFY_UI
   options.AdditionalBrowserArguments = "--remote-debugging-port=19222";
#endif
   var bundledRuntime = Path.Combine(AppContext.BaseDirectory, "WebView2Runtime");
   var env = await CoreWebView2Environment.CreateWithOptionsAsync(File.Exists(Path.Combine(bundledRuntime, "msedgewebview2.exe")) ? bundledRuntime : "", Path.Combine(Path.GetDirectoryName(profilePath)!, "WebView2"), options);
   if(closing)return;
   await Web.EnsureCoreWebView2Async(env);
   if(closing)return;
   var core = Web.CoreWebView2;
   core.Settings.AreHostObjectsAllowed = false; core.Settings.AreDevToolsEnabled = false;
   core.Settings.AreDefaultContextMenusEnabled = false; core.Settings.IsPasswordAutosaveEnabled = false;
   core.Settings.IsGeneralAutofillEnabled = false; core.Settings.IsStatusBarEnabled = false;
   core.Settings.IsWebMessageEnabled = true;
   core.AddWebResourceRequestedFilter("*", CoreWebView2WebResourceContext.All, CoreWebView2WebResourceRequestSourceKinds.All);
   core.WebResourceRequested += ResourceRequested;
   core.NavigationStarting += (_, e) => { if (!ShellPolicy.IsDocument(e.Uri, origin) || bridgeBusy || saving != null) e.Cancel = true; else navigation++; };
   core.FrameNavigationStarting += (_, e) => e.Cancel = true;
   core.NewWindowRequested += (_, e) => e.Handled = true;
   core.PermissionRequested += (_, e) => e.State = CoreWebView2PermissionState.Deny;
   core.WebMessageReceived += (sender, e) => {
    if (closing || !ShellPolicy.IsDocument(e.Source, origin)) return;
    var task = HandleMessageAsync(e.WebMessageAsJson, navigation); active.Add(task);
    _ = task.ContinueWith(_ => DispatcherQueue.TryEnqueue(() => active.Remove(task)));
   };
   core.DownloadStarting += (_, e) => e.Cancel = true;
   core.NavigationCompleted += (_, e) => {
    if (e.IsSuccess) Startup.Visibility = Visibility.Collapsed;
    else { Loading.IsActive = false; StartupMessage.Text = "工作台加载失败：" + e.WebErrorStatus; Startup.Visibility = Visibility.Visible; }
   };
   core.ProcessFailed += (_, e) => { Loading.IsActive = false; Startup.Visibility = Visibility.Visible; StartupMessage.Text = "WebView2 进程已退出，请重新启动工作台。"; Log(new Exception(e.ProcessFailedKind.ToString())); };
   core.Navigate(new Uri(origin, "__workbench/index.html").AbsoluteUri);
  } catch (Exception e) { Loading.IsActive = false; StartupMessage.Text = "无法加载工作台。请检查完整发布目录及 Microsoft Edge WebView2 Runtime。\n" + e.Message; Log(e); }
 }
 private void ResourceRequested(CoreWebView2 sender, CoreWebView2WebResourceRequestedEventArgs e) {
  var uri = new Uri(e.Request.Uri); var local = ShellPolicy.LocalResource(uri, origin, frontend);
  if (local != null) {
   if (!File.Exists(local)) { e.Response = Response(sender, 404, "Not Found", "text/plain", Encoding.UTF8.GetBytes("Local resource missing")); return; }
   var mime = Path.GetExtension(local) switch { ".html" => "text/html; charset=utf-8", ".js" => "text/javascript; charset=utf-8", ".css" => "text/css; charset=utf-8", ".svg" => "image/svg+xml", ".png" => "image/png", ".woff2" => "font/woff2", _ => "application/octet-stream" };
   e.Response = Response(sender, 200, "OK", mime, File.ReadAllBytes(local)); return;
  }
  // Only API fetches reach the network; executable content is always local.
  if (ShellPolicy.IsApi(uri, origin) && e.ResourceContext is CoreWebView2WebResourceContext.Fetch or CoreWebView2WebResourceContext.XmlHttpRequest or CoreWebView2WebResourceContext.Websocket) return;
  e.Response = Response(sender, 403, "Forbidden", "text/plain", []);
 }
 private static CoreWebView2WebResourceResponse Response(CoreWebView2 core, int status, string reason, string mime, byte[] bytes) => core.Environment.CreateWebResourceResponse(new MemoryStream(bytes).AsRandomAccessStream(), status, reason,
  "Content-Type: " + mime + "\r\nCache-Control: no-store\r\nX-Content-Type-Options: nosniff\r\nContent-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' ws: wss:; frame-src 'none'; worker-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'\r\n");
 private async Task HandleMessageAsync(string json, int epoch) {
  string id = "";
  try {
   using (var envelope = JsonDocument.Parse(json)) {
    if (envelope.RootElement.TryGetProperty("id", out var identity) && identity.ValueKind == JsonValueKind.String && identity.GetString() is { Length: > 0 and <= 64 } candidate && candidate.All(char.IsAsciiDigit)) id = candidate;
   }
   var message = ShellPolicy.Parse(json); id = message.Id;
   if (bridgeBusy) throw new InvalidOperationException("平台操作正在进行。");
   bridgeBusy = true; object? result = null; Uri? navigate = null;
   try {
    switch (message.Method) {
     case "getProfile": result = profile; break;
     case "saveProfile":
      var next = message.Args.Deserialize<ServerProfile>(Wire.Json)!; ShellPolicy.ValidateProfile(next, profile);
      await next.SaveAsync(profilePath); profile = next;
      Root.RequestedTheme = profile.Theme == "Dark" ? ElementTheme.Dark : profile.Theme == "Light" ? ElementTheme.Light : ElementTheme.Default; break;
     case "connect":
      var url = message.Args.GetProperty("server_url").GetString()!;
      var requested = (profile with { ServerUrl = url }).BaseUri();
      if (requested != profile.BaseUri()) throw new ArgumentException("先保存连接配置。");
      if (requested != origin) navigate = requested; break;
     case "beginSave":
      if(saving!=null)throw new InvalidOperationException("已有保存正在进行。");
      var name=message.Args.GetProperty("name").GetString()!;
      var size=message.Args.GetProperty("size").GetInt64();var sha=message.Args.GetProperty("sha256").GetString()!;
      if(string.IsNullOrWhiteSpace(name)||name.Length>255||name!=Path.GetFileName(name)||name.IndexOfAny(Path.GetInvalidFileNameChars())>=0||size is < 0 or > 1073741824||sha.Length!=64||sha.Any(c=>!char.IsAsciiHexDigit(c)))throw new ArgumentException("无效保存参数。");
      var savePicker=new FileSavePicker(AppWindow.Id){Title="另存文件",SuggestedFileName=name};
      savePicker.FileTypeChoices.Add("文件",new List<string>{Path.GetExtension(name) is {Length:>1} extension?extension:".bin"});
      var target=await savePicker.PickSaveFileAsync().AsTask(lifetime.Token);
#if VERIFY_UI
      await File.WriteAllTextAsync(Path.Combine(Path.GetDirectoryName(profilePath)!, "save-target.txt"), target?.Path ?? "cancelled");
#endif
      if(target!=null){saving=new PickedFileSave(target.Path,size,sha);result=saving.Id;}
      break;
     case "saveChunk": case "finishSave": case "cancelSave":
      if(saving==null||message.Args.GetProperty("handle").GetString()!=saving.Id)throw new ArgumentException("保存句柄无效。");
      if(message.Method=="saveChunk")await saving.WriteAsync(message.Args.GetProperty("offset").GetInt64(),message.Args.GetProperty("base64").GetString()!,lifetime.Token);
      else {try {if(message.Method=="finishSave")await saving.CompleteAsync(lifetime.Token);}finally{await saving.DisposeAsync();saving=null;}}
      break;
     case "openEndpoint":
      var endpoint = message.Args.Deserialize<Endpoint>(Wire.Json)!; ShellPolicy.ValidateEndpoint(endpoint);
#if VERIFY_UI
      await File.AppendAllTextAsync(Path.Combine(Path.GetDirectoryName(profilePath)!, "launches.txt"), endpoint.Service + "\n");
#else
      EndpointLauncher.Open(endpoint, profile);
#endif
      break;
     case "chooseClient":
      var service = message.Args.GetProperty("service").GetString()!;
      if (service is not ("ssh" or "telnet")) throw new ArgumentException("无效客户端类型。");
      var picker = new FileOpenPicker(AppWindow.Id) { Title = "选择 " + service + ".exe 或 putty.exe" }; picker.FileTypeFilter.Add(".exe");
      var file = await picker.PickSingleFileAsync().AsTask(lifetime.Token);
      if (file != null) {
       var putty = string.Equals(Path.GetFileName(file.Path), "putty.exe", StringComparison.OrdinalIgnoreCase);
       if (!putty && !string.Equals(Path.GetFileName(file.Path), service + ".exe", StringComparison.OrdinalIgnoreCase)) throw new ArgumentException("仅支持系统客户端或 PuTTY。");
       profile = service == "ssh" ? profile with { SshExecutable = file.Path, SshUsePutty = putty } : profile with { TelnetExecutable = file.Path, TelnetUsePutty = putty };
       await profile.SaveAsync(profilePath);
      }
      result = profile; break;
    }
   } finally { bridgeBusy = false; }
   if (!closing && epoch == navigation) Web.CoreWebView2.PostWebMessageAsJson(JsonSerializer.Serialize(new { id, result }, Wire.Json));
   if (navigate != null && !closing) { origin = navigate; Web.CoreWebView2.Navigate(new Uri(origin, "__workbench/index.html#connect").AbsoluteUri); }
  } catch (Exception e) {
   if (!closing && epoch == navigation) Web.CoreWebView2.PostWebMessageAsJson(JsonSerializer.Serialize(new { id, error = e is OperationCanceledException ? "操作已取消" : e.Message }, Wire.Json));
  }
 }
 private static void Log(Exception e) { try { Directory.CreateDirectory(Path.GetDirectoryName(ServerProfile.DefaultPath)!); File.WriteAllText(Path.Combine(Path.GetDirectoryName(ServerProfile.DefaultPath)!, "last-error.log"), e.ToString()); } catch (IOException) { } }
}
