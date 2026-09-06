using System.Net;
using System.Net.Sockets;
using System.Net.WebSockets;
using System.Text.Json;
using RouterWorkbench.Core;

namespace RouterWorkbench.Tests;

internal sealed class MockEvents : IAsyncDisposable
{
    private readonly HttpListener listener = new();
    private readonly CancellationTokenSource stop = new();
    private readonly List<Task> work = [];
    private readonly object gate = new();
    private readonly Task serving;
    public WebSocket? Socket;
    public int Connections;
    public int DeviceReads;
    public TaskCompletionSource? HoldDevices;
    public string DeviceId = "old";
    public Uri Uri { get; }
    public MockEvents()
    {
        var port = FreePort(); Uri = new($"http://127.0.0.1:{port}/"); listener.Prefixes.Add(Uri.AbsoluteUri); listener.Start(); serving = ServeAsync();
    }
    public static int FreePort() { var l = new TcpListener(IPAddress.Loopback, 0); l.Start(); var p = ((IPEndPoint)l.LocalEndpoint).Port; l.Stop(); return p; }
    private async Task ServeAsync()
    {
        try { while (!stop.IsCancellationRequested) { var ctx = await listener.GetContextAsync(); lock (gate) work.Add(HandleAsync(ctx)); } }
        catch (Exception e) when (e is HttpListenerException or ObjectDisposedException) { }
    }
    private async Task HandleAsync(HttpListenerContext ctx)
    {
        try
        {
            if (ctx.Request.IsWebSocketRequest)
            {
                var upgraded = await ctx.AcceptWebSocketAsync(null); var socket = upgraded.WebSocket; Socket = socket; Interlocked.Increment(ref Connections);
                await NotifyAsync("resync_required", socket);
                var buffer = new byte[128];
                while (!stop.IsCancellationRequested && socket.State == WebSocketState.Open) await socket.ReceiveAsync(buffer, stop.Token);
                socket.Dispose(); return;
            }
            object[] items = [];
            if (ctx.Request.Url!.AbsolutePath.EndsWith("/devices"))
            {
                Interlocked.Increment(ref DeviceReads); var id = DeviceId;
                if (HoldDevices is { } hold) await hold.Task.WaitAsync(stop.Token);
                items = [new { device_id = id, status = "online", registration = new { hostname = "fixture", model = "", capabilities = Array.Empty<string>() } }];
            }
            var bytes = JsonSerializer.SerializeToUtf8Bytes(new { data = new { items, total = items.Length, offset = 0, limit = 200 } });
            ctx.Response.ContentType = "application/json"; await ctx.Response.OutputStream.WriteAsync(bytes, stop.Token); ctx.Response.Close();
        }
        catch (Exception e) when (e is WebSocketException or HttpListenerException or IOException or OperationCanceledException or ObjectDisposedException)
        { try { ctx.Response.Abort(); } catch (ObjectDisposedException) { } }
    }
    public async Task NotifyAsync(string type = "resource_changed", WebSocket? socket = null)
    {
        var bytes = JsonSerializer.SerializeToUtf8Bytes(new { type, topic = "devices", sequence = "1" });
        await (socket ?? Socket!).SendAsync(bytes, WebSocketMessageType.Text, true, stop.Token);
    }
    public async ValueTask DisposeAsync()
    {
        stop.Cancel(); Socket?.Abort(); listener.Close(); await serving;
        Task[] active; lock (gate) active = work.ToArray(); await Task.WhenAll(active); stop.Dispose();
    }
}
