using System.Collections.Specialized;
using System.Windows;
using System.Windows.Controls;

namespace RouterWorkbench.Desktop;

public sealed class DeviceList : DataGrid
{
    public static readonly DependencyProperty RowNumberProperty = DependencyProperty.RegisterAttached("RowNumber", typeof(int), typeof(DeviceList), new PropertyMetadata(0));
    public static int GetRowNumber(DependencyObject row) => (int)row.GetValue(RowNumberProperty);
    public static void SetRowNumber(DependencyObject row, int value) => row.SetValue(RowNumberProperty, value);
    public DeviceList() => LoadingRow += (_, e) => SetRowNumber(e.Row, e.Row.GetIndex() + 1);
    protected override void OnItemsChanged(NotifyCollectionChangedEventArgs e)
    {
        base.OnItemsChanged(e);
        foreach (var row in TableBehavior.Visuals<DataGridRow>(this)) SetRowNumber(row, row.GetIndex() + 1);
    }
}
