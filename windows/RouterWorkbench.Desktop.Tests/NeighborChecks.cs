using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;
namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
 private static async Task NeighborChecks(MainWindow main)
 {
  var source=((DataGrid)main.FindName("DevicesGrid")).SelectedItem as Device ?? throw new Exception("device required");
  var row=new NeighborRow("192.0.2.2","02:00:00:00:00:02","LAN2","测试设备","arp+fdb","cached");
  var configs=new[]{new NeighborDomainConfig("lan","lan","br0",["LAN1","LAN2"]),new NeighborDomainConfig("local","broadcast","br0")};
  Device? current=source with {Registration=source.Registration with {Capabilities=["neighbors_v1"]},NeighborDomains=configs,Neighbors=new(1,30,[new("lan","lan","br0","ok","",false,[row]),new("local","broadcast","br0","ok","",false,[row,row with {Ip="",Mac="02:00:00:00:00:03",State="mac_only"}])],[],false,DateTimeOffset.Now,false)};
  var lan=new NeighborView("lan",()=>null,()=>current);var local=new NeighborView("broadcast",()=>null,()=>current);
  var tabs=new TabControl{Items={new TabItem{Header="LAN 下接设备",Content=lan},new TabItem{Header="本机广播域设备",Content=local}}};
  var window=new Window{Title="邻居发现验证",Width=1120,Height=650,Content=tabs};window.SetResourceReference(Control.BackgroundProperty,"Background");window.Show();
  try{
   lan.Update();local.Update();await Task.Delay(80);window.UpdateLayout();
   Check(lan.Entries.Items.Count==1&&local.Entries.Items.Count==2,"LAN and broadcast lists overlap without classifying an uplink");
   Check(configs[1].ToString()=="local · br0","domain selector shows domain and interface instead of DTO members");
   Check(row.StateText=="缓存记录"&&((NeighborRow)local.Entries.Items[1]).IpText=="未知","cached and MAC-only records never pretend online or invent IP");
   var selected=lan.Entries.Items[0];lan.Entries.SelectedItem=selected;lan.Update();Check(ReferenceEquals(selected,lan.Entries.SelectedItem),"unchanged neighbor refresh preserves selection and row identity");
   foreach(var theme in new[]{"Light","Dark"}){Theme.Apply(theme);tabs.SelectedIndex=0;window.UpdateLayout();Render(window,"neighbors-lan-"+theme.ToLowerInvariant()+".png");tabs.SelectedIndex=1;window.UpdateLayout();Render(window,"neighbors-broadcast-"+theme.ToLowerInvariant()+".png");}
   window.Width=760;window.Height=520;await Task.Delay(50);window.UpdateLayout();Render(window,"neighbors-narrow.png");
   current=current with {Neighbors=current.Neighbors! with {Stale=true}};local.Update();Check(Visuals<TextBlock>(local).Any(t=>t.Text.Contains("已过期")),"stale neighbor snapshot is explicit");
   current=null;lan.Update();local.Update();Check(lan.Entries.Items.Count==0&&local.Entries.Items.Count==0,"device change clears both neighbor lists");
  }finally{window.Close();Theme.Apply("Light");main.Activate();}
 }
}
