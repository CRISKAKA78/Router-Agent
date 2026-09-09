using ProbeTemplateGenerator.Models;

namespace ProbeTemplateGenerator.Services;

public sealed record LayoutField(string Key, string Name, string Kind);

public static class DisplayLayout
{
    public static readonly IReadOnlyDictionary<string,string> BuiltinGroups = new Dictionary<string,string> {
        ["builtin_system"]="系统信息", ["builtin_resources"]="资源监控", ["builtin_interfaces"]="接口信息", ["other"]="其他信息"
    };
    public static bool IsBuiltinGroup(string id) => BuiltinGroups.ContainsKey(id);
    public static readonly IReadOnlyDictionary<string,string> Categories = new Dictionary<string,string> {
        ["hardware"]="CPU与硬件详情",["cpu"]="CPU监控",["memory"]="内存",["disk"]="磁盘明细",
        ["network"]="逻辑网口明细",["switch"]="物理端口明细",["egress"]="出口信息"
    };
    public static bool CategoryVisible(TemplateProject project,string category) =>
        project.Presentation?.BuiltinVisibility.GetValueOrDefault(category,DefaultVisible(category)) ?? DefaultVisible(category);
    public static bool DefaultVisible(string category) => category is not ("network" or "switch");
    public static string Category(TemplateProject project,string key) {
        if(project.Attributes.Any(a=>a.Key==key&&a.Visibility==AttributeVisibility.Display))return "template";
        if(key.StartsWith("disk_"))return "disk";
        if(key.StartsWith("net_"))return "network";
        if(key.StartsWith("switch_"))return "switch";
        if(key.StartsWith("memory_"))return "memory";
        if(key.StartsWith("egress_"))return "egress";
        if(key is "cpu_usage" or "cpu_current_mhz")return "cpu";
        return key.StartsWith("cpu_")||key is "kernel_arch" or "kernel_bits" or "probe_bits" or "model" or "firmware"?"hardware":"";
    }
    public static bool IsVisible(TemplateProject project,string key) => Placement(project,key).Visible ?? (key != "session_id" && CategoryVisible(project,Category(project,key)));
    public static IReadOnlyDictionary<string,string> Builtins => RouterWorkbench.Display.DevicePresentation.Builtins;

    public static IEnumerable<LayoutField> Fields(TemplateProject project)
    {
        var fields=Builtins.ToDictionary(p=>p.Key,p=>new LayoutField(p.Key,p.Value,"内置"),StringComparer.Ordinal);
        foreach(var row in project.Attributes.Where(a=>a.Visibility==AttributeVisibility.Display && a.Key.Length>0))
            fields[row.Key]=new(row.Key,row.Name,"自定义");
        foreach(var key in project.Presentation?.Fields.Keys ?? Enumerable.Empty<string>())
            fields.TryAdd(key,new(key,key,"设备字段"));
        var virtuals=project.Attributes.Where(a=>a.Visibility==AttributeVisibility.Virtual).Select(a=>a.Key).ToHashSet(StringComparer.Ordinal);
        return fields.Values.Where(f=>!virtuals.Contains(f.Key)||Builtins.ContainsKey(f.Key));
    }
    public static string DefaultGroup(string key) => RouterWorkbench.Display.DevicePresentation.DefaultGroup(key);
    public static DisplayField Placement(TemplateProject project,string key) => project.Presentation?.Fields.GetValueOrDefault(key) ?? new(){GroupId=DefaultGroup(key),Order=1000000};
    public static IEnumerable<DisplayGroup> CustomGroups(TemplateProject project) => (project.Presentation?.Groups ?? []).OrderBy(g=>g.Order).ThenBy(g=>g.Id,StringComparer.Ordinal);
    public static IEnumerable<DisplayGroup> Groups(TemplateProject project) {
        yield return new(){Id="builtin_system",Name="系统信息"};
        yield return new(){Id="builtin_resources",Name="资源监控"};
        foreach(var group in CustomGroups(project)) yield return group;
        yield return new(){Id="other",Name="其他信息"};
    }
    public static string GroupId(TemplateProject project,string key)
    {
        var id=Placement(project,key).GroupId;
        if(id=="builtin_interfaces")return "other";
        return IsBuiltinGroup(id) || project.Presentation?.Groups.Any(g=>g.Id==id)==true ? id : DefaultGroup(key);
    }
    public static IEnumerable<LayoutField> InGroup(TemplateProject project,string group) => Fields(project).Where(f=>GroupId(project,f.Key)==group)
        .OrderBy(f=>Placement(project,f.Key).Order).ThenBy(f=>f.Key,StringComparer.Ordinal);
}
