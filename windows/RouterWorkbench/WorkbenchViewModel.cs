using System.Text.Json;
using RouterWorkbench.Core;

namespace RouterWorkbench;

// Presentation coordinator. The server owns all business states; a notification
// invalidates HTTP data. One coordinator lives for the entire window, not per page.
public sealed class WorkbenchViewModel(Action<Action> dispatch) : IAsyncDisposable
{
    public event Action? Changed;
    public WorkspaceConnection? Connection { get; private set; }
    public Snapshot? Snapshot { get; private set; }
    public string? DeviceId { get; private set; }
    public Device? Device => Snapshot?.Devices.FirstOrDefault(d => d.DeviceId == DeviceId);
    public Maintenance? Maintenance => Snapshot?.Maintenance.Where(m => m.DeviceId == DeviceId)
        .OrderByDescending(m => m.CreatedAt).FirstOrDefault();
    public ServerProfile Profile { get; private set; } = new();
    public string ConnectionStatus { get; private set; } = "未连接";
    public string Notice { get; private set; } = "";
    public bool IsError { get; private set; }
    public string ErrorDetails { get; private set; } = "";
    public bool Busy { get; private set; }
    public bool Closing { get; private set; }
    public bool Synchronized => Connection?.IsSynchronized == true;
    public bool CanWrite => Synchronized && !Busy && !Closing && Connection?.Pending == null;
    public bool Online => Device is { Status: "online", CurrentSession: not null };
    public bool CanOpen => CanWrite && Online && Maintenance is { State: "ready", Released: false } m &&
        m.SessionId == Device?.CurrentSession?.SessionId && m.ExpiresAt > DateTimeOffset.UtcNow;
    public bool CanCreate => CanWrite && Online && Snapshot?.MaintenanceError == null &&
        !((Snapshot?.Maintenance ?? []).Any(m => m.DeviceId == DeviceId && !m.Released && m.State != "closed"));
    private readonly CancellationTokenSource lifetime = new();
    private Task? currentOperation;
    private DateTimeOffset lastAction;
    private string lastActionLabel = "";
    public CancellationToken Lifetime => lifetime.Token;
    public void SelectDevice(string? id) { if (DeviceId == id) return; DeviceId = id; Changed?.Invoke(); }
    public void SetNotice(string text, bool error = false) { Notice = text; IsError = error; Changed?.Invoke(); }
    public void SetError(Exception error, string operation) { ErrorDetails = Errors.Describe(error, operation); SetNotice(Errors.Summary(error, operation), true); }
    public void Tick() { if (Maintenance is { State: not "closed" } m && m.ExpiresAt <= DateTimeOffset.UtcNow) Connection?.Invalidate(); Changed?.Invoke(); }
    public async Task ConnectAsync(ServerProfile profile)
    {
        profile.BaseUri();
        var old = Connection; Connection = null; Snapshot = null; DeviceId = null;
        Profile = profile; ConnectionStatus = "正在连接"; Notice = ""; Changed?.Invoke();
        if (old != null) await old.DisposeAsync();
        if (Closing) return;
        var owner = new WorkspaceConnection(profile); Connection = owner;
        void Post(Action action) => dispatch(() => { if (!Closing && ReferenceEquals(owner, Connection)) action(); });
        owner.SnapshotChanged += value => Post(() => {
            Snapshot = value;
            if (!value.Devices.Any(d => d.DeviceId == DeviceId)) DeviceId = value.Devices.FirstOrDefault()?.DeviceId;
            Changed?.Invoke();
        });
        owner.ConnectionChanged += value => Post(() => { ConnectionStatus = value; Changed?.Invoke(); });
        owner.RefreshFailed += value => Post(() => SetNotice(value, true));
        owner.Start();
    }
    public async Task DisconnectAsync()
    {
        var old = Connection; Connection = null; Snapshot = null; DeviceId = null;
        ConnectionStatus = "未连接"; Notice = ""; Changed?.Invoke();
        if (old != null) await old.DisposeAsync();
    }
    // The gate includes dialogs and file pickers, so repeated clicks cannot open
    // multiple dialogs or queue mutations. Read workers remain independent.
    public Task RunAsync(string label, Func<Task> action)
    {
        if (Busy || Closing || label == lastActionLabel && DateTimeOffset.UtcNow - lastAction < TimeSpan.FromMilliseconds(400)) return Task.CompletedTask;
        Busy = true; lastAction = DateTimeOffset.UtcNow; lastActionLabel = label; Changed?.Invoke();
        return currentOperation = RunCoreAsync(label, action);
    }
    private async Task RunCoreAsync(string label, Func<Task> action)
    {
        try { await action(); }
        catch (Exception e) { if (!Closing) SetError(e, label); }
        finally { Busy = false; if (!Closing) Changed?.Invoke(); }
    }
    public WorkspaceConnection RequireConnection() => Synchronized && Connection != null ? Connection : throw new InvalidOperationException("服务器尚未同步，请等待连接恢复。");
    public string RequireDevice() => Online && DeviceId != null ? DeviceId : throw new InvalidOperationException("请选择在线设备。");
    public async Task<JsonElement> ExecuteAsync(Mutation request, bool retry = false)
    {
        var owner = RequireConnection();
        var result = await owner.ExecuteAsync(request, retry);
        if (ReferenceEquals(owner, Connection) && !Closing)
            SetNotice(result.TryGetProperty("dispatch_uncertain", out var uncertain) && uncertain.GetBoolean()
                ? "请求已记录，派发结果尚不确定。请在任务页核对原任务。" : request.Label + "请求已由服务器处理。");
        return result;
    }
    public async ValueTask DisposeAsync()
    {
        if (Closing) return;
        Closing = true; lifetime.Cancel();
        await DisconnectAsync();
        if (currentOperation != null) await currentOperation;
        lifetime.Dispose();
    }
}

public sealed record DeviceItem(Device Value, string MaintenanceStatus)
{
    public string Id => Value.DeviceId;
    public string Name => string.IsNullOrWhiteSpace(Value.Registration.Hostname) ? Id : Value.Registration.Hostname;
    public string Status => Display.State(Value.Status);
    public string Subtitle => Display.Join(Value.Registration.Model, Id);
}
public sealed record ListItem(string Id, string Title, string Subtitle, object Value);
public static class Display
{
    public static string Join(params string[] values) { var text = string.Join(" · ", values.Where(v => !string.IsNullOrWhiteSpace(v))); return text.Length == 0 ? "未上报" : text; }
    public static string CheckField(string value) => value switch { "artifact" => "产物", "platform" => "平台", "arch" => "架构", "libc" => "运行库", "model" => "型号", "kernel" => "内核", _ => value.StartsWith("capability:") ? "所需能力：" + value[11..] : value };
    public static string CheckReason(string value) => value switch {
        "invalid artifact constraints" => "产物规则无效", "existing Probe platform is linux" => "当前探针平台为 Linux",
        "explicitly unrestricted" => "明确不限制此项", "device did not declare constrained field" => "设备未声明此项信息",
        "allowed value" => "符合允许值", "declared value is not allowed" => "声明值不在允许范围内", "declared" => "已声明此能力",
        "required capability not declared" => "未声明所需能力", _ => value };
    public static string State(string value) => value switch {
        "online" => "在线", "offline" => "离线", "ready" => "已开启", "closed" => "已关闭",
        "opening" or "creating" => "正在开启", "closing" => "正在关闭", "pending" => "等待处理",
        "created" => "已创建", "sent" => "已发送", "acked" or "accepted" or "received" => "已接收", "queued" => "排队中", "running" => "执行中",
        "completed" => "已完成", "success" => "成功", "failed" => "失败", "rejected" => "已拒绝",
        "timeout" or "timed_out" => "已超时", "cancelled" => "已取消", "compatible" => "兼容",
        "incompatible" => "不兼容", "unknown" => "无法确定", "exec" => "命令执行", "upload" => "文件上传",
        "download" => "文件下载", "expired" => "租期到期", "session_replaced" => "会话已替换",
        "device_offline" => "设备离线", "operator_closed" or "manual" or "requested" or "requested_disconnect" => "主动关闭",
        "session_ended" => "会话已结束", "replaced" => "会话已替换", "disconnected" => "连接断开",
        "heartbeat_timeout" => "心跳超时", "write_error" => "发送失败", "protocol_error" => "协议错误", "server_closed" => "服务器已关闭",
        "" => "—", _ => "服务器状态：" + value };
    public static string Size(long bytes) => bytes >= 1048576 ? $"{bytes / 1048576d:0.##} MB" : bytes >= 1024 ? $"{bytes / 1024d:0.#} KB" : $"{bytes} 字节";
    public static string Yes(bool value) => value ? "是" : "否";
    public static string Remaining(Maintenance? m)
    {
        if (m == null || m.State == "closed") return "按需开启，结束后自动回收入口";
        var remaining = m.ExpiresAt - DateTimeOffset.UtcNow;
        return remaining > TimeSpan.Zero ? $"剩余 {(int)remaining.TotalHours:00}:{remaining.Minutes:00}:{remaining.Seconds:00}" : "租期已到，正在向服务器确认";
    }
}
