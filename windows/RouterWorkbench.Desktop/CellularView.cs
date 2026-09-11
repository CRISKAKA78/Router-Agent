using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;
internal sealed class CellularView : ContentControl
{
 private readonly Func<Device?> device;
 private readonly ComboBox ports=new(){MinWidth=210,MaxWidth=420};
 private readonly TextBlock status=Ui.Text("请选择设备",true);
 private string? owner;
 internal DataGrid Details {get;}=PropertySheet.Create("蜂窝模块 AT 身份",system:true,inspectable:true);
 internal Dictionary<string,string> Sections {get;private set;}=[];
 internal event Action? RowsChanged;
 internal CellularView(Func<Device?> device,Action refresh)
 {
  this.device=device;
  System.Windows.Automation.AutomationProperties.SetName(ports,"AT自动探测端口");
  ports.SelectionChanged+=(_,_)=>Render();status.TextWrapping=TextWrapping.Wrap;
  var controls=new WrapPanel();controls.Children.Add(Ui.Text("自动发现端口"));controls.Children.Add(ports);controls.Children.Add(Ui.Button("刷新快照",refresh));
  var note=Ui.Text("自动跳过占用端口；模块重启或串口号变化后重新识别。刷新快照不触发AT，采集按已应用模板周期执行。",true);note.TextWrapping=TextWrapping.Wrap;
  Content=Ui.Page(new StackPanel{Margin=new(0,0,0,8),Children={controls,status,note}},Details);
 }
 internal void Update()
 {
  var d=device();var key=d?.DeviceId+"/"+d?.CurrentSession?.SessionId;
  var old=owner==key?ports.SelectedItem as CellularPort:null;owner=key;
  var rows=d?.Cellular?.Ports??[];
  ports.ItemsSource=rows;
  ports.SelectedItem=rows.FirstOrDefault(p=>p.Path==old?.Path&&p.DeviceKey==old.DeviceKey)
   ??rows.FirstOrDefault(p=>p.DeviceKey==old?.DeviceKey&&p.Selected)??rows.FirstOrDefault(p=>p.Selected)??rows.FirstOrDefault();
  Render();
 }
 private void Render()
 {
  var d=device();var sample=d?.Cellular;
  status.Text=d is null?"请选择设备":!d.Registration.Capabilities.Contains("cellular_identity_v1")?"当前Probe不支持AT自动探测，需要更新Probe。":d.CellularConfiguration is null?"未启用：请在模板生成器启用蜂窝模块AT配置，发布并显式应用。":sample is null?"等待已应用模板的首次AT探测；无需手工填写串口号。":
   $"{CellularLabels.Status(sample.Status)} · {(sample.Stale||!d.Online?"离线或已过期的最后观测":"最近采集")} {Labels.Time(sample.SampledAt)} · 周期 {sample.IntervalSeconds} 秒 · 修订 {sample.ConfigRevision}"+(sample.Limited?" · 本轮达到数量/时间上限，后续继续探测":"")+(string.IsNullOrEmpty(sample.Reason)?"":" · "+CellularLabels.Reason(sample.Reason));
  var selected=ports.SelectedItem as CellularPort;
  PropertyRow[] rows=selected is null?[]:[
   new("端口","当前路径",selected.Path){Key="cellular_path"},
   new("端口","USB设备身份",selected.DeviceKey){Key="cellular_device"},
   new("端口","检测结果",CellularLabels.Status(selected.Status)){Key="cellular_status"},
   new("端口","当前选择",selected.Selected?"是 · 同一USB设备的已选AT端口":"否"){Key="cellular_selected"},
   new("端口","说明",DeviceProperties.Text(CellularLabels.Reason(selected.Reason))){Key="cellular_reason"},
   new("ATI","查询指令",DeviceProperties.Text(selected.ATI.Command)){Key="cellular_ati_command"},
   new("ATI","ATI状态",CellularLabels.Status(selected.ATI.Status)){Key="cellular_ati_status"},
   new("ATI","ATI原始信息",DeviceProperties.Text(selected.ATI.Value)){Key="cellular_ati_value"},
   new("IMEI","实际查询指令",DeviceProperties.Text(selected.IMEI.Command)){Key="cellular_imei_command"},
   new("IMEI","IMEI状态",CellularLabels.Status(selected.IMEI.Status)){Key="cellular_imei_status"},
   new("IMEI","IMEI",DeviceProperties.Text(selected.IMEI.Value)){Key="cellular_imei_value"},
   new("采样","端口观测时间",Labels.Time(selected.SampledAt)){Key="cellular_sampled"}];
  Sections=rows.ToDictionary(r=>r.Key,r=>r.Group);
  var previous=(Details.SelectedItem as PropertyRow)?.Key;
  var anchor=TableBehavior.Capture(Details);
  PropertySheet.Bind(Details,rows,Sections);Details.SelectedItem=rows.FirstOrDefault(r=>r.Key==previous);
  TableBehavior.Restore(Details,anchor);RowsChanged?.Invoke();
 }
}
