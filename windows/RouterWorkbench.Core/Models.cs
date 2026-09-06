using System.Text.Json;
namespace RouterWorkbench.Core;
public static class Wire { public static readonly JsonSerializerOptions Json = new() { PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower, UnmappedMemberHandling = System.Text.Json.Serialization.JsonUnmappedMemberHandling.Disallow }; }
// Platform launch argument; the sole business API client and DTOs live in TypeScript.
public sealed record Endpoint(string Service, string Host, int Port, string Address, string State, string? Url);
