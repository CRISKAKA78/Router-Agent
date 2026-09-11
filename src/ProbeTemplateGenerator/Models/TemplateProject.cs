using System.Text.Json.Serialization;

namespace ProbeTemplateGenerator.Models;

[JsonConverter(typeof(JsonStringEnumConverter<AttributeVisibility>))]
public enum AttributeVisibility
{
    [JsonStringEnumMemberName("display")] Display,
    [JsonStringEnumMemberName("virtual")] Virtual
}

[JsonConverter(typeof(JsonStringEnumConverter<AttributeSource>))]
public enum AttributeSource
{
    [JsonStringEnumMemberName("command")] Command,
    [JsonStringEnumMemberName("nvram")] Nvram,
    [JsonStringEnumMemberName("uci")] Uci,
    [JsonStringEnumMemberName("expression")] Expression,
    [JsonStringEnumMemberName("rules")] Rules
}

public sealed class MonitoringSettings
{
 [JsonPropertyName("egress_seconds")] public int EgressSeconds {get;set;}=600;
 [JsonPropertyName("network_interfaces"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public string? NetworkInterfaces {get;set;}

 public MonitoringSettings Copy() => new(){CpuSeconds=CpuSeconds,MemorySeconds=MemorySeconds,DiskSeconds=DiskSeconds,NetworkSeconds=NetworkSeconds,EgressSeconds=EgressSeconds,NetworkInterfaces=NetworkInterfaces};
 [JsonPropertyName("cpu_seconds")] public int CpuSeconds {get;set;}=5;
 [JsonPropertyName("memory_seconds")] public int MemorySeconds {get;set;}=5;
 [JsonPropertyName("disk_seconds")] public int DiskSeconds {get;set;}=60;
 [JsonPropertyName("network_seconds")] public int NetworkSeconds {get;set;}=5;
}

public sealed class TemplateProject
{
 [JsonPropertyName("presentation"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public PresentationSettings? Presentation {get;set;}
 [JsonPropertyName("cellular_probe"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public CellularSettings? CellularProbe {get;set;}
 [JsonPropertyName("neighbor_probe"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public NeighborSettings? NeighborProbe {get;set;}
 [JsonPropertyName("switch_probe"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public SwitchSettings? SwitchProbe {get;set;}
    [JsonPropertyName("format")] public string Format { get; set; } = "router-agent-template-project";
    [JsonPropertyName("schema_version")] public int SchemaVersion { get; set; } = 8;
    [JsonPropertyName("monitoring"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public MonitoringSettings? Monitoring {get;set;}
    [JsonPropertyName("name")] public string Name { get; set; } = "新模板";
    [JsonPropertyName("attributes")] public List<TemplateAttribute> Attributes { get; set; } = [];
}

public sealed class TemplateAttribute
{
    [JsonPropertyName("interval_seconds")] public int IntervalSeconds {get;set;}
    [JsonPropertyName("id")] public string Id { get; set; } = Guid.NewGuid().ToString();
    [JsonPropertyName("key")] public string Key { get; set; } = "";
    [JsonPropertyName("name")] public string Name { get; set; } = "";
    [JsonPropertyName("visibility")] public AttributeVisibility Visibility { get; set; }
    [JsonPropertyName("source")] public AttributeSource Source { get; set; }
    [JsonPropertyName("input")] public string Input { get; set; } = "";
    [JsonPropertyName("timeout")] public double Timeout { get; set; } = 5;
    [JsonPropertyName("sample")] public string Sample { get; set; } = "";
    [JsonPropertyName("rules"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public List<ResultRule>? Rules { get; set; }
    [JsonPropertyName("fallback"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? Fallback { get; set; }
}

public sealed class ResultRule
{
    [JsonPropertyName("condition")] public string Condition { get; set; } = "";
    [JsonPropertyName("value")] public string Value { get; set; } = "";
}

public class RuntimeTemplate
{
 [JsonPropertyName("presentation"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public PresentationSettings? Presentation {get;set;}
 [JsonPropertyName("cellular_probe"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public CellularSettings? CellularProbe {get;set;}
 [JsonPropertyName("neighbor_probe"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public NeighborSettings? NeighborProbe {get;set;}
 [JsonPropertyName("switch_probe"),JsonIgnore(Condition=JsonIgnoreCondition.WhenWritingNull)] public SwitchSettings? SwitchProbe {get;set;}
    [JsonPropertyName("monitoring"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public MonitoringSettings? Monitoring {get;set;}
    [JsonPropertyName("name")] public string Name { get; set; } = "";
    [JsonPropertyName("properties")] public Dictionary<string, RuntimeProperty> Properties { get; set; } = new(StringComparer.Ordinal);
}

public sealed class RuntimeProperty
{
    [JsonPropertyName("interval_seconds"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingDefault)] public int IntervalSeconds {get;set;}
    [JsonPropertyName("name")] public string Name { get; set; } = "";
    [JsonPropertyName("command"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? Command { get; set; }
    [JsonPropertyName("source"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? Source { get; set; }
    [JsonPropertyName("key"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? Key { get; set; }
    [JsonPropertyName("timeout_seconds")] public int TimeoutSeconds { get; set; } = 5;
}

public sealed record PreviewResult(string Key, string Name, string? Value = null, string? Error = null, int? MatchedRule = null, bool UsesRules = false);
public sealed record ValidationIssue(string? AttributeId, string Field, string Message);
