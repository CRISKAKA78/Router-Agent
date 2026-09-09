using System.Windows;
using System.Windows.Automation.Peers;
using System.Windows.Automation.Provider;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Data;
using RouterWorkbench.Client;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;
internal static partial class Program
{
    private static void VisibilityChecks(Device device)
    {
        var snapshot=device with {Presentation=new([],new(){["device_id"]=new("",0,false),["net_eth0_rx_bytes"]=new("",0,true)},new(){["egress"]=false})};
        var input=new[]{new PropertyRow("","设备 ID","id"){Key="device_id"},new PropertyRow("","网口字节","3"){Key="net_eth0_rx_bytes",MetricGroup="network"},new PropertyRow("","网口状态","up"){Key="net_eth0_state",MetricGroup="network"},new PropertyRow("","出口","1.1.1.1"){Key="egress_ipv4",MetricGroup="egress"},new PropertyRow("","CPU","30%"){Key="cpu_usage",MetricGroup="cpu"}};
        var rows=(PropertyRow[])typeof(MainWindow).GetMethod("PresentRows",System.Reflection.BindingFlags.NonPublic|System.Reflection.BindingFlags.Static)!.Invoke(null,[snapshot,input])!;
        Check(rows.Select(r=>r.Key).Order().SequenceEqual(new[]{"cpu_usage","net_eth0_rx_bytes"}),"per-field visibility overrides categories and hides built-in identity fields");
        Check(rows.All(r=>!r.Name.Contains('(')&&r.ValueTip.Contains(r.Key)),"property labels omit identifiers while tooltip preserves them");
    }
    private static async Task TableRefinementChecks(MainWindow window)
    {
        var grid=Field<DataGrid>(window,"properties");
        var original=grid.ItemsSource;var width=grid.Columns[1].Width;var cellStyle=grid.Columns[1].CellStyle;
        var longValue=string.Join(" ",Enumerable.Repeat("pgqy 中文完整显示",30))+"\n原始第二行";
        var items=Enumerable.Range(0,100).Select(i=>new PropertyRow("长分组",$"字段 {i:D3}",i==0?longValue:$"第 {i} 项 pgqy"){Key=$"test_{i}",GroupId="test-long"}).ToArray();
        try {
            grid.ItemsSource=items;TableBehavior.FitPropertyColumns(grid,items);
            Check(grid.Columns[1].Width.Value>1000,"initial columns fit full values beyond viewport width");
            grid.UpdateLayout();await Task.Delay(70);
            var scroll=Visuals<ScrollViewer>(grid).First();scroll.ScrollToTop();grid.UpdateLayout();
            Check(!grid.CanUserSortColumns&&grid.Columns.Count==2,"property table has no sortable or redundant group column");
            Check(!Visuals<GroupToggleButton>(grid).Any(),"flat property table has no group toggles");
            scroll.ScrollToVerticalOffset(13);grid.UpdateLayout();
            Check(Math.Abs(scroll.VerticalOffset-13)<0.1,"long group scrolls by 13 pixels without jumping to another group");
            scroll.ScrollToVerticalOffset(800);grid.UpdateLayout();
            Check(Visuals<DataGridRow>(grid).Any(r=>r.IsVisible&&r.Item is PropertyRow p&&int.Parse(p.Key[5..]) is >15 and <40),"middle of a group taller than the viewport remains reachable");
            var anchor=TableBehavior.Capture(grid);items[0].Value+=" updated";grid.UpdateLayout();TableBehavior.Restore(grid,anchor);grid.UpdateLayout();
            Check(Math.Abs(scroll.VerticalOffset-anchor.Offset)<1,"value refresh preserves pixel scroll position");
            scroll.ScrollToTop();grid.UpdateLayout();
            var text=Visuals<TextBlock>(grid).First(t=>t.Text.StartsWith("pgqy"));
            Check(text.TextWrapping==TextWrapping.NoWrap&&!text.Text.Contains('\n'),"multiline data defaults to one visual line");
            Check(((Binding)grid.Columns[1].ClipboardContentBinding).Path.Path=="Value"&&items[0].Value.Contains('\n'),"copy binding retains original newlines");
            var columnHeader=Visuals<DataGridColumnHeader>(grid).Single(h=>h.Column==grid.Columns[1]);
            var thumb=Visuals<Thumb>(columnHeader).Single(t=>t.Name=="PART_RightHeaderGripper");
            thumb.RaiseEvent(new DragStartedEventArgs(0,0){RoutedEvent=Thumb.DragStartedEvent});
            grid.Columns[1].Width=190;grid.UpdateLayout();
            thumb.RaiseEvent(new DragCompletedEventArgs(-300,0,false){RoutedEvent=Thumb.DragCompletedEvent});grid.UpdateLayout();await Task.Delay(70);
            var row=Visuals<DataGridRow>(grid).Single(r=>ReferenceEquals(r.Item,items[0]));
            text=Visuals<TextBlock>(row).First(t=>t.Text.StartsWith("pgqy"));
            TableBehavior.FitPropertyColumns(grid,items);Check(grid.Columns[1].Width.Value==190,"sample refresh preserves manually narrowed width");
            Check(text.TextWrapping==TextWrapping.Wrap&&text.Text.Contains('\n')&&row.ActualHeight>100,"manual narrowing wraps full content and grows row height");
            Check(text.ActualHeight>=text.DesiredSize.Height-1,"wrapped glyph descenders fit measured cell height");
            Render(window,"long-group-wrapped.png");
            scroll.ScrollToVerticalOffset(37);grid.UpdateLayout();Check(Math.Abs(scroll.VerticalOffset-37)<1,"wrapped tall row supports partial pixel scrolling");
        } finally {
            grid.Columns[1].Width=width;grid.Columns[1].CellStyle=cellStyle;grid.ItemsSource=original;grid.UpdateLayout();
        }
    }
}
