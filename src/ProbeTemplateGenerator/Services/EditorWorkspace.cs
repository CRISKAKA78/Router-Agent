using Microsoft.JSInterop;
using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Persistence;

namespace ProbeTemplateGenerator.Services;

/// <summary>One browser workspace: selection, editing lifecycle, persistence and user actions.</summary>
public sealed partial class EditorWorkspace(TemplateCompiler compiler, ProjectFiles files, WorkspacePersistence persistence,
    TemplatePublishingService publishing, IJSRuntime js) : IAsyncDisposable
{
    public TemplateProject Project { get; private set; } = TemplateCompiler.EmptyProject();
    public TemplatePublishingService Publishing => publishing;
    public TemplateAttribute? Selected => Project.Attributes.FirstOrDefault(a => a.Id == SelectedId) ?? Project.Attributes.FirstOrDefault();
    public string SelectedId { get; private set; } = "";
    public string Search { get; set; } = "";
    public string Theme { get; private set; } = "Default";
    public string ServerUrl { get; set; } = "http://127.0.0.1:8080";
    public int Stage { get; private set; }
    public bool Ready { get; private set; }
    public bool Saving { get; private set; }
    public bool IsDirty { get; private set; }
    public bool LocalBusy { get; private set; }
    public bool Locked => !Ready || LocalBusy || publishing.Busy || publishing.Pending is not null;
    public string? PersistenceError { get; private set; }
    public string? Toast { get; private set; }
    public bool ToastError { get; private set; }
    public string? ConfirmationTitle { get; private set; }
    public string? ConfirmationMessage { get; private set; }
    public string ConfirmationAction { get; private set; } = "确认";
    public bool ConfirmationDanger { get; private set; }
    public IReadOnlyList<ValidationIssue> Issues { get; private set; } = [];
    public IReadOnlyList<PreviewResult> PreviewRows { get; private set; } = [];
    public RuntimeTemplate? Compiled { get; private set; }
    public string TemplateJson => Compiled is null ? "请先修正校验错误。" : files.SerializeTemplate(Compiled);
    public IEnumerable<TemplateAttribute> VisibleAttributes => Project.Attributes.Where(a =>
        string.IsNullOrWhiteSpace(Search) || a.Name.Contains(Search, StringComparison.OrdinalIgnoreCase) || a.Key.Contains(Search, StringComparison.OrdinalIgnoreCase));
    public event Action? Changed;
    private Func<Task>? confirmation;
    private CancellationTokenSource? autosave;
    private CancellationTokenSource? toastTimer;
    private bool canPersist;
    private bool disposed;
    private readonly SemaphoreSlim saveGate = new(1, 1);

    public async Task InitializeAsync()
    {
        publishing.Changed += PublishingChanged;
        publishing.PersistStateAsync = SaveDraftAsync;
        try
        {
            var draft = await persistence.LoadAsync();
            if (draft is not null)
            {
                Project = draft.Project;
                ServerUrl = draft.ServerUrl;
                Theme = draft.Theme;
                publishing.Restore(draft.Target, draft.Pending);
            }
            canPersist = true;
        }
        catch (Exception e) { PersistenceError = $"读取草稿失败：{e.Message}。请打开工程或新建工程后继续自动保存。"; }
        SelectedId = Project.Attributes.FirstOrDefault()?.Id ?? "";
        Ready = true;
        Recalculate();
        await js.InvokeVoidAsync("workspace.setTheme", Theme);
        Notify();
    }

    public void Select(string id) { SelectedId = id; Stage = 1; Notify(); }
    public void SetStage(int stage) { Stage = Math.Clamp(stage,0,3); Notify(); }
    public void Touch()
    {
        if (!Ready || disposed) return;
        IsDirty = true;
        Recalculate();
        ScheduleSave();
        Notify();
    }
    public string? FieldError(string field, string? attributeId = null) =>
        Issues.FirstOrDefault(i => i.AttributeId == attributeId && i.Field.Equals(field, StringComparison.OrdinalIgnoreCase))?.Message;
    public bool HasErrors(TemplateAttribute row) => Issues.Any(i => i.AttributeId == row.Id);
    public void GoToIssue(ValidationIssue issue)
    {
        if (issue.AttributeId is not null) SelectedId = issue.AttributeId;
        Stage = issue.AttributeId is null ? 0 : 1;
        if(issue.Field=="NeighborProbe")ConfigurationTab=3;else if(issue.Field=="SwitchProbe")ConfigurationTab=2;else if(issue.Field=="Presentation")ConfigurationTab=1;
        Notify();
    }
    public void AddAttribute(AttributeVisibility visibility)
    {
        if (Locked || Project.Attributes.Count >= 128) return;
        var row = TemplateCompiler.FreshAttribute(visibility);
        Project.Attributes.Add(row);
        SelectedId = row.Id;
        Search = "";
        Stage = 1;
        Touch();
    }
    public void CopyAttribute()
    {
        if (Locked || Selected is not { } row || Project.Attributes.Count >= 128) return;
        var copy = TemplateCompiler.CopyAttribute(Project.Attributes, row.Id);
        Project.Attributes.Insert(Project.Attributes.IndexOf(row) + 1, copy);
        if(Project.Presentation?.Fields.TryGetValue(row.Key,out var placement)==true)
            Layout.Fields[copy.Key]=new(){GroupId=placement.GroupId,Order=NextOrder(placement.GroupId,copy.Key),Visible=placement.Visible};
        SelectedId = copy.Id;
        Touch();
        ShowToast("已复制属性，副本使用独立标识。");
    }
    public void RemoveAttribute()
    {
        if (Locked || Selected is not { } row) return;
        Confirm("删除属性", $"确认删除“{row.Name}”？引用此属性的公式或条件需要同步修改。", () =>
        {
            var index = Project.Attributes.IndexOf(row);
            Project.Attributes.Remove(row);
            Project.Presentation?.Fields.Remove(pendingLayoutKeys.GetValueOrDefault(row.Id,row.Key));
            pendingLayoutKeys.Remove(row.Id);
            SelectedId = Project.Attributes.ElementAtOrDefault(Math.Max(0, index - 1))?.Id ?? "";
            Touch();
            return Task.CompletedTask;
        }, "删除", true);
    }
    public void ChangeSource(TemplateAttribute row, AttributeSource source)
    {
        row.Source = source;
        row.Input = "";
        if (source == AttributeSource.Rules)
        {
            row.Rules ??= [new ResultRule()];
            row.Fallback ??= "未知";
        }
        Touch();
    }
    public void MoveRule(TemplateAttribute row, int index, int offset)
    {
        if (row.Rules is not { } rules || index + offset < 0 || index + offset >= rules.Count) return;
        (rules[index], rules[index + offset]) = (rules[index + offset], rules[index]);
        Touch();
    }
    public void AddRule(TemplateAttribute row)
    {
        if (Locked || row.Rules?.Count >= 32) return;
        row.Rules ??= [];
        row.Rules.Add(new ResultRule());
        Touch();
    }
    public void RemoveRule(TemplateAttribute row, int index) => Confirm("删除结果规则",
        $"确认删除第 {index + 1} 条结果规则？其余规则将按顺序继续判断。", () =>
        { row.Rules?.RemoveAt(index); Touch(); return Task.CompletedTask; }, "删除", true);

    public void NewProject() => ReplaceWithConfirmation("新建工程", TemplateCompiler.EmptyProject());
    public void LoadExample(bool rules) => ReplaceWithConfirmation(rules ? "加载条件示例" : "加载示例",
        rules ? TemplateCompiler.RulesExampleProject() : TemplateCompiler.ExampleProject());
    public void OpenProject(string json)
    {
        var draft = persistence.ReadImport(json);
        ReplaceWithConfirmation("打开工程 / 模板", draft.Project, draft.Target, "工程已导入。", draft.Pending,
            draft.Pending?.Origin ?? draft.Target?.Origin);
    }
    private void ReplaceWithConfirmation(string title, TemplateProject project, PublishingTarget? target = null, string? notice = null,
        PendingTemplateMutation? pending = null, string? origin = null)
    {
        if (Locked) return;
        async Task Replace()
        {
            if (pending is not null && publishing.Origin is not null && publishing.Origin != pending.Origin)
                await publishing.DisconnectAsync();
            Project = project;
            pendingLayoutKeys.Clear();
            publishing.Restore(target, pending);
            if (origin is not null) ServerUrl = origin;
            SelectedId = project.Attributes.FirstOrDefault()?.Id ?? "";
            Search = "";
            Stage = 1;
            canPersist = true;
            PersistenceError = null;
            Touch();
            if (notice is not null) ShowToast(notice);
        }
        if (Project.Attributes.Count == 0 && !IsDirty) _ = RunAsync(Replace);
        else Confirm(title, $"确认替换当前工程“{Project.Name}”？请先保存需要保留的工程文件。", Replace, "替换");
    }
    public async Task SaveProjectAsync()
    {
        if (!Ready || LocalBusy || publishing.Busy) return;
        await RunAsync(async () =>
        {
            await js.InvokeVoidAsync("workspace.download", $"{SafeName()}.project.json", files.SerializeProject(Project));
            IsDirty = false;
            if (canPersist) await SaveDraftAsync();
            ShowToast("工程文件已导出，包含公式、规则与模拟值。");
        });
    }
    public async Task ExportAsync()
    {
        if (Locked || Compiled is null) return;
        await RunAsync(async () =>
        {
            await js.InvokeVoidAsync("workspace.download", $"{SafeName()}.template.json", files.SerializeTemplate(compiler.Compile(Project)));
            ShowToast("运行模板已导出。");
        });
    }
    public async Task SetThemeAsync(string theme)
    {
        Theme = theme;
        await js.InvokeVoidAsync("workspace.setTheme", theme);
        ScheduleSave();
        Notify();
    }
    public Task ConnectAsync() => RunAsync(async () => { await publishing.ConnectAsync(ServerUrl); ServerUrl = publishing.Origin ?? ServerUrl; await SaveDraftAsync(); });
    public Task DisconnectAsync() => RunAsync(publishing.DisconnectAsync);
    public Task RefreshAsync() => RunAsync(publishing.RefreshAsync);
    public void Publish(bool update)
    {
        if (Locked || Compiled is null) return;
        async Task PublishCore()
        {
            var item = await publishing.PublishAsync(compiler.Compile(Project), update);
            ShowToast($"“{item.Name}”已发布，版本 {item.Version}。请在纳管或设备资料中选择并应用此版本。");
        }
        if (update) Confirm("更新服务器模板", $"确认使用当前工程覆盖已绑定模板？预期版本为 {publishing.Target?.Version}。", PublishCore, "更新");
        else _ = RunAsync(PublishCore);
    }
    public void BindTemplate(PublishedTemplate item)
    {
        publishing.Bind(item);
        ScheduleSave();
        ShowToast($"已绑定“{item.Name}”版本 {item.Version}。");
    }
    public void ImportTemplate(PublishedTemplate item) => ReplaceWithConfirmation($"导入“{item.Name}”", files.FromTemplate(item),
        new PublishingTarget(publishing.Origin!, item.TemplateId, item.Version), "运行模板已导入；生成的命令保留为命令来源。");
    public void DeleteTemplate(PublishedTemplate item) => Confirm("删除服务器模板",
        $"确认删除“{item.Name}”（版本 {item.Version}）？型号或设备仍引用此模板时服务端会拒绝删除。",
        async () => { await publishing.DeleteAsync(item); ShowToast("服务器模板已删除。"); }, "删除", true);
    public void RetryPending() => Confirm("核对并重试原请求",
        "请核对服务器没有重启。确认后将使用保留的原始请求及幂等键重试；服务器已重启时请取消并先核对执行结果。",
        async () => { await publishing.RetryPendingAsync(true); ShowToast("原请求已处理。"); }, "确认未重启并重试");
    public void AbandonPending() => Confirm("核对后放弃原请求",
        "请先核对服务器上的执行结果。确认后只清除本机保留的原请求，不会撤销服务器已完成的操作。",
        async () => { await publishing.AbandonPendingAsync(true); ShowToast("已清除保留的原请求。"); }, "已核对，放弃原请求");

    public void Confirm(string title, string message, Func<Task> action, string button = "确认", bool danger = false)
    {
        ConfirmationTitle = title; ConfirmationMessage = message; ConfirmationAction = button;
        ConfirmationDanger = danger; confirmation = action; Notify();
    }
    public void CancelConfirmation() { confirmation = null; ConfirmationTitle = null; Notify(); }
    public async Task AcceptConfirmationAsync()
    {
        if (confirmation is not { } action || LocalBusy || publishing.Busy) return;
        await RunAsync(async () => { await action(); CancelConfirmation(); });
    }
    public async Task RunAsync(Func<Task> action)
    {
        if (LocalBusy) return;
        LocalBusy = true;
        Notify();
        try { await action(); }
        catch (Exception e) { ShowToast(e.Message, true); }
        finally { LocalBusy = false; Notify(); }
    }
    public void ShowToast(string message, bool error = false)
    {
        Toast = message; ToastError = error;
        toastTimer?.Cancel(); toastTimer?.Dispose(); toastTimer = new();
        _ = ClearToastLaterAsync(toastTimer.Token);
        Notify();
    }
    public void DismissToast() { Toast = null; Notify(); }
    private async Task ClearToastLaterAsync(CancellationToken token)
    {
        try { await Task.Delay(TimeSpan.FromSeconds(7), token); Toast = null; Notify(); }
        catch (OperationCanceledException) { }
    }
    private void Recalculate()
    {
        Issues = compiler.Validate(Project);
        Compiled = null;
        PreviewRows = [];
        if (Issues.Count > 0) return;
        try { Compiled = compiler.Compile(Project); PreviewRows = compiler.Preview(Project); }
        catch (Exception e) { Issues = [new ValidationIssue(null, "project", e.Message)]; }
    }
    private string SafeName() => string.Concat((string.IsNullOrWhiteSpace(Project.Name) ? "模板" : Project.Name).Select(c =>
        Path.GetInvalidFileNameChars().Contains(c) || c == '/' || c == '\\' ? '_' : c));
    private void ScheduleSave()
    {
        if (!canPersist || disposed) return;
        autosave?.Cancel(); autosave?.Dispose(); autosave = new();
        Saving = true;
        _ = SaveLaterAsync(autosave.Token);
    }
    private async Task SaveLaterAsync(CancellationToken token)
    {
        try { await Task.Delay(450, token); await SaveDraftAsync(); }
        catch (OperationCanceledException) { }
        catch (Exception e) { PersistenceError = $"自动保存失败：{e.Message}。请保存工程文件。"; Saving = false; Notify(); }
    }
    private async Task SaveDraftAsync()
    {
        if (!canPersist) throw new InvalidOperationException("请先打开或新建工程，恢复本机草稿保存后再发布。");
        await saveGate.WaitAsync();
        try
        {
            await persistence.SaveAsync(new WorkspaceDraft
            {
                Project = Project, Target = publishing.Target, Pending = publishing.Pending,
                SavedAt = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(), ServerUrl = ServerUrl, Theme = Theme
            });
            PersistenceError = null;
        }
        finally { saveGate.Release(); Saving = false; Notify(); }
    }
    private void PublishingChanged() { Notify(); }
    private void Notify() { if (!disposed) Changed?.Invoke(); }
    public async ValueTask DisposeAsync()
    {
        disposed = true;
        autosave?.Cancel(); autosave?.Dispose(); toastTimer?.Cancel(); toastTimer?.Dispose();
        publishing.Changed -= PublishingChanged;
        publishing.PersistStateAsync = null;
        await publishing.DisposeAsync();
    }
}
