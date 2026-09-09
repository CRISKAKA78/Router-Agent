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
    public static string Normalize(string path) {
        if(!path.StartsWith('/')||path.Contains('\0'))throw new ArgumentException("请输入不含 NUL 的绝对路径。");
        var parts=new List<string>();foreach(var p in path.Split('/')){if(p is "" or ".")continue;if(p==".."){if(parts.Count>0)parts.RemoveAt(parts.Count-1);}else parts.Add(p);}
        return "/"+string.Join('/',parts);
    }
    public static string FileName(string path){var name=Normalize(path).Split('/').Last();return name.Length==0?throw new ArgumentException("请填写包含文件名的完整路径。"):name;}
    public static string Join(string directory,string name)=>Normalize(directory.TrimEnd('/')+"/"+name);
    public static string Parent(string path){var value=Normalize(path);var at=value.LastIndexOf('/');return at<=0?"/":value[..at];}
    public static Task<(RemoteEntry[] Entries,bool Limited)> ReadAsync(WorkspaceConnection owner,string deviceId,string path,CancellationToken token)=>owner.TrackAsync(async()=>{
        token.ThrowIfCancellationRequested();
        var response=await owner.ExecuteAsync(new Mutation("读取设备目录","tasks",new{device_id=deviceId,command=Command(Normalize(path)),timeout_seconds=10}));
        var taskId=response.GetProperty("task_id").GetString()!;
        using var cancel=CancellationTokenSource.CreateLinkedTokenSource(token,owner.Token);cancel.CancelAfter(TimeSpan.FromSeconds(20));
        while(true){var task=await owner.Api.GetAsync<TaskDetail>("tasks/"+ApiClient.Segment(taskId),cancel.Token);
            if(task.Result is {} result){if(result.Status!="success"||result.ExitCode!=0)throw new IOException("读取目录失败："+result.Stderr);if(result.Truncated)throw new IOException("目录响应超出上限，请选择更小的子目录。");return Parse(result.Stdout);}
            if(task.State=="rejected")throw new IOException("设备拒绝读取目录。");await Task.Delay(150,cancel.Token);}
    });
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
