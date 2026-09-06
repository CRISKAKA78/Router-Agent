using RouterWorkbench.Core;

namespace RouterWorkbench;

internal sealed class InputDialog : Form
{
    private readonly Dictionary<string, TextBox> fields = [];
    public InputDialog(string title, params (string Key, string Label, string Value)[] inputs)
    {
        Text = title; StartPosition = FormStartPosition.CenterParent; MinimizeBox = false; MaximizeBox = false;
        FormBorderStyle = FormBorderStyle.FixedDialog; AutoSize = true; AutoSizeMode = AutoSizeMode.GrowAndShrink;
        Font = new Font("Microsoft YaHei UI", 9); Padding = new Padding(16);
        var table = new TableLayoutPanel { AutoSize = true, ColumnCount = 2, Dock = DockStyle.Fill };
        foreach (var (key, label, value) in inputs)
        {
            var box = new TextBox { Text = value, Width = 410, Margin = new Padding(6), AccessibleName = label };
            fields.Add(key, box);
            table.Controls.Add(new Label { Text = label, AutoSize = true, Anchor = AnchorStyles.Left, Margin = new Padding(6) });
            table.Controls.Add(box);
        }
        var buttons = new FlowLayoutPanel { AutoSize = true, FlowDirection = FlowDirection.RightToLeft, Dock = DockStyle.Fill };
        var ok = new Button { Text = "确定", DialogResult = DialogResult.OK, AutoSize = true };
        var cancel = new Button { Text = "取消", DialogResult = DialogResult.Cancel, AutoSize = true };
        buttons.Controls.AddRange([cancel, ok]); table.Controls.Add(buttons); table.SetColumnSpan(buttons, 2);
        Controls.Add(table); AcceptButton = ok; CancelButton = cancel;
    }
    public string this[string key] => fields[key].Text;
}

internal sealed class SettingsDialog : Form
{
    private readonly TextBox ssh = new() { Width = 390 };
    private readonly TextBox telnet = new() { Width = 390 };
    private readonly TextBox user = new() { Width = 220 };
    private readonly CheckBox sshPutty = new() { Text = "SSH 使用 PuTTY 参数", AutoSize = true };
    private readonly CheckBox telnetPutty = new() { Text = "Telnet 使用 PuTTY 参数", AutoSize = true };
    public SettingsDialog(ServerProfile profile)
    {
        Text = "连接与外部客户端设置"; StartPosition = FormStartPosition.CenterParent;
        Size = new Size(690, 340); FormBorderStyle = FormBorderStyle.FixedDialog; MaximizeBox = false; MinimizeBox = false;
        Font = new Font("Microsoft YaHei UI", 9); Padding = new Padding(18);
        ssh.Text = profile.SshExecutable; telnet.Text = profile.TelnetExecutable; user.Text = profile.SshUser;
        sshPutty.Checked = profile.SshUsePutty; telnetPutty.Checked = profile.TelnetUsePutty;
        var panel = new FlowLayoutPanel { Dock = DockStyle.Fill, FlowDirection = FlowDirection.TopDown, WrapContents = false };
        panel.Controls.Add(new Label { Text = "路径留空使用 Windows 系统客户端；可选择已有 PuTTY.exe。", AutoSize = true });
        AddPath(panel, "SSH 程序", ssh); panel.Controls.Add(sshPutty);
        AddPath(panel, "Telnet 程序", telnet); panel.Controls.Add(telnetPutty);
        var row = new FlowLayoutPanel { AutoSize = true }; row.Controls.Add(new Label { Text = "SSH 用户名", AutoSize = true, Padding = new Padding(0, 6, 0, 0) }); row.Controls.Add(user); panel.Controls.Add(row);
        panel.Controls.Add(new Label { Text = "HTTP/HTTPS 地址在主窗口设置。认证、TLS 与访问限制由部署层配置。", AutoSize = true });
        var ok = new Button { Text = "保存", DialogResult = DialogResult.OK, AutoSize = true }; panel.Controls.Add(ok);
        Controls.Add(panel); AcceptButton = ok;
    }
    private static void AddPath(Control parent, string label, TextBox text)
    {
        var row = new FlowLayoutPanel { AutoSize = true };
        row.Controls.Add(new Label { Text = label, AutoSize = true, Padding = new Padding(0, 6, 0, 0) }); row.Controls.Add(text);
        var browse = new Button { Text = "浏览…", AutoSize = true };
        browse.Click += (_, _) => { using var dialog = new OpenFileDialog { Filter = "Windows 程序|*.exe", CheckFileExists = true }; if (dialog.ShowDialog(parent.FindForm()) == DialogResult.OK) text.Text = dialog.FileName; };
        row.Controls.Add(browse); parent.Controls.Add(row);
    }
    public ServerProfile Apply(ServerProfile profile) => profile with { SshExecutable = ssh.Text.Trim(), TelnetExecutable = telnet.Text.Trim(), SshUser = user.Text.Trim(), SshUsePutty = sshPutty.Checked, TelnetUsePutty = telnetPutty.Checked };
}
