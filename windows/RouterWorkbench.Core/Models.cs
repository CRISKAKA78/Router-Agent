using System.Text.Json;

namespace RouterWorkbench.Core;

public static class Wire
{
    public static readonly JsonSerializerOptions Json = new() { PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower };
    public static string Segment(string value) => Uri.EscapeDataString(value);
}
public sealed record Page<T>(T[] Items, int Total, int Offset, int Limit);
public sealed record Registration(string DeviceId, string Serial, string Model, string Firmware, string ProbeVersion,
    string Hostname, string Arch, string Kernel, string Libc, string BootId, string[] Capabilities);
public sealed record Session(string SessionId, Registration Registration, DateTimeOffset StartedAt,
    DateTimeOffset LastSeenAt, DateTimeOffset? EndedAt, string EndReason);
public sealed record Device(string DeviceId, Registration Registration, string Status, Session? CurrentSession,
    Session? LatestSession, DateTimeOffset? FirstSeenAt, DateTimeOffset? LastSeenAt,
    DateTimeOffset? LastOnlineAt, DateTimeOffset? LastOfflineAt, long TotalSessions, long EvictedSessions);
public sealed record TaskSummary(string TaskId, string DeviceId, string Type, string State, DateTimeOffset CreatedAt);
public sealed record ExecResult(string TaskId, string Status, int ExitCode, string Stdout, string Stderr,
    bool Truncated, DateTimeOffset StartedAt, DateTimeOffset FinishedAt);
public sealed record TaskDetail(string TaskId, string DeviceId, string Type, string State, string Command, string Cwd,
    uint TimeoutSeconds, string? LastSessionId, int DispatchCount, ExecResult? Result);
public sealed record Transfer(string TaskId, string TransferId, bool Committed, bool Released, bool Failed, long Size, string Sha256);
public sealed record Asset(string AssetId, string Name, long Size, string Sha256, bool Archived, DateTimeOffset CreatedAt);
public sealed record Tool(string ToolId, string Name, string Description, bool Archived, DateTimeOffset CreatedAt);
public sealed record Rules(string[] Arch, string[] Libc, string[]? Models = null, string[]? Kernels = null,
    string[]? RequiredCapabilities = null);
public sealed record Artifact(string ArtifactId, string AssetId, string Platform, string Mode, Rules Rules);
public sealed record ToolVersion(string ToolId, string Version, Artifact[] Artifacts, bool Archived, DateTimeOffset CreatedAt);
public sealed record MatchCheck(string Field, string Status, string Reason);
public sealed record Compatibility(Artifact Artifact, string Status, MatchCheck[] Checks);
public sealed record Endpoint(string Service, string Host, int Port, string Address, string State, string? Url);
public sealed record Maintenance(string MaintenanceId, string DeviceId, string SessionId, string State, string Reason,
    DateTimeOffset CreatedAt, DateTimeOffset ExpiresAt, bool Released, DateTimeOffset? ReusableAfter, int Connections, Endpoint[] Endpoints);
public sealed record Snapshot(Device[] Devices, TaskSummary[] Tasks, Asset[] Assets, Tool[] Tools,
    Maintenance[] Maintenance, DateTimeOffset FetchedAt, string? MaintenanceError = null);

public sealed class ApiException(string code, string message, int status = 0) : Exception(message)
{
    public string Code { get; } = code;
    public int Status { get; } = status;
}
public static class Errors
{
    public static string Summary(Exception error, string operation) => error is ApiException e
        ? $"{operation}：{Meaning(e.Code, operation)}。" : Describe(error, operation);
    public static string Describe(Exception error, string operation) => error switch
    {
        ApiException e => $"{operation}：{Meaning(e.Code, operation)} [{e.Code}, HTTP {e.Status}] {e.Message}",
        HttpRequestException => $"{operation}：服务器不可达或连接中断，请检查地址和网络。",
        OperationCanceledException => $"{operation}：请求超时或连接已关闭。",
        _ => $"{operation}：{error.Message}"
    };
    private static string Meaning(string code, string operation) => code switch
    {
        "device_offline" => "设备离线",
        "session_changed" => "会话已替换或当前会话不支持维护，请刷新设备",
        "capacity_exhausted" when operation.Contains("维护") => "维护端口池或服务器容量不足",
        "capacity_exhausted" => "服务器容量不足",
        "idempotency_capacity" => "服务器幂等账本已满，请联系管理员",
        "conflict" when operation.Contains("维护") => "维护创建失败：设备可能已有维护，请查看当前维护",
        "incompatible" => "工具产物不兼容或无法唯一选择",
        "maintenance_disabled" => "服务器未启用远程维护",
        "not_found" => "资源不存在、已被清理或服务器已重启",
        "invalid_request" => "输入不符合 API 契约",
        "operation_timeout" => "服务器操作超时",
        _ => "API 业务错误"
    };
}
