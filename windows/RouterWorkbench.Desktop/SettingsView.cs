using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Media;
using System.Windows.Markup;
using System.Globalization;
using Microsoft.Win32;
using RouterWorkbench.Core;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private UIElement settingsContent = null!;
    private Window? settingsWindow;
    private void OpenSettings()
    {
        if(settingsWindow!=null){settingsWindow.Activate();return;}
        var dialog = new Window { Owner=this, Title="设置", Width=700, Height=660, MinWidth=560, MinHeight=440, WindowStartupLocation=WindowStartupLocation.CenterOwner, Content=settingsContent };
        dialog.SetResourceReference(StyleProperty,typeof(Window));
        settingsWindow=dialog;
        dialog.Closed+=(_,_)=>{dialog.Content=null;settingsWindow=null;};
        dialog.ShowDialog();
    }
    private TextBox sshUser = null!, ServerBox = null!;
    private PasswordBox sshPassword = null!;
    private Button ConnectButton = null!;
    private TextBlock sshClient = null!, telnetClient = null!;
    private UIElement BuildSettings()
    {
        var appearance = Ui.Combo(["跟随系统", "浅色", "深色"], profile.Theme == "Light" ? 1 : profile.Theme == "Dark" ? 2 : 0, 170);
        appearance.HorizontalAlignment = HorizontalAlignment.Left;
        appearance.SelectionChanged += (_, _) => _ = Run("切换主题", () => SetTheme(new[] { "Default", "Light", "Dark" }[appearance.SelectedIndex]));
        sshUser = Ui.Input(profile.SshUser); sshUser.HorizontalAlignment = HorizontalAlignment.Stretch;
        sshPassword = new PasswordBox { HorizontalAlignment = HorizontalAlignment.Stretch };
        sshPassword.SetResourceReference(Control.BackgroundProperty, "Surface"); sshPassword.SetResourceReference(Control.ForegroundProperty, "Text"); sshPassword.SetResourceReference(Control.BorderBrushProperty, "Line");
        try { sshPassword.Password = SshPasswordStore.Load(profilePath); } catch (Exception e) { Log("错误", e.Message); }
        ServerBox = Ui.Input(profile.ServerUrl); ServerBox.HorizontalAlignment = HorizontalAlignment.Stretch;
        ServerBox.KeyDown += ServerKeyDown;
        sshClient = Ui.Text("", true); telnetClient = Ui.Text("", true); sshClient.TextWrapping = telnetClient.TextWrapping = TextWrapping.Wrap;
        Border Section(string title) { var heading=Ui.Heading(title); heading.Margin=new(0,8,0,10); return heading; }
        var panel = new StackPanel { Margin = new(16), MaxWidth = 780, HorizontalAlignment = HorizontalAlignment.Stretch };
        panel.Children.Add(Section("服务器连接")); panel.Children.Add(Ui.Labeled("服务器地址", ServerBox));
        ConnectButton = Ui.Button("保存并连接", () => _ = Run("保存并连接", SaveAndConnect), true);
        panel.Children.Add(Ui.FormActions(ConnectButton, Ui.Button("断开连接", () => DisconnectClick(this, new RoutedEventArgs())), Ui.Button("重新连接", () => _ = Run("连接", Connect))));
        panel.Children.Add(Section("外观")); panel.Children.Add(Ui.Labeled("主题", appearance));
        BuildFontSettings(panel);
        panel.Children.Add(Section("终端客户端")); panel.Children.Add(Ui.Labeled("SSH 用户名", sshUser));
        panel.Children.Add(Ui.Labeled("SSH 密码", sshPassword));
        panel.Children.Add(Ui.Labeled("SSH 客户端", sshClient));
        panel.Children.Add(Ui.FormActions(Ui.Button("选择 SSH 客户端…", () => _ = Run("选择 SSH", () => ChooseClient("ssh"))), Ui.Button("使用系统 SSH", () => _ = Run("恢复 SSH", () => ResetClient("ssh")))));
        panel.Children.Add(Ui.Labeled("Telnet 客户端", telnetClient));
        panel.Children.Add(Ui.FormActions(Ui.Button("选择 Telnet 客户端…", () => _ = Run("选择 Telnet", () => ChooseClient("telnet"))), Ui.Button("使用系统 Telnet", () => _ = Run("恢复 Telnet", () => ResetClient("telnet")))));
        panel.Children.Add(Ui.Note("密码在本机加密保存，可在维护页复制。修改此处不会修改路由器账号。"));
        var save = Ui.Button("保存 SSH 设置", () => _ = Run("保存 SSH 设置", SaveSshSettings), true); save.HorizontalAlignment = HorizontalAlignment.Left; save.Margin = new(164,8,0,8); panel.Children.Add(save);
        UpdateClientLabels(); return new ScrollViewer { Content = panel, VerticalScrollBarVisibility = ScrollBarVisibility.Auto };
    }
    private sealed record FontChoice(string Family, string Label) { public override string ToString() => Label; }
    private void ApplyTypography(string family, double size)
    {
        Typography.Apply(family, size);
        // DataGridColumn does not reliably refresh application dynamic resources; measure these compact columns explicitly.
        TableBehavior.FitTextColumn(ActivityGrid, 0, "88:88:88", 80);
        TableBehavior.FitTextColumn(ActivityGrid, 1, "连接", 60);
        TableBehavior.FitTextColumn(DevicesGrid, 3, "状态", 68);
    }
    private ComboBox uiFont = null!, uiFontSize = null!;
    private TextBlock fontPreview = null!, fontStatus = null!;
    private void BuildFontSettings(Panel panel)
    {
        var language = XmlLanguage.GetLanguage("zh-cn");
        var fonts = new[] { new FontChoice(ServerProfile.DefaultUiFontFamily, "默认字体（微软雅黑 UI）") }
            .Concat(Fonts.SystemFontFamilies.Select(f => new FontChoice(f.Source, f.FamilyNames.TryGetValue(language, out var name) ? name : f.Source)).OrderBy(f => f.Label, StringComparer.CurrentCultureIgnoreCase)).ToArray();
        uiFont = new ComboBox { ItemsSource = fonts, SelectedItem = fonts.FirstOrDefault(f => f.Family == profile.UiFontFamily) ?? fonts[0], IsEditable = true, IsTextSearchEnabled = true, MaxDropDownHeight = 280 };
        uiFontSize = Ui.Combo(["10", "11", "12", "13", "14", "15", "16", "18", "20", "22", "24"], -1, 110);
        uiFontSize.IsEditable = true; uiFontSize.Text = profile.UiFontSize.ToString(CultureInfo.CurrentCulture); uiFontSize.HorizontalAlignment = HorizontalAlignment.Left;
        fontPreview = Ui.Text("字体预览  Router Workbench  192.168.1.1"); fontPreview.TextWrapping = TextWrapping.Wrap;
        fontStatus = Ui.Text("字号范围 10～24，应用后立即生效并保存。", true); fontStatus.TextWrapping = TextWrapping.Wrap; fontStatus.Margin = new(0, 0, 0, 8);
        panel.Children.Add(Ui.Labeled("界面字体", uiFont)); panel.Children.Add(Ui.Labeled("字号", uiFontSize));
        panel.Children.Add(Ui.Labeled("预览", fontPreview));
        panel.Children.Add(Ui.FormActions(Ui.Button("应用字体", () => _ = Run("应用字体", () => SaveTypography(false))), Ui.Button("恢复默认字体", () => _ = Run("恢复默认字体", () => SaveTypography(true)))));
        panel.Children.Add(fontStatus);
        void Preview() {
            fontPreview.FontFamily = new FontFamily((uiFont.SelectedItem as FontChoice)?.Family ?? profile.UiFontFamily);
            if (double.TryParse(uiFontSize.Text, out var size) && double.IsFinite(size) && size is >= 10 and <= 24) fontPreview.FontSize = size;
        }
        uiFont.SelectionChanged += (_, _) => Preview();
        uiFontSize.AddHandler(TextBoxBase.TextChangedEvent, new TextChangedEventHandler((_, _) => Preview())); Preview();
    }
    private async Task SaveTypography(bool reset)
    {
        try {
            var family = reset ? ServerProfile.DefaultUiFontFamily : (uiFont.SelectedItem as FontChoice)?.Family ?? throw new ArgumentException("请从本机字体列表中选择字体。");
            var size = ServerProfile.DefaultUiFontSize;
            if (!reset && (!double.TryParse(uiFontSize.Text, out size) || !double.IsFinite(size) || size is < 10 or > 24)) throw new ArgumentException("字号须为 10～24，可输入小数。");
            var next = profile with { UiFontFamily = family, UiFontSize = size }; await next.SaveAsync(profilePath); profile = next;
            var anchors = propertyPages.Values.ToDictionary(p => p.Table, p => TableBehavior.Capture(p.Table));
            ApplyTypography(family, size);
            foreach (var page in propertyPages.Values) { TableBehavior.FitPropertyColumns(page.Table, page.Rows, true); TableBehavior.Restore(page.Table, anchors[page.Table]); }
            TableBehavior.FitPropertyColumns(transferGrid, transferGrid.Items.Cast<RouterWorkbench.Client.PropertyRow>(), true);
            TableBehavior.FitPropertyColumns(discoveryProperties, discoveryProperties.Items.Cast<RouterWorkbench.Client.PropertyRow>(), true);
            uiFont.SelectedItem = uiFont.Items.Cast<FontChoice>().First(f => f.Family == family); uiFontSize.Text = size.ToString(CultureInfo.CurrentCulture);
            fontStatus.SetResourceReference(TextBlock.ForegroundProperty, "Muted"); fontStatus.Text = "字体设置已保存，工作区和工具窗口已生效。";
        }
        catch (Exception e) { fontStatus.SetResourceReference(TextBlock.ForegroundProperty, "Error"); fontStatus.Text = e.Message; }
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
        sshClient.Text = profile.SshExecutable == "" ? "Windows OpenSSH（系统默认）" : profile.SshExecutable;
        telnetClient.Text = profile.TelnetExecutable == "" ? "Windows Telnet（系统默认）" : profile.TelnetExecutable;
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
