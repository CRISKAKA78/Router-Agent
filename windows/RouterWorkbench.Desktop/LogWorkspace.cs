using System.IO;
using System.Text;
using System.Text.Json;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using Microsoft.Win32;
using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;
internal sealed class LogWorkspace : UserControl
{
 private readonly Func<WorkspaceConnection?> getConnection;
 private readonly Func<Device?> getDevice;
 private readonly TabControl pages=new();
 private readonly TextBox live=Ui.Code("",true),preview=Ui.Code("",true),search=Ui.Input("",180),directory=Ui.Input("",300),interval=Ui.Input("300",90);
 private readonly TextBlock status=Ui.Text("选择支持日志功能的在线设备。",true),historyStatus=Ui.Text("",true),previewStatus=Ui.Text("厂商AT解析器待样本确认；当前仅提供原文查看，不生成基站结论。",true);
 private readonly CheckBox follow=new(){Content="跟随滚动",IsChecked=true},enabled=new(){Content="历史保存"},persist=new(){Content="保存到设备配置（commit）"},asText=new(){Content="解压为TXT"};
 private readonly DataGrid files=Ui.Table("历史日志",("文件名","Name",260),("类型","KindText",150),("大小","SizeText",110),("修改时间","ModifiedText",190),("设备路径","Path",-1));
 private readonly DeviceLogBuffer buffer=new();
 private CancellationTokenSource cancel=new();
 private Task loop=Task.CompletedTask,action=Task.CompletedTask;
 private string scope="",assetContext="";
 private bool visible,paused,busy,ready;
 private DeviceLogClient? client;
 private DeviceLogCommand? liveEnable,lastCommand;
 private DeviceLogExport? exporting;
 private Asset? lastAsset;
 private string error="";
 public LogWorkspace(Func<WorkspaceConnection?> connection,Func<Device?> device){
  getConnection=connection;getDevice=device;
  status.TextWrapping=historyStatus.TextWrapping=previewStatus.TextWrapping=TextWrapping.Wrap;
  AutomationProperties.SetName(live,"设备实时日志");AutomationProperties.SetName(preview,"日志原文预览");AutomationProperties.SetName(search,"日志关键词");AutomationProperties.SetName(directory,"自定义历史日志目录");AutomationProperties.SetName(interval,"历史保存间隔秒");
  search.TextChanged+=(_,_)=>RenderLive();
  var liveButtons=Ui.Bar(Ui.Button("开始 / 继续",()=>RestartLive()),Ui.Button("停止采集",StopLive),Ui.Button("暂停 / 继续显示",()=>{paused=!paused;if(!paused)RenderLive();}),follow,Ui.Text("关键词"),search,Ui.Button("清空显示",()=>{buffer.ClearDisplay();RenderLive();}),Ui.Button("导出已采集文本",()=>_=SaveLive()));
  var notice=Ui.Text("打开实时日志将由探针开启 debuglog_enable=1、syslogd_enable=3 并自动 commit。关闭页面不关闭开关；commit 会提交整份已暂存的 NVRAM。",true);notice.TextWrapping=TextWrapping.Wrap;notice.Margin=new(8);
  var realtime=Ui.Page(new StackPanel{Children={notice,liveButtons,status}},live);
  files.SelectionMode=DataGridSelectionMode.Extended;
  var filenameStyle=new Style(typeof(TextBlock),Ui.CellTextStyle());filenameStyle.Setters.Add(new Setter(TextBlock.TextAlignmentProperty,TextAlignment.Left));((DataGridTextColumn)files.Columns[0]).ElementStyle=filenameStyle;
  var controls=Ui.Bar(Ui.Text("目录（留空自动发现）"),directory,Ui.Button("刷新",()=>StartAction(RefreshHistory)),enabled,Ui.Text("间隔秒"),interval,persist,Ui.Button("应用历史设置",()=>StartAction(ApplyHistory)));
  var fileActions=Ui.Bar(asText,Ui.Button("导出选中",()=>StartAction(ExportSelected)),Ui.Button("继续原导出",()=>StartAction(ContinueExport)),Ui.Button("查看已导出原文",()=>StartAction(PreviewAsset)),Ui.Button("继续原设置",()=>StartAction(ContinueCommand)));
  var history=Ui.Page(new StackPanel{Children={controls,fileActions,historyStatus}},files);
  var analysis=Ui.Page(new StackPanel{Children={previewStatus,Ui.Bar(Ui.Button("导入本地日志查看",()=>StartAction(ImportPreview)),Ui.Button("查看上次导出的日志",()=>StartAction(PreviewAsset)))}},preview);
  pages.Items.Add(new TabItem{Header="实时日志",Content=realtime});pages.Items.Add(new TabItem{Header="历史日志",Content=history});pages.Items.Add(new TabItem{Header="分析 / 原文",Content=analysis});
  pages.SelectionChanged+=(_,e)=>{if(e.Source==pages)Update(visible);};Content=new Border{Padding=new(12),Child=pages};
 }
 public void Update(bool active){
  visible=active;var owner=getConnection();var device=getDevice();var session=device?.CurrentSession;
  var available=owner is {Synchronized:true}&&device is {Online:true,Managed:true}&&session!=null;
  var analysisAvailable=owner is {Synchronized:true}&&device is {Managed:true}&&pages.SelectedIndex==2;
  var context=owner==null||device==null?"":$"{owner.Api.Origin}|{device.DeviceId}";
  if(context!=assetContext){lastAsset=null;assetContext=context;preview.Clear();}
  var supported=session?.Registration.Capabilities.Contains("device_logs_v1")==true;
  var next=(available&&supported||analysisAvailable)&&active?$"{owner!.Api.Origin}|{device!.DeviceId}|{session?.SessionId??device?.LatestSession?.SessionId??"offline"}|{pages.SelectedIndex}":"";
  if(next==scope)return;
  ready=false;files.ItemsSource=null;
  scope=next;cancel.Cancel();var previous=loop;var oldAction=action;var oldCancel=cancel;cancel=new();var token=cancel.Token;
  client=available&&supported||analysisAvailable?new(owner!,device!.DeviceId,session?.SessionId??device!.LatestSession?.SessionId??"offline"):null;
  var nextClient=client;loop=Transition(previous,oldAction,oldCancel,token,next,nextClient);
  if(next=="")status.Text=supported?"采集已停止；设备日志开关保持持久开启。":"当前设备未提供 device_logs_v1 能力，请更新Probe。";
 }
 private async Task Transition(Task previous,Task previousAction,CancellationTokenSource old,CancellationToken token,string requested,DeviceLogClient? c){
  try{await previous;await previousAction;}catch(OperationCanceledException){}finally{old.Dispose();}
  if(token.IsCancellationRequested||requested==""||c==null)return;
  ready=true;
  if(exporting!=null&&!Same(exporting.Client,c)){exporting.Dispose();exporting=null;}
  if(lastCommand!=null&&!Same(lastCommand.Client,c)){lastCommand.Dispose();lastCommand=null;}
  if(liveEnable!=null&&(!Same(liveEnable.Client,c)||liveEnable.Result!=null)){liveEnable.Dispose();liveEnable=null;}buffer.Reset();RenderLive();error="";
  try{if(pages.SelectedIndex==0){liveEnable??=c.Command(new(){["action"]="enable_live"});status.Text="正在开启日志并持久化配置…";await liveEnable.RunAsync(token);await Poll(c,token);}else if(pages.SelectedIndex==1)await RefreshHistory(c,token);}
  catch(OperationCanceledException){}catch(Exception e){if(!token.IsCancellationRequested){error=e.Message;status.Text=historyStatus.Text=error;}}
 }
 private async Task Poll(DeviceLogClient c,CancellationToken token){
  while(!token.IsCancellationRequested){
   try{var batch=await c.ReadAsync<DeviceLogBatch>("live",token,generation:buffer.Generation,offset:buffer.Offset);token.ThrowIfCancellationRequested();buffer.Add(batch);if(!paused)RenderLive();status.Text=batch.State=="waiting_for_file"?"开关已设置，等待固件产生日志文件…":$"实时读取中 · 缺段 {buffer.Gaps}"+(buffer.Trimmed?" · 仅保留最近512KiB（首次从文件尾部读取）":"");}
   catch(ApiException e) when(e.Code=="log_busy"){}await Task.Delay(1000,token);
  }
 }
 private void RenderLive(){var text=buffer.Text;if(search.Text.Length>0)text=string.Join('\n',text.Split('\n').Where(x=>x.Contains(search.Text,StringComparison.OrdinalIgnoreCase)));live.Text=text;if(follow.IsChecked==true)live.ScrollToEnd();}
 private void StopLive(){cancel.Cancel();status.Text="采集已停止，设备开关保持持久开启。";}
 private void RestartLive(){
  if(!visible||pages.SelectedIndex!=0||client==null)return;
  if(!loop.IsCompleted)return;
  // A known/uncertain enable is resumed; never replace it automatically.
  var old=cancel;cancel=new();old.Dispose();var c=client;var token=cancel.Token;
  if(liveEnable?.Result!=null){liveEnable.Dispose();liveEnable=null;}
  loop=ResumeLive(c,token);
 }
 private async Task ResumeLive(DeviceLogClient c,CancellationToken token){try{liveEnable??=c.Command(new(){["action"]="enable_live"});await liveEnable.RunAsync(token);await Poll(c,token);}catch(OperationCanceledException){}catch(Exception e){if(!token.IsCancellationRequested)status.Text=e.Message;}}
 public async Task StopAsync(){visible=false;scope="";cancel.Cancel();try{await loop;await action;}catch(OperationCanceledException){}exporting?.Dispose();exporting=null;lastCommand?.Dispose();lastCommand=null;liveEnable?.Dispose();liveEnable=null;client=null;}
 private static bool Same(DeviceLogClient a,DeviceLogClient b)=>a.Owner==b.Owner&&a.DeviceId==b.DeviceId&&a.SessionId==b.SessionId;
 private void StartAction(Func<DeviceLogClient,CancellationToken,Task> perform){if(busy)return;if(client==null||!ready){historyStatus.Text=previewStatus.Text="请选择支持日志功能的在线设备。";return;}var c=client;var token=cancel.Token;action=Action(c,token,perform);}
 private async Task Action(DeviceLogClient c,CancellationToken token,Func<DeviceLogClient,CancellationToken,Task> perform){busy=true;try{await perform(c,token);}catch(OperationCanceledException){}catch(Exception e){if(!token.IsCancellationRequested)historyStatus.Text=previewStatus.Text=e.Message;}finally{busy=false;}}
 private async Task RefreshHistory(DeviceLogClient c,CancellationToken token){
  var settings=await c.ReadAsync<DeviceLogSettings>("status",token);var history=await c.ReadAsync<DeviceLogHistory>("history",token,directory:directory.Text.Trim());token.ThrowIfCancellationRequested();
  enabled.IsChecked=settings.LogSaveEn=="1";interval.Text=string.IsNullOrWhiteSpace(settings.LogSaveItv)?"300":settings.LogSaveItv;files.ItemsSource=history.Files.OrderByDescending(x=>x.Modified).ToArray();
  var checkedPaths=string.Join("；",history.Directories.Select(d=>$"{d.Path} ({d.State})"));
  historyStatus.Text=history.Files.Length>0?$"发现 {history.Files.Length} 份日志。历史开关 {settings.LogSaveEn}，总开关 {settings.DebuglogEnable}。":settings.LogSaveEn=="0"?"历史保存未开启，已检查目录未发现日志；可独立开启或填写其他目录。":"未发现历史文件，请检查保存周期、RAM缓存及目录状态。";
  if(history.Directories.Any(d=>d.State is "unreadable" or "unavailable"))historyStatus.Text+=" 有目录无法读取，不能据此判定没有历史文件。";
  historyStatus.Text+="\n已检查："+checkedPaths+(history.Limited?"；扫描达到上限，请指定更小的目录。":"");
 }
 private async Task ApplyHistory(DeviceLogClient c,CancellationToken token){
  if(!uint.TryParse(interval.Text,out var seconds)||seconds<1||seconds>65535)throw new ArgumentException("保存间隔须为1～65535秒，默认300秒。");
  var on=enabled.IsChecked==true;var save=persist.IsChecked==true;
  if(MessageBox.Show(Window.GetWindow(this),$"{(on?"开启":"关闭")}历史保存，间隔 {seconds} 秒。\n{(on?"开启时同时打开debuglog总开关。":"不关闭debuglog总开关或实时页面输出。")}\n{(save?"将commit整份NVRAM，包含其他已暂存修改。":"仅设置运行值，不commit。")}\n不重启设备或业务服务。", "历史日志设置",MessageBoxButton.OKCancel)!=MessageBoxResult.OK)return;
  if(c.Owner.Pending!=null)throw new InvalidOperationException("请先继续原设置或处理响应不确定的原请求。");
  lastCommand?.Dispose();lastCommand=c.Command(new(){["action"]="history_settings",["enabled"]=on?"1":"0",["interval"]=seconds.ToString(),["persist"]=save?"1":"0"});await lastCommand.RunAsync(token);await RefreshHistory(c,token);historyStatus.Text="设置已回读确认；实际落盘以发现的文件为准。\n"+historyStatus.Text;
 }
 private async Task ContinueCommand(DeviceLogClient c,CancellationToken token){if(lastCommand==null||!Same(lastCommand.Client,c))throw new InvalidOperationException("没有需要继续的日志设置。");await lastCommand.RunAsync(token);await RefreshHistory(c,token);}
 private async Task SaveLive(){try{var picker=new SaveFileDialog{Filter="文本日志|*.txt",FileName="实时日志.txt"};if(picker.ShowDialog(Window.GetWindow(this))==true)await File.WriteAllBytesAsync(picker.FileName,buffer.Bytes());}catch(Exception e){status.Text=e.Message;}}
 private async Task ExportSelected(DeviceLogClient c,CancellationToken token){
  if(c.Owner.Pending!=null)throw new InvalidOperationException("请先继续原导出或处理响应不确定的原请求。");
  var selected=files.SelectedItems.Cast<DeviceLogFile>().ToArray();if(selected.Length==0)throw new InvalidOperationException("请选择要导出的文件（支持多选）。");
  string folder="";string? target=null;
  if(selected.Length==1){var name=asText.IsChecked==true?selected[0].Name.Replace(".txt.gz",".txt"):selected[0].Name;var picker=new SaveFileDialog{FileName=name,Filter="日志文件|*.*"};if(picker.ShowDialog(Window.GetWindow(this))!=true)return;target=picker.FileName;}
  else{var picker=new OpenFolderDialog{Title="选择批量日志导出目录"};if(picker.ShowDialog(Window.GetWindow(this))!=true)return;folder=picker.FolderName;}
  var names=new HashSet<string>(StringComparer.OrdinalIgnoreCase);
  foreach(var entry in selected){token.ThrowIfCancellationRequested();var name=asText.IsChecked==true?entry.Name.Replace(".txt.gz",".txt"):entry.Name;var unique=name;for(int i=2;!names.Add(unique);i++)unique=$"{i}-{name}";var path=target??System.IO.Path.Combine(folder,unique);
   if(target==null&&File.Exists(path)&&MessageBox.Show(Window.GetWindow(this),"覆盖已存在的文件？\n"+path,"日志导出",MessageBoxButton.YesNo)!=MessageBoxResult.Yes)continue;
   exporting?.Dispose();exporting=new(c,entry,path,asText.IsChecked==true);exporting.Changed+=()=>{if(!token.IsCancellationRequested)historyStatus.Text=exporting.Status;};await exporting.RunAsync(token);lastAsset=exporting.Asset;
  }
 }
 private async Task ContinueExport(DeviceLogClient c,CancellationToken token){if(exporting==null||!Same(exporting.Client,c))throw new InvalidOperationException("没有待继续的导出。");await exporting.RunAsync(token);lastAsset=exporting.Asset;historyStatus.Text=exporting.Status;}
 private async Task PreviewAsset(DeviceLogClient c,CancellationToken token){if(lastAsset==null)throw new InvalidOperationException("请先导出一份日志或导入本地日志。");var result=await c.Owner.TrackAsync(()=>c.Owner.Api.GetAsync<DeviceLogPreview>($"log-assets/{ApiClient.Segment(lastAsset.AssetId)}/preview",token));token.ThrowIfCancellationRequested();preview.Text=result.Text;previewStatus.Text=$"原文共 {result.Bytes} 字节"+(result.Truncated?"，只预览前512KiB。":"。")+"\n厂商AT解析器待样本确认，尚不生成时间范围/基站分析。";pages.SelectedIndex=2;}
 private async Task ImportPreview(DeviceLogClient c,CancellationToken token){var picker=new OpenFileDialog{Filter="日志文件|*.txt;*.gz|所有文件|*.*"};if(picker.ShowDialog(Window.GetWindow(this))!=true)return;var mutation=await Mutation.ImportAsync(picker.FileName,token);lastAsset=(await c.Owner.ExecuteAsync(mutation)).Deserialize<Asset>(ApiJson.Options)!;await PreviewAsset(c,token);}
}
