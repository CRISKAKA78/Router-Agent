using System.Text.Json;

namespace RouterWorkbench.Client;

// One user-requested exchange. Successful stages and uncertain bytes survive explicit retry.
public sealed class FileExchange(WorkspaceConnection owner,string deviceId,string localPath,string remotePath,bool download,bool overwrite=false,string mode="0644") : IDisposable
{
    public WorkspaceConnection Owner {get;}=owner;
    public string DeviceId {get;}=deviceId;
    public string LocalPath {get;}=localPath;
    public string RemotePath {get;}=remotePath;
    public bool Download {get;}=download;
    public string TaskId {get;private set;}="";
    public Asset? Asset {get;private set;}
    public string Status {get;private set;}="等待传输";
    public bool Complete {get;private set;}
    public Mutation? PendingStage {get;private set;}
    public event Action? Changed;
    public void Dispose(){if(PendingStage!=Owner.Pending)PendingStage?.Dispose();}
    public static FileExchange ExistingDownload(WorkspaceConnection owner,string deviceId,string taskId,string localPath){var value=new FileExchange(owner,deviceId,localPath,"/tmp/download.bin",true);value.TaskId=taskId;return value;}
    private void Report(string status){Status=status;Changed?.Invoke();}
    public Task RunAsync(CancellationToken token)=>Owner.TrackAsync(async()=>{
        using var cancel=CancellationTokenSource.CreateLinkedTokenSource(token,Owner.Token);
        var ct=cancel.Token;
        async Task<JsonElement> Send(Func<Task<Mutation>> create){
            ct.ThrowIfCancellationRequested();PendingStage??=await create();ct.ThrowIfCancellationRequested();
            while(Owner.Busy)await Task.Delay(50,ct);
            if(Owner.Pending!=null&&Owner.Pending!=PendingStage)throw new InvalidOperationException("请先处理响应不确定的原请求。");
            var response=await Owner.ExecuteAsync(PendingStage,Owner.Pending==PendingStage);
            PendingStage=null;return response;
        }
        try{
            if(Complete)return true;
            if(!Download&&Asset==null){Report("正在准备本地文件…");Asset=(await Send(()=>Mutation.ImportAsync(LocalPath,ct))).Deserialize<Asset>(ApiJson.Options)!;}
            if(TaskId.Length==0){
                Report(Download?"正在请求设备文件…":"正在上传到设备…");
                var response=await Send(()=>Task.FromResult(Download
                    ?new Mutation("下载文件","downloads",new{device_id=DeviceId,remote_path=RemotePath,name=RemoteDirectory.FileName(RemotePath),timeout_seconds=120})
                    :new Mutation("上传文件","uploads",new{device_id=DeviceId,asset_id=Asset!.AssetId,remote_path=RemotePath,mode,overwrite,timeout_seconds=120})));
                TaskId=response.GetProperty("task_id").GetString()!;Changed?.Invoke();
            }
            var until=DateTime.UtcNow.AddSeconds(150);
            TaskDetail? task=null;
            while(true){
                ct.ThrowIfCancellationRequested();task=await Owner.Api.GetAsync<TaskDetail>("tasks/"+ApiClient.Segment(TaskId),ct);
                if(Download){
                    var transfer=await Owner.Api.GetAsync<Transfer>($"tasks/{ApiClient.Segment(TaskId)}/transfer",ct);
                    if(transfer.Committed&&transfer.Released)break;
                    if(transfer.Failed||task.State=="rejected"||task.Result is {Status:not "success"})throw new IOException("设备下载失败："+task.StateText+" "+task.Result?.Stderr);
                }else if(task.Result!=null||task.State=="rejected"){
                    if(task.Result?.Status!="success")throw new IOException("设备上传失败："+task.StateText+" "+task.Result?.Stderr);
                    Complete=true;Report("上传完成");return true;
                }
                if(DateTime.UtcNow>=until)throw new TimeoutException("等待原任务结果超时，可继续查询原传输。");
                Report("传输中 · "+task.StateText);await Task.Delay(250,ct);
            }
            if(Asset==null){Report("正在确认完整文件…");var response=await Send(()=>Task.FromResult(new Mutation("确认设备下载",$"downloads/{ApiClient.Segment(TaskId)}/complete")));Asset=response.GetProperty("asset").Deserialize<Asset>(ApiJson.Options)!;}
            ct.ThrowIfCancellationRequested();Report("正在校验并保存本地文件…");await Owner.Api.SaveContentAsync(Asset,LocalPath,ct);
            Complete=true;Report(task?.Result?.Status=="success"?"下载完成，已保存本地":"文件已校验保存 · 设备结果"+(task?.Result==null?"待确认":"："+task.StateText));return true;
        }catch(OperationCanceledException){if(PendingStage!=Owner.Pending){PendingStage?.Dispose();PendingStage=null;}Report("已停止本地等待，可继续原传输");throw;}
        catch(Exception error){Report(Owner.Pending==PendingStage&&PendingStage!=null?"响应不确定，请核对后重试原请求":error.Message);throw;}
    });
}
