using System.Text.Json;
using System.Text.Json.Serialization;
using RouterWorkbench.Core;

namespace RouterWorkbench.Client;

public static class ApiJson
{
    public static readonly JsonSerializerOptions Options = new() { PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower };
    public static string Pretty(object value) => JsonSerializer.Serialize(value, new JsonSerializerOptions(Options) { WriteIndented = true });
}
public sealed record TemplateReference(string TemplateId, string Name, ulong Version);
public sealed record Metric(string Name, string Value, string Unit, string Status, string Entity, uint IntervalSeconds, string Source, string Group, DateTimeOffset SampledAt, bool Stale, string? Reason = null);
public sealed record DeviceRuntime(long? UptimeSeconds, DateTimeOffset ReportedAt);
public sealed record Registration(string DeviceId, string Hostname, string Serial, string Model, string Firmware,
    string ProbeVersion, string Arch, string Kernel, string Libc, string BootId, string[] Capabilities);
public sealed record DeviceSession(string SessionId, Registration Registration, DateTimeOffset StartedAt,
    DateTimeOffset LastSeenAt, DateTimeOffset? EndedAt, string EndReason, DeviceRuntime? Runtime = null, string? SourceIp = null, Dictionary<string, Metric>? EffectiveMetrics = null)
{
    [JsonIgnore] public string StartedText => Labels.Time(StartedAt);
}
public sealed record Device(string DeviceId, Registration Registration, string Status, DeviceSession? CurrentSession,
    DeviceSession? LatestSession, DateTimeOffset? FirstSeenAt, DateTimeOffset? LastSeenAt,
    DateTimeOffset? LastOnlineAt, DateTimeOffset? LastOfflineAt, long TotalSessions, long EvictedSessions, DeviceRuntime? Runtime = null, string? SourceIp = null, Dictionary<string, Metric>? EffectiveMetrics = null, DeviceProfile? Profile=null, Presentation? Presentation=null, TemplateReference? ActiveTemplate=null, ulong AppliedRevision=0)
{
    [JsonIgnore] public string DeviceName => Profile?.Name ?? (string.IsNullOrEmpty(Registration.Hostname) ? "—" : Registration.Hostname);
    [JsonIgnore] public string DisplayName => Profile?.Name ?? (string.IsNullOrEmpty(Registration.Hostname) ? DeviceId : Registration.Hostname);
 [JsonIgnore] public bool Managed => Profile?.Admission=="managed";
    [JsonIgnore] public string StatusText => Labels.State(Status);
    [JsonIgnore] public bool Online => Status == "online";
}
public record TaskSummary(string TaskId, string DeviceId, string Type, string State, DateTimeOffset CreatedAt)
{
    [JsonIgnore] public string StateText => Labels.State(State);
    [JsonIgnore] public string TypeText => Labels.Type(Type);
    [JsonIgnore] public string CreatedText => Labels.Time(CreatedAt);
}
public sealed record TaskResult(string Status, int ExitCode, string Stdout, string Stderr, bool Truncated,
    DateTimeOffset? StartedAt, DateTimeOffset? FinishedAt);
public sealed record TaskDetail(string TaskId, string DeviceId, string Type, string State, DateTimeOffset CreatedAt,
    string Command, string Cwd, uint TimeoutSeconds, string? LastSessionId, int DispatchCount, TaskResult? Result,
    Dictionary<string, string>? Env, [property: JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingDefault)] JsonElement Params) : TaskSummary(TaskId, DeviceId, Type, State, CreatedAt);
public sealed record Transfer(string TaskId, string TransferId, bool Committed, bool Released, bool Failed, long Size, string Sha256);
public sealed record Operation(string TaskId, string TransferId, string DeviceId, string SessionId, string ToolId, string Version, string ArtifactId, string AssetId);
public sealed record Asset(string AssetId, string Name, long Size, string Sha256, bool Archived, DateTimeOffset CreatedAt)
{
    [JsonIgnore] public string SizeText => Labels.Bytes(Size);
    [JsonIgnore] public string StateText => Archived ? "已归档" : "可用";
}
public sealed record Tool(string ToolId, string Name, string Description, bool Archived, DateTimeOffset CreatedAt)
{
    [JsonIgnore] public string StateText => Archived ? "已归档" : "可用";
}
public sealed record Rules(string[] Arch, string[] Libc, string[]? Models, string[]? Kernels, string[]? RequiredCapabilities);
public sealed record Artifact(string ArtifactId, string AssetId, string Platform, string Mode, Rules Rules);
public sealed record ToolVersion(string ToolId, string Version, Artifact[] Artifacts, bool Archived, DateTimeOffset CreatedAt)
{
    public override string ToString() => Version;
    [JsonIgnore] public string CreatedText => Labels.Time(CreatedAt);
    [JsonIgnore] public string StateText => Archived ? "已归档" : "可用";
    [JsonIgnore] public int Count => Artifacts.Length;
}
public sealed record CompatibilityCheck(string Field, string Status, string Reason);
public sealed record Compatibility(Artifact Artifact, string Status, CompatibilityCheck[] Checks)
{
    [JsonIgnore] public string StatusText => Labels.State(Status);
    [JsonIgnore] public string Reason => string.Join("；", Checks.Select(c => $"{c.Field}: {c.Reason}"));
}
public sealed record Maintenance(string MaintenanceId, string DeviceId, string SessionId, string State, string Reason,
    DateTimeOffset CreatedAt, DateTimeOffset ExpiresAt, bool Released, DateTimeOffset? ReusableAfter, int Connections, Endpoint[] Endpoints)
{
    [JsonIgnore] public string StateText => Labels.State(State);
    [JsonIgnore] public string ExpiresText => Labels.Time(ExpiresAt);
    [JsonIgnore] public string CreatedText => Labels.Time(CreatedAt);
}
public sealed record Snapshot(Device[] Devices, TaskSummary[] Tasks, Asset[] Assets, Tool[] Tools,
    Maintenance[] Maintenance, string MaintenanceError, DateTimeOffset? FetchedAt)
{
    public static Snapshot Empty { get; } = new([], [], [], [], [], "", null);
}
public sealed class PropertyRow(string group, string name, string value, string tip = "") : System.ComponentModel.INotifyPropertyChanged
{
    private string currentValue = value;
    private string currentTip = tip;
    public string ValueTip { get => currentTip; set { if(value==currentTip)return;currentTip=value;PropertyChanged?.Invoke(this,new(nameof(ValueTip))); } }
    public string Group { get; set; } = group;
    public string GroupId { get; set; } = "other";
    public string MetricGroup { get; set; } = "";
 public string Key { get; set; } = "";
    public string Name { get; } = name;
    public string Value {
        get => currentValue;
        set { if (value == currentValue) return; currentValue = value; PropertyChanged?.Invoke(this, new(nameof(Value))); }
    }
    public event System.ComponentModel.PropertyChangedEventHandler? PropertyChanged;
}
public sealed record ActivityRow(string Time, string Level, string Message);
public static class Labels
{
    public static string State(string s) => s switch {
        "online" => "在线", "offline" => "离线", "ready" => "就绪", "closing" => "关闭中", "closed" => "已关闭",
        "created" => "已创建", "queued" => "排队中", "dispatched" => "已派发", "acknowledged" => "已确认",
        "received" => "已接收", "accepted" => "已接受", "running" => "执行中", "success" => "成功", "succeeded" => "成功", "completed" => "已完成",
        "failed" => "失败", "timeout" => "超时", "timed_out" => "超时", "rejected" => "已拒绝",
        "compatible" => "兼容", "incompatible" => "不兼容", "unknown" => "无法判定", _ => s };
    public static string Type(string s) => s switch { "exec" => "命令", "router_config" => "配置", "upload" => "上传", "download" => "下载", _ => s };
    public static string Time(DateTimeOffset? value) => value?.ToLocalTime().ToString("yyyy-MM-dd HH:mm:ss") ?? "—";
    public static string Bytes(long size) => size < 1024 ? $"{size} B" : size < 1048576 ? $"{size / 1024d:0.#} KB" : $"{size / 1048576d:0.##} MB";
}
