using System.Globalization;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Threading;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public sealed class PortRatePanel : UserControl
{
    private sealed class History {
        public RateSeries Rx {get;}=new();public RateSeries Tx {get;}=new();
        public string? Session,Source;public double Elapsed;
    }
    private readonly Dictionary<string,History> history=[];
    private readonly DockPanel panel=new();
    private readonly DispatcherTimer timer=new(){Interval=TimeSpan.FromSeconds(1)};
    private Device? device;private string key="",name="",scope="";
    private RatePlot? plot;
    public PortRatePanel(){
        MinHeight=210;
        Content=panel;
        timer.Tick+=(_,_)=>Refresh();Loaded+=(_,_)=>timer.Start();Unloaded+=(_,_)=>timer.Stop();
    }
    public void Update(Device? value,string identity,string selected,string label){
        if(scope!=identity){history.Clear();scope=identity;key="";if(plot!=null)panel.Children.Remove(plot);plot=null;}
        device=value;name=label;
        if(key!=selected||plot==null){key=selected;if(plot!=null)panel.Children.Remove(plot);plot=null;
            if(key.Length>0){if(!history.TryGetValue(key,out var h)){if(history.Count>=64)history.Clear();h=new();history[key]=h;}plot=new(h.Rx,h.Tx,true){Margin=new(12,0,12,4)};panel.Children.Add(plot);}}
        Refresh();
    }
    private void Refresh(){
        if(key.Length==0||device==null||!history.TryGetValue(key,out var h)){return;}
        var now=DateTimeOffset.UtcNow;
        Metric? M(string f)=>device.EffectiveMetrics?.GetValueOrDefault(key+"_"+f);
        var source=M("counter_source")?.Value;var session=device.CurrentSession?.SessionId;
        var elapsed=M("elapsed_seconds");var next=elapsed is {Status:"ok"}&&double.TryParse(elapsed.Value,CultureInfo.InvariantCulture,out var seconds)?seconds:h.Elapsed;
        if(h.Session!=session||h.Source!=source||next<h.Elapsed){h.Rx.Gap(now);h.Tx.Gap(now);}h.Session=session;h.Source=source;h.Elapsed=next;
        h.Rx.Observe(M("rx_bytes_per_sec"),device.Online,now);h.Tx.Observe(M("tx_bytes_per_sec"),device.Online,now);
        plot?.InvalidateVisual();
    }
}

public static class PhysicalPorts
{
    public static string Rate(Metric m,bool online)=>!online||m.Status!="ok"||TelemetryPresentation.Stale(m)||!double.TryParse(m.Value,NumberStyles.Float,CultureInfo.InvariantCulture,out var n)||!double.IsFinite(n)||n<0?"—":(n*8/1000000).ToString("0.000",CultureInfo.InvariantCulture)+" Mbps";
}
