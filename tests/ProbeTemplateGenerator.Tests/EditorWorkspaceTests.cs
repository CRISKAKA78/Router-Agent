using System.Text.Json;
using Microsoft.JSInterop;
using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Persistence;
using ProbeTemplateGenerator.Services;

namespace ProbeTemplateGenerator.Tests;

public sealed partial class EditorWorkspaceTests
{
    [Fact]
    public async Task OpeningProjectRequiresReplacementConfirmationAndCancelPreservesEdits()
    {
        await using var fixture = await WorkspaceFixture.CreateAsync();
        var state = fixture.State;
        state.LoadExample(false);
        await state.AcceptConfirmationAsync();
        var original = state.Project;
        original.Name = "尚未保存的编辑";
        state.Touch();
        state.OpenProject(new ProjectFiles().SerializeProject(TemplateCompiler.RulesExampleProject()));
        Assert.Same(original, state.Project);
        Assert.NotNull(state.ConfirmationTitle);
        state.CancelConfirmation();
        Assert.Equal("尚未保存的编辑", state.Project.Name);
        state.OpenProject(new ProjectFiles().SerializeProject(TemplateCompiler.RulesExampleProject()));
        await state.AcceptConfirmationAsync();
        Assert.NotSame(original, state.Project);
        Assert.Null(state.ConfirmationTitle);
        Assert.False(state.LocalBusy);
        Assert.NotEmpty(state.Project.Attributes);
    }

    [Fact]
    public async Task NativeDraftImportRestoresOriginalPendingRequestAndServerForRecovery()
    {
        await using var fixture = await WorkspaceFixture.CreateAsync();
        var pending = new PendingTemplateMutation("http://localhost:8181", "发布新模板", "POST", "probe-templates",
            "{ \"name\": \"保留原请求\", \"properties\": {} }", "original-request-key");
        var draft = new WorkspaceDraft
        {
            Project = TemplateCompiler.ExampleProject(), SavedAt = 100, Pending = pending,
            Target = new PublishingTarget(pending.Origin, "previous-template", 4)
        };
        fixture.State.OpenProject(fixture.Persistence.Serialize(draft));
        await fixture.State.AcceptConfirmationAsync();
        Assert.Equal(pending, fixture.State.Publishing.Pending);
        Assert.Equal(pending.Origin, fixture.State.ServerUrl);
        Assert.Equal("previous-template", fixture.State.Publishing.Target!.Id);
        Assert.True(fixture.State.Locked);
        Assert.False(fixture.State.LocalBusy);
        await fixture.State.SaveProjectAsync();
        Assert.Single(fixture.Browser.Downloads);
        Assert.Contains("original-request-key", fixture.Browser.Storage[WorkspacePersistence.StorageKey]);
    }

    [Fact]
    public async Task InvalidImportCannotDiscardTheCurrentProject()
    {
        await using var fixture = await WorkspaceFixture.CreateAsync();
        var original = fixture.State.Project;
        Assert.ThrowsAny<Exception>(() => fixture.State.OpenProject("{\"schema_version\":999}"));
        Assert.Same(original, fixture.State.Project);
        Assert.Null(fixture.State.ConfirmationTitle);
    }

    [Fact]
    public async Task DamagedStoredDraftIsPreservedUntilAnExplicitNewProject()
    {
        var browser = new MemoryBrowser();
        browser.Storage[WorkspacePersistence.StorageKey] = "broken draft bytes";
        await using var fixture = await WorkspaceFixture.CreateAsync(browser);
        Assert.NotNull(fixture.State.PersistenceError);
        fixture.State.Project.Name = "临时内容";
        fixture.State.Touch();
        await fixture.State.SaveProjectAsync();
        Assert.Equal("broken draft bytes", browser.Storage[WorkspacePersistence.StorageKey]);
        fixture.State.NewProject();
        await fixture.State.AcceptConfirmationAsync();
        await fixture.State.SaveProjectAsync();
        Assert.Null(fixture.State.PersistenceError);
        Assert.NotEqual("broken draft bytes", browser.Storage[WorkspacePersistence.StorageKey]);
    }

    [Fact]
    public async Task CopyIsIndependentAndDeleteRequiresConfirmation()
    {
        await using var fixture = await WorkspaceFixture.CreateAsync();
        fixture.State.LoadExample(true);
        await fixture.State.AcceptConfirmationAsync();
        var state = fixture.State;
        var original = state.Selected!;
        var count = state.Project.Attributes.Count;
        state.CopyAttribute();
        var copy = state.Selected!;
        Assert.NotEqual(original.Id, copy.Id);
        Assert.NotEqual(original.Key, copy.Key);
        copy.Name = "独立副本";
        state.Touch();
        state.Select(original.Id);
        Assert.NotEqual(copy.Name, state.Selected!.Name);
        state.RemoveAttribute();
        Assert.Equal(count + 1, state.Project.Attributes.Count);
        state.CancelConfirmation();
        Assert.Contains(original, state.Project.Attributes);
        state.RemoveAttribute();
        await state.AcceptConfirmationAsync();
        Assert.DoesNotContain(original, state.Project.Attributes);
        Assert.Equal(count, state.Project.Attributes.Count);
    }

    private sealed class WorkspaceFixture : IAsyncDisposable
    {
        public EditorWorkspace State { get; }
        public MemoryBrowser Browser { get; }
        public WorkspacePersistence Persistence { get; }
        private readonly HttpClient http = new();
        private WorkspaceFixture(MemoryBrowser browser)
        {
            Browser = browser;
            var files = new ProjectFiles();
            Persistence = new WorkspacePersistence(browser, files);
            State = new EditorWorkspace(new TemplateCompiler(), files, Persistence, new TemplatePublishingService(http), browser);
        }
        public static async Task<WorkspaceFixture> CreateAsync(MemoryBrowser? browser = null)
        {
            var fixture = new WorkspaceFixture(browser ?? new MemoryBrowser());
            await fixture.State.InitializeAsync();
            return fixture;
        }
        public async ValueTask DisposeAsync() { await State.DisposeAsync(); http.Dispose(); }
    }

    private sealed class MemoryBrowser : IJSRuntime
    {
        public Dictionary<string, string> Storage { get; } = [];
        public List<(string Name, string Content)> Downloads { get; } = [];
        public ValueTask<TValue> InvokeAsync<TValue>(string identifier, object?[]? args) => InvokeAsync<TValue>(identifier, CancellationToken.None, args);
        public ValueTask<TValue> InvokeAsync<TValue>(string identifier, CancellationToken cancellationToken, object?[]? args)
        {
            object? value = null;
            switch (identifier)
            {
                case "workspace.storageGet": value = Storage.GetValueOrDefault((string)args![0]!); break;
                case "workspace.storageSet": Storage[(string)args![0]!] = (string)args[1]!; break;
                case "workspace.download": Downloads.Add(((string)args![0]!, (string)args[1]!)); break;
                case "workspace.setTheme": break;
                default: throw new InvalidOperationException(identifier);
            }
            return ValueTask.FromResult(value is null ? default! : (TValue)value);
        }
    }
}
