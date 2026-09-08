using System.Text;
using System.Text.Encodings.Web;
using System.Text.Json;
using System.Text.Json.Serialization;
using Microsoft.JSInterop;
using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;

namespace ProbeTemplateGenerator.Persistence;

public sealed class WorkspaceDraft
{
    [JsonPropertyName("workspace_version")] public int WorkspaceVersion { get; set; } = 2;
    [JsonPropertyName("project")] public TemplateProject Project { get; set; } = new();
    [JsonPropertyName("target")] public PublishingTarget? Target { get; set; }
    [JsonPropertyName("pending")] public PendingTemplateMutation? Pending { get; set; }
    [JsonPropertyName("savedAt")] public long SavedAt { get; set; }
    [JsonPropertyName("serverUrl")] public string ServerUrl { get; set; } = "http://127.0.0.1:8080";
    [JsonPropertyName("theme")] public string Theme { get; set; } = "Default";
}

/// <summary>Versioned browser draft storage, including imported native generator drafts.</summary>
public sealed class WorkspacePersistence(IJSRuntime js, ProjectFiles files)
{
    public const string StorageKey = "router-template-generator.workspace.v2";
    public const string LegacyStorageKey = "router-template-generator.draft.v1";
    private const int MaximumBytes = 1024 * 1024;
    private static readonly JsonSerializerOptions Json = new(JsonSerializerDefaults.Web)
    {
        WriteIndented = true,
        Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping
    };
    private readonly SemaphoreSlim saveGate = new(1, 1);

    public async Task<WorkspaceDraft?> LoadAsync()
    {
        var current = await js.InvokeAsync<string?>("workspace.storageGet", StorageKey);
        // A corrupt current draft is never replaced by an older project or silently overwritten.
        if (!string.IsNullOrEmpty(current)) return Parse(current);
        var legacy = await js.InvokeAsync<string?>("workspace.storageGet", LegacyStorageKey);
        return string.IsNullOrEmpty(legacy) ? null : Parse(legacy);
    }

    public async Task SaveAsync(WorkspaceDraft draft)
    {
        // Serialize before awaiting so this write contains a coherent UI snapshot.
        var text = Serialize(draft);
        await saveGate.WaitAsync();
        try { await js.InvokeVoidAsync("workspace.storageSet", StorageKey, text); }
        finally { saveGate.Release(); }
    }

    public string Serialize(WorkspaceDraft draft)
    {
        ValidateMetadata(draft);
        var text = JsonSerializer.Serialize(draft, Json);
        CheckSize(text);
        return text;
    }

    public WorkspaceDraft Parse(string text)
    {
        CheckSize(text);
        using var document = JsonDocument.Parse(text, new JsonDocumentOptions { MaxDepth = 32 });
        var root = document.RootElement;
        if (root.ValueKind != JsonValueKind.Object || !root.TryGetProperty("project", out var project))
            throw new InvalidDataException("不是可识别的生成器草稿，请使用导入工程读取工程或模板文件");
        var draft = JsonSerializer.Deserialize<WorkspaceDraft>(text, Json)
            ?? throw new InvalidDataException("草稿格式无效");
        if (root.TryGetProperty("workspace_version", out var version) &&
            (!version.TryGetInt32(out var parsedVersion) || parsedVersion is not (1 or 2)))
            throw new InvalidDataException("不支持此草稿版本；原草稿已保留");
        if (!root.TryGetProperty("savedAt", out var timestamp) || !timestamp.TryGetInt64(out var savedAt) || savedAt < 0)
            throw new InvalidDataException("草稿时间格式无效");
        // Version and compatibility checks belong to the same project-file reader used by Open.
        draft.Project = files.ReadProject(project.GetRawText());
        draft.SavedAt = savedAt;
        draft.WorkspaceVersion = 2;
        if (!root.TryGetProperty("serverUrl", out _))
            draft.ServerUrl = draft.Pending?.Origin ?? draft.Target?.Origin ?? "http://127.0.0.1:8080";
        ValidateMetadata(draft);
        return draft;
    }

    public WorkspaceDraft ReadImport(string text)
    {
        CheckSize(text);
        using var document = JsonDocument.Parse(text, new JsonDocumentOptions { MaxDepth = 32 });
        // The original native generator stores this exact envelope in generator-draft.json.
        if (document.RootElement.ValueKind == JsonValueKind.Object && document.RootElement.TryGetProperty("project", out _))
            return Parse(text);
        return new WorkspaceDraft
        {
            Project = files.ReadProject(text),
            SavedAt = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
        };
    }

    private static void ValidateMetadata(WorkspaceDraft draft)
    {
        if (draft.Project is null || draft.SavedAt < 0)
            throw new InvalidDataException("草稿工程或时间格式无效");
        if (draft.Target is not null) TemplatePublishingService.ValidateTarget(draft.Target);
        if (draft.Pending is not null) TemplatePublishingService.ValidatePending(draft.Pending);
        if (draft.Theme is not ("Default" or "Light" or "Dark")) throw new InvalidDataException("草稿外观设置无效");
        // A partially typed address is valid editing state; it is validated before connecting.
        if (draft.ServerUrl is null || draft.ServerUrl.Length > 2048) throw new InvalidDataException("草稿服务器地址格式无效");
    }

    private static void CheckSize(string text)
    {
        if (Encoding.UTF8.GetByteCount(text) > MaximumBytes) throw new InvalidDataException("文件或草稿不能超过 1 MiB");
    }
}
