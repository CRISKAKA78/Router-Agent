using System.Windows;
using System.Windows.Controls;
using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;
public partial class MainWindow
{
    private TextBox samplingSeconds=null!,samplingNames=null!;
    private ComboBox samplingScope=null!;
    private TextBlock samplingState=null!;
    private string samplingIdentity="";
    private UIElement BuildSamplingView()
    {
        samplingSeconds=Ui.Input("5");samplingNames=Ui.Input();samplingScope=Ui.Combo(["模板默认","全部接口","指定接口"],0,double.NaN);
        samplingState=Ui.Text("",true);samplingState.TextWrapping=TextWrapping.Wrap;
        var panel=new StackPanel {Margin=new(20),MaxWidth=560,HorizontalAlignment=HorizontalAlignment.Left};
        panel.Children.Add(Ui.Labeled("采样周期（秒，0关闭）",samplingSeconds));panel.Children.Add(Ui.Labeled("采样接口",samplingScope));panel.Children.Add(Ui.Labeled("接口名称（逗号分隔）",samplingNames));
        samplingScope.SelectionChanged+=(_,_)=>samplingNames.IsEnabled=samplingScope.SelectedIndex==2;
        panel.Children.Add(Ui.Bar(WriteButton("保存接口设置",()=>_ = Run("保存接口设置",()=>SaveSamplingView(false))),WriteButton("恢复模板默认",()=>_ = Run("恢复模板默认",()=>SaveSamplingView(true)))));panel.Children.Add(samplingState);
        return new ScrollViewer {Content=panel,VerticalScrollBarVisibility=ScrollBarVisibility.Auto};
    }
    private void UpdateSamplingView(Device? device)
    {
        if(samplingSeconds==null)return;
        var identity=(connection?.Api.Origin.ToString()??"")+"|"+device?.DeviceId+"|"+device?.Profile?.Version;
        samplingState.Text=device?.Profile is {} profile?ConfigState(profile.ConfigurationState)+(string.IsNullOrEmpty(profile.ConfigurationError)?"":" · "+profile.ConfigurationError):"请选择已纳管设备。";
        if(samplingIdentity==identity)return;samplingIdentity=identity;
        var defaults=device?.Profile?.Monitoring??device?.Profile?.BoundTemplate?.Monitoring??new();var sampling=device?.Profile?.InterfaceSampling;
        samplingSeconds.Text=(sampling?.NetworkSeconds??defaults.NetworkSeconds).ToString();samplingNames.Text=sampling?.NetworkInterfaces??"";
        samplingScope.SelectedIndex=sampling?.NetworkInterfaces is null?0:sampling.NetworkInterfaces.Length==0?1:2;samplingNames.IsEnabled=samplingScope.SelectedIndex==2;
    }
    private async Task SaveSamplingView(bool reset)
    {
        var device=RequireDevice(false);if(!device.Managed)throw new InvalidOperationException("请先纳管设备。");var profile=device.Profile!;
        InterfaceSampling? sampling=null;
        if(!reset){if(!uint.TryParse(samplingSeconds.Text,out var seconds)||seconds>86400)throw new InvalidOperationException("采样周期须为0～86400的整数。");
            var names=samplingScope.SelectedIndex==0?null:samplingScope.SelectedIndex==1?"":samplingNames.Text.Trim();
            if(samplingScope.SelectedIndex==2&&string.IsNullOrEmpty(names))throw new InvalidOperationException("请填写需要采样的接口名称。");sampling=new(seconds,names);}
        await SaveProfile(device,"managed",profile.Name,profile.ModelId,profile.BoundTemplate?.TemplateId??"",0,false,profile.Monitoring,profile.PropertyIntervals,sampling,true);
    }
}
