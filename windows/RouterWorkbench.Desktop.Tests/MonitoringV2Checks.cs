using System.Net;
using System.Net.Http;
using System.Windows.Controls;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;
namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static async Task MonitoringV2Checks()
    {
        var now=DateTimeOffset.UtcNow;var m=new Metric("接收","0","bytes_per_sec","ok","eth0",1,"builtin","network",now,false);
        var series=new RateSeries();series.Observe(m,true,now);series.Observe(m,true,now.AddSeconds(1));
        Check(series.Points.Count==1&&series.Points[0].Value==0,"chart retains real zero and deduplicates snapshots");
        series.Observe(m,false,now.AddSeconds(2));series.Observe(m with{SampledAt=now.AddSeconds(3),Value="20"},true,now.AddSeconds(3));
        Check(series.Points.Count==3&&series.Points[1].Value==null,"offline interval produces chart gap");
        series.Observe(m with{SampledAt=now.AddMinutes(11)},true,now.AddMinutes(11));
        Check(series.Points.All(p=>p.At>=now.AddMinutes(1)),"chart keeps at most ten minutes");
        Check(TelemetryPresentation.Value(m with{Status="error",Reason="timeout"},true)=="—"&&TelemetryPresentation.Tip(m with{Status="error",Reason="timeout"},true).Contains("超时"),"failure reason moved to tooltip");
        using var lookup=new IpLocation(new LocationHandler((request,token)=>Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK){Content=new StringContent("{\"success\":true,\"ip\":\"2001:4860:4860::8888\",\"country\":\"美国\",\"connection\":{\"isp\":\"Google\"}}") })));
        var location=await lookup.LookupDetailsAsync("2001:4860:4860::8888",default);
        Check(location.Place=="美国"&&location.Isp=="Google"&&location.Reason=="","IPv6 lookup separates place and operator");
        using var partial=new IpLocation(new LocationHandler((request,token)=>Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK){Content=new StringContent("{\"success\":true,\"ip\":\"8.8.8.8\",\"country\":\"美国\"}")})));
        var missing=await partial.LookupDetailsAsync("8.8.8.8",default);Check(missing.Place=="美国"&&missing.Isp=="—"&&missing.Reason.Length>0,"partial lookup preserves available value");
    }
    private static async Task NativeMonitoringV2Checks(MainWindow window,TestProbe peer)
    {
        static object M(string name,string value,string unit="text",string entity="eth0")=>new{name,value,unit,entity,status="ok",interval_seconds=1};
        var key="net_65746830";
        var values=new Dictionary<string,object>{[key+"_rx_bytes_per_sec"]=M("接收","1024","bytes_per_sec"),[key+"_tx_bytes_per_sec"]=M("发送","2048","bytes_per_sec"),[key+"_rx_bytes"]=M("累计接收","1048576","bytes"),[key+"_tx_bytes"]=M("累计发送","2097152","bytes"),[key+"_elapsed_seconds"]=M("时长","3661","seconds"),[key+"_state"]=M("状态","up"),[key+"_kind"]=M("类型","物理接口")};
        await peer.ReportAsync("network",values);
        var grid=Field<DataGrid>(window,"networkMetrics");
        await Eventually(()=>Task.FromResult(grid.Items.Count==1&&((MonitorTableRow)grid.Items[0]).F=="1 MB"),"API flow carries cumulative traffic");
        var row=(MonitorTableRow)grid.Items[0];Check(row.G=="2 MB"&&row.H=="1小时1分钟1秒"&&grid.Columns[^1].Header.ToString()=="更新时间","traffic duration and last update columns");
        var quick=(DataGrid)window.FindName("QuickProperties");Check(quick.Items.Count==10&&quick.Items[9].GetType().GetProperty("Name")!.GetValue(quick.Items[9])?.ToString()=="最近心跳","selected-device split IP rows, template version and heartbeat order");
        var devices=(DataGrid)window.FindName("DevicesGrid");Check(devices.Columns.Select(c=>c.Header.ToString()).SequenceEqual(new[]{"设备名","设备ID","状态"}),"device list columns");
        Invoke(window,"OpenNetworkChart",row);var charts=Field<List<NetworkRateWindow>>(window,"networkCharts");Check(charts.Count==1,"double-click action opens interface chart");
        await Task.Delay(100);values[key+"_rx_bytes_per_sec"]=M("接收","4096","bytes_per_sec");await peer.ReportAsync("network",values);
        await Eventually(()=>Task.FromResult(charts[0].Receive.Points.Count(p=>p.Value.HasValue)>=2),"real API samples update chart");
        foreach(var theme in new[]{"Light","Dark"}){Theme.Apply(theme);Render(charts[0],"network-chart-"+theme.ToLowerInvariant()+".png");}
        Theme.Apply("Light");charts[0].Close();Check(charts.Count==0,"closing chart releases registered window");
    }
}
