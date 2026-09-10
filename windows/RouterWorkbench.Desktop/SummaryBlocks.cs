using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Shapes;

namespace RouterWorkbench.Desktop;

// Shared by the persistent device header and the system page's more prominent overview.
public sealed class SummaryBlock : Border
{
    public TextBlock Value { get; } = Ui.Text("—");
    public double MinimumReadingWidth { get; }
    public double Priority { get; }
    private readonly Ellipse dot = new() { Width = 6, Height = 6, Margin = new(0, 0, 6, 0), VerticalAlignment = VerticalAlignment.Center, Visibility = Visibility.Collapsed };
    public SummaryBlock(string label, double minimumWidth, double priority = 1, bool overview = false)
    {
        MinimumReadingWidth = minimumWidth; Priority = priority;
        SetResourceReference(StyleProperty, overview ? "OverviewBlock" : "HeaderInfoBlock");
        var caption = Ui.Text(label, true); caption.SetResourceReference(TextBlock.FontSizeProperty, "UiSmallFontSize");
        var valueLine = new DockPanel(); valueLine.Children.Add(dot); valueLine.Children.Add(Value);
        Value.TextTrimming = overview ? TextTrimming.None : TextTrimming.CharacterEllipsis;
        Value.TextWrapping = overview ? TextWrapping.Wrap : TextWrapping.NoWrap;
        Value.SetResourceReference(TextBlock.FontSizeProperty, overview ? "UiTitleFontSize" : "UiFontSize");
        Value.FontWeight = overview ? FontWeights.SemiBold : FontWeights.Medium;
        Value.SetResourceReference(TextBlock.ForegroundProperty, "Text");
        Value.Margin = new(0, 3, 0, 0); Value.VerticalAlignment = VerticalAlignment.Top;
        Child = new StackPanel { Children = { caption, valueLine } };
        System.Windows.Automation.AutomationProperties.SetName(Value, label);
    }
    public void Update(string value, string? reason = null, string? stateBrush = null)
    {
        Value.Text = value;
        Value.ToolTip = string.Join("\n", new[] { value, reason }.Where(s => !string.IsNullOrEmpty(s)).Distinct());
        dot.Visibility = stateBrush == null ? Visibility.Collapsed : Visibility.Visible;
        if (stateBrush != null) dot.SetResourceReference(Shape.FillProperty, stateBrush);
    }
}

// Allocate all available width before trimming. Firmware/IP receive spare width first;
// no maximum text width or pre-truncated string can hide content while space is free.
public sealed class FlexibleSummaryPanel : Panel
{
    private readonly List<(UIElement Child, Rect Rect)> layout = [];
    protected override Size MeasureOverride(Size availableSize)
    {
        layout.Clear();
        var children = InternalChildren.Cast<SummaryBlock>().Where(c => c.Visibility != Visibility.Collapsed).ToArray();
        if (children.Length == 0) return new();
        var scale = (double)FindResource("UiFontSize") / 13;
        const double gap = 6;
        foreach (var child in children) child.Measure(new(double.PositiveInfinity, double.PositiveInfinity));
        var desired = children.Select(c => c.DesiredSize.Width).ToArray();
        var minimum = children.Select((c, i) => Math.Min(desired[i], c.MinimumReadingWidth * scale)).ToArray();
        var width = double.IsFinite(availableSize.Width) ? Math.Max(1, availableSize.Width) : desired.Sum() + gap * (children.Length - 1);
        var rowCount = Math.Min(children.Length, Math.Max(1, (int)Math.Ceiling((minimum.Sum() + gap * (children.Length - 1)) / width)));
        var perRow = (int)Math.Ceiling((double)children.Length / rowCount);
        double y = 0;
        for (var start = 0; start < children.Length;)
        {
            var count = Math.Min(perRow, children.Length - start);
            // An unusually large font may need fewer items than the balanced estimate.
            while (count > 1 && minimum.Skip(start).Take(count).Sum() + gap * (count - 1) > width) count--;
            var budget = Math.Max(1, width - gap * (count - 1));
            var sizes = minimum.Skip(start).Take(count).Select(v => Math.Min(v, budget)).ToArray();
            var spare = budget - sizes.Sum();
            while (spare > .1)
            {
                var needing = Enumerable.Range(0, count).Where(i => desired[start + i] - sizes[i] > .1).ToArray();
                if (needing.Length == 0) break;
                var totalPriority = needing.Sum(i => children[start + i].Priority);
                double used = 0;
                foreach (var i in needing) { var extra = Math.Min(desired[start + i] - sizes[i], spare * children[start + i].Priority / totalPriority); sizes[i] += extra; used += extra; }
                spare -= used;
            }
            // Fill the row with useful blocks, rather than leaving an empty right edge.
            if (spare > 0) for (var i = 0; i < count; i++) sizes[i] += spare * children[start + i].Priority / children.Skip(start).Take(count).Sum(c => c.Priority);
            double height = 0;
            for (var i = 0; i < count; i++) { children[start + i].Measure(new(sizes[i], double.PositiveInfinity)); height = Math.Max(height, children[start + i].DesiredSize.Height); }
            double x = 0;
            for (var i = 0; i < count; i++) { layout.Add((children[start + i], new(x, y, sizes[i], height))); x += sizes[i] + gap; }
            y += height + gap;
            start += count;
        }
        return new(width, Math.Max(0, y - gap));
    }
    protected override Size ArrangeOverride(Size finalSize)
    {
        foreach (var (child, rect) in layout) child.Arrange(rect);
        return finalSize;
    }
}
