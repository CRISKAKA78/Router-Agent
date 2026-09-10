using System.IO;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Media;
using RouterWorkbench.Core;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;

internal static partial class Program
{
    private static async Task ComponentChecks()
    {
        var window = CreateComponentGallery(); window.Show(); await Task.Delay(100); window.UpdateLayout();
        try {
            var combos = Visuals<ComboBox>(window).ToArray(); var combo = combos.First(c => !c.IsEditable); var editor = combos.First(c => c.IsEditable);
            var toggle = (ToggleButton)combo.Template.FindName("Toggle", combo);
            toggle.IsChecked = true; window.UpdateLayout(); await Task.Delay(50);
            Check(combo.IsDropDownOpen && ((Popup)combo.Template.FindName("PART_Popup", combo)).IsOpen, "dropdown toggle opens the native ComboBox popup");
            ((ComboBoxItem)combo.ItemContainerGenerator.ContainerFromIndex(2)).IsSelected = true; window.UpdateLayout();
            Check(combo.SelectedIndex == 2 && combo.Text == "设备信息", "templated dropdown still updates the selected value");
            combo.IsDropDownOpen = false; window.UpdateLayout(); Check(toggle.IsChecked == false, "native popup dismissal resets the toggle");
            var input = (TextBox)editor.Template.FindName("PART_EditableTextBox", editor); input.Text = "手动输入的新模板"; await Task.Delay(30);
            Check(editor.Text == input.Text && input.IsVisible && !Visuals<TextBlock>(editor).Any(t => t.Text == "默认模板" && t.IsVisible), "editable combo accepts custom text without duplicate selection text");
            editor.IsReadOnly = true; window.UpdateLayout(); Check(input.IsReadOnly, "editable combo preserves its read-only contract"); editor.IsReadOnly = false;
            var search = Visuals<TextBox>(window).First(ControlChrome.GetIsSearch); search.Text = "FNR100"; window.UpdateLayout();
            Check(((TextBlock)search.Template.FindName("Hint", search)).Visibility == Visibility.Collapsed, "search input hides the placeholder when text is present");
            search.Text = ""; var check = Visuals<CheckBox>(window).First(); check.IsChecked = true; window.UpdateLayout();
            Check(((WorkbenchIcon)check.Template.FindName("Mark", check)).Visibility == Visibility.Visible, "checkbox selection shows a vector checkmark"); check.IsChecked = false;
            var dataGrid = Visuals<DataGrid>(window).Single(); var context = dataGrid.ContextMenu!;
            context.PlacementTarget = dataGrid; context.IsOpen = true; window.UpdateLayout(); await Task.Delay(30);
            var more = (MenuItem)context.Items[1]; more.IsSubmenuOpen = true; window.UpdateLayout(); await Task.Delay(30);
            Check(((Popup)more.Template.FindName("PART_Popup", more)).IsOpen && more.Items.Count == 2, "context menu retains the nested submenu and checkable commands");
            more.IsSubmenuOpen = false; context.IsOpen = false;
            foreach (var family in new[] { ServerProfile.DefaultUiFontFamily, "DengXian" })
            foreach (var size in new[] { 13d, 18d, 24d }) {
                Typography.Apply(family, size); window.UpdateLayout();
                var arrow = (WorkbenchIcon)combo.Template.FindName("Arrow", combo);
                var center = arrow.TranslatePoint(new(arrow.ActualWidth / 2, arrow.ActualHeight / 2), combo);
                Check(Math.Abs(center.Y - combo.ActualHeight / 2) <= 0.5 && Math.Abs(center.X - (combo.ActualWidth - 17)) <= 0.5,
                    $"dropdown arrow stays centered in its slot: {family} / {size}");
                Check(arrow.ActualWidth == 20 && arrow.ActualHeight == 20 && combo.ActualHeight >= combo.FontSize + 6,
                    $"arrow size is independent of text size and control height accommodates text ({arrow.ActualWidth}x{arrow.ActualHeight}, control {combo.ActualHeight}, font {combo.FontSize})");
            }
            combo.SelectedIndex = 0; editor.Text = "默认模板";
            foreach (var theme in new[] { "Light", "Dark" })
            foreach (var size in new[] { 13d, 24d }) {
                Theme.Apply(theme); Typography.Apply(ServerProfile.DefaultUiFontFamily, size); window.UpdateLayout();
                Check(window.FontFamily.Source == ServerProfile.DefaultUiFontFamily && ColorOf(combo.Foreground) == ColorOf((Brush)window.FindResource("Text")), $"component text follows {theme} / font {size}");
                Render(window, $"components-{theme.ToLowerInvariant()}-{size}.png");
            }
        }
        finally { window.Close(); Theme.Apply("Light"); Typography.Apply(ServerProfile.DefaultUiFontFamily, 13); }
    }
}
