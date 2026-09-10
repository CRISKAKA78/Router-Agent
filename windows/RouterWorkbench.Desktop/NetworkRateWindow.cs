using System.Globalization;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Threading;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public sealed record RatePoint(DateTimeOffset At,double? Value);
public sealed class RateSeries
{
    private readonly List<RatePoint> points=[];
    public IReadOnlyList<RatePoint> Points=>points;
    private DateTimeOffset? lastSample;
    private bool valid;
    public void Gap(DateTimeOffset now){if(points.Count>0&&points[^1].Value!=null)points.Add(new(now,null));valid=false;}
    public void Observe(Metric? metric,bool online,DateTimeOffset now)
    {
        var ok=online&&metric is {Status:"ok"}&&!metric.Stale&&
            (metric.IntervalSeconds==0||now-metric.SampledAt<=TimeSpan.FromSeconds(Math.Max(15,3*metric.IntervalSeconds)));
        double n=0;ok=ok&&double.TryParse(metric!.Value,NumberStyles.Float,CultureInfo.InvariantCulture,out n)&&double.IsFinite(n)&&n>=0;
        if(!ok)Gap(now);
        else if(lastSample!=metric!.SampledAt){
            if(lastSample is {} previous&&metric.SampledAt<previous)Gap(now);
            if(valid&&lastSample is {} at&&metric.SampledAt-at>TimeSpan.FromSeconds(Math.Max(15,metric.IntervalSeconds*3)))Gap(at.AddMilliseconds(1));
            points.Add(new(metric.SampledAt,n));lastSample=metric.SampledAt;valid=true;
        }
        points.RemoveAll(p=>p.At<now.AddMinutes(-10));
        if(points.Count>1200)points.RemoveRange(0,points.Count-1200);
    }
}

public sealed class NetworkRateWindow : Window
{
    public string InterfaceKey {get;}
    public RateSeries Receive {get;}=new();
    public RateSeries Transmit {get;}=new();
    private readonly RatePlot plot;
    private readonly DispatcherTimer timer=new(){Interval=TimeSpan.FromSeconds(1)};
    private Device? current;
    private string? session;
    private double elapsed;
    public NetworkRateWindow(Window owner,string key,string name)
    {
        Owner=owner;InterfaceKey=key;Title=name+" · 实时速率";Width=840;Height=430;MinWidth=600;MinHeight=300;WindowStartupLocation=WindowStartupLocation.CenterOwner;
        SetResourceReference(StyleProperty,typeof(Window));
        plot=new(Receive,Transmit){Margin=new(12)};
        var panel=new DockPanel();panel.SetResourceReference(BackgroundProperty,"Surface");panel.Children.Add(plot);Content=panel;
        timer.Tick+=(_,_)=>Refresh();Loaded+=(_,_)=>timer.Start();Closed+=(_,_)=>{timer.Stop();current=null;};
    }
    public void Update(Device? device){current=device;Refresh();}
    private void Refresh()
    {
        var now=DateTimeOffset.UtcNow;var metrics=current?.EffectiveMetrics;
        Metric? M(string suffix)=>metrics?.GetValueOrDefault(InterfaceKey+"_"+suffix);
        var nextSession=current?.CurrentSession?.SessionId;
        var duration=M("elapsed_seconds");
        var nextElapsed=duration is {Status:"ok"}&&double.TryParse(duration.Value,CultureInfo.InvariantCulture,out var v)?v:elapsed;
        if(session!=nextSession||nextElapsed<elapsed){Receive.Gap(now);Transmit.Gap(now);}session=nextSession;elapsed=nextElapsed;
        Receive.Observe(M("rx_bytes_per_sec"),current?.Online==true,now);Transmit.Observe(M("tx_bytes_per_sec"),current?.Online==true,now);
        plot.InvalidateVisual();
    }
}

internal sealed class RatePlot : Control
{
    private readonly RateSeries receive, transmit;
    private readonly bool megabits;
    private Rect area;
    public RatePlot(RateSeries receive, RateSeries transmit, bool megabits = false) {
        this.receive = receive; this.transmit = transmit; this.megabits = megabits;
        Focusable = false; SetResourceReference(FontSizeProperty, "UiPlotFontSize");
    }
    private readonly Brush rxBrush=new SolidColorBrush(Color.FromRgb(40,125,225));
    private readonly Brush txBrush=new SolidColorBrush(Color.FromRgb(223,133,35));
    private DateTimeOffset start,end;
    protected override void OnRender(DrawingContext dc)
    {
        base.OnRender(dc);if(ActualWidth<120||ActualHeight<100)return;
        var foreground=TryFindResource("Text") as Brush??Brushes.Black;var line=TryFindResource("Line") as Brush??Brushes.Gray;
        dc.DrawRectangle(Brushes.Transparent,null,new Rect(RenderSize));
        end=DateTimeOffset.UtcNow;start=end.AddMinutes(-10);
        var scale=FontSize/11; area=new Rect(82*scale,30*scale,Math.Max(1,ActualWidth-105*scale),Math.Max(1,ActualHeight-65*scale));
        var max=Math.Max(1,receive.Points.Concat(transmit.Points).Where(p=>p.At>=start&&p.Value.HasValue).Select(p=>p.Value!.Value).DefaultIfEmpty().Max()*1.1);
        FormattedText Format(string text, Brush? brush=null)=>new(text,CultureInfo.CurrentCulture,FlowDirection.LeftToRight,new Typeface(FontFamily,FontStyle,FontWeight,FontStretch),FontSize,brush??foreground,VisualTreeHelper.GetDpi(this).PixelsPerDip);
        void Text(string text,double x,double y,Brush? brush=null)=>dc.DrawText(Format(text,brush),new Point(x,y));
        var legend=megabits?"最近10分钟 · Mbps":"最近10分钟 · B/s";
        Text("● 接收",area.Left,4,rxBrush);Text("● 发送",area.Left+78*scale,4,txBrush);
        if(250*scale+Format(legend).Width<=ActualWidth)Text(legend,250*scale,4);
        else {Text(legend,area.Left,FontSize+12);area=new(area.X,area.Y+FontSize+10,area.Width,Math.Max(1,area.Height-FontSize-10));}
        for(var i=0;i<=4;i++){var y=area.Top+area.Height*i/4;dc.DrawLine(new(line,0.5),new(area.Left,y),new(area.Right,y));Text(megabits?(max*(4-i)/4*8/1000000).ToString("0.000",CultureInfo.InvariantCulture):TelemetryPresentation.Size(max*(4-i)/4)+"/s",0,y-7*scale);}
        var steps=Math.Clamp((int)(area.Width/(Format("00:00:00").Width+16)),2,5);
        for(var i=0;i<=steps;i++){var at=start.AddSeconds(600.0*i/steps);var label=at.ToLocalTime().ToString("HH:mm:ss");Text(label,Math.Clamp(area.Left+area.Width*i/steps-Format(label).Width/2,0,Math.Max(0,ActualWidth-Format(label).Width)),area.Bottom+10);}
        void Series(RateSeries data,Brush brush){RatePoint? previous=null;foreach(var point in data.Points){
            if(point.At<start||point.At>end||point.Value==null){previous=null;continue;}
            Point Position(RatePoint p)=>new(area.Left+(p.At-start).TotalSeconds/600*area.Width,area.Bottom-p.Value!.Value/max*area.Height);
            if(previous is {} p&&point.At>=p.At)dc.DrawLine(new(brush,1.8),Position(p),Position(point));
            dc.DrawEllipse(brush,null,Position(point),2,2);previous=point;
        }}
        dc.PushClip(new RectangleGeometry(area));Series(receive,rxBrush);Series(transmit,txBrush);dc.Pop();
        if(!receive.Points.Concat(transmit.Points).Any(p=>p.Value.HasValue&&p.At>=start))Text("等待有效速率采样",area.Left+area.Width/2-50,area.Top+area.Height/2);
    }
    protected override void OnMouseMove(System.Windows.Input.MouseEventArgs e)
    {
        base.OnMouseMove(e);if(area.Width<=0)return;
        var at=start.AddSeconds(Math.Clamp((e.GetPosition(this).X-area.Left)/area.Width,0,1)*600);
        string Nearest(RateSeries data){var p=data.Points.MinBy(p=>Math.Abs((p.At-at).TotalSeconds));return p==null?"—":p.At.ToLocalTime().ToString("HH:mm:ss")+"  "+(p.Value is {} v?(megabits?(v*8/1000000).ToString("0.000",CultureInfo.InvariantCulture)+" Mbps":TelemetryPresentation.Size(v)+"/s"):"采样中断");}
        ToolTip="接收 "+Nearest(receive)+"\n发送 "+Nearest(transmit);
    }
}
