using System.Net.Http.Headers;
using System.Net.WebSockets;
using System.Text;
using System.Text.Encodings.Web;
using System.Text.Json;
using System.Text.Json.Serialization;
using ProbeTemplateGenerator.Models;

namespace ProbeTemplateGenerator.Services;

public sealed class PublishedTemplate : RuntimeTemplate
{
    [JsonPropertyName("template_id")] public string TemplateId { get; set; } = "";
    [JsonPropertyName("version")] public ulong Version { get; set; }
}

public sealed record PublishingTarget(
    [property: JsonPropertyName("origin")] string Origin,
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("version")] ulong Version);

// Body is the original UTF-8 JSON text. It must never be deserialized and rebuilt on retry.
public sealed record PendingTemplateMutation(
    [property: JsonPropertyName("origin")] string Origin,
    [property: JsonPropertyName("label")] string Label,
    [property: JsonPropertyName("method")] string Method,
    [property: JsonPropertyName("path")] string Path,
    [property: JsonPropertyName("body")] string Body,
    [property: JsonPropertyName("key")] string Key);

public sealed class TemplateApiException(string code, string message, int status)
    : Exception($"{Meaning(code)} [{code}, HTTP {status}] {message}")
{
    public string Code { get; } = code;
    public int Status { get; } = status;

    private static string Meaning(string code) => code switch
    {
        "conflict" => "模板名称或版本冲突，请刷新列表并核对更新目标",
        "idempotency_conflict" => "原请求的幂等键发生冲突",
        "idempotency_capacity" => "服务器幂等账本已满",
        "not_found" => "模板不存在或已被删除",
        "invalid_request" => "输入不符合 API 契约",
        "capacity_exhausted" => "服务器模板容量不足",
        "operation_timeout" => "服务器操作超时",
        _ => "API 业务错误"
    };
}

/// <summary>One browser circuit's connection and template operations, using only the public API.</summary>
public sealed class TemplatePublishingService(HttpClient http) : IAsyncDisposable
{
    private static readonly JsonSerializerOptions Json = new(JsonSerializerDefaults.Web)
    {
        Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping
    };
    private readonly SemaphoreSlim lifecycle = new(1, 1);
    private readonly SemaphoreSlim writeGate = new(1, 1);
    private readonly SemaphoreSlim refreshGate = new(1, 1);
    private readonly object activeLock = new();
    private readonly HashSet<Task> active = [];
    private CancellationTokenSource? lifetime;
    private Task? eventLoop;
    private Task? periodicLoop;
    private bool socketReady;
    private bool disposed;
    private long generation;

    public string? Origin { get; private set; }
    public string Status { get; private set; } = "未连接";
    public string? Error { get; private set; }
    public bool Synchronized { get; private set; }
    public bool Busy { get; private set; }
    public IReadOnlyList<PublishedTemplate> Templates { get; private set; } = [];
    public PublishingTarget? Target { get; private set; }
    public PendingTemplateMutation? Pending { get; private set; }
    public event Action? Changed;

    // Set by the workspace, which owns the project alongside these publishing fields.
    // A request is sent only after its exact bytes/key are durably stored by this callback.
    public Func<Task>? PersistStateAsync { get; set; }

    public static string NormalizeOrigin(string value)
    {
        if (!Uri.TryCreate(value, UriKind.Absolute, out var uri) ||
            uri.Scheme is not ("http" or "https") || string.IsNullOrEmpty(uri.Host) ||
            uri.UserInfo.Length != 0 || uri.AbsolutePath != "/" ||
            uri.Query.Length != 0 || uri.Fragment.Length != 0)
            throw new ArgumentException("服务器地址须为 http(s)://主机:端口，不含账号、路径或查询");
        return uri.GetLeftPart(UriPartial.Authority);
    }

    public async Task ConnectAsync(string origin)
    {
        origin = NormalizeOrigin(origin);
        await lifecycle.WaitAsync();
        try
        {
            ObjectDisposedException.ThrowIf(disposed, this);
            if (Pending is not null && Pending.Origin != origin)
                throw new InvalidOperationException("请先连接原服务器并处理响应不确定的请求，再切换服务器");
            await DisconnectCoreAsync();
            Origin = origin;
            lifetime = new CancellationTokenSource();
            Status = "正在连接…";
            Error = null;
            Notify();
            eventLoop = WatchEventsAsync(origin, lifetime.Token);
            periodicLoop = RefreshPeriodicallyAsync(lifetime.Token);
        }
        finally { lifecycle.Release(); }
    }

    public async Task DisconnectAsync()
    {
        await lifecycle.WaitAsync();
        try { await DisconnectCoreAsync(); }
        finally { lifecycle.Release(); }
    }

    private async Task DisconnectCoreAsync()
    {
        var previous = lifetime;
        if (previous is null) return;
        lifetime = null;
        await previous.CancelAsync();
        socketReady = false;
        Synchronized = false;
        generation++;
        Task[] inFlight;
        lock (activeLock) inFlight = active.ToArray();
        await IgnoreFailuresAsync(Task.WhenAll(inFlight.Concat(new[] { eventLoop, periodicLoop }.OfType<Task>())));
        previous.Dispose();
        Origin = null;
        Templates = [];
        Status = "未连接";
        // Target and uncertain writes survive disconnect. They remain bound to their original origin.
        Notify();
    }

    public void Restore(PublishingTarget? target, PendingTemplateMutation? pending)
    {
        if (Busy || (Pending is not null && Pending != pending))
            throw new InvalidOperationException("请先处理当前响应不确定的请求");
        if (target is not null) ValidateTarget(target);
        if (pending is not null) ValidatePending(pending);
        Target = target;
        Pending = pending;
        Notify();
    }

    public void Bind(PublishedTemplate template)
    {
        EnsureWritable();
        ValidateTemplate(template);
        Target = new PublishingTarget(Origin!, template.TemplateId, template.Version);
        Notify();
    }

    public Task RefreshAsync()
    {
        var token = lifetime?.Token ?? throw new InvalidOperationException("请先连接服务器");
        return TrackAsync(() => RefreshCoreAsync(token));
    }

    private async Task RefreshCoreAsync(CancellationToken token)
    {
        await refreshGate.WaitAsync(token);
        var epoch = generation;
        var origin = Origin!;
        try
        {
            List<PublishedTemplate> items = [];
            for (var offset = 0; ;)
            {
                var page = await RequestAsync<TemplatePage>(origin,
                    $"probe-templates?offset={offset}&limit=200", HttpMethod.Get, null, null, token);
                if (page.Items is null || page.Total < 0)
                    throw new InvalidDataException("模板列表响应格式无效");
                foreach (var template in page.Items) ValidateTemplate(template);
                items.AddRange(page.Items);
                offset += page.Items.Count;
                if (offset >= page.Total || page.Items.Count == 0) break;
                if (offset >= 10_000) throw new InvalidDataException("列表超过 10000 项，快照未完成");
            }
            token.ThrowIfCancellationRequested();
            if (generation != epoch || Origin != origin) return;
            Templates = items;
            Synchronized = socketReady;
            Status = socketReady ? "已连接" : "等待连接恢复…";
            Error = null;
            Notify();
        }
        catch (Exception ex) when (!token.IsCancellationRequested)
        {
            if (generation == epoch && Origin == origin)
            {
                Synchronized = false;
                Status = "快照刷新失败，正在恢复…";
                Error = ex.Message;
                Notify();
            }
            throw;
        }
        finally { refreshGate.Release(); }
    }

    public async Task<PublishedTemplate> PublishAsync(RuntimeTemplate template, bool update)
    {
        EnsureWritable();
        if (update && (Target is null || Target.Origin != Origin))
            throw new InvalidOperationException("当前工程未绑定此服务器模板，请另存为新模板或选择更新目标");
        var body = update
            ? JsonSerializer.Serialize(new UpdateTemplate { Name = template.Name, Properties = template.Properties, Version = Target!.Version }, Json)
            : JsonSerializer.Serialize(template, Json);
        var request = NewMutation(update ? "更新模板" : "发布新模板", update ? "PUT" : "POST",
            update ? "probe-templates/" + Uri.EscapeDataString(Target!.Id) : "probe-templates", body);
        return (await ExecuteAsync(request, false))!;
    }

    public async Task DeleteAsync(PublishedTemplate template)
    {
        EnsureWritable();
        ValidateTemplate(template);
        var request = NewMutation("删除模板", "DELETE", "probe-templates/" + Uri.EscapeDataString(template.TemplateId),
            JsonSerializer.Serialize(new TemplateVersion(template.Version), Json));
        await ExecuteAsync(request, false);
    }

    public Task<PublishedTemplate?> RetryPendingAsync(bool serverRestartChecked)
    {
        if (!serverRestartChecked) throw new InvalidOperationException("请先核对服务器未重启，再重试原请求");
        var request = Pending ?? throw new InvalidOperationException("没有待核对的原请求");
        if (!Synchronized || Origin != request.Origin)
            throw new InvalidOperationException("请连接原服务器并完成 HTTP 快照同步后再重试");
        return ExecuteAsync(request, true);
    }

    public async Task AbandonPendingAsync(bool resultChecked)
    {
        if (!resultChecked) throw new InvalidOperationException("请先核对服务器执行结果；放弃不会撤销已完成的操作");
        if (!await writeGate.WaitAsync(0)) throw new InvalidOperationException("操作仍在进行");
        try
        {
            var original = Pending;
            Pending = null;
            try { await PersistAsync(); }
            catch { Pending = original; throw; }
            Notify();
        }
        finally { writeGate.Release(); }
    }

    private async Task<PublishedTemplate?> ExecuteAsync(PendingTemplateMutation request, bool retry)
    {
        if (!await writeGate.WaitAsync(0)) throw new InvalidOperationException("操作正在进行");
        try
        {
            if (retry)
            {
                if (Pending != request) throw new InvalidOperationException("只允许重试保留的原请求");
            }
            else EnsureWritable();
            var token = lifetime?.Token ?? throw new InvalidOperationException("连接已关闭");
            Busy = true;
            Pending = request;
            Notify();
            // A failed local save means nothing was sent; retain the draft for explicit recovery.
            await PersistAsync();
            return await TrackAsync(async () =>
            {
                try
                {
                    PublishedTemplate? published = null;
                    if (request.Method == "DELETE")
                    {
                        var deletion = await RequestAsync<DeletedTemplate>(request.Origin, request.Path,
                            HttpMethod.Delete, request.Body, request.Key, token);
                        if (!deletion.Deleted) throw new InvalidDataException("删除响应无法确认，原请求已保留");
                        if (Target?.Origin == request.Origin && request.Path == "probe-templates/" + Uri.EscapeDataString(Target.Id))
                            Target = null;
                    }
                    else
                    {
                        published = await RequestAsync<PublishedTemplate>(request.Origin, request.Path,
                            new HttpMethod(request.Method), request.Body, request.Key, token);
                        ValidateTemplate(published);
                        Target = new PublishingTarget(request.Origin, published.TemplateId, published.Version);
                    }
                    Pending = null;
                    try { await PersistAsync(); }
                    catch
                    {
                        // The browser still contains the request. Keep matching in-memory recovery state.
                        Pending = request;
                        throw;
                    }
                    if (!token.IsCancellationRequested) await IgnoreFailuresAsync(RefreshCoreAsync(token));
                    return published;
                }
                catch (TemplateApiException)
                {
                    Pending = null;
                    try { await PersistAsync(); }
                    catch { Pending = request; throw; }
                    throw;
                }
            });
        }
        finally
        {
            Busy = false;
            Notify();
            writeGate.Release();
        }
    }

    private PendingTemplateMutation NewMutation(string label, string method, string path, string body)
        => new(Origin!, label, method, path, body, Guid.NewGuid().ToString("N"));

    private void EnsureWritable()
    {
        if (Busy) throw new InvalidOperationException("操作正在进行");
        if (Pending is not null) throw new InvalidOperationException("请先处理响应不确定的原请求");
        if (!Synchronized || Origin is null || lifetime is null)
            throw new InvalidOperationException("请先连接服务器并完成 HTTP 快照同步");
    }

    private Task PersistAsync() => PersistStateAsync?.Invoke()
        ?? throw new InvalidOperationException("工程自动保存尚未就绪，未发送请求");

    private async Task<T> RequestAsync<T>(string origin, string path, HttpMethod method, string? body, string? key, CancellationToken token)
    {
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(token);
        deadline.CancelAfter(TimeSpan.FromSeconds(40));
        using var request = new HttpRequestMessage(method, origin + "/api/v1/" + path);
        request.Headers.CacheControl = new CacheControlHeaderValue { NoCache = true, NoStore = true };
        if (body is not null) request.Content = new StringContent(body, Encoding.UTF8, "application/json");
        if (key is not null) request.Headers.Add("Idempotency-Key", key);
        using var response = await http.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, deadline.Token);
        var bytes = await response.Content.ReadAsByteArrayAsync(deadline.Token);
        var envelope = JsonSerializer.Deserialize<ApiEnvelope<T>>(bytes, Json)
            ?? throw new InvalidDataException("API 响应无法解析");
        if (!response.IsSuccessStatusCode)
        {
            if (string.IsNullOrWhiteSpace(envelope.Error?.Code)) throw new InvalidDataException("API 错误响应无法解析");
            throw new TemplateApiException(envelope.Error.Code, envelope.Error.Message ?? "", (int)response.StatusCode);
        }
        return envelope.Data ?? throw new InvalidDataException("API 响应缺少 data");
    }

    private async Task WatchEventsAsync(string origin, CancellationToken token)
    {
        var failures = 0;
        while (!token.IsCancellationRequested)
        {
            using var socket = new ClientWebSocket();
            socket.Options.KeepAliveInterval = TimeSpan.FromSeconds(15);
            try
            {
                using var initial = CancellationTokenSource.CreateLinkedTokenSource(token);
                initial.CancelAfter(TimeSpan.FromSeconds(10));
                var url = new UriBuilder(origin) { Scheme = origin.StartsWith("https:", StringComparison.Ordinal) ? "wss" : "ws", Path = "/api/v1/events" };
                await socket.ConnectAsync(url.Uri, initial.Token);
                var first = true;
                while (!token.IsCancellationRequested)
                {
                    var notification = await ReadEventAsync(socket, first ? initial.Token : token);
                    if (first && notification.Type != "resync_required") throw new InvalidDataException("首条通知应要求重新同步");
                    if (notification.Type == "resync_required")
                    {
                        first = false;
                        failures = 0;
                        generation++;
                        socketReady = true;
                        Synchronized = false;
                        Status = "正在同步 HTTP 快照…";
                        Notify();
                    }
                    if (notification.Type is "resync_required" or "resource_changed")
                        await IgnoreFailuresAsync(RefreshCoreAsync(token));
                }
            }
            catch (Exception ex) when (!token.IsCancellationRequested)
            {
                socketReady = false;
                Synchronized = false;
                generation++;
                Status = "连接中断，正在重连…";
                Error = ex is WebSocketException ? "服务器连接中断" : ex.Message;
                Notify();
            }
            catch (OperationCanceledException) when (token.IsCancellationRequested) { break; }
            finally { socket.Abort(); }
            try { await Task.Delay(TimeSpan.FromSeconds(Math.Min(30, Math.Pow(2, Math.Min(failures++, 5)))), token); }
            catch (OperationCanceledException) { break; }
        }
    }

    private static async Task<TemplateEvent> ReadEventAsync(ClientWebSocket socket, CancellationToken token)
    {
        var buffer = new byte[8192];
        var count = 0;
        WebSocketReceiveResult frame;
        do
        {
            frame = await socket.ReceiveAsync(new ArraySegment<byte>(buffer, count, buffer.Length - count), token);
            if (frame.MessageType != WebSocketMessageType.Text) throw new WebSocketException("通知连接已关闭");
            count += frame.Count;
            if (count >= buffer.Length) throw new InvalidDataException("通知超过大小限制");
        } while (!frame.EndOfMessage);
        return JsonSerializer.Deserialize<TemplateEvent>(buffer.AsSpan(0, count), Json)
            ?? throw new InvalidDataException("通知格式无效");
    }

    private async Task RefreshPeriodicallyAsync(CancellationToken token)
    {
        using var timer = new PeriodicTimer(TimeSpan.FromSeconds(5));
        try
        {
            while (await timer.WaitForNextTickAsync(token))
                if (socketReady) await IgnoreFailuresAsync(RefreshCoreAsync(token));
        }
        catch (OperationCanceledException) when (token.IsCancellationRequested) { }
    }

    private async Task<T> TrackAsync<T>(Func<Task<T>> action)
    {
        Task<T> task;
        lock (activeLock)
        {
            if (lifetime is null) throw new InvalidOperationException("连接已关闭");
            task = action();
            active.Add(task);
        }
        try { return await task; }
        finally { lock (activeLock) active.Remove(task); }
    }

    private Task TrackAsync(Func<Task> action) => TrackAsync(async () => { await action(); return true; });
    private static async Task IgnoreFailuresAsync(Task task) { try { await task; } catch { /* Owning operation records its state. */ } }
    private void Notify() => Changed?.Invoke();

    public static void ValidateTarget(PublishingTarget target)
    {
        if (NormalizeOrigin(target.Origin) != target.Origin || string.IsNullOrWhiteSpace(target.Id) ||
            Encoding.UTF8.GetByteCount(target.Id) > 128 || target.Version == 0)
            throw new InvalidDataException("草稿发布目标格式无效");
    }

    public static void ValidatePending(PendingTemplateMutation pending)
    {
        if (NormalizeOrigin(pending.Origin) != pending.Origin || string.IsNullOrWhiteSpace(pending.Label) ||
            pending.Key is null || pending.Key.Length is < 1 or > 128 || pending.Key.Any(c => c < 33 || c > 126) ||
            pending.Body is null || Encoding.UTF8.GetByteCount(pending.Body) > 65536)
            throw new InvalidDataException("草稿原请求格式无效；请保留原文件并核对服务器结果");
        var itemPath = pending.Path is not null && pending.Path.StartsWith("probe-templates/", StringComparison.Ordinal) &&
            pending.Path.Length > "probe-templates/".Length && !pending.Path.Contains('?') &&
            !pending.Path.Contains('#') && !pending.Path["probe-templates/".Length..].Contains('/');
        if (!((pending.Method == "POST" && pending.Path == "probe-templates") ||
            (pending.Method is "PUT" or "DELETE" && itemPath)))
            throw new InvalidDataException("原请求不是受支持的模板 API 操作");
        using var body = JsonDocument.Parse(pending.Body);
        if (body.RootElement.ValueKind != JsonValueKind.Object) throw new InvalidDataException("原请求必须是 JSON 对象");
    }

    private static void ValidateTemplate(PublishedTemplate template)
    {
        if (string.IsNullOrWhiteSpace(template.TemplateId) || template.Version == 0 ||
            string.IsNullOrWhiteSpace(template.Name) || template.Properties is null)
            throw new InvalidDataException("服务器模板响应格式无效");
    }

    public async ValueTask DisposeAsync()
    {
        await lifecycle.WaitAsync();
        try
        {
            if (disposed) return;
            disposed = true;
            await DisconnectCoreAsync();
            http.Dispose();
        }
        finally { lifecycle.Release(); }
    }

    private sealed class ApiEnvelope<T> { public T? Data { get; set; } public ApiFailure? Error { get; set; } }
    private sealed class ApiFailure { public string Code { get; set; } = ""; public string? Message { get; set; } }
    private sealed class TemplatePage { public List<PublishedTemplate>? Items { get; set; } public int Total { get; set; } }
    private sealed class DeletedTemplate { public bool Deleted { get; set; } }
    private sealed class TemplateEvent { public string Type { get; set; } = ""; }
    private sealed class UpdateTemplate : RuntimeTemplate { [JsonPropertyName("version")] public ulong Version { get; set; } }
    private sealed record TemplateVersion([property: JsonPropertyName("version")] ulong Version);
}
