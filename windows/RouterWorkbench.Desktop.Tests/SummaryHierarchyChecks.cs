using System.Globalization;
using System.Windows;
using System.Windows.Media;
using RouterWorkbench.Desktop;

namespace RouterWorkbench.Desktop.Tests;

internal static partial class Program
{
    private static void SummaryAllocationChecks()
    {
        var family = ((FontFamily)Application.Current.FindResource("UiFontFamily")).Source;
        var originalSize = (double)Application.Current.FindResource("UiFontSize");
        var panel = new FlexibleSummaryPanel();
        var items = new[] {
            ("状态", "在线", 76d, 1d), ("固件版本", "v1.1.20260909-enterprise-router-release", 120d, 3d),
            ("开机时长", "2天3小时4分钟5秒", 106d, 1d), ("出口 IP", "240e:3b6:d051:ffff:1234:5678:90ab:1af6", 140d, 3d),
            ("运营商", "中国电信互联网骨干网", 86d, 1d), ("归属地", "中国·广东省·深圳市", 96d, 1d)
        }.Select(item => { var block = new SummaryBlock(item.Item1, item.Item3, item.Item4); block.Update(item.Item2); panel.Children.Add(block); return block; }).ToArray();
        try
        {
            foreach (var size in new[] { 10d, 13d, 18d, 24d })
            {
                Typography.Apply(family, size);
                foreach (var width in new[] { 640d, 1280d, 2600d })
                {
                    panel.Measure(new(width, double.PositiveInfinity)); panel.Arrange(new(0, 0, width, panel.DesiredSize.Height));
                    var rects = items.Select(b => new Rect(b.TranslatePoint(new(), panel), b.RenderSize)).ToArray();
                    Check(rects.All(r => r.Left >= 0 && r.Right <= width + 1 && r.Width > 0) && !rects.Where((r, i) => rects.Skip(i + 1).Any(other => r.IntersectsWith(other))).Any(),
                        $"summary blocks fit and do not overlap at {width} DIP / font {size}");
                    Check(rects.GroupBy(r => r.Top).All(row => Math.Abs(row.Max(r => r.Right) - width) < 1),
                        "summary uses each row's available width before truncating values");
                    if (width == 2600)
                        Check(items.All(b => b.Value.ActualWidth + 1 >= new FormattedText(b.Value.Text, CultureInfo.CurrentCulture, FlowDirection.LeftToRight,
                            new Typeface(b.Value.FontFamily, b.Value.FontStyle, b.Value.FontWeight, b.Value.FontStretch), b.Value.FontSize, Brushes.Black, 1).WidthIncludingTrailingWhitespace),
                            "wide summary fully exposes long firmware, IPv6, ISP and place");
                    else if (size == 13 && width == 1280)
                        Check(items[1].ActualWidth > items[0].ActualWidth && items[3].ActualWidth > items[0].ActualWidth,
                            "firmware and IPv6 have more reading space than status when width is constrained");
                }
            }
        }
        finally { Typography.Apply(family, originalSize); }
    }
}
