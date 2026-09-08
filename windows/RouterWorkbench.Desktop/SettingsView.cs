using System.Windows;
using System.Windows.Controls;
using Microsoft.Win32;
using RouterWorkbench.Core;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private TextBox sshUser = null!, ServerBox = null!;
    private PasswordBox sshPassword = null!;
    private Button ConnectButton = null!;
    private TextBlock sshClient = null!, telnetClient = null!;
    private UIElement BuildSettings()
    {
        var appearance = Ui.Combo(["跟随系统", "浅色", "深色"], profile.Theme == "Light" ? 1 : profile.Theme == "Dark" ? 2 : 0, 170);
        appearance.HorizontalAlignment = HorizontalAlignment.Left;
        appearance.SelectionChanged += (_, _) => _ = Run("切换主题", () => SetTheme(new[] { "Default", "Light", "Dark" }[appearance.SelectedIndex]));
        sshUser = Ui.Input(profile.SshUser, 380); sshUser.HorizontalAlignment = HorizontalAlignment.Left;
        sshPassword = new PasswordBox { Width = 380, MinHeight = 28, Padding = new(6,3,6,3), HorizontalAlignment = HorizontalAlignment.Left };
        sshPassword.SetResourceReference(Control.BackgroundProperty, "Surface"); sshPassword.SetResourceReference(Control.ForegroundProperty, "Text"); sshPassword.SetResourceReference(Control.BorderBrushProperty, "Line");
        try { sshPassword.Password = SshPasswordStore.Load(profilePath); } catch (Exception e) { Log("错误", e.Message); }
        ServerBox = Ui.Input(profile.ServerUrl, 480); ServerBox.HorizontalAlignment = HorizontalAlignment.Left;
        ServerBox.KeyDown += ServerKeyDown;
        sshClient = Ui.Text("", true); telnetClient = Ui.Text("", true); sshClient.TextWrapping = telnetClient.TextWrapping = TextWrapping.Wrap;
        var panel = new StackPanel { Margin = new(20), MaxWidth = 780, HorizontalAlignment = HorizontalAlignment.Left };
        panel.Children.Add(Ui.Heading("服务器连接")); panel.Children.Add(Ui.Labeled("服务器地址", ServerBox));
        ConnectButton = Ui.Button("保存并连接", () => _ = Run("保存并连接", SaveAndConnect), true);
        panel.Children.Add(Ui.Bar(ConnectButton, Ui.Button("断开连接", () => DisconnectClick(this, new RoutedEventArgs())), Ui.Button("重新连接", () => _ = Run("连接", Connect))));
        panel.Children.Add(Ui.Note("程序启动时自动连接上次保存的服务器。连接失败会重试，也可在此修改地址。"));
        panel.Children.Add(Ui.Heading("外观")); panel.Children.Add(Ui.Labeled("主题", appearance));
        panel.Children.Add(Ui.Heading("终端客户端")); panel.Children.Add(Ui.Labeled("SSH 用户名", sshUser));
        panel.Children.Add(Ui.Labeled("SSH 密码", sshPassword));
        panel.Children.Add(Ui.Bar(Ui.Button("选择 SSH 客户端…", () => _ = Run("选择 SSH", () => ChooseClient("ssh"))), Ui.Button("使用系统 SSH", () => _ = Run("恢复 SSH", () => ResetClient("ssh"))))); panel.Children.Add(sshClient);
        panel.Children.Add(Ui.Bar(Ui.Button("选择 Telnet 客户端…", () => _ = Run("选择 Telnet", () => ChooseClient("telnet"))), Ui.Button("使用系统 Telnet", () => _ = Run("恢复 Telnet", () => ResetClient("telnet"))))); panel.Children.Add(telnetClient);
        panel.Children.Add(Ui.Note("SSH 默认账号和密码均为 admin。密码加密保存在当前 Windows 用户本机；外部客户端询问密码时，在维护页复制并粘贴。修改此处不会修改路由器账号。"));
        var save = Ui.Button("保存 SSH 设置", () => _ = Run("保存 SSH 设置", SaveSshSettings), true); save.HorizontalAlignment = HorizontalAlignment.Left; save.Margin = new(0,12,0,14); panel.Children.Add(save);
        panel.Children.Add(Ui.Note("Ctrl+O 打开设置，Ctrl+F 搜索设备，F5 刷新，Ctrl+J 显示或隐藏活动输出。设备属性按实际模板上报展示。"));
        UpdateClientLabels(); return new ScrollViewer { Content = panel, VerticalScrollBarVisibility = ScrollBarVisibility.Auto };
    }
    private async Task SaveAndConnect()
    {
        var next = profile with { ServerUrl = ServerBox.Text.Trim() }; next.BaseUri();
        if (connection?.Pending != null && !Confirm("切换连接", "存在响应不确定的请求。切换将放弃本地记录但不会撤销服务器操作。继续？")) return;
        await next.SaveAsync(profilePath); profile = next;
        if (connection?.Pending != null) connection.Abandon();
        await Connect();
    }
    private async Task SaveSshSettings()
    {
        var next = profile with { SshUser = sshUser.Text.Trim() }; ShellPolicy.ValidateProfile(next, profile);
        await SshPasswordStore.SaveAsync(profilePath, sshPassword.Password); await next.SaveAsync(profilePath); profile = next;
        UpdateEndpoints(); Log("设置", "SSH 账号与密码已保存。");
    }
    private Task CopySshPassword() { Clipboard.SetText(SshPasswordStore.Load(profilePath)); Log("维护", "已复制 SSH 密码。"); return Task.CompletedTask; }
    private void UpdateClientLabels() {
        sshClient.Text = "SSH  ·  " + (profile.SshExecutable == "" ? "Windows OpenSSH（系统默认）" : profile.SshExecutable);
        telnetClient.Text = "Telnet  ·  " + (profile.TelnetExecutable == "" ? "Windows Telnet（系统默认）" : profile.TelnetExecutable);
    }
    private async Task ChooseClient(string service)
    {
        var picker = new OpenFileDialog { Title = $"选择 {service}.exe 或 putty.exe", Filter = "客户端程序 (*.exe)|*.exe", CheckFileExists = true }; if (picker.ShowDialog(this) != true) return;
        var name = System.IO.Path.GetFileName(picker.FileName).ToLowerInvariant(); if (name != service + ".exe" && name != "putty.exe") throw new ArgumentException("请选择对应客户端或 putty.exe。");
        profile = service == "ssh" ? profile with { SshExecutable = picker.FileName, SshUsePutty = name == "putty.exe" } : profile with { TelnetExecutable = picker.FileName, TelnetUsePutty = name == "putty.exe" };
        await profile.SaveAsync(profilePath); UpdateClientLabels();
    }
    private async Task ResetClient(string service)
    {
        profile = service == "ssh" ? profile with { SshExecutable = "", SshUsePutty = false } : profile with { TelnetExecutable = "", TelnetUsePutty = false };
        await profile.SaveAsync(profilePath); UpdateClientLabels();
    }
}
