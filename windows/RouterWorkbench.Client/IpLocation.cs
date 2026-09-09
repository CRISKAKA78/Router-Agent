using System.Net;
using System.Net.Sockets;
using System.Text.Json;

namespace RouterWorkbench.Client;

public sealed record IpLocationResult(string Place,string Isp,string Reason);

public sealed class IpLocation : IDisposable
{
    private readonly HttpClient http;
    private readonly Dictionary<string, (DateTimeOffset Until, IpLocationResult Value)> cache = [];
    public IpLocation(HttpMessageHandler? handler = null)
    {
        http = handler == null ? new HttpClient(new HttpClientHandler { AllowAutoRedirect = false }) : new HttpClient(handler);
        http.Timeout = TimeSpan.FromSeconds(5);
        http.MaxResponseContentBufferSize = 16384;
    }
    public static bool IsPublic(string? text)
    {
        if (!IPAddress.TryParse(text, out var ip)) return false;
        if (ip.IsIPv4MappedToIPv6) ip = ip.MapToIPv4();
        if (IPAddress.IsLoopback(ip)) return false;
        var b = ip.GetAddressBytes();
        if (ip.AddressFamily == AddressFamily.InterNetwork)
        {
            if (b[0] is 0 or 10 or 127 || b[0] >= 224 || b[0] == 169 && b[1] == 254 ||
                b[0] == 172 && b[1] is >= 16 and <= 31 || b[0] == 192 && b[1] == 168 ||
                b[0] == 100 && b[1] is >= 64 and <= 127 || b[0] == 198 && b[1] is 18 or 19) return false;
            return !(b[0] == 192 && b[1] == 0 && b[2] is 0 or 2 || b[0] == 192 && b[1] == 88 && b[2] == 99 ||
                b[0] == 198 && b[1] == 51 && b[2] == 100 || b[0] == 203 && b[1] == 0 && b[2] == 113);
        }
        // Only global unicast; exclude documentation, benchmarking, ORCHID and transition ranges.
        if (b[0] < 0x20 || b[0] > 0x3f || b[0] == 0x20 && b[1] == 2) return false;
        if (b[0] == 0x20 && b[1] == 1 && (b[2] < 2 || b[2] == 0x0d && b[3] == 0xb8)) return false;
        if (b[0] == 0x3f && b[1] == 0xff && (b[2] & 0xf0) == 0) return false;
        return true;
    }
    public async Task<string> LookupAsync(string text, CancellationToken token)
    {
        var result=await LookupDetailsAsync(text,token);
        return result.Place=="—"?"未获取（"+result.Reason+"）":result.Place+" / "+result.Isp;
    }
    public async Task<IpLocationResult> LookupDetailsAsync(string text, CancellationToken token)
    {
        token.ThrowIfCancellationRequested();
        if (!IsPublic(text)) return new("—","—","内网或保留地址不查询公网归属地");
        var ip = IPAddress.Parse(text); if (ip.IsIPv4MappedToIPv6) ip = ip.MapToIPv4();
        text = ip.ToString();
        if (cache.TryGetValue(text, out var found) && found.Until > DateTimeOffset.UtcNow) return found.Value;
        var result = new IpLocationResult("—","—","归属地服务未返回有效的对应 IP 结果"); var ttl = TimeSpan.FromMinutes(5);
        try
        {
            using var response = await http.GetAsync("https://ipwho.is/" + Uri.EscapeDataString(text) + "?lang=zh-CN&fields=ip,success,country,region,city,connection.isp", token);
            response.EnsureSuccessStatusCode();
            using var doc = JsonDocument.Parse(await response.Content.ReadAsStringAsync(token)); var root = doc.RootElement;
            if (root.GetProperty("success").GetBoolean() && IPAddress.TryParse(root.GetProperty("ip").GetString(), out var returned) && returned.Equals(ip))
            {
                string Field(JsonElement e, string name) => e.TryGetProperty(name, out var v) && v.ValueKind == JsonValueKind.String ? (v.GetString() ?? "")[..Math.Min(v.GetString()!.Length, 128)] : "";
                var parts = new[] { Field(root, "country"), Field(root, "region"), Field(root, "city") };
                var place = string.Join("·", parts.Where(s => !string.IsNullOrWhiteSpace(s)).Distinct());
                var isp=root.TryGetProperty("connection",out var c)&&c.ValueKind==JsonValueKind.Object?Field(c,"isp"):"";
                result=new(place.Length>0?place:"—",string.IsNullOrWhiteSpace(isp)?"—":isp,place.Length==0||string.IsNullOrWhiteSpace(isp)?"查询服务未提供完整归属地或运营商":"");
                if(place.Length>0&&!string.IsNullOrWhiteSpace(isp))ttl=TimeSpan.FromHours(24);
            }
        }
        catch (OperationCanceledException) when (!token.IsCancellationRequested) { result=new("—","—","归属地查询超时"); }
        catch (HttpRequestException e) { result=new("—","—",e.StatusCode==HttpStatusCode.TooManyRequests?"归属地服务请求限流，稍后重试":"归属地服务连接失败"); }
        catch (Exception e) when (e is JsonException or InvalidOperationException or KeyNotFoundException) { result=new("—","—","归属地服务返回格式无效"); }
        token.ThrowIfCancellationRequested();
        if (cache.Count >= 256) cache.Remove(cache.Keys.First());
        cache[text] = (DateTimeOffset.UtcNow + ttl, result); return result;
    }
    public void Dispose() => http.Dispose();
}
