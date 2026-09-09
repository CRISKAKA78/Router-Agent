using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Data;
using System.Windows.Media;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private DataGrid discoveries=null!,switchPorts=null!;
    private ComboBox discoveryState=null!;
    private TextBlock discoveryNote=null!;
    private static string ConfigState(string? state)=>state switch {"applied"=>"已生效","waiting_dispatch"=>"等待下发","waiting_confirmation"=>"等待探针确认","failed"=>"应用失败","not_managed"=>"未纳管",_=>"未提供"};
 private UIElement BuildDiscoveries()
    {
        discoveries=Ui.Table("设备发现",("设备名称","DisplayName",140),("设备ID","DeviceId",150),("状态","StatusText",60));
        discoveries.SelectionChanged += DeviceSelectionChanged;
        discoveries.SelectionMode=DataGridSelectionMode.Extended;
        discoveryState=Ui.Combo(["新发现","已忽略"],0,double.NaN);discoveryState.SelectionChanged+=(_,_)=>UpdateDiscoveries();
        discoveryNote=Ui.Text("");
        var actions = new UniformGrid { Columns=2, Margin=new(8,0,8,6) };
        foreach(var button in new[]{
            Ui.Button("纳管设备",()=>_ = Run("纳管设备",async()=>{if(discoveries.SelectedItem is Device d)await EnrollDevice(d);})),
            Ui.Button("批量纳管",()=>_ = Run("批量纳管",async()=>{var selected=discoveries.SelectedItems.Cast<Device>().ToArray();if(selected.Length==0)return;if(!Confirm("批量纳管",$"按探测名称和型号默认模板添加 {selected.Length} 台设备？"))return;foreach(var d in selected){var match=await MatchModel(d);await SaveProfile(d,"managed",d.DisplayName,match?.ModelId??"",match?.TemplateId??"",0,false,d.Profile?.Monitoring,d.Profile?.PropertyIntervals);}})),
            Ui.Button("忽略",()=>_ = Run("忽略设备",()=>ChangeAdmission("ignored"))),
            Ui.Button("恢复发现",()=>_ = Run("恢复设备",()=>ChangeAdmission("pending")))}) { button.HorizontalAlignment=HorizontalAlignment.Stretch;button.Margin=new(3);actions.Children.Add(button); }
        var toolbar=new StackPanel();toolbar.Children.Add(Ui.Bar(discoveryState));toolbar.Children.Add(actions);
        return Ui.Page(toolbar,discoveries,Ui.Bar(discoveryNote));
    }
    private void UpdateDiscoveries()
    {
        if(discoveries==null)return;var state=discoveryState.SelectedIndex==1?"ignored":"pending";
        var selected=discoveries.SelectedItems.Cast<Device>().Select(d=>d.DeviceId).ToHashSet();
        var query=DeviceSearch.Text.Trim();
        var rows=snapshot.Devices.Where(d=>d.Profile?.Admission==state && (OnlineOnly.IsChecked!=true || d.Online) && $"{d.DeviceId} {d.DisplayName} {d.Registration.Model}".Contains(query,StringComparison.OrdinalIgnoreCase)).ToArray();Ui.SetRows(discoveries,rows);foreach(var row in rows.Where(d=>selected.Contains(d.DeviceId)||d.DeviceId==selectedDevice))discoveries.SelectedItems.Add(row);
        discoveryNote.Text=rows.Length==0?"暂无设备":"共 "+rows.Length+" 台";
    }
    private async Task<DeviceModel?> MatchModel(Device d)
    {
        var raw=await Connected().Api.GetAsync<JsonElement>("device-models/match?name="+Id(d.Registration.Model));
        return raw.ValueKind==JsonValueKind.Null?null:raw.Deserialize<DeviceModel>(ApiJson.Options);
    }
    private async Task ChangeAdmission(string state)
    {foreach(var d in discoveries.SelectedItems.Cast<Device>().ToArray())await SaveProfile(d,state,d.DisplayName,d.Profile?.ModelId??"",d.Profile?.BoundTemplate?.TemplateId??"",0,false,d.Profile?.Monitoring,d.Profile?.PropertyIntervals);}
    private Task<JsonElement> SaveProfile(Device d,string admission,string name,string model,string template,ulong templateVersion,bool apply,MonitoringSettings? monitoring,Dictionary<string,uint>? intervals,InterfaceSampling? sampling=null,bool replaceSampling=false)
    {
        var body=new Dictionary<string,object>{["version"]=d.Profile?.Version??0,["admission"]=admission,["name"]=name,["model_id"]=model,["template_id"]=template,["template_version"]=templateVersion,["apply_template"]=apply};
        var interfaceSampling = replaceSampling ? sampling : d.Profile?.InterfaceSampling;
        if (interfaceSampling != null) body["interface_sampling"] = interfaceSampling;
        if(monitoring!=null)body["monitoring"]=monitoring;
        if(intervals!=null)body["property_intervals"]=intervals;
        return Write(new("更新设备资料",$"devices/{Id(d.DeviceId)}/profile",body,"PUT"));
    }
    private async Task EnrollDevice(Device d)
    {
        var ownerConnection=Connected();var models=await ownerConnection.Api.ListAsync<DeviceModel>("device-models");var templates=await ownerConnection.Api.ListAsync<ProbeTemplate>("probe-templates");
        var matched=d.Profile?.ModelId is {Length:>0} modelID?models.FirstOrDefault(m=>m.ModelId==modelID):await MatchModel(d);
        matched=models.FirstOrDefault(m=>m.ModelId==matched?.ModelId);
        if(connection!=ownerConnection||closing)return;
        var window=new Window {Owner=this,Title="添加设备",Width=650,Height=570,WindowStartupLocation=WindowStartupLocation.CenterOwner};window.SetResourceReference(StyleProperty,typeof(Window));
        var body=new StackPanel {Margin=new(18)};window.Content=new ScrollViewer {Content=body,VerticalScrollBarVisibility=ScrollBarVisibility.Auto};
        body.Children.Add(Ui.Note($"设备 ID：{d.DeviceId}\n探测型号：{DeviceProperties.Text(d.Registration.Model)}"));
        var name=Ui.Input(d.DisplayName);body.Children.Add(Ui.Labeled("设备名称",name));
        var model=new ComboBox {ItemsSource=new[]{new DeviceModel("","未指定型号",0,[],"")}.Concat(models).ToArray(),SelectedItem=matched};if(matched==null)model.SelectedIndex=0;
        body.Children.Add(Ui.Labeled("型号",model));
        var choices=new[]{new ProbeTemplate("","内置采集（无模板）",0,null,[])}.Concat(templates).ToArray();
        var selection=new ComboBox {ItemsSource=choices,IsEditable=true,IsTextSearchEnabled=false,MaxDropDownHeight=250};
        var initial=d.Profile?.BoundTemplate?.TemplateId??matched?.TemplateId??"";selection.SelectedItem=choices.FirstOrDefault(t=>t.TemplateId==initial)??choices[0];
        var view=CollectionViewSource.GetDefaultView(choices);selection.ItemsSource=view;
        var filtering=false;
        selection.AddHandler(TextBoxBase.TextChangedEvent,new TextChangedEventHandler((_,e)=>{
            if(filtering||e.OriginalSource is not TextBox editor)return;var q=editor.Text;
            if(selection.SelectedItem is ProbeTemplate t&&q==t.ToString())return;
            filtering=true;try{var caret=editor.CaretIndex;selection.SelectedItem=null;view.Filter=o=>o is ProbeTemplate x&&x.ToString().Contains(q,StringComparison.OrdinalIgnoreCase);selection.Text=q;editor.Text=q;editor.CaretIndex=Math.Min(caret,q.Length);selection.IsDropDownOpen=true;}finally{filtering=false;}
        }));
        model.SelectionChanged+=(_,_)=>{if(selection.SelectedItem is ProbeTemplate current&&current.TemplateId!=initial)return;if(model.SelectedItem is DeviceModel m){view.Filter=null;selection.SelectedItem=choices.FirstOrDefault(t=>t.TemplateId==m.TemplateId)??choices[0];initial=m.TemplateId;}};
        body.Children.Add(Ui.Labeled("模板（输入名称或 ID 搜索）",selection));
        var apply=new CheckBox {Content="应用所选模板的当前发布版本",IsChecked=!d.Managed,Margin=new(0,5,0,10)};body.Children.Add(apply);
        if(d.Profile?.BoundTemplate is {} bound)body.Children.Add(Ui.Note($"当前绑定：{bound.Name} · v{bound.Version}。搜索列表展示服务端最新发布版本。"));
        body.Children.Add(Ui.Note("发布新模板不会自动改变本设备。勾选应用后，在线探针同步配置；离线探针下次连接同步。"));
        var error=Ui.Text("");error.TextWrapping=TextWrapping.Wrap;body.Children.Add(error);
        var save=Ui.Button("确认纳管",()=>{});body.Children.Add(save);
        save.Click+=async(_,_)=>{save.IsEnabled=false;try{if(selection.SelectedItem is not ProbeTemplate t)throw new InvalidOperationException("请从搜索结果中选择模板。");if(name.Text.Trim().Length==0)throw new InvalidOperationException("设备名称不能为空。");var intervals=d.Profile?.PropertyIntervals;if(intervals!=null&&(apply.IsChecked==true||t.TemplateId!=d.Profile?.BoundTemplate?.TemplateId))intervals=intervals.Where(p=>t.Properties.ContainsKey(p.Key)).ToDictionary(p=>p.Key,p=>p.Value);await SaveProfile(d,"managed",name.Text.Trim(),(model.SelectedItem as DeviceModel)?.ModelId??"",t.TemplateId,t.Version,apply.IsChecked==true,d.Profile?.Monitoring,intervals);window.Close();if(!d.Managed){refreshing=true;try{ExplorerTabs.SelectedIndex=0;}finally{refreshing=false;}selectedDevice=d.DeviceId;ApplySnapshot();Navigate("overview");}}catch(Exception e){error.Text=e.Message;}finally{save.IsEnabled=true;}};
        window.ShowDialog();
    }
    private DataGrid internalPorts=null!;
    private PortRatePanel portChart=null!;
    private Device? portDevice;
    private string portScope="";
    private MonitorTableRow[] physicalRows=[];
    private UIElement BuildSwitchTable()
    {
        switchPorts=Ui.Table("外壳网口",("端口","Entity",80),("链路","A",70),("协商速率 / 双工","B",145),("接收速率","C",110),("发送速率","D",110),("累计接收","E",100),("累计发送","F",100),("统计时长","G",150),("采集状态","H",120));
        foreach(var c in switchPorts.Columns.OfType<DataGridTextColumn>())c.ElementStyle=Ui.CellTextStyle(c.Binding is Binding b?b.Path.Path+"Tip":null);
        switchPorts.SelectionChanged+=(_,_)=>UpdatePortSelection();
        internalPorts=Ui.Table("内部与待核验端口",("名称","Entity",100),("系统接口","A",100),("交换机 / 端口","B",130),("CPU侧关联","C",100),("链路","D",70),("管理状态","E",80),("协商 / 双工","F",130),("用途","G",100));
        portChart=new();
        switchPorts.MinHeight=130;
        return Ui.Split(switchPorts,portChart,true,1);
    }
    private UIElement BuildSystemPorts()
    {
        return Ui.Split(Ui.Page(Ui.Heading("系统接口"),networkMetrics),Ui.Page(Ui.Heading("内部与待核验端口"),internalPorts),true,1.7);
    }

    private void UpdateSwitchTable(Device? d)
    {
        if(switchPorts==null)return;
        var scope=(connection?.Api.Origin.ToString()??"")+"|"+(d?.DeviceId??"");bool same=portScope==scope;portScope=scope;portDevice=d;
        var metrics=d?.EffectiveMetrics??[];
        string V(string k)=>metrics.TryGetValue(k,out var m)?TelemetryPresentation.Value(m,d?.Online==true):"—";
        string T(string k)=>metrics.TryGetValue(k,out var m)?TelemetryPresentation.Tip(m,d?.Online==true):"未提供物理口字节计数";
        var keys=metrics.Where(p=>p.Key.StartsWith("switch_")&&p.Key.EndsWith("_state")).Select(p=>p.Key[..^6]).OrderBy(k=>int.TryParse(V(k+"_order"),out var order)?order:int.MaxValue).ThenBy(k=>k,StringComparer.Ordinal).ToArray();
        string Label(string k)=>V(k+"_label");
        string Link(string k)=>V(k) switch {"up"=>"已连接","down"=>"未连接",_=>"未知"};
        string Rate(string k)=>metrics.TryGetValue(k,out var m)?PhysicalPorts.Rate(m,d?.Online==true):"—";
        var rows=keys.Where(k=>V(k+"_role")=="external").Select(k=>{
            var fields=new[]{"state","speed","rx_bytes_per_sec","tx_bytes_per_sec","rx_bytes","tx_bytes","elapsed_seconds","rx_bytes_per_sec"};
            var state=d?.Online!=true?"离线":!metrics.TryGetValue(k+"_rx_bytes_per_sec",out var rate)?"未提供计数":TelemetryPresentation.Stale(rate)?"采样过期":rate.Status switch {"ok"=>"正常","waiting"=>"等待采样","error"=>"采集失败",_=>"未提供计数"};
            return new MonitorTableRow(k,Label(k),[Link(k+"_state"),V(k+"_speed")+" Mbps / "+(V(k+"_duplex") switch {"full"=>"全双工","half"=>"半双工",_=>"未知"}),Rate(k+"_rx_bytes_per_sec"),Rate(k+"_tx_bytes_per_sec"),V(k+"_rx_bytes"),V(k+"_tx_bytes"),V(k+"_elapsed_seconds"),state],fields.Select(f=>T(k+"_"+f)).ToArray());
        }).ToArray();
        var selected=same?(switchPorts.SelectedItem as MonitorTableRow)?.Key:null;
        if(same&&physicalRows.Select(r=>r.Key).SequenceEqual(rows.Select(r=>r.Key))){for(int i=0;i<rows.Length;i++)physicalRows[i].Update(rows[i]);}
        else {physicalRows=rows;Ui.SetRows(switchPorts,rows);switchPorts.SelectedItem=rows.FirstOrDefault(r=>r.Key==selected)??rows.FirstOrDefault();}
        Ui.SetRows(internalPorts,keys.Where(k=>V(k+"_role")!="external").Select(k=>new MonitorTableRow(k,Label(k),[V(k+"_system"),V(k+"_chip")+" / "+V(k+"_port"),V(k+"_uplink"),Link(k+"_state"),V(k+"_admin"),V(k+"_speed")+" / "+V(k+"_duplex"),V(k+"_role")=="cpu"?"CPU内部口":"待核验映射"])).ToArray());
        UpdatePortSelection();
    }
    private void UpdatePortSelection()
    {
        if(portChart==null)return;var row=switchPorts.SelectedItem as MonitorTableRow;var key=row?.Key??"";
        portChart.Update(portDevice,portScope,key,row?.Entity??"");

    }
}
