using System.Globalization;
using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;
public static class TelemetryPresentation
{
    public static string Architecture(Device d) => d.EffectiveMetrics?.GetValueOrDefault("cpu_arch") is {Status:"ok"} m ? m.Value : d.Registration.Arch;
    public static string Size(double value)
    {
        var units=new[]{"B","KB","MB","GB","TB","PB","EB"};var n=0;
        while(value>=1024&&n<units.Length-1){value/=1024;n++;}
        return value.ToString(n==0?"0":"0.##",CultureInfo.InvariantCulture)+" "+units[n];
    }
    public static string Value(Metric m,bool online)
    {
        if(m.Status!="ok")return "—";
        if(double.TryParse(m.Value,NumberStyles.Float,CultureInfo.InvariantCulture,out var n)&&double.IsFinite(n))
            return m.Unit switch {"percent"=>n.ToString("0.##",CultureInfo.InvariantCulture)+"%","bytes"=>Size(n),"bytes_per_sec"=>Size(n)+"/s",
                "mhz"=>n>=1000?(n/1000).ToString("0.##",CultureInfo.InvariantCulture)+" GHz":n.ToString("0.##",CultureInfo.InvariantCulture)+" MHz",
                "bits"=>n+" 位","seconds"=>long.TryParse(m.Value,out var seconds)?DeviceProperties.FormatUptime(seconds):"—",_=>DeviceProperties.Text(m.Value)};
        return DeviceProperties.Text(m.Value);
    }
    public static bool Stale(Metric m) => m.Stale||m.IntervalSeconds>0&&DateTimeOffset.UtcNow-m.SampledAt>TimeSpan.FromSeconds(Math.Max(15,m.IntervalSeconds*3));
    public static string Tip(Metric m,bool online)
    {
        var parts=new List<string>();
        if(m.Status!="ok")parts.Add(!string.IsNullOrEmpty(m.Reason)?DeviceProperties.Reason(m.Reason):m.Status switch{"waiting"=>"等待有效采样；速率需要两次采样","error"=>"采集失败或输出无效",_=>"设备未提供该参数"});
        if(!online)parts.Add("设备离线，显示最后采样");else if(Stale(m))parts.Add("采样已过期");
        return string.Join("；",parts);
    }
    public static IEnumerable<PropertyRow> Rows(Device d)
    {
        foreach(var (key,m) in (d.EffectiveMetrics??[]).OrderBy(p=>p.Value.Group).ThenBy(p=>p.Value.Entity).ThenBy(p=>p.Key)){
            if(m.Group=="egress")continue;
            var group=m.Group switch{"hardware"=>"CPU 详情","cpu"=>"CPU","memory"=>"内存","disk"=>"存储空间","network"=>"网络接口",_=>"模板属性"};
            var entity=m.Entity;
            var value = key == "memory_usage" ? MemoryUsage(d, m) : Value(m, d.Online);
            var name = m.Source == "template" ? m.Name : RouterWorkbench.Display.DevicePresentation.Builtins.GetValueOrDefault(key, m.Name);
            yield return new(group,$"{name}{(entity.Length>0?" · "+entity:"")}",value,Tip(m,d.Online)){Key=key,MetricGroup=m.Source=="template"?"template":m.Group};
        }
    }
    internal static string MemoryUsage(Device device, Metric usage)
    {
        var percent = Value(usage, device.Online);
        if (usage.Status != "ok") return percent;
        string Capacity(string key) => device.EffectiveMetrics?.GetValueOrDefault(key) is { Status: "ok", Unit: "bytes" } metric ? Value(metric, device.Online) : "—";
        return $"{percent}（{Capacity("memory_used_bytes")}/{Capacity("memory_total_bytes")}）";
    }
}
public sealed class MonitorTableRow(string key,string entity,string[] cells,string[]? tips=null) : System.ComponentModel.INotifyPropertyChanged
{
    public string Key {get;}=key;
    public string Entity {get;private set;}=entity;
    private string Cell(int n)=>n<cells.Length?cells[n]:"—";
    private string Tip(int n)=>tips!=null&&n<tips.Length?tips[n]:"设备未提供该参数";
    public string A=>Cell(0);public string B=>Cell(1);public string C=>Cell(2);public string D=>Cell(3);public string E=>Cell(4);public string F=>Cell(5);public string G=>Cell(6);public string H=>Cell(7);
    public string ATip=>Tip(0);public string BTip=>Tip(1);public string CTip=>Tip(2);public string DTip=>Tip(3);public string ETip=>Tip(4);public string FTip=>Tip(5);public string GTip=>Tip(6);public string HTip=>Tip(7);public string EntityTip=>Entity;
    public event System.ComponentModel.PropertyChangedEventHandler? PropertyChanged;
    public void Update(MonitorTableRow next){Entity=next.Entity;cells=next.Cells;tips=next.Tips;PropertyChanged?.Invoke(this,new(null));}
    private string[] Cells=>cells;private string[]? Tips=>tips;
}
public partial class MainWindow
{
    private System.Windows.Controls.DataGrid storageMetrics=null!,networkMetrics=null!;
    private MonitorTableRow[] storageRows=[],networkRows=[];
    private string monitoringDevice="";
    private readonly List<NetworkRateWindow> networkCharts=[];
    private void OpenNetworkChart(MonitorTableRow row)
    {
        if(Device==null)return;
        var existing=networkCharts.FirstOrDefault(w=>w.InterfaceKey==row.Key);
        if(existing!=null){existing.Activate();return;}
        var window=new NetworkRateWindow(this,row.Key,row.Entity);networkCharts.Add(window);
        window.Closed+=(_,_)=>networkCharts.Remove(window);window.Update(Device);window.Show();
    }
    private void UpdateMonitorTables(Device? d)
    {
        var metrics=d?.EffectiveMetrics??[];
        string V(string key)=>metrics.TryGetValue(key,out var m)?TelemetryPresentation.Value(m,d?.Online==true):"—";
        string T(string key)=>metrics.TryGetValue(key,out var m)?TelemetryPresentation.Tip(m,d?.Online==true):"设备未提供该参数";
        string Time(string prefix)=>Labels.Time(metrics.Where(p=>p.Key.StartsWith(prefix+"_",StringComparison.Ordinal)).Select(p=>(DateTimeOffset?)p.Value.SampledAt).Max());
        var disks=metrics.Where(p=>p.Key.StartsWith("disk_",StringComparison.Ordinal)&&p.Key.EndsWith("_total_bytes",StringComparison.Ordinal)).OrderBy(p=>p.Value.Entity).Select(p=>{
            var k=p.Key[..^12];return new MonitorTableRow(k,p.Value.Entity,[V(k+"_usage"),V(k+"_used_bytes")+" / "+V(k+"_total_bytes"),V(k+"_available_bytes"),V(k+"_kind"),Time(k)],
                [T(k+"_usage"),T(k+"_used_bytes")+" "+T(k+"_total_bytes"),T(k+"_available_bytes"),T(k+"_kind"),""]);
        }).ToArray();
        var nets=metrics.Where(p=>p.Key.StartsWith("net_",StringComparison.Ordinal)&&p.Key.EndsWith("_rx_bytes_per_sec",StringComparison.Ordinal)).OrderBy(p=>p.Value.Entity).Select(p=>{
            var k=p.Key[..^17];var fields=new[]{"rx_bytes_per_sec","tx_bytes_per_sec","state","kind","","rx_bytes","tx_bytes","elapsed_seconds"};
            return new MonitorTableRow(k,p.Value.Entity,fields.Select(f=>f==""?Time(k):V(k+"_"+f)).ToArray(),fields.Select(f=>f==""?"":T(k+"_"+f)).ToArray());
        }).ToArray();
        bool same=monitoringDevice==(d?.DeviceId??"");monitoringDevice=d?.DeviceId??"";
        if(!same)foreach(var window in networkCharts.ToArray())window.Close();
        foreach(var window in networkCharts)window.Update(d);
        void Update(System.Windows.Controls.DataGrid grid,ref MonitorTableRow[] rows,MonitorTableRow[] next){if(same&&rows.Select(r=>r.Key).SequenceEqual(next.Select(r=>r.Key))){for(int i=0;i<rows.Length;i++)rows[i].Update(next[i]);}else{rows=next;Ui.SetRows(grid,rows);}}
        UpdateSwitchTable(d);
 Update(storageMetrics,ref storageRows,disks);Update(networkMetrics,ref networkRows,nets);
    }
    private readonly IpLocation locations;
    private CancellationTokenSource? locationCancel;
    private Task locationWork=Task.CompletedTask;
    private string locationKey="";
    private DateTimeOffset locationRetry;
    private readonly Dictionary<string,IpLocationResult> locationResults=[];
    private static string Address(Device d,string family)=>d.EffectiveMetrics?.GetValueOrDefault("egress_"+family) is {Status:"ok"} m?m.Value:"";
    private IpLocationResult Location(string? ip)=>string.IsNullOrEmpty(ip) ? new("—","—","尚未获取 IP") : locationResults.GetValueOrDefault(ip)??new("—","—","正在查询归属地与运营商");
    private IEnumerable<PropertyRow> LocationRows(Device d)
    {
        yield return new("连接","连接来源 IP",DeviceProperties.Text(d.SourceIp),string.IsNullOrEmpty(d.SourceIp)?"服务器未提供连接来源":""){Key="source_ip"};
        foreach(var family in new[]{"ipv4","ipv6"}){
            var address=Address(d,family);var metric=d.EffectiveMetrics?.GetValueOrDefault("egress_"+family);var result=Location(address);var name=family=="ipv4"?"IPv4":"IPv6";
            yield return new("出口",name,DeviceProperties.Text(address),metric==null?"尚无出口采样：请检查采集开关与采样状态":TelemetryPresentation.Tip(metric,d.Online)){Key="egress_"+family,MetricGroup="egress"};
            yield return new("出口",name+" 归属地",result.Place,result.Reason){Key="egress_"+family+"_location",MetricGroup="egress"};
            yield return new("出口",name+" 运营商",result.Isp,result.Reason){Key="egress_"+family+"_isp",MetricGroup="egress"};
        }
    }
    private string SourceLocationSummary(Device d)
    {
        var result = Location(d.SourceIp);
        return result.Isp == "—" && result.Place == "—" ? "—" : result.Isp + " / " + result.Place;
    }
    private void UpdateLocation(Device? device)
    {
        var addresses=device==null?[]:new[]{device.SourceIp,Address(device,"ipv4"),Address(device,"ipv6")}.OfType<string>().Where(s=>s.Length>0).Distinct().ToArray();
        var key=device==null?"":$"{device.DeviceId}|{device.CurrentSession?.SessionId??device.LatestSession?.SessionId}|{string.Join('|',addresses)}";
        if(key==locationKey&&(DateTimeOffset.UtcNow<locationRetry||!locationWork.IsCompleted))return;
        locationCancel?.Cancel();locationKey=key;locationRetry=DateTimeOffset.UtcNow.AddMinutes(5);locationResults.Clear();
        var previous=locationWork;var oldCancel=locationCancel;var cts=new CancellationTokenSource();locationCancel=cts;
        locationWork=Query();
        async Task Query(){try{
            await previous;oldCancel?.Dispose();
            foreach(var ip in addresses){var result=await locations.LookupDetailsAsync(ip,cts.Token);if(cts.IsCancellationRequested||key!=locationKey||closing)return;locationResults[ip]=result;UpdateOverview();}
        }catch(OperationCanceledException){}}
    }
    private async Task CancelLocation()
    {
        locationCancel?.Cancel();await locationWork;locationCancel?.Dispose();locationCancel=null;locationKey="";locationResults.Clear();
        foreach(var window in networkCharts.ToArray())window.Close();
    }
}
