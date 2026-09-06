using System.Diagnostics;
using System.Net;
using System.Text.Json;
using RouterWorkbench.Core;
namespace RouterWorkbench.Tests;
internal static class Program
{
 static void Assert(bool value, string label) { if (!value) throw new Exception(label); Console.WriteLine("PASS " + label); }
 static void Reject(Action f, string label) { try { f(); } catch (ArgumentException) { Assert(true, label); return; } throw new Exception(label); }
 static async Task Main(string[] args) {
  if(args.FirstOrDefault()=="--terminal-services"){await TerminalServiceChecks.RunAsync(args[1]);return;}
  var origin = new Uri("http://127.0.0.1:18080/"); var root = Path.GetFullPath("frontend/dist");
  Assert(ShellPolicy.IsDocument("http://127.0.0.1:18080/__workbench/index.html#tasks", origin), "local SPA document");
  foreach (var url in new[] { "https://evil.test/__workbench/index.html", "http://127.0.0.1:18080/api/v1/devices", "http://127.0.0.1:18080/__workbench/index.html?external=1", "file:///C:/test.html" }) Assert(!ShellPolicy.IsDocument(url, origin), "reject navigation " + url);
  Assert(ShellPolicy.LocalResource(new Uri(origin, "__workbench/index.html"), origin, root) == Path.Combine(root, "index.html"), "local path mapping");
  Assert(ShellPolicy.LocalResource(new Uri(origin, "__workbench/%2e%2e%5csecret"), origin, root) == null, "reject path traversal");
  Assert(ShellPolicy.Parse("{\"id\":\"1\",\"method\":\"getProfile\",\"args\":{}}").Method == "getProfile", "allow profile bridge");
  foreach (var method in new[] { "exec", "readFile", "writeFile", "Process.Start", "shell" }) Reject(() => ShellPolicy.Parse(JsonSerializer.Serialize(new { id = "1", method, args = new { } })), "deny bridge " + method);
  Reject(() => ShellPolicy.Parse("{\"id\":\"1\",\"method\":\"getProfile\",\"args\":{},\"id\":\"2\"}"), "reject duplicate properties");
  Reject(() => ShellPolicy.ValidateProfile(new ServerProfile { SshExecutable = "C:/arbitrary.exe" }, new()), "reject script-selected executable");
  Reject(() => ShellPolicy.ValidateEndpoint(new("ssh", "-proxycommand", 22, "", "ready", null)), "reject option injection");
  Reject(() => ShellPolicy.ValidateEndpoint(new("web", "localhost", 80, "", "ready", "file:///secret")), "reject external URI schemes");
  var launch = EndpointLauncher.Build(new("web", "127.0.0.1", 32000, "127.0.0.1:32000", "ready", "http://127.0.0.1:32000/"), new());
  Assert(launch.UseShellExecute && launch.FileName == "http://127.0.0.1:32000/", "system browser launch");
  var systemSsh = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.System), "OpenSSH/ssh.exe");
  if (File.Exists(systemSsh)) { var ssh = EndpointLauncher.Build(new("ssh", "127.0.0.1", 32001, "", "ready", null), new()); Assert(ssh.ArgumentList.SequenceEqual(new[] { "-p", "32001", "-l", "root", "127.0.0.1" }), "SSH independent arguments"); }
  await VerifyTerminalAsync();
  if (args.Length == 0) return;
  var saveRoot=Path.Combine(Path.GetTempPath(),"workbench-save-test-"+Guid.NewGuid().ToString("N"));Directory.CreateDirectory(saveRoot);
  var saved=Path.Combine(saveRoot,"result.txt");
  await using(var save=new PickedFileSave(saved,3,"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")){
   await save.WriteAsync(0,"YWJj",CancellationToken.None);await save.CompleteAsync(CancellationToken.None);
  }
  Assert(await File.ReadAllTextAsync(saved)=="abc","picked save publishes verified bytes");
  await using(var save=new PickedFileSave(saved,3,new string('0',64))){
   await save.WriteAsync(0,"YWJj",CancellationToken.None);
   try{await save.CompleteAsync(CancellationToken.None);throw new Exception("bad digest accepted");}catch(InvalidDataException){}
  }
  Assert(await File.ReadAllTextAsync(saved)=="abc"&&Directory.GetFiles(saveRoot).Length==1,"failed save preserves target and removes temporary file");
  File.Delete(saved);Directory.Delete(saveRoot);
  var output = Path.GetFullPath(args[1]); Directory.CreateDirectory(output);
  var info = new ProcessStartInfo(Path.GetFullPath(args[0])) { UseShellExecute = false, CreateNoWindow = true };
  foreach (var a in new[] { "-listen", "127.0.0.1:18081", "-http-listen", "127.0.0.1:18080", "-repository-dir", Path.Combine(output,"repository-"+Guid.NewGuid().ToString("N")), "-tunnel-data-listen", "127.0.0.1:0", "-tunnel-port-first", "32200", "-tunnel-port-last", "32299", "-tunnel-port-reuse-delay", "1ms" }) info.ArgumentList.Add(a);
  using var server = Process.Start(info)!; var probe = new TestProbe();
  try {
   using var http = new HttpClient();
   for (var i=0;;i++) { try { await http.GetStringAsync(new Uri(origin,"api/v1/devices")); break; } catch when(i<100) { await Task.Delay(50); } }
   await probe.StartAsync(18081, "ui-device");
   var p = new ServerProfile { ServerUrl = origin.GetLeftPart(UriPartial.Authority), Theme = "Light" }; await p.SaveAsync(Path.Combine(output,"profile.json"));
   using var listener = new HttpListener(); listener.Prefixes.Add("http://127.0.0.1:18082/"); listener.Start();
   await File.WriteAllTextAsync(Path.Combine(output,"fixture-ready.txt"), "ready");
   while(true) { var request = await listener.GetContextAsync(); var path = request.Request.Url!.AbsolutePath;
    if(path=="/replace") { var old = probe; probe = new TestProbe(); await probe.StartAsync(18081,"ui-device"); await old.DisposeAsync(); }
    if(path=="/offline") { await probe.DisposeAsync(); probe = new TestProbe(); }
    request.Response.StatusCode = 200; request.Response.Close(); if(path=="/stop") break;
   }
  } finally { await probe.DisposeAsync(); if(!server.HasExited) { server.Kill(true); await server.WaitForExitAsync(); } }
 }
 static async Task VerifyTerminalAsync() {
  Reject(() => ShellPolicy.Parse("{\"id\":\"1\",\"method\":\"terminalOpen\",\"args\":{\"command\":\"cmd.exe\"}}"), "terminal bridge rejects arbitrary process");
  Reject(() => EmbeddedTerminal.ValidateSize(0,24), "terminal size bounded");
  var client = new ProcessStartInfo(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.System), "cmd.exe"));
  client.ArgumentList.Add("/q");
  await using (var terminal = new EmbeddedTerminal(client,80,24)) {
   terminal.Resize(100,30);
   terminal.Write("chcp 65001\recho terminal-中文-verified\r");
   var output = new MemoryStream(); var deadline = DateTime.UtcNow.AddSeconds(10);
   while (DateTime.UtcNow < deadline) {
    var value=JsonSerializer.SerializeToElement(terminal.Read());output.Write(Convert.FromBase64String(value.GetProperty("base64").GetString()!));
    if(System.Text.Encoding.UTF8.GetString(output.ToArray()).Contains("terminal-中文-verified"))break;
    await Task.Delay(25);
   }
   Assert(System.Text.Encoding.UTF8.GetString(output.ToArray()).Contains("terminal-中文-verified"),"ConPTY UTF-8 input/output and resize");
   terminal.Write("for /l %i in (1,1,5000) do @echo final-tail-%i\rexit\r");
   var exited=false; deadline=DateTime.UtcNow.AddSeconds(10);
   while(DateTime.UtcNow<deadline) {
    var value=JsonSerializer.SerializeToElement(terminal.Read());output.Write(Convert.FromBase64String(value.GetProperty("base64").GetString()!));
    if(value.GetProperty("exited").GetBoolean()){exited=true;break;}
    await Task.Delay(10);
   }
   Assert(exited && System.Text.Encoding.UTF8.GetString(output.ToArray()).Contains("final-tail-5000"),"natural client exit drains final bounded output");
  }
  var flood = new EmbeddedTerminal(client,80,24);
  flood.Write("for /l %i in (1,1,100000) do @echo 012345678901234567890123456789012345678901234567890123456789\r");
  await Task.Delay(300);
  await flood.DisposeAsync().AsTask().WaitAsync(TimeSpan.FromSeconds(10));
  Assert(true,"ConPTY closes with blocked bounded output");
  await using var sessions=new TerminalSessions();
  var ssh=Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.System),"OpenSSH/ssh.exe");
  if(File.Exists(ssh)) {
   var id=await sessions.OpenAsync(new("ssh","127.0.0.1",9,"127.0.0.1:9","ready",null),new(),DateTimeOffset.UtcNow.AddMilliseconds(200),80,24);
   await Task.Delay(1200);
   try {await sessions.InvokeAsync(id);throw new Exception("expired terminal accepted");} catch(ArgumentException){Assert(true,"native lease expiry releases terminal");}
  }
 }
}
