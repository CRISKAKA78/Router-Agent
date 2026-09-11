using System.Text.Json;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;

public sealed class NetworkWorkspace : UserControl
{
 private readonly Func<WorkspaceConnection?> getConnection;
 private readonly Func<string?> getObserver;
 private string? observer;
 private readonly ComboBox networks=new(){MinWidth=200,DisplayMemberPath=nameof(OverlayNetwork.Name)};
 private readonly TextBlock status=Ui.Text("连接服务器后管理异地组网",true),feedback=Ui.Text("",true);
 private readonly DataGrid members=Ui.Table("组网成员",("节点","Name",145),("虚拟 IP","IP",130),("模式","Mode",85),("协议","Protocol",90),("延迟","Latency",95),("丢包率","Loss",85),("NAT 类型","Nat",160),("管理状态","Management",95),("组网状态","State",130),("配置","Revision",155),("最近更新时间","Sampled",135));
 private readonly DataGrid operations=Ui.Table("组网操作",("时间","TimeText",135),("设备","DeviceId",160),("操作","ActionText",120),("状态","StateText",155),("步骤","Step",190));
 private readonly TextBox details=Ui.Input("");
 private readonly NetworkTopologyView graph=new();
 private readonly ComboBox topologyMode=Ui.Combo(["运行观测","逻辑成员"],0,120);
 private readonly List<Button> writeButtons=[];
 private readonly Button join,start,stop,remove,reconcile,configure;
 private NetworkSettings? settings;
 private OverlayNetwork[] available=[];
 private NetworkTopology topology=NetworkTopology.Empty;
 private WorkspaceConnection? owner;
 private bool fetching,mutating,rendering;
 private bool followObserver=true;
 private int generation;
 private DateTimeOffset fetched=DateTimeOffset.MinValue;
 private OverlayNetwork? Network=>networks.SelectedItem as OverlayNetwork;
 private sealed record MemberRow(OverlayMember? Member,string Key,string Name,string IP,string Mode,string Protocol,string Latency,string Loss,string Nat,string Management,string State,string Revision,string Sampled);
 public NetworkWorkspace(Func<WorkspaceConnection?> connection,Func<string?>? selectedDevice=null)
 {
  getConnection=connection;getObserver=selectedDevice??(()=>null);observer=getObserver();
  AutomationProperties.SetName(networks,"异地网络选择");details.IsReadOnly=true;details.TextWrapping=TextWrapping.Wrap;details.MinHeight=70;
  var create=Action("新建网络",()=>Edit(null));var edit=Action("编辑网络",()=>{if(Network is {} n)Edit(n);});var delete=Action("删除空网络",()=>_=Delete());
  join=Action("添加成员",Join);start=Action("应用 / 启动",()=>_=MemberAction("start"));stop=Action("停止组网",()=>_=MemberAction("stop"));remove=Action("移除成员",()=>_=Remove());configure=Action("成员配置",Configure);reconcile=Action("核实所选操作",()=>_=Reconcile());
  var refresh=Ui.Button("刷新",()=>_=Refresh(true));var password=Ui.Button("查看网络密码",()=>_=Password());
  networks.SelectionChanged+=(_,_)=>{if(!rendering){followObserver=false;generation++;fetched=DateTimeOffset.MinValue;ClearView();_=Refresh(true);}};
  members.SelectionChanged+=(_,_)=>{if(!rendering)ShowMember();Enabled();};operations.SelectionChanged+=(_,_)=>{if(!rendering&&operations.SelectedItem is NetworkOperation op)details.Text=$"{op.TimeText} · {op.DeviceId} · {op.ActionText}\n{op.StateText} · {op.Step}\n{NetworkLabels.Error(op.Error)}\n操作：{op.OperationId}\n关联任务：{string.Join("、",op.TaskIds)}";Enabled();};
  topologyMode.SelectionChanged+=(_,_)=>RenderGraph();graph.NodeSelected+=node=>members.SelectedItem=members.Items.Cast<MemberRow>().FirstOrDefault(r=>r.Key==node.Id);
  var top=Ui.Bar(networks,create,edit,password,delete,refresh);var actions=Ui.Bar(join,configure,start,stop,remove);
  status.TextWrapping=feedback.TextWrapping=TextWrapping.Wrap;status.Margin=feedback.Margin=new(12,5,12,5);
  var motion=new CheckBox{Content="流量动画",IsChecked=true,VerticalAlignment=VerticalAlignment.Center};motion.Checked+=(_,_)=>graph.AnimationsEnabled=true;motion.Unchecked+=(_,_)=>graph.AnimationsEnabled=false;
  var topologyPage=Ui.Page(Ui.Bar(topologyMode,Ui.Button("放大",()=>graph.Zoom(1.15)),Ui.Button("缩小",()=>graph.Zoom(1/1.15)),Ui.Button("适应窗口",graph.Fit),motion),graph);
  var tabs=new TabControl{Style=(Style)FindResource("WorkbenchTabs")};tabs.Items.Add(new TabItem{Header="成员",Content=members});tabs.Items.Add(new TabItem{Header="拓扑",Content=topologyPage});tabs.Items.Add(new TabItem{Header="操作记录",Content=Ui.Page(Ui.Bar(reconcile),operations)});
  var diagnostic=new Expander{Header="详情",Content=details,Margin=new(12,0,12,8)};
  Content=Ui.Page(new StackPanel{Children={top,status,actions,feedback}},tabs,diagnostic);
  Loaded+=(_,_)=>_=Refresh(true);Unloaded+=(_,_)=>generation++;Enabled();
 }
 private Button Action(string name,Action action){var b=Ui.Button(name,action);writeButtons.Add(b);return b;}
 private bool Confirm(string title,string text)=>MessageBox.Show(Window.GetWindow(this),text,title,MessageBoxButton.OKCancel,MessageBoxImage.Question,MessageBoxResult.Cancel)==MessageBoxResult.OK;
 private void Enabled(){var c=getConnection();var writable=c is {Synchronized:true,Busy:false,Pending:null}&&!mutating;foreach(var b in writeButtons)b.IsEnabled=writable;join.IsEnabled=writable&&Network!=null&&settings?.Configured==true;var row=members.SelectedItem as MemberRow;start.IsEnabled=stop.IsEnabled=writable&&row?.Member!=null&&settings?.Configured==true;configure.IsEnabled=writable&&row?.Member!=null;remove.IsEnabled=writable&&row?.Member?.Desired=="stop";reconcile.IsEnabled=writable&&operations.SelectedItem is NetworkOperation{State:"uncertain"} operation&&string.IsNullOrEmpty(operation.SupersededBy);}
 private void ClearView(){topology=NetworkTopology.Empty;members.ItemsSource=null;operations.ItemsSource=null;details.Text="";graph.Render(topology,null,new Dictionary<string,string>(),false);}
 public void Update(){var selected=getObserver();if(getConnection()!=owner||selected!=observer){owner=getConnection();observer=selected;followObserver=true;generation++;fetched=DateTimeOffset.MinValue;ClearView();if(owner==null){available=[];networks.ItemsSource=null;settings=null;}feedback.Text="";}Enabled();if(IsVisible)_=Refresh();}
 private async Task Refresh(bool force=false)
 {
  var c=getConnection();if(c is null||!c.Synchronized){status.Text=c?.Status??"连接服务器后管理异地组网";return;}if(fetching||(!force&&DateTimeOffset.UtcNow-fetched<TimeSpan.FromSeconds(4)))return;
  fetching=true;var epoch=generation;var selected=Network?.NetworkId;var observerId=observer;var follow=followObserver;owner=c;
  try {
   var result=await c.TrackAsync(async()=>{var conf=await c.Api.GetAsync<NetworkSettings>("network-settings");var list=await c.Api.ListAsync<OverlayNetwork>("networks");var follows=list.FirstOrDefault(n=>n.Members.Any(m=>m.DeviceId==observerId));var id=(follow?follows?.NetworkId:null)??list.FirstOrDefault(n=>n.NetworkId==selected)?.NetworkId??follows?.NetworkId??list.FirstOrDefault()?.NetworkId;var data=id==null?NetworkTopology.Empty:await c.Api.GetAsync<NetworkTopology>($"networks/{ApiClient.Segment(id)}/topology");var ops=id==null?[]:await c.Api.ListAsync<NetworkOperation>($"networks/{ApiClient.Segment(id)}/operations");return(conf,list,id,data,ops);});
   if(c!=getConnection()||epoch!=generation||c.Token.IsCancellationRequested)return;
   var key=(members.SelectedItem as MemberRow)?.Key;var operation=(operations.SelectedItem as NetworkOperation)?.OperationId;followObserver=false;settings=result.conf;available=result.list;topology=result.data;rendering=true;
   try{networks.ItemsSource=available;networks.SelectedItem=available.FirstOrDefault(n=>n.NetworkId==result.id);RenderMembers(key);Ui.SetRows(operations,result.ops);operations.SelectedItem=result.ops.FirstOrDefault(o=>o.OperationId==operation);}finally{rendering=false;}
   status.Text=!settings.Configured?"EasyTier 配置服务尚未配置。":Network?.Members.Any(m=>m.DeviceId==observer)==true?$"观察设备：{c.Snapshot.Devices.FirstOrDefault(d=>d.DeviceId==observer)?.DisplayName??observer} · {Network.Name}":"左侧设备未加入此网络，暂无本机链路视角；可新建网络或添加成员。";
   if(topology.Limited)status.Text+=" 部分观测已截断。";if(!settings.PackageConfigured)status.Text+=" 缺失引擎时需先配置仓库工具。";RenderGraph();fetched=DateTimeOffset.UtcNow;
  }catch(Exception e){if(c==getConnection()&&epoch==generation&&!c.Token.IsCancellationRequested)status.Text="组网数据获取失败，旧数据可能已过期："+e.Message;}
  finally{fetching=false;Enabled();if(c==getConnection()&&epoch!=generation&&IsVisible)_=Refresh(true);}
 }
 private NetworkObservation? Perspective=>topology.Observations.FirstOrDefault(o=>o.DeviceId==observer&&!o.Stale&&DateTimeOffset.UtcNow-o.SampledAt<TimeSpan.FromSeconds(35));
 internal static string NatLabel(int? value)=>value switch{1=>"公网（无 NAT）",2=>"无端口转换",3=>"完全锥形（NAT1）",4=>"受限锥形（NAT2）",5=>"端口受限（NAT3）",6=>"对称型（NAT4）",7=>"对称 UDP 防火墙",8=>"对称递增（NAT4）",9=>"对称递减（NAT4）",_=>"未提供"};
 private void RenderMembers(string? selected)
 {
  var devices=getConnection()?.Snapshot.Devices??[];var source=Perspective;var n=Network;
  var rows=topology.Nodes.Select(node=>{
   var m=n?.Members.FirstOrDefault(m=>m.DeviceId==node.DeviceId);var d=devices.FirstOrDefault(d=>d.DeviceId==node.DeviceId);var own=topology.Observations.FirstOrDefault(o=>o.DeviceId==node.DeviceId);bool local=m!=null&&m.DeviceId==observer;
   var route=source?.Routes.FirstOrDefault(r=>m!=null&&r.InstanceId==m.InstanceId)??source?.Routes.FirstOrDefault(r=>r.PeerId==node.PeerId);var peer=route?.PeerId??node.PeerId;var connections=source?.Links.Where(l=>l.PeerId==peer).ToArray()??[];
   bool server=node.External&&connections.Any(l=>Uri.TryCreate(l.RemoteUrl,UriKind.Absolute,out var u)&&u.Host=="47.119.168.150");
   string name=local?"本机":m!=null?d?.DisplayName??m.DeviceId:server?"服务器":route?.Hostname is {Length:>0} host?host:node.Hostname.Length>0?node.Hostname:$"外部节点 {peer}";
   string ip=local?own?.VirtualIp??"":route?.VirtualIp??own?.VirtualIp??node.VirtualIp;if(string.IsNullOrEmpty(ip))ip=m?.VirtualIp is {Length:>0} fixedIp?fixedIp:"待分配";
   string mode=local?"—":source==null?"未知":connections.Length>0?"直连":route!=null&&route.NextHopPeerId!=0&&route.NextHopPeerId!=peer?"中继":"未观测";
   var latency=connections.Where(l=>l.LatencyMs!=null).Select(l=>$"{l.Transport}: {l.LatencyMs:F1} ms");var loss=connections.Where(l=>l.LossRate!=null).Select(l=>$"{l.Transport}: {l.LossRate:P1}");
   var nat=local?own?.Nat:route?.Nat;var sample=local?own?.SampledAt:source?.SampledAt;
   return new MemberRow(m,node.Id,name,ip,mode,local?"—":connections.Length==0?"未提供":string.Join(" / ",connections.Select(l=>l.Transport).Distinct()),local?"—":latency.Any()?string.Join(" / ",latency):"未提供",local?"—":loss.Any()?string.Join(" / ",loss):"未提供",NatLabel(nat?.Udp),m==null?"非托管":d?.Online==true?"在线":"离线",own is null?(source==null?"未观测":"已发现"):own.Stale?"数据已过期":NetworkLabels.State(own.State),m==null?"—":$"网络 {m.AppliedRevision}/{n!.Revision} · 成员 {m.AppliedConfigRevision}/{m.ConfigRevision}",sample is {} at&&at.Year>2000?at.ToLocalTime().ToString("MM-dd HH:mm:ss"):"未提供");
  }).ToArray();Ui.SetRows(members,rows);members.SelectedItem=rows.FirstOrDefault(r=>r.Key==selected);
 }
 private void RenderGraph(){var names=(getConnection()?.Snapshot.Devices??[]).ToDictionary(d=>d.DeviceId,d=>d.DisplayName);graph.Render(topology,Network,names,topologyMode.SelectedIndex==1,observer);}
 private void ShowMember(){if(members.SelectedItem is not MemberRow row)return;details.Text=$"{row.Name} · {row.IP}\n{row.Mode} · {row.Protocol} · {row.Latency} · 丢包 {row.Loss}\n{row.Management} · {row.State}\n{(row.Member is {} m?$"设备：{m.DeviceId}\nMachine ID：{m.MachineId}\n实例 ID：{m.InstanceId}":"外部节点不受平台管理。")}";}
 private async Task<bool> Mutate(Mutation mutation)
 {
  var c=getConnection();if(c is not {Synchronized:true,Busy:false,Pending:null}||mutating){mutation.Dispose();return false;}mutating=true;Enabled();
  try{var result=await c.ExecuteAsync(mutation);if(c!=getConnection())return false;
   feedback.Text=result.ValueKind==JsonValueKind.Array?$"已受理 {result.EnumerateArray().Count(x=>x.TryGetProperty("operation",out _))} 个成员；未受理 {result.EnumerateArray().Count(x=>x.TryGetProperty("error",out _))} 个。各成员进度见操作记录。未受理：{string.Join("、",result.EnumerateArray().Where(x=>x.TryGetProperty("error",out _)).Select(x=>x.GetProperty("device_id").GetString()))}":result.TryGetProperty("operation_id",out _)?"已受理，系统将自动核实完成状态。":"配置已保存；运行中的成员将按需重建组网实例。";
   fetched=DateTimeOffset.MinValue;await Refresh(true);return true;
  }catch(Exception e){if(c==getConnection()&&!c.Token.IsCancellationRequested)feedback.Text=e.Message+(c.Pending!=null?"；原请求已保留，请先核实请求结果，勿重复提交。":"");return false;}finally{mutating=false;Enabled();}
 }
 private void Edit(OverlayNetwork? current){var c=getConnection();if(c==null)return;NetworkDialog.Network(Window.GetWindow(this),current,body=>c==getConnection()?Mutate(new("保存组网",current==null?"networks":$"networks/{ApiClient.Segment(current.NetworkId)}",body,current==null?"POST":"PUT")):Task.FromResult(false));}
 private void Join(){var c=getConnection();var n=Network;if(c==null||n==null)return;var devices=c.Snapshot.Devices.Where(d=>d.Online&&d.Managed&&d.Registration.Capabilities.Contains("network_agent_v1")&&!available.Any(n=>n.Members.Any(m=>m.DeviceId==d.DeviceId))).Select(d=>(d.DeviceId,d.DisplayName)).ToArray();if(devices.Length==0){feedback.Text="没有可添加的在线设备。设备须具备组网能力且未加入其他网络。";return;}NetworkDialog.Members(Window.GetWindow(this),n,devices,body=>c==getConnection()?Mutate(new("添加组网成员",$"networks/{ApiClient.Segment(n.NetworkId)}/members/batch",body)):Task.FromResult(false));}
 private void Configure(){var c=getConnection();if(c==null||Network is not {} n||members.SelectedItem is not MemberRow{Member:{} m} row)return;NetworkDialog.Member(Window.GetWindow(this),m,row.Name,cfg=>c==getConnection()?Mutate(new("保存成员配置",$"networks/{ApiClient.Segment(n.NetworkId)}/members/{ApiClient.Segment(m.DeviceId)}/config",new{config=cfg,revision=m.ConfigRevision},"PUT")):Task.FromResult(false),n.Routes);}
 private async Task Password(){var c=getConnection();var n=Network;if(c==null||n==null)return;try{var result=await c.TrackAsync(()=>c.Api.GetAsync<NetworkPassword>($"networks/{ApiClient.Segment(n.NetworkId)}/password"));if(c!=getConnection()||Network?.NetworkId!=n.NetworkId)return;var box=new PasswordBox{Password=result.Password,MinHeight=34};var text=Ui.Input(result.Password);text.IsReadOnly=true;text.Visibility=Visibility.Collapsed;var reveal=new CheckBox{Content="显示密码"};reveal.Checked+=(_,_)=>{box.Visibility=Visibility.Collapsed;text.Visibility=Visibility.Visible;};reveal.Unchecked+=(_,_)=>{box.Visibility=Visibility.Visible;text.Visibility=Visibility.Collapsed;};var window=new Window{Owner=Window.GetWindow(this),Title="网络密码 · "+n.Name,Width=460,SizeToContent=SizeToContent.Height,WindowStartupLocation=WindowStartupLocation.CenterOwner};window.SetResourceReference(StyleProperty,typeof(Window));window.Content=new StackPanel{Margin=new(20),Children={box,text,Ui.Bar(reveal,Ui.Button("复制密码",()=>Clipboard.SetText(result.Password)))}};window.ShowDialog();box.Clear();text.Clear();}catch(Exception e){feedback.Text=e.Message;}}
 private Task MemberAction(string action){if(Network is not {} n||members.SelectedItem is not MemberRow{Member:{} m})return Task.CompletedTask;if(!Confirm(action=="start"?"应用组网配置":"停止组网",action=="start"?"应用此成员的当前配置，会短暂重建组网实例。继续？":"仅停止所选成员的组网实例。继续？"))return Task.CompletedTask;return Mutate(new("组网操作",$"networks/{ApiClient.Segment(n.NetworkId)}/members/{ApiClient.Segment(m.DeviceId)}/operations",new{action}));}
 private Task Reconcile()=>operations.SelectedItem is NetworkOperation op?Mutate(new("核实组网操作",$"network-operations/{ApiClient.Segment(op.OperationId)}/reconcile")):Task.CompletedTask;
 private Task Remove(){if(Network is not {} n||members.SelectedItem is not MemberRow{Member:{} m})return Task.CompletedTask;if(!Confirm("移除成员","确认停止后移除成员记录。此操作不能撤销设备已获知的网络密码。继续？"))return Task.CompletedTask;return Mutate(new("移除组网成员",$"networks/{ApiClient.Segment(n.NetworkId)}/members/{ApiClient.Segment(m.DeviceId)}",method:"DELETE"));}
 private Task Delete(){if(Network is not {} n||!Confirm("删除空网络","删除没有成员的网络记录？"))return Task.CompletedTask;return Mutate(new("删除网络",$"networks/{ApiClient.Segment(n.NetworkId)}",method:"DELETE"));}
}
