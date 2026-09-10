using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static async Task WorkspaceChecks(MainWindow window, ApiClient api, WorkspaceConnection connection)
    {
        var devices = (DataGrid)window.FindName("DevicesGrid");
        var device = connection.Snapshot.Devices.Single(d=>d.DeviceId=="managed-ui");
        Check(devices.ContextMenu.Items.Cast<MenuItem>().Select(i=>i.Header.ToString()).SequenceEqual(new[]{"复制","修改设备名…","更新模板…"}),"device context menu has requested operations");
        string copied;
        for (var attempt = 0; ; attempt++) {
            try {
                devices.ContextMenu.Items.Cast<MenuItem>().First().RaiseEvent(new RoutedEventArgs(MenuItem.ClickEvent));
                copied = Clipboard.GetText(); break;
            }
            // The desktop clipboard is shared with the user's applications; retry only its transient busy error.
            catch (System.Runtime.InteropServices.COMException error) when (error.HResult == unchecked((int)0x800401D0) && attempt < 4) { await Task.Delay(250); }
        }
        Check(copied.Contains("managed-ui")&&copied.Contains(device.DisplayName),"device context copy includes selected device name and identity");
        async Task Submit(string method,string title,string? name=null,string? templateChoice=null,bool cancel=false) {
            var done=new TaskCompletionSource();
            _=window.Dispatcher.BeginInvoke(new Action(async()=>{try{await InvokeAsync(window,method,device);done.SetResult();}catch(Exception error){done.SetException(error);}}));
            await Eventually(()=>Task.FromResult(Application.Current.Windows.Cast<Window>().Any(w=>w.Owner==window&&w.Title==title)),title+" opens");
            var dialog=Application.Current.Windows.Cast<Window>().Single(w=>w.Owner==window&&w.Title==title);dialog.UpdateLayout();
            if(name!=null)Visuals<TextBox>(dialog).Single().Text=name;
            if(templateChoice!=null){Visuals<TextBox>(dialog).Single().Text=templateChoice;var list=Visuals<ListBox>(dialog).Single();Check(list.Items.Count==1,"template search filters selectable templates");list.SelectedIndex=0;}
            Render(dialog,method+".png");
            if(cancel)dialog.Close();else Visuals<Button>(dialog).Single(b=>b.Content?.ToString()==(method=="UpdateDeviceTemplate"?"应用":"确定")).RaiseEvent(new RoutedEventArgs(Button.ClickEvent));
            await Eventually(()=>Task.FromResult(done.Task.IsCompleted),title+" submitted");await done.Task;
        }
        var generation=device.Profile!.TemplateGeneration;
        await Submit("RenameDevice","修改设备名称","已重命名设备");
        await Eventually(()=>Task.FromResult(connection.Snapshot.Devices.Single(d=>d.DeviceId==device.DeviceId).DisplayName=="已重命名设备"),"context rename updates admin name");
        device=connection.Snapshot.Devices.Single(d=>d.DeviceId==device.DeviceId);
        Check(device.Registration.Hostname=="探测名称"&&device.Profile!.TemplateGeneration==generation,"rename preserves registration and collection generation");
        await Submit("UpdateDeviceTemplate","选择设备模板");
        await Eventually(()=>Task.FromResult(connection.Snapshot.Devices.Single(d=>d.DeviceId==device.DeviceId).Profile is {ConfigurationState:"applied"} p&&p.TemplateGeneration==generation+1),"same-version context application awaits probe ACK and advances generation");
        var templateId=device.Profile!.BoundTemplate!.TemplateId;
        await api.ExecuteAsync(new("更新测试模板","probe-templates/"+templateId,new{version=1,name="更新后的模板",properties=new{signal=new{name="信号",command="printf 95",interval_seconds=7}},presentation=new{fields=new{signal=new{group_id="builtin_resources",order=1}}}},"PUT"));
        connection.Invalidate();
        await Eventually(()=>Task.FromResult(connection.Snapshot.Devices.Single(d=>d.DeviceId==device.DeviceId).Profile?.LatestTemplate?.Version==2),"latest template version appears without applying it");
        var updateButton = ((DeviceSummary)window.FindName("Summary")).TemplateButton;
        Check(updateButton.Visibility == Visibility.Visible && connection.Snapshot.Devices.Single(d=>d.DeviceId==device.DeviceId).ActiveTemplate?.Version == 1,"update icon appears while applied version remains unchanged");
        device=connection.Snapshot.Devices.Single(d=>d.DeviceId==device.DeviceId);await Submit("UpdateDeviceTemplate","选择设备模板");
        await Eventually(()=>Task.FromResult(connection.Snapshot.Devices.Single(d=>d.DeviceId==device.DeviceId).ActiveTemplate?.Version==2),"template update reflects actual acknowledged version");
        Check(updateButton.Visibility == Visibility.Collapsed,"update icon disappears after latest template applies");
        Check(!Field<PropertyRow[]>(window,"propertyRows").Any(r=>r.Key=="session_id"),"session identifier hidden by default");
        await InvokeAsync(window,"RefreshDetails");
        var periods=await api.ListAsync<ConnectionPeriod>("devices/managed-ui/connections");
        var period=periods.Single();Check(period.State=="online"&&period.OfflineSeconds==null,"live connection history is separate from session details");
        var row=new ConnectionRow(period);var before=row.OnlineDuration;row.Advance(5);Check(row.OnlineDuration!=before&&row.OfflineDuration=="—","active online duration advances alone");
        row=new ConnectionRow(period with{State="offline",OnlineSeconds=60,OfflineSeconds=5});row.Advance(7);Check(row.OnlineDuration=="1分钟0秒"&&row.OfflineDuration=="12秒","offline duration advances after online duration freezes");
        row=new ConnectionRow(period with{State="completed",OnlineSeconds=60,OfflineSeconds=5});row.Advance(7);Check(row.OnlineDuration=="1分钟0秒"&&row.OfflineDuration=="5秒","completed periods remain fixed");
        var tabs=Field<TabControl>(window,"overviewTabs");tabs.SelectedItem=tabs.Items.Cast<TabItem>().Single(t=>t.Header.ToString()=="连接历史");await Task.Delay(100);Render(window,"connection-history.png");tabs.SelectedIndex=0;
        var alternate=(await api.ExecuteAsync(new("另一模板","probe-templates",new{name="可选择的备用模板",properties=new{signal=new{name="信号",command="printf 88"}}}))).Deserialize<ProbeTemplate>(ApiJson.Options)!;
        device=await api.GetAsync<Device>("devices/"+device.DeviceId);var unchanged=device.Profile!.TemplateGeneration;
        await Submit("UpdateDeviceTemplate","选择设备模板",templateChoice:alternate.Name,cancel:true);
        Check((await api.GetAsync<Device>("devices/"+device.DeviceId)).Profile!.TemplateGeneration==unchanged,"cancelled template selection leaves applied generation unchanged");
        await Submit("UpdateDeviceTemplate","选择设备模板",templateChoice:alternate.Name);
        await Eventually(()=>Task.FromResult(connection.Snapshot.Devices.Single(d=>d.DeviceId==device.DeviceId).ActiveTemplate?.TemplateId==alternate.TemplateId),"explicit selection applies a different template after probe acknowledgement");
    }
}
