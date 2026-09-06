using System.Net.Http.Headers;
using System.Security.Cryptography;
using System.Text.Json;

namespace RouterWorkbench.Core;

public sealed record Mutation(string Label, string Method, string Path, byte[] Body, string Key,
    string? FilePath = null, long FileLength = 0, string? Sha256 = null)
{
    public static Mutation Json(string label, string path, object body, string method = "POST") =>
        new(label, method, path, JsonSerializer.SerializeToUtf8Bytes(body, Wire.Json), Guid.NewGuid().ToString("N"));

    public static async Task<Mutation> ImportAsync(string path, CancellationToken ct)
    {
        await using var stream = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read, 65536, true);
        var length = stream.Length;
        if (length > 1L << 30) throw new ArgumentException("资产超过客户端 1 GiB 导入上限。");
        var hash = Convert.ToHexStringLower(await SHA256.HashDataAsync(stream, ct));
        return new("导入文件", "POST", "assets?name=" + Wire.Segment(System.IO.Path.GetFileName(path)), [],
            Guid.NewGuid().ToString("N"), path, length, hash);
    }
}

public sealed class ApiClient : IDisposable
{
    private readonly HttpClient http;
    public Uri BaseUri { get; }
    public ApiClient(Uri baseUri, HttpMessageHandler? handler = null)
    {
        BaseUri = baseUri;
        http = new HttpClient(handler ?? new SocketsHttpHandler { AllowAutoRedirect = false, UseCookies = false,
            ConnectTimeout = TimeSpan.FromSeconds(10), PooledConnectionLifetime = TimeSpan.FromMinutes(5) });
        http.BaseAddress = new Uri(baseUri, "api/v1/");
        http.Timeout = TimeSpan.FromSeconds(40);
    }
    public async Task<T> GetAsync<T>(string path, CancellationToken ct)
    {
        using var request = new HttpRequestMessage(HttpMethod.Get, path);
        var data = await SendAsync(request, ct).ConfigureAwait(false);
        return data.Deserialize<T>(Wire.Json) ?? throw new InvalidDataException("API data 为空");
    }
    public async Task<T[]> ListAsync<T>(string path, CancellationToken ct)
    {
        var items = new List<T>();
        var separator = path.Contains('?') ? "&" : "?";
        for (var offset = 0; ;)
        {
            var page = await GetAsync<Page<T>>($"{path}{separator}offset={offset}&limit=200", ct).ConfigureAwait(false);
            items.AddRange(page.Items);
            offset += page.Items.Length;
            if (offset >= page.Total || page.Items.Length == 0) return items.ToArray();
            // Explicit failure instead of silently presenting a truncated inventory.
            if (offset >= 10000) throw new InvalidDataException("列表超过 10000 项，客户端未完成快照，请缩小 Server 管理范围。");
        }
    }
    public async Task<JsonElement> ExecuteAsync(Mutation mutation, CancellationToken ct)
    {
        using var request = new HttpRequestMessage(new HttpMethod(mutation.Method), mutation.Path);
        request.Headers.Add("Idempotency-Key", mutation.Key);
        if (mutation.FilePath is { } path)
        {
            var stream = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read, 65536, true);
            request.Content = new StreamContent(stream);
            if (stream.Length != mutation.FileLength) throw new InvalidDataException("导入源文件长度已变化，请重新选择文件。");
            request.Content.Headers.ContentType = new MediaTypeHeaderValue("application/octet-stream");
            request.Content.Headers.ContentLength = mutation.FileLength;
            request.Headers.Add("X-Content-SHA256", mutation.Sha256);
        }
        else
        {
            request.Content = new ByteArrayContent(mutation.Body);
            request.Content.Headers.ContentType = new MediaTypeHeaderValue("application/json");
        }
        return await SendAsync(request, ct).ConfigureAwait(false);
    }
    private async Task<JsonElement> SendAsync(HttpRequestMessage request, CancellationToken ct)
    {
        using var response = await http.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, ct).ConfigureAwait(false);
        // Timeout also covers response body, not only headers.
        using var bodyDeadline = CancellationTokenSource.CreateLinkedTokenSource(ct);
        bodyDeadline.CancelAfter(TimeSpan.FromSeconds(40));
        await using var stream = await response.Content.ReadAsStreamAsync(bodyDeadline.Token).ConfigureAwait(false);
        using var document = await JsonDocument.ParseAsync(stream, cancellationToken: bodyDeadline.Token).ConfigureAwait(false);
        if (!response.IsSuccessStatusCode)
        {
            var error = document.RootElement.GetProperty("error");
            throw new ApiException(error.GetProperty("code").GetString()!, error.GetProperty("message").GetString()!, (int)response.StatusCode);
        }
        return document.RootElement.GetProperty("data").Clone();
    }
    public async Task SaveAssetAsync(Asset asset, string target, CancellationToken ct)
    {
        var temporary = target + "." + Guid.NewGuid().ToString("N") + ".part";
        try
        {
            using var response = await http.GetAsync($"assets/{Wire.Segment(asset.AssetId)}/content", HttpCompletionOption.ResponseHeadersRead, ct).ConfigureAwait(false);
            if (!response.IsSuccessStatusCode)
            {
                var text = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
                using var doc = JsonDocument.Parse(text);
                var e = doc.RootElement.GetProperty("error");
                throw new ApiException(e.GetProperty("code").GetString()!, e.GetProperty("message").GetString()!, (int)response.StatusCode);
            }
            using var deadline = CancellationTokenSource.CreateLinkedTokenSource(ct);
            deadline.CancelAfter(TimeSpan.FromMinutes(5));
            await using (var output = new FileStream(temporary, FileMode.CreateNew, FileAccess.ReadWrite, FileShare.None, 65536, true))
            {
                await response.Content.CopyToAsync(output, deadline.Token).ConfigureAwait(false);
                if (output.Length != asset.Size) throw new InvalidDataException("资产长度校验失败");
                output.Position = 0;
                var hash = Convert.ToHexStringLower(await SHA256.HashDataAsync(output, deadline.Token).ConfigureAwait(false));
                if (hash != asset.Sha256) throw new InvalidDataException("资产 SHA-256 校验失败");
            }
            File.Move(temporary, target, true);
        }
        finally { if (File.Exists(temporary)) File.Delete(temporary); }
    }
    public void Dispose() => http.Dispose();
}
