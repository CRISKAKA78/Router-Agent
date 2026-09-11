using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Data;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

internal sealed class DeviceDirectoryView : UserControl
{
    private readonly Func<WorkspaceConnection?> connection;
    private readonly Func<Device?> device;
    private CancellationTokenSource cancel=new();
    private readonly TextBox path=Ui.Input("/tmp/root");
    private readonly TextBlock status=Ui.Text("",true);
    public DataGrid Entries {get;}=Ui.Table("设备目录",("名称","Name",-1),("类型","KindText",80),("大小","SizeText",100),("修改时间","ModifiedText",160));
    public string CurrentPath {get;private set;}="/tmp/root";
    public bool Ready {get;private set;}
    public event Action? StateChanged;
    public DeviceDirectoryView(Func<WorkspaceConnection?> connection,Func<Device?> device)
    {
        this.connection=connection;this.device=device;
        var address=new DockPanel();var buttons=new StackPanel {Orientation=Orientation.Horizontal,Margin=new(6,0,0,0)};DockPanel.SetDock(buttons,Dock.Right);address.Children.Add(buttons);
        buttons.Children.Add(CompactWorkspace.Action("上一级","arrow-up",()=>_ = LoadDirectory(RemoteDirectory.Parent(CurrentPath))));buttons.Children.Add(CompactWorkspace.Action("刷新","refresh",()=>_ = LoadDirectory(path.Text)));
        address.Children.Add(path);System.Windows.Automation.AutomationProperties.SetName(path,"设备目录路径");
        path.KeyDown+=(_,e)=>{if(e.Key==Key.Enter){e.Handled=true;_ = LoadDirectory(path.Text);}};
        Entries.MouseDoubleClick+=(_,e)=>{if(e.OriginalSource is DependencyObject source&&ItemsControl.ContainerFromElement(Entries,source) is DataGridRow {Item:RemoteEntry {Kind:"d"} entry})_ = LoadDirectory(RemoteDirectory.Join(CurrentPath,entry.Name));};
        Entries.PreviewKeyDown += (_,e) => { if(e.Key == Key.Enter && Entries.SelectedItem is RemoteEntry {Kind:"d"} entry) { e.Handled=true; _ = LoadDirectory(RemoteDirectory.Join(CurrentPath,entry.Name)); } };
        Entries.SelectionChanged += (_,_) => StateChanged?.Invoke();
        var name = new FrameworkElementFactory(typeof(DockPanel));
        var icon = new FrameworkElementFactory(typeof(WorkbenchIcon)); icon.SetValue(FrameworkElement.MarginProperty,new Thickness(4,0,12,0));
        icon.SetValue(FrameworkElement.WidthProperty,20d);icon.SetValue(FrameworkElement.HeightProperty,20d);
        var iconStyle = new Style(typeof(WorkbenchIcon)); iconStyle.Setters.Add(new Setter(WorkbenchIcon.KindProperty,"file"));
        iconStyle.Setters.Add(new Setter(Control.ForegroundProperty,new DynamicResourceExtension("Accent")));
        var folder = new DataTrigger {Binding=new Binding("Kind"),Value="d"};folder.Setters.Add(new Setter(WorkbenchIcon.KindProperty,"files"));iconStyle.Triggers.Add(folder);
        icon.SetValue(FrameworkElement.StyleProperty,iconStyle); name.AppendChild(icon);
        var label = new FrameworkElementFactory(typeof(TextBlock));label.SetBinding(TextBlock.TextProperty,new Binding("Name"));label.SetValue(TextBlock.TextAlignmentProperty,TextAlignment.Left);
        label.SetValue(TextBlock.TextTrimmingProperty,TextTrimming.CharacterEllipsis);label.SetBinding(FrameworkElement.ToolTipProperty,new Binding("Name"));
        label.SetBinding(TextBlock.TextWrappingProperty,new Binding {RelativeSource=new(RelativeSourceMode.Self),Path=new PropertyPath(TableBehavior.WrapModeProperty)});name.AppendChild(label);
        Entries.Columns[0] = new DataGridTemplateColumn {Header="名称",SortMemberPath="Name",ClipboardContentBinding=new Binding("Name"),CellTemplate=new DataTemplate {VisualTree=name},Width=new(1.6,DataGridLengthUnitType.Star),MinWidth=170};
        for(var i=1;i<Entries.Columns.Count;i++) { Entries.Columns[i].Width=new(i==3?1.2:.8,DataGridLengthUnitType.Star); Entries.Columns[i].MinWidth=i==3?175:70; }
        Entries.MinRowHeight=36;Entries.GridLinesVisibility=DataGridGridLinesVisibility.Horizontal;Entries.SetResourceReference(DataGrid.HorizontalGridLinesBrushProperty,"Line");
        Entries.SetResourceReference(Control.FontSizeProperty,"UiControlFontSize");
        status.Margin=new(0,12,0,12);status.TextWrapping=TextWrapping.Wrap;
        var top=new StackPanel {Children={address,status}};
        Content=new Border {Padding=new(16,10,16,16),Child=Ui.Page(top,Entries)};
        Unloaded+=(_,_)=>Stop();
    }
    public void Reset(){Stop();Ready=false;CurrentPath="/tmp/root";path.Text="/tmp/root";Entries.ItemsSource=null;status.Text="请选择在线设备。";StateChanged?.Invoke();}
    public void Stop(){cancel.Cancel();cancel.Dispose();cancel=new();}
    public async Task LoadDirectory(string directory)
    {
        Stop();var token=cancel.Token;var owner=connection();var target=device();Ready=false;StateChanged?.Invoke();
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
        finally{if(!token.IsCancellationRequested)StateChanged?.Invoke();}
    }
}
