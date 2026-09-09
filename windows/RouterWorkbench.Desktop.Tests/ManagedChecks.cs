using System.Text.Json;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using System.Windows.Media;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static IEnumerable<T> Visuals<T>(DependencyObject parent) where T:DependencyObject {
        for(var i=0;i<VisualTreeHelper.GetChildrenCount(parent);i++){var child=VisualTreeHelper.GetChild(parent,i);if(child is T t)yield return t;foreach(var next in Visuals<T>(child))yield return next;}
    }
    private static async Task ManagedDeviceChecks(MainWindow window,ApiClient api,WorkspaceConnection connection,List<TestProbe> peers,int port)
    {
        var raw=await api.ExecuteAsync(new("分组模板","probe-templates",new{name="分组测试模板",properties=new{signal=new{name="信号",command="printf 90",interval_seconds=7}},
            presentation=new{groups=new[]{new{id="network",name="网络信息",order=20},new{id="basic",name="基本信息",order=10}},fields=new Dictionary<string,object>{["device_id"]=new{group_id="basic",order=20},["hostname"]=new{group_id="basic",order=10},["signal"]=new{group_id="network",order=10}}}}));
        var template=raw.Deserialize<ProbeTemplate>(ApiJson.Options)!;
        await api.ExecuteAsync(new("测试型号","device-models/test-board",new{version=0,name="标准型号",aliases=new[]{"board-test"},template_id=template.TemplateId},"PUT"));
        var peer=new TestProbe();peers.Add(peer);
        await peer.StartAsync(port,"managed-ui",new{device_id="managed-ui",hostname="探测名称",model="board-test",probe_version="managed-fixture",arch="x86_64",boot_id="b",capabilities=new[]{"exec","telemetry_v2","managed_config_v1"}});
        await Eventually(()=>Task.FromResult(connection.Snapshot.Devices.Any(d=>d.DeviceId=="managed-ui")),"new discovery reaches unified snapshot");
        ((TabControl)window.FindName("ExplorerTabs")).SelectedIndex=1;var pool=Field<DataGrid>(window,"discoveries");
        await Eventually(()=>Task.FromResult(pool.Items.Cast<Device>().Any(d=>d.DeviceId=="managed-ui")),"pending device shown in admission pool");
        Check(!((DataGrid)window.FindName("DevicesGrid")).Items.Cast<Device>().Any(d=>d.DeviceId=="managed-ui"),"pending excluded from managed device table");
        Render(window,"device-discovery.png");
        var pending=pool.Items.Cast<Device>().Single(d=>d.DeviceId=="managed-ui");pool.SelectedItem=pending;
        var edit=InvokeAsync(window,"EnrollDevice",pending);
        await Eventually(()=>Task.FromResult(Application.Current.Windows.Cast<Window>().Any(w=>w.Owner==window&&w.Title=="添加设备")),"native admission form opens");
        var dialog=Application.Current.Windows.Cast<Window>().Single(w=>w.Owner==window&&w.Title=="添加设备");dialog.UpdateLayout();
        var name=Visuals<TextBox>(dialog).Single(t=>AutomationProperties.GetName(t)=="设备名称");name.Text="管理员机房路由器";
        var combos=Visuals<ComboBox>(dialog).ToArray();var model=combos.Single(c=>!c.IsEditable);var selection=combos.Single(c=>c.IsEditable);
        Check((model.SelectedItem as DeviceModel)?.ModelId=="test-board"&&(selection.SelectedItem as ProbeTemplate)?.TemplateId==template.TemplateId,"detected model fills server default template");
        var editor=Visuals<TextBox>(selection).Single();editor.Focus();editor.SelectAll();editor.Text="分组测试";await Task.Delay(100);
        Check(selection.Items.Cast<ProbeTemplate>().Single().TemplateId==template.TemplateId,"template input filters choices while typing");selection.SelectedItem=selection.Items[0];selection.IsDropDownOpen=false;
        Render(dialog,"admission-form.png");dialog.UpdateLayout();await Task.Delay(100);
        Visuals<Button>(dialog).Single(b=>b.Content?.ToString()=="确认纳管").RaiseEvent(new RoutedEventArgs(Button.ClickEvent));await Eventually(()=>Task.FromResult(edit.IsCompleted),"admission confirmation completes");await edit;
        await Eventually(()=>Task.FromResult(connection.Snapshot.Devices.Any(d=>d.DeviceId=="managed-ui"&&d.Profile?.ConfigurationState=="applied")),"admission applies server configuration via current control connection");
        var devices=(DataGrid)window.FindName("DevicesGrid");devices.SelectedItem=devices.Items.Cast<Device>().Single(d=>d.DeviceId=="managed-ui");Invoke(window,"Navigate","overview");
        await peer.ReportAsync("template",new{signal=new{name="信号",value="90",unit="text",status="ok",interval_seconds=7}});
        await Eventually(()=>Task.FromResult(Field<PropertyRow[]>(window,"propertyRows").Any(r=>r.Key=="signal"&&r.Value=="90")),"template field reaches grouped native properties");
        await TableRefinementChecks(window);
        var props=Field<DataGrid>(window,"properties");var rows=Field<PropertyRow[]>(window,"propertyRows");
        Check(rows.Where(r=>r.GroupId=="basic").Select(r=>r.Key).SequenceEqual(new[]{"hostname","device_id"})&&rows.Single(r=>r.Key=="signal").GroupId=="network","template field order and custom assignment");
        Check(Field<TabControl>(window,"overviewTabs").Items.Cast<TabItem>().Select(t=>t.Header.ToString()).SequenceEqual(new[]{"系统信息","资源监控","基本信息","网络信息","外壳端口","系统端口","其他信息","存储空间","连接历史","接口采样设置"}),"built-in and custom groups are parallel; specialized pages retained");
        props.UpdateLayout();Check(!Visuals<Expander>(props).Any(),"property pages have no nested group expanders");
        var switchValues=new Dictionary<string,object>();foreach(var (key,value) in new[]{("state","up"),("admin","down"),("chip","switch0"),("port","1"),("system","vlan3"),("uplink","eth0"),("label","端口1"),("speed","1000"),("duplex","full"),("role","external")})switchValues["switch_lan1_"+key]=new{name=key,value,unit="text",status="ok",entity="lan1",interval_seconds=5};
        foreach(var (field,value,unit) in new[]{("rx_bytes_per_sec","1000","bytes_per_sec"),("tx_bytes_per_sec","2000","bytes_per_sec"),("rx_bytes","1048576","bytes"),("tx_bytes","2097152","bytes"),("rx_raw_bytes","9007199254740993","bytes"),("tx_raw_bytes","9007199254741003","bytes"),("elapsed_seconds","60","seconds"),("counter_source","swconfig_mib switch0:1 RxGoodByte/TxByte","text"),("counter_basis","RX good / TX bytes","text")})switchValues["switch_lan1_"+field]=new{name=field,value,unit,status="ok",entity="lan1",interval_seconds=5};
        await peer.ReportAsync("switch",switchValues);
        await Eventually(()=>Task.FromResult(Field<DataGrid>(window,"switchPorts").Items.Count==1),"physical port telemetry reaches WPF table");
        var row=(MonitorTableRow)Field<DataGrid>(window,"switchPorts").Items[0];Check(row.Entity=="端口1"&&row.A=="已连接"&&row.B=="1000 Mbps / 全双工","port label, system topology, physical and administrative states remain separate");
        Check(row.C=="0.008 Mbps"&&row.D=="0.016 Mbps"&&row.E=="1 MB"&&row.F=="2 MB"&&row.G=="1分钟0秒","physical row shows independent rates, totals and duration together");
        Check((await api.GetAsync<Device>("devices/managed-ui")).EffectiveMetrics!["switch_lan1_rx_raw_bytes"].Value=="9007199254740993","removing details preserves raw integer counter in public API");
        switchValues["switch_lan1_rx_bytes_per_sec"]=new{name="RX",value="",unit="bytes_per_sec",status="error",reason="port_counter_failed",entity="lan1",interval_seconds=5};
        await peer.ReportAsync("switch",switchValues);
        await Eventually(()=>Task.FromResult(((MonitorTableRow)Field<DataGrid>(window,"switchPorts").Items[0]).C=="—"),"physical counter failure is not replaced by logical interface rate");
        Check(ReferenceEquals(row,Field<DataGrid>(window,"switchPorts").Items[0])&&row.A=="已连接","counter refresh preserves row selection and independent link state");
        switchValues["switch_lan1_rx_bytes_per_sec"]=new{name="RX",value="1000",unit="bytes_per_sec",status="ok",entity="lan1",interval_seconds=5};
        await peer.ReportAsync("switch",switchValues);
        var d=await api.GetAsync<Device>("devices/managed-ui");VisibilityChecks(d);Check(!props.Items.Cast<PropertyRow>().Any(p=>p.Key.StartsWith("switch_")),"physical port detail stays hidden in properties while specialized port table remains populated");Check(((DataGrid)window.FindName("QuickProperties")).Items[1].GetType().GetProperty("Value")!.GetValue(((DataGrid)window.FindName("QuickProperties")).Items[1])?.ToString()=="标准型号","selected-device summary prefers administrator model");Check(d.DisplayName=="管理员机房路由器"&&d.Registration.Hostname=="探测名称","administrator name preserves raw registration");
        await api.ExecuteAsync(new("采样修改","devices/managed-ui/profile",new{version=d.Profile!.Version,admission="managed",name=d.Profile.Name,model_id=d.Profile.ModelId,template_id=template.TemplateId,interface_sampling=new InterfaceSampling(NetworkSeconds:3)},"PUT"));
        await Eventually(()=>Task.FromResult(peer.ConfigurationRevision==2&&connection.Snapshot.Devices.Single(d=>d.DeviceId=="managed-ui").AppliedRevision==2),"sampling revision changes without reconnect");
        Check(peer.SessionId==d.CurrentSession?.SessionId,"configuration update retains original Session");
        await peer.ReportAsync("template",new{signal=new{name="信号",value="90",unit="text",status="ok",interval_seconds=7}});await peer.ReportAsync("switch",switchValues);await Eventually(()=>Task.FromResult(Field<DataGrid>(window,"switchPorts").Items.Count==1),"new configuration replaces previous port sample");
        Render(window,"managed-groups.png");var tabs=Field<TabControl>(window,"overviewTabs");tabs.SelectedItem=tabs.Items.Cast<TabItem>().Single(t=>t.Header?.ToString()=="外壳端口");await Task.Delay(100);Render(window,"managed-switch-ports.png");tabs.SelectedIndex=0;
        await WorkspaceChecks(window,api,connection);
        await api.ExecuteAsync(new("离线配置验证","devices/managed-ui/disconnect",new{}));
        await Eventually(()=>Task.FromResult(!connection.Snapshot.Devices.Single(d=>d.DeviceId=="managed-ui").Online),"device disconnect preserves managed offline record");
        devices.SelectedItem=devices.Items.Cast<Device>().Single(d=>d.DeviceId=="managed-ui");
        var overview=Field<TabControl>(window,"overviewTabs");overview.SelectedItem=overview.Items.Cast<TabItem>().Single(t=>t.Header.ToString()=="接口采样设置");
        window.UpdateLayout();Render(window,"interface-sampling.png");
        Field<TextBox>(window,"samplingSeconds").Text="4";
        await InvokeAsync(window,"SaveSamplingView",false);
        var offline=await api.GetAsync<Device>("devices/managed-ui");
        Check(!offline.Online&&offline.Profile?.InterfaceSampling?.NetworkSeconds==4&&offline.Profile.Monitoring==null&&offline.Profile.ConfigurationState=="waiting_dispatch","native offline sampling save persists desired configuration for reconnect");
        var counterTemplate=(await api.ExecuteAsync(new("可选计数能力模板","probe-templates",new{name="硬件字节",properties=new{},switch_probe=new{backend="swconfig",ports=new[]{new{id="lan1",switch_id="switch0",port=1,role="external"}},counters=new{backend="swconfig_mib",rx_field="RxGoodByte",tx_field="TxByte",bits=64,basis="hardware bytes"}}}))).Deserialize<ProbeTemplate>(ApiJson.Options)!;
        var limitedPeer=new TestProbe();peers.Add(limitedPeer);await limitedPeer.StartAsync(port,"limited-port-probe",new{device_id="limited-port-probe",hostname="limited",probe_version="current-fixture",arch="x86_64",boot_id="limited",capabilities=new[]{"telemetry_v2","managed_config_v1"}});
        Device? limitedDevice=null;await Eventually(async()=>{try{limitedDevice=await api.GetAsync<Device>("devices/limited-port-probe");return limitedDevice.Profile!=null;}catch{return false;}},"probe without counter capability discovery available");
        await api.ExecuteAsync(new("应用硬件模板","devices/limited-port-probe/profile",new{version=limitedDevice!.Profile!.Version,admission="managed",name="无计数能力探针",template_id=counterTemplate.TemplateId,apply_template=true},"PUT"));
        await Eventually(async()=>{var current=await api.GetAsync<Device>("devices/limited-port-probe");return current.Profile?.ConfigurationState=="failed"&&current.Profile.ConfigurationError=="unsupported_port_counters";},"probe without counter capability capability failure is explicit instead of false applied acknowledgement");
        Check(limitedPeer.ConfigurationRevision==0,"unsupported counter configuration never dispatched to probe without counter capability");
        devices.SelectedItem=devices.Items.Cast<Device>().Single(d=>d.DeviceId=="desktop-router-02");
    }
}
