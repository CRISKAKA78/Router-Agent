using System.Globalization;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;
internal sealed class CellularView : ContentControl
{
 private readonly Func<Device?> device;
 internal ComboBox Ports {get;}=new(){MinWidth=180,MaxWidth=360};
 internal TextBlock PortStatus {get;}=Ui.Text("");
 private readonly TextBlock state=Ui.Text("",true);
 private readonly Grid body=new();
 private readonly ScrollViewer chartScroll;
 internal CellularTrend[] Trends {get;}=[new("rsrp"),new("sinr"),new("rsrq")];
 private bool updating,rebind;
 private string? owner;
 private PropertyRow[] currentRows=[];
 internal DataGrid Details {get;}=PropertySheet.Create("蜂窝模块",system:true,inspectable:true);
 internal Dictionary<string,string> Sections {get;private set;}=[];
 internal event Action? RowsChanged;
 internal CellularView(Func<Device?> device,Action refresh)
 {
  this.device=device;AutomationProperties.SetName(Ports,"选择已发现的AT端口");
  Ports.SelectionChanged+=(_,_)=>{if(!updating)Render();};state.TextWrapping=TextWrapping.Wrap;
  var toolbar=new WrapPanel{Margin=new(0,0,0,8),Children={PortStatus,Ports,Ui.Button("刷新",refresh),state}};
  PortStatus.Margin=new(0,0,12,0);state.Margin=new(12,0,0,0);
  body.ColumnDefinitions.Add(new(){Width=new(1,GridUnitType.Star)});body.ColumnDefinitions.Add(new(){Width=new(300)});
  var charts=new StackPanel();foreach(var trend in Trends)charts.Children.Add(trend);
  chartScroll=new(){Content=charts,VerticalScrollBarVisibility=ScrollBarVisibility.Auto,HorizontalScrollBarVisibility=ScrollBarVisibility.Disabled,Margin=new(12,0,0,0)};
  Grid.SetColumn(chartScroll,1);body.Children.Add(Details);body.Children.Add(chartScroll);
  Content=Ui.Page(toolbar,body);
  SizeChanged+=(_,_)=>{body.ColumnDefinitions[1].Width=new(Math.Clamp(ActualWidth*.37,230,360));};
 }
 internal void Update()
 {
  var d=device();var key=d?.DeviceId+"/"+d?.CurrentSession?.SessionId+"/"+d?.Cellular?.ConfigRevision;
  var same=owner==key;var old=same?Ports.SelectedItem as CellularPort:null;owner=key;
  var all=d?.Cellular?.Ports??[];var selected=all.Where(p=>p.Selected).ToArray();
  // One automatic module needs no selector; multiple modules are displayed together, not chosen by tty number.
  var candidates=selected.Length>0?selected:all;
  updating=true;
  Ports.ItemsSource=candidates;
  Ports.SelectedItem=candidates.FirstOrDefault(p=>p.DeviceKey==old?.DeviceKey&&p.Path==old.Path)??candidates.FirstOrDefault();
  Ports.Visibility=selected.Length==0&&all.Length>0?Visibility.Visible:Visibility.Collapsed;
  PortStatus.Text=selected.Length>0?"端口已自动选择":all.Length>0?"未自动选中端口":"未发现端口";
  updating=false;
  if(!same){rebind=true;Details.SelectedItem=null;}
  Render();
 }
 private void Render()
 {
  var d=device();var sample=d?.Cellular;
  state.Text=d is null?"请选择设备":d.CellularConfiguration is null?"未启用":!d.Registration.Capabilities.Contains("cellular_identity_v1")?"需更新 Probe":sample is null?"等待采集":sample.Stale||!d.Online?"已过期":sample.Status=="unavailable"?"暂无可用端口":"";
  var automatic=sample?.Ports.Where(p=>p.Selected).ToArray()??[];
  var shown=automatic.Length>0?automatic:Ports.SelectedItem is CellularPort p?[p]:Array.Empty<CellularPort>();
  var rows=new List<PropertyRow>();
  foreach(var port in shown){
   var prefix=shown.Length>1?port.DeviceKey+"/":"";var group=shown.Length>1?port.Path:"蜂窝模块";
   void Add(string key,string name,string value)=>rows.Add(new(group,name,value){Key=prefix+key});
   Add("cellular_path","当前路径",port.Path);
   var fields=port.Fields??[];
   if(fields.Length==0&&port.ATI.Status=="ok")Add("cellular_ati_value","模块信息",port.ATI.Value);
   foreach(var field in fields){var fg=field.Group.Length>0?field.Group:group;if(shown.Length>1&&field.Group.Length>0)fg=port.Path+" · "+fg;rows.Add(new(fg,field.Name.Length>0?field.Name:FieldName(field.Key),field.Value){Key=prefix+"cellular_"+field.Key});}
   Add("cellular_imei_value","IMEI",port.IMEI.Status=="ok"?port.IMEI.Value:"未提供");
   foreach(var sig in port.Signals??[])Add("cellular_"+sig.Rat+"_"+sig.Key,sig.Key.ToUpperInvariant()+" · "+sig.Rat,sig.Qualifier+sig.Value.ToString("0.##",CultureInfo.InvariantCulture)+(sig.Unit=="code"?"（编码）":" "+sig.Unit));
   if(automatic.Length==0&&port.Status=="busy")state.Text="端口已占用";
  }
  var incoming=rows.ToArray();bool structure=rebind||!currentRows.Select(r=>(r.Key,r.Name,r.Group)).SequenceEqual(incoming.Select(r=>(r.Key,r.Name,r.Group)));
  if(!structure){for(var i=0;i<incoming.Length;i++){currentRows[i].Value=incoming[i].Value;currentRows[i].ValueTip=incoming[i].ValueTip;}}
  else{
   var anchor=TableBehavior.Capture(Details);var selection=(Details.SelectedItem as PropertyRow)?.Key;
   rebind=false;currentRows=incoming;Sections=currentRows.ToDictionary(r=>r.Key,r=>r.Group);
   PropertySheet.Bind(Details,currentRows,Sections);Details.SelectedItem=currentRows.FirstOrDefault(r=>r.Key==selection);TableBehavior.Restore(Details,anchor);RowsChanged?.Invoke();
  }
  // In a multi-modem snapshot do not combine samples from different USB devices into one trace.
  var chartPort=shown.FirstOrDefault();var id=owner+"/"+chartPort?.DeviceKey+"/"+chartPort?.IMEI.Value;
  foreach(var trend in Trends){var signal=chartPort?.Signals?.Where(s=>s.Key==trend.Metric).OrderBy(s=>s.Unit=="code"?2:s.Rat=="NR"?0:1).FirstOrDefault();
   trend.Observe(id,chartPort?.SampledAt,signal,sample?.Stale!=false||d?.Online!=true,sample?.IntervalSeconds??30);
  }
 }
 private static string FieldName(string key)=>key switch{
  "manufacturer"=>"厂家","model"=>"模块型号","firmware"=>"模块固件","svn"=>"SVN","sim"=>"SIM","iccid"=>"ICCID","imsi"=>"IMSI","operator"=>"运营商","mcc"=>"MCC","mnc"=>"MNC","access"=>"接入制式","eps_registration"=>"EPS 注册","nr_registration"=>"NR 注册","rssi"=>"RSSI","signal"=>"信号",_=>key};
}
