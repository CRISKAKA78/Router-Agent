using System.Globalization;

namespace RouterWorkbench.Client;

public sealed record RemoteEntry(string Name, string Kind, long Size, long Modified)
{
    public string KindText => Kind == "d" ? "目录" : "文件";
    public string SizeText => Labels.Bytes(Size);
    public string ModifiedText => Labels.Time(DateTimeOffset.FromUnixTimeSeconds(Modified));
}
public static class RemoteDirectory
{
    public static string Command(string path)
    {
        if (!path.StartsWith('/') || path.Contains('\0')) throw new ArgumentException("请输入不含 NUL 的绝对目录路径。");
        var quoted = "'" + path.Replace("'", "'\\''") + "'";
        return "cd " + quoted + " || exit 1\n" + """
export LC_ALL=C
n=0
for f in .[!.]* ..?* *; do
 [ -e "./$f" ] || [ -L "./$f" ] || continue
 [ "$n" -lt 250 ] || { printf 'LIMIT\0'; break; }
 kind=f; [ ! -d "./$f" ] || kind=d
 meta=$(stat -c '%s %Y' "./$f") || exit 2
 set -- $meta
 printf '%s\0%s\0%s\0%s\0' "$kind" "$1" "$2" "$f"
 n=$((n+1))
done
""";
    }
    public static (RemoteEntry[] Entries, bool Limited) Parse(string output)
    {
        if (output == "") return ([], false);
        if (!output.EndsWith('\0')) throw new InvalidDataException("目录响应不完整。");
        var parts = output.Split('\0').SkipLast(1).ToList(); var limited = parts.LastOrDefault() == "LIMIT";
        if (limited) parts.RemoveAt(parts.Count - 1);
        if (parts.Count % 4 != 0 || parts.Count > 1000) throw new InvalidDataException("目录响应格式错误。");
        var result = new List<RemoteEntry>();
        for (var i = 0; i < parts.Count; i += 4) {
            if (parts[i] is not ("d" or "f") || !long.TryParse(parts[i+1], NumberStyles.None, CultureInfo.InvariantCulture, out var size)
                || !long.TryParse(parts[i+2], out var modified) || modified < -62135596800 || modified > 253402300799
                || parts[i+3].Length == 0 || parts[i+3] is "." or ".." || parts[i+3].Contains('/')) throw new InvalidDataException("目录响应字段无效。");
            result.Add(new(parts[i+3], parts[i], size, modified));
        }
        return ([.. result.OrderBy(e => e.Kind).ThenBy(e => e.Name, StringComparer.Ordinal)], limited);
    }
}
