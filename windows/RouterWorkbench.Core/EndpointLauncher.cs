using System.Diagnostics;
using System.Text.RegularExpressions;

namespace RouterWorkbench.Core;

public static partial class EndpointLauncher
{
    public static ProcessStartInfo Build(Endpoint endpoint, ServerProfile profile)
    {
        if (endpoint.State == "closed") throw new InvalidOperationException("维护入口已关闭，请刷新。");
        if (endpoint.Port is < 1 or > 65535 || Uri.CheckHostName(endpoint.Host) == UriHostNameType.Unknown || endpoint.Host.StartsWith('-'))
            throw new InvalidDataException("API 返回了无效入口地址。");
        if (endpoint.Service == "web")
        {
            var uri = new UriBuilder("http", endpoint.Host, endpoint.Port) { Path = "/" }.Uri;
            if (!Uri.TryCreate(endpoint.Url, UriKind.Absolute, out var supplied) || supplied != uri)
                throw new InvalidDataException("API Web 入口 URL 与地址不一致。");
            return new ProcessStartInfo(uri.AbsoluteUri) { UseShellExecute = true };
        }
        var ssh = endpoint.Service == "ssh";
        if (!ssh && endpoint.Service != "telnet") throw new ArgumentException("不支持的服务");
        var putty = ssh ? profile.SshUsePutty : profile.TelnetUsePutty;
        var executable = ssh ? profile.SshExecutable : profile.TelnetExecutable;
        if (string.IsNullOrWhiteSpace(executable))
        {
            if (putty) throw new ArgumentException("请在连接设置中指定 PuTTY.exe 路径。");
            executable = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.System), ssh ? @"OpenSSH\ssh.exe" : "telnet.exe");
        }
        if (!Path.IsPathFullyQualified(executable) || !File.Exists(executable) || !string.Equals(Path.GetExtension(executable), ".exe", StringComparison.OrdinalIgnoreCase))
            throw new FileNotFoundException(ssh ? "未找到 SSH 客户端，请启用 Windows OpenSSH 或在设置中选择 PuTTY。" : "未找到 Telnet 客户端，请在设置中选择已有 PuTTY 或启用 Windows Telnet Client。", executable);
        var info = new ProcessStartInfo(executable) { UseShellExecute = false, CreateNoWindow = false };
        if (putty)
        {
            info.ArgumentList.Add(ssh ? "-ssh" : "-telnet");
            info.ArgumentList.Add("-P"); info.ArgumentList.Add(endpoint.Port.ToString(System.Globalization.CultureInfo.InvariantCulture));
        }
        else if (ssh) { info.ArgumentList.Add("-p"); info.ArgumentList.Add(endpoint.Port.ToString(System.Globalization.CultureInfo.InvariantCulture)); }
        if (ssh)
        {
            if (!UserName().IsMatch(profile.SshUser)) throw new ArgumentException("SSH 用户名只能包含字母、数字、点、下划线和连字符，且不能以连字符开头。");
            info.ArgumentList.Add("-l"); info.ArgumentList.Add(profile.SshUser);
        }
        info.ArgumentList.Add(endpoint.Host);
        if (!putty && !ssh) info.ArgumentList.Add(endpoint.Port.ToString(System.Globalization.CultureInfo.InvariantCulture));
        return info;
    }
    public static void Open(Endpoint endpoint, ServerProfile profile)
    {
        using var process = Process.Start(Build(endpoint, profile));
    }
    [GeneratedRegex(@"^[A-Za-z0-9_][A-Za-z0-9_.-]{0,63}$")]
    private static partial Regex UserName();
}
