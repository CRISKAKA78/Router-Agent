using System.IO;
using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static async Task ExchangeWorkspaceChecks(MainWindow window,ApiClient api,WorkspaceConnection connection,Tool tool)
    {
        await Eventually(()=>Task.FromResult(!connection.Busy&&Field<object>(window,"fileBrowser").GetType().GetProperty("Ready")!.GetValue(Field<object>(window,"fileBrowser")) is true),"file page automatically loads tmp directory");
        var directory=await RemoteDirectory.ReadAsync(connection,"desktop-router-02","/tmp/a'b",default);
        Check(directory.Entries.Any(e=>e.Name=="line\nname"),"device directory travels through existing exec API and preserves unusual names");
        var source=Path.Combine(output,"exchange-source.bin");await File.WriteAllBytesAsync(source,Enumerable.Range(0,4096).Select(n=>(byte)n).ToArray());
        await InvokeAsync(window,"StartExchange",source,"/tmp/custom file.bin",false,false);
        var upload=Field<FileExchange>(window,"exchange");Check(upload.Complete&&upload.TaskId.Length>0,"local upload prepares file and sends it with the native file API");
        var target=Path.Combine(output,"exchange-returned.bin");await InvokeAsync(window,"StartExchange",target,"/tmp/custom file.bin",true,false);
        var download=Field<FileExchange>(window,"exchange");
        var sent=await File.ReadAllBytesAsync(source);var received=await File.ReadAllBytesAsync(target);
        Check(download.Complete&&sent.SequenceEqual(received),"download automatically completes and saves verified identical local bytes");
        var original=download.TaskId;await InvokeAsync(window,"ResumeExchange");Check(download.TaskId==original,"completed exchange cannot create a replacement task");
        connection.Invalidate();await Eventually(()=>Task.FromResult(connection.Snapshot.Tools.Any(t=>t.ToolId==tool.ToolId)),"repository tools loaded through public snapshot");
        Invoke(window,"Navigate","tools");Field<TextBox>(window,"toolSearch").Text="网络诊断";var grid=Field<DataGrid>(window,"toolsGrid");grid.SelectedItem=grid.Items.Cast<Tool>().Single(t=>t.ToolId==tool.ToolId);
        await Eventually(()=>Task.FromResult(Field<DataGrid>(window,"toolVersions").Items.Count==1),"selected tool shows current published versions");
        var versions=Field<DataGrid>(window,"toolVersions");versions.SelectedIndex=0;
        async Task OpenDeployment(bool accept){
            var done=new TaskCompletionSource();_ = window.Dispatcher.BeginInvoke(new Action(async()=>{try{await InvokeAsync(window,"DeployTool");done.SetResult();}catch(Exception e){done.SetException(e);}}));
            await Eventually(()=>Task.FromResult(Application.Current.Windows.Cast<Window>().Any(w=>w.Title=="确认投放工具")),"tool context action opens explicit deployment dialog");
            var dialog=Application.Current.Windows.Cast<Window>().Single(w=>w.Title=="确认投放工具");dialog.UpdateLayout();
            await Eventually(()=>Task.FromResult(Visuals<ComboBox>(dialog).Any(c=>c.SelectedItem?.GetType().Name=="ToolCandidate")),"server-authoritative compatible artifact selected");
            Check(Visuals<TextBlock>(dialog).Any(t=>t.Text.StartsWith("目标文件：/tmp/",StringComparison.Ordinal)),"deployment dialog displays the full target before confirmation");
            Check(Visuals<TextBox>(dialog).Single(t=>System.Windows.Automation.AutomationProperties.GetName(t)=="目标目录").Text=="/tmp","deployment directory defaults to tmp");
            Render(dialog,"tool-deploy.png");
            if(accept){Visuals<TextBox>(dialog).Single(t=>System.Windows.Automation.AutomationProperties.GetName(t)=="目标目录").Text="/tmp/custom tools";Visuals<Button>(dialog).Single(b=>b.Content?.ToString()=="确认投放").RaiseEvent(new RoutedEventArgs(Button.ClickEvent));}else dialog.Close();
            await done.Task;
        }
        var before=(await api.ListAsync<TaskSummary>("tasks")).Length;await OpenDeployment(false);
        Check((await api.ListAsync<TaskSummary>("tasks")).Length==before,"cancelled tool dialog creates no task");await OpenDeployment(true);
        var deployment=Field<string>(window,"taskId");await Eventually(async()=> (await api.GetAsync<TaskDetail>("tasks/"+deployment)).Result?.Status=="success","confirmed tool deployment succeeds");
        var op=await api.GetAsync<Operation>($"tasks/{deployment}/operation");Check(op.ToolId==tool.ToolId&&op.Version=="1.0","deployment preserves selected tool identity and version");
        var deployed=await api.GetAsync<TaskDetail>($"tasks/{deployment}");
        Check(deployed.Params.GetProperty("remote_path").GetString()!.StartsWith("/tmp/custom tools/",StringComparison.Ordinal),"deployment uses the confirmed custom directory");
        Render(window,"tools-workspace.png");
        var settingsDone=new TaskCompletionSource();_ = window.Dispatcher.BeginInvoke(new Action(()=>{Invoke(window,"OpenSettings");settingsDone.SetResult();}));
        await Eventually(()=>Task.FromResult(Application.Current.Windows.Cast<Window>().Any(w=>w.Title=="设置")),"top settings opens modal without workspace navigation");
        var settings=Application.Current.Windows.Cast<Window>().Single(w=>w.Title=="设置");settings.UpdateLayout();Render(settings,"settings-dialog.png");settings.Close();await settingsDone.Task;
        var device=connection.Snapshot.Devices.Single(d=>d.DeviceId=="desktop-router-02");
        Invoke(window,"UpdatePropertyPages",device with{Presentation=new([],[],StorageVisible:false)});
        Check(!Field<TabControl>(window,"overviewTabs").Items.Cast<TabItem>().Any(t=>t.Header.ToString()=="存储空间"),"applied presentation can hide storage page independently");
        Invoke(window,"UpdatePropertyPages",device);Check(Field<TabControl>(window,"overviewTabs").Items.Cast<TabItem>().Any(t=>t.Header.ToString()=="存储空间"),"omitted storage option retains default visible page");
        Invoke(window,"Navigate","overview");
    }
}
