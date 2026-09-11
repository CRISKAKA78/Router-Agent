using System.Text.Json.Serialization;
namespace RouterWorkbench.Client;

public sealed record NetworkSettings(bool Configured, string Engine, string Version, bool PackageConfigured, string[] Listeners, bool ManualRoutes, string[] Routes);
public sealed record OverlayMember(string DeviceId, string MachineId, string InstanceId, string VirtualIp, string Desired, ulong AppliedRevision, string? OperationId) { public MemberNetworkConfig? Config {get;init;} public ulong ConfigRevision {get;init;} public ulong AppliedConfigRevision {get;init;} }
public sealed record OverlayNetwork(string NetworkId, string Name, string Cidr, string[] PeerUrls, string[] Routes, ulong Revision, DateTimeOffset CreatedAt, OverlayMember[] Members) {public int Mtu {get;init;} public int Profile {get;init;} }
public sealed record NetworkOperation(string OperationId, string NetworkId, string DeviceId, string Action, ulong Revision, string State, string Step, string[] TaskIds, string? Error, DateTimeOffset CreatedAt, DateTimeOffset UpdatedAt)
{
 public string? SupersededBy {get;init;}
 [JsonIgnore] public string StateText => SupersededBy is {Length:>0}?"已被后续停止替代":NetworkLabels.State(State);
 [JsonIgnore] public string ActionText => Action=="start"?"加入 / 应用":"停止";
 [JsonIgnore] public string TimeText => UpdatedAt.ToLocalTime().ToString("MM-dd HH:mm:ss");
}
public sealed record NetworkLink(uint PeerId, string Transport, ulong? RxBytes, ulong? TxBytes, double? LatencyMs, double? LossRate) {public string? RemoteUrl {get;init;} }
public sealed record NetworkRoute(uint PeerId, string VirtualIp, [property:JsonPropertyName("next_hop_peer_id")] uint NextHopPeerId, int Cost, string[] ProxyCidrs) {public string Hostname {get;init;}=""; public NetworkNat? Nat {get;init;} public string InstanceId {get;init;}=""; }
public sealed record NetworkObservation(string DeviceId, string State, uint PeerId, string VirtualIp, string Version, string Interface, DateTimeOffset SampledAt, bool Stale, bool Limited, string? Error, NetworkLink[] Links, NetworkRoute[] Routes) {public string Hostname {get;init;}=""; public NetworkNat? Nat {get;init;} }
public sealed record NetworkNode(string Id, string? DeviceId, uint PeerId, string VirtualIp, string State, bool ManagementOnline, bool External) {public string Hostname {get;init;}=""; public NetworkNat? Nat {get;init;} }
public sealed record NetworkEdge(string Source, string Target, string Transport, string ReportedBy, DateTimeOffset ObservedAt, bool ConfirmedBoth, bool Stale, NetworkLink Link);
public sealed record NetworkTopology(NetworkNode[] Nodes, NetworkEdge[] Edges, NetworkObservation[] Observations, bool Limited)
{
 public static readonly NetworkTopology Empty=new([],[],[],false);
}
public static class NetworkLabels
{
 public static string State(string value)=>value switch{
 "unknown"=>"未知 / 已过期", "running"=>"引擎运行", "stopped"=>"引擎未运行", "error"=>"引擎异常", "observed_peer"=>"观测到的节点",
 "queued"=>"排队中", "succeeded"=>"已完成", "failed"=>"失败", "uncertain"=>"正在核实结果", "reconciled"=>"当前状态已核实", _=>value};
 public static string Error(string? value)=>value switch{
 "controller_unavailable"=>"EasyTier 配置服务不可用，请检查本机配置及设备接入",
 "package_or_controller_not_configured"=>"尚未配置 EasyTier 服务或仓库工具",
 "reconcile_required"=>"结果待核实，系统自动检查；可选择该操作查看详情",
 "server_restarted_reconcile_required"=>"服务已重启，正在核实当前状态；未重放历史任务",
 "persistence_failed"=>"状态落盘失败，请先核对服务器存储",
 "operation_failed"=>"操作失败，请查看关联任务结果",
 "controller_observation_unavailable"=>"未获得最新数据面观测", _=>value??""};
}

public sealed record NetworkNat(int Udp,int Tcp);
public sealed record MemberNetworkConfig(string Hostname,string VirtualIp,bool SystemForward,bool LazyP2p,bool NeedP2p,bool P2pOnly,bool DisableP2p,string[] ProxyCidrs,bool EnableManualRoutes,string[] Routes)
{
 public static MemberNetworkConfig Default(string name,string ip="")=>new(name,ip,true,false,false,false,false,[],true,[]);
}
public sealed record NetworkBatchResult(string DeviceId,NetworkOperation? Operation,string? Error);
public sealed record NetworkPassword(string Password);
