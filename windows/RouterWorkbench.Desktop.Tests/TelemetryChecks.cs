using System.Net;
using System.Net.Http;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;
namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static async Task NativeTelemetryChecks(MainWindow window,TestProbe peer)
    {
        static object M(string name,string value,string unit="text",string entity="")=>new {name,value,unit,entity,status="ok",interval_seconds=5};
        await peer.ReportAsync("hardware",new Dictionary<string,object>{["cpu_model"]=M("CPU 型号","ARM Cortex-A53"),["cpu_arch"]=M("CPU 架构","ARMv8-A"),["cpu_hardware_bits"]=M("CPU 硬件位数","64","bits"),["kernel_arch"]=M("内核架构","armv7l"),["probe_bits"]=M("Probe 位数","32","bits")});
        await peer.ReportAsync("memory",new Dictionary<string,object>{["memory_usage"]=M("内存使用率","50","percent"),["memory_used_bytes"]=M("已用内存","1073741824","bytes"),["memory_total_bytes"]=M("总内存","2147483648","bytes")});
        await peer.ReportAsync("disk",new Dictionary<string,object>{["disk_abc_usage"]=M("使用率","25","percent","/overlay"),["disk_abc_used_bytes"]=M("已用","1073741824","bytes","/overlay"),["disk_abc_total_bytes"]=M("总量","4294967296","bytes","/overlay"),["disk_abc_available_bytes"]=M("可用","3221225472","bytes","/overlay"),["disk_abc_kind"]=M("类型","overlay","text","/overlay")});
        await peer.ReportAsync("network",new Dictionary<string,object>{["net_65746830_rx_bytes_per_sec"]=M("接收","1048576","bytes_per_sec","eth0"),["net_65746830_tx_bytes_per_sec"]=M("发送","2048","bytes_per_sec","eth0"),["net_65746830_state"]=M("状态","up","text","eth0"),["net_65746830_kind"]=M("类型","物理接口","text","eth0")});
        var storage=Field<System.Windows.Controls.DataGrid>(window,"storageMetrics");var network=Field<System.Windows.Controls.DataGrid>(window,"networkMetrics");
        await Eventually(()=>Task.FromResult(storage.Items.Count==1&&network.Items.Count==1),"WS notification refreshes native disk/network tables");
        Check(((MonitorTableRow)storage.Items[0]).B=="1 GB / 4 GB"&&((MonitorTableRow)storage.Items[0]).C=="3 GB","native mounted storage capacity");Check(((MonitorTableRow)network.Items[0]).A=="1 MB/s"&&((MonitorTableRow)network.Items[0]).B=="2 KB/s","native receive/transmit rates");
        var rows=Field<PropertyRow[]>(window,"propertyRows");Check(rows.Any(r=>r.Value.Contains("ARMv8-A"))&&rows.Any(r=>r.Value.Contains("armv7l"))&&rows.Any(r=>r.Key=="source_ip"&&r.Value=="127.0.0.1"),"native CPU details and server-observed IP");
        Check(rows.Single(r=>r.Key=="memory_usage").Value=="50%（1 GB/2 GB）"&&rows.Single(r=>r.Key=="memory_usage").GroupId=="builtin_resources","memory usage shows real capacities in resource group");
        Invoke(window,"Navigate","overview");var tabs=Field<System.Windows.Controls.TabControl>(window,"overviewTabs");tabs.SelectedItem=tabs.Items.Cast<System.Windows.Controls.TabItem>().Single(t=>t.Header.ToString()=="资源监控");await Task.Delay(100);Render(window,"resource-monitoring.png");tabs.SelectedIndex=0;
        var selected=(MonitorTableRow)network.Items[0];network.SelectedItem=selected;
        await peer.ReportAsync("network",new Dictionary<string,object>{["net_65746830_rx_bytes_per_sec"]=M("接收","2097152","bytes_per_sec","eth0"),["net_65746830_tx_bytes_per_sec"]=M("发送","4096","bytes_per_sec","eth0"),["net_65746830_state"]=M("状态","up","text","eth0"),["net_65746830_kind"]=M("类型","物理接口","text","eth0")});
        await Eventually(()=>Task.FromResult(selected.A=="2 MB/s"),"native telemetry updates existing rate row");Check(ReferenceEquals(network.SelectedItem,selected),"network selection preserved across samples");
        await NativeMonitoringV2Checks(window,peer);
    }
    private sealed class LocationHandler(Func<HttpRequestMessage,CancellationToken,Task<HttpResponseMessage>> respond) : HttpMessageHandler
    { protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request,CancellationToken token)=>respond(request,token); }
    private static async Task TelemetryChecks()
    {
        await MonitoringV2Checks();
        foreach(var ip in new[]{"10.1.1.1","127.0.0.1","192.168.1.1","172.16.0.1","100.64.1.1","169.254.1.1","192.0.2.1","198.51.100.1","203.0.113.2","224.0.0.1","240.0.0.1","::1","fc00::1","fe80::1","2001:db8::1","::ffff:192.168.1.1","not-an-IP"})
            Check(!IpLocation.IsPublic(ip),"no public lookup for "+ip);
        Check(IpLocation.IsPublic("8.8.8.8")&&IpLocation.IsPublic("2001:4860:4860::8888")&&IpLocation.IsPublic("::ffff:8.8.8.8"),"public v4/v6 classification");
        var calls=0;
        using var lookup=new IpLocation(new LocationHandler((request,token)=>{calls++;Check(request.RequestUri!.Scheme=="https"&&request.RequestUri.Host=="ipwho.is"&&request.RequestUri.AbsolutePath=="/8.8.8.8","only target public IP sent over HTTPS");return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK){Content=new StringContent("{\"success\":true,\"ip\":\"8.8.8.8\",\"country\":\"美国\",\"region\":\"加利福尼亚\",\"city\":\"山景城\",\"connection\":{\"isp\":\"Google\"}}")});}));
        Check((await lookup.LookupAsync("192.168.1.1",default)).Contains("内网")&&calls==0,"private address never sent");
        Check((await lookup.LookupAsync("8.8.8.8",default)).Contains("山景城"),"location parsed");await lookup.LookupAsync("8.8.8.8",default);Check(calls==1,"location cache prevents repeated network requests");
        using var failing=new IpLocation(new LocationHandler((request,token)=>Task.FromResult(new HttpResponseMessage(HttpStatusCode.TooManyRequests))));Check((await failing.LookupAsync("8.8.4.4",default)).Contains("未获取"),"rate limit degrades to unavailable");
        using var canceled=new IpLocation(new LocationHandler(async(request,token)=>{await Task.Delay(10000,token);return new HttpResponseMessage(HttpStatusCode.OK);}));using var cts=new CancellationTokenSource(20);try{await canceled.LookupAsync("1.1.1.1",cts.Token);throw new Exception("cancellation lost");}catch(OperationCanceledException){Check(true,"location cancellation observed");}
        Check(TelemetryPresentation.Size(2147483648)=="2 GB"&&TelemetryPresentation.Size(1048576)=="1 MB","capacity adaptive units");
        var metric=new Metric("CPU","42.5","percent","ok","",1,"template","template",DateTimeOffset.UtcNow,false);
        Check(TelemetryPresentation.Value(metric,true)=="42.5%"&&TelemetryPresentation.Value(metric,false)=="42.5%"&&TelemetryPresentation.Tip(metric,false).Contains("离线"),"metric percent and offline state");Check(TelemetryPresentation.Tip(metric with{SampledAt=DateTimeOffset.UtcNow.AddMinutes(-1)},true).Contains("过期"),"stale sample marked");
    }
}
