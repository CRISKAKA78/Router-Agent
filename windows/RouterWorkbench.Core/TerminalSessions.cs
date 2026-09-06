namespace RouterWorkbench.Core;

// Owns local process handles, including a native expiry independent of the renderer.
public sealed class TerminalSessions : IAsyncDisposable
{
    private readonly SemaphoreSlim gate = new(1);
    private readonly Dictionary<string, (EmbeddedTerminal Terminal, DateTimeOffset Expires)> sessions = [];
    private readonly CancellationTokenSource lifetime = new();
    private readonly Task reaper;
    public TerminalSessions() { reaper = ReapAsync(); }
    public async Task<string> OpenAsync(Endpoint endpoint, ServerProfile profile, DateTimeOffset expires, int columns, int rows) {
        await gate.WaitAsync(lifetime.Token);
        try {
            if (expires <= DateTimeOffset.UtcNow) throw new ArgumentException("维护租期已到期。");
            if (sessions.Count >= 4) throw new InvalidOperationException("最多同时打开 4 个内置终端。");
            var client = EmbeddedTerminal.Client(endpoint, profile);
            var terminal = await Task.Run(() => new EmbeddedTerminal(client, columns, rows));
            sessions.Add(terminal.Id, (terminal, expires));
            return terminal.Id;
        } finally { gate.Release(); }
    }
    public async Task<object?> InvokeAsync(string id, Action<EmbeddedTerminal>? action = null) {
        await gate.WaitAsync(lifetime.Token);
        try {
            if (!sessions.TryGetValue(id, out var value) || value.Expires <= DateTimeOffset.UtcNow) throw new ArgumentException("终端已关闭或租期已到期。");
            if (action != null) { action(value.Terminal); return null; }
            return value.Terminal.Read();
        } finally { gate.Release(); }
    }
    public async Task CloseAsync(string? id = null) {
        await gate.WaitAsync();
        try {
            var targets = sessions.Where(pair => id == null || pair.Key == id).ToArray();
            foreach (var pair in targets) sessions.Remove(pair.Key);
            await Task.WhenAll(targets.Select(pair => pair.Value.Terminal.DisposeAsync().AsTask()));
        } finally { gate.Release(); }
    }
    private async Task ReapAsync() {
        using var timer = new PeriodicTimer(TimeSpan.FromSeconds(1));
        try {
            while (await timer.WaitForNextTickAsync(lifetime.Token)) {
                await gate.WaitAsync(lifetime.Token);
                try {
                    var expired = sessions.Where(pair => pair.Value.Expires <= DateTimeOffset.UtcNow).ToArray();
                    foreach (var pair in expired) sessions.Remove(pair.Key);
                    await Task.WhenAll(expired.Select(pair => pair.Value.Terminal.DisposeAsync().AsTask()));
                } finally { gate.Release(); }
            }
        } catch (OperationCanceledException) { }
    }
    public async ValueTask DisposeAsync() {
        lifetime.Cancel(); await reaper; await CloseAsync(); lifetime.Dispose(); gate.Dispose();
    }
}
