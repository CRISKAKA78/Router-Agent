using System.Windows;
using System.Windows.Controls;
using System.Windows.Automation;
using RouterWorkbench.Client;
using RouterAgent.Neighbors;

namespace RouterWorkbench.Desktop;

internal sealed class NeighborView : UserControl
{
 private readonly string scope;
 private readonly Func<WorkspaceConnection?> connection;
 private readonly Func<Device?> device;
 private readonly ComboBox domains=new(),ranges=new(),view=new();
 private readonly CheckBox custom=new(){Content="自定义范围"};
 private readonly TextBlock rangeInfo=Ui.Text("尚未识别扫描范围。",true),rangeError=Ui.Text("",true);
 private ulong revision;
 private string? inspectionKey;
 private ScanRange? selectedRange;
 private bool rendering;
 private readonly Button inspect;
 private readonly ProgressBar progress=new(){IsIndeterminate=true,Height=4,Visibility=Visibility.Collapsed,Margin=new(0,4,0,4)};
 private readonly TextBox cidr=Ui.Input(""),search=Ui.Input("");
 private readonly TextBlock status=Ui.Text("请选择设备。",true),operation=Ui.Text("",true);
 internal DataGrid Entries {get;}=Ui.Table("邻居设备",("IP地址","IpText",165),("MAC地址","Mac",170),("本机转发端口","PortText",130),("主机名","NameText",150),("记录状态","StateText",140),("来源","SourceText",180),("首次发现","FirstSeenText",160),("最后发现","LastSeenText",160));
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
  domains.MinWidth=200;domains.DisplayMemberPath=nameof(NeighborDomainConfig.Label);domains.SelectionChanged+=(_,_)=>{ranges.ItemsSource=null;Render();};
  AutomationProperties.SetName(domains,"邻居广播域");AutomationProperties.SetName(cidr,"主动发现IPv4范围");AutomationProperties.SetName(search,"搜索IP MAC或主机名");
  ranges.MinWidth=220;ranges.DisplayMemberPath=nameof(ScanRange.Label);ranges.SelectionChanged+=(_,_)=>Render();AutomationProperties.SetName(ranges,"直连IPv4扫描范围");
  view.ItemsSource=new[]{"最近发现","当前记录"};view.SelectedIndex=0;view.SelectionChanged+=(_,_)=>Render();AutomationProperties.SetName(view,"发现记录视图");
  custom.Checked+=(_,_)=>Render();custom.Unchecked+=(_,_)=>Render();cidr.TextChanged+=(_,_)=>Render();AutomationProperties.SetHelpText(cidr,"仅允许当前直连网络内的/24到/32；输入主机地址将规范化。错误在扫描范围下方显示。");
  cidr.MinWidth=150;cidr.ToolTip="填写该接口直连的IPv4 CIDR，例如192.168.5.0/24；每次最多256个地址。";
  search.MinWidth=140;search.ToolTip="按IP、MAC、主机名或端口筛选";search.TextChanged+=(_,_)=>Render();
  scan=Ui.Button("主动发现",()=>_=StartScan());cancel=Ui.Button("停止发现",()=>_=CancelScan());refresh=Ui.Button("刷新记录",()=>{connection()?.Invalidate();Render();});
  inspect=Ui.Button("识别网络",()=>_=Inspect());
  var controls=new WrapPanel();foreach(var child in new UIElement[]{Ui.Text("采集网络"),domains,view,scan,cancel,refresh,inspect,Ui.Text("搜索"),search}){if(child is FrameworkElement f)f.Margin=new(0,3,8,3);controls.Children.Add(child);}
  status.TextWrapping=operation.TextWrapping=TextWrapping.Wrap;status.Margin=new(0,8,0,4);operation.Margin=new(0,0,0,8);
  unknown=new Expander{Header="未匹配LAN端口的记录（不计入LAN清单）",Content=unmatched,Visibility=Visibility.Collapsed};unmatched.MaxHeight=200;
  var bottom=new StackPanel{Children={unknown}};
  var body=new DockPanel();DockPanel.SetDock(bottom,Dock.Bottom);body.Children.Add(bottom);body.Children.Add(Entries);
  var note=Ui.Text(scope=="lan"?"依据本机LAN转发端口归类；端口表示到达路径，不能证明终端直接插线。":"所选本地接口广播域中的已发现记录；与LAN清单允许重叠。缓存和租约不表示当前在线。",true);note.TextWrapping=TextWrapping.Wrap;
  rangeInfo.TextWrapping=rangeError.TextWrapping=TextWrapping.Wrap;
  var advanced=new Expander{Header="高级选项",Content=new StackPanel{Children={custom,cidr}}};cidr.Visibility=Visibility.Collapsed;
  var retention=Ui.Text("最近发现保留24小时/每设备最多1024条，仅存Server内存；重启、Session或配置切换清空。历史记录不表示当前在线。",true);retention.TextWrapping=TextWrapping.Wrap;
  Content=new Border{Padding=new(16,10,16,16),Child=Ui.Page(new StackPanel{Children={note,controls,ranges,rangeInfo,advanced,rangeError,status,progress,operation,retention}},body)};
  Loaded+=(_,_)=>Update();
 }
 public void Update()
 {
  var current=device();var next=connection();
  if(current?.DeviceId!=deviceId||current?.CurrentSession?.SessionId!=sessionId||next!=owner||current?.AppliedRevision!=revision){deviceId=current?.DeviceId;sessionId=current?.CurrentSession?.SessionId;owner=next;revision=current?.AppliedRevision??0;inspectionKey=null;lastResultTask=null;ranges.ItemsSource=null;scanTaskId=null;operation.Text="";Entries.ItemsSource=null;unmatched.ItemsSource=null;cidr.Text="";search.Text="";}
  var definitions=(current?.NeighborDomains??[]).Where(d=>d.Scope==scope).ToArray();
  if(!available.Select(d=>(d.Id,d.Interface)).SequenceEqual(definitions.Select(d=>(d.Id,d.Interface)))){var selected=(domains.SelectedItem as NeighborDomainConfig)?.Id;available=definitions;domains.ItemsSource=available;domains.SelectedItem=available.FirstOrDefault(d=>d.Id==selected)??available.FirstOrDefault();}
  // Recover a known scan after view/device selection changes using server tasks.
  if(scanTaskId is null&&current is not null){scanTaskId=next?.Snapshot.Tasks.FirstOrDefault(t=>t.DeviceId==current.DeviceId&&t.Type=="neighbor_scan"&&t.State is "received" or "queued" or "running")?.TaskId;}
  if(scanTaskId is {} id&&next?.Snapshot.Tasks.FirstOrDefault(t=>t.TaskId==id) is {} task&&task.State is "success" or "failed" or "timeout" or "rejected"){
   operation.Text="主动发现："+task.StateText+"。记录将随采样刷新。";scanTaskId=null;_ = ShowResult(next!,task.TaskId);
  }
  Render();
  if(current?.NeighborDiscovery is null&&current?.Registration.Capabilities.Contains("neighbors_inspect_v1")==true&&current.Online&&next?.Synchronized==true&&!next.Busy&&next.Pending is null&&available.Length>0&&inspectionKey is null&&!next.Snapshot.Tasks.Any(t=>t.DeviceId==current.DeviceId&&t.Type=="neighbor_inspect"&&t.State is "received" or "queued" or "running"))_=Inspect();
 }
 private async Task ShowResult(WorkspaceConnection c,string taskId)
 {
  var requestedDevice=deviceId;var requestedSession=sessionId;var requestedRevision=revision;lastResultTask=taskId;
  try{
   var detail=await c.TrackAsync(()=>c.Api.GetAsync<TaskDetail>($"tasks/{ApiClient.Segment(taskId)}"));
   if(c!=owner||requestedDevice!=deviceId||requestedSession!=sessionId||requestedRevision!=revision||scanTaskId!=null||lastResultTask!=taskId)return;
   var reason=NeighborDiscovery.Reason(detail.Result?.Stderr);
   operation.Text=detail.NeighborSummary is {} summary?$"响应{summary.Responses}台，其中新增{summary.Added}台、更新{summary.Updated}台。刷新记录不会重新扫描。":"主动发现："+Labels.State(detail.State)+(string.IsNullOrEmpty(reason)?"。记录将随采样刷新。":" · "+reason);
  }catch(Exception error){if(c==owner&&requestedDevice==deviceId&&requestedSession==sessionId&&!c.Token.IsCancellationRequested)operation.Text+=" 结果详情读取失败："+error.Message;}
 }
 private void Render()
 {
  if(scan is null||rendering)return;rendering=true;try{
  var current=device();var snapshot=current?.Neighbors;var selected=domains.SelectedItem as NeighborDomainConfig;
  var domain=snapshot?.Domains.FirstOrDefault(d=>d.Id==selected?.Id);var query=search.Text.Trim();
  var candidates=view.SelectedIndex==0&&current?.RecentNeighbors is {} recent?recent.Where(r=>r.DomainId==selected?.Id).ToArray():domain?.Rows??[];
  var rows=candidates.Where(r=>new[]{r.Ip,r.Mac,r.Hostname,r.Port}.Any(v=>v.Contains(query,StringComparison.OrdinalIgnoreCase))).ToArray();
  Bind(Entries,rows);
  var others=(snapshot?.Unclassified??[]).Where(r=>r.Interface==selected?.Interface).ToArray();Bind(unmatched,others);unknown.Visibility=scope=="lan"&&others.Length>0?Visibility.Visible:Visibility.Collapsed;
  var supported=current?.Registration.Capabilities?.Contains("neighbors_v1")==true;
  status.Text=current is null?"请选择设备。":!supported?"当前Probe不支持邻居发现，请升级探针。":available.Length==0?"尚未配置此类广播域，请在模板生成器的“邻居发现”中配置、发布并应用。":domain is null?"等待探针采集。":$"{rows.Length} 条 · {domain.Interface} · 采样 {Labels.Time(snapshot!.SampledAt)}"+(snapshot.Stale?" · 已过期/设备离线":"")+((snapshot.Limited||domain.Limited)?" · 达到采集上限，清单不完整":"")+(domain.Reason.Length>0?" · "+NeighborDiscovery.Reason(domain.Reason):"")+(rows.Length==0?" · 本次未发现匹配记录":"");
  var discovery=current?.NeighborDiscovery;
  var network=discovery?.Networks.FirstOrDefault(n=>n.Interface==selected?.Interface);
  var options=network is null?[]:NeighborNetworks.Ranges(network);
  if(ranges.ItemsSource is not ScanRange[] previous||!previous.SequenceEqual(options)){var old=ranges.SelectedItem as ScanRange;ranges.ItemsSource=options;ranges.SelectedItem=options.FirstOrDefault(r=>r==old)??options.FirstOrDefault();}
  ranges.Visibility=options.Length>1?Visibility.Visible:Visibility.Collapsed;
  selectedRange=ranges.SelectedItem as ScanRange;string? error=null;
  if(network is null||discovery is null)error="尚未设备验证，请点击“识别网络”；旧Probe需要更新后才能自动选择范围。";
  else if(discovery.Expired(DateTimeOffset.UtcNow)||discovery.ConfigRevision!=current?.AppliedRevision||discovery.SessionId!=current?.CurrentSession?.SessionId)error="网络检测已过期，请重新识别网络。";
  else if(options.Length==0)error="当前网络没有可探测的直连IPv4地址。";
  else if(custom.IsChecked==true)error=NeighborNetworks.Validate(cidr.Text,network,out selectedRange);
  cidr.Visibility=custom.IsChecked==true?Visibility.Visible:Visibility.Collapsed;
  rangeInfo.Text=selectedRange is {} range?$"扫描范围：{range.Cidr}，共{range.Hosts}个主机地址，预计约{range.Seconds}秒 · 最多256地址"+(range.Cidr!=range.ConnectedNetwork?$"（直连{range.ConnectedNetwork}的子范围）":""):"扫描范围不可用";
  rangeError.Text=error??(custom.IsChecked==true&&selectedRange?.Cidr!=cidr.Text.Trim()?$"将自动规范化为 {selectedRange?.Cidr}":"");AutomationProperties.SetHelpText(cidr,rangeError.Text);
  var ready=connection()?.Synchronized==true&&current is {Online:true,Managed:true}&&supported&&selected!=null;
  scan.IsEnabled=ready&&error is null&&selectedRange!=null&&!busy&&scanTaskId is null&&connection()?.Busy!=true&&connection()?.Pending==null;
  cancel.IsEnabled=ready&&!busy&&scanTaskId!=null&&connection()?.Busy!=true&&connection()?.Pending==null;refresh.IsEnabled=connection()!=null;progress.Visibility=scanTaskId is null?Visibility.Collapsed:Visibility.Visible;inspect.IsEnabled=current?.Online==true&&current.Registration.Capabilities.Contains("neighbors_inspect_v1")&&connection()?.Synchronized==true&&connection()?.Busy!=true&&connection()?.Pending==null&&!busy;
  }finally{rendering=false;}
 }
 private static void Bind(DataGrid grid,NeighborRow[] rows)
 {
  if(grid.ItemsSource is NeighborRow[] previous&&previous.SequenceEqual(rows))return;
  var selected=grid.SelectedItem as NeighborRow;var anchor=TableBehavior.Capture(grid);grid.ItemsSource=rows;
  grid.SelectedItem=rows.FirstOrDefault(r=>r.Mac==selected?.Mac&&r.Ip==selected.Ip);TableBehavior.Restore(grid,anchor);
 }
 private async Task StartScan()
 {
  var current=device();var c=connection();var selected=domains.SelectedItem as NeighborDomainConfig;if(c is null||current is null||selected is null||busy||selectedRange is null||!scan.IsEnabled)return;
  busy=true;lastResultTask=null;Render();operation.Text="正在请求主动发现…";
  try{
   var response=await c.ExecuteAsync(new Mutation("主动发现邻居",$"devices/{ApiClient.Segment(current.DeviceId)}/neighbor-scans",new{domain_id=selected.Id,cidr=selectedRange.Cidr,config_revision=current.AppliedRevision}));
   if(c!=connection()||current.DeviceId!=device()?.DeviceId||current.CurrentSession?.SessionId!=device()?.CurrentSession?.SessionId||current.AppliedRevision!=device()?.AppliedRevision)return;scanTaskId=response.GetProperty("task_id").GetString();operation.Text="⏳ 主动发现已提交，正在等待结果，最多30秒；只扫描所选直连网络。";
  }catch(Exception error){if(c==connection()&&current.DeviceId==device()?.DeviceId)operation.Text=c.Pending!=null?"响应不确定，请从原请求入口重试；不会创建替代扫描。":error.Message;}
  finally{busy=false;Render();}
 }
 private async Task Inspect()
 {
  var c=connection();var current=device();if(c is null||current?.CurrentSession is null||busy||c.Pending!=null||c.Busy)return;
  var key=current.DeviceId+"/"+current.CurrentSession.SessionId+"/"+current.AppliedRevision;inspectionKey=key;busy=true;Render();
  try{await c.ExecuteAsync(new Mutation("只读识别网络",$"devices/{ApiClient.Segment(current.DeviceId)}/neighbor-inspections",new{session_id=current.CurrentSession.SessionId,config_revision=current.AppliedRevision,vendor_test=false}));if(c==owner&&current.DeviceId==deviceId)operation.Text="已请求只读检测；刷新记录将显示结果，不会扫描设备。";}
  catch(Exception e){if(c==owner&&current.DeviceId==deviceId)operation.Text=c.Pending is not null?"检测响应不确定，请核对原请求，不自动重复执行。":e.Message;}
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
