using System.Net;
using System.Text.Json;
using System.Text.Json.Serialization;
using ProbeTemplateGenerator.Models;
using RouterAgent.Neighbors;
namespace ProbeTemplateGenerator.Services;
public sealed record NeighborReference(
 [property:JsonPropertyName("device_id")]string DeviceId,string Status,
 ReferenceRegistration Registration,
 [property:JsonPropertyName("current_session")]ReferenceSession? CurrentSession,
 [property:JsonPropertyName("applied_revision")]ulong AppliedRevision,
 [property:JsonPropertyName("neighbor_discovery")]NetworkDiscovery? NeighborDiscovery,
 [property:JsonPropertyName("neighbor_domains")]NeighborDomainSettings[]? NeighborDomains,
 [property:JsonPropertyName("neighbor_configuration")]NeighborSettings? NeighborConfiguration=null)
{public string Label=>$"{DeviceId} · {Registration.Model} · Probe {Registration.ProbeVersion}";}
public sealed record ReferenceRegistration(string Model,[property:JsonPropertyName("probe_version")]string ProbeVersion,string[] Capabilities);
public sealed record ReferenceSession([property:JsonPropertyName("session_id")]string SessionId);
public sealed partial class TemplatePublishingService
{
 private string? LastInspectionTaskId;
 public string[] ServerCapabilities {get;private set;}=[];
 public NeighborReference[] ReferenceDevices {get;private set;}=[];
 public bool SupportsNeighbors=>ServerCapabilities.Contains("neighbor_probe");
 public string NeighborCompatibility=>SupportsNeighbors?"Server：支持 neighbor_probe":"当前Management Server不支持邻居发现模板，请更新Server后再发布";
 public string CapabilitySummary=>$"当前能力：{(ServerCapabilities.Length==0?"未提供":string.Join("、",ServerCapabilities))}；所需能力：neighbor_probe";
 private async Task RefreshNeighborCapabilitiesAsync(string origin,CancellationToken token){
  try {var c=await RequestAsync<CapabilityResponse>(origin,"capabilities",HttpMethod.Get,null,null,token);token.ThrowIfCancellationRequested();if(Origin!=origin)return;ServerCapabilities=c.Capabilities??[];}
  catch(TemplateApiException e)when(e.Status is 404 or 405){ServerCapabilities=[];}
  catch(JsonException){ServerCapabilities=[];}
  if(SupportsNeighbors){var all=new List<NeighborReference>();for(int offset=0;;){var page=await RequestAsync<ReferencePage>(origin,"devices?limit=200&offset="+offset,HttpMethod.Get,null,null,token);all.AddRange(page.Items.Where(d=>d.Status=="online"));offset+=page.Items.Length;if(offset>=page.Total||page.Items.Length==0||offset>=10000)break;}token.ThrowIfCancellationRequested();if(Origin!=origin)return;ReferenceDevices=all.ToArray();}else ReferenceDevices=[];
 }
 public async Task<NetworkDiscovery> InspectNeighborsAsync(NeighborReference reference,bool vendorTest){
  EnsureWritable();if(!ServerCapabilities.Contains("neighbors_inspect_v1")||!reference.Registration.Capabilities.Contains("neighbors_inspect_v1"))throw new InvalidOperationException("Probe不支持只读网络检测（需要neighbors_inspect_v1）；仍可保存模板并使用高级设置。");
  var origin=Origin!;var token=lifetime!.Token;var path="devices/"+Uri.EscapeDataString(reference.DeviceId);var session=reference.CurrentSession?.SessionId??throw new InvalidOperationException("参考设备已离线");
  // One immutable inspection request; no automatic re-creation after uncertain delivery.
  var body=JsonSerializer.Serialize(new{session_id=session,config_revision=reference.AppliedRevision,vendor_test=vendorTest},Json);
  await ExecuteAsync(NewMutation(vendorTest?"FNR100只读测试":"识别采集网络","POST",path+"/neighbor-inspections",body),false);
  var taskId=LastInspectionTaskId??throw new InvalidDataException("检测响应缺少任务标识");
  return await TrackAsync(async()=>{
   for(int i=0;i<90;i++){await Task.Delay(400,token);var task=await RequestAsync<InspectionTask>(origin,"tasks/"+Uri.EscapeDataString(taskId),HttpMethod.Get,null,null,token);if(task.State is "failed" or "rejected" or "timeout")throw new InvalidOperationException("只读检测失败："+task.Result?.Stderr);if(task.State!="success")continue;
    var current=await RequestAsync<DiscoveryResponse>(origin,path+"/neighbor-discovery",HttpMethod.Get,null,null,token);if(current.Discovery is {} d&&d.SessionId==session&&d.ConfigRevision==reference.AppliedRevision&&!d.Expired(DateTimeOffset.UtcNow))return d;
   }throw new InvalidOperationException("检测等待超时或Session/配置已变化；请刷新参考设备，不自动重复执行。");});
 }
 private sealed record CapabilityResponse(string[]? Capabilities);
 private sealed record ReferencePage(NeighborReference[] Items,int Total);
 private sealed record InspectionAccepted([property:JsonPropertyName("task_id")]string TaskId);
 private sealed record InspectionTask(string State,InspectionResult? Result);
 private sealed record InspectionResult(string Stderr);
 private sealed record DiscoveryResponse(NetworkDiscovery? Discovery);
}
