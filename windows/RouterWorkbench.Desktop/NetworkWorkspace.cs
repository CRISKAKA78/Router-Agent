using System.Net;
using System.Text.Json;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;

public sealed class NetworkWorkspace : UserControl
{
 private readonly Func<WorkspaceConnection?> getConnection;
 private readonly ComboBox networks=new(){MinWidth=200,DisplayMemberPath=nameof(OverlayNetwork.Name)};
 private readonly TextBlock status=Ui.Text("连接服务器后管理异地组网",true),feedback=Ui.Text("",true);
 private readonly DataGrid members=Ui.Table("组网成员",("设备","Name",150),("虚拟IP","IP",130),("管理连接","Management",100),("引擎观测","State",170),("配置","Revision",125),("最近采样","Sampled",120));
 private readonly DataGrid operations=Ui.Table("组网操作",("时间","TimeText",135),("设备","DeviceId",160),("操作","ActionText",120),("状态","StateText",200),("步骤","Step",200));
 private readonly DataGrid links=Ui.Table("观测链路",("来源设备","ReportedBy",140),("来源","Source",170),("目标","Target",170),("协议","Transport",100),("双边确认","ConfirmedBoth",90),("过期","Stale",80),("链路RTT ms","Link.LatencyMs",110));
 private readonly TextBox details=Ui.Input("");
 private readonly NetworkTopologyView graph=new();
 private readonly ComboBox topologyMode=Ui.Combo(["运行观测","逻辑成员"],0,120);
 private readonly List<Button> writeButtons=[];
 private readonly Button join,start,stop,remove,reconcile;
 private NetworkSettings? settings;
 private OverlayNetwork[] available=[];
 private NetworkTopology topology=NetworkTopology.Empty;
 private WorkspaceConnection? owner;
 private bool fetching,mutating,rendering;
 private int generation;
 private DateTimeOffset fetched=DateTimeOffset.MinValue;
 private OverlayNetwork? Network=>networks.SelectedItem as OverlayNetwork;
 private sealed record MemberRow(OverlayMember Member,string Name,string IP,string Management,string State,string Revision,string Sampled);
 public NetworkWorkspace(Func<WorkspaceConnection?> connection){
  getConnection=connection;AutomationProperties.SetName(networks,"异地网络选择");details.IsReadOnly=true;details.TextWrapping=TextWrapping.Wrap;details.MinHeight=90;
  var create=Action("新建网络",()=>Edit(null));var edit=Action("编辑网络",()=>{if(Network is {} n)Edit(n);});var delete=Action("删除空网络",()=>_=Delete());
  join=Action("添加设备",Join);start=Action("应用 / 启动",()=>_=MemberAction("start"));stop=Action("停止组网",()=>_=MemberAction("stop"));remove=Action("移除已停止成员",()=>_=Remove());reconcile=Action("回查不确定操作",()=>_=Reconcile());
  var refresh=Ui.Button("刷新",()=>_=Refresh(true));networks.SelectionChanged+=(_,_)=>{if(!rendering){generation++;fetched=DateTimeOffset.MinValue;topology=NetworkTopology.Empty;members.ItemsSource=null;operations.ItemsSource=null;_ = Refresh(true);}};
  members.SelectionChanged+=(_,_)=>{if(!rendering)ShowMember();Enabled();};operations.SelectionChanged+=(_,_)=>{if(operations.SelectedItem is NetworkOperation op)details.Text=ApiJson.Pretty(op)+"\n"+NetworkLabels.Error(op.Error);Enabled();};topologyMode.SelectionChanged+=(_,_)=>RenderGraph();graph.NodeSelected+=node=>{if(node.DeviceId is {} id)members.SelectedItem=members.Items.Cast<MemberRow>().FirstOrDefault(r=>r.Member.DeviceId==id);};
  var top=Ui.Bar(networks,create,edit,delete,refresh);var actions=Ui.Bar(join,start,stop,remove,reconcile);
  status.TextWrapping=feedback.TextWrapping=TextWrapping.Wrap;status.Margin=feedback.Margin=new(12,6,12,6);
  var topologyPage=Ui.Page(Ui.Bar(topologyMode,Ui.Button("放大",()=>graph.Zoom(1.15)),Ui.Button("缩小",()=>graph.Zoom(1/1.15)),Ui.Button("复位",graph.Reset)),graph);
  var tabs=new TabControl{Style=(Style)FindResource("WorkbenchTabs")};tabs.Items.Add(new TabItem{Header="成员",Content=members});tabs.Items.Add(new TabItem{Header="拓扑",Content=topologyPage});tabs.Items.Add(new TabItem{Header="链路",Content=links});tabs.Items.Add(new TabItem{Header="操作记录",Content=operations});
  Content=Ui.Page(new StackPanel{Children={top,status,actions,feedback}},Ui.Split(tabs,details,true,3));Loaded+=(_,_)=>_=Refresh(true);Unloaded+=(_,_)=>{generation++;};Enabled();
 }
 private Button Action(string name,Action action){var b=Ui.Button(name,action);writeButtons.Add(b);return b;}
 private bool Confirm(string title,string text)=>MessageBox.Show(Window.GetWindow(this),text,title,MessageBoxButton.OKCancel,MessageBoxImage.Question,MessageBoxResult.Cancel)==MessageBoxResult.OK;
 private void Enabled(){var c=getConnection();var writable=c is {Synchronized:true,Busy:false,Pending:null}&&!mutating;foreach(var b in writeButtons)b.IsEnabled=writable;join.IsEnabled=writable&&Network!=null&&settings?.Configured==true;var row=members.SelectedItem as MemberRow;start.IsEnabled=stop.IsEnabled=writable&&row!=null&&settings?.Configured==true;remove.IsEnabled=writable&&row?.Member.Desired=="stop";reconcile.IsEnabled=writable&&operations.SelectedItem is NetworkOperation{State:"uncertain"};}
 public void Update(){if(getConnection()!=owner){owner=getConnection();generation++;fetched=DateTimeOffset.MinValue;available=[];networks.ItemsSource=null;members.ItemsSource=null;links.ItemsSource=null;operations.ItemsSource=null;details.Text="";feedback.Text="";settings=null;topology=NetworkTopology.Empty;graph.Render(topology,null,new Dictionary<string,string>(),false);}Enabled();if(IsVisible)_=Refresh();}
 private async Task Refresh(bool force=false){
  var c=getConnection();if(c is null||!c.Synchronized){status.Text=c?.Status??"连接服务器后管理异地组网";return;}if(fetching||(!force&&DateTimeOffset.UtcNow-fetched<TimeSpan.FromSeconds(4)))return;fetching=true;var epoch=generation;var selected=Network?.NetworkId;owner=c;
  try{
   var result=await c.TrackAsync(async()=>{var conf=await c.Api.GetAsync<NetworkSettings>("network-settings");var list=await c.Api.ListAsync<OverlayNetwork>("networks");var id=list.FirstOrDefault(n=>n.NetworkId==selected)?.NetworkId??list.FirstOrDefault()?.NetworkId;var graph=id==null?NetworkTopology.Empty:await c.Api.GetAsync<NetworkTopology>($"networks/{ApiClient.Segment(id)}/topology");var ops=id==null?[]:await c.Api.ListAsync<NetworkOperation>($"networks/{ApiClient.Segment(id)}/operations");return(conf,list,id,graph,ops);});
   if(c!=getConnection()||epoch!=generation||c.Token.IsCancellationRequested)return;
   var member=(members.SelectedItem as MemberRow)?.Member.DeviceId;var operation=(operations.SelectedItem as NetworkOperation)?.OperationId;settings=result.conf;available=result.list;topology=result.graph;rendering=true;
   try{networks.ItemsSource=available;networks.SelectedItem=available.FirstOrDefault(n=>n.NetworkId==result.id);RenderMembers(member);Ui.SetRows(links,topology.Edges);Ui.SetRows(operations,result.ops);operations.SelectedItem=result.ops.FirstOrDefault(o=>o.OperationId==operation);}finally{rendering=false;}
   var current=Network;status.Text=!settings.Configured?"尚未配置同主机 EasyTier Web API。请在 Server 的本地配置文件中设置专用账号；界面不接收密码。":$"EasyTier {settings.Version} · TCP / UDP监听（无WG） · 自定义路由已开启 · {(current?.Routes.Length??0)} 条路由 · 管理断线保持已有组网";
   if(!settings.PackageConfigured)status.Text+="；缺失引擎时需要先配置仓库工具。";RenderGraph();fetched=DateTimeOffset.UtcNow;
  }catch(ApiException e) when(e.Code=="not_found"||e.Message.Contains("404")){if(c==getConnection())status.Text="当前服务器尚未提供异地组网 API，请更新配套 Server。";fetched=DateTimeOffset.UtcNow;}
  catch(Exception e){if(c==getConnection()&&!c.Token.IsCancellationRequested)status.Text="组网快照获取失败，显示的信息可能已过期："+e.Message;}
  finally{fetching=false;Enabled();if(c==getConnection()&&epoch!=generation&&IsVisible)_=Refresh(true);}
 }
 private void RenderMembers(string? selected){var devices=getConnection()?.Snapshot.Devices??[];var rows=(Network?.Members??[]).Select(m=>{var d=devices.FirstOrDefault(d=>d.DeviceId==m.DeviceId);var o=topology.Observations.FirstOrDefault(o=>o.DeviceId==m.DeviceId);return new MemberRow(m,d?.DisplayName??m.DeviceId,string.IsNullOrEmpty(o?.VirtualIp)?(m.VirtualIp==""?"DHCP / 待分配":m.VirtualIp):o.VirtualIp,d?.Online==true?"管理在线":"管理离线",o is null||o.Stale?"未知 / 已过期":NetworkLabels.State(o.State),$"{m.AppliedRevision} / {Network?.Revision}",o is null||o.SampledAt.Year<2000?"未提供":o.SampledAt.ToLocalTime().ToString("HH:mm:ss"));}).ToArray();Ui.SetRows(members,rows);members.SelectedItem=rows.FirstOrDefault(r=>r.Member.DeviceId==selected);}
 private void RenderGraph(){var names=(getConnection()?.Snapshot.Devices??[]).ToDictionary(d=>d.DeviceId,d=>d.DisplayName);graph.Render(topology,Network,names,topologyMode.SelectedIndex==1);}
 private void ShowMember(){if(members.SelectedItem is not MemberRow row)return;var o=topology.Observations.FirstOrDefault(o=>o.DeviceId==row.Member.DeviceId);details.Text=$"{row.Name}\n虚拟IP可达性尚需端到端验证；引擎运行不等于业务可达。\n"+ApiJson.Pretty(new{member=row.Member,observation=o});}
 private async Task Mutate(Mutation m){var c=getConnection();if(c is not {Synchronized:true,Busy:false,Pending:null}||mutating){m.Dispose();return;}mutating=true;Enabled();try{var result=await c.ExecuteAsync(m);if(c!=getConnection())return;feedback.Text=result.TryGetProperty("operation_id",out var id)?$"操作 {id.GetString()} 已受理，请查看操作记录与关联任务；尚未确认完成。":"操作已保存。配置编辑不会自动重启成员，请按需应用。";fetched=DateTimeOffset.MinValue;await Refresh(true);}catch(Exception e){if(c==getConnection()&&!c.Token.IsCancellationRequested)feedback.Text=e.Message+ (c.Pending!=null?"；响应不确定，原请求已保留，请勿创建替代请求。":"");}finally{mutating=false;Enabled();}}
 private void Edit(OverlayNetwork? current){var c=getConnection();if(c==null)return;new FormWindow(Window.GetWindow(this),current==null?"新建异地网络":"编辑异地网络","默认使用官方 DHCP；静态地址仅在添加设备时显式指定。路由默认留空，不自动发布LAN。",[new("name","网络名称",current?.Name??""),new("cidr","静态地址网段",current?.Cidr??"10.144.144.0/24"),new("peers","引导 / 中继地址，每行一项",string.Join("\n",current?.PeerUrls??[]),true),new("routes","自定义路由，默认空",string.Join("\n",current?.Routes??[]),true)],async values=>{if(c!=getConnection())return;string[] Lines(string key)=>values[key].Split(['\r','\n'],StringSplitOptions.RemoveEmptyEntries|StringSplitOptions.TrimEntries);object body=current==null?new{name=values["name"],cidr=values["cidr"],peer_urls=Lines("peers"),routes=Lines("routes")}:new{name=values["name"],cidr=values["cidr"],peer_urls=Lines("peers"),routes=Lines("routes"),revision=current.Revision};await Mutate(new("保存组网",current==null?"networks":$"networks/{ApiClient.Segment(current.NetworkId)}",body,current==null?"POST":"PUT"));}).ShowDialog();}
 private void Join(){var c=getConnection();var n=Network;if(c==null||n==null)return;var devices=c.Snapshot.Devices.Where(d=>d.Online&&d.Managed&&d.Registration.Capabilities.Contains("network_agent_v1")&&!available.Any(n=>n.Members.Any(m=>m.DeviceId==d.DeviceId))).ToArray();if(devices.Length==0){feedback.Text="没有可添加的在线设备：需要配套 Probe 的 network_agent_v1 能力，且尚未加入其他网络。";return;}var choices=devices.Select(d=>$"{d.DisplayName} / {d.Registration.Arch} / {d.DeviceId}").ToArray();new FormWindow(Window.GetWindow(this),"添加组网设备","缺失 EasyTier 时会从已配置的仓库工具选择兼容产物安装，并接入配置服务。不会自动桥接LAN或更换默认路由。",[new("device","在线设备",choices[0],false,choices),new("ip","静态虚拟IP，可空（官方DHCP）","")],async v=>{if(c!=getConnection())return;var index=Array.IndexOf(choices,v["device"]);if(index<0)return;await Mutate(new("设备加入组网",$"networks/{ApiClient.Segment(n.NetworkId)}/members",new{device_id=devices[index].DeviceId,virtual_ip=v["ip"].Trim()}));}).ShowDialog();}
 private Task MemberAction(string action){if(Network is not {} n||members.SelectedItem is not MemberRow row)return Task.CompletedTask;if(!Confirm(action=="start"?"应用组网配置":"停止组网",action=="start"?"下发当前网络配置到所选设备，可能短暂重建组网接口。继续？":"停止所选成员的组网实例，不关闭现有远程维护连接。继续？"))return Task.CompletedTask;return Mutate(new("组网操作",$"networks/{ApiClient.Segment(n.NetworkId)}/members/{ApiClient.Segment(row.Member.DeviceId)}/operations",new{action}));}
 private Task Reconcile(){if(operations.SelectedItem is not NetworkOperation op)return Task.CompletedTask;return Mutate(new("回查原组网操作",$"network-operations/{ApiClient.Segment(op.OperationId)}/reconcile"));}
 private Task Remove(){if(Network is not {} n||members.SelectedItem is not MemberRow row)return Task.CompletedTask;if(!Confirm("移除平台成员记录","仅允许停止已确认的成员。移除记录不等于撤销设备已获知的共享网络密钥。继续？"))return Task.CompletedTask;return Mutate(new("移除组网成员",$"networks/{ApiClient.Segment(n.NetworkId)}/members/{ApiClient.Segment(row.Member.DeviceId)}",method:"DELETE"));}
 private Task Delete(){if(Network is not {} n||!Confirm("删除空网络","仅删除没有成员的网络记录。继续？"))return Task.CompletedTask;return Mutate(new("删除网络",$"networks/{ApiClient.Segment(n.NetworkId)}",method:"DELETE"));}
}
