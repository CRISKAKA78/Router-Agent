using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;
public partial class MainWindow
{
    private DataGrid toolsGrid=null!,toolVersions=null!;
    private TextBox toolSearch=null!;
    private TextBlock toolDescription=null!,toolStatus=null!;
    private string selectedTool="",versionKey="";
    private Expander toolVersionSection=null!;
    private TextBlock toolEmpty=null!;
    private StackPanel toolEmptyPanel=null!;
    private Button deployToolButton=null!, clearToolSearch=null!;
    private UIElement BuildTools()
    {
        toolsGrid=Ui.Table("仓库工具",("工具名称","Name",-1),("说明","Description",-2));
        toolVersions=Ui.Table("工具版本",("版本","Version",-1),("产物数","Count",80),("发布时间","CreatedText",180));
        toolSearch=Ui.Input("",260);toolSearch.ToolTip="搜索工具名称或说明";toolSearch.Tag="搜索工具名称 / 说明";ControlChrome.SetIsSearch(toolSearch,true);System.Windows.Automation.AutomationProperties.SetName(toolSearch,"搜索仓库工具");toolSearch.TextChanged+=(_,_)=>UpdateTools();
        toolDescription=Ui.Text("请选择工具",true);toolDescription.TextWrapping=TextWrapping.Wrap;
        toolStatus=Ui.Text("",true);toolStatus.TextWrapping=TextWrapping.Wrap;
        toolsGrid.SelectionChanged+=(_,_)=>{if(refreshing)return;selectedTool=(toolsGrid.SelectedItem as Tool)?.ToolId??"";toolVersionSection.IsExpanded=true;UpdateToolSelection();_ = RefreshToolVersions();};
        toolVersions.SelectionChanged+=(_,_)=>UpdateToolActions();
        var menu=new ContextMenu();var deploy=new MenuItem {Header="投放到设备…"};menu.Items.Add(deploy);
        deploy.Click+=(_,_)=>_ = Run("投放仓库工具",DeployTool);menu.Opened+=(_,_)=>deploy.IsEnabled=Writable&&Device is {Online:true,Managed:true}&&selectedTool.Length>0;
        toolsGrid.ContextMenu=menu;toolsGrid.PreviewMouseRightButtonDown+=(_,e)=>{var row=TableBehavior.Ancestor<DataGridRow>(e.OriginalSource as DependencyObject);if(row!=null)row.IsSelected=true;};
        deployToolButton=Ui.Button("投放到设备…",()=>_ = Run("投放仓库工具",DeployTool),true);
        var versionBody=Ui.Page(Ui.Bar(deployToolButton,toolDescription),toolVersions,new Border {Child=toolStatus,Padding=new(12,7,12,7)});
        toolVersionSection=CompactWorkspace.History("工具版本",versionBody);toolVersionSection.Visibility=Visibility.Collapsed;
        toolEmpty=Ui.Text("连接服务器后查看仓库工具",true);toolEmpty.TextWrapping=TextWrapping.Wrap;toolEmpty.TextAlignment=TextAlignment.Center;
        clearToolSearch=Ui.Button("清除搜索",()=>toolSearch.Clear());
        toolEmptyPanel=new StackPanel {HorizontalAlignment=HorizontalAlignment.Center,VerticalAlignment=VerticalAlignment.Center,Margin=new(24)};
        toolEmptyPanel.Children.Add(toolEmpty);
        var emptyActions=new WrapPanel {HorizontalAlignment=HorizontalAlignment.Center,Margin=new(0,12,0,0)};
        emptyActions.Children.Add(clearToolSearch);emptyActions.Children.Add(Ui.Button("刷新",RefreshTools));toolEmptyPanel.Children.Add(emptyActions);
        var list=new Grid {Margin=new(16)};list.Children.Add(toolsGrid);list.Children.Add(toolEmptyPanel);
        toolsGrid.MinRowHeight=32;
        return Ui.Page(CompactWorkspace.Toolbar(toolSearch,CompactWorkspace.Action("刷新","refresh",RefreshTools)),CompactWorkspace.WithHistory(list,toolVersionSection));
    }
    private void RefreshTools(){versionKey="";connection?.Invalidate();_ = RefreshToolVersions();}
    private void UpdateToolActions()
    {
        if(deployToolButton==null)return;
        deployToolButton.IsEnabled=Writable&&Device is {Online:true,Managed:true}&&selectedTool!=""&&toolVersions.SelectedItem is ToolVersion {Archived:false};
        deployToolButton.ToolTip=deployToolButton.IsEnabled?"确认版本、兼容产物和目标目录后投放，不自动执行":"请选择工具版本及在线设备后投放";
    }
    private void UpdateToolSelection()
    {
        toolVersionSection.Visibility=selectedTool==""?Visibility.Collapsed:Visibility.Visible;
        toolVersionSection.Header="工具版本 · "+(toolsGrid.SelectedItem as Tool)?.Name;
        if(selectedTool==""){toolVersions.ItemsSource=null;toolDescription.Text="";toolStatus.Text="";versionKey="";}
        UpdateToolActions();
    }
    private void UpdateTools()
    {
        if(toolsGrid==null)return;
        var rows=snapshot.Tools.Where(t=>!t.Archived&&$"{t.Name} {t.Description} {t.ToolId}".Contains(toolSearch.Text,StringComparison.OrdinalIgnoreCase)).ToArray();
        var was=refreshing;refreshing=true;Ui.SetRows(toolsGrid,rows);toolsGrid.SelectedItem=rows.FirstOrDefault(t=>t.ToolId==selectedTool);refreshing=was;
        if(toolsGrid.SelectedItem==null)selectedTool="";
        var synchronized=connection?.Synchronized==true;
        toolEmpty.Text=connection==null?"连接服务器后查看仓库工具":!synchronized?connection.Error!=""?"快照加载失败，请刷新重试。\n"+connection.Error:connection.Status:toolSearch.Text.Length>0?"没有匹配的工具，请调整关键词。":"仓库暂无工具。可刷新检查最新内容。";
        toolEmptyPanel.Visibility=rows.Length==0?Visibility.Visible:Visibility.Collapsed;
        clearToolSearch.Visibility=toolSearch.Text.Length>0?Visibility.Visible:Visibility.Collapsed;
        UpdateToolSelection();
        if(Page=="tools")_ = RefreshToolVersions();
    }
    private async Task RefreshToolVersions()
    {
        var owner=connection;var id=selectedTool;if(owner?.Synchronized!=true||id.Length==0)return;
        var key=owner.Api.Origin+"|"+id;if(versionKey==key)return;versionKey=key;
        toolVersions.ItemsSource=null;toolStatus.Text="正在加载版本…";UpdateToolActions();
        toolDescription.Text=snapshot.Tools.FirstOrDefault(t=>t.ToolId==id)?.Description??"";
        try{var versions=await owner.TrackAsync(()=>owner.Api.ListAsync<ToolVersion>($"tools/{Id(id)}/versions"));
            if(owner!=connection||id!=selectedTool||closing)return;
            toolVersions.ItemsSource=versions.OrderByDescending(v=>v.CreatedAt).ToArray();toolStatus.Text=versions.Length==0?"此工具尚无发布版本，可刷新检查最新内容。":"选择版本后确认兼容产物和目标目录；投放不会自动执行。";
        }catch(Exception error){if(owner==connection&&id==selectedTool){toolStatus.Text=error.Message;versionKey="";}}
    }
    private async Task DeployTool()
    {
        var device=RequireDevice();var owner=Connected();var tool=snapshot.Tools.FirstOrDefault(t=>t.ToolId==selectedTool)??throw new InvalidOperationException("请选择仓库工具。");
        var versions=await owner.TrackAsync(()=>owner.Api.ListAsync<ToolVersion>($"tools/{Id(tool.ToolId)}/versions"));
        if(owner!=connection||device.DeviceId!=selectedDevice||closing)return;
        var model=new ComboBox {ItemsSource=versions,DisplayMemberPath="Version",SelectedItem=versions.FirstOrDefault(v=>v.Version==(toolVersions.SelectedItem as ToolVersion)?.Version)};
        var artifacts=new ComboBox {DisplayMemberPath="Label"};var path=Ui.Input("/tmp");var overwrite=new CheckBox {Content="覆盖已存在文件"};
        var info=Ui.Text("请选择要投放的版本。",true);info.TextWrapping=TextWrapping.Wrap;
        var body=new StackPanel {Children={Ui.Note($"工具：{tool.Name}\n目标设备：{device.DisplayName}（{device.DeviceId}）"),Ui.Labeled("工具版本",model),Ui.Labeled("兼容产物",artifacts),Ui.Labeled("目标目录",path),overwrite,info}};
        int generation=0;
        async Task LoadCandidates(){var turn=++generation;artifacts.ItemsSource=null;info.Text="正在检查兼容性…";if(model.SelectedItem is not ToolVersion selected){info.Text="请选择版本。";return;}
            try{var matches=await owner.TrackAsync(()=>owner.Api.ListAsync<Compatibility>($"tools/{Id(tool.ToolId)}/versions/{Id(selected.Version)}/compatibility?device_id={Id(device.DeviceId)}"));
                var candidates=new List<ToolCandidate>();foreach(var match in matches.Where(m=>m.Status=="compatible")){var asset=await owner.TrackAsync(()=>owner.Api.GetAsync<Asset>($"assets/{Id(match.Artifact.AssetId)}"));candidates.Add(new(match.Artifact,asset));}
                if(turn!=generation||owner!=connection)return;artifacts.ItemsSource=candidates;
                info.Text=candidates.Count==0?"没有可投放产物。"+string.Join("；",matches.Select(m=>m.Reason)):candidates.Count>1?"请选择兼容产物。":"";
                if(candidates.Count==1)artifacts.SelectedIndex=0;
            }catch(Exception error){if(turn==generation)info.Text=error.Message;}}
        ActionWindow? dialog=null;
        var browse=Ui.Button("浏览设备目录…",()=>{var chosen=SelectDeviceDirectory(dialog!,device,path.Text);if(chosen!=null)path.Text=chosen;});body.Children.Insert(4,Ui.FormActions(browse));
        void Describe(){if(artifacts.SelectedItem is ToolCandidate candidate)info.Text="目标文件："+RemoteDirectory.Join(path.Text,candidate.Asset.Name);}
        artifacts.SelectionChanged+=(_,_)=>{try{Describe();}catch(ArgumentException){info.Text="请输入绝对目录路径。";}};path.TextChanged+=(_,_)=>{try{Describe();}catch(ArgumentException){info.Text="请输入绝对目录路径。";}};
        model.SelectionChanged+=async(_,_)=>await LoadCandidates();
        dialog=new ActionWindow(this,"确认投放工具",new ScrollViewer {Content=body,VerticalScrollBarVisibility=ScrollBarVisibility.Auto},"确认投放",async()=>{
            if(owner!=connection||selectedDevice!=device.DeviceId||closing)throw new OperationCanceledException();
            if(model.SelectedItem is not ToolVersion version||artifacts.SelectedItem is not ToolCandidate candidate)throw new InvalidOperationException("请选择版本和兼容产物。");
            var response=await Write(new("投放工具","deployments",new{device_id=device.DeviceId,tool_id=tool.ToolId,version=version.Version,artifact_id=candidate.Artifact.ArtifactId,remote_path=RemoteDirectory.Join(path.Text,candidate.Asset.Name),overwrite=overwrite.IsChecked==true,timeout_seconds=120}));
            ShowCreatedTask(response);toolStatus.Text="投放任务已提交，等待设备确认。";
        });
        dialog.Loaded+=async(_,_)=>{if(model.SelectedItem!=null)await LoadCandidates();};dialog.Closed+=(_,_)=>generation++;dialog.ShowDialog();
    }
    private sealed record ToolCandidate(Artifact Artifact,Asset Asset){public string Label=>Asset.Name+" · "+string.Join('/',Artifact.Rules.Arch)+" / "+string.Join('/',Artifact.Rules.Libc);public override string ToString()=>Label;}
}
