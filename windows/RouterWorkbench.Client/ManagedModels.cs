namespace RouterWorkbench.Client;

public sealed record DisplayGroup(string Id,string Name,int Order);
public sealed record DisplayField(string GroupId,int Order,bool? Visible=null);
public sealed record Presentation(DisplayGroup[] Groups,Dictionary<string,DisplayField> Fields,Dictionary<string,bool>? BuiltinVisibility=null,bool? StorageVisible=null);
public sealed record MonitoringSettings(uint CpuSeconds=5,uint MemorySeconds=5,uint DiskSeconds=60,uint NetworkSeconds=5,uint EgressSeconds=600,[property:System.Text.Json.Serialization.JsonIgnore(Condition=System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull)]string? NetworkInterfaces=null);
public sealed record ProbeProperty(string Name,uint IntervalSeconds);
public sealed record InterfaceSampling(
 [property:System.Text.Json.Serialization.JsonIgnore(Condition=System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull)]uint? NetworkSeconds=null,
 [property:System.Text.Json.Serialization.JsonIgnore(Condition=System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull)]string? NetworkInterfaces=null);
public sealed record ProbeTemplate(string TemplateId,string Name,ulong Version,MonitoringSettings? Monitoring,Dictionary<string,ProbeProperty> Properties,Presentation? Presentation=null, System.Text.Json.JsonElement? NeighborProbe=null, CellularSettings? CellularProbe=null)
{
 public string? NeighborCapabilityError(IEnumerable<string> capabilities)
 {
  if (CellularProbe is not null&&!capabilities.Contains("cellular_identity_v1")) return "当前Probe不支持cellular_identity_v1；请更新Probe后再应用AT自动探测模板。";
  if (NeighborProbe is not { ValueKind: System.Text.Json.JsonValueKind.Object } settings) return null;
  if (!capabilities.Contains("neighbors_v1")) return "当前Probe不支持neighbors_v1；模板已保存，但设备应用需先更新Probe。";
  if (settings.TryGetProperty("fdb_preset", out var preset) && preset.GetString()=="fnr100" && !capabilities.Contains("neighbors_inspect_v1"))
   return "当前Probe不支持FNR100预设所需的neighbors_inspect_v1；请更新Probe，或移除预设后使用内核FDB。";
  return null;
 }
 public override string ToString()=>$"{Name} · v{Version} · {TemplateId}";
}
public sealed record DeviceModel(string ModelId,string Name,ulong Version,string[] Aliases,string TemplateId)
{public override string ToString()=>Name;}
public sealed record DeviceProfile(ulong Version,string Admission,string Name,string ModelId,string ModelName,MonitoringSettings? Monitoring,
 Dictionary<string,uint>? PropertyIntervals,ProbeTemplate? BoundTemplate,ulong DesiredRevision,string ConfigurationState,string? ConfigurationError,
 InterfaceSampling? InterfaceSampling=null,ulong TemplateGeneration=0,ProbeTemplate? LatestTemplate=null);

public sealed record ConnectionPeriod(ulong Id,string State,DateTimeOffset OnlineAt,DateTimeOffset? OfflineAt,
 DateTimeOffset? ReconnectedAt,long OnlineSeconds,long? OfflineSeconds,string EndReason,DateTimeOffset ObservedAt);
