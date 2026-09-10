using System.Windows;
using System.Windows.Controls;

namespace RouterWorkbench.Desktop;

internal sealed class ActionWindow : Window
{
    public Button Submit { get; }
    public TextBlock Error { get; } = Ui.Text("");
    public ActionWindow(Window owner,string title,UIElement content,string action,Func<Task> accept)
    {
        Owner=owner;Title=title;Width=670;Height=590;MinWidth=520;MinHeight=380;
        WindowStartupLocation=WindowStartupLocation.CenterOwner;SetResourceReference(StyleProperty,typeof(Window));
        var root=new DockPanel {Margin=new(16)};Content=root;
        var footer=new StackPanel();var footerBand=new Border {Child=footer,Margin=new(0,12,0,0),Padding=new(0,12,0,0),BorderThickness=new(0,1,0,0)};
        footerBand.SetResourceReference(Border.BorderBrushProperty,"Line");DockPanel.SetDock(footerBand,Dock.Bottom);root.Children.Add(footerBand);
        Error.TextWrapping=TextWrapping.Wrap;Error.SetResourceReference(TextBlock.ForegroundProperty,"Error");footer.Children.Add(Error);
        var buttons=new StackPanel {Orientation=Orientation.Horizontal,HorizontalAlignment=HorizontalAlignment.Right,Margin=new(0,12,0,0)};footer.Children.Add(buttons);
        Submit=Ui.Button(action,()=>{},true);Submit.IsDefault=true;Submit.MinWidth=78;
        var cancel=new Button {Content="取消",IsCancel=true,MinWidth=78};buttons.Children.Add(Submit);buttons.Children.Add(cancel);root.Children.Add(content);
        bool busy=false;Closing+=(_,e)=>{if(busy)e.Cancel=true;};
        Submit.Click+=async(_,_)=>{if(busy)return;busy=true;Submit.IsEnabled=cancel.IsEnabled=content.IsEnabled=false;Error.Text="";
            try{await accept();busy=false;DialogResult=true;}
            catch(Exception error){Error.Text=error is OperationCanceledException?"连接或设备已切换，请关闭后重新打开。":error.Message;}
            finally{busy=false;Submit.IsEnabled=cancel.IsEnabled=content.IsEnabled=true;}
        };
    }
}
