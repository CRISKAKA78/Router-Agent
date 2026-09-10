using System.Net;
using System.Text.Json;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop.Tests;

internal static partial class Program
{
    private static async Task NullableSessionChecks()
    {
        const string valid = "\"started_at\":\"2026-09-10T01:00:00Z\",\"last_seen_at\":\"2026-09-10T01:01:00Z\",";
        string Session(string times) => "{\"session_id\":\"fixture-session\",\"registration\":{\"device_id\":\"fixture\",\"capabilities\":[]}," + times + "\"ended_at\":null,\"end_reason\":\"\"}";
        string Device(string session) => "{\"device_id\":\"fixture\",\"registration\":{\"device_id\":\"fixture\",\"capabilities\":[]},\"status\":\"offline\",\"current_session\":null,\"latest_session\":" + session + "}";
        foreach (var times in new[] { "\"started_at\":null,\"last_seen_at\":null,", "", valid })
        {
            using var api = new ApiClient(new Uri("http://localhost"), default, new DelegateHandler((_, _) => Task.FromResult(Json(
                "{\"data\":{\"items\":[" + Device(Session(times)) + "," + Device(Session(valid)) + "],\"total\":2,\"limit\":200,\"offset\":0}}"))));
            var rows = await api.ListAsync<Client.Device>("devices?admission=all");
            Check(rows.Length == 2 && rows[1].LatestSession!.LastSeenAt == DateTimeOffset.Parse("2026-09-10T01:01:00Z"), "session time parsing retains the entire device page and valid timestamps");
            Check(times == valid ? rows[0].LatestSession!.StartedAt != null : rows[0].LatestSession is { StartedAt: null, LastSeenAt: null, StartedText: "—" }, "null or absent session times remain unknown rather than rejecting the snapshot or inventing dates");
        }
        var current = JsonSerializer.Deserialize<Client.Device>(Device(Session("\"started_at\":null,\"last_seen_at\":null,")).Replace("\"current_session\":null", "\"current_session\":" + Session("\"started_at\":null,\"last_seen_at\":null,")), ApiJson.Options)!;
        Check(current.CurrentSession is { StartedAt: null, LastSeenAt: null }, "nullable times also work for a current session");
        try { JsonSerializer.Deserialize<DeviceSession>(Session("\"started_at\":\"invalid-time\","), ApiJson.Options); throw new Exception("Invalid timestamp accepted"); }
        catch (JsonException) { Check(true, "malformed non-null session timestamps are still rejected"); }
    }
}
