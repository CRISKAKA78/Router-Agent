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
   Check(view.Details.Items.Cast<PropertyRow>().Any(r=>r.Key=="cellular_imei_value"&&r.Value=="867123456789012"),"AT view exposes validated IMEI and exact query");
   var imei=view.Details.Items.Cast<PropertyRow>().Single(r=>r.Key=="cellular_imei_value");view.Details.SelectedItem=imei;view.Update();Check((view.Details.SelectedItem as PropertyRow)?.Key==imei.Key,"AT refresh preserves field selection");
   foreach(var theme in new[]{"Light","Dark"}){Theme.Apply(theme);window.UpdateLayout();Render(window,"cellular-"+theme.ToLowerInvariant()+".png");}
   current=current with{Cellular=current.Cellular! with{Ports=[port with{Path="/dev/ttyUSB9"}]}};view.Update();Check(view.Details.Items.Cast<PropertyRow>().Any(r=>r.Key=="cellular_path"&&r.Value=="/dev/ttyUSB9"),"AT renamed port retains physical device selection");
   current=current with{Cellular=current.Cellular! with{Stale=true}};view.Update();Check(Visuals<TextBlock>(view).Any(t=>t.Text.Contains("已过期")),"AT stale data never appears current");
   current=current with{Cellular=current.Cellular! with{Status="unavailable",Ports=[port with{Selected=false,Status="busy",Reason="port_busy",ATI=new("ATI","","not_queried"),IMEI=new("","","not_queried")}]}};view.Update();Check(view.Details.Items.Cast<PropertyRow>().Any(r=>r.Key=="cellular_reason"&&r.Value.Contains("其他程序")),"AT occupied port has actionable state");
   window.Width=720;window.Height=500;await Task.Delay(40);window.UpdateLayout();Render(window,"cellular-narrow-busy.png");
   current=current with{Cellular=null,CellularConfiguration=null};view.Update();Check(view.Details.Items.Count==0&&Visuals<TextBlock>(view).Any(t=>t.Text.Contains("未启用")),"AT disabled config clears identity display");
   current=null;view.Update();Check(view.Details.Items.Count==0,"AT device switch clears previous results");
   var template=new ProbeTemplate("at","AT",1,null,[],CellularProbe:new());Check(template.NeighborCapabilityError([])?.Contains("cellular_identity_v1")==true&&template.NeighborCapabilityError(["cellular_identity_v1"])==null,"AT template application capability gating");
   var options=new JsonSerializerOptions{PropertyNamingPolicy=JsonNamingPolicy.SnakeCaseLower};var text=JsonSerializer.Serialize(port,options);Check(JsonSerializer.Deserialize<CellularPort>(text,options)?.IMEI.Value==port.IMEI.Value,"AT public DTO snake-case round trip");
  }finally{window.Close();Theme.Apply("Light");main.Activate();}
 }
}
