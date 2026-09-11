using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;
namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program {
 private static async Task IntegratedNetworkChecks(MainWindow window, WorkspaceConnection connection) {
  var tabs=(TabControl)window.FindName("WorkspaceTabs");var previous=tabs.SelectedItem;
  var tab=tabs.Items.Cast<TabItem>().Single(t=>t.Tag?.ToString()=="networks");
  Check(tab.IsEnabled,"overlay workspace remains visible without device capability or configured engine");
  tabs.SelectedItem=tab;
  var workspace=(NetworkWorkspace)tab.Content;
  await Eventually(()=>Task.FromResult(Field<NetworkSettings?>(workspace,"settings") is not null),"integrated overlay page reads real Server settings");
  Check(!Field<NetworkSettings>(workspace,"settings").Configured,"unconfigured EasyTier does not prevent core Server or UI startup");
  Check(!Field<Button>(workspace,"join").IsEnabled,"overlay join is disabled until controller configured");
  var n=(await connection.ExecuteAsync(new Mutation("集成组网定义","networks",new{name="集成组网检查"}))).Deserialize<OverlayNetwork>(ApiJson.Options)!;
  await Eventually(()=>{workspace.Update();return Task.FromResult(Field<OverlayNetwork[]>(workspace,"available").Any(v=>v.NetworkId==n.NetworkId));},"overlay HTTP refresh loads new network through shared connection");
  await connection.ExecuteAsync(new Mutation("清理测试网络",$"networks/{n.NetworkId}",method:"DELETE"));
  tabs.SelectedItem=previous;
 }
 private static async Task NetworkChecks(){
  var network=JsonSerializer.Deserialize<OverlayNetwork>("""{"network_id":"test-network","name":"测试异地组网","cidr":"10.144.144.0/24","peer_urls":[],"routes":[],"revision":1,"created_at":"2026-09-11T00:00:00Z","members":[]}""",ApiJson.Options)!;
  Check(network.NetworkId=="test-network"&&network.Routes.Length==0,"network API identity and empty custom routes");
  var route=JsonSerializer.Deserialize<NetworkRoute>("""{"peer_id":3,"virtual_ip":"10.1.1.3","next_hop_peer_id":2,"cost":2,"proxy_cidrs":[]}""",ApiJson.Options)!;
  Check(route.NextHopPeerId==2,"network next hop contract");
  Check(NetworkWorkspace.NatLabel(3)=="完全锥形（NAT1）"&&NetworkWorkspace.NatLabel(6)=="对称型（NAT4）"&&NetworkWorkspace.NatLabel(1)=="公网（无 NAT）"&&NetworkWorkspace.NatLabel(null)=="未提供","NAT labels preserve unknown and non-NAT states");
  var cfg=MemberNetworkConfig.Default("device");var payload=JsonSerializer.Serialize(cfg,ApiJson.Options);Check(payload.Contains("\"lazy_p2p\"")&&payload.Contains("\"enable_manual_routes\":true")&&payload.Contains("\"system_forward\":true"),"member configuration uses exact API field names");
  string? observed="a";var observerPage=new NetworkWorkspace(()=>null,()=>observed);observerPage.Update();observed="b";observerPage.Update();Check(Field<string?>(observerPage,"observer")=="b"&&Field<NetworkTopology>(observerPage,"topology").Nodes.Length==0,"left observer change discards previous snapshots");
  var page=new NetworkWorkspace(()=>null);var window=new Window{Content=page,Width=1280,Height=780,Title="组网验证"};window.Show();await Task.Delay(80);page.Update();
  Check(Field<DataGrid>(page,"members").Columns.Count==11,"member list provides all eleven requested fields");Check(!Visuals<TabItem>(page).Any(t=>t.Header?.ToString()=="链路"),"redundant links tab removed");
  Check(!Visuals<Button>(page).Any(b=>b.IsEnabled&&b.Content?.ToString()=="添加成员"),"network writes disabled without connection");Render(window,"networks-empty-light.png");
  Theme.Apply("Dark");window.Width=1000;window.Height=700;await Task.Delay(60);Render(window,"networks-empty-dark.png");window.Close();
  var graph=new NetworkTopologyView();var now=DateTimeOffset.UtcNow;
  var data=new NetworkTopology([new("device:a","a",1,"10.144.144.1","running",true,false),new("device:b","b",2,"10.144.144.2","unknown",false,false)], [new("device:a","device:b","udp","a",now,false,true,new(2,"udp",null,null,null,null))],[],false);
  window=new Window{Content=graph,Width=1100,Height=700,Title="组网拓扑验证"};window.Show();graph.Render(data,network,new Dictionary<string,string>{{"a","厂区 A"},{"b","厂区 B"}},false);await Task.Delay(60);
  Check(Visuals<Button>(graph).Count()==2,"topology uses real nodes with accessible buttons");Check(Visuals<System.Windows.Shapes.Line>(graph).Count()==1,"topology does not invent mesh edges");Render(window,"networks-topology-dark.png");
  Theme.Apply("Light");graph.Render(data,network,new Dictionary<string,string>(),true);await Task.Delay(60);Check(!Visuals<System.Windows.Shapes.Line>(graph).Any(),"logical mode does not imply live links");Render(window,"networks-logical-light.png");window.Close();
  window=new Window{Content=new TextBlock{Text="表单验证"},Width=760,Height=700};window.Show();
  void InspectDialog(string screenshot,Action<Window> check){
   var timer=new System.Windows.Threading.DispatcherTimer{Interval=TimeSpan.FromMilliseconds(180)};
   timer.Tick+=(_,_)=>{var modal=Application.Current.Windows.Cast<Window>().FirstOrDefault(w=>w is NetworkDialog&&w.IsVisible);if(modal==null)return;timer.Stop();check(modal);Render(modal,screenshot);modal.Close();};timer.Start();
  }
  InspectDialog("networks-create-light.png",modal=>{Check(Visuals<PasswordBox>(modal).Any(),"create network password is masked");Check(Visuals<Button>(modal).Any(b=>b.Content?.ToString()=="随机"),"password has random generator");Check(!Visuals<TextBlock>(modal).Any(t=>t.Text=="自定义路由"),"network form does not edit member routes");});
  NetworkDialog.Network(window,null,_=>Task.FromResult(true));
  InspectDialog("networks-members-light.png",modal=>{Check(Visuals<CheckBox>(modal).Count()==2,"join dialog offers multiple device selection");Check(Visuals<TextBlock>(modal).Any(t=>t.Text.Contains("固定地址成员")),"first join requires a static anchor");});
  NetworkDialog.Members(window,network,[("a","厂区 A"),("b","厂区 B")],_=>Task.FromResult(true));
  Theme.Apply("Dark");InspectDialog("networks-config-dark.png",modal=>{Check(Visuals<CheckBox>(modal).Count()==6,"member settings offer five connection flags and manual routes");Check(Visuals<TextBlock>(modal).Any(t=>t.Text.Contains("仅保存配置")),"stopped member edit does not imply automatic start");});
  NetworkDialog.Member(window,new("a","machine","instance","","stop",1,null),"厂区 A",_=>Task.FromResult(true));window.Close();
 }
}
