using System.Text.Json;
using System.Text;
using System.Text.Encodings.Web;
using System.Text.RegularExpressions;
using ProbeTemplateGenerator.Models;
using static ProbeTemplateGenerator.Services.ExpressionParser;

namespace ProbeTemplateGenerator.Services;

/// <summary>Validates editable projects, previews samples, and compiles existing Probe runtime templates.</summary>
public sealed partial class TemplateCompiler
{
    private static readonly HashSet<string> Reserved = new(StringComparer.Ordinal)
    {
        "device_id", "arch", "boot_id", "probe_version", "capabilities", "template", "attributes", "collection_errors",
        "true", "false", "and", "or", "not"
    };
    private static readonly Dictionary<string, int> Standard = new(StringComparer.Ordinal)
    {
        ["serial"] = 128, ["model"] = 128, ["firmware"] = 128, ["hostname"] = 255, ["kernel"] = 128, ["libc"] = 64
    };
    private static readonly JsonSerializerOptions CompactJson = new() { Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping };

    [GeneratedRegex("^[A-Za-z0-9_./:][A-Za-z0-9_./:-]*$")]
    private static partial Regex NvramPattern();
    [GeneratedRegex("^[A-Za-z0-9_]+\\.(?:[A-Za-z0-9_]+|@[A-Za-z0-9_]+\\[-?[0-9]+\\])\\.[A-Za-z0-9_]+$")]
    private static partial Regex UciPattern();

    private sealed class ProjectValidationException(ValidationIssue issue) : InvalidOperationException(issue.Message)
    {
        public ValidationIssue Issue { get; } = issue;
    }
    private static void Fail(string message, TemplateAttribute? row = null, string field = "Attributes") =>
        throw new ProjectValidationException(new(row?.Id, field, message));

    public IReadOnlyList<ValidationIssue> Validate(TemplateProject project)
    {
        var issues = ValidateFields(project);
        if (issues.Count != 0) return issues;
        try { Compile(project); }
        catch (ProjectValidationException error) { issues.Add(error.Issue); }
        catch (InvalidOperationException error) { issues.Add(new(null, "Attributes", error.Message)); }
        return issues;
    }

    private static List<ValidationIssue> ValidateFields(TemplateProject project)
    {
        var issues = new List<ValidationIssue>();
 ValidatePresentation(project,issues);
 ValidateNeighbors(project,issues);
        void Check(Action action, TemplateAttribute? row, string field)
        {
            try { action(); }
            catch (InvalidOperationException error) { issues.Add(new(row?.Id, field, error.Message)); }
            catch (JsonException) { issues.Add(new(row?.Id, field, "公式包含无效的文本字符")); }
        }
        if(project.Monitoring is {} m && new[]{m.CpuSeconds,m.MemorySeconds,m.DiskSeconds,m.NetworkSeconds,m.EgressSeconds}.Any(n=>n<0||n>86400)) issues.Add(new(null,"Monitoring","监控周期须为0～86400秒，0为关闭"));
        if(project.Monitoring?.NetworkInterfaces is {Length:>0} interfaces){
            var names=interfaces.Split(',');
            if(names.Length>32||names.Distinct(StringComparer.Ordinal).Count()!=names.Length||names.Any(n=>n is "." or ".."||!System.Text.RegularExpressions.Regex.IsMatch(n,@"^[A-Za-z0-9_.-]{1,15}$")))issues.Add(new(null,"Monitoring","接口名须精确填写，逗号分隔，最多32项，每项1～15字符，不可重复"));
        }
        Check(() => ValidateText(project.Name, 128, "模板名称"), null, "Name");
        if (project.Name != project.Name.Trim()) issues.Add(new(null, "Name", "模板名称首尾不能有空白"));
        if (project.Attributes.Count>128) issues.Add(new(null, "Attributes", "工程最多128个属性"));
        var keys = new HashSet<string>(StringComparer.Ordinal);
        foreach (var row in project.Attributes)
        {
            if (!KeyPattern().IsMatch(row.Key) || Reserved.Contains(row.Key))
                issues.Add(new(row.Id, "Key", $"属性标识“{row.Key}”无效或为保留名称"));
            if (!keys.Add(row.Key)) issues.Add(new(row.Id, "Key", $"属性标识重复：{row.Key}"));
            Check(() => ValidateText(row.Name, 128, $"{row.Key} 的名称"), row, "Name");
            if (!double.IsFinite(row.Timeout) || row.Timeout != Math.Truncate(row.Timeout) || row.Timeout is < 1 or > 30)
                issues.Add(new(row.Id, "Timeout", $"{row.Key} 的超时须为 1～30 秒整数"));
            if(row.IntervalSeconds<0||row.IntervalSeconds>86400) issues.Add(new(row.Id,"IntervalSeconds","周期须为0～86400秒，0为仅启动采集"));
            if (!Enum.IsDefined(row.Source)) issues.Add(new(row.Id, "Source", "获取方式无效"));
            if (!Enum.IsDefined(row.Visibility)) issues.Add(new(row.Id, "Visibility", "属性类型无效"));
            if (row.Source == AttributeSource.Rules)
            {
                if (row.Visibility != AttributeVisibility.Display) issues.Add(new(row.Id, "Visibility", $"{row.Key}：条件结果仅用于展示属性"));
                if (row.Rules is null || row.Rules.Count is < 1 or > 32) issues.Add(new(row.Id, "Rules", $"{row.Key} 需要 1～32 条结果规则"));
                if (row.Rules is not null)
                    for (var i = 0; i < row.Rules.Count; i++)
                    {
                        var index = i;
                        Check(() => ValidateResult(row.Rules[index].Value, row.Key, $"第 {index + 1} 条显示文本"), row, $"Rules[{index}].Value");
                        Check(() =>
                        {
                            try { Parse(row.Rules[index].Condition, true); }
                            catch (InvalidOperationException error) { throw new InvalidOperationException($"{row.Key} 第 {index + 1} 条条件：{error.Message}"); }
                        }, row, $"Rules[{index}].Condition");
                    }
                Check(() => ValidateResult(row.Fallback ?? "", row.Key, "默认显示文本"), row, "Fallback");
            }
            else
            {
                Check(() => ValidateText(row.Input, 4096, $"{row.Key} 的来源或公式"), row, "Input");
                if (row.Input.Length > 0 && string.IsNullOrWhiteSpace(row.Input)) issues.Add(new(row.Id, "Input", $"{row.Key} 的来源不能为空白"));
            }
            if (row.Source == AttributeSource.Expression)
                Check(() =>
                {
                    try { Parse(row.Input); }
                    catch (InvalidOperationException error) { throw new InvalidOperationException($"{row.Key}：{error.Message}"); }
                }, row, "Input");
            if (row.Source == AttributeSource.Nvram && (Bytes(row.Input) > 128 || !NvramPattern().IsMatch(row.Input)))
                issues.Add(new(row.Id, "Input", $"{row.Key} 的 NVRAM 键名无效"));
            if (row.Source == AttributeSource.Uci && (Bytes(row.Input) > 256 || !UciPattern().IsMatch(row.Input)))
                issues.Add(new(row.Id, "Input", $"{row.Key} 的 UCI 路径须为 package.section.option"));
        }
        var display = project.Attributes.Where(row => row.Visibility == AttributeVisibility.Display).ToList();
        if ((display.Count==0 && project.Monitoring is null && project.Presentation is null && project.SwitchProbe is null && project.NeighborProbe is null) || display.Count > 38 || display.Count(row => !Standard.ContainsKey(row.Key)) > 32)
            issues.Add(new(null, "Attributes", "请添加展示属性或内置监控/展示配置；最多 32 个自定义展示属性及 6 个已有字段"));
        return issues;
    }

    private sealed class Analysis
    {
        public Dictionary<string, TemplateAttribute> Rows { get; } = new(StringComparer.Ordinal);
        public Dictionary<string, ExpressionNode> Expressions { get; } = new(StringComparer.Ordinal);
        public Dictionary<string, List<ExpressionNode>> Conditions { get; } = new(StringComparer.Ordinal);
        public List<TemplateAttribute> Display { get; } = [];
        public List<string> Order { get; } = [];
        public IEnumerable<ExpressionNode> Nodes(string key) => Expressions.TryGetValue(key, out var expression)
            ? [expression] : Conditions.GetValueOrDefault(key) ?? [];
        public IEnumerable<string> Refs(string key) => Nodes(key).SelectMany(node => References(node));
        public List<string> Dependencies(string key)
        {
            var needed = new HashSet<string>(StringComparer.Ordinal) { key };
            void Add(string current)
            {
                foreach (var dependency in Refs(current)) if (needed.Add(dependency)) Add(dependency);
            }
            Add(key);
            return Order.Where(needed.Contains).ToList();
        }
        public HashSet<string> NumericDependencies(IEnumerable<string> keys) => keys
            .SelectMany(key => Nodes(key).SelectMany(node => References(node, numericOnly: true))).ToHashSet(StringComparer.Ordinal);
    }

    private static Analysis Analyze(TemplateProject project)
    {
        var issues = ValidateFields(project);
        if (issues.Count > 0) throw new ProjectValidationException(issues[0]);
        var result = new Analysis();
        foreach (var row in project.Attributes)
        {
            result.Rows.Add(row.Key, row);
            if (row.Visibility == AttributeVisibility.Display) result.Display.Add(row);
            if (row.Source == AttributeSource.Expression) result.Expressions.Add(row.Key, Parse(row.Input));
            if (row.Source == AttributeSource.Rules) result.Conditions.Add(row.Key, row.Rules!.Select(rule => Parse(rule.Condition, true)).ToList());
        }
        void ValidateTextReferences(ExpressionNode node, TemplateAttribute owner)
        {
            if (IsTextComparison(node))
            {
                foreach (var key in References(node))
                    if (result.Rows.TryGetValue(key, out var row) && row.Source == AttributeSource.Expression)
                        Fail($"{key} 是数值公式，请使用不带引号的数值比较", owner, "Rules");
            }
            else if (node is UnaryExpression unary) ValidateTextReferences(unary.Operand, owner);
            else if (node is BinaryExpression binary) { ValidateTextReferences(binary.Left, owner); ValidateTextReferences(binary.Right, owner); }
        }
        foreach (var row in project.Attributes)
            foreach (var node in result.Nodes(row.Key)) ValidateTextReferences(node, row);
        var visiting = new HashSet<string>(StringComparer.Ordinal);
        var visited = new HashSet<string>(StringComparer.Ordinal);
        void Visit(string key, List<string> path)
        {
            var owner = path.Count == 0 ? null : result.Rows[path[^1]];
            var field = owner?.Source == AttributeSource.Rules ? "Rules" : "Input";
            if (!result.Rows.ContainsKey(key)) Fail($"未定义属性：{key}（被 {path.LastOrDefault()} 引用）", owner, field);
            if (visiting.Contains(key)) Fail($"循环引用：{string.Join(" → ", path.Append(key))}", owner, field);
            if (path.Count > 0 && result.Rows[key].Source == AttributeSource.Rules)
                Fail($"条件结果 {key} 是最终展示文本，不能被其他属性引用；请引用其虚拟属性或数值公式", owner, field);
            if (visited.Contains(key)) return;
            visiting.Add(key);
            foreach (var dependency in result.Refs(key)) Visit(dependency, [.. path, key]);
            visiting.Remove(key);
            visited.Add(key);
            result.Order.Add(key);
        }
        foreach (var row in project.Attributes) Visit(row.Key, []);
        return result;
    }

    private static void ValidateResult(string value, string key, string label = "展示值")
    {
        var trimmed = Trim(value);
        ValidateText(trimmed, Standard.GetValueOrDefault(key, 4096), label);
        if (Standard.ContainsKey(key) && (trimmed.Contains('\r') || trimmed.Contains('\n') ||
            (key == "libc" && trimmed.Any(c => c is < ' ' or > '~'))))
            throw new InvalidOperationException("已有字段要求单行输出，libc 仅允许可打印 ASCII");
    }

    private static string Quote(string value) => "'" + value.Replace("'", "'\"'\"'", StringComparison.Ordinal) + "'";
    // Only compiler-created names/code enter awk. Source output is ENVIRON data, never awk source or -v arguments.
    private const string AwkFunctions = """
        function bad(){exit 65}
        function finite(x){if(x!=x||x>1e308||x< -1e308)bad();return x}
        function num(s){gsub(/^[ \t\r\n]+|[ \t\r\n]+$/,"",s);if(s=="true")return 1;if(s=="false")return 0;if(length(s)>4096||s!~/^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?$/)bad();return finite(s+0)}
        function div(a,b){if(b==0)bad();return finite(a/b)}
        """;

    public RuntimeTemplate Compile(TemplateProject project)
    {
        var analysis = Analyze(project);
        var output = new RuntimeTemplate { NeighborProbe=project.NeighborProbe?.Copy(),Presentation=project.Presentation?.Copy(),SwitchProbe=project.SwitchProbe?.Copy(),Name = project.Name, Monitoring = project.Monitoring?.Copy() };
        foreach (var row in analysis.Display)
        {
            if (row.Source is not (AttributeSource.Expression or AttributeSource.Rules))
            {
                output.Properties.Add(row.Key, new RuntimeProperty
                {
                    Name = row.Name, IntervalSeconds = row.IntervalSeconds, TimeoutSeconds = (int)row.Timeout,
                    Command = row.Source == AttributeSource.Command ? row.Input : null,
                    Source = row.Source == AttributeSource.Command ? null : row.Source.ToString().ToLowerInvariant(),
                    Key = row.Source == AttributeSource.Command ? null : row.Input
                });
                continue;
            }
            var keys = analysis.Dependencies(row.Key);
            var names = keys.Select((key, i) => (key, name: $"v{i}")).ToDictionary(item => item.key, item => item.name, StringComparer.Ordinal);
            var numericKeys = analysis.NumericDependencies(keys);
            var shell = new List<string> { "# Router-Agent template generator v1", "export LC_ALL=C" };
            var statements = new List<string>();
            foreach (var key in keys)
            {
                var dependency = analysis.Rows[key];
                var name = names[key];
                if (dependency.Source == AttributeSource.Rules) continue;
                if (analysis.Expressions.TryGetValue(key, out var expression))
                {
                    statements.Add($"{name}={AwkExpression(expression, names)}");
                    statements.Add($"{name}s=sprintf(\"%.12g\",{name})");
                }
                else
                {
                    var command = dependency.Source == AttributeSource.Command ? $"/bin/sh -c {Quote(dependency.Input)}"
                        : $"{dependency.Source.ToString().ToLowerInvariant()} get {Quote(dependency.Input)}";
                    shell.Add($"RA_{name}=$({command}) || exit 64");
                    shell.Add($"export RA_{name}");
                    statements.Add($"{name}s=ENVIRON[\"RA_{name}\"]");
                    statements.Add($"gsub(/^[ \\t\\r\\n]+|[ \\t\\r\\n]+$/,\"\",{name}s)");
                    if (numericKeys.Contains(key)) statements.Add($"{name}=num({name}s)");
                }
            }
            if (row.Source == AttributeSource.Rules)
            {
                var conditions = analysis.Conditions[row.Key];
                for (var i = 0; i < conditions.Count; i++)
                    statements.Add($"if({AwkExpression(conditions[i], names)}){{printf \"%s\\n\",{AwkString(Trim(row.Rules![i].Value))};exit}}");
                statements.Add($"printf \"%s\\n\",{AwkString(Trim(row.Fallback!))}");
            }
            else statements.Add($"printf \"%.12g\\n\",finite({names[row.Key]})");
            shell.Add($"awk {Quote(AwkFunctions.Replace("\r\n", "\n", StringComparison.Ordinal) + "\nBEGIN{" + string.Join(';', statements) + "}")} </dev/null");
            var compiledCommand = string.Join('\n', shell);
            if (Bytes(compiledCommand) > 4096) Fail($"{row.Key} 编译后的采集命令超过 4096 字节，请减少依赖或缩短采集指令", row, row.Source == AttributeSource.Rules ? "Rules" : "Input");
            output.Properties.Add(row.Key, new RuntimeProperty { Name = row.Name, Command = compiledCommand, IntervalSeconds = row.IntervalSeconds, TimeoutSeconds = (int)row.Timeout });
        }
        // Match the Go service's HTML escaping when applying its serialized 48 KiB limit.
        var encoded = JsonSerializer.Serialize(output, CompactJson)
            .Replace("<", "\\u003c", StringComparison.Ordinal).Replace(">", "\\u003e", StringComparison.Ordinal)
            .Replace("&", "\\u0026", StringComparison.Ordinal).Replace("\u2028", "\\u2028", StringComparison.Ordinal)
            .Replace("\u2029", "\\u2029", StringComparison.Ordinal);
        if (Bytes(encoded) > 48 * 1024) Fail("生成模板超过服务器 48 KiB 上限，请减少属性或指令");
        return output;
    }

    public IReadOnlyList<PreviewResult> Preview(TemplateProject project)
    {
        var analysis = Analyze(project);
        var output = new List<PreviewResult>();
        foreach (var row in analysis.Display)
        {
            try
            {
                string value;
                int? matchedRule = null;
                if (row.Source is not (AttributeSource.Expression or AttributeSource.Rules)) value = Trim(row.Sample);
                else
                {
                    var values = new Dictionary<string, double>(StringComparer.Ordinal);
                    var raw = new Dictionary<string, string>(StringComparer.Ordinal);
                    var keys = analysis.Dependencies(row.Key);
                    var numericKeys = analysis.NumericDependencies(keys);
                    foreach (var key in keys)
                    {
                        if (analysis.Rows[key].Source == AttributeSource.Rules) continue;
                        try
                        {
                            if (analysis.Expressions.TryGetValue(key, out var expression))
                            {
                                var number = Evaluate(expression, values);
                                values[key] = number;
                                raw[key] = FormatNumber(number);
                            }
                            else
                            {
                                raw[key] = Trim(analysis.Rows[key].Sample);
                                if (numericKeys.Contains(key)) values[key] = Number(raw[key]);
                            }
                        }
                        catch (InvalidOperationException error) { throw new InvalidOperationException($"{key}：{error.Message}"); }
                    }
                    if (row.Source == AttributeSource.Rules)
                    {
                        var index = analysis.Conditions[row.Key].FindIndex(condition => Evaluate(condition, values, raw) != 0);
                        matchedRule = index < 0 ? null : index + 1;
                        value = Trim(index < 0 ? row.Fallback! : row.Rules![index].Value);
                    }
                    else value = FormatNumber(values[row.Key]);
                }
                ValidateResult(value, row.Key);
                output.Add(new(row.Key, row.Name, Value: value, MatchedRule: matchedRule, UsesRules: row.Source == AttributeSource.Rules));
            }
            catch (InvalidOperationException error) { output.Add(new(row.Key, row.Name, Error: error.Message, UsesRules: row.Source == AttributeSource.Rules)); }
        }
        return output;
    }

    public static TemplateAttribute FreshAttribute(AttributeVisibility visibility) => new() { Visibility = visibility };
    public static TemplateAttribute CopyAttribute(IReadOnlyList<TemplateAttribute> attributes, string id)
    {
        if (attributes.Count >= 128) throw new InvalidOperationException("工程最多 128 个属性，无法继续复制");
        var original = attributes.FirstOrDefault(row => row.Id == id) ?? throw new InvalidOperationException("要复制的属性不存在");
        var used = attributes.Select(row => row.Key).ToHashSet(StringComparer.Ordinal);
        var stem = KeyPattern().IsMatch(original.Key) ? original.Key : "attribute";
        string key;
        for (var i = 1; ; i++)
        {
            var suffix = i == 1 ? "_copy" : $"_copy_{i}";
            key = stem[..Math.Min(stem.Length, 64 - suffix.Length)] + suffix;
            if (!used.Contains(key)) break;
        }
        const string nameSuffix = "（副本）";
        var name = "";
        foreach (var character in (original.Name.Length == 0 ? "未命名属性" : original.Name).EnumerateRunes())
        {
            if (Bytes(name + character + nameSuffix) > 128) break;
            name += character.ToString();
        }
        return new TemplateAttribute
        {
            Key = key, Name = name + nameSuffix, Visibility = original.Visibility, Source = original.Source,
            IntervalSeconds=original.IntervalSeconds, Input = original.Input, Timeout = original.Timeout, Sample = original.Sample, Fallback = original.Fallback,
            Rules = original.Rules?.Select(rule => new ResultRule { Condition = rule.Condition, Value = rule.Value }).ToList()
        };
    }

    public static TemplateProject EmptyProject() => new();
    public static TemplateProject ExampleProject() => new()
    {
        Name = "内存与联网示例",
        Attributes =
        [
            new() { Key = "memory_total", Name = "内存总量（KiB）", Visibility = AttributeVisibility.Virtual, Input = "awk '/^MemTotal:/{print $2}' /proc/meminfo", Sample = "262144" },
            new() { Key = "memory_free", Name = "空闲内存（KiB）", Visibility = AttributeVisibility.Virtual, Input = "awk '/^MemFree:/{print $2}' /proc/meminfo", Sample = "65536" },
            new() { Key = "used_percent", Name = "内存非空闲比例（%）", Source = AttributeSource.Expression, Input = "(memory_total - memory_free) / memory_total * 100" },
            new() { Key = "link_up", Name = "链路状态", Visibility = AttributeVisibility.Virtual, Input = "cat /sys/class/net/eth0/carrier", Sample = "1" },
            new() { Key = "has_address", Name = "是否有 IPv4 地址", Visibility = AttributeVisibility.Virtual, Input = "ip -4 addr show dev eth0 | awk '/inet /{ok=1} END{print ok+0}'", Sample = "1" },
            new() { Key = "network_ready", Name = "接口就绪（1/0）", Source = AttributeSource.Expression, Input = "link_up && has_address" }
        ]
    };
    public static TemplateProject RulesExampleProject() => new()
    {
        Name = "条件结果逻辑示例（请按固件修改）",
        Attributes =
        [
            new() { Key = "wan_proto", Name = "虚拟 WAN 连接类型", Visibility = AttributeVisibility.Virtual, Source = AttributeSource.Nvram, Input = "wan_proto", Sample = "0" },
            new() { Key = "link_up", Name = "虚拟链路状态", Visibility = AttributeVisibility.Virtual, Input = "cat /sys/class/net/eth0/carrier", Sample = "1" },
            new() { Key = "wan_type", Name = "WAN 连接类型", Source = AttributeSource.Rules, Rules =
            [
                new() { Condition = "wan_proto == 0", Value = "4G" },
                new() { Condition = "wan_proto == 1", Value = "5G" },
                new() { Condition = "wan_proto == 2", Value = "有线连接" }
            ], Fallback = "未知连接类型" },
            new() { Key = "connection_state", Name = "组合判断结果", Source = AttributeSource.Rules, Rules =
            [
                new() { Condition = "link_up != 0 && wan_proto == 0", Value = "示例：链路已连接，类型为 0" },
                new() { Condition = "link_up != 0 && wan_proto != 0", Value = "示例：链路已连接，类型非 0" },
                new() { Condition = "link_up == 0", Value = "示例：链路未连接" }
            ], Fallback = "其他组合" }
        ]
    };
}
