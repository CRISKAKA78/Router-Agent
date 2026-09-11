using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterAgent.Neighbors;
using RouterWorkbench.Desktop;
namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
 private static async Task NeighborChecks(MainWindow main)
 {
  var source=((DataGrid)main.FindName("DevicesGrid")).SelectedItem as Device ?? throw new Exception("device required");
  var row=new NeighborRow("192.0.2.2","02:00:00:00:00:02","LAN2","测试设备","arp+fdb","cached");
  var configs=new[]{new NeighborDomainConfig("lan","lan","br0",["LAN1","LAN2"]),new NeighborDomainConfig("local","broadcast","br0")};
  Device? current=source with {Registration=source.Registration with {Capabilities=["neighbors_v1"]},NeighborDomains=configs,RecentNeighbors=null,Neighbors=new(1,30,[new("lan","lan","br0","ok","",false,[row]),new("local","broadcast","br0","ok","",false,[row,row with {Ip="",Mac="02:00:00:00:00:03",State="mac_only"}])],[],false,DateTimeOffset.Now,false)};
  var lan=new NeighborView("lan",()=>null,()=>current);var local=new NeighborView("broadcast",()=>null,()=>current);
  var tabs=new TabControl{Items={new TabItem{Header="LAN 下接设备",Content=lan},new TabItem{Header="本机广播域设备",Content=local}}};
  var window=new Window{Title="邻居发现验证",Width=1120,Height=650,Content=tabs};window.SetResourceReference(Control.BackgroundProperty,"Background");window.Show();
  try{
   lan.Update();local.Update();await Task.Delay(80);window.UpdateLayout();
   Check(lan.Entries.Items.Count==1&&local.Entries.Items.Count==2,"LAN and broadcast lists overlap without classifying an uplink");
   tabs.SelectedIndex=1;await Task.Delay(40);window.UpdateLayout();
   Check(Visuals<TextBox>(local).All(t=>System.Windows.Automation.AutomationProperties.GetName(t)!="主动发现IPv4范围"||!t.IsVisible),"custom CIDR is collapsed by default");
   var discovered=new NetworkDiscovery([new("br0",true,false,"",true,"",["192.0.2.1/24"],["192.0.2.0/24"],["LAN1","LAN2"])],"","not_tested","",[],current.AppliedRevision,current.CurrentSession?.SessionId??"",DateTimeOffset.UtcNow,false);
   current=current with{NeighborDiscovery=discovered};local.Update();
   Check(Visuals<TextBlock>(local).Any(t=>t.Text.Contains("扫描范围：192.0.2.0/24，共254个主机地址，预计约18秒")),"WPF computes scan range and estimate without text input");
   current=current with{RecentNeighbors=[row with{DomainId="local",Scope="broadcast",State="recent",LastSeen=DateTimeOffset.UtcNow.AddMinutes(-2),FirstSeen=DateTimeOffset.UtcNow.AddMinutes(-3),Current=false}]};local.Update();
   Check(local.Entries.Items.Count==1&&((NeighborRow)local.Entries.Items[0]).StateText.Contains("最近发现于2分钟前"),"default recent history survives absent current snapshot without claiming online");
   window.UpdateLayout();Render(window,"neighbors-recent-history.png");
   current=current with{RecentNeighbors=null};local.Update();
   Check(configs[1].ToString()=="local · br0","domain selector shows domain and interface instead of DTO members");
   Check(row.StateText=="缓存记录"&&((NeighborRow)local.Entries.Items[1]).IpText=="未知","cached and MAC-only records never pretend online or invent IP");
   var selected=lan.Entries.Items[0];lan.Entries.SelectedItem=selected;lan.Update();Check(ReferenceEquals(selected,lan.Entries.SelectedItem),"unchanged neighbor refresh preserves selection and row identity");
   foreach(var theme in new[]{"Light","Dark"}){Theme.Apply(theme);tabs.SelectedIndex=0;window.UpdateLayout();Render(window,"neighbors-lan-"+theme.ToLowerInvariant()+".png");tabs.SelectedIndex=1;window.UpdateLayout();Render(window,"neighbors-broadcast-"+theme.ToLowerInvariant()+".png");}
   window.Width=760;window.Height=520;await Task.Delay(50);window.UpdateLayout();Render(window,"neighbors-narrow.png");
   current=current with {Neighbors=current.Neighbors! with {Stale=true}};local.Update();Check(Visuals<TextBlock>(local).Any(t=>t.Text.Contains("已过期")),"stale neighbor snapshot is explicit");
   current=null;lan.Update();local.Update();Check(lan.Entries.Items.Count==0&&local.Entries.Items.Count==0,"device change clears both neighbor lists");
  }finally{window.Close();Theme.Apply("Light");main.Activate();}
 }

 private static async Task NeighborInteractionChecks(MainWindow main,ApiClient api,WorkspaceConnection connection,int controlPort)
 {
  var presetTemplate=new ProbeTemplate("fixture","fixture",1,null,[],NeighborProbe:JsonSerializer.SerializeToElement(new{fdb_preset="fnr100"}));
  Check(presetTemplate.NeighborCapabilityError([])?.Contains("neighbors_v1")==true,"template application checks basic Probe capability before send");
  Check(presetTemplate.NeighborCapabilityError(["neighbors_v1"])?.Contains("neighbors_inspect_v1")==true,"FNR100 preset checks additional Probe capability before send");
  Check(presetTemplate.NeighborCapabilityError(["neighbors_v1","neighbors_inspect_v1"]) is null,"capable Probe accepts saved FNR100 preset");
  await using var peer=new TestProbe();const string id="neighbor-smart-wpf";
  await peer.StartAsync(controlPort,id,new{device_id=id,probe_version="neighbor-ui-fixture",arch="x86_64",boot_id="neighbor-test",model="fixture",capabilities=new[]{"neighbors_v1","neighbors_inspect_v1"}});
  var d=await api.GetAsync<Device>("devices/"+id);
  var template=(await api.ExecuteAsync(new("邻居交互模板","probe-templates",new{name="neighbor-ui",properties=new{},neighbor_probe=new{interval_seconds=30,domains=new[]{new{id="local",scope="broadcast",@interface="br0"}}}}))).Deserialize<ProbeTemplate>(ApiJson.Options)!;
  await api.ExecuteAsync(new("纳管邻居测试对端","devices/"+id+"/profile",new{version=d.Profile!.Version,admission="managed",name=id,model_id="",template_id=template.TemplateId,apply_template=true},"PUT"));
  await Eventually(()=>Task.FromResult(peer.ConfigurationRevision==1),"neighbor UI fixture configuration ACK");
  var view=new NeighborView("broadcast",()=>connection,()=>connection.Snapshot.Devices.FirstOrDefault(x=>x.DeviceId==id));
  var window=new Window{Content=view,Width=1120,Height=700};window.Show();
  try {
   connection.Invalidate();await Eventually(()=>{view.Update();return Task.FromResult(Field<Button>(view,"scan").IsEnabled);},"WPF automatically requests kernel-only inventory and enables default range");
   Check(peer.NeighborInspections==1&&peer.NeighborScans==0,"opening neighbors only detects networks, never scans");
   var scan=Field<Button>(view,"scan");scan.RaiseEvent(new RoutedEventArgs(Button.ClickEvent));
   await Eventually(()=>{view.Update();return Task.FromResult(Field<ProgressBar>(view,"progress").Visibility==Visibility.Visible);},"active scan displays indeterminate progress");
   await Eventually(()=>{view.Update();return Task.FromResult(Field<TextBlock>(view,"operation").Text.Contains("响应1台，其中新增1台、更新0台"));},"WPF shows Server-owned response/new/update summary");
   Check(peer.NeighborCidr=="192.0.2.0/24"&&peer.NeighborScans==1,"single click uses normalized direct network without a CIDR textbox");
   Field<Button>(view,"refresh").RaiseEvent(new RoutedEventArgs(Button.ClickEvent));await Task.Delay(200);view.Update();Check(peer.NeighborScans==1,"refresh does not trigger another active scan");
   Field<CheckBox>(view,"custom").IsChecked=true;Field<TextBox>(view,"cidr").Text="198.51.100.1/24";
   Check(!scan.IsEnabled&&Field<TextBlock>(view,"rangeError").Text.Contains("该范围不属于 br0 当前直连网络 192.0.2.0/24"),"WPF off-link custom range has field-level feedback before submission");
   Field<TextBox>(view,"cidr").Text="192.0.2.222/24";Check(scan.IsEnabled&&Field<TextBlock>(view,"rangeError").Text.Contains("自动规范化为 192.0.2.0/24"),"WPF host-address prefix is visibly normalized");
   window.UpdateLayout();Render(window,"neighbors-live-workflow.png");
  } finally {window.Close();main.Activate();}
 }
}
