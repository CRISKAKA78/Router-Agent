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
}
