using System.Diagnostics;
using System.Net;

using System.Text;
using System.Text.Json;
using RouterWorkbench.Core;

namespace RouterWorkbench.Tests;

internal static class Program
{
    private static int assertions;
    private static string output = "";
    private static Process server = null!;
    private static int controlPort;
    private static ServerProfile profile = null!;
    [STAThread]
    private static int Main(string[] args)
    {
        if (Environment.GetEnvironmentVariable("RMP_CAPTURE_ARGS") is { Length: > 0 } capture)
        { File.WriteAllText(capture, JsonSerializer.Serialize(args)); return 0; }
        if (args.Length != 2) { Console.Error.WriteLine("usage: RouterWorkbench.Tests <server.exe> <output-directory>"); return 2; }
        output = Path.GetFullPath(args[1]); Directory.CreateDirectory(output);
        try
        {
            StartServer(Path.GetFullPath(args[0]));
            Task.Run(CoreTestsAsync).GetAwaiter().GetResult();


            Console.WriteLine($"PASS: {assertions} assertions; client/HTTP/WebSocket integration.");
            return 0;
        }
        catch (Exception e) { Console.Error.WriteLine(e); return 1; }
        finally { if (server != null) { if (!server.HasExited) { server.Kill(true); server.WaitForExit(); } server.Dispose(); } }
    }
    private static void Assert(bool value, string label)
    {
        if (!value) throw new InvalidOperationException("FAIL: " + label);
        assertions++;
    }
    private static async Task UntilAsync(Func<bool> condition, string label, int seconds = 10)
    {
        var timeout = Stopwatch.StartNew();
        while (!condition()) { if (timeout.Elapsed > TimeSpan.FromSeconds(seconds)) throw new TimeoutException(label); await Task.Delay(20); }
        Assert(true, label);
    }
    private static async Task ThrowsAsync<T>(Func<Task> operation, string label) where T : Exception
    {
        try { await operation(); } catch (T) { Assert(true, label); return; }
        throw new InvalidOperationException("Expected " + typeof(T).Name + ": " + label);
    }
    private static void StartServer(string binary)
    {
        var port = MockEvents.FreePort(); controlPort = MockEvents.FreePort();
        profile = new() { ServerUrl = $"http://127.0.0.1:{port}" };
        var info = new ProcessStartInfo(binary) { UseShellExecute = false, CreateNoWindow = true, RedirectStandardOutput = true, RedirectStandardError = true };
        foreach (var arg in new[] { "-listen", $"127.0.0.1:{controlPort}", "-http-listen", $"127.0.0.1:{port}", "-tunnel-data-listen", "127.0.0.1:0", "-tunnel-port-first", "32000", "-tunnel-port-last", "32029", "-tunnel-port-reuse-delay", "1ms", "-repository-dir", Path.Combine(output, "repository-" + Guid.NewGuid().ToString("N")) }) info.ArgumentList.Add(arg);
        server = Process.Start(info)!;
        server.OutputDataReceived += (_, e) => { if (e.Data != null) File.AppendAllText(Path.Combine(output, "server.log"), e.Data + "\n"); };
        server.ErrorDataReceived += (_, e) => { if (e.Data != null) Console.Error.WriteLine(e.Data); };
        server.BeginOutputReadLine(); server.BeginErrorReadLine();
    }
    private static async Task CoreTestsAsync()
    {
        using var api = new ApiClient(profile.BaseUri());
        for (var i = 0; ; i++) { try { await api.ListAsync<Device>("devices", default); break; } catch (HttpRequestException) when (i < 40) { await Task.Delay(50); } }
        await using var probe = new TestProbe(); await probe.StartAsync(controlPort, "phase6-device");
        await using var workspace = new WorkspaceConnection(profile);
        Snapshot? snapshot = null;
        workspace.SnapshotChanged += value => Volatile.Write(ref snapshot, value);
        workspace.Start();
        await UntilAsync(() => snapshot?.Devices.Any(d => d.DeviceId == "phase6-device") == true && workspace.IsSynchronized, "initial HTTP snapshot after WS");
        var original = await workspace.ExecuteAsync(Mutation.Json("创建远程维护", "maintenance", new { device_id = "phase6-device" }));
        var maintenance = original.Deserialize<Maintenance>(Wire.Json)!;
        Assert(maintenance.ExpiresAt - maintenance.CreatedAt == TimeSpan.FromMinutes(240), "default 240 min");
        Assert(maintenance.Endpoints.Select(e => e.Service).SequenceEqual(["web", "ssh", "telnet"]), "three fixed endpoints");
        Assert(!original.GetRawText().Contains("connection_id") && !original.GetRawText().Contains("token"), "no private data");
        await UntilAsync(() => snapshot!.Maintenance.Any(m => m.MaintenanceId == maintenance.MaintenanceId), "maintenance live refresh");
        await ThrowsAsync<ApiException>(() => workspace.ExecuteAsync(Mutation.Json("创建远程维护", "maintenance", new { device_id = "phase6-device" })), "maintenance conflict");
        Assert(workspace.Pending == null, "business error clears pending");
        await workspace.ExecuteAsync(Mutation.Json("关闭远程维护", $"maintenance/{maintenance.MaintenanceId}/close", new { }));
        await UntilAsync(() => snapshot!.Maintenance.Any(m => m.MaintenanceId == maintenance.MaintenanceId && m.Released), "close refreshed");
        var shortLease = (await workspace.ExecuteAsync(Mutation.Json("创建远程维护", "maintenance", new { device_id = "phase6-device", lease_ms = 150 }))).Deserialize<Maintenance>(Wire.Json)!;
        Assert(shortLease.ExpiresAt - shortLease.CreatedAt == TimeSpan.FromMilliseconds(150), "custom positive lease");
        await UntilAsync(() => snapshot!.Maintenance.Any(m => m.MaintenanceId == shortLease.MaintenanceId && m.Released), "expiry refreshed from HTTP");
        var oldSession = snapshot!.Devices.Single(d => d.DeviceId == "phase6-device").CurrentSession!.SessionId;
        var active = (await workspace.ExecuteAsync(Mutation.Json("创建远程维护", "maintenance", new { device_id = "phase6-device" }))).Deserialize<Maintenance>(Wire.Json)!;
        await using var replacement = new TestProbe(); await replacement.StartAsync(controlPort, "phase6-device");
        await UntilAsync(() => snapshot!.Devices.Single(d => d.DeviceId == "phase6-device").CurrentSession?.SessionId != oldSession && snapshot.Maintenance.Any(m => m.MaintenanceId == active.MaintenanceId && m.Released), "session replacement revokes maintenance");
        Assert(snapshot!.Devices.Single(d => d.DeviceId == "phase6-device").LastOfflineAt == null, "replacement remains online");

        var exec = Mutation.Json("创建 Exec", "tasks", new { device_id = "phase6-device", command = "fixture", timeout_seconds = 30 });
        var accepted = await workspace.ExecuteAsync(exec); var taskId = accepted.GetProperty("task_id").GetString()!;
        Assert(!string.IsNullOrEmpty(taskId), "task identity preserved");
        var replay = await api.ExecuteAsync(exec, default); Assert(replay.GetProperty("task_id").GetString() == taskId, "same HTTP key replay");
        await WaitTaskAsync(api, taskId);
        var task = await api.GetAsync<TaskDetail>($"tasks/{taskId}", default);
        Assert(task.Result!.Stdout.Contains("中文结果") && task.Result.Stderr == "fixture stderr", "exec result fields");
        await workspace.ExecuteAsync(Mutation.Json("重发原 Task", $"tasks/{taskId}/resend", new { }));
        Assert(replacement.Executions == 1, "resend does not execute again");

        var source = Path.Combine(output, "source.txt"); await File.WriteAllTextAsync(source, "asset fixture bytes\n中文");
        var asset = (await workspace.ExecuteAsync(await Mutation.ImportAsync(source, default))).Deserialize<Asset>(Wire.Json)!;
        var saved = Path.Combine(output, "saved.txt"); await workspace.SaveAssetAsync(asset, saved);
        Assert(File.ReadAllBytes(source).SequenceEqual(File.ReadAllBytes(saved)), "raw asset round trip");
        var tool = (await workspace.ExecuteAsync(Mutation.Json("创建工具", "tools", new { name = "测试工具", description = "phase6" }))).Deserialize<Tool>(Wire.Json)!;
        var versionPath = $"tools/{tool.ToolId}/versions/v1";
        var version = (await workspace.ExecuteAsync(Mutation.Json("发布工具版本", versionPath, new { artifacts = new[] { new { asset_id = asset.AssetId, platform = "linux", mode = "0755", rules = new { arch = new[] { "x86_64" }, libc = new[] { "any" } } } } }, "PUT"))).Deserialize<ToolVersion>(Wire.Json)!;
        var matches = await api.ListAsync<Compatibility>(versionPath + "/compatibility?device_id=phase6-device", default);
        Assert(matches.Single().Status == "compatible", "compatibility comes from server");
        var deployed = await workspace.ExecuteAsync(Mutation.Json("投放工具", "deployments", new { device_id = "phase6-device", tool_id = tool.ToolId, version = "v1", artifact_id = version.Artifacts[0].ArtifactId, remote_path = "/tmp/tool", timeout_seconds = 30 }));
        await WaitTaskAsync(api, deployed.GetProperty("task_id").GetString()!);
        var uploaded = await workspace.ExecuteAsync(Mutation.Json("上传文件", "uploads", new { device_id = "phase6-device", asset_id = asset.AssetId, mode = "0644", remote_path = "/tmp/upload", timeout_seconds = 30 }));
        await WaitTaskAsync(api, uploaded.GetProperty("task_id").GetString()!);
        var downloaded = await workspace.ExecuteAsync(Mutation.Json("下载文件", "downloads", new { device_id = "phase6-device", remote_path = "/tmp/upload", name = "returned.txt", timeout_seconds = 30 }));
        var downloadId = downloaded.GetProperty("task_id").GetString()!; await WaitTaskAsync(api, downloadId);
        var transfer = await api.GetAsync<Transfer>($"tasks/{downloadId}/transfer", default);
        Assert(transfer.Committed && transfer.Released, "download commit/release facts");
        var complete = await workspace.ExecuteAsync(Mutation.Json("导入下载", $"downloads/{downloadId}/complete", new { }));
        var completedAgain = await workspace.ExecuteAsync(Mutation.Json("导入下载", $"downloads/{downloadId}/complete", new { }));
        Assert(complete.GetProperty("asset").GetProperty("asset_id").GetString() == completedAgain.GetProperty("asset").GetProperty("asset_id").GetString(), "stable imported identity");
        await workspace.ExecuteAsync(Mutation.Json("清理下载", $"downloads/{downloadId}/cleanup", new { }));
        await workspace.ExecuteAsync(Mutation.Json("归档版本", versionPath + "/archive", new { }));
        await workspace.ExecuteAsync(Mutation.Json("归档工具", $"tools/{tool.ToolId}/archive", new { }));
        await workspace.ExecuteAsync(Mutation.Json("归档资产", $"assets/{asset.AssetId}/archive", new { }));
        await UntilAsync(() => snapshot!.Assets.Any(a => a.AssetId == asset.AssetId && a.Archived) && snapshot.Tools.Any(t => t.ToolId == tool.ToolId && t.Archived), "file/tool real-time refresh");

        await EndpointTestsAsync(maintenance);
        await LifecycleTestsAsync();
        await MutationFailureTestsAsync();
        foreach (var code in new[] { "device_offline", "session_changed", "capacity_exhausted", "conflict", "internal_error", "incompatible" })
            Assert(Errors.Describe(new ApiException(code, "fixture", 409), "创建远程维护").Contains(code), "business code visible: " + code);
        await api.ExecuteAsync(Mutation.Json("断开设备", "devices/phase6-device/disconnect", new { }), default);
        await UntilAsync(() => snapshot!.Devices.Single(d => d.DeviceId == "phase6-device").Status == "offline", "device offline live refresh");
        await ThrowsAsync<ApiException>(() => workspace.ExecuteAsync(Mutation.Json("创建 Exec", "tasks", new { device_id = "phase6-device", command = "fixture", timeout_seconds = 30 })), "offline error");
        Console.WriteLine("PASS core: live server devices/session/maintenance/exec/file/tool and reconnect/errors");
    }
    private static async Task WaitTaskAsync(ApiClient api, string id)
    {
        for (var i = 0; i < 200; i++) { var t = await api.GetAsync<TaskDetail>($"tasks/{id}", default); if (t.Result != null) { Assert(t.Result.Status == "success", "file/exec result success"); return; } await Task.Delay(20); }
        throw new TimeoutException("Task result " + id);
    }
    private static async Task EndpointTestsAsync(Maintenance maintenance)
    {
        var web = EndpointLauncher.Build(maintenance.Endpoints[0], profile); Assert(web.UseShellExecute && web.FileName.StartsWith("http://"), "system browser dispatch");
        var executable = Environment.ProcessPath!;
        // dotnet run launches the generated apphost, so the test executable can
        // capture actual argv without spawning any real SSH/Telnet session.
        if (Path.GetFileNameWithoutExtension(executable) == "dotnet") executable = Path.Combine(AppContext.BaseDirectory, "RouterWorkbench.Tests.exe");
        var launchProfile = profile with { SshExecutable = executable, TelnetExecutable = executable, SshUser = "root" };
        foreach (var putty in new[] { false, true })
        foreach (var endpoint in maintenance.Endpoints.Skip(1))
        {
            var start = EndpointLauncher.Build(endpoint, launchProfile with { SshUsePutty = putty, TelnetUsePutty = putty });
            var capture = Path.Combine(output, $"argv-{endpoint.Service}-{putty}.json"); start.Environment["RMP_CAPTURE_ARGS"] = capture; start.CreateNoWindow = true;
            using var p = Process.Start(start)!; await p.WaitForExitAsync();
            Assert(p.ExitCode == 0, "external process starts");
            var args = JsonSerializer.Deserialize<string[]>(File.ReadAllText(capture))!;
            Assert(args.Contains(endpoint.Host) && args.Contains(endpoint.Port.ToString()) && !start.UseShellExecute, "host/port separate argv");
            if (endpoint.Service == "ssh") Assert(args.Contains("-l") && args.Contains("root"), "SSH username argument");
        }
        await ThrowsAsync<InvalidDataException>(() => Task.Run(() => EndpointLauncher.Build(maintenance.Endpoints[1] with { Host = "-oProxyCommand=bad" }, launchProfile)), "reject option injection");
        await ThrowsAsync<FileNotFoundException>(() => Task.Run(() => EndpointLauncher.Build(maintenance.Endpoints[2], launchProfile with { TelnetExecutable = "missing.exe" })), "missing client actionable error");
    }
    private static async Task LifecycleTestsAsync()
    {
        await using var mock = new MockEvents();
        mock.HoldDevices = new(TaskCreationOptions.RunContinuationsAsynchronously);
        var c = new WorkspaceConnection(new() { ServerUrl = mock.Uri.AbsoluteUri }, reconnectDelay: TimeSpan.FromMilliseconds(25));
        Snapshot? snapshot = null; var updates = 0;
        c.SnapshotChanged += value => { snapshot = value; Interlocked.Increment(ref updates); }; c.Start();
        await UntilAsync(() => mock.DeviceReads > 0, "resync begins HTTP fetch");
        mock.DeviceId = "new"; await mock.NotifyAsync(); var hold = mock.HoldDevices; mock.HoldDevices = null; hold.SetResult();
        await UntilAsync(() => snapshot?.Devices.Single().DeviceId == "new", "event during snapshot triggers second fetch");
        Assert(mock.DeviceReads >= 2, "no lost invalidation");
        mock.DeviceId = "after-disconnect"; mock.Socket!.Abort();
        await UntilAsync(() => mock.Connections >= 2 && snapshot?.Devices.Single().DeviceId == "after-disconnect", "WS reconnect gets current snapshot without replay");
        var before = updates; await c.DisposeAsync(); await c.DisposeAsync();
        await Task.Delay(100); Assert(updates == before && !c.IsSynchronized, "dispose joins updates/socket and is idempotent");
        await using var pending = new MockEvents(); pending.HoldDevices = new(TaskCreationOptions.RunContinuationsAsynchronously);
        var closing = new WorkspaceConnection(new() { ServerUrl = pending.Uri.AbsoluteUri }); closing.Start();
        await UntilAsync(() => pending.DeviceReads > 0, "inflight fetch established");
        await closing.DisposeAsync().AsTask().WaitAsync(TimeSpan.FromSeconds(2)); Assert(true, "close cancels pending HTTP");
        var unreachable = new WorkspaceConnection(new() { ServerUrl = $"http://127.0.0.1:{MockEvents.FreePort()}" }, reconnectDelay: TimeSpan.FromMilliseconds(20));
        string state = ""; unreachable.ConnectionChanged += value => state = value; unreachable.Start();
        await UntilAsync(() => state.Contains("不可达"), "unreachable distinguished"); await unreachable.DisposeAsync();
    }
    private sealed class LostResponseHandler : HttpMessageHandler
    {
        public readonly TaskCompletionSource Entered = new(TaskCreationOptions.RunContinuationsAsynchronously);
        public readonly TaskCompletionSource Release = new(TaskCreationOptions.RunContinuationsAsynchronously);
        public readonly List<string> Keys = []; public readonly List<byte[]> Bodies = [];
        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken ct)
        {
            Keys.Add(request.Headers.GetValues("Idempotency-Key").Single()); Bodies.Add(await request.Content!.ReadAsByteArrayAsync(ct));
            Entered.TrySetResult(); await Release.Task.WaitAsync(ct);
            if (Keys.Count == 1) throw new HttpRequestException("response lost");
            return new(HttpStatusCode.Accepted) { Content = new StringContent("{\"data\":{\"task_id\":\"original-task\",\"dispatch_uncertain\":true}}") };
        }
    }
    private static async Task MutationFailureTestsAsync()
    {
        var handler = new LostResponseHandler(); await using var c = new WorkspaceConnection(profile, handler);
        var request = Mutation.Json("创建 Exec", "tasks", new { device_id = "d", command = "side-effect", timeout_seconds = 30 });
        var first = c.ExecuteAsync(request); await handler.Entered.Task;
        await ThrowsAsync<InvalidOperationException>(() => c.ExecuteAsync(request), "duplicate click while in flight");
        handler.Release.SetResult(); await ThrowsAsync<HttpRequestException>(() => first, "response lost");
        Assert(ReferenceEquals(c.Pending, request), "original uncertain request retained");
        await ThrowsAsync<InvalidOperationException>(() => c.ExecuteAsync(Mutation.Json("新请求", "tasks", new { })), "new mutation blocked while unresolved");
        var response = await c.ExecuteAsync(request, true);
        Assert(response.GetProperty("task_id").GetString() == "original-task" && response.GetProperty("dispatch_uncertain").GetBoolean(), "nonempty task with uncertain dispatch retained");
        Assert(handler.Keys.Distinct().Count() == 1 && handler.Bodies[0].SequenceEqual(handler.Bodies[1]), "retry identical key and bytes");
        Assert(c.Pending == null, "response settles pending request");
    }
}
