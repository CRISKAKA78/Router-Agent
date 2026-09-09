using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

internal sealed class DeviceDirectoryView : UserControl
{
    private readonly Func<WorkspaceConnection?> connection;
    private readonly Func<Device?> device;
    private CancellationTokenSource cancel=new();
    private readonly TextBox path=Ui.Input("/tmp");
    private readonly TextBlock status=Ui.Text("",true);
    public DataGrid Entries {get;}=Ui.Table("设备目录",("名称","Name",-1),("类型","KindText",80),("大小","SizeText",100),("修改时间","ModifiedText",160));
    public string CurrentPath {get;private set;}="/tmp";
    public bool Ready {get;private set;}
    public DeviceDirectoryView(Func<WorkspaceConnection?> connection,Func<Device?> device)
    {
        this.connection=connection;this.device=device;
        var address=new DockPanel();var buttons=new StackPanel {Orientation=Orientation.Horizontal};DockPanel.SetDock(buttons,Dock.Right);address.Children.Add(buttons);
        buttons.Children.Add(Ui.Button("上一级",()=>_ = LoadDirectory(RemoteDirectory.Parent(CurrentPath))));buttons.Children.Add(Ui.Button("刷新",()=>_ = LoadDirectory(path.Text)));
        address.Children.Add(path);System.Windows.Automation.AutomationProperties.SetName(path,"设备目录路径");
        path.KeyDown+=(_,e)=>{if(e.Key==Key.Enter){e.Handled=true;_ = LoadDirectory(path.Text);}};
        Entries.MouseDoubleClick+=(_,e)=>{if(e.OriginalSource is DependencyObject source&&ItemsControl.ContainerFromElement(Entries,source) is DataGridRow {Item:RemoteEntry {Kind:"d"} entry})_ = LoadDirectory(RemoteDirectory.Join(CurrentPath,entry.Name));};
        Content=Ui.Page(new Border {Child=address,Padding=new(8)},Entries,Ui.Note(""));
        var panel=(DockPanel)Content;panel.Children.RemoveAt(1);DockPanel.SetDock(status,Dock.Bottom);status.Margin=new(10,6,10,6);status.TextWrapping=TextWrapping.Wrap;panel.Children.Insert(1,status);
        Unloaded+=(_,_)=>Stop();
    }
    public void Reset(){Stop();Ready=false;CurrentPath="/tmp";path.Text="/tmp";Entries.ItemsSource=null;status.Text="请选择在线设备。";}
    public void Stop(){cancel.Cancel();cancel.Dispose();cancel=new();}
    public async Task LoadDirectory(string directory)
    {
        Stop();var token=cancel.Token;var owner=connection();var target=device();Ready=false;
        if(owner?.Synchronized!=true||target is not {Online:true,Managed:true}){Entries.ItemsSource=null;status.Text="请选择在线且已纳管的设备。";return;}
        if(owner.Busy||owner.Pending!=null){status.Text="请先完成当前请求，再刷新目录。";return;}
        status.Text="正在读取目录…";
        try{
            directory=RemoteDirectory.Normalize(directory);Entries.ItemsSource=null;
            var result=await RemoteDirectory.ReadAsync(owner,target.DeviceId,directory,token);
            if(token.IsCancellationRequested||owner!=connection()||device()?.DeviceId!=target.DeviceId)return;
            CurrentPath=directory;path.Text=directory;Entries.ItemsSource=result.Entries;Ready=true;
            status.Text=result.Limited?"仅显示前250项，请进入子目录查看。":result.Entries.Length==0?"目录为空":"共 "+result.Entries.Length+" 项";
        }catch(OperationCanceledException){if(!token.IsCancellationRequested)status.Text="读取目录超时，请刷新重试。";}
        catch(Exception error){if(!token.IsCancellationRequested)status.Text=error.Message;}
    }
}
