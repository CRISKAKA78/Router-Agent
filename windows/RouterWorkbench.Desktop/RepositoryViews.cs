using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using Microsoft.Win32;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private DataGrid assetsGrid = null!, assetProperties = null!;
    private TextBox assetSearch = null!;
    private CheckBox assetArchived = null!;
    private TextBlock assetEmpty = null!;
    private string assetId = "";
    private TabControl fileTabs = null!;
    private UIElement BuildFiles()
    {
        assetSearch = Ui.Input("", 210); assetSearch.Tag = "搜索文件 / 资产 ID"; assetSearch.ToolTip = "搜索文件名称或资产 ID";
        assetArchived = new CheckBox { Content = "包含归档" };
        assetSearch.TextChanged += (_, _) => UpdateFiles(); assetArchived.Checked += (_, _) => UpdateFiles(); assetArchived.Unchecked += (_, _) => UpdateFiles();
        assetsGrid = Ui.Table("文件资产列表", ("名称", "Name", -1), ("大小", "SizeText", 95), ("状态", "StateText", 75), ("资产 ID", "AssetId", 275));
        assetsGrid.SelectionChanged += (_, _) => { if (refreshing) return; assetId = (assetsGrid.SelectedItem as Asset)?.AssetId ?? ""; UpdateAssetDetail(); };
        assetProperties = Ui.Table("资产属性", ("属性", "Name", 115), ("值", "Value", -1));
        assetEmpty = Ui.Text("仓库中没有匹配的文件。使用“导入文件”添加资产。", true); assetEmpty.Margin = new(12,52,12,0); assetEmpty.VerticalAlignment = VerticalAlignment.Top; assetEmpty.HorizontalAlignment = HorizontalAlignment.Center; assetEmpty.IsHitTestVisible = false;
        var list = new Grid(); list.Children.Add(assetsGrid); list.Children.Add(assetEmpty);
        var assets = Ui.Page(Ui.Bar(WriteButton("导入文件…", () => _ = Run("导入文件", ImportAsset)),
            Ui.Button("保存到本机…", () => _ = Run("保存文件", SaveAsset)), DeviceButton("上传到设备…", UploadSelectedAsset),
            DeviceButton("从设备下载…", () => DownloadForm()), Ui.Button("归档…", () => _ = Run("归档资产", ArchiveAsset))),
            Ui.Page(Ui.Bar(assetSearch, assetArchived), Ui.Split(list, Ui.Page(Ui.Heading("资产属性"), assetProperties), true, 2.2)),
            Ui.Note("导入和保存校验长度与 SHA-256。下载任务完整提交并释放后，在文件传输中显式入库；归档保留资产身份与内容。"));
        fileTabs = new TabControl();
        fileTabs.Items.Add(new TabItem { Header = "文件资产", Content = assets });
        fileTabs.Items.Add(new TabItem { Header = "文件传输", Content = BuildTransfers() });
        return fileTabs;
    }
    private void UpdateFiles()
    {
        if (assetsGrid == null) return;
        var rows = snapshot.Assets.Where(a => (assetArchived.IsChecked == true || !a.Archived) && $"{a.Name} {a.AssetId}".Contains(assetSearch.Text, StringComparison.OrdinalIgnoreCase)).ToArray();
        var was = refreshing; refreshing = true; Ui.SetRows(assetsGrid, rows); assetsGrid.SelectedItem = rows.FirstOrDefault(a => a.AssetId == assetId); refreshing = was;
        assetEmpty.Visibility = rows.Length == 0 ? Visibility.Visible : Visibility.Collapsed; UpdateAssetDetail();
    }
    private void UpdateAssetDetail()
    {
        var asset = snapshot.Assets.FirstOrDefault(a => a.AssetId == assetId);
        assetProperties.ItemsSource = asset == null ? Array.Empty<PropertyRow>() : new[] { new PropertyRow("", "资产 ID", asset.AssetId), new("", "名称", asset.Name), new("", "状态", asset.StateText), new("", "长度", asset.Size + " 字节"), new("", "SHA-256", asset.Sha256), new("", "创建时间", Labels.Time(asset.CreatedAt)) };
    }
    private Asset SelectedAsset() => snapshot.Assets.FirstOrDefault(a => a.AssetId == assetId) ?? throw new InvalidOperationException("请选择文件资产。");
    private async Task ImportAsset()
    {
        var c = Connected(); var picker = new OpenFileDialog { Title = "导入文件资产", CheckFileExists = true };
        if (picker.ShowDialog(this) != true) return;
        var mutation = await Mutation.ImportAsync(picker.FileName, c.Token);
        if (connection != c) { mutation.Dispose(); throw new OperationCanceledException(); }
        var data = await Write(mutation); assetId = data.GetProperty("asset_id").GetString()!;
    }
    private async Task SaveAsset()
    {
        var asset = SelectedAsset(); if (asset.Archived) throw new InvalidOperationException("归档资产不能下载。"); var c = Connected();
        var dialog = new SaveFileDialog { Title = "保存资产到本机", FileName = System.IO.Path.GetFileName(asset.Name), OverwritePrompt = true };
        if (dialog.ShowDialog(this) != true) return;
        await c.TrackAsync(async () => { await c.Api.SaveContentAsync(asset, dialog.FileName); return true; }); Log("文件", "文件已校验并保存。");
    }
    private async Task ArchiveAsset()
    {
        var asset = SelectedAsset(); if (!Confirm("归档资产", $"归档 {asset.Name}？归档后阻止新引用和投放，现有内容和身份保留。")) return;
        await Write(new("归档资产", $"assets/{Id(asset.AssetId)}/archive"));
    }
    private void UploadSelectedAsset()
    {
        var asset = snapshot.Assets.FirstOrDefault(a => a.AssetId == assetId);
        if (asset == null) { Log("提示", "请选择可用文件资产。"); return; } UploadForm(asset);
    }
    private void UploadForm(Asset asset, string? directory = null)
    {
        var device = Device; var c = connection;
        if (device == null) { Log("提示", "请选择设备。"); return; }
        Form("上传文件到设备", $"{asset.Name} → {device.DisplayName}",
            [new("path", "设备目标路径", (directory ?? "/tmp").TrimEnd('/') + "/" + asset.Name), new("mode", "文件权限（八进制）", "0644"), new("timeout", "超时（秒）", "60"), new("overwrite", "覆盖已存在文件", "否", Choices: ["否", "是"])],
            async v => {
                if (connection != c || selectedDevice != device.DeviceId) throw new InvalidOperationException("连接或设备已切换。");
                ShowCreatedTask(await Write(new("上传文件", "uploads", new { device_id = device.DeviceId, asset_id = asset.AssetId, remote_path = v["path"], mode = v["mode"], timeout_seconds = Positive(v["timeout"]), overwrite = v["overwrite"] == "是" })));
            });
    }
    private void DownloadForm(string path = "/tmp/result.txt")
    {
        var device = Device; var c = connection;
        if (device == null) { Log("提示", "请选择设备。"); return; }
        Form("从设备下载", "文件先传到服务器，完整提交并释放后在文件传输中显式入库。",
            [new("path", "设备文件路径", path), new("name", "资产名称", path.Split('/').Last()), new("timeout", "超时（秒）", "60")],
            async v => {
                if (connection != c || selectedDevice != device.DeviceId) throw new InvalidOperationException("连接或设备已切换。");
                ShowCreatedTask(await Write(new("下载文件", "downloads", new { device_id = device.DeviceId, remote_path = v["path"], name = v["name"], timeout_seconds = Positive(v["timeout"]) })));
            });
    }
}
