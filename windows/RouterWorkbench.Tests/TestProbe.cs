using System.Buffers.Binary;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using RouterWorkbench.Core;

namespace RouterWorkbench.Tests;

// Test-only Protocol v1 peer. Product assemblies have no Probe/Tunnel wire code.
internal sealed class TestProbe : IAsyncDisposable
{
    private readonly TcpClient tcp = new();
    private readonly CancellationTokenSource stop = new();
    private Task? reader;
    private Task? heartbeat;
    private readonly SemaphoreSlim writer = new(1, 1);
    private ulong next = 1;
    private readonly Dictionary<string, byte[]> files = [];
    private readonly Dictionary<string, object> results = [];
    private JsonElement fileTask;
    private MemoryStream? uploading;
    public string SessionId { get; private set; } = "";
    public int Executions { get; private set; }
    public async Task StartAsync(int port, string id)
    {
        await tcp.ConnectAsync("127.0.0.1", port);
        await SendAsync(1, new { device_id = id, probe_version = "phase6-fixture", arch = "x86_64", hostname = "售后测试路由器", boot_id = "fixture", capabilities = new[] { "exec", "file", "tunnel" } });
        var (_, _, _, ack) = await ReadAsync();
        SessionId = JsonDocument.Parse(ack).RootElement.GetProperty("session_id").GetString()!;
        reader = RunAsync();
        heartbeat = HeartbeatAsync();
    }
    private async Task HeartbeatAsync() {
        try { while (!stop.IsCancellationRequested) { await Task.Delay(10000, stop.Token); await SendAsync(3, new { uptime = 10, running_tasks = 0 }); } }
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
                if (type == 0x10)
                {
                    var id = value.GetProperty("task_id").GetString()!;
                    await SendAsync(0x11, new { reply_to = message, task_id = id, accepted = true, state = results.ContainsKey(id) ? "success" : "queued" }, 1);
                    if (results.TryGetValue(id, out var old)) { await SendAsync(0x12, old); continue; }
                    var taskType = value.GetProperty("type").GetString();
                    if (taskType == "exec") { Executions++; var directory=value.GetProperty("params").GetProperty("command").GetString()!.StartsWith("cd "); await ResultAsync(value, directory ? "f\0"+"12\0"+"1788700000\0"+"network\0"+"f\0"+"0\0"+"1788700000\0"+"line\nname\0" : "fixture stdout\n中文结果", directory ? "" : "fixture stderr", new { }); }
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
