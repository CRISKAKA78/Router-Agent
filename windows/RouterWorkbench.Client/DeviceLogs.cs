using System.Text;
using System.Text.Json;
namespace RouterWorkbench.Client;
public sealed record DeviceLogResult<T>(string SessionId,T Value);
public sealed record DeviceLogSettings(string DebuglogEnable,string SyslogdEnable,string LogSaveEn,string LogSaveItv);
public sealed record DeviceLogBatch(string State,string Generation,ulong Start,ulong Offset,bool Gap,byte[] Data);
public sealed record DeviceLogFile(string Name,string Path,ulong Size,long Modified,bool Cached)
{
 public string SizeText=>Labels.Bytes((long)Size);
 public string ModifiedText=>Modified<=253402300799?Labels.Time(DateTimeOffset.FromUnixTimeSeconds(Modified)):"时间未知";
 public string KindText=>Cached?"RAM缓存（未落盘）":"历史文件";
}
public sealed record DeviceLogDirectory(string Path,string State);
public sealed record DeviceLogHistory(DeviceLogFile[] Files,DeviceLogDirectory[] Directories,bool Limited);
public sealed record DeviceLogPreview(string Text,bool Truncated,ulong Bytes,string ParserStatus);
public sealed class DeviceLogClient(WorkspaceConnection owner,string deviceId,string sessionId)
{
 public WorkspaceConnection Owner=>owner;
 public string DeviceId=>deviceId;
 public string SessionId=>sessionId;
 public Task<T> ReadAsync<T>(string operation,CancellationToken token,string directory="",string generation="",ulong offset=0)=>owner.TrackAsync(async()=>{
  var path=$"devices/{ApiClient.Segment(deviceId)}/logs/{operation}?session_id={ApiClient.Segment(sessionId)}&directory={Uri.EscapeDataString(directory)}&generation={Uri.EscapeDataString(generation)}&offset={offset}";
  var r=await owner.Api.GetAsync<DeviceLogResult<T>>(path,token);token.ThrowIfCancellationRequested();
  if(r.SessionId!=sessionId)throw new InvalidDataException("日志响应来自其他设备会话。");return r.Value;
 });
 public DeviceLogCommand Command(Dictionary<string,string> parameters)=>new(this,parameters);
}
// Retains the exact mutation and known task across cancellation/uncertain replies.
public sealed class DeviceLogCommand : IDisposable
{
 private readonly DeviceLogClient client;
 private readonly Mutation mutation;
 public DeviceLogClient Client=>client;
 public string TaskId {get;private set;}="";
 public JsonElement? Result {get;private set;}
 public Mutation Request=>mutation;
 public DeviceLogCommand(DeviceLogClient client,Dictionary<string,string> parameters){this.client=client;mutation=new("设备日志操作",$"devices/{ApiClient.Segment(client.DeviceId)}/log-tasks",new{session_id=client.SessionId,@params=parameters});}
 public Task<JsonElement> RunAsync(CancellationToken token)=>client.Owner.TrackAsync(async()=>{
  using var cancel=CancellationTokenSource.CreateLinkedTokenSource(token,client.Owner.Token);var ct=cancel.Token;
  if(Result is {} saved)return saved;
  if(TaskId.Length==0){ct.ThrowIfCancellationRequested();if(client.Owner.Pending!=null&&client.Owner.Pending!=mutation)throw new InvalidOperationException("请先处理响应不确定的原请求。");
   var response=await client.Owner.ExecuteAsync(mutation,client.Owner.Pending==mutation);TaskId=response.GetProperty("task_id").GetString()!;
  }
  while(true){ct.ThrowIfCancellationRequested();var task=await client.Owner.Api.GetAsync<TaskDetail>($"tasks/{ApiClient.Segment(TaskId)}",ct);
   if(task.DeviceId!=client.DeviceId)throw new InvalidDataException("日志任务设备不匹配。");
   if(task.Result is {} result){if(result.Status!="success"||result.ExitCode!=0||result.Truncated)throw new IOException($"日志操作失败（{TaskId}）：{result.Stderr}。设备配置可能已经部分改变，请核对后再操作。");using var json=JsonDocument.Parse(result.Stdout);Result=json.RootElement.Clone();return Result.Value;}
   if(task.State=="rejected")throw new IOException("Probe拒绝日志任务，请检查能力或任务容量。");await Task.Delay(250,ct);
  }
 });
 public void Dispose(){if(client.Owner.Pending!=mutation)mutation.Dispose();}
}
public sealed class DeviceLogBuffer(int capacity=512*1024)
{
 private readonly Queue<byte[]> chunks=[];
 private int size;
 public int Gaps {get;private set;}
 public bool Trimmed {get;private set;}
 public string Generation {get;private set;}="";
 public ulong Offset {get;private set;}
 public void Add(DeviceLogBatch batch){
  if(batch.Data.Length>8192||batch.Offset<batch.Start||batch.Offset-batch.Start!=(ulong)batch.Data.Length)throw new InvalidDataException("日志批次格式错误。");
  if(batch.Gap||Generation.Length>0&&(batch.Generation!=Generation||batch.Start!=Offset))Gaps++;
  if(Generation.Length==0&&batch.Start>0)Trimmed=true;
  Generation=batch.Generation;Offset=batch.Offset;
  if(batch.Data.Length>0){chunks.Enqueue(batch.Data.ToArray());size+=batch.Data.Length;}
  while(size>capacity&&chunks.Count>0){size-=chunks.Dequeue().Length;Trimmed=true;}
 }
 public byte[] Bytes()=>chunks.SelectMany(x=>x).ToArray();
 public string Text=>Encoding.UTF8.GetString(Bytes());
 public void Reset(){ClearDisplay();Generation="";Offset=0;Gaps=0;Trimmed=false;}
 public void ClearDisplay(){chunks.Clear();size=0;}
}
public sealed class DeviceLogExport : IDisposable
{
 private readonly DeviceLogClient client;
 private readonly DeviceLogCommand snapshot;
 private DeviceLogCommand? release;
 private FileExchange? exchange;
 private readonly string destination;
 private readonly bool text;
 private readonly string rawLocal;
 private Mutation? decode;
 private Asset? decoded;
 public DeviceLogClient Client=>client;
 public Asset? Asset=>exchange?.Asset;
 public string Status {get;private set;}="等待导出";
 public bool Complete {get;private set;}
 public event Action? Changed;
 public DeviceLogExport(DeviceLogClient client,DeviceLogFile file,string destination,bool text){this.client=client;this.destination=destination;this.text=text&&file.Name.EndsWith(".gz",StringComparison.OrdinalIgnoreCase);rawLocal=this.text?System.IO.Path.Combine(System.IO.Path.GetTempPath(),"router-log-"+Guid.NewGuid().ToString("N")+".gz"):destination;snapshot=client.Command(new(){["action"]="snapshot",["path"]=file.Path});}
 private void Report(string value){Status=value;Changed?.Invoke();}
 public Task RunAsync(CancellationToken token)=>client.Owner.TrackAsync(async()=>{
  if(Complete)return true;Report("正在建立设备文件快照…");var snap=await snapshot.RunAsync(token);token.ThrowIfCancellationRequested();
  exchange??=new(client.Owner,client.DeviceId,rawLocal,snap.GetProperty("remote_path").GetString()!,true);
  exchange.Changed-=ExchangeChanged;exchange.Changed+=ExchangeChanged;await exchange.RunAsync(token);
  release??=client.Command(new(){["action"]="release",["path"]=snap.GetProperty("remote_path").GetString()!});
  Report("文件已确认，正在释放探针临时快照…");await release.RunAsync(token);
  if(text){Report("正在解压并校验全部 gzip 数据段…");if(decoded==null){decode??=new("解压历史日志",$"log-assets/{ApiClient.Segment(exchange.Asset!.AssetId)}/text");if(client.Owner.Pending!=null&&client.Owner.Pending!=decode)throw new InvalidOperationException("请先处理响应不确定的原请求。");decoded=(await client.Owner.ExecuteAsync(decode,client.Owner.Pending==decode)).Deserialize<Asset>(ApiJson.Options)!;}
   await client.Owner.Api.SaveContentAsync(decoded,destination,token);
  }
  if(!text && exchange.Asset!.Name.EndsWith(".gz",StringComparison.OrdinalIgnoreCase)){
   try{await client.Owner.Api.GetAsync<DeviceLogPreview>($"log-assets/{ApiClient.Segment(exchange.Asset.AssetId)}/preview",token);}
   catch(ApiException e) when(e.Code=="log_preview_failed"){Complete=true;Report("原始字节已导出，但压缩日志校验未通过（损坏/截断/解压上限），不代表完整历史："+destination);return true;}
  }
  Complete=true;Report("已完整导出："+destination);return true;
 });
 private void ExchangeChanged()=>Report(exchange!.Status);
 public void Dispose(){snapshot.Dispose();release?.Dispose();exchange?.Dispose();if(decode!=client.Owner.Pending)decode?.Dispose();if(text&&File.Exists(rawLocal))File.Delete(rawLocal);}
}
