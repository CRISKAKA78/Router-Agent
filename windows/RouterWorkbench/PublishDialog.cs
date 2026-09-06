using RouterWorkbench.Core;

namespace RouterWorkbench;

internal sealed class PublishDialog : Form
{
    private readonly TextBox version = new() { Width = 240 };
    private readonly ComboBox asset = new() { Width = 400, DropDownStyle = ComboBoxStyle.DropDownList, DisplayMember = "Name" };
    private readonly TextBox mode = new() { Text = "0755", Width = 100 };
    private readonly TextBox arch = new() { Width = 350, PlaceholderText = "例如 mipsel；不限制请显式填写 any" };
    private readonly TextBox libc = new() { Width = 350, PlaceholderText = "例如 uclibc；不限制请显式填写 any" };
    private readonly TextBox models = new() { Width = 350 };
    private readonly TextBox kernels = new() { Width = 350 };
    private readonly TextBox capabilities = new() { Width = 350 };
    private readonly ListBox list = new() { Width = 570, Height = 130 };
    private readonly List<Specification> specs = [];
    public sealed record Specification(string AssetId, string Platform, string Mode, Rules Rules);
    public string Version => version.Text;
    public Specification[] Specifications => specs.ToArray();
    public PublishDialog(Asset[] assets)
    {
        Text = "发布不可变工具版本"; StartPosition = FormStartPosition.CenterParent; Size = new Size(660, 640); MinimizeBox = false; MaximizeBox = false;
        Font = new Font("Microsoft YaHei UI", 9); Padding = new Padding(16);
        var panel = new FlowLayoutPanel { Dock = DockStyle.Fill, FlowDirection = FlowDirection.TopDown, WrapContents = false, AutoScroll = true };
        void Field(string title, Control input) { var row = new FlowLayoutPanel { AutoSize = true }; row.Controls.Add(new Label { Text = title, Width = 120, Padding = new Padding(0, 5, 0, 0) }); row.Controls.Add(input); panel.Controls.Add(row); }
        asset.DataSource = assets;
        Field("版本标签", version); Field("文件资产", asset); Field("权限", mode); Field("架构(逗号分隔)", arch); Field("libc(逗号分隔)", libc);
        Field("型号(可选)", models); Field("内核(可选)", kernels); Field("必需能力(可选)", capabilities);
        var add = new Button { Text = "添加产物", AutoSize = true };
        add.Click += (_, _) => {
            if (asset.SelectedItem is not Asset selected || string.IsNullOrWhiteSpace(arch.Text) || string.IsNullOrWhiteSpace(libc.Text)) { MessageBox.Show(this, "请选择资产，并显式填写架构和 libc 约束。" ); return; }
            specs.Add(new(selected.AssetId, "linux", mode.Text, new(Split(arch.Text), Split(libc.Text), Split(models.Text), Split(kernels.Text), Split(capabilities.Text))));
            list.Items.Add($"{selected.Name} | {selected.AssetId} | {arch.Text} / {libc.Text}");
        };
        panel.Controls.Add(add); panel.Controls.Add(list);
        var remove = new Button { Text = "移除所选产物", AutoSize = true }; remove.Click += (_, _) => { if (list.SelectedIndex >= 0) { specs.RemoveAt(list.SelectedIndex); list.Items.RemoveAt(list.SelectedIndex); } }; panel.Controls.Add(remove);
        var publish = new Button { Text = "发布版本", AutoSize = true };
        publish.Click += (_, _) => { if (string.IsNullOrWhiteSpace(Version) || specs.Count == 0) { MessageBox.Show(this, "请填写版本并添加至少一个产物。"); return; } DialogResult = DialogResult.OK; };
        panel.Controls.Add(publish); Controls.Add(panel);
    }
    private static string[] Split(string text) => text.Split(',', StringSplitOptions.TrimEntries | StringSplitOptions.RemoveEmptyEntries);
}
