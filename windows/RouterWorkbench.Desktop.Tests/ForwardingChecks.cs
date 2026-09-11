using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;
namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
 private static async Task ForwardingUiChecks(MainWindow window)
 {
  var devices=(DataGrid)window.FindName("DevicesGrid");var d=(Device)devices.SelectedItem;var originalWidth=window.Width;var originalHeight=window.Height;
  foreach(var kind in new[]{"lan","serial"}){
   Invoke(window,"Navigate",kind=="lan"?"forwarding":"serial");await Task.Delay(250);
   var view=Field<ForwardingView>(window,kind=="lan"?"forwardLan":"forwardSerial");
   var result=JsonSerializer.SerializeToElement(new{mapping=new{id="visual-fixture",device_id=d.DeviceId,kind,protocol=kind=="lan"?"udp":"tcp",@interface="br0",target_ip="192.168.5.100",target_port=502,serial="/dev/ttyUSB0",baud=115200,data_bits=8,stop_bits=1,parity="none",lease_minutes=0,session_id=d.CurrentSession!.SessionId,state="active",reason="",host="example.invalid",port=22000,source_ip="192.168.5.222",created_at=DateTimeOffset.UtcNow,released=false,device_released=false},registration=kind=="serial"?"AUTH "+new string('a',64)+"\r\n":""});
   view.Accept(result);Check(Field<TextBlock>(view,"message").Text.Contains("实时状态"),kind+" creation acknowledgement does not override live mapping state");window.UpdateLayout();var rows=Field<DataGrid>(view,"rows");var row=rows.SelectedItem as ForwardMapping;
   Check(row is {CanConnect:true,LeaseMinutes:0}&&row.SourceIp=="192.168.5.222",kind+" public DTO, proxy source and unlimited lease rendered");
   Check(!Field<Button>(view,"create").IsEnabled,"old Probe cannot enable "+kind+" creation");
   if(kind=="serial"){Check(Field<TextBox>(view,"registration").Text.EndsWith("\r\n"),"registration copy value contains real CRLF");Check(!Visuals<ComboBox>(view).Any(x=>x.IsVisible&&x.Items.Cast<object>().Any(i=>i.ToString()=="udp")),"serial UI never offers UDP");}
   foreach(var theme in new[]{"Light","Dark"}){Theme.Apply(theme);window.UpdateLayout();Render(window,"forwarding-"+kind+"-"+theme.ToLowerInvariant()+".png");}
   window.Width=960;window.Height=640;await Task.Delay(100);window.UpdateLayout();
   Render(window,"forwarding-"+kind+"-narrow.png");Check(rows.ActualHeight>=60,kind+" narrow view retains the mappings table");window.Width=originalWidth;window.Height=originalHeight;
   devices.SelectedIndex=(devices.SelectedIndex+1)%devices.Items.Count;window.UpdateLayout();view.Update();Check(Field<TextBox>(view,"registration").Text=="","switching device clears "+kind+" registration secret");devices.SelectedItem=d;view.Update();
  }
  Theme.Apply("Light");Invoke(window,"Navigate","overview");
 }
}
