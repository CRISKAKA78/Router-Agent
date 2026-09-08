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

public sealed class TemplateProject
{
    [JsonPropertyName("format")] public string Format { get; set; } = "router-agent-template-project";
    [JsonPropertyName("schema_version")] public int SchemaVersion { get; set; } = 2;
    [JsonPropertyName("name")] public string Name { get; set; } = "新模板";
    [JsonPropertyName("attributes")] public List<TemplateAttribute> Attributes { get; set; } = [];
}

public sealed class TemplateAttribute
{
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
    [JsonPropertyName("name")] public string Name { get; set; } = "";
    [JsonPropertyName("properties")] public Dictionary<string, RuntimeProperty> Properties { get; set; } = new(StringComparer.Ordinal);
}

public sealed class RuntimeProperty
{
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
