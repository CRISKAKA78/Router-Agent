using System.IO;
using System.Text.Json;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static async Task RemoteDirectoryFailureChecks()
    {
        foreach (var stderr in new[] { "", "   ", "Permission denied" }) {
            var posts = 0;
            using var handler = new DelegateHandler((request, _) => {
                if (request.Method == System.Net.Http.HttpMethod.Post) {
                    posts++;
                    return Task.FromResult(Json("{\"data\":{\"task_id\":\"directory-failure\"}}"));
                }
                return Task.FromResult(Json(JsonSerializer.Serialize(new { data = new {
                    task_id = "directory-failure", device_id = "router", type = "exec", state = "failed",
                    created_at = DateTimeOffset.UtcNow, command = "", cwd = "", timeout_seconds = 10,
                    dispatch_count = 1, result = new { status = "failed", exit_code = 2, stdout = "", stderr, truncated = false }
                }})));
            });
            await using var connection = new WorkspaceConnection(new Uri("http://localhost:18080"), handler);
            try { await RemoteDirectory.ReadAsync(connection, "router", "/tmp/root", default); throw new Exception("directory failure was hidden"); }
            catch (IOException e) {
                Check(string.IsNullOrWhiteSpace(stderr)
                    ? e.Message.Contains("failed") && e.Message.Contains("退出码 2") && e.Message.Contains("directory-failure")
                    : e.Message.Contains(stderr), "directory error retains stderr or reports status, exit code and original task identity");
                Check(posts == 1, "directory failure does not automatically create a replacement task");
            }
        }
    }
}
