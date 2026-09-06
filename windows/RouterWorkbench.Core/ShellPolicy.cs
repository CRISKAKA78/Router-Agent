using System.Text.Json;
namespace RouterWorkbench.Core;
public static class ShellPolicy
{
 public sealed record Message(string Id, string Method, JsonElement Args);
 private static bool SameOrigin(Uri uri, Uri origin) => uri.Scheme == origin.Scheme && uri.Host == origin.Host && uri.Port == origin.Port && uri.UserInfo.Length == 0;
 public static bool IsDocument(string value, Uri origin) => Uri.TryCreate(value, UriKind.Absolute, out var uri) && SameOrigin(uri, origin) && uri.AbsolutePath == "/__workbench/index.html" && uri.Query.Length == 0;
 public static bool IsApi(Uri uri, Uri origin) => SameOrigin(uri, origin) && uri.AbsolutePath.StartsWith("/api/v1/", StringComparison.Ordinal);
 public static string? LocalResource(Uri uri, Uri origin, string root) {
  if (!SameOrigin(uri, origin) || !uri.AbsolutePath.StartsWith("/__workbench/", StringComparison.Ordinal)) return null;
  var relative = Uri.UnescapeDataString(uri.AbsolutePath[13..]);
  if (relative.Length == 0 || relative.Contains('\\') || relative.Contains(':') || relative.Split('/').Any(s => s is "." or "..")) return null;
  var resolved = Path.GetFullPath(Path.Combine(root, relative));
  return resolved.StartsWith(Path.GetFullPath(root) + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase) ? resolved : null;
 }
 public static Message Parse(string json) {
  if (json.Length > 131072) throw new ArgumentException("Bridge 消息过大。");
  using var doc = JsonDocument.Parse(json, new JsonDocumentOptions { MaxDepth = 8 });
  var root = doc.RootElement; Exact(root, "id", "method", "args");
  var id = root.GetProperty("id").GetString(); var method = root.GetProperty("method").GetString();
  if (id == null || id.Length is < 1 or > 64 || !id.All(char.IsAsciiDigit)) throw new ArgumentException("无效请求编号。");
  var args = root.GetProperty("args");
  switch (method) {
   case "getProfile": Exact(args); break;
   case "connect": Exact(args, "server_url"); break;
   case "chooseClient": Exact(args, "service"); break;
   case "beginSave": Exact(args, "name", "size", "sha256"); break;
   case "saveChunk": Exact(args, "handle", "offset", "base64"); break;
   case "finishSave": case "cancelSave": Exact(args, "handle"); break;
   case "saveProfile": Exact(args, "schema_version", "server_url", "theme", "ssh_user", "ssh_executable", "telnet_executable", "ssh_use_putty", "telnet_use_putty"); break;
   case "openEndpoint": Exact(args, "service", "host", "port", "address", "state", "url"); break;
   default: throw new ArgumentException("不允许的 Bridge 操作。");
  }
  return new(id, method!, args.Clone());
 }
 private static void Exact(JsonElement value, params string[] allowed) {
  if (value.ValueKind != JsonValueKind.Object) throw new ArgumentException("参数必须为对象。");
  var keys = new HashSet<string>();
  foreach (var p in value.EnumerateObject()) if (!allowed.Contains(p.Name) || !keys.Add(p.Name)) throw new ArgumentException("包含未知或重复参数。");
  if (allowed.Any(k => k != "url" && !keys.Contains(k))) throw new ArgumentException("缺少参数。");
 }
 public static void ValidateProfile(ServerProfile next, ServerProfile previous) {
  next.BaseUri();
  if (next.SchemaVersion != 1 || next.Theme is not ("Default" or "Light" or "Dark") || next.SshUser.Length is < 1 or > 64 || next.SshUser.StartsWith('-') || next.SshUser.Any(c => !char.IsAsciiLetterOrDigit(c) && c is not ('_' or '.' or '-'))) throw new ArgumentException("配置无效。");
  if (next.SshExecutable != previous.SshExecutable || next.TelnetExecutable != previous.TelnetExecutable || next.SshUsePutty != previous.SshUsePutty || next.TelnetUsePutty != previous.TelnetUsePutty) throw new ArgumentException("客户端路径必须通过 Windows 文件选择器修改。");
 }
 public static void ValidateEndpoint(Endpoint e) {
  if (e.State != "ready" || e.Service is not ("web" or "ssh" or "telnet") || e.Port is < 1 or > 65535 || e.Host.Length > 253 || Uri.CheckHostName(e.Host) == UriHostNameType.Unknown || e.Host.StartsWith('-')) throw new ArgumentException("维护入口无效或未就绪。");
  var expected = new UriBuilder("http", e.Host, e.Port) { Path = "/" }.Uri;
  if (e.Service == "web" && (!Uri.TryCreate(e.Url, UriKind.Absolute, out var url) || url != expected)) throw new ArgumentException("Web 地址与维护入口不一致。");
 }
}
