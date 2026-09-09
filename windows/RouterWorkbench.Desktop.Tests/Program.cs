using System.Diagnostics;
using System.IO;
using System.Net;
using System.Net.Sockets;
using System.Reflection;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using RouterWorkbench.Client;
using RouterWorkbench.Core;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;

internal static partial class Program
{
    private static int checks;
    private static string output = "";
    private static void Check(bool success, string description) { if (!success) throw new Exception(description); Console.WriteLine("PASS " + description); checks++; }
    private static async Task Eventually(Func<Task<bool>> condition, string description, int milliseconds = 10000) {
        var deadline = DateTime.UtcNow.AddMilliseconds(milliseconds);
        while (DateTime.UtcNow < deadline) { if (await condition()) { Check(true, description); return; } await Task.Delay(70); }
        throw new Exception("Timeout: " + description);
    }
    [STAThread] private static int Main(string[] args)
    {
        RenderOptions.ProcessRenderMode = System.Windows.Interop.RenderMode.SoftwareOnly;
        output = Path.GetFullPath(args.Length > 1 ? args[1] : "build/windows-desktop/verification"); Directory.CreateDirectory(output);
        var app = new Application { ShutdownMode = ShutdownMode.OnExplicitShutdown };
        app.Resources.MergedDictionaries.Add(new ResourceDictionary { Source = new Uri("pack://application:,,,/RouterWorkbench;component/Themes/Controls.xaml") });
        Theme.Apply("Light"); var exit = 1;
        app.Dispatcher.BeginInvoke(async () => {
            try {
                await TelemetryChecks(); await ClientChecks(); await FileExchangeRetryChecks(); await IntegrationChecks(args[0]);
                Console.WriteLine($"PASS {checks} checks; screenshots: {output}"); exit = 0;
            } catch (Exception e) { Console.Error.WriteLine(e); }
            finally { app.Shutdown(); }
        });
        app.Run(); return exit;
    }
    private static async Task ClientChecks()
    {
        foreach (var (seconds, expected) in new (long?, string)[] {
            (null, "—"), (-1, "—"), (0, "0秒"), (59, "59秒"), (60, "1分钟0秒"),
            (3599, "59分钟59秒"), (3600, "1小时0分钟0秒"), (86400, "1天0小时0分钟0秒"),
            (30L*86400-1, "29天23小时59分钟59秒"), (30L*86400, "1月0天0小时0分钟0秒"),
            (365L*86400, "1年0月0天0小时0分钟0秒"),
            ((365L+30+2)*86400+3*3600+4*60+5, "1年1月2天3小时4分钟5秒") })
            Check(DeviceProperties.FormatUptime(seconds) == expected, $"uptime format {seconds}: {expected}");
        Check(DeviceProperties.FormatUptime(long.MaxValue).EndsWith("秒"), "uptime formatting handles API int64 without TimeSpan overflow");
        Check(RemoteDirectory.Command("/tmp/a'b\n目录").Contains("'\\''"), "directory shell quoting retains quotes and newlines");
        var parsed = RemoteDirectory.Parse("f\0" + "12\0" + "1700000000\0" + "a\nb\0");
        Check(parsed.Entries[0].Name == "a\nb" && parsed.Entries[0].Size == 12, "NUL directory records preserve embedded newline");
        Check(RemoteDirectory.Parse("LIMIT\0").Limited, "directory cap reported");
        foreach (var bad in new[] { "f\0", "f\0x\00\0name\0", "f\01\00\0../bad\0", "f\01\00\0name" }) {
            try { RemoteDirectory.Parse(bad); throw new Exception("accepted malformed directory"); } catch (InvalidDataException) { Check(true, "reject malformed directory"); }
        }
        Check(Labels.State("success") == "成功", "wire success label");
        var attempts = new List<(string Key, byte[] Bytes)>();
        var count = 0;
        using var handler = new DelegateHandler(async (request, token) => {
            attempts.Add((request.Headers.GetValues("Idempotency-Key").Single(), await request.Content!.ReadAsByteArrayAsync(token)));
            if (++count == 1) throw new HttpRequestException("lost response");
            return Json("{\"data\":{\"task_id\":\"original\",\"dispatch_uncertain\":true}}");
        });
        await using (var connection = new WorkspaceConnection(new Uri("http://localhost:18080"), handler)) {
            var mutation = new Mutation("write", "tasks", new { command = "printf '中文'", device_id = "a/b" });
            try { await connection.ExecuteAsync(mutation); throw new Exception("failure missing"); } catch (HttpRequestException) { }
            Check(connection.Pending == mutation && !connection.Busy, "lost response retains original mutation");
            try { await connection.ExecuteAsync(new("new", "tasks")); throw new Exception("new write allowed"); } catch (InvalidOperationException) { Check(true, "uncertain request blocks new mutation"); }
            var result = await connection.ExecuteAsync(mutation, true);
            Check(result.GetProperty("task_id").GetString() == "original" && connection.Pending == null, "explicit retry resolves original task");
            Check(attempts[0].Key == attempts[1].Key && attempts[0].Bytes.SequenceEqual(attempts[1].Bytes), "retry reuses exact key and UTF-8 payload");
        }
        var importPath = Path.Combine(output, "import.bin"); await File.WriteAllBytesAsync(importPath, "desktop-中文-content"u8.ToArray());
        using (var import = await Mutation.ImportAsync(importPath, default)) {
            try { using var write = new FileStream(importPath, FileMode.Open, FileAccess.Write); throw new Exception("mutable input"); } catch (IOException) { Check(true, "raw import holds immutable selected file"); }
            Check(import.Hash == Convert.ToHexStringLower(SHA256.HashData(await File.ReadAllBytesAsync(importPath))), "raw import SHA-256");
        }
        var saved = Path.Combine(output, "preserve.txt"); await File.WriteAllTextAsync(saved, "original");
        using (var bad = new ApiClient(new Uri("http://localhost"), default, new DelegateHandler((_,_) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK) { Content = new ByteArrayContent("bad"u8.ToArray()) })))) {
            try { await bad.SaveContentAsync(new("asset", "file", 3, new string('0',64), false, DateTimeOffset.Now), saved); throw new Exception("bad hash accepted"); }
            catch (InvalidDataException) { Check(await File.ReadAllTextAsync(saved) == "original" && !Directory.GetFiles(output, "preserve.txt.*.tmp").Any(), "failed content validation preserves target and cleans own staging"); }
        }
        var started = new TaskCompletionSource(); var canceled = false;
        var slow = new WorkspaceConnection(new Uri("http://localhost"), new DelegateHandler(async (_, token) => { started.TrySetResult(); try { await Task.Delay(30000, token); } catch (OperationCanceledException) { canceled = true; throw; } return Json("{}"); }));
        var pending = slow.TrackAsync(() => slow.Api.GetAsync<JsonElement>("devices")); await started.Task;
        await slow.DisposeAsync(); Check(canceled && pending.IsCompleted, "connection disposal cancels and awaits active request");
        var credentialProfile = Path.Combine(output, "test-credentials-" + Guid.NewGuid().ToString("N") + ".json");
        Check(SshPasswordStore.Load(credentialProfile) == "admin", "default SSH password is admin");
        await SshPasswordStore.SaveAsync(credentialProfile, "test-secret-中文");
        Check(SshPasswordStore.Load(credentialProfile) == "test-secret-中文" && !Encoding.UTF8.GetString(await File.ReadAllBytesAsync(credentialProfile + ".ssh-password")).Contains("test-secret"), "SSH password DPAPI roundtrip without plaintext storage");
        var template = JsonSerializer.Deserialize<TemplateReference>("{\"template_id\":\"a\",\"name\":\"b\",\"version\":18446744073709551615}", ApiJson.Options);
        Check(template!.Version == ulong.MaxValue, "template version accepts protocol uint64");
    }
    private static async Task VerifyRuntime(MainWindow window, WorkspaceConnection connection, TestProbe peer, ApiClient api)
    {
        const string id = "desktop-router-02";
        var grid = Field<DataGrid>(window, "properties");
        var row = grid.Items.Cast<PropertyRow>().Single(p => p.Name == "开机时长");
        grid.SelectedItem = row;
        await peer.ReportUptimeAsync(3661);
        await Eventually(() => Task.FromResult(row.Value == "1小时1分钟1秒"), "periodic HTTP refresh displays measured uptime", 9000);
        Check(ReferenceEquals(grid.SelectedItem, row) && grid.Items.Cast<PropertyRow>().Any(p => ReferenceEquals(p, row)), "uptime updates preserve selected property row");
        var device = await api.GetAsync<Device>("devices/" + id);
        Check(device.Runtime?.UptimeSeconds == 3661 && device.CurrentSession?.Runtime == device.Runtime && device.LatestSession?.Runtime == device.Runtime, "HTTP device and session runtime DTOs agree");
        Check(grid.Items.Cast<PropertyRow>().Any(p => p.Name == "工具兼容架构" && p.Value == "x86_64") && grid.Items.Cast<PropertyRow>().Any(p => p.Name == "内核版本" && p.Value == "6.6-test"), "CPU architecture and kernel visible in native properties");
        var offline = DeviceProperties.Rows(device with { Status = "offline" }).Single(p => p.Name == "开机时长").Value;
        Check(offline == "1小时1分钟1秒" && DeviceProperties.Uptime(device with { Status = "offline" }).ValueTip.Contains("离线"), "offline displays final measured uptime without extrapolation");
        Check(DeviceProperties.Rows(device with { Runtime = null }).Single(p => p.Name == "开机时长").Value == "—", "old API or first heartbeat pending state");
        await peer.ReportUptimeAsync(null);
        connection.Invalidate();
        await Eventually(() => Task.FromResult(row.Value == "—" && row.ValueTip.Contains("无法读取")), "failed collection replaces previous uptime with unknown");
        await peer.ReportUptimeAsync(0);
        connection.Invalidate();
        await Eventually(() => Task.FromResult(row.Value == "0秒"), "valid zero seconds distinguished from failure");
        await peer.ReportUptimeAsync(59);
        connection.Invalidate();
        await Eventually(() => Task.FromResult(row.Value == "59秒"), "under one minute hides all larger units");
    }
    private static int FreePort() { using var listener = new TcpListener(IPAddress.Loopback, 0); listener.Start(); return ((IPEndPoint)listener.LocalEndpoint).Port; }
    private static async Task IntegrationChecks(string serverPath)
    {
        var httpPort = FreePort(); var controlPort = FreePort(); var repository = Path.Combine(output, "repository-" + Guid.NewGuid().ToString("N"));
        var info = new ProcessStartInfo(Path.GetFullPath(serverPath)) { UseShellExecute = false, CreateNoWindow = true, RedirectStandardOutput = true, RedirectStandardError = true };
        foreach (var arg in new[] { "-http-listen", $"127.0.0.1:{httpPort}", "-listen", $"127.0.0.1:{controlPort}", "-repository-dir", repository, "-tunnel-data-listen", "127.0.0.1:0", "-tunnel-port-first", "34200", "-tunnel-port-last", "34259", "-tunnel-port-reuse-delay", "1ms" }) info.ArgumentList.Add(arg);
        using var server = Process.Start(info)!; var serverOut = server.StandardOutput.ReadToEndAsync(); var serverError = server.StandardError.ReadToEndAsync();
        var profile = new ServerProfile { ServerUrl = $"http://127.0.0.1:{httpPort}", Theme = "Light", SshUser = "admin" }; var profileFile = Path.Combine(output, "profile.json"); await profile.SaveAsync(profileFile);
        using var api = new ApiClient(profile.BaseUri(), default); var peers = new List<TestProbe>(); MainWindow? window = null;
        try {
            await Eventually(async () => { try { await api.ListAsync<Device>("devices"); return true; } catch { return false; } }, "isolated current Go server ready");
            for (var i=0; i<8; i++) {
                var peer = new TestProbe(); peers.Add(peer); var id = $"desktop-router-{i+1:00}";
                await peer.StartAsync(controlPort, id, new { device_id = id, hostname = new[] { "杭州 · 核心网关", "上海 · 边缘路由器", "北京 · 实验室网关", "广州 · 接入网关", "南京 · 分支路由器", "成都 · 备份网关", "武汉 · 办公路由器", "深圳 · 配置测试设备" }[i], serial = $"RMP-TEST-{i+1:000}", model = "OpenWrt 测试对端", firmware = "隔离测试固件", arch = "x86_64", libc = "musl", kernel = "6.6-test", boot_id = "desktop-fixture", probe_version = "desktop-fixture",
                    capabilities = i == 7 ? new[] { "exec", "file", "tunnel", "router_config", "telemetry_v2" } : new[] { "exec", "file", "tunnel", "telemetry_v2" } });
                await Eventually(async()=>{try{return (await api.GetAsync<Device>("devices/"+id)).Profile!=null;}catch{return false;}},"fixture discovered before explicit admission");
                var discovered=await api.GetAsync<Device>("devices/"+id);
                var fieldValues=Enumerable.Range(0,i==1?30:1).ToDictionary(n=>"property_"+n,n=>new {name=n==0?"模板固件描述":"扩展属性 "+n,value=n==0?"中文长文本\n"+new string('X',800):"值 "+n});
                var definitions=fieldValues.ToDictionary(p=>p.Key,p=>(object)new{name=p.Value.name,command="true"});definitions["signal"]=new{name="信号强度",command="true"};definitions["wan_ip"]=new{name="WAN地址",command="true"};
                var template=(await api.ExecuteAsync(new("测试模板","probe-templates",new{name="模板 "+id,properties=definitions}))).Deserialize<ProbeTemplate>(ApiJson.Options)!;
                await api.ExecuteAsync(new("纳管测试设备","devices/"+id+"/profile",new {version=discovered.Profile!.Version,admission="managed",name=discovered.DisplayName,model_id="",template_id=template.TemplateId,apply_template=true},"PUT"));
                await Eventually(()=>Task.FromResult(peer.ConfigurationRevision==1),"current template configuration acknowledged");
                var observations=fieldValues.ToDictionary(p=>p.Key,p=>(object)new{name=p.Value.name,value=p.Value.value,unit="text",status="ok"});
                observations["signal"]=new{name="信号强度",value="",unit="text",status="error",reason="timeout"};observations["wan_ip"]=new{name="WAN地址",value="",unit="text",status="error",reason="empty"};
                await peer.ReportAsync("template",observations);

            }
            window = new MainWindow(profileFile); window.Show();
            await Eventually(() => Task.FromResult(Field<WorkspaceConnection?>(window, "connection") != null), "saved server auto connects on window startup");
            var connection = Field<WorkspaceConnection>(window, "connection");
            await Eventually(() => Task.FromResult(connection.Synchronized && connection.Snapshot.Devices.Length == 8), "native window HTTP/WS snapshot with eight test peers");
            await Task.Delay(150);
            var pages = ((TabControl)window.FindName("WorkspaceTabs")).Items.Cast<TabItem>().Select(t => (string)t.Tag).ToArray();
            Check(pages.SequenceEqual(new[] { "overview", "maintenance", "files", "config", "tools" }), "customer navigation exposes file exchange and read-only repository tools");
            Check(window.GetType().Assembly.GetReferencedAssemblies().All(a => !a.Name!.Contains("WebView2")), "native customer executable has no WebView2 dependency");
            Check(connection.Snapshot.Tools.Length == 0, "customer snapshot no longer loads tool management");
            Check(((DataGrid)window.FindName("DevicesGrid")).Items.Count == 8, "native device table bound to server inventory");
            await VerifyConnectionDisplay(window, connection);
            var devices = (DataGrid)window.FindName("DevicesGrid"); devices.SelectedIndex = 1; await Task.Delay(100);
            Check(Field<string>(window, "selectedDevice") == "desktop-router-02", "device selection updates current scope");
            var props = Field<PropertyRow[]>(window, "propertyRows").ToArray();
            Check(props.Count(p => p.Key.StartsWith("property_") && p.Group == "其他信息" && p.Value != "—") == 30 && props.Count(p => p.Key is "signal" or "wan_ip" && p.Value == "—" && p.ValueTip.Contains("采集")) == 2 && props.Any(p => p.Value.Contains(new string('X', 800))), "all template attributes, failures and full multiline values reach native table");
            await VerifyRuntime(window, connection, peers[1], api);
            devices.SelectedItem = devices.Items.Cast<Device>().Single(d => d.DeviceId == "desktop-router-03");
            Check(Field<PropertyRow[]>(window, "propertyRows").Count(p => p.Key.StartsWith("property_") && p.Group == "其他信息" && p.Value != "—") == 1, "switching templates removes previous device attributes");
            devices.SelectedItem = devices.Items.Cast<Device>().Single(d => d.DeviceId == "desktop-router-02");
            ((TextBox)window.FindName("DeviceSearch")).Text = "03"; Check(devices.Items.Count == 1, "native device text filter"); ((TextBox)window.FindName("DeviceSearch")).Clear();
            devices.Items.SortDescriptions.Add(new("DisplayName", System.ComponentModel.ListSortDirection.Descending));
            Invoke(window, "ApplySnapshot"); Check(devices.Items.SortDescriptions.Count == 1 && devices.Items.SortDescriptions[0].Direction == System.ComponentModel.ListSortDirection.Descending, "device sorting survives snapshot refresh");
            var body = new { device_id = "desktop-router-02", command = "uname -a", timeout_seconds = 5 };
            var created = await InvokeResult(window, "Write", new Mutation("测试命令", "tasks", body));
            var taskId = created.GetProperty("task_id").GetString()!;
            await Eventually(async () => (await api.GetAsync<TaskDetail>("tasks/"+taskId)).Result != null, "Exec real API and test peer RESULT");
            connection.Invalidate(); await Task.Delay(400); await InvokeAsync(window, "RefreshDetails");

            var detail = await api.GetAsync<TaskDetail>("tasks/"+taskId); Check(detail.State == "success" && detail.StateText == "成功", "native DTO maps actual task success");
            await api.ExecuteAsync(new("resend", $"tasks/{taskId}/resend")); Check(peers[1].Executions == 1, "resend original task does not reexecute peer command");
            var file = Path.Combine(output, "import.bin"); var imported = await api.ExecuteAsync(await Mutation.ImportAsync(file, default)); var asset = imported.Deserialize<Asset>(ApiJson.Options)!;
            var upload = await api.ExecuteAsync(new("upload", "uploads", new { device_id = "desktop-router-02", asset_id = asset.AssetId, remote_path = "/tmp/desktop.bin", mode = "0644", timeout_seconds = 10 }));
            var uploadId = upload.GetProperty("task_id").GetString()!;
            await Eventually(async () => (await api.GetAsync<TaskDetail>("tasks/"+uploadId)).Result?.Status == "success", "file upload through Go and test peer");
            var download = await api.ExecuteAsync(new("download", "downloads", new { device_id = "desktop-router-02", remote_path = "/tmp/desktop.bin", name = "returned.bin", timeout_seconds = 10 }));
            var downloadId = download.GetProperty("task_id").GetString()!;
            await Eventually(async () => { var transfer = await api.GetAsync<Transfer>("tasks/"+downloadId+"/transfer"); return transfer.Committed && transfer.Released; }, "download committed and released independently");
            Invoke(window,"Navigate","files"); Invoke(window,"ShowCreatedTask",download); connection.Invalidate(); await Task.Delay(350); await InvokeAsync(window,"RefreshDetails");
            Check(Field<Transfer>(window,"transfer").Committed && Field<Transfer>(window,"transfer").Released && Field<DataGrid>(window,"tasksGrid").Items.Cast<TaskSummary>().All(t => t.Type is "upload" or "download"), "file page retains scoped transfer facts without general task entrance");

            var complete = await api.ExecuteAsync(new("complete", $"downloads/{downloadId}/complete")); var returned = complete.GetProperty("asset").Deserialize<Asset>(ApiJson.Options)!;
            var completedAgain = await api.ExecuteAsync(new("complete again", $"downloads/{downloadId}/complete")); Check(completedAgain.GetProperty("asset").GetProperty("asset_id").GetString() == returned.AssetId, "download explicit complete retains asset identity");
            var target = Path.Combine(output,"download.bin"); await api.SaveContentAsync(returned, target); var savedBytes = await File.ReadAllBytesAsync(target); var originalBytes = await File.ReadAllBytesAsync(file); Check(savedBytes.SequenceEqual(originalBytes), "asset stream saves identical verified content");
            var toolResult = await api.ExecuteAsync(new("tool", "tools", new { name = "网络诊断工具", description = "隔离测试资产" })); var tool = toolResult.Deserialize<Tool>(ApiJson.Options)!;
            var version = await api.ExecuteAsync(new("version", $"tools/{tool.ToolId}/versions/1.0", new { artifacts = new[] { new { asset_id = asset.AssetId, platform = "linux", mode = "0755", rules = new { arch = new[] { "any" }, libc = new[] { "any" } } } } }, "PUT"));
            var matches = await api.ListAsync<Compatibility>($"tools/{tool.ToolId}/versions/1.0/compatibility?device_id=desktop-router-02"); Check(matches.Single().Status == "compatible", "server-authoritative tool compatibility");
            var deploy = await api.ExecuteAsync(new("deploy", "deployments", new { device_id = "desktop-router-02", tool_id = tool.ToolId, version = "1.0", artifact_id = matches[0].Artifact.ArtifactId, remote_path = "/tmp/tool", timeout_seconds = 10 }));
            await Eventually(async () => (await api.GetAsync<TaskDetail>("tasks/"+deploy.GetProperty("task_id").GetString())).Result?.Status == "success", "tool deployment completes without executing asset");
            var maintenance = (await api.ExecuteAsync(new("maintenance", "maintenance", new { device_id = "desktop-router-02" }))).Deserialize<Maintenance>(ApiJson.Options)!;
            Check(maintenance.Endpoints.Length == 3 && (maintenance.ExpiresAt-maintenance.CreatedAt).TotalMinutes == 240, "default maintenance lease and fixed three endpoints");
            var custom = (await api.ExecuteAsync(new("custom maintenance", "maintenance", new { device_id = "desktop-router-03", lease_ms = 120000 }))).Deserialize<Maintenance>(ApiJson.Options)!;
            Check((custom.ExpiresAt-custom.CreatedAt).TotalMilliseconds == 120000, "custom positive lease unchanged");
            Invoke(window,"Navigate","maintenance"); connection.Invalidate(); await Task.Delay(400);
            Check(Field<DataGrid>(window,"endpointGrid").Items.Count == 3, "maintenance shows three public channel links");
            var resolved = (Task<(Maintenance, Endpoint)>)Invoke(window,"ResolveEndpoint","ssh")!;
            var (_, endpoint) = await resolved;
            var launch = EndpointLauncher.Build(endpoint, profile);
            Check(launch.ArgumentList.Contains("admin") && !launch.ArgumentList.Contains("-pw"), "external SSH launch uses configured account without password command argument");
            var ipv6 = (string)Invoke(window,"EndpointLink", new Endpoint("ssh","::1",2222,"[::1]:2222","ready",null))!;
            Check(ipv6 == "ssh://admin@[::1]:2222/", "IPv6 SSH link encodes public endpoint and account");
            try { await api.ExecuteAsync(new("config", "devices/desktop-router-02/config-tasks", new { backend = "nvram", operation = "get", key = "SN" })); throw new Exception("missing capability accepted"); }
            catch (ApiException e) { Check(e.Code == "unsupported_capability", "config capability error preserved"); }
            devices.SelectedItem = devices.Items.Cast<Device>().Single(d => d.DeviceId == "desktop-router-08"); Invoke(window,"Navigate","config");
            Field<TextBox>(window,"configKey").Text = "SN"; var pendingConfig = InvokeAsync(window,"SubmitConfig");
            Invoke(window,"Navigate","files"); await pendingConfig; var configTaskId = Field<string>(window,"configTaskId");
            Check(((TabItem)((TabControl)window.FindName("WorkspaceTabs")).SelectedItem).Tag.ToString() == "files" && Field<string>(window,"taskId") != configTaskId, "late configuration response neither navigates nor replaces file transfer selection");
            Invoke(window,"Navigate","config");
            await Eventually(async () => (await api.GetAsync<TaskDetail>("tasks/"+configTaskId)).Result?.Status == "success", "native NVRAM get creates configuration task through real API");
            var configDetail = await api.GetAsync<TaskDetail>("tasks/"+configTaskId); Check(configDetail.Params.GetProperty("backend").GetString() == "nvram" && configDetail.Params.GetProperty("operation").GetString() == "get", "native config parameters match public contract");
            await InvokeAsync(window,"RefreshDetails");
            Check(Field<TextBox>(window,"configOutput").Text.Contains("成功") && ((TabItem)((TabControl)window.FindName("WorkspaceTabs")).SelectedItem).Tag.ToString() == "config", "configuration result stays on configuration page");
            devices.SelectedItem = devices.Items.Cast<Device>().Single(d => d.DeviceId == "desktop-router-02");
            Invoke(window,"Navigate","maintenance");
            Invoke(window,"Navigate","files");
            await ExchangeWorkspaceChecks(window,api,connection,tool);
            await NativeTelemetryChecks(window,peers[1]);
            await ManagedDeviceChecks(window,api,connection,peers,controlPort);
            foreach (var theme in new[] { "Light", "Dark" }) {
                Theme.Apply(theme);
                foreach (var page in new[] { "overview", "maintenance", "files", "config", "tools" }) {
                    Invoke(window, "Navigate", page); await Task.Delay(150); await InvokeAsync(window,"RefreshDetails");
                    Render(window, theme.ToLowerInvariant()+"-"+page+"-1480.png");
                    if(page=="overview") {foreach(var pair in new[]{("storageMetrics","storage"),("networkMetrics","network")}) {var table=Field<DataGrid>(window,pair.Item1);var tabs=Field<TabControl>(window,"overviewTabs");var tab=tabs.Items.Cast<TabItem>().Single(t=>t.Header?.ToString()==(pair.Item2=="storage"?"存储空间":"系统端口"));tabs.SelectedItem=tab;await Task.Delay(80);Render(window,theme.ToLowerInvariant()+"-"+pair.Item2+"-1480.png");tabs.SelectedIndex=0;}}
                }
            }
            window.Width = 1000; window.Height = 700; Invoke(window,"Navigate","maintenance"); await Task.Delay(100); Render(window,"dark-maintenance-1000.png");
            Check(window.ActualWidth >= 960 && ((TabControl)window.FindName("WorkspaceTabs")).ActualWidth >= 640, "minimum desktop workspace sizing");
            window.Width = 1920; window.Height = 1080; Invoke(window,"Navigate","overview"); Theme.Apply("Light"); await Task.Delay(100); Render(window,"light-overview-1920.png");
            await api.ExecuteAsync(new("close", $"maintenance/{maintenance.MaintenanceId}/close")); Check((await api.GetAsync<Maintenance>($"maintenance/{maintenance.MaintenanceId}")).Released, "maintenance close releases public endpoints");
            try { await (Task<(Maintenance, Endpoint)>)Invoke(window,"ResolveEndpoint","ssh")!; throw new Exception("closed endpoint accepted"); }
            catch (InvalidOperationException) { Check(true, "closed maintenance link cannot launch external client"); }
            await VerifyMaintenanceReopen(window, api, connection);
            var beforeReplacement=(await api.ListAsync<ConnectionPeriod>("devices/desktop-router-02/connections")).First();
            var replacement = new TestProbe(); peers.Add(replacement); await replacement.StartAsync(controlPort,"desktop-router-02");
            await Eventually(() => Task.FromResult(connection.Snapshot.Devices.First(d=>d.DeviceId=="desktop-router-02").CurrentSession?.SessionId==replacement.SessionId),"WebSocket Session replacement refreshes HTTP snapshot");
            var afterReplacement=(await api.ListAsync<ConnectionPeriod>("devices/desktop-router-02/connections")).First();Check(beforeReplacement.Id==afterReplacement.Id&&beforeReplacement.OnlineAt==afterReplacement.OnlineAt,"session replacement retains continuous online history via API");
            await api.ExecuteAsync(new("archive version", $"tools/{tool.ToolId}/versions/1.0/archive")); await api.ExecuteAsync(new("archive asset", $"assets/{asset.AssetId}/archive"));
            Check((await api.GetAsync<Asset>($"assets/{asset.AssetId}")).Archived, "archive preserves original identity");
            await InvokeAsync(window, "Disconnect"); Check(!connection.Synchronized || connection.Token.IsCancellationRequested, "native disconnect cancels old connection");
            Check(((DataGrid)window.FindName("DevicesGrid")).Items.Count == 0, "disconnect clears native snapshot");
            Check(((TextBlock)window.FindName("StatusText")).Text == "已断开连接。" && ((TextBlock)window.FindName("ConnectionText")).Text == "未连接" && ((DataGrid)window.FindName("QuickProperties")).Items.Count == 0, "disconnect updates status and clears selected-device properties");
            Check(Field<string>(window,"taskId") == "" && Field<string>(window,"configTaskId") == "", "disconnect clears scoped file and configuration results before switching servers");
            Field<TextBox>(window,"ServerBox").Text = "http://bad/path";
            try { await InvokeAsync(window,"SaveAndConnect"); throw new Exception("invalid origin accepted"); } catch (ArgumentException) { Check(ServerProfile.Load(profileFile).ServerUrl == profile.ServerUrl, "invalid setting preserves saved server"); }
            Field<TextBox>(window,"ServerBox").Text = profile.ServerUrl;
            await InvokeAsync(window,"SaveAndConnect");
            var reconnected = Field<WorkspaceConnection>(window,"connection");
            await Eventually(() => Task.FromResult(reconnected.Synchronized), "settings save reconnects and replaces disposed connection");
            Field<TextBox>(window,"sshUser").Text = "operator"; Field<PasswordBox>(window,"sshPassword").Password = "edited-test-password";
            await InvokeAsync(window,"SaveSshSettings");
            Check(ServerProfile.Load(profileFile).SshUser == "operator" && SshPasswordStore.Load(profileFile) == "edited-test-password" && !File.ReadAllText(profileFile).Contains("edited-test-password"), "editable SSH account and protected password survive save");
            await InvokeAsync(window,"Disconnect"); window.Close(); await Task.Delay(250); window = null;
            window = new MainWindow(profileFile); window.Show();
            await Eventually(() => Task.FromResult(Field<WorkspaceConnection?>(window,"connection")?.Synchronized == true), "reopened window connects last saved server without toolbar input");
            Check(Field<TextBox>(window,"sshUser").Text == "operator" && Field<PasswordBox>(window,"sshPassword").Password == "edited-test-password", "reopened settings restore user-edited SSH credentials");
            await InvokeAsync(window,"Disconnect"); window.Close(); await Task.Delay(250); window = null;
        } finally {
            if (window != null) { try { await InvokeAsync(window,"Disconnect"); window.Close(); } catch { } }
            foreach (var peer in peers) { try { await peer.DisposeAsync(); } catch (IOException) { } }
            if (!server.HasExited) { server.Kill(true); await server.WaitForExitAsync(); }
            await File.WriteAllTextAsync(Path.Combine(output,"server.log"), await serverOut + await serverError);
        }
    }
    private static async Task VerifyConnectionDisplay(MainWindow window, WorkspaceConnection connection)
    {
        var quick = (DataGrid)window.FindName("QuickProperties");
        var source = quick.ItemsSource; var row = quick.Items[0];
        Invoke(window, "ApplySnapshot");
        Check(ReferenceEquals(source, quick.ItemsSource) && ReferenceEquals(row, quick.Items[0]), "unchanged snapshot preserves selected-device source and row identities");
        Check(((TextBlock)window.FindName("StatusText")).Text.StartsWith("已连接") && ((TextBlock)window.FindName("ConnectionText")).Text == "已连接", "successful startup updates both connection status displays");
        var log = Field<System.Collections.ObjectModel.ObservableCollection<ActivityRow>>(window, "activity");
        Check(log.Any(r => r.Level == "连接" && r.Message.StartsWith("已连接")), "output records completed connection after connecting entry");
        var count = log.Count;
        for (var n = 0; n < 4; n++) Invoke(window, "ApplySnapshot");
        Check(log.Count == count, "unchanged snapshots do not repeat connection messages");
        quick.UpdateLayout(); var container = quick.ItemContainerGenerator.ContainerFromIndex(0); var unloaded = 0;
        ((FrameworkElement)container).Unloaded += (_,_) => unloaded++;
        var changes = 0;
        if (quick.ItemsSource is System.Collections.Specialized.INotifyCollectionChanged observable) observable.CollectionChanged += (_,_) => changes++;
        var before = connection.Snapshot.FetchedAt;
        await Eventually(() => Task.FromResult(connection.Snapshot.FetchedAt > before), "five-second recovery refresh still runs", 7000);
        await Task.Delay(120);
        Check(ReferenceEquals(source, quick.ItemsSource) && ReferenceEquals(row, quick.Items[0]) && changes == 0 && log.Count == count, "periodic refresh keeps selected-device rows without reset or log spam");
        quick.UpdateLayout();
        Check(ReferenceEquals(container, quick.ItemContainerGenerator.ContainerFromIndex(0)) && unloaded == 0, "periodic refresh does not unload or recreate selected-device visual row");
        var heartbeat = quick.Items[8]; var notified = new List<string?>();
        ((System.ComponentModel.INotifyPropertyChanged)heartbeat).PropertyChanged += (_,e) => notified.Add(e.PropertyName);
        var original = Field<Snapshot>(window, "snapshot"); var selected = Field<string>(window, "selectedDevice");
        var device = original.Devices.Single(d => d.DeviceId == selected); var seen = (device.LastSeenAt ?? DateTimeOffset.UtcNow).AddMinutes(1);
        window.GetType().GetField("snapshot", BindingFlags.Instance | BindingFlags.NonPublic)!.SetValue(window, original with { Devices = original.Devices.Select(d => d.DeviceId == selected ? d with { Runtime = new DeviceRuntime(d.Runtime?.UptimeSeconds, seen), LastSeenAt = seen, Registration = d.Registration with { Model = "" } } : d).ToArray() });
        Invoke(window, "ApplySnapshot");
        Check(ReferenceEquals(heartbeat, quick.Items[8]) && heartbeat.GetType().GetProperty("Value")!.GetValue(heartbeat)?.ToString() == Labels.Time(seen) && notified.SequenceEqual(device.Runtime == null ? new[] { "ValueTip", "Value" } : new[] { "Value" }) && changes == 0, "heartbeat updates only existing value binding without resetting rows");
        Check(quick.Items[1].GetType().GetProperty("Value")!.GetValue(quick.Items[1])?.ToString() == "—", "missing selected-device field clears old value explicitly");
        Invoke(window, "ApplySnapshot"); Check(notified.Count == (device.Runtime == null ? 2 : 1), "unchanged heartbeat value emits no redundant notification");
        window.GetType().GetField("snapshot", BindingFlags.Instance | BindingFlags.NonPublic)!.SetValue(window, original); Invoke(window, "ApplySnapshot");
        Field<System.Net.WebSockets.ClientWebSocket>(connection, "socket").Abort();
        await Eventually(() => Task.FromResult(log.Any(r => r.Level == "连接" && r.Message.Contains("正在重连"))), "connection loss is recorded in activity output");
        await Eventually(() => Task.FromResult(connection.Synchronized && log.Count(r => r.Level == "连接" && r.Message.StartsWith("已连接")) == 2 && ((TextBlock)window.FindName("StatusText")).Text.StartsWith("已连接")), "automatic reconnect records recovery and replaces stale reconnect status");
        Invoke(window, "Log", "操作", "保留当前操作结果"); Invoke(window, "ApplySnapshot");
        Check(((TextBlock)window.FindName("StatusText")).Text == "保留当前操作结果", "unchanged connection refresh preserves current operation result");
    }
    private static async Task VerifyMaintenanceReopen(MainWindow window, ApiClient api, WorkspaceConnection connection)
    {
        Invoke(window, "Navigate", "maintenance"); connection.Invalidate();
        await Eventually(() => Task.FromResult(connection.Snapshot.Maintenance.Any(m => m.DeviceId == "desktop-router-02" && m.Released)), "closed maintenance history reaches HTTP snapshot");
        await Task.Delay(120);
        for (var cycle = 0; cycle < 3; cycle++) {
            var before = Field<Snapshot>(window, "snapshot");
            await InvokeAsync(window, "CreateMaintenance");
            var current = (await api.ListAsync<Maintenance>("maintenance?device_id=desktop-router-02")).Single(m => !m.Released);
            // A queued busy/status notification can still carry the pre-create HTTP snapshot.
            window.GetType().GetField("snapshot", BindingFlags.Instance | BindingFlags.NonPublic)!.SetValue(window, before);
            Invoke(window, "ApplySnapshot");
            Check(Field<string>(window, "maintenanceId") == current.MaintenanceId, "late pre-create snapshot preserves newly created maintenance identity");
            Field<DataGrid>(window, "maintenanceGrid").SelectedItem = Field<DataGrid>(window, "maintenanceGrid").Items.Cast<Maintenance>().First(m => m.Released);
            await InvokeAsync(window, "CreateMaintenance");
            var live = (await api.ListAsync<Maintenance>("maintenance?device_id=desktop-router-02")).Where(m => !m.Released).ToArray();
            Check(live.Length == 1 && Field<string>(window, "maintenanceId") == current.MaintenanceId && live[0].ExpiresAt == current.ExpiresAt, "open while history selected reuses live maintenance without conflict or lease change");
            await InvokeAsync(window, "CloseMaintenanceRecord", current);
            window.GetType().GetField("snapshot", BindingFlags.Instance | BindingFlags.NonPublic)!.SetValue(window, before with { Maintenance = [.. before.Maintenance, current] });
            Invoke(window, "ApplySnapshot");
            Check(Field<DataGrid>(window, "maintenanceGrid").Items.Cast<Maintenance>().Single(m => m.MaintenanceId == current.MaintenanceId).Released, "late pre-close snapshot cannot resurrect released maintenance");
            Check(!Field<Button>(window, "closeMaintenanceButton").IsEnabled, "released response immediately disables closing historical maintenance");
            Check((await api.GetAsync<Maintenance>($"maintenance/{current.MaintenanceId}")).Released, "close releases the current maintenance before reopen");
            connection.Invalidate();
            await Eventually(() => Task.FromResult(connection.Snapshot.Maintenance.Any(m => m.MaintenanceId == current.MaintenanceId && m.Released)), "repeated open-close cycle synchronizes released state");
            await Task.Delay(100);
        }
    }
    private static T Field<T>(object instance, string name) => (T)instance.GetType().GetField(name, BindingFlags.NonPublic | BindingFlags.Instance)!.GetValue(instance)!;
    private static object? Invoke(object instance, string name, params object[] args) => instance.GetType().GetMethod(name, BindingFlags.NonPublic | BindingFlags.Instance)!.Invoke(instance, args);
    private static Task InvokeAsync(object instance, string name, params object[] args) => (Task)Invoke(instance,name,args)!;
    private static Task<JsonElement> InvokeResult(object instance, string name, params object[] args) => (Task<JsonElement>)Invoke(instance,name,args)!;
    private static void Render(Window window, string name)
    {
        window.UpdateLayout(); var visual = (FrameworkElement)window.Content;
        var size = new Size(visual.ActualWidth, visual.ActualHeight);
        using (var xps = new System.Windows.Xps.Packaging.XpsDocument(Path.Combine(output, Path.ChangeExtension(name,"xps")), FileAccess.Write))
            System.Windows.Xps.Packaging.XpsDocument.CreateXpsDocumentWriter(xps).Write(visual);
        var bitmap = new RenderTargetBitmap((int)Math.Ceiling(size.Width), (int)Math.Ceiling(size.Height),96,96,PixelFormats.Pbgra32); bitmap.Render(visual);
        var pixels = new byte[bitmap.PixelWidth * bitmap.PixelHeight * 4]; bitmap.CopyPixels(pixels, bitmap.PixelWidth * 4, 0);
        if (!pixels.Any(p => p != 0)) { Console.WriteLine("RENDER XPS " + name + " (desktop raster capture unavailable)"); return; }
        var encoder = new PngBitmapEncoder(); encoder.Frames.Add(BitmapFrame.Create(bitmap)); using var file = File.Create(Path.Combine(output,name)); encoder.Save(file);
    }
    private static HttpResponseMessage Json(string json) => new(HttpStatusCode.OK) { Content = new StringContent(json,Encoding.UTF8,"application/json") };
    private sealed class DelegateHandler(Func<HttpRequestMessage,CancellationToken,Task<HttpResponseMessage>> send) : HttpMessageHandler {
        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken) => send(request,cancellationToken);
    }
}
