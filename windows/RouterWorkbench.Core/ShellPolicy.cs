namespace RouterWorkbench.Core;
public static class ShellPolicy
{
 public static void ValidateProfile(ServerProfile next, ServerProfile previous) {
  next.BaseUri();
  if (next.SchemaVersion != 1 || next.Theme is not ("Default" or "Light" or "Dark")) throw new ArgumentException("配置无效。");
  if (next.SshExecutable != previous.SshExecutable || next.TelnetExecutable != previous.TelnetExecutable || next.SshUsePutty != previous.SshUsePutty || next.TelnetUsePutty != previous.TelnetUsePutty) throw new ArgumentException("客户端路径必须通过 Windows 文件选择器修改。");
 }
 public static void ValidateEndpoint(Endpoint e) {
  if (e.State != "ready" || e.Service is not ("web" or "ssh" or "telnet") || e.Port is < 1 or > 65535 || e.Host.Length > 253 || Uri.CheckHostName(e.Host) == UriHostNameType.Unknown || e.Host.StartsWith('-')) throw new ArgumentException("维护入口无效或未就绪。");
  var expected = new UriBuilder("http", e.Host, e.Port) { Path = "/" }.Uri;
  if (e.Service == "web" && (!Uri.TryCreate(e.Url, UriKind.Absolute, out var url) || url != expected)) throw new ArgumentException("Web 地址与维护入口不一致。");
 }
}
