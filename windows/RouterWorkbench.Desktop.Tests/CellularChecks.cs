using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;
namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
 private static async Task CellularChecks(MainWindow main)
 {
  var source=((DataGrid)main.FindName("DevicesGrid")).SelectedItem as Device??throw new Exception("device required");
  var port=new CellularPort("/dev/ttyUSB2","/sys/devices/platform/usb1/1-1","ok","",true,0,new("ATI","Generic modem\nRevision 1.0","ok"),new("AT+CGSN","867123456789012","ok"),DateTimeOffset.Now);
  Device? current=source with{Registration=source.Registration with{Capabilities=["cellular_identity_v1"]},CellularConfiguration=new(30),Cellular=new(1,30,"ok","",false,[port],DateTimeOffset.Now,false)};
  var view=new CellularView(()=>current,()=>{});view.Update();
  var tabs=new TabControl{Style=(Style)Application.Current.FindResource("InspectorPages"),Items={new TabItem{Header="蜂窝模块",Tag="cellular",Content=view}}};
  tabs.SelectedIndex=0;var workspace=new PropertyInspectorWorkspace(tabs);workspace.SetPage(view.Details,view.Sections);
  view.RowsChanged+=()=>{workspace.SetPage(view.Details,view.Sections);workspace.RowsUpdated(view.Sections,false);};
  var window=new Window{Title="AT自动探测验证",Width=1050,Height=700,Content=workspace};window.SetResourceReference(Control.BackgroundProperty,"Surface");window.Show();
  try{
   window.UpdateLayout();Check(view.IsVisible&&workspace.PageSelector.SelectedIndex==0,"AT page is actually visible in the inspector");
   Check(view.Details.Items.Cast<PropertyRow>().Any(r=>r.Key=="cellular_imei_value"&&r.Value=="867123456789012"),"AT view exposes validated IMEI without diagnostic rows");
   var imei=view.Details.Items.Cast<PropertyRow>().Single(r=>r.Key=="cellular_imei_value");view.Details.SelectedItem=imei;view.Update();Check((view.Details.SelectedItem as PropertyRow)?.Key==imei.Key,"AT refresh preserves field selection");
   Check(view.Ports.Visibility==Visibility.Collapsed&&view.PortStatus.Text=="端口已自动选择","automatic port hides selector");
   Check(!view.Details.Items.Cast<PropertyRow>().Any(r=>r.Key.Contains("status")||r.Key.Contains("command")||r.Key.Contains("selected")),"diagnostic rows removed");
   var now=DateTimeOffset.Now;
   var fields=Enumerable.Range(0,35).Select(i=>new CellularField("test"+i,"value "+i)).ToArray();
   var signals=new[]{new CellularSignal("rsrp","LTE",-100,-140,-44,"dBm","≈"),new CellularSignal("sinr","NR",10,-23,40,"dB","≈"),new CellularSignal("rsrq","LTE",-10,-19.5,-3,"dB","≈")};
   port=port with{Fields=fields,Signals=signals,SampledAt=now};current=current with{Cellular=current.Cellular! with{Ports=[port]}};view.Update();window.UpdateLayout();
   var scroll=Visuals<ScrollViewer>(view.Details).First();scroll.ScrollToVerticalOffset(250);window.UpdateLayout();await Task.Delay(60);
   var offset=scroll.VerticalOffset;var rowSource=view.Details.ItemsSource;Check(offset>0,"AT table scrolled before fast refresh");
   for(int i=1;i<=20;i++){port=port with{SampledAt=now.AddSeconds(i),Signals=signals.Select(x=>x with{Value=x.Value+i*.1}).ToArray()};current=current with{Cellular=current.Cellular! with{Ports=[port]}};view.Update();workspace.RowsUpdated(view.Sections,false);window.UpdateLayout();}
   await Task.Delay(60);Check(ReferenceEquals(rowSource,view.Details.ItemsSource)&&Math.Abs(scroll.VerticalOffset-offset)<2,"fast refresh retains row instances and scroll offset");
   Check(view.Trends.Select(t=>t.Metric).SequenceEqual(new[]{"rsrp","sinr","rsrq"}),"charts ordered RSRP SINR RSRQ");
   Check(view.Trends[0].Samples.Count==21&&view.Trends[0].Signal!.Minimum==-140&&view.Trends[0].Signal!.Maximum==-44,"charts use unique samples and fixed manual range");
   view.Update();Check(view.Trends[0].Samples.Count==21,"HTTP refresh never duplicates signal points");
   workspace.Search.Text="test";window.UpdateLayout();scroll.ScrollToVerticalOffset(200);window.UpdateLayout();await Task.Delay(60);var filteredOffset=scroll.VerticalOffset;
   for(int i=0;i<5;i++){view.Update();workspace.RowsUpdated(view.Sections,false);window.UpdateLayout();await Task.Delay(20);}
   Check(filteredOffset>0&&Math.Abs(scroll.VerticalOffset-filteredOffset)<2,"filtered AT refresh preserves scroll anchor");workspace.Search.Clear();window.UpdateLayout();
   port=port with{SampledAt=now.AddSeconds(21),Signals=[]};current=current with{Cellular=current.Cellular! with{Ports=[port]}};view.Update();Check(view.Trends.All(t=>t.Samples.Last().Value==null),"missing signal creates gap instead of zero");
   port=port with{Fields=[new("manufacturer","Fibocom Wireless Inc."),new("model","FM160-CN"),new("firmware","89641.1000.00.01.04.18"),new("sim","就绪"),new("operator","46001"),new("access","LTE-NR 双连接")],Signals=signals,SampledAt=now.AddSeconds(22)};
   current=current with{Cellular=current.Cellular! with{Ports=[port]}};view.Update();scroll.ScrollToTop();window.UpdateLayout();
   foreach(var theme in new[]{"Light","Dark"}){Theme.Apply(theme);view.Update();window.UpdateLayout();Render(window,"cellular-"+theme.ToLowerInvariant()+".png");}
   var detailedPath=Environment.GetEnvironmentVariable("RMP_CELLULAR_DETAIL_SAMPLE");
   if(!string.IsNullOrEmpty(detailedPath)){
    var opts=new JsonSerializerOptions{PropertyNamingPolicy=JsonNamingPolicy.SnakeCaseLower,PropertyNameCaseInsensitive=true};
    port=JsonSerializer.Deserialize<CellularPort>(System.IO.File.ReadAllText(detailedPath),opts)??throw new Exception("missing detailed fixture");
    port=port with{Selected=true,SampledAt=now.AddSeconds(23)};
    current=current with{Cellular=current.Cellular! with{Ports=[port],Details=true}};view.Update();window.UpdateLayout();
    Check(port.Fields!.Length>100&&view.Details.Items.Count>100,"FM160 detailed normalized fixture is rendered without dropping queried fields");
    foreach(var key in new[]{"lock_band","lock_frequency","lock_cell"})Check(view.Details.Items.Cast<PropertyRow>().Single(r=>r.Key=="cellular_"+key).Value=="否","unlocked field displays 否: "+key);
    Check(view.Details.Items.Cast<PropertyRow>().Any(r=>r.Name=="锁频段"&&r.Group=="锁定配置"),"server supplied detailed names and groups are preserved");
    workspace.Search.Text="锁";window.UpdateLayout();
    foreach(var theme in new[]{"Light","Dark"}){Theme.Apply(theme);window.UpdateLayout();Render(window,"cellular-details-lock-"+theme.ToLowerInvariant()+".png");}
    workspace.Search.Clear();window.UpdateLayout();scroll=Visuals<ScrollViewer>(view.Details).First();scroll.ScrollToVerticalOffset(500);window.UpdateLayout();await Task.Delay(60);
    var detailOffset=scroll.VerticalOffset;var detailRows=view.Details.ItemsSource;
    port=port with{Fields=port.Fields.Select(f=>f.Key switch{"lock_band"=>f with{Value="n78"},"lock_frequency"=>f with{Value="NR-ARFCN 627264"},"lock_cell"=>f with{Value="NR-ARFCN 627264，PCI 198"},_=>f}).ToArray(),SampledAt=now.AddSeconds(24)};
    current=current with{Cellular=current.Cellular! with{Ports=[port]}};view.Update();workspace.RowsUpdated(view.Sections,false);window.UpdateLayout();await Task.Delay(60);
    Check(detailOffset>0&&ReferenceEquals(detailRows,view.Details.ItemsSource)&&Math.Abs(scroll.VerticalOffset-detailOffset)<2,"detailed field refresh preserves long-page scroll");
    Check(view.Details.Items.Cast<PropertyRow>().Single(r=>r.Key=="cellular_lock_cell").Value=="NR-ARFCN 627264，PCI 198","locked cell shows actual configured value");
    workspace.Search.Text="锁";window.UpdateLayout();Render(window,"cellular-details-locked.png");workspace.Search.Clear();scroll.ScrollToTop();window.UpdateLayout();Render(window,"cellular-details-overview.png");
   }
   current=current with{Cellular=current.Cellular! with{Ports=[port with{Path="/dev/ttyUSB9"}]}};view.Update();Check(view.Details.Items.Cast<PropertyRow>().Any(r=>r.Key=="cellular_path"&&r.Value=="/dev/ttyUSB9"),"AT renamed port retains physical device selection");
   current=current with{Cellular=current.Cellular! with{Stale=true}};view.Update();Check(Visuals<TextBlock>(view).Any(t=>t.Text.Contains("已过期")),"AT stale data never appears current");
   current=current with{Cellular=current.Cellular! with{Status="unavailable",Ports=[port with{Selected=false,Status="busy",Reason="port_busy",ATI=new("ATI","","not_queried"),IMEI=new("","","not_queried")}]}};view.Update();Check(view.Ports.Visibility==Visibility.Visible&&Visuals<TextBlock>(view).Any(t=>t.Text=="端口已占用"),"no auto selection exposes candidate port with brief busy state");
   window.Width=720;window.Height=500;await Task.Delay(40);window.UpdateLayout();Render(window,"cellular-narrow-busy.png");
   for(int i=0;i<200;i++)view.Trends[0].Observe("bounded-fixture",now.AddSeconds(i),signals[0],false,10);
   Check(view.Trends[0].Samples.Count==180,"signal history is bounded to 180 points");
   current=current with{Cellular=null,CellularConfiguration=null};view.Update();Check(view.Details.Items.Count==0&&Visuals<TextBlock>(view).Any(t=>t.Text.Contains("未启用")),"AT disabled config clears identity display");
   current=null;view.Update();Check(view.Details.Items.Count==0&&view.Trends.All(t=>t.Samples.Count==0),"AT device switch clears previous results and chart history");
   var template=new ProbeTemplate("at","AT",1,null,[],CellularProbe:new());Check(template.NeighborCapabilityError([])?.Contains("cellular_identity_v1")==true&&template.NeighborCapabilityError(["cellular_identity_v1"])==null,"AT template application capability gating");
   Check((template with{CellularProbe=new(30,true,true)}).NeighborCapabilityError(["cellular_identity_v1","cellular_telemetry_v2"])?.Contains("cellular_details_v1")==true,"details require advertised Probe capability");
   Check((template with{CellularProbe=new(30,true)}).NeighborCapabilityError(["cellular_identity_v1"])?.Contains("cellular_telemetry_v2")==true,"telemetry template rejects old Probe before apply");
   var options=new JsonSerializerOptions{PropertyNamingPolicy=JsonNamingPolicy.SnakeCaseLower};var text=JsonSerializer.Serialize(port,options);Check(JsonSerializer.Deserialize<CellularPort>(text,options)?.IMEI.Value==port.IMEI.Value,"AT public DTO snake-case round trip");
  }finally{window.Close();Theme.Apply("Light");main.Activate();}
 }
}
