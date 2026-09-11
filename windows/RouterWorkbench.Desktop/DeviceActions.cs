using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public partial class MainWindow
{
    private void ConfigureDeviceMenu()
    {
        var menu = new ContextMenu();
        var copy = new MenuItem { Header = "复制", InputGestureText = "Ctrl+C" };
        copy.Click += (_, _) => {
            var rows = DevicesGrid.SelectedItems.Cast<Device>();
            var text = string.Join("\n", rows.Select(d => $"{d.DisplayName}\t{d.DeviceId}\t{d.StatusText}"));
            if (text.Length > 0) Clipboard.SetText(text);
        };
        var rename = new MenuItem { Header = "修改设备名…" };
        rename.Click += (_, _) => { if (DevicesGrid.SelectedItem is Device device) _ = Run("修改设备名称", () => RenameDevice(device)); };
        var update = new MenuItem { Header = "更新模板…" };
        update.Click += (_, _) => { if (DevicesGrid.SelectedItem is Device device) _ = Run("更新模板", () => UpdateDeviceTemplate(device)); };
        menu.Items.Add(copy); menu.Items.Add(rename); menu.Items.Add(update);
        menu.Opened += (_, _) => {
            copy.IsEnabled = DevicesGrid.SelectedItems.Count > 0;
            rename.IsEnabled = update.IsEnabled = Writable && DevicesGrid.SelectedItems.Count == 1;
        };
        DevicesGrid.ContextMenu = menu;
        DevicesGrid.PreviewMouseRightButtonDown += (_, e) => {
            var row = TableBehavior.Ancestor<DataGridRow>(e.OriginalSource as DependencyObject);
            if (row != null && !row.IsSelected) { DevicesGrid.SelectedItem = row.Item; }
        };
    }

    private void ExplorerSelectionChanged(object sender, SelectionChangedEventArgs e)
    {
        if (e.Source != ExplorerTabs || refreshing || discoveries == null || tabs.Count == 0) return;
        var grid = ExplorerTabs.SelectedIndex == 0 ? DevicesGrid : discoveries;
        var device = grid.SelectedItem as Device ?? grid.Items.OfType<Device>().FirstOrDefault();
        selectedDevice = device?.DeviceId ?? "";
        CancelDetails(); ResetConfig(); ApplySnapshot();
    }

    private Task RenameDevice(Device device)
    {
        var profile = device.Profile ?? throw new InvalidOperationException("请先纳管设备。");
        new FormWindow(this, "修改设备名称", "", [new("name", "设备名称", device.DisplayName)], async values => {
            var name = values["name"].Trim();
            if (name.Length == 0) throw new InvalidOperationException("设备名称不能为空。");
            await SaveProfile(device, "managed", name, profile.ModelId, profile.BoundTemplate?.TemplateId ?? "", 0, false, profile.Monitoring, profile.PropertyIntervals);
        }).ShowDialog();
        return Task.CompletedTask;
    }

    private async Task UpdateDeviceTemplate(Device requested)
    {
        var owner = Connected();
        var device = await owner.TrackAsync(() => owner.Api.GetAsync<Device>($"devices/{Id(requested.DeviceId)}"));
        if (owner != connection || closing) return;
        if (!device.Managed || device.Profile is not { } profile) throw new InvalidOperationException("请先纳管设备。");
        var templates = await owner.TrackAsync(()=>owner.Api.ListAsync<ProbeTemplate>("probe-templates"));
        if(owner!=connection||closing)return;
        var choices=new[]{new ProbeTemplate("","内置采集（无模板）",0,null,[])}.Concat(templates).ToArray();
        var search=Ui.Input();var list=new ListBox {ItemsSource=choices};
        list.SelectedItem=choices.FirstOrDefault(t=>t.TemplateId==(profile.BoundTemplate?.TemplateId??""));
        var current=device.ActiveTemplate is {TemplateId:not "builtin"} active?$"{active.Name} · v{active.Version}":device.ActiveTemplate==null?"尚未确认生效":"内置采集";
        var note=Ui.Text($"设备：{device.DisplayName}\n当前生效：{current}",true);note.TextWrapping=TextWrapping.Wrap;
        var next=Ui.Text("",true);next.TextWrapping=TextWrapping.Wrap;
        void Selection(){next.Text=list.SelectedItem is ProbeTemplate t?$"应用版本：{t.Name} · {(t.Version==0?"内置":$"v{t.Version}")}\n"+(device.Online?"确认后重新应用，等待探针确认。":"设备离线，将在下次连接时应用。")+(t.NeighborCapabilityError(device.Registration.Capabilities) is {} capabilityError?"\n⚠ "+capabilityError:"")+(profile.Monitoring!=null||profile.PropertyIntervals?.Count>0?"\n原非接口采样覆盖恢复为模板设置，接口设置保留。":""):"请选择要应用的模板。";}
        list.SelectionChanged+=(_,_)=>Selection();Selection();
        search.TextChanged+=(_,_)=>{var selected=list.SelectedItem;list.ItemsSource=choices.Where(t=>t.ToString().Contains(search.Text,StringComparison.OrdinalIgnoreCase)).ToArray();if(list.Items.Contains(selected))list.SelectedItem=selected;};
        var content=Ui.Page(new StackPanel {Children={note,Ui.Labeled("搜索模板名称或 ID",search)}},list,Ui.Note(""));
        content.Children.RemoveAt(1);DockPanel.SetDock(next,Dock.Bottom);content.Children.Insert(1,next);
        new ActionWindow(this,"选择设备模板",content,"应用",async()=>{
            if(owner!=connection||closing||selectedDevice!=device.DeviceId)throw new OperationCanceledException();
            if(list.SelectedItem is not ProbeTemplate target)throw new InvalidOperationException("请从列表选择模板。");
            if(target.NeighborCapabilityError(device.Registration.Capabilities) is {} capabilityError)throw new InvalidOperationException(capabilityError);
            await SaveProfile(device,"managed",profile.Name,profile.ModelId,target.TemplateId,target.Version,true,profile.Monitoring,profile.PropertyIntervals);
        }).ShowDialog();
    }

    private void QuickTemplateClick(object sender, RoutedEventArgs e)
    {
        if (Device is { } device) _ = Run("更新模板", () => UpdateDeviceTemplate(device));
    }

    private static string AppliedTemplateVersion(Device device) => device.ActiveTemplate is { } active
        ? active.TemplateId == "builtin" ? "内置采集" : $"v{active.Version}"
        : "尚未确认";

    private static bool HasTemplateUpdate(Device device) => device.Profile?.LatestTemplate is { } latest &&
        latest.Version > (device.ActiveTemplate?.TemplateId == latest.TemplateId ? device.ActiveTemplate.Version : device.Profile.BoundTemplate?.Version ?? 0);
}
