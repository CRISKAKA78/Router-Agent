using System.Collections.Concurrent;
using System.Net;
using System.Net.WebSockets;
using System.Text;
using System.Text.Json;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Hosting.Server;
using Microsoft.AspNetCore.Hosting.Server.Features;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Microsoft.JSInterop;
using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Persistence;
using ProbeTemplateGenerator.Services;

namespace ProbeTemplateGenerator.Tests;

public sealed class PublishingTests
{
    [Theory]
    [InlineData(1)] [InlineData(2)]
    public void RejectsPreviousWorkspaceDrafts(int version)
    {
        var persistence=CreatePersistence();
        Assert.ThrowsAny<Exception>(()=>persistence.ReadImport($$"""{"workspace_version":{{version}},"project":{"format":"router-agent-template-project","schema_version":8,"name":"旧草稿","attributes":[]},"savedAt":1}"""));
    }

    [Fact]
    public async Task BrowserDraftRoundTripPreservesExactUncertainRequestAndInvalidEditingState()
    {
        var memory = new BrowserStorage();
        var persistence = new WorkspacePersistence(memory, new ProjectFiles());
        var pending = new PendingTemplateMutation("http://localhost:8080", "更新模板", "PUT", "probe-templates/one",
            "{ \"version\":9, \"name\":\"中文\", \"properties\":{} }", "original-key-1");
        var draft = new WorkspaceDraft
        {
            Project = new TemplateProject { Name = "", Attributes = [new TemplateAttribute()] },
            Pending = pending,
            SavedAt = 500,
            Target = new PublishingTarget(pending.Origin, "one", 9),
            ServerUrl = "http://incomplete:",
            Theme = "Dark"
        };
        await persistence.SaveAsync(draft);
        var restored = await persistence.LoadAsync();
        Assert.Equal(pending, restored!.Pending);
        Assert.Equal("", restored.Project.Name);
        Assert.Equal("", Assert.Single(restored.Project.Attributes).Key);
        Assert.Equal("http://incomplete:", restored.ServerUrl);
        Assert.Equal("Dark", restored.Theme);
    }

    [Fact]
    public async Task CorruptCurrentDraftIsPreserved()
    {
        var memory = new BrowserStorage();
        memory.Values[WorkspacePersistence.StorageKey] = "{\"broken\"";
        var persistence = new WorkspacePersistence(memory, new ProjectFiles());
        await Assert.ThrowsAnyAsync<JsonException>(() => persistence.LoadAsync());
        Assert.Equal("{\"broken\"", memory.Values[WorkspacePersistence.StorageKey]);
        Assert.Equal(0, memory.Writes);
    }

    [Theory]
    [InlineData("http://user:password@localhost:8080")]
    [InlineData("http://localhost/api")]
    [InlineData("file:///tmp/test")]
    [InlineData("http://localhost?x=1")]
    [InlineData("http://localhost#fragment")]
    public void RejectsAddressesOutsideOriginContract(string origin)
        => Assert.Throws<ArgumentException>(() => TemplatePublishingService.NormalizeOrigin(origin));

    [Fact]
    public async Task FirstWebSocketEventGatesWritesAndListPaginationIsComplete()
    {
        var releaseEvent = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var offsets = new ConcurrentQueue<int>();
        await using var server = await ApiPeer.StartAsync(async context =>
        {
            var offset = int.Parse(context.Request.Query["offset"]!);
            offsets.Enqueue(offset);
            var count = offset == 0 ? 200 : 1;
            await context.Response.WriteAsJsonAsync(new
            {
                data = new { items = Enumerable.Range(offset, count).Select(i => Template("id-" + i)), total = 201 }
            });
        }, releaseEvent.Task);
        await using var service = NewService();
        await service.ConnectAsync(server.Origin);
        Assert.False(service.Synchronized);
        await Assert.ThrowsAsync<InvalidOperationException>(() => service.PublishAsync(Template(), false));
        Assert.Empty(offsets);
        releaseEvent.SetResult();
        await UntilAsync(() => service.Synchronized);
        Assert.Equal(201, service.Templates.Count);
        Assert.Equal(new[] { 0, 200 }, offsets.Take(2));
    }

    [Fact]
    public async Task LostResponsePersistsBeforeSendingAndRetriesSameBytesAndKey()
    {
        var requests = new ConcurrentQueue<(string Method, string Path, string Body, string Key)>();
        PendingTemplateMutation? savedPending = null;
        await using var server = await ApiPeer.StartAsync(async context =>
        {
            if (context.Request.Method == "GET") { await WriteList(context); return; }
            var body = await new StreamReader(context.Request.Body).ReadToEndAsync();
            var key = context.Request.Headers["Idempotency-Key"].ToString();
            Assert.NotNull(savedPending);
            Assert.Equal(savedPending!.Body, body);
            Assert.Equal(savedPending.Key, key);
            requests.Enqueue((context.Request.Method, context.Request.Path, body, key));
            if (requests.Count == 1) { context.Abort(); return; }
            await context.Response.WriteAsJsonAsync(new { data = Template() });
        });
        await using var service = NewService();
        service.PersistStateAsync = () => { savedPending = service.Pending; return Task.CompletedTask; };
        await service.ConnectAsync(server.Origin);
        await UntilAsync(() => service.Synchronized);
        await Assert.ThrowsAnyAsync<Exception>(() => service.PublishAsync(Template(), false));
        var original = Assert.IsType<PendingTemplateMutation>(service.Pending);
        Assert.Equal(original, savedPending);
        await Assert.ThrowsAsync<InvalidOperationException>(() => service.PublishAsync(Template(), false));
        await Assert.ThrowsAsync<InvalidOperationException>(() => service.RetryPendingAsync(false));
        var result = await service.RetryPendingAsync(true);
        Assert.Equal("template-1", result!.TemplateId);
        Assert.Null(service.Pending);
        Assert.Null(savedPending);
        Assert.Equal(requests.First(), requests.Last());
        Assert.Equal(2, requests.Count);
        Assert.Equal((ulong)7, service.Target!.Version);
    }

    [Fact]
    public async Task VersionConflictIsDefinitiveAndPreservesExpectedBinding()
    {
        string? requestBody = null;
        await using var server = await ApiPeer.StartAsync(async context =>
        {
            if (context.Request.Method == "GET") { await WriteList(context); return; }
            requestBody = await new StreamReader(context.Request.Body).ReadToEndAsync();
            context.Response.StatusCode = 409;
            await context.Response.WriteAsJsonAsync(new { error = new { code = "conflict", message = "version mismatch" } });
        });
        await using var service = NewService();
        await service.ConnectAsync(server.Origin);
        await UntilAsync(() => service.Synchronized);
        service.Bind(Template());
        var error = await Assert.ThrowsAsync<TemplateApiException>(() => service.PublishAsync(Template(), true));
        Assert.Equal("conflict", error.Code);
        Assert.Null(service.Pending);
        Assert.Equal((ulong)7, service.Target!.Version);
        using var body = JsonDocument.Parse(requestBody!);
        Assert.Equal(7, body.RootElement.GetProperty("version").GetInt32());
        Assert.False(body.RootElement.TryGetProperty("template_id", out _));
    }

    [Fact]
    public async Task FailedLocalPersistencePreventsAnyWrite()
    {
        var writes = 0;
        await using var server = await ApiPeer.StartAsync(async context =>
        {
            if (context.Request.Method != "GET") Interlocked.Increment(ref writes);
            await WriteList(context);
        });
        await using var service = NewService();
        service.PersistStateAsync = () => throw new IOException("storage quota");
        await service.ConnectAsync(server.Origin);
        await UntilAsync(() => service.Synchronized);
        await Assert.ThrowsAsync<IOException>(() => service.PublishAsync(Template(), false));
        Assert.Equal(0, writes);
        Assert.NotNull(service.Pending);
        Assert.False(service.Busy);
    }

    [Fact]
    public async Task DeleteUsesExpectedVersionAndClearsOnlyMatchingTarget()
    {
        string? method = null, requestBody = null;
        await using var server = await ApiPeer.StartAsync(async context =>
        {
            if (context.Request.Method == "GET") { await WriteList(context); return; }
            method = context.Request.Method;
            requestBody = await new StreamReader(context.Request.Body).ReadToEndAsync();
            Assert.False(string.IsNullOrEmpty(context.Request.Headers["Idempotency-Key"]));
            await context.Response.WriteAsJsonAsync(new { data = new { deleted = true } });
        });
        await using var service = NewService();
        await service.ConnectAsync(server.Origin);
        await UntilAsync(() => service.Synchronized);
        service.Bind(Template());
        await service.DeleteAsync(Template());
        Assert.Equal("DELETE", method);
        Assert.Equal("{\"version\":7}", requestBody);
        Assert.Null(service.Target);
        Assert.Null(service.Pending);
    }

    [Fact]
    public async Task WebSocketReconnectFetchesANewHttpSnapshot()
    {
        var snapshots = 0;
        await using var server = await ApiPeer.StartAsync(async context =>
        {
            var revision = Interlocked.Increment(ref snapshots);
            await context.Response.WriteAsJsonAsync(new
            {
                data = new { items = new[] { Template("snapshot-" + revision) }, total = 1 }
            });
        }, closeFirstConnection: true);
        await using var service = NewService();
        await service.ConnectAsync(server.Origin);
        await UntilAsync(() => service.Synchronized && snapshots >= 2);
        Assert.Equal("snapshot-2", Assert.Single(service.Templates).TemplateId);
    }

    [Fact]
    public async Task DisconnectCancelsAndWaitsForOldSnapshotAndRetainsUncertainRequest()
    {
        var started = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var cancelled = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        await using var server = await ApiPeer.StartAsync(async context =>
        {
            started.TrySetResult();
            try { await Task.Delay(Timeout.Infinite, context.RequestAborted); }
            catch (OperationCanceledException) { cancelled.TrySetResult(); }
        });
        await using var service = NewService();
        var pending = new PendingTemplateMutation(server.Origin, "发布新模板", "POST", "probe-templates", "{}", "kept-key");
        service.Restore(null, pending);
        await service.ConnectAsync(server.Origin);
        await started.Task.WaitAsync(TimeSpan.FromSeconds(5));
        await service.DisconnectAsync().WaitAsync(TimeSpan.FromSeconds(5));
        await cancelled.Task.WaitAsync(TimeSpan.FromSeconds(5));
        Assert.Null(service.Origin);
        Assert.Empty(service.Templates);
        Assert.Equal(pending, service.Pending);
        await Assert.ThrowsAsync<InvalidOperationException>(() => service.ConnectAsync("http://localhost:9999"));
    }


    [Fact]
    public async Task OldServerBlocksNeighborPublishBeforeAnyWrite()
    {
        int writes=0;
        await using var server=await ApiPeer.StartAsync(async c=>{if(c.Request.Method=="GET")await WriteList(c);else{writes++;await c.Response.WriteAsJsonAsync(new {error=new{code="invalid_request"}});}});
        await using var service=NewService();await service.ConnectAsync(server.Origin);await UntilAsync(()=>service.Synchronized);
        var template=Template();template.NeighborProbe=new(){Domains=[new(){Interface="br0"}]};
        var error=await Assert.ThrowsAsync<InvalidOperationException>(()=>service.PublishAsync(template,false));
        Assert.Contains("当前Management Server不支持邻居发现模板",error.Message);Assert.Contains("neighbor_probe",service.CapabilitySummary);Assert.Equal(0,writes);Assert.Null(service.Pending);
    }

    [Fact]
    public async Task ReadOnlyInspectionPreservesOriginalRequestOnUncertainResponse()
    {
        var requests=new ConcurrentQueue<(string Body,string Key)>();
        PendingTemplateMutation? persisted=null;
        await using var server=await ApiPeer.StartAsync(WriteList,neighborHandler:async c=>
        {
            if(c.Request.Path=="/api/v1/capabilities") { await c.Response.WriteAsJsonAsync(new{data=new{capabilities=new[]{"neighbor_probe","neighbors_inspect_v1"}}}); return; }
            if(c.Request.Method=="GET") { await c.Response.WriteAsJsonAsync(new{data=new{items=Array.Empty<object>(),total=0}}); return; }
            var body=await new StreamReader(c.Request.Body).ReadToEndAsync();var key=c.Request.Headers["Idempotency-Key"].ToString();
            Assert.NotNull(persisted);Assert.Equal(persisted!.Body,body);Assert.Equal(persisted.Key,key);requests.Enqueue((body,key));
            if(requests.Count==1){c.Abort();return;}
            await c.Response.WriteAsJsonAsync(new{data=new{task_id="original-inspection"}});
        });
        await using var service=NewService();service.PersistStateAsync=()=>{persisted=service.Pending;return Task.CompletedTask;};
        await service.ConnectAsync(server.Origin);await UntilAsync(()=>service.Synchronized);
        var reference=new NeighborReference("reference","online",new ReferenceRegistration("FNR100","test",["neighbors_v1","neighbors_inspect_v1"]),new ReferenceSession("session-a"),9,null,null);
        await Assert.ThrowsAnyAsync<Exception>(()=>service.InspectNeighborsAsync(reference,true));
        var original=Assert.IsType<PendingTemplateMutation>(service.Pending);Assert.Contains("neighbor-inspections",original.Path);
        using(var body=JsonDocument.Parse(original.Body)){Assert.True(body.RootElement.GetProperty("vendor_test").GetBoolean());Assert.Equal((ulong)9,body.RootElement.GetProperty("config_revision").GetUInt64());}
        await service.RetryPendingAsync(true);Assert.Null(service.Pending);Assert.Equal(2,requests.Count);Assert.Equal(requests.First(),requests.Last());
    }

    [Fact]
    public async Task NeighborEditorRendersOfflineAccessibleFieldsAndPreciseErrors()
    {
        var js=new BrowserStorage();var files=new ProjectFiles();await using var publishing=NewService();
        await using var state=new EditorWorkspace(new TemplateCompiler(),files,new WorkspacePersistence(js,files),publishing,js);
        await state.InitializeAsync();state.Project.NeighborProbe=new(){FdbPreset="fnr100",Domains=[new(){Id="local",Interface="../bad"}]};state.Touch();
        using var services=new ServiceCollection().AddLogging().BuildServiceProvider();
        await using var renderer=new Microsoft.AspNetCore.Components.Web.HtmlRenderer(services,services.GetRequiredService<ILoggerFactory>());
        var html=await renderer.Dispatcher.InvokeAsync(async()=>{var rendered=await renderer.RenderComponentAsync<ProbeTemplateGenerator.Features.Attributes.NeighborEditor>(Microsoft.AspNetCore.Components.ParameterView.FromDictionary(new Dictionary<string,object?>{{"State",state}}));return rendered.ToHtmlString();});
        html=WebUtility.HtmlDecode(html);
        Assert.Contains("智能配置（推荐）",html);Assert.Contains("尚未设备验证",html);Assert.Contains("物理端口识别命令（高级）",html);Assert.Contains("所有应用该模板版本的设备",html);
        Assert.Contains("aria-invalid=",html);Assert.Contains("neighbor-domain-0-error",html);Assert.Contains("原始Linux接口名",html);Assert.Contains("清除型号预设（保留现有域）",html);Assert.DoesNotContain("邻居三层接口",html);
        Assert.Contains(state.Issues,i=>i.Field=="neighbor_probe.domains[0].interface");
    }

    private static WorkspacePersistence CreatePersistence() => new(new BrowserStorage(), new ProjectFiles());

    private static TemplatePublishingService NewService()
        => new(new HttpClient(new HttpClientHandler { AllowAutoRedirect = false, UseCookies = false }))
        { PersistStateAsync = () => Task.CompletedTask };

    private static PublishedTemplate Template(string id = "template-1") => new()
    {
        TemplateId = id, Version = 7, Name = "测试模板",
        Properties = new() { ["model"] = new() { Name = "型号", Command = "uname -m", TimeoutSeconds = 5 } }
    };

    private static Task WriteList(HttpContext context)
        => context.Response.WriteAsJsonAsync(new { data = new { items = new[] { Template() }, total = 1 } });

    private static async Task UntilAsync(Func<bool> condition)
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        while (!condition()) await Task.Delay(20, timeout.Token);
    }

    private sealed class BrowserStorage : IJSRuntime
    {
        public Dictionary<string, string> Values { get; } = [];
        public int Writes { get; private set; }
        public ValueTask<TValue> InvokeAsync<TValue>(string identifier, object?[]? args)
            => InvokeAsync<TValue>(identifier, CancellationToken.None, args);
        public ValueTask<TValue> InvokeAsync<TValue>(string identifier, CancellationToken cancellationToken, object?[]? args)
        {
            object? result = null;
            if (identifier == "workspace.storageGet") result = Values.GetValueOrDefault((string)args![0]!);
            else if (identifier == "workspace.storageSet")
            {
                Values[(string)args![0]!] = (string)args[1]!;
                Writes++;
            }
            else if(identifier=="workspace.setTheme") { /* Native HTML rendering does not execute browser theme scripts. */ }
            else throw new NotSupportedException(identifier);
            return ValueTask.FromResult(result is null ? default! : (TValue)result);
        }
    }

    private sealed class ApiPeer(WebApplication app, string origin) : IAsyncDisposable
    {
        public string Origin { get; } = origin;

        public static async Task<ApiPeer> StartAsync(RequestDelegate handler, Task? initialGate = null, bool closeFirstConnection = false, RequestDelegate? neighborHandler = null)
        {
            var builder = WebApplication.CreateBuilder(new WebApplicationOptions { EnvironmentName = "Testing" });
            builder.WebHost.UseKestrel().UseUrls("http://127.0.0.1:0");
            builder.Logging.ClearProviders();
            var app = builder.Build();
            app.UseWebSockets();
            var connections = 0;
            app.Map("/api/v1/events", async context =>
            {
                using var socket = await context.WebSockets.AcceptWebSocketAsync();
                var connection = Interlocked.Increment(ref connections);
                try
                {
                    if (initialGate is not null) await initialGate.WaitAsync(context.RequestAborted);
                    await socket.SendAsync(Encoding.UTF8.GetBytes("{\"type\":\"resync_required\"}"), WebSocketMessageType.Text,
                        true, context.RequestAborted);
                    if (closeFirstConnection && connection == 1)
                    {
                        await Task.Delay(100, context.RequestAborted);
                        await socket.CloseAsync(WebSocketCloseStatus.NormalClosure, "reconnect test", context.RequestAborted);
                        return;
                    }
                    var buffer = new byte[1024];
                    while (socket.State == WebSocketState.Open)
                        await socket.ReceiveAsync(buffer, context.RequestAborted);
                }
                catch (Exception ex) when (ex is WebSocketException or OperationCanceledException) { }
            });
            if(neighborHandler is not null) {
                app.Map("/api/v1/capabilities",neighborHandler);
                app.Map("/api/v1/devices",neighborHandler);
                app.Map("/api/v1/devices/{**path}",neighborHandler);
            }
            app.Map("/api/v1/probe-templates/{**path}", handler);
            app.Map("/api/v1/probe-templates", handler);
            await app.StartAsync();
            var origin = app.Services.GetRequiredService<IServer>().Features.Get<IServerAddressesFeature>()!.Addresses.Single();
            return new ApiPeer(app, origin);
        }

        public async ValueTask DisposeAsync()
        {
            await app.StopAsync(TimeSpan.FromSeconds(5));
            await app.DisposeAsync();
        }
    }
}
