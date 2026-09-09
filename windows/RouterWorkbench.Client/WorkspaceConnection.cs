using System.Net.WebSockets;
using System.Text.Json;
using System.Threading.Channels;

namespace RouterWorkbench.Client;

public sealed class WorkspaceConnection : IAsyncDisposable
{
    private readonly CancellationTokenSource lifetime = new();
    private readonly Channel<bool> invalidations = Channel.CreateBounded<bool>(new BoundedChannelOptions(1) { FullMode = BoundedChannelFullMode.DropWrite });
    private readonly object gate = new();
    private readonly HashSet<Task> active = [];
    private Task[] workers = [];
    private ClientWebSocket? socket;
    private Task? disposal;
    private int generation;
    private bool ready;
    public ApiClient Api { get; }
    public CancellationToken Token => lifetime.Token;
    public Snapshot Snapshot { get; private set; } = Snapshot.Empty;
    public string Status { get; private set; } = "未连接";
    public string Error { get; private set; } = "";
    public bool Synchronized { get; private set; }
    public Mutation? Pending { get; private set; }
    public bool Busy { get; private set; }
    public event Action? Changed;
    public WorkspaceConnection(Uri origin, HttpMessageHandler? handler = null) => Api = new(origin, Token, handler);
    private void Emit() { if (!Token.IsCancellationRequested) Changed?.Invoke(); }
    public void Start()
    {
        if (workers.Length != 0) throw new InvalidOperationException("连接已经启动。");
        Status = "正在连接…"; workers = [SocketLoop(), RefreshLoop(), PeriodicLoop()]; Emit();
    }
    public void Invalidate() => invalidations.Writer.TryWrite(true);
    private async Task SocketLoop()
    {
        var failures = 0;
        while (!Token.IsCancellationRequested) {
            using var ws = new ClientWebSocket(); socket = ws;
            try {
                var uri = new UriBuilder(Api.Origin) { Scheme = Api.Origin.Scheme == "https" ? "wss" : "ws", Path = "/api/v1/events" }.Uri;
                using var firstDeadline = CancellationTokenSource.CreateLinkedTokenSource(Token); firstDeadline.CancelAfter(TimeSpan.FromSeconds(10));
                await ws.ConnectAsync(uri, firstDeadline.Token);
                var first = true; var buffer = new byte[8192];
                while (ws.State == WebSocketState.Open) {
                    var length = 0; WebSocketReceiveResult message;
                    do {
                        if (length == buffer.Length) throw new InvalidDataException("事件消息过大。");
                        message = await ws.ReceiveAsync(new ArraySegment<byte>(buffer, length, buffer.Length - length), first ? firstDeadline.Token : Token);
                        if (message.MessageType != WebSocketMessageType.Text) throw new IOException("事件连接已关闭。");
                        length += message.Count;
                    } while (!message.EndOfMessage);
                    using var json = JsonDocument.Parse(buffer.AsMemory(0, length));
                    var type = json.RootElement.GetProperty("type").GetString();
                    if (first && type != "resync_required") throw new InvalidDataException("缺少初始同步事件。");
                    if (type == "resync_required") {
                        first = false; failures = 0; Interlocked.Increment(ref generation); ready = true;
                        Synchronized = false; Status = "正在同步…"; Emit(); Invalidate();
                    } else if (type == "resource_changed") Invalidate();
                }
            } catch (Exception e) when (e is WebSocketException or IOException or OperationCanceledException or JsonException or InvalidOperationException) {
                if (!Token.IsCancellationRequested) Error = e.Message;
            } finally { ready = false; Interlocked.Increment(ref generation); Synchronized = false; socket = null; ws.Abort(); }
            if (Token.IsCancellationRequested) break;
            Status = "连接中断，正在重连…"; Emit();
            try { await Task.Delay(TimeSpan.FromSeconds(Math.Min(30, Math.Pow(2, Math.Min(failures++, 5)))), Token); } catch (OperationCanceledException) { break; }
        }
    }
    private async Task PeriodicLoop()
    {
        using var timer = new PeriodicTimer(TimeSpan.FromSeconds(5));
        try { while (await timer.WaitForNextTickAsync(Token)) Invalidate(); } catch (OperationCanceledException) { }
    }
    private async Task RefreshLoop()
    {
        try {
            await foreach (var _ in invalidations.Reader.ReadAllAsync(Token)) {
                if (!ready) continue;
                var epoch = Volatile.Read(ref generation);
                try {
                    var devices = await Api.ListAsync<Device>("devices?admission=all");
                    var tasks = await Api.ListAsync<TaskSummary>("tasks");
                    Asset[] assets = [];
                    var tools = await Api.ListAsync<Tool>("tools");
                    Maintenance[] maintenance = []; var maintenanceError = "";
                    try { maintenance = await Api.ListAsync<Maintenance>("maintenance"); }
                    catch (ApiException e) when (e.Code == "maintenance_disabled") { maintenanceError = "服务器未启用远程维护"; }
                    if (ready && epoch == Volatile.Read(ref generation) && !Token.IsCancellationRequested) {
                        Snapshot = new(devices, tasks, assets, tools, maintenance, maintenanceError, DateTimeOffset.Now);
                        Synchronized = true; Status = "已连接"; Error = ""; Emit();
                    }
                } catch (Exception e) when (!Token.IsCancellationRequested) {
                    Synchronized = false; Status = "快照刷新失败，正在恢复…"; Error = e.Message; Emit();
                }
            }
        } catch (OperationCanceledException) { }
    }
    public Task<T> TrackAsync<T>(Func<Task<T>> action)
    {
        lock (gate) {
            Token.ThrowIfCancellationRequested();
            var task = action(); active.Add(task);
            _ = task.ContinueWith(t => { lock (gate) active.Remove(t); }, CancellationToken.None, TaskContinuationOptions.ExecuteSynchronously, TaskScheduler.Default);
            return task;
        }
    }
    public Task<JsonElement> ExecuteAsync(Mutation mutation, bool retry = false) => TrackAsync(async () => {
        lock (gate) {
            if (Busy) { if (Pending != mutation) mutation.Dispose(); throw new InvalidOperationException("操作正在进行。"); }
            if (Pending != null && (!retry || Pending != mutation)) { if (Pending != mutation) mutation.Dispose(); throw new InvalidOperationException("请先处理响应不确定的原请求。"); }
            Busy = true; Pending = mutation;
        }
        Emit();
        try {
            var result = await Api.ExecuteAsync(mutation);
            Pending = null; mutation.Dispose(); Invalidate(); return result;
        } catch (ApiException) { Pending = null; mutation.Dispose(); throw; }
        finally { Busy = false; Emit(); }
    });
    public void Abandon()
    {
        lock (gate) {
            if (Busy) throw new InvalidOperationException("操作仍在进行。");
            Pending?.Dispose(); Pending = null;
        }
        Emit();
    }
    public ValueTask DisposeAsync()
    {
        lock (gate) { disposal ??= DisposeCore(); return new(disposal); }
    }
    private async Task DisposeCore()
    {
        lifetime.Cancel(); invalidations.Writer.TryComplete(); socket?.Abort();
        Task[] requests; lock (gate) requests = [.. active];
        try { await Task.WhenAll(workers.Concat(requests)); } catch (Exception) when (Token.IsCancellationRequested) { }
        Pending?.Dispose(); Pending = null; Api.Dispose();
        // Keep the canceled token source available to callers completing a late UI continuation.
    }
}
