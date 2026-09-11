using System.Text;
using System.Text.Json;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;
namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
 private static async Task DeviceLogClientChecks(){
  var b=new DeviceLogBuffer(12);var bytes="中文"u8.ToArray();b.Add(new("ok","1:2",0,2,false,bytes[..2]));b.Add(new("ok","1:2",2,6,false,bytes[2..]));Check(b.Text=="中文","log buffer preserves split UTF8 bytes");
  var original=b.Bytes();b.ClearDisplay();Check(b.Offset==6&&b.Bytes().Length==0,"clear live display does not rewind remote cursor");b.Add(new("ok","3:4",0,3,true,"new"u8.ToArray()));Check(b.Gaps==1,"rotation records a visible log gap");b.Add(new("ok","3:4",3,15,false,Encoding.UTF8.GetBytes("abcdefghijkl")));Check(b.Trimmed&&b.Bytes().Length<=12,"live cache is bounded");b.Reset();Check(b.Offset==0&&b.Gaps==0,"new device resets cursor and gap state");
  var attempts=new List<(string Key,byte[] Bytes)>();int posts=0;
  using var handler=new DelegateHandler(async(request,token)=>{
   if(request.Method==System.Net.Http.HttpMethod.Post){attempts.Add((request.Headers.GetValues("Idempotency-Key").Single(),await request.Content!.ReadAsByteArrayAsync(token)));if(++posts==1)throw new System.Net.Http.HttpRequestException("lost commit reply");return Json("{\"data\":{\"task_id\":\"original\"}}");}
   return Json("{\"data\":{\"task_id\":\"original\",\"device_id\":\"router\",\"type\":\"device_logs\",\"state\":\"success\",\"created_at\":\"2026-09-11T00:00:00Z\",\"command\":\"\",\"cwd\":\"\",\"timeout_seconds\":30,\"dispatch_count\":1,\"result\":{\"status\":\"success\",\"exit_code\":0,\"stdout\":\"{}\",\"stderr\":\"\",\"truncated\":false}}}");
  });
  await using var owner=new WorkspaceConnection(new Uri("http://localhost:18080"),handler);using var command=new DeviceLogClient(owner,"router","session").Command(new(){["action"]="enable_live"});
  try{await command.RunAsync(default);throw new Exception("expected uncertain result");}catch(System.Net.Http.HttpRequestException){}
  Check(owner.Pending==command.Request,"uncertain persistent log enable retains its original mutation");await command.RunAsync(default);
  Check(command.TaskId=="original"&&attempts.Count==2&&attempts[0].Key==attempts[1].Key&&attempts[0].Bytes.SequenceEqual(attempts[1].Bytes),"log enable explicit retry keeps original key bytes and task");
  await command.RunAsync(default);Check(posts==2,"completed log enable is not silently recommitted");
 }
 private static async Task DeviceLogWorkspaceChecks(MainWindow window,WorkspaceConnection owner,TestProbe peer){
  var devices=(DataGrid)window.FindName("DevicesGrid");devices.SelectedItem=devices.Items.Cast<Device>().Single(d=>d.DeviceId=="desktop-router-02");Invoke(window,"Navigate","logs");
  var view=Field<LogWorkspace>(window,"logWorkspace");
  await Eventually(()=>Task.FromResult(peer.LogEnables==1&&Field<TextBox>(view,"live").Text.Contains("AT+CSQ")),"opening live logs sends persistent enable and renders real API batch");
  Check(peer.LogCommits==1,"live log enable requests one commit from test peer");Render(window,"device-logs-live.png");
  var tabs=Field<TabControl>(view,"pages");tabs.SelectedIndex=1;
  await Eventually(()=>Task.FromResult(Field<DataGrid>(view,"files").Items.Count==1),"history discovery works while history switch is disabled");
  await Task.Delay(150);var reads=peer.LogReads;await Task.Delay(1100);Check(peer.LogReads==reads&&peer.LogCommits==1,"leaving live tab stops reads without restoring persistent settings");Render(window,"device-logs-history.png");
  var device=owner.Snapshot.Devices.Single(d=>d.DeviceId=="desktop-router-02");var client=new DeviceLogClient(owner,device.DeviceId,device.CurrentSession!.SessionId);
  var file=Field<DataGrid>(view,"files").Items.Cast<DeviceLogFile>().Single();var outputFile=Path.Combine(output,"history-log.txt");
  using var export=new DeviceLogExport(client,file,outputFile,true);await export.RunAsync(default);
  Check(export.Complete&&await File.ReadAllTextAsync(outputFile)=="first AT+CSQ\nsecond 中文\n","snapshot File API and multi-member gzip text export complete end to end");
  Check(peer.LogSnapshots==1&&peer.LogReleases==1,"one export creates and releases its immutable snapshot");await export.RunAsync(default);Check(peer.LogSnapshots==1,"completed export does not create a replacement snapshot");
  var original=await owner.Api.GetAsync<DeviceLogPreview>($"log-assets/{export.Asset!.AssetId}/preview");Check(original.Text=="first AT+CSQ\nsecond 中文\n"&&original.ParserStatus=="awaiting_vendor_samples","analysis retains raw text and honestly reports parser unavailable");
  using(var setting=client.Command(new(){["action"]="history_settings",["enabled"]="1",["interval"]="600",["persist"]="0"})){await setting.RunAsync(default);}
  var configured=await client.ReadAsync<DeviceLogSettings>("status",default);Check(configured.LogSaveEn=="1"&&configured.LogSaveItv=="600"&&peer.LogCommits==1,"history runtime settings are independent and do not implicitly commit");
  using(var setting=client.Command(new(){["action"]="history_settings",["enabled"]="0",["interval"]="300",["persist"]="1"})){await setting.RunAsync(default);}
  configured=await client.ReadAsync<DeviceLogSettings>("status",default);Check(configured.LogSaveEn=="0"&&configured.SyslogdEnable=="3"&&peer.LogCommits==2,"history disable persists only on explicit request and leaves realtime enabled");
  view.GetType().GetField("lastAsset",System.Reflection.BindingFlags.NonPublic|System.Reflection.BindingFlags.Instance)!.SetValue(view,export.Asset);
  await InvokeAsync(view,"PreviewAsset",client,CancellationToken.None);Check(Field<TextBox>(view,"preview").Text.Contains("second 中文"),"archive preview remains visible after automatic analysis tab navigation");
  Render(window,"device-logs-analysis.png");Invoke(window,"Navigate","overview");
 }
}
