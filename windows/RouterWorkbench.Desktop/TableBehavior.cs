using System.Collections.Specialized;
using System.Globalization;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Data;
using System.Windows.Input;
using System.Windows.Media;

namespace RouterWorkbench.Desktop;

public static class TableBehavior
{
    private static readonly System.Runtime.CompilerServices.ConditionalWeakTable<DataGrid, HashSet<DataGridColumn>> manualColumns = new();

    public static void FitPropertyColumns(DataGrid grid, IEnumerable<RouterWorkbench.Client.PropertyRow> rows)
    {
        var values = rows.ToArray();
        var manual = manualColumns.GetOrCreateValue(grid);
        var face = new Typeface(grid.FontFamily, grid.FontStyle, grid.FontWeight, grid.FontStretch);
        var dpi = VisualTreeHelper.GetDpi(grid).PixelsPerDip;
        double Measure(string value) => new FormattedText(value.Replace("\r\n", " ").Replace('\n', ' ').Replace('\r', ' '), CultureInfo.CurrentUICulture, grid.FlowDirection, face, grid.FontSize, Brushes.Black, dpi).WidthIncludingTrailingWhitespace + 28;
        for (var i = 0; i < Math.Min(2, grid.Columns.Count); i++)
        {
            var column = grid.Columns[i];
            if (manual.Contains(column)) continue;
            var width = Math.Max(Measure(column.Header?.ToString() ?? ""), values.Select(row => Measure(i == 0 ? row.Name : row.Value)).DefaultIfEmpty(0).Max());
            var existing = column.Width.IsAbsolute ? column.Width.Value : column.MinWidth;
            column.Width = new DataGridLength(Math.Ceiling(Math.Max(existing, width)));
        }
    }
    public static readonly DependencyProperty EnabledProperty = DependencyProperty.RegisterAttached("Enabled",typeof(bool),typeof(TableBehavior),new PropertyMetadata(false,Changed));
    public static void SetEnabled(DependencyObject target,bool value)=>target.SetValue(EnabledProperty,value);
    public static bool GetEnabled(DependencyObject target)=>(bool)target.GetValue(EnabledProperty);
    public static readonly DependencyProperty WrapModeProperty = DependencyProperty.RegisterAttached("WrapMode",typeof(TextWrapping),typeof(TableBehavior),new FrameworkPropertyMetadata(TextWrapping.NoWrap,FrameworkPropertyMetadataOptions.Inherits));
    public static void SetWrapMode(DependencyObject target,TextWrapping value)=>target.SetValue(WrapModeProperty,value);
    public static TextWrapping GetWrapMode(DependencyObject target)=>(TextWrapping)target.GetValue(WrapModeProperty);

    private static void Changed(DependencyObject target,DependencyPropertyChangedEventArgs args)
    {
        if(target is not DataGrid grid || args.NewValue is not true)return;
        var columns=new HashSet<DataGridColumn>();
        DataGridColumn? resizing=null;double startWidth=0;
        void Prepare() {
            foreach(var column in grid.Columns) {
                if(!columns.Add(column)||column is not DataGridTextColumn text||text.Binding is not BindingBase raw)continue;
                text.ClipboardContentBinding=raw;
                if(raw is Binding binding && string.IsNullOrEmpty(text.SortMemberPath))text.SortMemberPath=binding.Path?.Path??"";
                var display=new MultiBinding{Converter=new CellDisplayConverter()};display.Bindings.Add(raw);
                display.Bindings.Add(new Binding{RelativeSource=new(RelativeSourceMode.Self),Path=new PropertyPath(WrapModeProperty)});
                text.Binding=display;
                var style=new Style(typeof(TextBlock),text.ElementStyle??Ui.CellTextStyle());
                var tip=new MultiBinding{Converter=new CellTipConverter()};tip.Bindings.Add(raw);
                var originalTip=text.ElementStyle?.Setters.OfType<Setter>().FirstOrDefault(s=>s.Property==FrameworkElement.ToolTipProperty)?.Value as BindingBase;
                if(originalTip!=null)tip.Bindings.Add(originalTip);
                style.Setters.Add(new Setter(FrameworkElement.ToolTipProperty,tip));text.ElementStyle=style;
            }
        }
        grid.Loaded+=(_,_)=>Prepare();
        grid.Columns.CollectionChanged+=(_,_)=>{if(grid.IsLoaded)Prepare();};
        grid.AddHandler(Thumb.DragStartedEvent,new DragStartedEventHandler((_,e)=>{
            var header=Ancestor<DataGridColumnHeader>(e.OriginalSource as DependencyObject);
            resizing=header?.Column;
            if(e.OriginalSource is Thumb {Name:"PART_LeftHeaderGripper"} && header?.Column is {} current)resizing=grid.Columns.FirstOrDefault(c=>c.DisplayIndex==current.DisplayIndex-1);
            startWidth=resizing?.ActualWidth??0;
        }),true);
        grid.AddHandler(Thumb.DragCompletedEvent,new DragCompletedEventHandler((_,_)=>{
            if(resizing is {} changed && Math.Abs(changed.ActualWidth-startWidth)>0.5) manualColumns.GetOrCreateValue(grid).Add(changed);
            if(resizing is {} column && column.ActualWidth<startWidth-0.5) {
                var style=new Style(typeof(DataGridCell),column.CellStyle??(Style)grid.FindResource(typeof(DataGridCell)));
                style.Setters.Add(new Setter(WrapModeProperty,TextWrapping.Wrap));column.CellStyle=style;
            }
            resizing=null;
        }),true);
    }

    public static T? Ancestor<T>(DependencyObject? node) where T:DependencyObject {
        while(node!=null){if(node is T found)return found;node=node is Visual||node is System.Windows.Media.Media3D.Visual3D?VisualTreeHelper.GetParent(node):LogicalTreeHelper.GetParent(node);}return null;
    }
    public static IEnumerable<T> Visuals<T>(DependencyObject node) where T:DependencyObject {
        for(var i=0;i<VisualTreeHelper.GetChildrenCount(node);i++){var child=VisualTreeHelper.GetChild(node,i);if(child is T found)yield return found;foreach(var nested in Visuals<T>(child))yield return nested;}
    }
    public sealed record Anchor(string? Key,double Top,double Offset);
    public static Anchor Capture(DataGrid grid) {
        var scroll=Visuals<ScrollViewer>(grid).FirstOrDefault();
        var first=Visuals<DataGridRow>(grid).Where(r=>r.IsVisible&&r.Item is RouterWorkbench.Client.PropertyRow)
            .Select(r=>(Row:r,Y:r.TranslatePoint(new Point(),grid).Y)).Where(p=>p.Y+p.Row.ActualHeight>30)
            .OrderBy(p=>p.Y).FirstOrDefault();
        return new((first.Row?.Item as RouterWorkbench.Client.PropertyRow)?.Key,first.Y,scroll?.VerticalOffset??0);
    }
    public static void Restore(DataGrid grid,Anchor anchor) {
        grid.UpdateLayout();var scroll=Visuals<ScrollViewer>(grid).FirstOrDefault();if(scroll==null)return;
        scroll.ScrollToVerticalOffset(anchor.Offset);grid.UpdateLayout();
        var row=Visuals<DataGridRow>(grid).FirstOrDefault(r=>r.IsVisible&&r.Item is RouterWorkbench.Client.PropertyRow p&&p.Key==anchor.Key);
        if(row!=null)scroll.ScrollToVerticalOffset(scroll.VerticalOffset+row.TranslatePoint(new Point(),grid).Y-anchor.Top);
    }
}

public sealed class CellDisplayConverter:IMultiValueConverter {
    public object Convert(object[] values,Type target,object parameter,CultureInfo culture) {
        var text=values[0]==DependencyProperty.UnsetValue?"":values[0]?.ToString()??"";
        return values.Length>1&&values[1] is TextWrapping.Wrap?text:text.Replace("\r\n"," ").Replace('\r',' ').Replace('\n',' ');
    }
    public object[] ConvertBack(object value,Type[] targets,object parameter,CultureInfo culture)=>throw new NotSupportedException();
}
public sealed class CellTipConverter:IMultiValueConverter {
    public object Convert(object[] values,Type target,object parameter,CultureInfo culture)=>string.Join("\n",values.Where(v=>v!=DependencyProperty.UnsetValue&&v is not null).Select(v=>v.ToString()).Where(v=>!string.IsNullOrEmpty(v)).Distinct());
    public object[] ConvertBack(object value,Type[] targets,object parameter,CultureInfo culture)=>throw new NotSupportedException();
}

public sealed class GroupToggleButton:ToggleButton {
    protected override void OnPreviewMouseLeftButtonDown(MouseButtonEventArgs e) {
        if(e.ClickCount>1){e.Handled=true;return;}base.OnPreviewMouseLeftButtonDown(e);
    }
}
