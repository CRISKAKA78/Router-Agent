using System.Text;
using System.Text.Json;
using RouterWorkbench.Core;

namespace RouterWorkbench.Tests;

// Optional loopback integration against an isolated OpenSSH / BusyBox test container.
internal static class TerminalServiceChecks
{
    public static async Task RunAsync(string fixtureDirectory)
    {
        var root=Path.GetFullPath(fixtureDirectory);
        foreach(var service in new[]{"ssh","telnet"})
        {
            var port=service=="ssh"?20222:20223;
            var client=EmbeddedTerminal.Client(new(service,"127.0.0.1",port,$"127.0.0.1:{port}","ready",null),new());
            if(service=="ssh")
            {
                // Trust only the key copied directly from our container, never modify user known_hosts.
                foreach(var option in new[]{"UserKnownHostsFile="+Path.Combine(root,"known_hosts"),"StrictHostKeyChecking=yes"})
                {client.ArgumentList.Insert(0,option);client.ArgumentList.Insert(0,"-o");}
            }
            await using var terminal=new EmbeddedTerminal(client,80,24);
            var output=new StringBuilder();
            async Task Until(string expected)
            {
                var deadline=DateTime.UtcNow.AddSeconds(15);
                while(DateTime.UtcNow<deadline)
                {
                    var value=JsonSerializer.SerializeToElement(terminal.Read());
                    output.Append(Encoding.UTF8.GetString(Convert.FromBase64String(value.GetProperty("base64").GetString()!)));
                    if(output.ToString().Contains(expected,StringComparison.Ordinal))return;
                    if(value.GetProperty("exited").GetBoolean())break;
                    await Task.Delay(30);
                }
                // Fixture output contains no production credentials. Do not print terminal transcripts.
                throw new Exception(service+" did not produce expected terminal output: "+expected);
            }
            if(service=="ssh")
            {
                await Until("password:");
                terminal.Write((await File.ReadAllTextAsync(Path.Combine(root,"password"))).Trim()+"\r");
            }
            await Until("#");
            output.Clear();
            terminal.Write($"printf 'conpty-%s\\n' '{service}-ok'\r");
            await Until($"conpty-{service}-ok");
            Console.WriteLine($"PASS ConPTY {service} real service command round trip");
            if(service=="ssh")
            {
                terminal.Resize(100,40);
                output.Clear();terminal.Write("stty size\r");await Until("40 100");
                Console.WriteLine("PASS SSH remote PTY receives window resize");
            }
            terminal.Write("exit\r");
        }
    }
}
