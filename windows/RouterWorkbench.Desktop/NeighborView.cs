using System.Windows;
using System.Windows.Controls;
using System.Windows.Automation;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

internal sealed class NeighborView : UserControl
{
 private readonly string scope;
 private readonly Func<WorkspaceConnection?> connection;
 private readonly Func<Device?> device;
 private readonly ComboBox domains=new();
 private readonly TextBox cidr=Ui.Input(""),search=Ui.Input("");
 private readonly TextBlock status=Ui.Text("请选择设备。",true),operation=Ui.Text("",true);
 internal DataGrid Entries {get;}=Ui.Table("邻居设备",("IP地址","IpText",165),("MAC地址","Mac",170),("本机转发端口","PortText",130),("主机名","NameText",150),("记录状态","StateText",140),("来源","SourceText",180));
 private readonly DataGrid unmatched=Ui.Table("未匹配LAN端口记录",("接口","Interface",100),("IP地址","IpText",165),("MAC地址","Mac",170),("转发端口","PortText",120),("来源","SourceText",160));
 private readonly Expander unknown;
 private readonly Button scan,cancel,refresh;
 private string? deviceId,sessionId,scanTaskId,lastResultTask;
 private WorkspaceConnection? owner;
 private bool busy;
 private NeighborDomainConfig[] available=[];
 public NeighborView(string scope,Func<WorkspaceConnection?> connection,Func<Device?> device)
 {
  this.scope=scope;this.connection=connection;this.device=device;
  domains.MinWidth=200;domains.DisplayMemberPath=nameof(NeighborDomainConfig.Label);domains.SelectionChanged+=(_,_)=>Render();
  AutomationProperties.SetName(domains,"邻居广播域");AutomationProperties.SetName(cidr,"主动发现IPv4范围");AutomationProperties.SetName(search,"搜索IP MAC或主机名");
  cidr.MinWidth=150;cidr.ToolTip="填写该接口直连的IPv4 CIDR，例如192.168.5.0/24；每次最多256个地址。";
  search.MinWidth=140;search.ToolTip="按IP、MAC、主机名或端口筛选";search.TextChanged+=(_,_)=>Render();
  scan=Ui.Button("主动发现",()=>_=StartScan());cancel=Ui.Button("停止发现",()=>_=CancelScan());refresh=Ui.Button("刷新记录",()=>{connection()?.Invalidate();Render();});
  var controls=new WrapPanel();foreach(var child in new UIElement[]{Ui.Text("广播域"),domains,Ui.Text("IPv4范围"),cidr,scan,cancel,refresh,Ui.Text("搜索"),search}){if(child is FrameworkElement f)f.Margin=new(0,3,8,3);controls.Children.Add(child);}
  status.TextWrapping=operation.TextWrapping=TextWrapping.Wrap;status.Margin=new(0,8,0,4);operation.Margin=new(0,0,0,8);
  unknown=new Expander{Header="未匹配端口",Content=unmatched,Visibility=Visibility.Collapsed};unmatched.MaxHeight=200;
  var bottom=new StackPanel{Children={unknown}};
  var body=new DockPanel();DockPanel.SetDock(bottom,Dock.Bottom);body.Children.Add(bottom);body.Children.Add(Entries);
  Content=new Border{Padding=new(16,10,16,16),Child=Ui.Page(new StackPanel{Children={controls,status,operation}},body)};
  Loaded+=(_,_)=>Update();
 }
 public void Update()
 {
  var current=device();var next=connection();
  if(current?.DeviceId!=deviceId||current?.CurrentSession?.SessionId!=sessionId||next!=owner){deviceId=current?.DeviceId;sessionId=current?.CurrentSession?.SessionId;owner=next;scanTaskId=null;operation.Text="";Entries.ItemsSource=null;unmatched.ItemsSource=null;cidr.Text="";search.Text="";}
  var definitions=(current?.NeighborDomains??[]).Where(d=>d.Scope==scope).ToArray();
  if(!available.Select(d=>(d.Id,d.Interface)).SequenceEqual(definitions.Select(d=>(d.Id,d.Interface)))){var selected=(domains.SelectedItem as NeighborDomainConfig)?.Id;available=definitions;domains.ItemsSource=available;domains.SelectedItem=available.FirstOrDefault(d=>d.Id==selected)??available.FirstOrDefault();}
  // Recover a known scan after view/device selection changes using server tasks.
  if(scanTaskId is null&&current is not null){scanTaskId=next?.Snapshot.Tasks.FirstOrDefault(t=>t.DeviceId==current.DeviceId&&t.Type=="neighbor_scan"&&t.State is "received" or "queued" or "running")?.TaskId;}
  if(scanTaskId is {} id&&next?.Snapshot.Tasks.FirstOrDefault(t=>t.TaskId==id) is {} task&&task.State is "success" or "failed" or "timeout" or "rejected"){
   operation.Text="主动发现："+task.StateText+"。记录将随采样刷新。";scanTaskId=null;_ = ShowResult(next!,task.TaskId);
  }
  Render();
 }
 private async Task ShowResult(WorkspaceConnection c,string taskId)
 {
  var requestedDevice=deviceId;var requestedSession=sessionId;lastResultTask=taskId;
  try{
   var detail=await c.TrackAsync(()=>c.Api.GetAsync<TaskDetail>($"tasks/{ApiClient.Segment(taskId)}"));
   if(c!=owner||requestedDevice!=deviceId||requestedSession!=sessionId||scanTaskId!=null||lastResultTask!=taskId)return;
   var reason=NeighborDiscovery.Reason(detail.Result?.Stderr);
   operation.Text="主动发现："+Labels.State(detail.State)+(string.IsNullOrEmpty(reason)?"。记录将随采样刷新。":" · "+reason);
  }catch(Exception error){if(c==owner&&requestedDevice==deviceId&&requestedSession==sessionId&&!c.Token.IsCancellationRequested)operation.Text+=" 结果详情读取失败："+error.Message;}
 }
 private void Render()
 {
  if(scan is null)return;
  var current=device();var snapshot=current?.Neighbors;var selected=domains.SelectedItem as NeighborDomainConfig;
  var domain=snapshot?.Domains.FirstOrDefault(d=>d.Id==selected?.Id);var query=search.Text.Trim();
  var rows=(domain?.Rows??[]).Where(r=>new[]{r.Ip,r.Mac,r.Hostname,r.Port}.Any(v=>v.Contains(query,StringComparison.OrdinalIgnoreCase))).ToArray();
  Bind(Entries,rows);
  var others=(snapshot?.Unclassified??[]).Where(r=>r.Interface==selected?.Interface).ToArray();Bind(unmatched,others);unknown.Visibility=scope=="lan"&&others.Length>0?Visibility.Visible:Visibility.Collapsed;
  var supported=current?.Registration.Capabilities?.Contains("neighbors_v1")==true;
  status.Text=current is null?"请选择设备。":!supported?"当前Probe不支持邻居发现，请升级探针。":available.Length==0?"尚未配置此类广播域，请在模板生成器的“邻居发现”中配置、发布并应用。":domain is null?"等待探针采集。":$"{rows.Length} 条 · {domain.Interface} · 采样 {Labels.Time(snapshot!.SampledAt)}"+(snapshot.Stale?" · 已过期/设备离线":"")+((snapshot.Limited||domain.Limited)?" · 达到采集上限，清单不完整":"")+(domain.Reason.Length>0?" · "+NeighborDiscovery.Reason(domain.Reason):"")+(rows.Length==0?" · 本次未发现匹配记录":"");
  var ready=connection()?.Synchronized==true&&current is {Online:true,Managed:true}&&supported&&selected!=null;
  scan.IsEnabled=ready&&!busy&&scanTaskId is null&&connection()?.Busy!=true&&connection()?.Pending==null;
  cancel.IsEnabled=ready&&!busy&&scanTaskId!=null&&connection()?.Busy!=true&&connection()?.Pending==null;refresh.IsEnabled=connection()!=null;
 }
 private static void Bind(DataGrid grid,NeighborRow[] rows)
 {
  if(grid.ItemsSource is NeighborRow[] previous&&previous.SequenceEqual(rows))return;
  var selected=grid.SelectedItem as NeighborRow;var anchor=TableBehavior.Capture(grid);grid.ItemsSource=rows;
  grid.SelectedItem=rows.FirstOrDefault(r=>r.Mac==selected?.Mac&&r.Ip==selected.Ip);TableBehavior.Restore(grid,anchor);
 }
 private async Task StartScan()
 {
  var current=device();var c=connection();var selected=domains.SelectedItem as NeighborDomainConfig;if(c is null||current is null||selected is null||busy)return;
  busy=true;lastResultTask=null;Render();operation.Text="正在请求主动发现…";
  try{
   var response=await c.ExecuteAsync(new Mutation("主动发现邻居",$"devices/{ApiClient.Segment(current.DeviceId)}/neighbor-scans",new{domain_id=selected.Id,cidr=cidr.Text.Trim(),config_revision=current.AppliedRevision}));
   if(c!=connection()||current.DeviceId!=device()?.DeviceId)return;scanTaskId=response.GetProperty("task_id").GetString();operation.Text="已提交，执行最多30秒；仅扫描该本地接口广播域。";
  }catch(Exception error){if(c==connection()&&current.DeviceId==device()?.DeviceId)operation.Text=c.Pending!=null?"响应不确定，请从原请求入口重试；不会创建替代扫描。":error.Message;}
  finally{busy=false;Render();}
 }
 private async Task CancelScan()
 {
  var c=connection();var current=device();var id=scanTaskId;if(c is null||current is null||id is null||busy)return;
  busy=true;Render();try{await c.ExecuteAsync(new Mutation("停止邻居发现",$"devices/{ApiClient.Segment(current.DeviceId)}/neighbor-scans/{ApiClient.Segment(id)}/cancel",new{}));if(c==connection()&&current.DeviceId==device()?.DeviceId)operation.Text="已请求停止，等待原扫描任务结束。";}
  catch(Exception error){if(c==connection()&&current.DeviceId==device()?.DeviceId)operation.Text=c.Pending!=null?"停止请求响应不确定，请重试原请求。":error.Message;}
  finally{busy=false;Render();}
 }
}
