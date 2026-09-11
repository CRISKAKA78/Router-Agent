using System.Globalization;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Media;
using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;

internal sealed class CellularTrend : FrameworkElement
{
 internal sealed record Sample(DateTimeOffset At,double? Value);
 internal string Metric {get;}
 internal IReadOnlyList<Sample> Samples=>samples;
 internal CellularSignal? Signal {get;private set;}
 private readonly List<Sample> samples=[];
 private string owner="";
 private DateTimeOffset? last;
 internal CellularTrend(string metric){Metric=metric;MinHeight=142;ClipToBounds=true;AutomationProperties.SetName(this,metric.ToUpperInvariant()+" 趋势");}
 internal void Observe(string identity,DateTimeOffset? at,CellularSignal? signal,bool stale,uint interval)
 {
  // Missing samples are gaps in the existing series, never a fabricated zero.
  if(identity!=owner || (Signal==null&&signal!=null) || (Signal!=null&&signal!=null&&(Signal.Rat!=signal.Rat||Signal.Unit!=signal.Unit))){samples.Clear();last=null;Signal=null;owner=identity;}
  if(signal!=null)Signal=signal;
  if(at is {} time && (last==null||time>last)) {
   if(last is {} prior && time-prior>TimeSpan.FromSeconds(Math.Max(1,interval)*3))samples.Add(new(prior.AddMilliseconds(1),null));
   samples.Add(new(time,stale?null:signal?.Value));last=time;
   while(samples.Count>180)samples.RemoveAt(0);
  }
  ToolTip=Signal is {} s?$"{Metric.ToUpperInvariant()} · {s.Rat} · {s.Minimum:g}～{s.Maximum:g} {s.Unit}\n"+(stale?"已过期":signal==null?"未提供":$"{signal.Qualifier}{signal.Value:g} {signal.Unit}"):"未提供";
  AutomationProperties.SetHelpText(this,ToolTip.ToString());InvalidateVisual();
 }
 protected override void OnRender(DrawingContext dc)
 {
  base.OnRender(dc);var text=(Brush)FindResource("Text");var muted=(Brush)FindResource("Muted");var accent=(Brush)FindResource("Accent");
  var font=System.Windows.Documents.TextElement.GetFontFamily(this);var type=new Typeface(font,FontStyles.Normal,FontWeights.Normal,FontStretches.Normal);
  var fs=Math.Clamp(System.Windows.Documents.TextElement.GetFontSize(this),11,20);var dpi=VisualTreeHelper.GetDpi(this).PixelsPerDip;
  void Label(string value,double x,double y,Brush brush){dc.DrawText(new FormattedText(value,CultureInfo.CurrentCulture,FlowDirection.LeftToRight,type,fs,brush,dpi),new(x,y));}
  var signal=Signal;var title=Metric.ToUpperInvariant()+(signal==null?"":$" · {signal.Rat}"+(signal.Unit=="code"?" 编码":$" ({signal.Unit})"));
  Label(title,8,4,text);
  double left=48,top=32,right=Math.Max(left+1,ActualWidth-12),bottom=Math.Max(top+1,ActualHeight-26);
  var grid=new Pen(muted,.4){DashStyle=DashStyles.Dot};
  var low=signal?.Minimum??0;var high=signal?.Maximum??1;
  for(int i=0;i<3;i++){double y=top+(bottom-top)*i/2;dc.DrawLine(grid,new(left,y),new(right,y));if(signal!=null)Label((high-(high-low)*i/2).ToString("0.#",CultureInfo.InvariantCulture),2,y-7,muted);}
  if(signal==null||samples.All(p=>p.Value==null)){Label("未提供",left+12,top+20,muted);return;}
  var begin=samples[0].At;var end=samples[^1].At;double span=Math.Max(1,(end-begin).TotalSeconds);Point? previous=null;
  var pen=new Pen(accent,1.8);
  foreach(var point in samples){if(point.Value is not {} value){previous=null;continue;}
   var pos=new Point(left+(right-left)*(point.At-begin).TotalSeconds/span,bottom-(bottom-top)*(Math.Clamp(value,low,high)-low)/(high-low));
   if(previous is {} from)dc.DrawLine(pen,from,pos);dc.DrawEllipse(accent,null,pos,2,2);previous=pos;
  }
  Label(begin.ToLocalTime().ToString("HH:mm:ss"),left,bottom+6,muted);
  if(right-left>155)Label(end.ToLocalTime().ToString("HH:mm:ss"),right-64,bottom+6,muted);
 }
}
