using System.Text.Json;

namespace RouterWorkbench.Core;

// Only non-secret preferences are persisted. Future authentication belongs in
// this connection boundary, with a separately designed credential provider.
public sealed record ServerProfile
{
    public int SchemaVersion { get; init; } = 1;
    public string ServerUrl { get; init; } = "http://127.0.0.1:8080";
    public string SshExecutable { get; init; } = "";
    public string TelnetExecutable { get; init; } = "";
    public bool SshUsePutty { get; init; }
    public bool TelnetUsePutty { get; init; }
    public string SshUser { get; init; } = "root";
    public string Theme { get; init; } = "Default";

    public Uri BaseUri()
    {
        if (!Uri.TryCreate(ServerUrl.Trim(), UriKind.Absolute, out var uri) ||
            (uri.Scheme != "http" && uri.Scheme != "https") || uri.UserInfo.Length != 0 ||
            uri.Query.Length != 0 || uri.Fragment.Length != 0 || uri.AbsolutePath != "/")
            throw new ArgumentException("服务器地址须为 http(s)://主机:端口，不含账号、查询或路径。" );
        return uri;
    }
    public static string DefaultPath => Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "RouterWorkbench", "profile.json");
    public static ServerProfile Load(string path)
    {
        if (!File.Exists(path)) return new();
        var value = JsonSerializer.Deserialize<ServerProfile>(File.ReadAllText(path), Wire.Json)
            ?? throw new InvalidDataException("连接配置为空");
        if (value.SchemaVersion != 1) throw new InvalidDataException("不支持的连接配置版本");
        value.BaseUri();
        return value;
    }
    public async Task SaveAsync(string path)
    {
        BaseUri();
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path))!);
        var temporary = path + ".tmp";
        await File.WriteAllTextAsync(temporary, JsonSerializer.Serialize(this, Wire.Json));
        File.Move(temporary, path, true);
    }
}
