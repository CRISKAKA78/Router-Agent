using System.Net.WebSockets;
using System.Text.Json;
using System.Threading.Channels;

namespace RouterWorkbench.Core;

// A connection owns every HTTP request and WebSocket worker. No domain state
// transitions are reconstructed here; events only invalidate HTTP snapshots.
public sealed class WorkspaceConnection : IAsyncDisposable
{
    private readonly CancellationTokenSource lifetime = new();
    private readonly Channel<bool> refresh = Channel.CreateBounded<bool>(new BoundedChannelOptions(1) {
        FullMode = BoundedChannelFullMode.DropWrite, SingleReader = true });
    private readonly object gate = new();
    private readonly HashSet<Task> operations = [];
    private readonly ApiClient api;
    private readonly TimeSpan reconnectDelay;
    private Task? workers;
    private Task? disposal;
    private bool closing;
    private bool mutationBusy;
    private Mutation? pending;
    private volatile bool socketConnected;
    private int socketGeneration;
    private int syncedGeneration;
    public bool IsSynchronized => socketConnected && Volatile.Read(ref syncedGeneration) == Volatile.Read(ref socketGeneration);
    public event Action<Snapshot>? SnapshotChanged;
    public event Action<string>? ConnectionChanged;
    public event Action<string>? RefreshFailed;
    public Mutation? Pending { get { lock (gate) return pending; } }
    public WorkspaceConnection(ServerProfile profile, HttpMessageHandler? handler = null, TimeSpan? reconnectDelay = null)
    {
        api = new ApiClient(profile.BaseUri(), handler);
        this.reconnectDelay = reconnectDelay ?? TimeSpan.FromSeconds(1);
    }
    public void Start()
    {
        lock (gate)
        {
            ObjectDisposedException.ThrowIf(closing, this);
            if (workers != null) return;
            workers = Task.WhenAll(Task.Run(EventsAsync), Task.Run(RefreshAsync), Task.Run(PeriodicAsync));
        }
    }
    public void Invalidate() => refresh.Writer.TryWrite(true);
    public Task<T> ReadAsync<T>(Func<ApiClient, CancellationToken, Task<T>> operation) => Track(() => operation(api, lifetime.Token));
    public Task SaveAssetAsync(Asset asset, string target) => Track(async () => { await api.SaveAssetAsync(asset, target, lifetime.Token); return true; });
    public Task<JsonElement> ExecuteAsync(Mutation mutation, bool retry = false)
    {
        lock (gate)
        {
            ObjectDisposedException.ThrowIf(closing, this);
            if (mutationBusy) throw new InvalidOperationException("操作正在进行，请等待当前响应。");
            if (pending != null && (!retry || !ReferenceEquals(mutation, pending)))
                throw new InvalidOperationException("上次响应不确定，请先核对或重试原请求。");
            mutationBusy = true;
            pending = mutation;
        }
        return Track(async () =>
        {
            try
            {
                var data = await api.ExecuteAsync(mutation, lifetime.Token).ConfigureAwait(false);
                lock (gate) pending = null;
                Invalidate();
                return data;
            }
            catch (ApiException) { lock (gate) pending = null; throw; }
            // Any response/transport failure after sending may hide a committed action.
            // Keep the same key and bytes for an explicit, same-server-process retry.
            finally { lock (gate) mutationBusy = false; }
        });
    }
    public void AbandonPending() { lock (gate) { if (mutationBusy) throw new InvalidOperationException("操作仍在进行"); pending = null; } }
    private Task<T> Track<T>(Func<Task<T>> action)
    {
        lock (gate)
        {
            ObjectDisposedException.ThrowIf(closing, this);
            var task = Task.Run(action);
            operations.Add(task);
            _ = task.ContinueWith(done => { lock (gate) operations.Remove(done); }, CancellationToken.None,
                TaskContinuationOptions.ExecuteSynchronously, TaskScheduler.Default);
            return task;
        }
    }
    private async Task EventsAsync()
    {
        var failures = 0;
        while (!lifetime.IsCancellationRequested)
        {
            using var socket = new ClientWebSocket();
            socket.Options.KeepAliveInterval = TimeSpan.FromSeconds(15);
            socket.Options.KeepAliveTimeout = TimeSpan.FromSeconds(15);
            using var abort = lifetime.Token.Register(socket.Abort);
            try
            {
                ConnectionChanged?.Invoke(failures == 0 ? "正在连接 Server…" : "实时连接中断，正在重连…");
                var uri = new UriBuilder(new Uri(api.BaseUri, "api/v1/events")) { Scheme = api.BaseUri.Scheme == "https" ? "wss" : "ws" }.Uri;
                using var connect = CancellationTokenSource.CreateLinkedTokenSource(lifetime.Token);
                connect.CancelAfter(TimeSpan.FromSeconds(10));
                await socket.ConnectAsync(uri, connect.Token).ConfigureAwait(false);
                var buffer = new byte[8192];
                var first = true;
                while (!lifetime.IsCancellationRequested)
                {
                    var count = 0;
                    WebSocketReceiveResult result;
                    do
                    {
                        using var receive = CancellationTokenSource.CreateLinkedTokenSource(lifetime.Token);
                        if (first) receive.CancelAfter(TimeSpan.FromSeconds(10));
                        result = await socket.ReceiveAsync(new ArraySegment<byte>(buffer, count, buffer.Length - count), receive.Token).ConfigureAwait(false);
                        if (result.MessageType != WebSocketMessageType.Text) throw new WebSocketException("实时连接已关闭或消息无效");
                        count += result.Count;
                        if (count == buffer.Length && !result.EndOfMessage) throw new InvalidDataException("WebSocket 消息过大");
                    } while (!result.EndOfMessage);
                    using var doc = JsonDocument.Parse(buffer.AsMemory(0, count));
                    var type = doc.RootElement.GetProperty("type").GetString();
                    if (first && type != "resync_required") throw new InvalidDataException("缺少首次同步通知");
                    if (type == "resync_required")
                    {
                        Interlocked.Increment(ref socketGeneration);
                        socketConnected = true;
                        first = false;
                        failures = 0;
                        ConnectionChanged?.Invoke("实时连接已建立，正在获取 HTTP 快照…");
                        Invalidate();
                    }
                    else if (type == "resource_changed") Invalidate();
                }
            }
            catch (Exception e) when (e is WebSocketException or HttpRequestException or OperationCanceledException or JsonException or InvalidDataException)
            {
                socketConnected = false;
                if (lifetime.IsCancellationRequested) break;
                ConnectionChanged?.Invoke("Server 不可达或 WebSocket 中断；快照可能过期，正在重连。");
            }
            try { await Task.Delay(TimeSpan.FromMilliseconds(Math.Min(30000, reconnectDelay.TotalMilliseconds * Math.Pow(2, Math.Min(failures++, 5)))), lifetime.Token).ConfigureAwait(false); }
            catch (OperationCanceledException) { break; }
        }
    }
    private async Task RefreshAsync()
    {
        try
        {
            await foreach (var _ in refresh.Reader.ReadAllAsync(lifetime.Token).ConfigureAwait(false))
            {
                if (!socketConnected) continue;
                var generation = Volatile.Read(ref socketGeneration);
                try
                {
                    // One snapshot worker; an invalidation arriving during fetch
                    // remains queued and triggers another fetch afterwards.
                    var devices = await api.ListAsync<Device>("devices", lifetime.Token).ConfigureAwait(false);
                    var tasks = await api.ListAsync<TaskSummary>("tasks", lifetime.Token).ConfigureAwait(false);
                    var assets = await api.ListAsync<Asset>("assets?include_archived=true", lifetime.Token).ConfigureAwait(false);
                    var tools = await api.ListAsync<Tool>("tools?include_archived=true", lifetime.Token).ConfigureAwait(false);
                    Maintenance[] maintenance = [];
                    string? maintenanceError = null;
                    try { maintenance = await api.ListAsync<Maintenance>("maintenance", lifetime.Token).ConfigureAwait(false); }
                    catch (ApiException e) when (e.Code == "maintenance_disabled") { maintenanceError = Errors.Describe(e, "远程维护"); }
                    if (socketConnected && generation == Volatile.Read(ref socketGeneration) && !lifetime.IsCancellationRequested)
                    {
                        Volatile.Write(ref syncedGeneration, generation);
                        SnapshotChanged?.Invoke(new(devices, tasks, assets, tools, maintenance, DateTimeOffset.UtcNow, maintenanceError));
                        ConnectionChanged?.Invoke("已连接 · HTTP 快照已同步 · 实时更新开启");
                    }
                }
                catch (OperationCanceledException) when (lifetime.IsCancellationRequested) { break; }
                catch (Exception e) when (e is HttpRequestException or OperationCanceledException or ApiException or JsonException or InvalidDataException)
                {
                    Volatile.Write(ref syncedGeneration, -1);
                    RefreshFailed?.Invoke(Errors.Describe(e, "快照刷新"));
                    ConnectionChanged?.Invoke("快照刷新失败；当前显示可能过期，正在恢复。");
                }
            }
        }
        catch (OperationCanceledException) { }
    }
    private async Task PeriodicAsync()
    {
        using var timer = new PeriodicTimer(TimeSpan.FromSeconds(5));
        try { while (await timer.WaitForNextTickAsync(lifetime.Token).ConfigureAwait(false)) Invalidate(); }
        catch (OperationCanceledException) { }
    }
    public ValueTask DisposeAsync()
    {
        lock (gate)
        {
            if (disposal != null) return new(disposal);
            closing = true;
            lifetime.Cancel();
            refresh.Writer.TryComplete();
            disposal = StopAsync(workers ?? Task.CompletedTask, operations.ToArray());
            return new(disposal);
        }
    }
    private async Task StopAsync(Task background, Task[] active)
    {
        try { await Task.WhenAll(active.Append(background)).ConfigureAwait(false); }
        catch (Exception) when (lifetime.IsCancellationRequested) { /* Individual requests report to their callers. */ }
        finally { api.Dispose(); lifetime.Dispose(); socketConnected = false; }
    }
}
