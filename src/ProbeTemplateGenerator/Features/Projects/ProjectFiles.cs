using System.Text;
using System.Text.Encodings.Web;
using System.Text.Json;
using System.Text.Json.Serialization;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;

namespace ProbeTemplateGenerator.Features.Projects;

/// <summary>Current editable project and runtime template file boundary.</summary>
public sealed class ProjectFiles
{
    public const int MaxFileBytes = 1024 * 1024;
    private static readonly JsonSerializerOptions ReadOptions = new() { MaxDepth = 64 };
    private static readonly JsonSerializerOptions PrettyOptions = new() { WriteIndented = true, Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping };
    private static readonly JsonSerializerOptions CompactOptions = new() { Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping };

    public string SerializeProject(TemplateProject project) => JsonSerializer.Serialize(project, PrettyOptions);
    public string SerializeTemplate(RuntimeTemplate template, bool indented = true) => JsonSerializer.Serialize(template, indented ? PrettyOptions : CompactOptions);

    public TemplateProject ReadProject(string json)
    {
        if (Encoding.UTF8.GetByteCount(json) > MaxFileBytes) throw new InvalidOperationException("工程文件不能超过 1 MiB");
        ImportedFile file;
        try { file = JsonSerializer.Deserialize<ImportedFile>(json, ReadOptions) ?? throw new JsonException(); }
        catch (JsonException error) { throw new InvalidOperationException("不是有效的工程或模板 JSON，或工程属性格式不正确", error); }
        if (file.Format != "router-agent-template-project") return ReadTemplate(file);
        if (file.SchemaVersion != 8 || file.Name is null || file.Attributes is null || file.Attributes.Count > 128)
            throw new InvalidOperationException("不支持的工程格式或版本");
        var attributes = new List<TemplateAttribute>();
        foreach (var row in file.Attributes)
        {
            if (row is null || row.Key is null || row.Name is null || row.Input is null || row.Sample is null ||
                row.Visibility is not ("display" or "virtual") || row.Timeout is null ||
                row.Source is not ("command" or "nvram" or "uci" or "expression" or "rules"))
                throw new InvalidOperationException("工程属性格式不正确");
            if ((row.Source == "rules" || row.HasRules) && (row.Rules is null || row.Rules.Count > 32 || row.Rules.Any(rule => rule?.Condition is null || rule.Value is null)))
                throw new InvalidOperationException("工程结果规则格式不正确");
            if ((row.Source == "rules" || row.HasFallback) && row.Fallback is null)
                throw new InvalidOperationException("工程默认显示文本格式不正确");
            attributes.Add(new TemplateAttribute
            {
                IntervalSeconds=row.IntervalSeconds, Key = row.Key, Name = row.Name, Input = row.Input, Sample = row.Sample, Timeout = row.Timeout.Value,
                Source = Enum.Parse<AttributeSource>(row.Source, ignoreCase: true),
                Visibility = Enum.Parse<AttributeVisibility>(row.Visibility, ignoreCase: true),
                Rules = row.Rules?.Select(rule => new ResultRule { Condition = rule!.Condition!, Value = rule.Value! }).ToList(),
                Fallback = row.Rules is not null ? row.Fallback : null
            });
        }
        // Invalid/incomplete draft values remain editable; compilation performs semantic validation.
        return new TemplateProject { Name = file.Name, Attributes = attributes, Presentation=file.Presentation,SwitchProbe=file.SwitchProbe,Monitoring=file.Monitoring };
    }

    public TemplateProject FromTemplate(RuntimeTemplate template)
    {
        var project = TemplateCompiler.EmptyProject();
        project.Presentation=template.Presentation?.Copy();project.SwitchProbe=template.SwitchProbe?.Copy();
 project.Name = template.Name; project.Monitoring=template.Monitoring?.Copy();
        foreach (var (key, property) in template.Properties)
        {
            if (property is null || property.Source is not (null or "command" or "nvram" or "uci"))
                throw new InvalidOperationException($"属性 {key} 格式无效");
            var source = property.Source ?? "command";
            var input = source == "command" ? property.Command : property.Key;
            if (input is null) throw new InvalidOperationException($"属性 {key} 缺少来源");
            project.Attributes.Add(new TemplateAttribute
            {
                IntervalSeconds=property.IntervalSeconds, Key = key, Name = property.Name, Source = Enum.Parse<AttributeSource>(source, true), Input = input,
                Timeout = property.TimeoutSeconds == 0 ? 5 : property.TimeoutSeconds
            });
        }
        new TemplateCompiler().Compile(project);
        return project;
    }

    private static TemplateProject ReadTemplate(ImportedFile file)
    {
        if (file.Name is null || file.Properties is null) throw new InvalidOperationException("不是有效的模板 JSON");
        var project = new TemplateProject { Name = file.Name,Presentation=file.Presentation,SwitchProbe=file.SwitchProbe,Monitoring=file.Monitoring };
        foreach (var (key, property) in file.Properties)
        {
            if (property?.Name is null || property.Source is not (null or "command" or "nvram" or "uci"))
                throw new InvalidOperationException($"属性 {key} 格式无效");
            var source = property.Source ?? "command";
            var input = source == "command" ? property.Command : property.Key;
            if (input is null) throw new InvalidOperationException($"属性 {key} 缺少来源");
            project.Attributes.Add(new TemplateAttribute
            {
                IntervalSeconds=property.IntervalSeconds, Key = key, Name = property.Name, Source = Enum.Parse<AttributeSource>(source, true), Input = input,
                Timeout = property.TimeoutSeconds is null or 0 ? 5 : property.TimeoutSeconds.Value
            });
        }
        new TemplateCompiler().Compile(project);
        return project;
    }

    // Nullable import records distinguish missing/invalid fields from valid empty draft values.
    private sealed class ImportedFile
    {
 [JsonPropertyName("presentation")] public PresentationSettings? Presentation{get;set;}
 [JsonPropertyName("switch_probe")] public SwitchSettings? SwitchProbe{get;set;}
        [JsonPropertyName("monitoring")] public MonitoringSettings? Monitoring {get;set;}
        [JsonPropertyName("format")] public string? Format { get; set; }
        [JsonPropertyName("schema_version")] public double? SchemaVersion { get; set; }
        [JsonPropertyName("name")] public string? Name { get; set; }
        [JsonPropertyName("attributes")] public List<ImportedAttribute?>? Attributes { get; set; }
        [JsonPropertyName("properties")] public Dictionary<string, ImportedProperty?>? Properties { get; set; }
    }
    private sealed class ImportedAttribute
    {
        [JsonPropertyName("interval_seconds")] public int IntervalSeconds {get;set;}
        private List<ImportedRule?>? rules;
        private string? fallback;
        [JsonPropertyName("key")] public string? Key { get; set; }
        [JsonPropertyName("name")] public string? Name { get; set; }
        [JsonPropertyName("input")] public string? Input { get; set; }
        [JsonPropertyName("sample")] public string? Sample { get; set; }
        [JsonPropertyName("timeout")] public double? Timeout { get; set; }
        [JsonPropertyName("visibility")] public string? Visibility { get; set; }
        [JsonPropertyName("source")] public string? Source { get; set; }
        [JsonIgnore] public bool HasRules { get; private set; }
        [JsonIgnore] public bool HasFallback { get; private set; }
        [JsonPropertyName("rules")] public List<ImportedRule?>? Rules { get => rules; set { rules = value; HasRules = true; } }
        [JsonPropertyName("fallback")] public string? Fallback { get => fallback; set { fallback = value; HasFallback = true; } }
    }
    private sealed class ImportedRule
    {
        [JsonPropertyName("condition")] public string? Condition { get; set; }
        [JsonPropertyName("value")] public string? Value { get; set; }
    }
    private sealed class ImportedProperty
    {
        [JsonPropertyName("interval_seconds")] public int IntervalSeconds {get;set;}
        [JsonPropertyName("name")] public string? Name { get; set; }
        [JsonPropertyName("source")] public string? Source { get; set; }
        [JsonPropertyName("command")] public string? Command { get; set; }
        [JsonPropertyName("key")] public string? Key { get; set; }
        [JsonPropertyName("timeout_seconds")] public double? TimeoutSeconds { get; set; }
    }
}
