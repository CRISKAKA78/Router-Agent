using System.Buffers.Binary;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using RouterWorkbench.Core;

namespace RouterWorkbench.Desktop.Tests;

// Test-only Protocol v1 peer. Product assemblies have no Probe/Tunnel wire code.
internal sealed class TestProbe : IAsyncDisposable
{
    private readonly TcpClient tcp = new();
    private readonly CancellationTokenSource stop = new();
    private Task? reader;
    private Task? heartbeat;
    private object heartbeatPayload = new { uptime = 10, uptime_valid = true, running_tasks = 0 };
    private readonly SemaphoreSlim writer = new(1, 1);
    private ulong next = 1;
    private readonly Dictionary<string, byte[]> files = [];
    private readonly Dictionary<string, object> results = [];
    private JsonElement fileTask;
    private MemoryStream? uploading;
    public string SessionId { get; private set; } = "";
    public ulong ConfigurationRevision{get;private set;}
    public int LogEnables {get;private set;}
    public int LogCommits {get;private set;}
    public int LogReads {get;private set;}
    public int LogSnapshots {get;private set;}
    public int LogReleases {get;private set;}
    private readonly Dictionary<string,string> logSettings=new(){["debuglog_enable"]="0",["syslogd_enable"]="0",["log_save_en"]="0",["log_save_itv"]="300"};
    public int Executions { get; private set; }
    public int NeighborScans {get;private set;}
    public int NeighborInspections {get;private set;}
    public string? NeighborCidr {get;private set;}
    public Task ReportAsync(string group, object values) => SendAsync(0x20, new { config_revision=ConfigurationRevision,@event="telemetry",group,values });
    public async Task StartAsync(int port, string id, object? registration = null)
    {
        await tcp.ConnectAsync("127.0.0.1", port);
        var registrationNode=System.Text.Json.Nodes.JsonNode.Parse(JsonSerializer.Serialize(registration ?? new { device_id = id, probe_version = "phase6-fixture", arch = "x86_64", hostname = "售后测试路由器", boot_id = "fixture", capabilities = new[] { "exec", "file", "tunnel" } }))!.AsObject();
        var capabilities=registrationNode["capabilities"]!.AsArray();
        foreach(var capability in new[]{"managed_config_v1","telemetry_v2"})if(!capabilities.Any(v=>v?.GetValue<string>()==capability))capabilities.Add(capability);
        await SendAsync(1,registrationNode);
        var (_, _, _, ack) = await ReadAsync();
        SessionId = JsonDocument.Parse(ack).RootElement.GetProperty("session_id").GetString()!;
        reader = RunAsync();
        heartbeat = HeartbeatAsync();
    }
    private async Task HeartbeatAsync() {
        try { while (!stop.IsCancellationRequested) { await Task.Delay(10000, stop.Token); await SendAsync(3, heartbeatPayload); } }
        catch (Exception e) when (e is IOException or OperationCanceledException or ObjectDisposedException) { }
    }
    private async Task RunAsync()
    {
        try
        {
            while (!stop.IsCancellationRequested)
            {
                var (type, _, message, payload) = await ReadAsync();
                if (type == 0x31)
                {
                    uploading!.Write(payload, 28, payload.Length - 28); continue;
                }
                using var doc = JsonDocument.Parse(payload); var value = doc.RootElement;
                if(type==0x07){ConfigurationRevision=value.GetProperty("revision").GetUInt64();await SendAsync(0x08,new{reply_to=message,revision=ConfigurationRevision,success=true,error=""},1);}
                else if(type==0x20&&value.GetProperty("event").GetString()=="device_log_query"){
                    EnsureLogFiles();object data;
                    var op=value.GetProperty("operation").GetString();
                    if(op=="status")data=logSettings;
                    else if(op=="history")data=new{files=new[]{new{name="FF_BKDATA_2025-09-25.txt.gz",path="/jffs/FF_BKDATA_2025-09-25.txt.gz",size=files["/jffs/FF_BKDATA_2025-09-25.txt.gz"].Length,modified=1758800000,cached=false}},directories=new[]{new{path="/tmp/third_party/data",state="ok"},new{path="/jffs",state="alias"},new{path="/tmp/root",state="ok"}},limited=false};
                    else{LogReads++;var bytes="2025-09-25 12:00:00: AT+CSQ\n+CSQ: 20,99\n中文\n"u8.ToArray();var at=(int)Math.Min(value.GetProperty("offset").GetUInt64(),(ulong)bytes.Length);data=new{state="ok",generation="fixture:1",start=at,offset=bytes.Length,gap=false,data_hex=Convert.ToHexStringLower(bytes.AsSpan(at))};}
                    await SendAsync(0x20,new{@event="device_log_reply",request_id=value.GetProperty("request_id").GetString(),data,error=""});
                }
                else if (type == 0x10)
                {
                    var id = value.GetProperty("task_id").GetString()!;
                    await SendAsync(0x11, new { reply_to = message, task_id = id, accepted = true, state = results.ContainsKey(id) ? "success" : "queued" }, 1);
                    if (results.TryGetValue(id, out var old)) { await SendAsync(0x12, old); continue; }
                    var taskType = value.GetProperty("type").GetString();
                    if (taskType == "exec") { Executions++; var directory=value.GetProperty("params").GetProperty("command").GetString()!.StartsWith("cd "); await ResultAsync(value, directory ? "f\0"+"12\0"+"1788700000\0"+"network\0"+"f\0"+"0\0"+"1788700000\0"+"line\nname\0" : "fixture stdout\n中文结果", directory ? "" : "fixture stderr", new { }); }
                    else if(taskType=="neighbor_inspect") {
                        NeighborInspections++;
                        await ResultAsync(value,JsonSerializer.Serialize(new{networks=new[]{new{ @interface="br0",bridge=true,vlan=false,master="",eligible=true,reason="",ipv4=new[]{"192.0.2.1/24"},ports=new[]{"lan1"}}},preset="",preset_status="not_tested",raw_summary="",ports=Array.Empty<string>()}),"",new{});
                    }
                    else if(taskType=="neighbor_scan") {
                        NeighborScans++;NeighborCidr=value.GetProperty("params").GetProperty("cidr").GetString();
                        await Task.Delay(1000,stop.Token);
                        var rows=new[]{new{ip="192.0.2.2",mac="02:00:00:00:00:02",port="",hostname="fixture",source="active_arp",state="responded",active_age_ms=0}};
                        await ResultAsync(value,JsonSerializer.Serialize(new{requests=253,responses=1,rows}),"",new{});
                    }
                    else if(taskType=="device_logs"){
                        EnsureLogFiles();var p=value.GetProperty("params");var a=p.GetProperty("action").GetString();object result=logSettings;
                        if(a=="enable_live"){LogEnables++;LogCommits++;logSettings["debuglog_enable"]="1";logSettings["syslogd_enable"]="3";}
                        else if(a=="history_settings"){logSettings["log_save_en"]=p.GetProperty("enabled").GetString()!;logSettings["log_save_itv"]=p.GetProperty("interval").GetString()!;if(logSettings["log_save_en"]=="1")logSettings["debuglog_enable"]="1";if(p.GetProperty("persist").GetString()=="1")LogCommits++;}
                        else if(a=="release"){LogReleases++;files.Remove(p.GetProperty("path").GetString()!);}
                        else if(a=="snapshot"){LogSnapshots++;var source=p.GetProperty("path").GetString()!;var path="/tmp/router-agent-log-fixture/"+Path.GetFileName(source);files[path]=files[source].ToArray();result=new{remote_path=path,name=Path.GetFileName(source),size=files[path].Length,expires_in_seconds=3600};}
                        else result=new{};
                        await ResultAsync(value,JsonSerializer.Serialize(result),"",new{});
                    }
                    else if (taskType == "router_config") { Executions++; await ResultAsync(value, "fixture configuration result", "", new { }); }
                    else
                    {
                        fileTask = value.Clone();
                        if (taskType == "download")
                        {
                            var p = fileTask.GetProperty("params"); var content = files.GetValueOrDefault(p.GetProperty("remote_path").GetString()!, Encoding.UTF8.GetBytes("fixture download"));
                            var hash = Convert.ToHexStringLower(SHA256.HashData(content));
                            await SendAsync(0x30, new { transfer_id = p.GetProperty("transfer_id").GetString(), task_id = id, direction = "device_to_server", name = p.GetProperty("result_name").GetString(), remote_path = p.GetProperty("remote_path").GetString(), size = content.Length, sha256 = hash, chunk_size = 65536 });
                        }
                    }
                }
                else if (type == 0x30)
                {
                    uploading = new MemoryStream();
                    await SendAsync(0x33, new { reply_to = message, transfer_id = value.GetProperty("transfer_id").GetString(), status = "ready", received = 0 }, 1);
                }
                else if (type == 0x32)
                {
                    var p = fileTask.GetProperty("params"); var content = uploading!.ToArray(); uploading.Dispose(); uploading = null;
                    files[p.GetProperty("remote_path").GetString()!] = content;
                    var hash = Convert.ToHexStringLower(SHA256.HashData(content));
                    await SendAsync(0x33, new { reply_to = message, transfer_id = p.GetProperty("transfer_id").GetString(), status = "done", received = content.Length, sha256_ok = true }, 1);
                    await ResultAsync(fileTask, "", "", new { transfer_id = p.GetProperty("transfer_id").GetString(), size = content.Length, sha256 = hash });
                }
                else if (type == 0x33)
                {
                    var p = fileTask.GetProperty("params"); var id = p.GetProperty("transfer_id").GetString()!;
                    var content = files.GetValueOrDefault(p.GetProperty("remote_path").GetString()!, Encoding.UTF8.GetBytes("fixture download"));
                    var hash = Convert.ToHexStringLower(SHA256.HashData(content));
                    if (value.GetProperty("status").GetString() == "ready")
                    {
                        var chunk = new byte[28 + content.Length]; Convert.FromHexString(id.Replace("-", "")).CopyTo(chunk, 0);
                        BinaryPrimitives.WriteUInt32BigEndian(chunk.AsSpan(24), (uint)content.Length); content.CopyTo(chunk, 28);
                        if (content.Length > 0) await SendBytesAsync(0x31, chunk, 2);
                        await SendAsync(0x32, new { transfer_id = id, size = content.Length, sha256 = hash });
                    }
                    else if (value.GetProperty("status").GetString() == "done") await ResultAsync(fileTask, "", "", new { transfer_id = id, size = content.Length, sha256 = hash });
                }
            }
        }
        catch (Exception e) when (e is IOException or OperationCanceledException or ObjectDisposedException) { }
    }
    private void EnsureLogFiles(){
        const string path="/jffs/FF_BKDATA_2025-09-25.txt.gz";if(files.ContainsKey(path))return;
        using var raw=new MemoryStream();foreach(var text in new[]{"first AT+CSQ\n","second 中文\n"}){using var gz=new System.IO.Compression.GZipStream(raw,System.IO.Compression.CompressionLevel.Fastest,true);gz.Write(Encoding.UTF8.GetBytes(text));}files[path]=raw.ToArray();
    }
    public Task ReportUptimeAsync(long? seconds) {
        heartbeatPayload = new { uptime = seconds ?? 0, uptime_valid = seconds.HasValue, running_tasks = 0 };
        return SendAsync(3, heartbeatPayload);
    }
    private async Task ResultAsync(JsonElement task, string stdout, string stderr, object details)
    {
        var id = task.GetProperty("task_id").GetString()!;
        var now = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
        var result = new { task_id = id, status = "success", started_at = now, finished_at = now, exit_code = 0, stdout, stderr, truncated = false, result = details };
        results[id] = result; await SendAsync(0x12, result);
    }
    private Task SendAsync(byte type, object payload, ushort flags = 0) => SendBytesAsync(type, JsonSerializer.SerializeToUtf8Bytes(payload), flags);
    private async Task SendBytesAsync(byte type, byte[] payload, ushort flags)
    {
        await writer.WaitAsync(stop.Token);
        try {
        var header = new byte[20]; "RMP1"u8.CopyTo(header); header[4] = 1; header[5] = type;
        BinaryPrimitives.WriteUInt16BigEndian(header.AsSpan(6), flags); BinaryPrimitives.WriteUInt32BigEndian(header.AsSpan(8), (uint)payload.Length); BinaryPrimitives.WriteUInt64BigEndian(header.AsSpan(12), next++);
        await tcp.GetStream().WriteAsync(header, stop.Token); await tcp.GetStream().WriteAsync(payload, stop.Token);
        } finally { writer.Release(); }
    }
    private async Task<(byte Type, ushort Flags, ulong Id, byte[] Payload)> ReadAsync()
    {
        var header = new byte[20]; await tcp.GetStream().ReadExactlyAsync(header, stop.Token);
        var payload = new byte[BinaryPrimitives.ReadUInt32BigEndian(header.AsSpan(8))]; await tcp.GetStream().ReadExactlyAsync(payload, stop.Token);
        return (header[5], BinaryPrimitives.ReadUInt16BigEndian(header.AsSpan(6)), BinaryPrimitives.ReadUInt64BigEndian(header.AsSpan(12)), payload);
    }
    public async ValueTask DisposeAsync() { stop.Cancel(); tcp.Dispose(); if (reader != null) await reader; if (heartbeat != null) await heartbeat; uploading?.Dispose(); stop.Dispose(); writer.Dispose(); }
}
