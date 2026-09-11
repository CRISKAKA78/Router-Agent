using System.Text.Json.Serialization;
namespace ProbeTemplateGenerator.Models;
public sealed class CellularSettings
{
 [JsonPropertyName("interval_seconds")] public int IntervalSeconds {get;set;}=30;
 public CellularSettings Copy()=>new(){IntervalSeconds=IntervalSeconds};
}
