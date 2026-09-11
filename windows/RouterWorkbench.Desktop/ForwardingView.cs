using System.Text.Json;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

// Public API only. Scope changes cancel reads; mutations retain WorkspaceConnection identity.
public sealed class ForwardingView : UserControl
{
 private readonly string kind;
 private readonly Func<WorkspaceConnection?> connection;
 private readonly Func<Device?> device;
 private readonly Func<Mutation,Task<JsonElement>> execute;
 private readonly ComboBox interfaces=new(){MinWidth=190},ip=new(){IsEditable=true,MinWidth=180},protocol=new(){ItemsSource=new[]{"tcp","udp"},SelectedIndex=0,MinWidth=80},serial=new(){MinWidth=175},parity=new(){ItemsSource=new[]{"none","odd","even"},SelectedIndex=0,MinWidth=90};
 private readonly TextBox port=Ui.Input("80",75),lease=Ui.Input("240",80),baud=Ui.Input("115200",90),registration=Ui.Input(""),details=Ui.Input("");
 private readonly TextBlock status=Ui.Text("请选择在线设备。",true),message=Ui.Text("",true);
 private readonly DataGrid rows=Ui.Table("穿透条目",("状态","StateText",80),("协议","Protocol",70),("目标","Target",235),("外部地址","Endpoint",190),("到期","LeaseText",200),("连接","Connections",145),("双向字节","Traffic",175));
 private readonly Button create,close,copy,refresh;
 private readonly Expander authPanel;
 private WorkspaceConnection? owner;
 private string scope="",registrationId="";
 private bool loading,busy;
 private int epoch;
 private CancellationTokenSource reads=new();
 private ForwardCapabilities? capabilities;
 private ForwardMapping[] mappings=[];
 private DateTime nextRead;
 public ForwardingView(string kind,Func<WorkspaceConnection?> connection,Func<Device?> device,Func<Mutation,Task<JsonElement>> execute)
 {
  this.kind=kind;this.connection=connection;this.device=device;this.execute=execute;ip.DisplayMemberPath=nameof(ForwardCandidate.Label);TextSearch.SetTextPath(ip,nameof(ForwardCandidate.Ip));
  create=Ui.Button(kind=="lan"?"开启内网穿透":"开启串口透传",()=>_=Create(),true);
  close=Ui.Button("关闭所选条目",()=>_=CloseSelected());copy=Ui.Button("复制连接地址",Copy);refresh=Ui.Button("刷新",()=>{capabilities=null;nextRead=DateTime.MinValue;Update();});
  var form=new WrapPanel();void Field(string label,FrameworkElement value){AutomationProperties.SetName(value,label);value.Margin=new(0,4,12,4);form.Children.Add(new StackPanel{Children={Ui.Text(label,true),value}});}
  if(kind=="lan"){Field("本机接口",interfaces);Field("目标IPv4（可手输）",ip);Field("目标端口",port);Field("协议",protocol);}else{Field("设备串口（排除控制台）",serial);Field("波特率",baud);Field("校验位",parity);form.Children.Add(Ui.Text("8 数据位 / 1 停止位 · TCP",true));}
  Field("有效期（分钟）",lease);form.Children.Add(create);form.Children.Add(refresh);
  var hint=Ui.Text(kind=="lan"?"以所选接口的本机IP代理访问同网段目标，无需修改目标默认网关。UDP已知丢包/延迟问题暂未处理。":"先发送注册包并等待 OK，再发送业务数据。未鉴权数据不进入串口；同一串口仅一个已认证连接。",true);hint.TextWrapping=TextWrapping.Wrap;
  var leaseHint=Ui.Text("默认240分钟；0=不限时。设备失联、会话替换或服务重启仍关闭，不自动重建。",true);
  registration.IsReadOnly=true;registration.MinWidth=420;registration.ToolTip="注册包只在本次创建响应显示；不写入本地设置。明文TCP注册不防窃听。";
  var auth=new StackPanel{Visibility=kind=="serial"?Visibility.Visible:Visibility.Collapsed,Children={Ui.Text("注册包（末尾为实际CRLF；网络工具也可手动追加换行）",true),registration,Ui.Button("复制注册包",()=>{if(registration.Text.Length>0)Clipboard.SetText(registration.Text);})}};
  details.IsReadOnly=true;details.TextWrapping=TextWrapping.Wrap;details.AcceptsReturn=true;details.MaxHeight=230;details.VerticalScrollBarVisibility=ScrollBarVisibility.Auto;
  authPanel=new Expander{Header="注册包（仅本次创建响应可见）",Content=auth,Visibility=kind=="serial"?Visibility.Visible:Visibility.Collapsed};
 var actions=new WrapPanel{Children={copy,close}};var bottom=new StackPanel{Children={actions,new Expander{Header="条目详情 / 关闭原因",Content=details},authPanel}};
  rows.SelectionChanged+=(_,_)=>RenderSelected();interfaces.SelectionChanged+=(_,_)=>Candidates();
  status.TextWrapping=message.TextWrapping=TextWrapping.Wrap;
  var footer=new ScrollViewer{Content=bottom,VerticalScrollBarVisibility=ScrollBarVisibility.Auto,MaxHeight=250};
  var header=new ScrollViewer{Content=new StackPanel{Children={form,hint,leaseHint,status,message}},VerticalScrollBarVisibility=ScrollBarVisibility.Auto,MaxHeight=230};
  var body=new DockPanel();DockPanel.SetDock(footer,Dock.Bottom);body.Children.Add(footer);body.Children.Add(rows);
  Content=new Border{Padding=new(16),Child=Ui.Page(header,body)};
  SizeChanged+=(_,_)=>{var space=Math.Max(0,ActualHeight-32);header.MaxHeight=Math.Max(90,Math.Min(220,space*.42));footer.MaxHeight=Math.Max(45,Math.Min(230,space*.27));};
  Loaded+=(_,_)=>Update();
 }
 private bool Current(WorkspaceConnection c,int version)=>owner==c&&connection()==c&&epoch==version&&!c.Token.IsCancellationRequested;
 public void Update()
 {
  var c=connection();var d=device();var key=d?.DeviceId+"/"+d?.CurrentSession?.SessionId;
  if(owner!=c||scope!=key){reads.Cancel();reads.Dispose();reads=new();owner=c;scope=key;epoch++;capabilities=null;loading=false;busy=false;mappings=[];rows.ItemsSource=mappings;registration.Text="";registrationId="";authPanel.IsExpanded=false;message.Text="";nextRead=DateTime.MinValue;}
  var supported=d?.Registration.Capabilities.Contains("forwarding_v1")==true;
  status.Text=c?.Synchronized!=true?"等待服务器同步。":d?.Online!=true?"请选择在线设备；离线设备不能创建通道。":!supported?"设备不支持穿透：需新版Probe及同目录router-forwarding-agent、补丁版gost，并重新连接。":capabilities is {Backend:false}?"设备缺少GOST辅助程序。":$"{mappings.Length} 条记录 · "+(capabilities is null?"等待能力查询":"GOST设备能力已获取");
  create.IsEnabled=!busy&&c is {Synchronized:true,Busy:false,Pending:null}&&d?.Online==true&&supported&&capabilities?.Backend==true;
  refresh.IsEnabled=!busy&&!loading&&c?.Synchronized==true;RenderSelected();
  if(c is {Synchronized:true}&&d!=null&&!loading&&DateTime.UtcNow>=nextRead&&IsLoaded)_=Read(c,d,epoch,supported);
 }
 private async Task Read(WorkspaceConnection c,Device d,int version,bool supported)
 {
  loading=true;nextRead=DateTime.UtcNow.AddSeconds(5);var token=reads.Token;
  try{
   var list=await c.TrackAsync(()=>c.Api.ListAsync<ForwardMapping>("forwardings?device_id="+ApiClient.Segment(d.DeviceId),token));if(!Current(c,version))return;
   var selected=(rows.SelectedItem as ForwardMapping)?.Id;mappings=list.Where(x=>x.Kind==kind).ToArray();rows.ItemsSource=mappings;rows.SelectedItem=mappings.FirstOrDefault(x=>x.Id==selected)??mappings.FirstOrDefault();
   if(registrationId!=""&&mappings.Any(x=>x.Id==registrationId&&(x.Released||x.State=="failed"))){registration.Text="";registrationId="";}
   if(d.Online&&supported&&capabilities==null){var v=await c.TrackAsync(()=>c.Api.GetAsync<ForwardCapabilities>("devices/"+ApiClient.Segment(d.DeviceId)+"/forwarding-capabilities",token));if(!Current(c,version))return;capabilities=v;interfaces.ItemsSource=v.Interfaces;interfaces.SelectedIndex=v.Interfaces.Length>0?0:-1;serial.ItemsSource=v.Serials;serial.SelectedIndex=v.Serials.Length>0?0:-1;Candidates();}
  }catch(OperationCanceledException){}catch(Exception e){if(Current(c,version)){message.Text=e.Message;nextRead=DateTime.UtcNow.AddSeconds(15);}}finally{if(Current(c,version)){loading=false;Update();}}
 }
 private void Candidates(){var name=(interfaces.SelectedItem as ForwardInterface)?.Name;var prior=ip.Text;ip.ItemsSource=ForwardingChoices.Candidates(device()?.Neighbors,name);ip.Text=prior;}
 private async Task Create()
 {
  if(!create.IsEnabled)return;var c=owner;var d=device();if(c==null||d==null)return;var version=epoch;
  if(!int.TryParse(lease.Text,out var minutes)||minutes<0||minutes>525600){message.Text="有效期须为0～525600分钟。";return;}
  if(kind=="lan"&&(!int.TryParse(port.Text,out var p)||p<1||p>65535||!System.Net.IPAddress.TryParse(ip.Text.Trim(),out _))){message.Text="请输入有效目标IP及1～65535端口。";return;}
  if(kind=="serial"&&(!int.TryParse(baud.Text,out var b)||b<50||b>4000000||serial.SelectedItem==null)){message.Text="请选择串口并输入有效波特率。";return;}
  var q=new {device_id=d.DeviceId,kind,protocol=kind=="serial"?"tcp":protocol.SelectedItem?.ToString(),@interface=(interfaces.SelectedItem as ForwardInterface)?.Name??"",target_ip=kind=="lan"?ip.Text.Trim():"",target_port=kind=="lan"?int.Parse(port.Text):0,serial=serial.SelectedItem?.ToString()??"",baud=kind=="serial"?int.Parse(baud.Text):0,data_bits=8,stop_bits=1,parity=parity.SelectedItem?.ToString()??"none",lease_minutes=minutes};
  busy=true;Update();try{var result=await execute(new Mutation("创建"+(kind=="lan"?"内网穿透":"串口透传"),"forwardings",q));if(Current(c,version))Accept(result);}catch(Exception e){if(Current(c,version))message.Text=e.Message;}finally{if(Current(c,version)){busy=false;nextRead=DateTime.MinValue;Update();}}
 }
 public void Accept(JsonElement result){if(!result.TryGetProperty("mapping",out _))return;var v=result.Deserialize<ForwardCreated>(ApiJson.Options);if(v?.Mapping.Kind!=kind||v.Mapping.DeviceId!=device()?.DeviceId||v.Mapping.SessionId!=device()?.CurrentSession?.SessionId)return;mappings=new[]{v.Mapping}.Concat(mappings.Where(x=>x.Id!=v.Mapping.Id)).ToArray();rows.ItemsSource=mappings;rows.SelectedItem=v.Mapping;registration.Text=v.Registration??"";registrationId=v.Mapping.Id;authPanel.IsExpanded=kind=="serial";message.Text="条目已创建；请以列表中的实时状态为准。";}
 private async Task CloseSelected(){if(rows.SelectedItem is not ForwardMapping row||row.Released||owner==null)return;var c=owner;var version=epoch;busy=true;Update();try{await execute(new Mutation("关闭穿透","forwardings/"+ApiClient.Segment(row.Id)+"/close"));if(Current(c,version)){registration.Text="";registrationId="";message.Text="已关闭Server入口，设备回收状态请查看详情。";}}catch(Exception e){if(Current(c,version))message.Text=e.Message;}finally{if(Current(c,version)){busy=false;nextRead=DateTime.MinValue;Update();}}}
 private void Copy(){if(rows.SelectedItem is ForwardMapping {CanConnect:true} row&&owner?.Synchronized==true)Clipboard.SetText(row.Endpoint);}
 private void RenderSelected(){var row=rows.SelectedItem as ForwardMapping;copy.IsEnabled=row?.CanConnect==true&&owner?.Synchronized==true;close.IsEnabled=row is {Released:false}&&!busy&&owner is {Synchronized:true,Busy:false,Pending:null};details.Text=row==null?"选择条目查看接口、源IP、租期、关闭原因和回收状态。":$"编号：{row.Id}\n设备：{row.DeviceId}\n状态：{row.StateText} / {row.ReasonText}\n目标：{row.Target}\n代理源IP：{row.SourceIp}\n历史外部地址：{row.Host}:{row.Port}（{(row.CanConnect?"有效":"已失效或未就绪")}）\n创建：{row.CreatedAt.LocalDateTime}\n到期：{row.LeaseText}\n关闭：{row.ClosedAt?.LocalDateTime}\nServer入口已回收：{row.Released} / 设备已确认回收：{row.DeviceReleased}\n端口可复用时间：{row.ReusableAfter?.LocalDateTime}\n流量：{row.Traffic} / 连接：{row.Connections}";}
}
