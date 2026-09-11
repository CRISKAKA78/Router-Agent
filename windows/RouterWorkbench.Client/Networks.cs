using System.Text.Json.Serialization;
namespace RouterWorkbench.Client;

public sealed record NetworkSettings(bool Configured, string Engine, string Version, bool PackageConfigured, string[] Listeners, bool ManualRoutes, string[] Routes);
public sealed record OverlayMember(string DeviceId, string MachineId, string InstanceId, string VirtualIp, string Desired, ulong AppliedRevision, string? OperationId);
public sealed record OverlayNetwork(string NetworkId, string Name, string Cidr, string[] PeerUrls, string[] Routes, ulong Revision, DateTimeOffset CreatedAt, OverlayMember[] Members);
public sealed record NetworkOperation(string OperationId, string NetworkId, string DeviceId, string Action, ulong Revision, string State, string Step, string[] TaskIds, string? Error, DateTimeOffset CreatedAt, DateTimeOffset UpdatedAt)
{
 [JsonIgnore] public string StateText => NetworkLabels.State(State);
 [JsonIgnore] public string ActionText => Action=="start"?"加入 / 应用":"停止";
 [JsonIgnore] public string TimeText => UpdatedAt.ToLocalTime().ToString("MM-dd HH:mm:ss");
}
public sealed record NetworkLink(uint PeerId, string Transport, ulong? RxBytes, ulong? TxBytes, double? LatencyMs, double? LossRate);
public sealed record NetworkRoute(uint PeerId, string VirtualIp, [property:JsonPropertyName("next_hop")] uint NextHopPeerId, int Cost, string[] ProxyCidrs);
public sealed record NetworkObservation(string DeviceId, string State, uint PeerId, string VirtualIp, string Version, string Interface, DateTimeOffset SampledAt, bool Stale, bool Limited, string? Error, NetworkLink[] Links, NetworkRoute[] Routes);
public sealed record NetworkNode(string Id, string? DeviceId, uint PeerId, string VirtualIp, string State, bool ManagementOnline, bool External);
public sealed record NetworkEdge(string Source, string Target, string Transport, string ReportedBy, DateTimeOffset ObservedAt, bool ConfirmedBoth, bool Stale, NetworkLink Link);
public sealed record NetworkTopology(NetworkNode[] Nodes, NetworkEdge[] Edges, NetworkObservation[] Observations, bool Limited)
{
 public static readonly NetworkTopology Empty=new([],[],[],false);
}
public static class NetworkLabels
{
 public static string State(string value)=>value switch{
 "unknown"=>"未知 / 已过期", "running"=>"引擎运行", "stopped"=>"引擎未运行", "error"=>"引擎异常", "observed_peer"=>"观测到的节点",
 "queued"=>"排队中", "succeeded"=>"已完成并回查", "failed"=>"失败", "uncertain"=>"结果不确定，需回查", "reconciled"=>"已回查（非完成证明）", _=>value};
 public static string Error(string? value)=>value switch{
 "controller_unavailable"=>"EasyTier 配置服务不可用，请检查本机配置及设备接入",
 "package_or_controller_not_configured"=>"尚未配置 EasyTier 服务或仓库工具",
 "reconcile_required"=>"结果不确定；请保留原操作并先回查",
 "server_restarted_reconcile_required"=>"服务重启，未重放旧任务；请先回查",
 "persistence_failed"=>"状态落盘失败，请先核对服务器存储",
 "operation_failed"=>"操作失败，请查看关联任务结果",
 "controller_observation_unavailable"=>"未获得最新数据面观测", _=>value??""};
}
