using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using Microsoft.Win32;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;
public partial class MainWindow
{
    private DeviceDirectoryView fileBrowser=null!;
    private TextBlock exchangeStatus=null!;
    private CancellationTokenSource fileCancel=new();
    private FileExchange? exchange;
    private string fileScope="";
    private bool fileInitialized;
    private UIElement BuildFiles()
    {
        fileBrowser=new(()=>connection,()=>Device);exchangeStatus=Ui.Text("",true);exchangeStatus.TextWrapping=TextWrapping.Wrap;
        return Ui.Page(Ui.Bar(DeviceButton("上传本地文件…",()=>_ = Run("上传文件",UploadLocalFile)),DeviceButton("下载所选文件…",()=>_ = Run("下载文件",DownloadSelectedFile)),
            Ui.Button("继续原传输",()=>_ = Run("继续传输",ResumeExchange)),exchangeStatus),Ui.Split(fileBrowser,BuildTransfers(),true,1.7));
    }
    private void UpdateFiles(){if(exchangeStatus!=null)exchangeStatus.Text=exchange?.DeviceId==selectedDevice?exchange.Status:"";}
    private void ResetFileWorkspace(){fileCancel.Cancel();fileCancel.Dispose();fileCancel=new();fileBrowser?.Reset();exchange?.Dispose();exchange=null;fileScope="";fileInitialized=false;}
    private void EnsureFileScope()
    {
        if(fileBrowser==null)return;var scope=(connection?.Api.Origin.ToString()??"")+"|"+selectedDevice;
        if(scope!=fileScope){fileCancel.Cancel();fileCancel.Dispose();fileCancel=new();fileBrowser.Reset();fileScope=scope;fileInitialized=false;}
        if(Page=="files"&&!fileInitialized&&Device is {Online:true,Managed:true}&&connection is {Synchronized:true,Busy:false,Pending:null}){fileInitialized=true;_ = fileBrowser.LoadDirectory("/tmp");}
    }
    private async Task RunExchange(FileExchange operation)
    {
        if(operation.Owner!=connection||operation.DeviceId!=selectedDevice)throw new InvalidOperationException("请选择原设备和服务器后再继续。");
        var token=fileCancel.Token;
        try{await operation.RunAsync(token);if(!token.IsCancellationRequested){Log("文件",operation.Status);if(!operation.Download)await fileBrowser.LoadDirectory(fileBrowser.CurrentPath);}}
        finally{if(!token.IsCancellationRequested&&operation.Owner==connection&&operation.DeviceId==selectedDevice){taskId=operation.TaskId;UpdateFiles();connection.Invalidate();_ = RefreshDetails();}}
    }
    private async Task StartExchange(string local,string remote,bool download,bool overwrite=false)
    {
        var device=RequireDevice();if(!device.Managed)throw new InvalidOperationException("请先纳管设备。");
        remote=RemoteDirectory.Normalize(remote);RemoteDirectory.FileName(remote);
        exchange?.Dispose();exchange=new(Connected(),device.DeviceId,local,remote,download,overwrite);var current=exchange;
        current.Changed+=()=>{if(exchange==current&&current.DeviceId==selectedDevice)UpdateFiles();};
        await RunExchange(current);
    }
    private Task ResumeExchange()
    {
        if(exchange==null)throw new InvalidOperationException("没有可继续的本地传输。");
        if(exchange.PendingStage!=null&&connection?.Pending==exchange.PendingStage&&!Confirm("重试原请求","确认服务器未重启并已核对原操作；使用原请求继续传输？"))return Task.CompletedTask;
        return RunExchange(exchange);
    }
    private async Task UploadLocalFile()
    {
        var device=RequireDevice();var owner=Connected();var picker=new OpenFileDialog {Title="选择上传到设备的文件",CheckFileExists=true};
        if(picker.ShowDialog(this)!=true)return;
        Dictionary<string,string>? chosen=null;
        Form("上传文件",$"{System.IO.Path.GetFileName(picker.FileName)} → {device.DisplayName}",
            [new("path","设备目标文件路径","/tmp/"+System.IO.Path.GetFileName(picker.FileName)),new("overwrite","覆盖已存在文件","否",Choices:["否","是"])],values=>{
                RemoteDirectory.FileName(values["path"]);chosen=values;return Task.CompletedTask;});
        if(chosen!=null){if(owner!=connection||device.DeviceId!=selectedDevice)throw new OperationCanceledException();await StartExchange(picker.FileName,chosen["path"],false,chosen["overwrite"]=="是");}
    }
    private async Task DownloadSelectedFile()
    {
        RequireDevice();if(!fileBrowser.Ready||fileBrowser.Entries.SelectedItem is not RemoteEntry {Kind:"f"} entry)throw new InvalidOperationException("请选择设备目录中的文件。");
        var dialog=new SaveFileDialog {Title="保存设备文件到本机",FileName=entry.Name,OverwritePrompt=true};
        if(dialog.ShowDialog(this)==true)await StartExchange(dialog.FileName,RemoteDirectory.Join(fileBrowser.CurrentPath,entry.Name),true);
    }
    private string? SelectDeviceDirectory(Window parent,Device device,string initial)
    {
        var owner=Connected();var browser=new DeviceDirectoryView(()=>owner==connection?owner:null,()=>owner==connection&&device.DeviceId==selectedDevice?Device:null);
        string? chosen=null;
        var dialog=new ActionWindow(parent,"选择设备目录",browser,"选择此目录",()=>{if(!browser.Ready)throw new InvalidOperationException("请等待目录读取成功。");chosen=browser.CurrentPath;return Task.CompletedTask;});
        dialog.Loaded+=async(_,_)=>await browser.LoadDirectory(initial);dialog.Closed+=(_,_)=>browser.Stop();dialog.ShowDialog();return chosen;
    }
}
