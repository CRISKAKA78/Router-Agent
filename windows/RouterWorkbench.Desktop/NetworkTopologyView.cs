using System.Globalization;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Shapes;
using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;

// Display-only topology: only reported links are drawn. Logical membership is
// rendered separately; it never fabricates a full mesh or assumes reachability.
public sealed class NetworkTopologyView : UserControl
{
 private readonly Canvas canvas=new(){Width=1100,Height=700};
 private readonly ScrollViewer scroll=new(){HorizontalScrollBarVisibility=ScrollBarVisibility.Auto,VerticalScrollBarVisibility=ScrollBarVisibility.Auto};
 private readonly ScaleTransform scale=new(1,1);
 private Point? drag;
 private double horizontal,vertical;
 public event Action<NetworkNode>? NodeSelected;
 public NetworkTopologyView(){
  canvas.LayoutTransform=scale;canvas.SetResourceReference(Panel.BackgroundProperty,"Surface");scroll.Content=canvas;Content=scroll;
  AutomationProperties.SetName(this,"组网拓扑，可使用旁边的成员列表和链路列表访问相同信息");
  PreviewMouseWheel+=(_,e)=>{if(Keyboard.Modifiers.HasFlag(ModifierKeys.Control)){Zoom(e.Delta>0?1.15:1/1.15);e.Handled=true;}};
  PreviewMouseDown+=(_,e)=>{if(e.ChangedButton==MouseButton.Middle){drag=e.GetPosition(this);horizontal=scroll.HorizontalOffset;vertical=scroll.VerticalOffset;CaptureMouse();e.Handled=true;}};
  PreviewMouseMove+=(_,e)=>{if(drag is {} start){var p=e.GetPosition(this);scroll.ScrollToHorizontalOffset(horizontal+start.X-p.X);scroll.ScrollToVerticalOffset(vertical+start.Y-p.Y);}};
  PreviewMouseUp+=(_,e)=>{if(e.ChangedButton==MouseButton.Middle){drag=null;ReleaseMouseCapture();}};
 }
 public void Zoom(double factor){scale.ScaleX=scale.ScaleY=Math.Clamp(scale.ScaleX*factor,0.4,2.5);}
 public void Reset(){scale.ScaleX=scale.ScaleY=1;scroll.ScrollToHorizontalOffset(0);scroll.ScrollToVerticalOffset(0);}
 public void Render(NetworkTopology data,OverlayNetwork? network,IReadOnlyDictionary<string,string> names,bool logical){
  canvas.Children.Clear();if(network is null){canvas.Children.Add(Ui.Text("选择网络后显示拓扑"));return;}
  var positions=new Dictionary<string,Point>();var nodes=data.Nodes.OrderBy(n=>n.External).ThenBy(n=>n.Id).ToArray();
  canvas.Width=Math.Max(1000,Math.Ceiling(Math.Sqrt(Math.Max(1,nodes.Length)))*220+120);canvas.Height=Math.Max(620,Math.Ceiling(nodes.Length/4.0)*120+120);
  for(var i=0;i<nodes.Length;i++){var n=nodes[i];var columns=Math.Max(1,(int)((canvas.Width-120)/220));positions[n.Id]=new(55+(i%columns)*220,75+(i/columns)*120);}
  var caption=Ui.Text(logical?$"逻辑成员关系 · {network.Name} · 不表示实际直连":"观测连接 · 实线双边确认 / 虚线单边或过期 · Ctrl+滚轮缩放 / 中键拖动",true);caption.TextWrapping=TextWrapping.Wrap;Canvas.SetLeft(caption,18);Canvas.SetTop(caption,15);canvas.Children.Add(caption);
  if(!logical){foreach(var edge in data.Edges){if(!positions.TryGetValue(edge.Source,out var a)||!positions.TryGetValue(edge.Target,out var b))continue;a+=new Vector(80,32);b+=new Vector(80,32);var line=new Line{X1=a.X,Y1=a.Y,X2=b.X,Y2=b.Y,StrokeThickness=1.6,ToolTip=$"{edge.Transport} · 来源 {edge.ReportedBy}\n{edge.ObservedAt.ToLocalTime():HH:mm:ss} · 链路延迟 {edge.Link.LatencyMs?.ToString("F1",CultureInfo.InvariantCulture)??"未提供"} ms"};line.SetResourceReference(Shape.StrokeProperty,edge.Stale?"Muted":"Accent");if(!edge.ConfirmedBoth||edge.Stale)line.StrokeDashArray=new DoubleCollection{5,4};canvas.Children.Add(line);}}
  foreach(var n in nodes){var p=positions[n.Id];var title=n.DeviceId is {} id&&names.TryGetValue(id,out var name)?name:$"Peer {n.PeerId}";var text=new StackPanel();text.Children.Add(Ui.Text(title));text.Children.Add(Ui.Text(string.IsNullOrEmpty(n.VirtualIp)?"虚拟IP未提供":n.VirtualIp,true));text.Children.Add(Ui.Text(logical?(n.External?"外部观测节点":"网络成员"):NetworkLabels.State(n.State),true));var button=new Button{Content=text,Width=180,MinHeight=76,Padding=new(9),HorizontalContentAlignment=HorizontalAlignment.Left,ToolTip=title};AutomationProperties.SetName(button,$"{title} {n.VirtualIp} {NetworkLabels.State(n.State)}");button.Click+=(_,_)=>NodeSelected?.Invoke(n);Canvas.SetLeft(button,p.X);Canvas.SetTop(button,p.Y);canvas.Children.Add(button);}
 }
}

