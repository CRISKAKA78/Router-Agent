using System.ComponentModel;
using System.Diagnostics;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public sealed class ConnectionRow(ConnectionPeriod period) : INotifyPropertyChanged
{
    public ulong Id => period.Id;
    public string StateText => period.State switch { "online" => "在线", "offline" => "离线", _ => "已结束" };
    public string OnlineText => Labels.Time(period.OnlineAt);
    public string OfflineText => Labels.Time(period.OfflineAt);
    public string OnlineDuration => DeviceProperties.FormatUptime(period.OnlineSeconds + (period.State == "online" ? elapsed : 0));
    public string OfflineDuration => DeviceProperties.FormatUptime(period.OfflineSeconds is { } seconds ? seconds + (period.State == "offline" ? elapsed : 0) : null);
    public string ReasonText => period.EndReason switch {
        "" => "—", "disconnected" => "连接断开", "heartbeat_timeout" => "心跳超时", "write_error" => "发送失败",
        "protocol_error" => "协议错误", "server_closed" => "服务器关闭", "requested_disconnect" => "主动断开", _ => period.EndReason
    };
    private long elapsed;
    public void Advance(long seconds)
    {
        if (elapsed == seconds || period.State == "completed") return;
        elapsed = seconds;
        PropertyChanged?.Invoke(this, new(nameof(OnlineDuration)));
        PropertyChanged?.Invoke(this, new(nameof(OfflineDuration)));
    }
    public event PropertyChangedEventHandler? PropertyChanged;
}

public partial class MainWindow
{
    private ConnectionRow[] connectionRows = [];
    private TextBlock historyStatus = null!;
    private long historyTick = Stopwatch.GetTimestamp();
    private TimeSpan historyElapsed;
    private bool historySynchronized;

    private void SetConnectionHistory(ConnectionPeriod[] periods)
    {
        var selected = (sessions.SelectedItem as ConnectionRow)?.Id;
        connectionRows = periods.Select(p => new ConnectionRow(p)).ToArray();
        sessions.ItemsSource = connectionRows;
        sessions.SelectedItem = connectionRows.FirstOrDefault(r => r.Id == selected);
        historyElapsed = TimeSpan.Zero;
        historyTick = Stopwatch.GetTimestamp();
        historySynchronized = connection?.Synchronized == true;
        UpdateConnectionHistoryClock();
    }
    private void ClearConnectionHistory() => SetConnectionHistory([]);
    private void UpdateConnectionHistoryClock()
    {
        if (historyStatus == null) return;
        var now = Stopwatch.GetTimestamp();
        var synchronized = connection?.Synchronized == true;
        if (synchronized && historySynchronized) historyElapsed += Stopwatch.GetElapsedTime(historyTick, now);
        historyTick = now; historySynchronized = synchronized;
        foreach (var row in connectionRows) row.Advance((long)historyElapsed.TotalSeconds);
        historyStatus.Text = !synchronized && connectionRows.Length > 0 ? "服务器连接中断，计时已暂停" : connectionRows.Length == 0 ? "暂无连接记录" : "共 " + connectionRows.Length + " 条连接记录";
    }
}
