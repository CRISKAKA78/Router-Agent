using System.Security.Cryptography;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;

internal sealed class NetworkDialog : Window
{
 internal readonly StackPanel Body=new();
 internal NetworkDialog(Window owner,string title,Func<Task<bool>> save,string action="保存")
 {
  Owner=owner;Title=title;Width=640;SizeToContent=SizeToContent.Height;MaxHeight=Math.Max(500,SystemParameters.WorkArea.Height-80);WindowStartupLocation=WindowStartupLocation.CenterOwner;
  SetResourceReference(StyleProperty,typeof(Window));
  var root=new DockPanel{Margin=new(20)};Content=root;
  var error=Ui.Text("",true);error.TextWrapping=TextWrapping.Wrap;var footer=new StackPanel();DockPanel.SetDock(footer,Dock.Bottom);root.Children.Add(footer);
  var accept=Ui.Button(action,()=>{});accept.IsDefault=true;accept.SetResourceReference(StyleProperty,"PrimaryButton");var cancel=Ui.Button("取消",()=>Close());cancel.IsCancel=true;
  footer.Children.Add(error);footer.Children.Add(Ui.Bar(accept,cancel));root.Children.Add(new ScrollViewer{Content=Body,VerticalScrollBarVisibility=ScrollBarVisibility.Auto,MaxHeight=Math.Max(330,SystemParameters.WorkArea.Height-230)});
  bool busy=false;Closing+=(_,e)=>e.Cancel=busy;
  accept.Click+=async(_,_)=>{if(busy)return;busy=true;Body.IsEnabled=accept.IsEnabled=cancel.IsEnabled=false;try{if(await save()){busy=false;DialogResult=true;}else error.Text="未保存。请查看主窗口的操作反馈；原请求不确定时先核实该请求。";}catch(Exception e){error.Text=e.Message;}finally{busy=false;Body.IsEnabled=accept.IsEnabled=cancel.IsEnabled=true;}};
 }
 internal TextBox Input(string label,string value,bool multiline=false){var input=Ui.Input(value);if(multiline){input.AcceptsReturn=true;input.Height=88;input.VerticalScrollBarVisibility=ScrollBarVisibility.Auto;}AutomationProperties.SetName(input,label);Body.Children.Add(Ui.Labeled(label,input));return input;}
 internal CheckBox Check(string label,bool value){var c=new CheckBox{Content=label,IsChecked=value,Margin=new(0,7,0,7)};AutomationProperties.SetName(c,label);Body.Children.Add(c);return c;}
 internal static string[] Lines(TextBox input)=>input.Text.Split(['\r','\n'],StringSplitOptions.RemoveEmptyEntries|StringSplitOptions.TrimEntries);
 internal static void Network(Window owner,OverlayNetwork? current,Func<object,Task<bool>> save)
 {
  TextBox name=null!,cidr=null!,peers=null!,mtu=null!,plain=null!;PasswordBox secret=new();CheckBox reveal=null!;
  var dialog=new NetworkDialog(owner,current is null?"新建网络":"编辑网络",async()=>{
   if(!int.TryParse(mtu.Text,out var size))throw new InvalidOperationException("MTU 必须为整数。");
   if(current is null){var pw=reveal.IsChecked==true?plain.Text:secret.Password;if(pw.Length==0)throw new InvalidOperationException("请设置网络密码。");return await save(new{name=name.Text.Trim(),password=pw,cidr=cidr.Text.Trim(),peer_urls=Lines(peers),mtu=size});}
   return await save(new{name=name.Text.Trim(),cidr=cidr.Text.Trim(),peer_urls=Lines(peers),mtu=size,routes=current.Routes,revision=current.Revision});
  });
  name=dialog.Input("网络名称",current?.Name??"");
  if(current is null){secret.MinHeight=34;AutomationProperties.SetName(secret,"网络密码");plain=Ui.Input("");plain.Visibility=Visibility.Collapsed;var box=new StackPanel();box.Children.Add(secret);box.Children.Add(plain);dialog.Body.Children.Add(Ui.Labeled("网络密码",box));
   reveal=new CheckBox{Content="显示密码"};reveal.Checked+=(_,_)=>{plain.Text=secret.Password;plain.Visibility=Visibility.Visible;secret.Visibility=Visibility.Collapsed;};reveal.Unchecked+=(_,_)=>{secret.Password=plain.Text;secret.Visibility=Visibility.Visible;plain.Visibility=Visibility.Collapsed;};dialog.Body.Children.Add(Ui.Bar(reveal,Ui.Button("随机",()=>{var value=Convert.ToHexString(RandomNumberGenerator.GetBytes(12)).ToLowerInvariant();secret.Password=plain.Text=value;})));}
  cidr=dialog.Input("虚拟地址网段",current?.Cidr??"10.144.144.0/24");
  peers=dialog.Input("初始节点（每行一项，可不填）",string.Join("\n",current?.PeerUrls??[]),true);
  dialog.Body.Children.Add(Ui.Note("留空使用服务器 TCP / UDP 11010；自定义需包含协议，例如 tcp://et.criskaka.com:11010。省略端口时使用 11010。"));
  var id=dialog.Input("网络 ID（自动生成）",current?.NetworkId??"保存后自动生成");id.IsReadOnly=true;
  mtu=dialog.Input("MTU",(current?.Mtu is >0?current.Mtu:1380).ToString());
  if(current!=null)dialog.Body.Children.Add(Ui.Text("网络配置保存后，成员显示待应用；需对成员显式应用。已有成员的路由设置不在此修改。",true));
  dialog.ShowDialog();secret.Clear();
 }
 internal static void Members(Window owner,OverlayNetwork network,(string Id,string Name)[] devices,Func<object,Task<bool>> save)
 {
  var choices=new List<(string Id,CheckBox Box)>();var search=Ui.Input("");var selected=Ui.Text("",true);var anchor=new ComboBox{DisplayMemberPath="Content",MinWidth=250};TextBox? ip=null;
  bool needsAnchor=!network.Members.Any(m=>m.VirtualIp.Length>0);
  var dialog=new NetworkDialog(owner,"添加成员",async()=>{
   var ids=choices.Where(c=>c.Box.IsChecked==true).Select(c=>c.Id).ToArray();if(ids.Length==0)throw new InvalidOperationException("请至少选择一个设备。");
   string? fixedId=(anchor.SelectedItem as ComboBoxItem)?.Tag as string;
   if(needsAnchor&&(fixedId is null||string.IsNullOrWhiteSpace(ip?.Text)))throw new InvalidOperationException("首次加入需指定一个固定地址成员及其虚拟 IP。");
   return await save(new{members=ids.Select(id=>new{device_id=id,virtual_ip=needsAnchor&&id==fixedId?ip!.Text.Trim():""}).ToArray()});
  },"加入组网");
  dialog.Body.Children.Add(Ui.Labeled("搜索在线设备",search));dialog.Body.Children.Add(selected);
  void Update(){var old=(anchor.SelectedItem as ComboBoxItem)?.Tag as string;anchor.Items.Clear();foreach(var c in choices.Where(c=>c.Box.IsChecked==true)){var item=new ComboBoxItem{Content=c.Box.Content,Tag=c.Id};anchor.Items.Add(item);if(c.Id==old)anchor.SelectedItem=item;}if(anchor.SelectedItem is null&&anchor.Items.Count>0)anchor.SelectedIndex=0;selected.Text=$"已选择 {choices.Count(c=>c.Box.IsChecked==true)} / {choices.Count} 台";}
  dialog.Body.Children.Add(Ui.Bar(Ui.Button("全选",()=>{foreach(var c in choices.Where(c=>c.Box.Visibility==Visibility.Visible))c.Box.IsChecked=true;}),Ui.Button("清空",()=>{foreach(var c in choices)c.Box.IsChecked=false;})));
  var list=new StackPanel();dialog.Body.Children.Add(new ScrollViewer{Content=list,MaxHeight=240,VerticalScrollBarVisibility=ScrollBarVisibility.Auto});
  foreach(var device in devices){var c=new CheckBox{Content=$"{device.Name} · {device.Id}",Margin=new(0,7,0,7)};choices.Add((device.Id,c));list.Children.Add(c);c.Checked+=(_,_)=>Update();c.Unchecked+=(_,_)=>Update();}
  search.TextChanged+=(_,_)=>{foreach(var c in choices)c.Box.Visibility=c.Box.Content.ToString()!.Contains(search.Text,StringComparison.OrdinalIgnoreCase)?Visibility.Visible:Visibility.Collapsed;};
  if(needsAnchor){dialog.Body.Children.Add(Ui.Labeled("固定地址成员（其余成员 DHCP）",anchor));var address=System.Net.IPAddress.Parse(network.Cidr.Split('/')[0]).GetAddressBytes();address[3]++;ip=dialog.Input("固定虚拟 IP",new System.Net.IPAddress(address).ToString());}
  Update();dialog.ShowDialog();
 }
 internal static void Member(Window owner,OverlayMember member,string name,Func<MemberNetworkConfig,Task<bool>> save,string[]? legacyRoutes=null)
 {
  var cfg=member.Config??MemberNetworkConfig.Default(name,member.VirtualIp) with {SystemForward=false,Routes=legacyRoutes??[]};
  TextBox hostname=null!,ip=null!,proxies=null!,routes=null!;CheckBox system=null!,lazy=null!,need=null!,only=null!,disable=null!,manual=null!;
  var dialog=new NetworkDialog(owner,"成员配置 · "+name,()=>save(new(hostname.Text.Trim(),ip.Text.Trim(),system.IsChecked==true,lazy.IsChecked==true,need.IsChecked==true,only.IsChecked==true,disable.IsChecked==true,Lines(proxies),manual.IsChecked==true,manual.IsChecked==true?Lines(routes):[])));
  if(member.Config is null)dialog.Body.Children.Add(Ui.Note("旧成员：保存时将采用新版 et0、私有模式、禁用对称打洞/UPnP 和单线程默认参数。"));
  dialog.Body.Children.Add(Ui.Text(member.Desired=="start"?"保存后重建此成员的组网实例，连接会短暂中断。":"成员已停止；仅保存配置，不启动实例。",true));
  hostname=dialog.Input("主机名",cfg.Hostname);ip=dialog.Input("虚拟 IP（留空使用 DHCP）",cfg.VirtualIp);
  system=dialog.Check("系统转发",cfg.SystemForward);lazy=dialog.Check("按需直连（Lazy P2P）",cfg.LazyP2p);need=dialog.Check("优先直连本机（Need P2P）",cfg.NeedP2p);only=dialog.Check("仅允许直连（P2P Only）",cfg.P2pOnly);disable=dialog.Check("关闭主动直连（Disable P2P）",cfg.DisableP2p);
  proxies=dialog.Input("子网代理（CIDR，每行一项）",string.Join("\n",cfg.ProxyCidrs),true);
  manual=dialog.Check("自定义路由",cfg.EnableManualRoutes);routes=dialog.Input("接收的路由网段（CIDR，每行一项）",string.Join("\n",cfg.Routes),true);routes.IsEnabled=cfg.EnableManualRoutes;manual.Checked+=(_,_)=>routes.IsEnabled=true;manual.Unchecked+=(_,_)=>routes.IsEnabled=false;
  dialog.Body.Children.Add(Ui.Text("开启且留空：不自动接收其他成员宣告的子网路由；关闭：恢复引擎自动路由。",true));dialog.ShowDialog();
 }
}
