using System.Text.Json.Serialization;
namespace ProbeTemplateGenerator.Models;
public sealed class CellularSettings
{
 [JsonPropertyName("interval_seconds")] public int IntervalSeconds {get;set;}=30;
 [JsonPropertyName("telemetry"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingDefault)] public bool Telemetry {get;set;}
 [JsonPropertyName("details"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingDefault)] public bool Details {get;set;}
 public CellularSettings Copy()=>new(){IntervalSeconds=IntervalSeconds,Telemetry=Telemetry,Details=Details};
}
