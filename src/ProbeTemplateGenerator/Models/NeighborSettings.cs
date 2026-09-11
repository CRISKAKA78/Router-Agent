using System.Text.Json.Serialization;
namespace ProbeTemplateGenerator.Models;
public sealed class NeighborSettings
{
 [JsonPropertyName("interval_seconds")] public int IntervalSeconds {get;set;}=30;
 [JsonPropertyName("domains")] public List<NeighborDomainSettings> Domains {get;set;}=[];
 [JsonPropertyName("fdb_command"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public string? FdbCommand {get;set;}
 [JsonPropertyName("fdb_preset"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public string? FdbPreset {get;set;}
 public NeighborSettings Copy()=>new(){FdbPreset=FdbPreset,IntervalSeconds=IntervalSeconds,FdbCommand=FdbCommand,Domains=Domains.Select(d=>d.Copy()).ToList()};
}
public sealed class NeighborDomainSettings
{
 [JsonPropertyName("id")] public string Id {get;set;}= "local";
 [JsonPropertyName("scope")] public string Scope {get;set;}="broadcast";
 [JsonPropertyName("interface")] public string Interface {get;set;}="";
 [JsonPropertyName("lease_file"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public string? LeaseFile {get;set;}
 [JsonPropertyName("ports"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public List<string>? Ports {get;set;}
 public NeighborDomainSettings Copy()=>new(){Id=Id,Scope=Scope,Interface=Interface,LeaseFile=LeaseFile,Ports=Ports?.ToList()};
}
