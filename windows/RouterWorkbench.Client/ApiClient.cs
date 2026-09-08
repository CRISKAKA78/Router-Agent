using System.Net;
using System.Net.Http.Headers;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using RouterWorkbench.Core;

namespace RouterWorkbench.Client;

public sealed class ApiException(string code, string message, HttpStatusCode status) : Exception($"{message} [{code}, HTTP {(int)status}]")
{
    public string Code { get; } = code;
}

// Immutable request identity and payload. A raw import retains an open read-only handle,
// denying writes/deletion until the response is resolved or explicitly abandoned.
public sealed class Mutation : IDisposable
{
    public string Label { get; }
    public string Method { get; }
    public string Path { get; }
    public string Key { get; } = Guid.NewGuid().ToString("N");
    public byte[] Body { get; }
    public FileStream? File { get; private init; }
    public string? Hash { get; private init; }
    public Mutation(string label, string path, object? body = null, string method = "POST")
    { Label = label; Path = path; Method = method; Body = JsonSerializer.SerializeToUtf8Bytes(body ?? new { }, ApiJson.Options); }
    public static async Task<Mutation> ImportAsync(string path, CancellationToken token)
    {
        var file = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read, 65536, FileOptions.Asynchronous | FileOptions.SequentialScan);
        try {
            if (file.Length > 1L << 30) throw new ArgumentException("文件超过 1 GiB 导入上限。");
            var hash = Convert.ToHexStringLower(await SHA256.HashDataAsync(file, token)); file.Position = 0;
            return new("导入文件", "assets?name=" + ApiClient.Segment(System.IO.Path.GetFileName(path))) { File = file, Hash = hash };
        } catch { await file.DisposeAsync(); throw; }
    }
    public void Dispose() => File?.Dispose();
}

public sealed class ApiClient : IDisposable
{
    private readonly HttpClient http;
    private readonly CancellationToken lifetime;
    public Uri Origin { get; }
    public ApiClient(Uri origin, CancellationToken token, HttpMessageHandler? handler = null)
    {
        Origin = new ServerProfile { ServerUrl = origin.AbsoluteUri }.BaseUri(); lifetime = token;
        http = new HttpClient(handler ?? new HttpClientHandler { AllowAutoRedirect = false, UseCookies = false }) { Timeout = Timeout.InfiniteTimeSpan };
    }
    public static string Segment(string value) => Uri.EscapeDataString(value);
    private HttpRequestMessage Request(string path, string method = "GET") => new(new HttpMethod(method), new Uri(Origin, "/api/v1/" + path));
    private async Task<JsonElement> JsonAsync(HttpRequestMessage request, CancellationToken token, int seconds = 40)
    {
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(lifetime, token); cancel.CancelAfter(TimeSpan.FromSeconds(seconds));
        using var response = await http.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancel.Token);
        using var stream = await response.Content.ReadAsStreamAsync(cancel.Token);
        using var document = await JsonDocument.ParseAsync(stream, cancellationToken: cancel.Token);
        if (!response.IsSuccessStatusCode) {
            if (!document.RootElement.TryGetProperty("error", out var error) || !error.TryGetProperty("code", out var code)) throw new InvalidDataException("API 错误响应无法解析。");
            throw new ApiException(code.GetString()!, error.GetProperty("message").GetString()!, response.StatusCode);
        }
        if (!document.RootElement.TryGetProperty("data", out var data)) throw new InvalidDataException("API 响应缺少 data。");
        return data.Clone();
    }
    public async Task<T> GetAsync<T>(string path, CancellationToken token = default)
    { using var request = Request(path); return (await JsonAsync(request, token)).Deserialize<T>(ApiJson.Options) ?? throw new InvalidDataException("API data 为空。"); }
    public async Task<T[]> ListAsync<T>(string path, CancellationToken token = default)
    {
        var items = new List<T>();
        for (var offset = 0; ;) {
            var page = await GetAsync<Page<T>>($"{path}{(path.Contains('?') ? '&' : '?')}offset={offset}&limit=200", token);
            items.AddRange(page.Items); offset += page.Items.Length;
            if (offset >= page.Total || page.Items.Length == 0) return [.. items];
            if (offset >= 10000) throw new InvalidDataException("列表超过 10000 项，快照未完成。");
        }
    }
    public async Task<JsonElement> ExecuteAsync(Mutation mutation, CancellationToken token = default)
    {
        using var request = Request(mutation.Path, mutation.Method);
        request.Headers.Add("Idempotency-Key", mutation.Key);
        if (mutation.File is { } file) {
            file.Position = 0; request.Content = new BorrowedFileContent(file);
            request.Content.Headers.ContentType = new("application/octet-stream");
            request.Headers.Add("X-Content-SHA256", mutation.Hash);
        } else { request.Content = new ByteArrayContent(mutation.Body); request.Content.Headers.ContentType = new("application/json"); }
        return await JsonAsync(request, token, mutation.File == null ? 40 : 300);
    }
    public async Task SaveContentAsync(Asset asset, string destination, CancellationToken token = default)
    {
        var target = System.IO.Path.GetFullPath(destination);
        var temporary = target + "." + Guid.NewGuid().ToString("N") + ".tmp";
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(lifetime, token); cancel.CancelAfter(TimeSpan.FromMinutes(5));
        try {
            using var request = Request($"assets/{Segment(asset.AssetId)}/content");
            using var response = await http.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancel.Token);
            response.EnsureSuccessStatusCode();
            await using var input = await response.Content.ReadAsStreamAsync(cancel.Token);
            using var hash = IncrementalHash.CreateHash(HashAlgorithmName.SHA256);
            await using (var output = new FileStream(temporary, FileMode.CreateNew, FileAccess.Write, FileShare.None, 65536, true)) {
                var buffer = new byte[65536]; long total = 0;
                for (int n; (n = await input.ReadAsync(buffer, cancel.Token)) > 0;) {
                    total += n; if (total > asset.Size) throw new InvalidDataException("文件长度校验失败。");
                    hash.AppendData(buffer, 0, n); await output.WriteAsync(buffer.AsMemory(0, n), cancel.Token);
                }
                if (total != asset.Size || !Convert.ToHexStringLower(hash.GetHashAndReset()).Equals(asset.Sha256, StringComparison.OrdinalIgnoreCase)) throw new InvalidDataException("文件长度或 SHA-256 校验失败。");
                await output.FlushAsync(cancel.Token);
            }
            cancel.Token.ThrowIfCancellationRequested(); System.IO.File.Move(temporary, target, true);
        } finally { if (System.IO.File.Exists(temporary)) System.IO.File.Delete(temporary); }
    }
    public void Dispose() => http.Dispose();
    private sealed record Page<T>(T[] Items, int Total);
    private sealed class BorrowedFileContent(FileStream file) : HttpContent
    {
        protected override bool TryComputeLength(out long length) { length = file.Length; return true; }
        protected override Task SerializeToStreamAsync(Stream stream, TransportContext? context) => file.CopyToAsync(stream);
        protected override Task SerializeToStreamAsync(Stream stream, TransportContext? context, CancellationToken cancellationToken) => file.CopyToAsync(stream, cancellationToken);
    }
}
