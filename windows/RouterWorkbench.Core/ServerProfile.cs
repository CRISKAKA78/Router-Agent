using System.Text.Json;

namespace RouterWorkbench.Core;

// Only non-secret preferences are persisted. Future authentication belongs in
// this connection boundary, with a separately designed credential provider.
public sealed record ServerProfile
{
    public const string DefaultUiFontFamily = "Microsoft YaHei UI";
    public const double DefaultUiFontSize = 13;
    public int SchemaVersion { get; init; } = 1;
    public string ServerUrl { get; init; } = "http://127.0.0.1:8080";
    public string SshExecutable { get; init; } = "";
    public string TelnetExecutable { get; init; } = "";
    public bool SshUsePutty { get; init; }
    public bool TelnetUsePutty { get; init; }
    public string SshUser { get; init; } = "root";
    public string Theme { get; init; } = "Default";
    public string UiFontFamily { get; init; } = DefaultUiFontFamily;
    public double UiFontSize { get; init; } = DefaultUiFontSize;

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
        return value with {
            UiFontFamily = string.IsNullOrWhiteSpace(value.UiFontFamily) ? DefaultUiFontFamily : value.UiFontFamily,
            UiFontSize = double.IsFinite(value.UiFontSize) && value.UiFontSize is >= 10 and <= 24 ? value.UiFontSize : DefaultUiFontSize
        };
    }
    public async Task SaveAsync(string path)
    {
        BaseUri();
        if (string.IsNullOrWhiteSpace(UiFontFamily) || !double.IsFinite(UiFontSize) || UiFontSize is < 10 or > 24)
            throw new ArgumentException("请选择字体，字号须为 10～24。");
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path))!);
        var temporary = path + ".tmp";
        await File.WriteAllTextAsync(temporary, JsonSerializer.Serialize(this, Wire.Json));
        File.Move(temporary, path, true);
    }
}
