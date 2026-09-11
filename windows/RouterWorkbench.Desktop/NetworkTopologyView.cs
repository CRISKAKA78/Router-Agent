using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Animation;
using System.Windows.Shapes;
using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;

public sealed class NetworkTopologyView : UserControl
{
 private readonly Canvas canvas=new(){Width=1100,Height=700};
 private readonly ScrollViewer scroll=new(){HorizontalScrollBarVisibility=ScrollBarVisibility.Auto,VerticalScrollBarVisibility=ScrollBarVisibility.Auto};
 private readonly ScaleTransform scale=new(1,1);
 private readonly Dictionary<string,(ulong? Rx,ulong? Tx,DateTimeOffset Sample)> previous=[];
 private readonly List<Ellipse> particles=[];
 private string context="";
 private Point? drag;
 private double horizontal,vertical;
 private bool animations=true;
 public bool AnimationsEnabled {get=>animations;set{animations=value;if(!value)StopAnimations();}}
 public event Action<NetworkNode>? NodeSelected;
 public NetworkTopologyView(){canvas.LayoutTransform=scale;canvas.SetResourceReference(Panel.BackgroundProperty,"Surface");scroll.Content=canvas;Content=scroll;AutomationProperties.SetName(this,"组网拓扑，Ctrl 加滚轮缩放，中键拖动");
  PreviewMouseWheel+=(_,e)=>{if(Keyboard.Modifiers.HasFlag(ModifierKeys.Control)){Zoom(e.Delta>0?1.15:1/1.15);e.Handled=true;}};
  PreviewMouseDown+=(_,e)=>{if(e.ChangedButton==MouseButton.Middle){drag=e.GetPosition(this);horizontal=scroll.HorizontalOffset;vertical=scroll.VerticalOffset;CaptureMouse();e.Handled=true;}};
  PreviewMouseMove+=(_,e)=>{if(drag is {} start){var p=e.GetPosition(this);scroll.ScrollToHorizontalOffset(horizontal+start.X-p.X);scroll.ScrollToVerticalOffset(vertical+start.Y-p.Y);}};
  PreviewMouseUp+=(_,e)=>{if(e.ChangedButton==MouseButton.Middle){drag=null;ReleaseMouseCapture();}};
  IsVisibleChanged+=(_,_)=>{if(!IsVisible)StopAnimations();};Unloaded+=(_,_)=>StopAnimations();Loaded+=(_,_)=>{if(Window.GetWindow(this) is {} w){w.StateChanged-=WindowStateChanged;w.StateChanged+=WindowStateChanged;}};
 }
 private void WindowStateChanged(object? sender,EventArgs e){if(sender is Window{WindowState:WindowState.Minimized})StopAnimations();}
 private void StopAnimations(){foreach(var p in particles){p.BeginAnimation(Canvas.LeftProperty,null);p.BeginAnimation(Canvas.TopProperty,null);p.Visibility=Visibility.Collapsed;}particles.Clear();}
 public void Zoom(double factor){scale.ScaleX=scale.ScaleY=Math.Clamp(scale.ScaleX*factor,0.25,2.5);}
 public void Reset(){scale.ScaleX=scale.ScaleY=1;scroll.ScrollToHorizontalOffset(0);scroll.ScrollToVerticalOffset(0);}
 public void Fit(){scale.ScaleX=scale.ScaleY=Math.Clamp(Math.Min(Math.Max(1,ActualWidth-28)/canvas.Width,Math.Max(1,ActualHeight-28)/canvas.Height),0.25,1.5);scroll.ScrollToHorizontalOffset(0);scroll.ScrollToVerticalOffset(0);}
 private void Particle(Point from,Point to){if(!animations||!IsVisible||Window.GetWindow(this)?.WindowState==WindowState.Minimized)return;var dot=new Ellipse{Width=7,Height=7,IsHitTestVisible=false};dot.SetResourceReference(Shape.FillProperty,"Accent");canvas.Children.Add(dot);particles.Add(dot);var x=new DoubleAnimation(from.X-3,to.X-3,TimeSpan.FromSeconds(1.6)){RepeatBehavior=new RepeatBehavior(2),FillBehavior=FillBehavior.Stop};var y=new DoubleAnimation(from.Y-3,to.Y-3,TimeSpan.FromSeconds(1.6)){RepeatBehavior=new RepeatBehavior(2),FillBehavior=FillBehavior.Stop};y.Completed+=(_,_)=>dot.Visibility=Visibility.Collapsed;dot.BeginAnimation(Canvas.LeftProperty,x);dot.BeginAnimation(Canvas.TopProperty,y);}
 public void Render(NetworkTopology data,OverlayNetwork? network,IReadOnlyDictionary<string,string> names,bool logical,string? observer=null)
 {
  StopAnimations();canvas.Children.Clear();var nextContext=$"{network?.NetworkId}/{observer}";if(context!=nextContext){previous.Clear();context=nextContext;}
  if(network is null){canvas.Children.Add(Ui.Text("选择网络后显示拓扑"));return;}
  var nodes=data.Nodes.OrderBy(n=>n.DeviceId!=observer).ThenBy(n=>n.External).ThenBy(n=>n.Id).ToArray();var positions=new Dictionary<string,Point>();
  canvas.Width=Math.Max(1000,Math.Sqrt(Math.Max(1,nodes.Length))*290);canvas.Height=Math.Max(650,Math.Sqrt(Math.Max(1,nodes.Length))*220);
  var center=new Point(canvas.Width/2,canvas.Height/2+15);var hasLocal=nodes.Length>0&&nodes[0].DeviceId==observer;var ring=nodes.Length-(hasLocal?1:0);
  for(int i=0;i<nodes.Length;i++){if(hasLocal&&i==0){positions[nodes[i].Id]=center;continue;}double angle=2*Math.PI*(i-(hasLocal?1:0))/Math.Max(1,ring)-Math.PI/2;positions[nodes[i].Id]=new(center.X+Math.Cos(angle)*(canvas.Width/2-130),center.Y+Math.Sin(angle)*(canvas.Height/2-115));}
  var caption=Ui.Text(logical?$"{network.Name} · 逻辑成员":$"{network.Name} · {(observer==null?"全部观测":hasLocal?"本机观察视角":"所选设备无组网观测")}",true);Canvas.SetLeft(caption,20);Canvas.SetTop(caption,18);canvas.Children.Add(caption);
  if(!logical){foreach(var group in data.Edges.Where(e=>observer==null||e.ReportedBy==observer).GroupBy(e=>$"{e.Source}/{e.Target}/{e.Transport}")){
   var edge=group.First();if(!positions.TryGetValue(edge.Source,out var a)||!positions.TryGetValue(edge.Target,out var b))continue;
   var line=new Line{X1=a.X,Y1=a.Y,X2=b.X,Y2=b.Y,StrokeThickness=edge.Stale?1.3:2,ToolTip=$"直连 · {edge.Transport}\n更新 {edge.ObservedAt.ToLocalTime():HH:mm:ss}"};line.SetResourceReference(Shape.StrokeProperty,edge.Stale?"Muted":"Accent");if(!edge.ConfirmedBoth||edge.Stale)line.StrokeDashArray=new DoubleCollection{5,4};canvas.Children.Add(line);
   var label=Ui.Text(edge.Transport,true);Canvas.SetLeft(label,(a.X+b.X)/2+8);Canvas.SetTop(label,(a.Y+b.Y)/2+5);canvas.Children.Add(label);
   ulong? Sum(Func<NetworkEdge,ulong?> select)=>group.All(e=>select(e)!=null)?group.Aggregate(0UL,(sum,e)=>sum+select(e)!.Value):null;
   var rx=Sum(e=>e.Link.RxBytes);var tx=Sum(e=>e.Link.TxBytes);if(previous.TryGetValue(group.Key,out var old)&&edge.ObservedAt>old.Sample&&!edge.Stale&&DateTimeOffset.UtcNow-edge.ObservedAt<TimeSpan.FromSeconds(35)){if(rx>old.Rx)Particle(b,a);if(tx>old.Tx)Particle(a,b);}previous[group.Key]=(rx,tx,edge.ObservedAt);
  }
  var source=data.Observations.FirstOrDefault(o=>o.DeviceId==observer&&!o.Stale);if(source!=null&&positions.TryGetValue("device:"+observer,out var root)){foreach(var route in source.Routes.Where(r=>r.NextHopPeerId!=0&&r.NextHopPeerId!=r.PeerId&&!source.Links.Any(l=>l.PeerId==r.PeerId))){var target=nodes.FirstOrDefault(n=>n.PeerId==route.PeerId||route.InstanceId.Length>0&&network.Members.Any(m=>m.DeviceId==n.DeviceId&&m.InstanceId==route.InstanceId));if(target==null||!positions.TryGetValue(target.Id,out var end))continue;var path=new System.Windows.Shapes.Path{Data=new LineGeometry(root,end),StrokeThickness=1.4,StrokeDashArray=new DoubleCollection{2,5},ToolTip=$"中继，经节点 {route.NextHopPeerId}；端到端延迟未提供"};path.SetResourceReference(Shape.StrokeProperty,"Muted");canvas.Children.Add(path);}}
  }
  foreach(var n in nodes){var p=positions[n.Id];var title=n.DeviceId==observer?"本机":n.DeviceId is {} id&&names.TryGetValue(id,out var name)?name:n.Hostname.Length>0?n.Hostname:$"外部节点 {n.PeerId}";
   var text=new StackPanel();var icon=new System.Windows.Shapes.Path{Data=Geometry.Parse("M 4,12 L 40,12 40,28 4,28 Z M 11,12 L 7,2 M 33,12 L 37,2 M 10,21 L 13,21 M 18,21 L 21,21 M 29,21 L 34,21"),StrokeThickness=2,Width=44,Height=30,Stretch=Stretch.Uniform,HorizontalAlignment=HorizontalAlignment.Center,Margin=new(0,0,0,5)};icon.SetResourceReference(Shape.StrokeProperty,n.State=="running"||n.State=="observed_peer"?"Accent":"Muted");text.Children.Add(icon);text.Children.Add(Ui.Text(title));text.Children.Add(Ui.Text(string.IsNullOrEmpty(n.VirtualIp)?"虚拟 IP 未提供":n.VirtualIp,true));text.Children.Add(Ui.Text(logical?(n.External?"外部节点":"网络成员"):NetworkLabels.State(n.State),true));
   var button=new Button{Content=text,Width=180,MinHeight=112,Padding=new(12),HorizontalContentAlignment=HorizontalAlignment.Center,ToolTip=title};AutomationProperties.SetName(button,$"{title} {n.VirtualIp} {NetworkLabels.State(n.State)}");button.Click+=(_,_)=>NodeSelected?.Invoke(n);Canvas.SetLeft(button,p.X-90);Canvas.SetTop(button,p.Y-56);canvas.Children.Add(button);
  }
 }
}
