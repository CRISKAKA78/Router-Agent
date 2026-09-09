using System.IO;
using System.Net;
using System.Security.Cryptography;
using System.Text.Json;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static async Task FileExchangeRetryChecks()
    {
        var path=Path.Combine(output,"exchange-retry.bin");var bytes="retained bytes"u8.ToArray();await File.WriteAllBytesAsync(path,bytes);
        var asset=new Asset("asset","sample.bin",bytes.Length,Convert.ToHexStringLower(SHA256.HashData(bytes)),false,DateTimeOffset.UtcNow);
        HttpResponseMessage Data(object data)=>Json(JsonSerializer.Serialize(new{data},ApiJson.Options));
        var calls=new List<(string key,string body)>();int imports=0;
        using var handler=new DelegateHandler(async(request,token)=>{
            if(request.RequestUri!.AbsolutePath.EndsWith("/assets")){imports++;return Data(asset);}
            if(request.RequestUri.AbsolutePath.EndsWith("/uploads")){calls.Add((request.Headers.GetValues("Idempotency-Key").Single(),await request.Content!.ReadAsStringAsync(token)));if(calls.Count==1)throw new HttpRequestException("lost upload response");return Data(new{task_id="original"});}
            return Data(new{task_id="original",device_id="device",type="upload",state="success",result=new{status="success",exit_code=0,stdout="",stderr=""}});
        });
        await using(var connection=new WorkspaceConnection(new Uri("http://localhost:18080"),handler)){
            using var operation=new FileExchange(connection,"device",path,"/tmp/with space",false);
            try{await operation.RunAsync(default);throw new Exception("missing lost response");}catch(HttpRequestException){}
            Check(connection.Pending==operation.PendingStage&&operation.Asset?.AssetId=="asset","upload uncertainty retains prepared asset and exact pending stage");
            await operation.RunAsync(default);
            Check(operation.Complete&&operation.TaskId=="original"&&imports==1&&calls.Count==2&&calls[0]==calls[1],"retry resumes original upload bytes and key without importing or dispatching replacements");
        }
        var cancel=new CancellationTokenSource();int downloads=0,completes=0;
        using var downloadHandler=new DelegateHandler((request,token)=>{
            var endpoint=request.RequestUri!.AbsolutePath;
            if(endpoint.EndsWith("/downloads")){downloads++;cancel.Cancel();return Task.FromResult(Data(new{task_id="download-original"}));}
            if(endpoint.EndsWith("/transfer"))return Task.FromResult(Data(new{committed=true,released=true,failed=false,size=bytes.Length,sha256=asset.Sha256}));
            if(endpoint.EndsWith("/complete")){completes++;return Task.FromResult(Data(new{asset}));}
            if(endpoint.EndsWith("/content"))return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK){Content=new ByteArrayContent(bytes)});
            return Task.FromResult(Data(new{task_id="download-original",device_id="device",type="download",state="failed",result=new{status="failed",exit_code=1,stdout="",stderr="ACK lost"}}));
        });
        await using(var connection=new WorkspaceConnection(new Uri("http://localhost:18080"),downloadHandler)){
            var target=Path.Combine(output,"exchange-cancelled.bin");await File.WriteAllTextAsync(target,"keep original");
            using var operation=new FileExchange(connection,"device",target,"/tmp/sample.bin",true);
            try{await operation.RunAsync(cancel.Token);throw new Exception("missing cancellation");}catch(OperationCanceledException){}
            Check(operation.TaskId=="download-original"&&await File.ReadAllTextAsync(target)=="keep original","cancelling local wait retains task identity and preserves destination");
            await operation.RunAsync(default);var saved=await File.ReadAllBytesAsync(target);
            Check(downloads==1&&completes==1&&saved.SequenceEqual(bytes)&&operation.Status.Contains("失败"),"resume saves committed bytes once without falsifying failed device RESULT");
        }
        cancel.Dispose();
    }
}
