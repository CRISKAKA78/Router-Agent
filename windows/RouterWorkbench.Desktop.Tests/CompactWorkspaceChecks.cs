using System.IO;
using System.Reflection;
using System.Text.Json;
using System.Windows;
using System.Windows.Automation.Peers;
using System.Windows.Automation.Provider;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Core;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static async Task CompactWorkspaceChecks(MainWindow window)
    {
        var context=Field<TextBlock>(window,"configContext").Text;
        Field<TextBox>(window,"configKey").Text="ANOTHER_KEY";
        Check(context.Contains("SN")&&Field<TextBlock>(window,"configContext").Text==context,"config result retains the submitted key while the form changes");
        var mode=Field<ComboBox>(window,"configOperation");var backend=Field<ComboBox>(window,"configBackend");
        Check(Field<FrameworkElement>(window,"configValueField").Visibility==Visibility.Collapsed,"read mode removes the irrelevant value field and label");
        mode.SelectedIndex=1;Check(Field<FrameworkElement>(window,"configValueField").Visibility==Visibility.Visible&&Field<TextBlock>(window,"configHelp").Text.Contains("不自动提交"),"write value and persistence explanation appear together");
        mode.SelectedIndex=2;Check(Field<FrameworkElement>(window,"configValueField").Visibility==Visibility.Collapsed,"delete mode does not show a write value");
        mode.SelectedIndex=3;Check(Field<FrameworkElement>(window,"configKeyField").Visibility==Visibility.Collapsed,"NVRAM commit has no irrelevant key input");
        backend.SelectedIndex=1;Check(Field<FrameworkElement>(window,"configKeyField").Visibility==Visibility.Visible&&Field<TextBlock>(window,"configKeyLabel").Text=="UCI 配置包","UCI commit retains its required package input");
        mode.SelectedIndex=0;backend.SelectedIndex=0;Field<TextBox>(window,"configKey").Text="SN";
        Invoke(window,"Navigate","files");
        var browser=Field<FrameworkElement>(window,"fileBrowser");
        var entries=(DataGrid)browser.GetType().GetProperty("Entries")!.GetValue(browser)!;
        await Eventually(()=>Task.FromResult(browser.GetType().GetProperty("Ready")!.GetValue(browser) is true),"compact file browser loads the current device through the existing API");
        Check((string?)browser.GetType().GetProperty("CurrentPath")!.GetValue(browser)=="/tmp/root", "file workspace initially opens tmp/root");
        var history=Field<Expander>(window,"transferHistory");
        Check(!history.IsExpanded,"file history initially yields its space to directory browsing");
        entries.SelectedItem=entries.Items.OfType<RemoteEntry>().First(e=>e.Kind=="f");
        Invoke(window,"UpdateEnabled");Check(Field<Button>(window,"downloadFileButton").IsEnabled,"download becomes available for a selected file");
        entries.SelectedItem=null;Check(!Field<Button>(window,"downloadFileButton").IsEnabled,"clearing file selection disables download immediately");
        window.UpdateLayout();var large=browser.ActualHeight;
        var peer=new ExpanderAutomationPeer(history);var expansion=(IExpandCollapseProvider)peer.GetPattern(PatternInterface.ExpandCollapse);
        expansion.Expand();window.UpdateLayout();Check(history.IsExpanded&&browser.ActualHeight<large,"native expand-collapse pattern opens history and reallocates directory height");
        expansion.Collapse();window.UpdateLayout();Check(!history.IsExpanded&&browser.ActualHeight>=large-1,"closing history restores directory space");
        Check(!Field<Button>(window,"resumeExchangeButton").IsEnabled&&!Field<Button>(window,"saveDownloadButton").IsEnabled,"absent exchange and unselected download have no enabled action");
        Invoke(window,"Navigate","tools");var search=Field<TextBox>(window,"toolSearch");search.Text="no-such-tool-for-ui-check";
        Check(Field<TextBlock>(window,"toolEmpty").Text.Contains("关键词")&&Field<Expander>(window,"toolVersionSection").Visibility==Visibility.Collapsed,"unmatched search explains recovery without an empty version pane");
        search.Clear();var tools=Field<DataGrid>(window,"toolsGrid");tools.SelectedIndex=0;
        await Eventually(()=>Task.FromResult(Field<DataGrid>(window,"toolVersions").Items.Count>0),"selected tool expands versions in the compact workspace");
        Check(Field<Expander>(window,"toolVersionSection").IsExpanded&&!Field<Button>(window,"deployToolButton").IsEnabled,"version section opens but deployment waits for explicit version choice");
        Field<DataGrid>(window,"toolVersions").SelectedIndex=0;Check(Field<Button>(window,"deployToolButton").IsEnabled,"explicit version choice enables the existing confirmation flow");
        tools.SelectedItem=null;Check(Field<Expander>(window,"toolVersionSection").Visibility==Visibility.Collapsed,"clearing tool selection releases version space");
        var width=window.Width;var height=window.Height;
        foreach(var size in new[]{(1480d,920d,13d),(1000d,760d,18d)}) {
            window.Width=size.Item1;window.Height=size.Item2;Typography.Apply(ServerProfile.DefaultUiFontFamily,size.Item3);
            foreach(var page in new[]{"files","maintenance","config","tools"}) {
                Invoke(window,"Navigate",page);await Task.Delay(90);window.UpdateLayout();
                Render(window,$"compact-{page}-{size.Item1}.png");
                var root=(FrameworkElement)((TabItem)((TabControl)window.FindName("WorkspaceTabs")).SelectedItem).Content;
                Check(root.ActualWidth<=((TabControl)window.FindName("WorkspaceTabs")).ActualWidth+1,"compact page remains in its workspace: "+page+" / "+size.Item1);
            }
        }
        Typography.Apply(ServerProfile.DefaultUiFontFamily,13);window.Width=width;window.Height=height;
    }

    // Preview-only fixture: production WPF controls, illustrative source-matched values, no real device writes.
    private static MainWindow CreateCompactPreview()
    {
        void Set(object target,string name,object? value)=>target.GetType().GetField(name,BindingFlags.NonPublic|BindingFlags.Instance)!.SetValue(target,value);
        var when=DateTimeOffset.Parse("2026-09-10T15:19:29+08:00");const string id="FE7140555489";
        var registration=new Registration(id,"FNR10000",id,"FNR100","FNR100 v1.1 (Jan 7 2026 11:51:01) std","1.0","arm","4.4","uclibc","fixture",["exec","file","tunnel","router_config"]);
        var runtime=new DeviceRuntime(1732783,when);
        var session=new DeviceSession("fixture",registration,when,when,null,"",runtime);
        var profile=new DeviceProfile(1,"managed","FNR10000","","",null,null,null,1,"applied",null);
        var device=new Device(id,registration,"online",session,session,when,when,when,null,1,0,runtime,"240e:3b6:d051:2b31:56d0:b4ff:fe14:1af6",Profile:profile);
        var endpoints=new[]{new Endpoint("web","pcv6.criskaka.com",20000,"pcv6.criskaka.com:20000","closed","http://pcv6.criskaka.com:20000/"),new Endpoint("ssh","pcv6.criskaka.com",20001,"pcv6.criskaka.com:20001","closed",null),new Endpoint("telnet","pcv6.criskaka.com",20002,"pcv6.criskaka.com:20002","closed",null)};
        var history=new Maintenance("fixture",id,"fixture","closed","requested",when.AddMinutes(-30),when.AddMinutes(210),true,null,0,endpoints);
        var snapshot=new Snapshot([device],[],[],[],[history],"",when);
        var connection=new WorkspaceConnection(new Uri("http://127.0.0.1:1"),new DelegateHandler((_,_)=>Task.FromResult(Json("{\"data\":[]}"))));
        typeof(WorkspaceConnection).GetProperty("Synchronized")!.SetValue(connection,true);
        typeof(WorkspaceConnection).GetProperty("Status")!.SetValue(connection,"布局样例");
        typeof(WorkspaceConnection).GetProperty("Snapshot")!.SetValue(connection,snapshot);
        var window=(MainWindow)typeof(MainWindow).GetConstructor(BindingFlags.NonPublic|BindingFlags.Instance,null,[typeof(string),typeof(IpLocation)],null)!.Invoke([Path.Combine(output,"compact-preview-profile.json"),new IpLocation(new DelegateHandler((_,_)=>Task.FromResult(Json("{}"))))]);
        Set(window,"connection",connection);Set(window,"snapshot",snapshot);Set(window,"selectedDevice",id);
        Set(window,"fileScope",connection.Api.Origin+"|"+id);Set(window,"fileInitialized",true);
        Invoke(window,"ApplySnapshot");
        window.Title="Router Workbench · 紧凑布局样例";window.Width=1466;window.Height=913;
        var browser=Field<FrameworkElement>(window,"fileBrowser");
        var names=new[]{"cron.d","etc","ipq","ipsecetc","lib","nvram","oet","root","router-remote","sysinfo"};
        ((DataGrid)browser.GetType().GetProperty("Entries")!.GetValue(browser)!).ItemsSource=names.Select((name,i)=>new RemoteEntry(name,"d",new long[]{60,80,60,160,60,40,60,160,40,80}[i],DateTimeOffset.Parse("2026-08-21T21:58:13+08:00").ToUnixTimeSeconds())).ToArray();
        browser.GetType().GetProperty("Ready")!.SetValue(browser,true);
        Field<TextBlock>(browser,"status").Text="共 10 项";
        Field<TextBox>(window,"configKey").Text="SN";
        var detail=new TaskDetail("fixture",id,"router_config","success",when,"","",5,null,1,new("success",0,id,"",false,when,when),null,JsonSerializer.SerializeToElement(new{backend="nvram",operation="get",key="SN"}));
        Invoke(window,"ShowConfigResult",detail);Set(window,"configTaskId","");
        Invoke(window,"Navigate","files");
        window.Loaded+=(_,_)=>{Invoke(window,"UpdateEnabled");((TextBlock)window.FindName("StatusText")).Text="布局样例 · 数据仅用于视觉检查";};
        return window;
    }
}
