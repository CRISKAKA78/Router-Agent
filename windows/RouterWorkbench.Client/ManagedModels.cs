namespace RouterWorkbench.Client;

public sealed record DisplayGroup(string Id,string Name,int Order);
public sealed record DisplayField(string GroupId,int Order,bool? Visible=null);
public sealed record Presentation(DisplayGroup[] Groups,Dictionary<string,DisplayField> Fields,Dictionary<string,bool>? BuiltinVisibility=null,bool? StorageVisible=null);
public sealed record MonitoringSettings(uint CpuSeconds=5,uint MemorySeconds=5,uint DiskSeconds=60,uint NetworkSeconds=5,uint EgressSeconds=600,[property:System.Text.Json.Serialization.JsonIgnore(Condition=System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull)]string? NetworkInterfaces=null);
public sealed record ProbeProperty(string Name,uint IntervalSeconds);
public sealed record InterfaceSampling(
 [property:System.Text.Json.Serialization.JsonIgnore(Condition=System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull)]uint? NetworkSeconds=null,
 [property:System.Text.Json.Serialization.JsonIgnore(Condition=System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull)]string? NetworkInterfaces=null);
public sealed record ProbeTemplate(string TemplateId,string Name,ulong Version,MonitoringSettings? Monitoring,Dictionary<string,ProbeProperty> Properties,Presentation? Presentation=null)
{public override string ToString()=>$"{Name} · v{Version} · {TemplateId}";}
public sealed record DeviceModel(string ModelId,string Name,ulong Version,string[] Aliases,string TemplateId)
{public override string ToString()=>Name;}
public sealed record DeviceProfile(ulong Version,string Admission,string Name,string ModelId,string ModelName,MonitoringSettings? Monitoring,
 Dictionary<string,uint>? PropertyIntervals,ProbeTemplate? BoundTemplate,ulong DesiredRevision,string ConfigurationState,string? ConfigurationError,
 InterfaceSampling? InterfaceSampling=null,ulong TemplateGeneration=0,ProbeTemplate? LatestTemplate=null);

public sealed record ConnectionPeriod(ulong Id,string State,DateTimeOffset OnlineAt,DateTimeOffset? OfflineAt,
 DateTimeOffset? ReconnectedAt,long OnlineSeconds,long? OfflineSeconds,string EndReason,DateTimeOffset ObservedAt);
