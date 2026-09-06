using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RouterWorkbench.Core;
using Microsoft.Windows.Storage.Pickers;

namespace RouterWorkbench;
public sealed partial class MainWindow
{
    private void Asset_Changed(object sender, SelectionChangedEventArgs e) { if (!binding) RenderAsset(); }
    private void RenderAsset()
    {
        var asset = SelectedAsset; AssetHeading.Text = asset?.Name ?? "选择文件";
        AssetSummary.Text = asset == null ? "从仓库选择文件，或导入一个本地文件。" : $"{Display.Size(asset.Size)} · {(asset.Archived ? "已归档" : "可用")} · {asset.CreatedAt.LocalDateTime:yyyy-MM-dd HH:mm}";
        AssetDetails.Text = asset == null ? "" : $"资产编号：{asset.AssetId}\nSHA-256：{asset.Sha256}";
        UploadButton.IsEnabled = Model.CanWrite && Model.Online && asset is { Archived: false };
        SaveButton.IsEnabled = Model.Synchronized && !Model.Busy && asset != null;
        ArchiveAssetButton.IsEnabled = Model.CanWrite && asset is { Archived: false };
    }
    private async void Import_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("导入文件", async () => {
        var owner = Model.RequireConnection();
        var picker = new FileOpenPicker(AppWindow.Id) { Title = "导入文件" }; picker.FileTypeFilter.Add("*");
        var file = await picker.PickSingleFileAsync().AsTask(Model.Lifetime); if (file == null || Model.Closing) return;
        var request = await Task.Run(() => Mutation.ImportAsync(file.Path, Model.Lifetime));
        if (ReferenceEquals(owner, Model.Connection)) await Model.ExecuteAsync(request);
    });
    private async void Save_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("保存文件", async () => {
        var asset = SelectedAsset ?? throw new InvalidOperationException("请选择文件。"); var owner = Model.RequireConnection();
        var picker = new FileSavePicker(AppWindow.Id) { SuggestedFileName = Path.GetFileName(asset.Name), Title = "另存文件" };
        picker.FileTypeChoices.Add("文件", new List<string> { Path.GetExtension(asset.Name) is { Length: > 1 } ext ? ext : ".bin" });
        var file = await picker.PickSaveFileAsync().AsTask(Model.Lifetime); if (file == null || Model.Closing) return;
        await owner.SaveAssetAsync(asset, file.Path); Model.SetNotice("文件已保存，大小与 SHA-256 校验通过。");
    });
    private async void Upload_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("上传文件", async () => {
        var id = Model.RequireDevice(); var asset = SelectedAsset ?? throw new InvalidOperationException("请选择文件。");
        var values = await FieldsAsync("上传到设备", ("path", "设备绝对路径", "/tmp/" + Path.GetFileName(asset.Name)), ("mode", "文件权限（四位八进制）", "0644"), ("overwrite", "覆盖现有文件（是 / 否）", "否"), ("timeout", "超时（秒）", "60"));
        if (values == null) return;
        if (values["overwrite"] is not ("是" or "否")) throw new ArgumentException("覆盖选项请输入‘是’或‘否’。");
        await Model.ExecuteAsync(Mutation.Json("上传文件", "uploads", new { device_id = id, asset_id = asset.AssetId, remote_path = values["path"], mode = values["mode"], overwrite = values["overwrite"] == "是", timeout_seconds = PositiveTimeout(values["timeout"]) }));
    });
    private async void Download_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("从设备下载", async () => {
        var id = Model.RequireDevice();
        var values = await FieldsAsync("从设备下载", ("path", "设备绝对路径", "/tmp/diagnostic.txt"), ("name", "文件名称", "diagnostic.txt"), ("timeout", "超时（秒）", "60"));
        if (values != null) await Model.ExecuteAsync(Mutation.Json("从设备下载", "downloads", new { device_id = id, remote_path = values["path"], name = values["name"], timeout_seconds = PositiveTimeout(values["timeout"]) }));
    });
    private async void ArchiveAsset_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("归档文件", async () => {
        var asset = SelectedAsset ?? throw new InvalidOperationException("请选择文件。");
        if (await DialogAsync("归档文件", Text($"归档“{asset.Name}”后，将不再用于新的工具发布或上传。已有身份和文件仍保留。"), "归档") == ContentDialogResult.Primary)
            await Model.ExecuteAsync(Mutation.Json("归档文件", $"assets/{Wire.Segment(asset.AssetId)}/archive", new { }));
    });
    private void Tool_Changed(object sender, SelectionChangedEventArgs e)
    {
        if (binding) return; selectionRevision++; VersionList.ItemsSource = null; ArtifactList.ItemsSource = null; QueueRead("tools", ReadToolsAsync);
    }
    private void Version_Changed(object sender, SelectionChangedEventArgs e) { if (!binding) { selectionRevision++; ArtifactList.ItemsSource = null; QueueRead("compatibility", ReadCompatibilityAsync); } }
    private void Artifact_Changed(object sender, SelectionChangedEventArgs e) { if (!binding) RenderArtifact(); }
    private async Task ReadToolsAsync()
    {
        var tool = SelectedTool; var owner = Model.Connection; var revision = selectionRevision;
        if (tool == null || owner == null) { VersionList.ItemsSource = null; ArtifactList.ItemsSource = null; ToolHeading.Text = "选择工具"; RenderArtifact(); return; }
        var versions = await owner.ReadAsync((a, ct) => a.ListAsync<ToolVersion>($"tools/{Wire.Segment(tool.ToolId)}/versions?include_archived=true", ct));
        if (Model.Closing || !ReferenceEquals(owner, Model.Connection) || revision != selectionRevision || tool.ToolId != SelectedTool?.ToolId) return;
        var previous = SelectedVersion?.Version; binding = true;
        VersionList.ItemsSource = versions; VersionList.SelectedItem = versions.FirstOrDefault(v => v.Version == previous) ?? versions.FirstOrDefault();
        binding = false; ToolHeading.Text = tool.Name + (tool.Archived ? " · 已归档" : "");
        QueueRead("compatibility", ReadCompatibilityAsync); Render();
    }
    private string VersionPath() => $"tools/{Wire.Segment(SelectedTool?.ToolId ?? throw new InvalidOperationException("请选择工具。"))}/versions/{Wire.Segment(SelectedVersion?.Version ?? throw new InvalidOperationException("请选择版本。"))}";
    private async Task ReadCompatibilityAsync()
    {
        var version = SelectedVersion; var owner = Model.Connection; var revision = selectionRevision; var device = Model.Device;
        if (version == null || owner == null) { ArtifactList.ItemsSource = null; RenderArtifact(); return; }
        var path = VersionPath();
        var matches = device == null ? version.Artifacts.Select(a => new Compatibility(a, "unknown", [])).ToArray()
            : await owner.ReadAsync((a, ct) => a.ListAsync<Compatibility>(path + "/compatibility?device_id=" + Wire.Segment(device.DeviceId), ct));
        if (Model.Closing || !ReferenceEquals(owner, Model.Connection) || revision != selectionRevision || version.Version != SelectedVersion?.Version || device?.CurrentSession?.SessionId != Model.Device?.CurrentSession?.SessionId) return;
        binding = true;
        BindList(ArtifactList, matches.Select(m => new ListItem(m.Artifact.ArtifactId,
            $"{string.Join(" / ", m.Artifact.Rules.Arch)} · {Display.State(m.Status)}", $"{m.Artifact.Platform} · {string.Join(" / ", m.Artifact.Rules.Libc)} · 权限 {m.Artifact.Mode}", m)).ToArray());
        binding = false; RenderArtifact();
    }
    private void RenderArtifact()
    {
        var match = SelectedArtifact;
        CompatibilityLabel.Text = match == null ? "选择工具版本，查看与当前设备的兼容性。" : $"服务器兼容性：{Display.State(match.Status)}";
        ArtifactDetails.Text = match == null ? "" : $"产物编号：{match.Artifact.ArtifactId}\n资产编号：{match.Artifact.AssetId}\n" + string.Join("\n", match.Checks.Select(c => $"{Display.CheckField(c.Field)}：{Display.State(c.Status)} · {Display.CheckReason(c.Reason)}"));
        DeployButton.IsEnabled = Model.CanWrite && Model.Online && match?.Status == "compatible" && SelectedTool is { Archived: false } && SelectedVersion is { Archived: false };
    }
    private async void CreateTool_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("创建工具", async () => {
        var values = await FieldsAsync("创建工具", ("name", "工具名称", ""), ("description", "用途说明", ""));
        if (values != null) await Model.ExecuteAsync(Mutation.Json("创建工具", "tools", new { name = Required(values["name"], "名称"), description = values["description"] }));
    });
    private async void Deploy_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("投放工具", async () => {
        var device = Model.RequireDevice(); var tool = SelectedTool!; var version = SelectedVersion!; var match = SelectedArtifact!;
        var values = await FieldsAsync("投放工具 · " + tool.Name, ("path", "设备绝对路径", "/tmp/" + tool.Name), ("timeout", "超时（秒）", "60"));
        if (values != null) await Model.ExecuteAsync(Mutation.Json("投放工具", "deployments", new { device_id = device, tool_id = tool.ToolId, version = version.Version, artifact_id = match.Artifact.ArtifactId, remote_path = values["path"], timeout_seconds = PositiveTimeout(values["timeout"]) }));
    });
    private async void ArchiveTool_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("归档工具", async () => {
        var tool = SelectedTool ?? throw new InvalidOperationException("请选择工具。");
        if (await DialogAsync("归档工具", Text($"归档“{tool.Name}”后，不再用于新的投放。"), "归档") == ContentDialogResult.Primary)
            await Model.ExecuteAsync(Mutation.Json("归档工具", $"tools/{Wire.Segment(tool.ToolId)}/archive", new { }));
    });
    private async void ArchiveVersion_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("归档版本", async () => {
        var path = VersionPath();
        if (await DialogAsync("归档版本", Text("此版本归档后，不再用于新的投放。"), "归档") == ContentDialogResult.Primary)
            await Model.ExecuteAsync(Mutation.Json("归档版本", path + "/archive", new { }));
    });
    private async void Publish_Click(object sender, RoutedEventArgs e) => await Model.RunAsync("发布版本", async () => {
        var tool = SelectedTool ?? throw new InvalidOperationException("请选择工具。");
        var available = (Model.Snapshot?.Assets ?? []).Where(a => !a.Archived).ToArray();
        if (available.Length == 0) throw new InvalidOperationException("请先在文件页导入产物文件。");
        var panel = new StackPanel { Spacing = 16, MinWidth = 440 };
        var version = new TextBox { Header = "版本名称", PlaceholderText = "例如 1.0.0" }; panel.Children.Add(version);
        panel.Children.Add(Text("每个产物选择一个仓库文件，填写与目标设备匹配的规则。多个值用逗号分隔。"));
        var rows = new List<ArtifactEditor>();
        var add = new Button { Content = "添加产物" };
        void Add() { var row = new ArtifactEditor(available); rows.Add(row); panel.Children.Insert(panel.Children.Count - 1, row.Panel); }
        panel.Children.Add(add); add.Click += (_, _) => Add(); Add();
        if (await DialogAsync("发布工具版本 · " + tool.Name, new ScrollViewer { Content = panel, MaxHeight = 500 }, "发布") != ContentDialogResult.Primary) return;
        var artifacts = rows.Where(r => r.Include.IsChecked == true).Select(r => r.Value()).ToArray();
        if (artifacts.Length == 0) throw new ArgumentException("至少保留一个产物。");
        await Model.ExecuteAsync(Mutation.Json("发布工具版本", $"tools/{Wire.Segment(tool.ToolId)}/versions/{Wire.Segment(Required(version.Text, "版本"))}", new { artifacts }, "PUT"));
        QueueRead("tools", ReadToolsAsync);
    });
    private sealed class ArtifactEditor
    {
        public StackPanel Panel { get; } = new() { Spacing = 10 };
        public CheckBox Include { get; } = new() { Content = "包含此产物", IsChecked = true };
        private readonly ComboBox asset;
        private readonly TextBox arch = new() { Header = "处理器架构（精确值）", Text = "x86_64" };
        private readonly TextBox libc = new() { Header = "运行库（例如 musl、uclibc、any）", Text = "any" };
        private readonly TextBox mode = new() { Header = "文件权限", Text = "0755" };
        private readonly TextBox models = new() { Header = "设备型号（可选）" };
        private readonly TextBox kernels = new() { Header = "内核（可选）" };
        private readonly TextBox capabilities = new() { Header = "所需能力（可选）" };
        public ArtifactEditor(Asset[] assets)
        {
            asset = new() { Header = "仓库文件", ItemsSource = assets, DisplayMemberPath = "Name", SelectedIndex = 0, HorizontalAlignment = HorizontalAlignment.Stretch };
            foreach (var element in new UIElement[] { Include, asset, arch, libc, mode, models, kernels, capabilities }) Panel.Children.Add(element);
        }
        private static string[] Split(string value) => value.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
        public object Value() => new { asset_id = ((Asset)asset.SelectedItem).AssetId, platform = "linux", mode = mode.Text, rules = new Rules(Split(Required(arch.Text, "架构")), Split(Required(libc.Text, "运行库")), Split(models.Text), Split(kernels.Text), Split(capabilities.Text)) };
    }
}
