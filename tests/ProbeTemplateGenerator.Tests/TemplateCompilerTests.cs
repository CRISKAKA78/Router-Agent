using System.Diagnostics;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;

namespace ProbeTemplateGenerator.Tests;

public class TemplateCompilerTests
{
    private readonly TemplateCompiler compiler = new();
    private readonly ProjectFiles files = new();

    internal static TemplateAttribute Attribute(string key, AttributeSource source, string input, string sample = "", AttributeVisibility visibility = AttributeVisibility.Virtual) => new()
    {
        Key = key, Name = key, Source = source, Input = input, Sample = sample, Visibility = visibility
    };
    internal static TemplateProject Formula(string formula, string a = "10", string b = "4") => new()
    {
        Name = "测试模板", Attributes =
        [
            Attribute("a", AttributeSource.Command, $"printf '%s' {ShellQuote(a)}", a),
            Attribute("b", AttributeSource.Command, $"printf '%s' {ShellQuote(b)}", b),
            Attribute("answer", AttributeSource.Expression, formula, visibility: AttributeVisibility.Display)
        ]
    };
    internal static string ShellQuote(string value) => "'" + value.Replace("'", "'\"'\"'", StringComparison.Ordinal) + "'";

    public static IEnumerable<object[]> FormulaCases()
    {
        var fixtures = JsonNode.Parse(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "Fixtures", "formula-cases.json")))!.AsArray();
        return fixtures.Select(fixture => new object[] { fixture!["name"]!.GetValue<string>(), fixture.ToJsonString() });
    }

    [Theory]
    [MemberData(nameof(FormulaCases))]
    public void FormulaCasesMatchExpectedRuntimeAndPreview(string name, string fixtureJson)
    {
        var fixture = JsonNode.Parse(fixtureJson)!;
        var project = files.ReadProject(fixture["project"]!.ToJsonString());
        var actual = JsonNode.Parse(files.SerializeTemplate(compiler.Compile(project)));
        Assert.True(JsonNode.DeepEquals(fixture["template"], actual), $"Runtime output differs: {name}");
        var preview = compiler.Preview(project);
        var expected = fixture["preview"]!.AsArray();
        Assert.Equal(expected.Count, preview.Count);
        for (var i = 0; i < preview.Count; i++)
        {
            Assert.Equal(expected[i]!["key"]!.GetValue<string>(), preview[i].Key);
            Assert.Equal(expected[i]!["name"]!.GetValue<string>(), preview[i].Name);
            Assert.Equal(expected[i]!["value"]?.GetValue<string>(), preview[i].Value);
            Assert.Equal(expected[i]!["error"]?.GetValue<string>(), preview[i].Error);
            Assert.Equal(expected[i]!["matchedRule"]?.GetValue<int>(), preview[i].MatchedRule);
        }
    }

    [Theory]
    [InlineData("(a - b) / b * 100", "150")]
    [InlineData("a + b * 2", "18")]
    [InlineData("a / b", "2.5")]
    [InlineData("a > b && !(b == 0)", "1")]
    [InlineData("a < b || b != 0", "1")]
    [InlineData("非 (a < b) 与 true", "1")]
    [InlineData("not false AND a >= b OR false", "1")]
    [InlineData("-a + +b", "-6")]
    [InlineData(".5 + 1e2", "100.5")]
    [InlineData("false && (1 / 0)", "0")]
    [InlineData("true || (1 / 0)", "1")]
    public void EvaluatesArithmeticAndLogicalExpressions(string expression, string value) => Assert.Equal(value, compiler.Preview(Formula(expression))[0].Value);

    [Fact]
    public void CompilesOnlyDisplayedPropertiesInDependencyOrder()
    {
        var project = Formula("middle + b");
        project.Attributes.Add(Attribute("middle", AttributeSource.Expression, "a * 2"));
        var runtime = compiler.Compile(project);
        Assert.Equal(["answer"], runtime.Properties.Keys);
        Assert.Equal("24", compiler.Preview(project)[0].Value);
        Assert.Contains("awk", runtime.Properties["answer"].Command);
    }

    [Fact]
    public void PreservesDirectConfigSourcesAndStandardFields()
    {
        var project = new TemplateProject
        {
            Attributes =
            [
                Attribute("serial", AttributeSource.Nvram, "SN", "001", AttributeVisibility.Display),
                Attribute("hostname", AttributeSource.Uci, "system.@system[0].hostname", "router", AttributeVisibility.Display),
                Attribute("kernel", AttributeSource.Command, "uname -r", "6.1", AttributeVisibility.Display)
            ]
        };
        var runtime = compiler.Compile(project);
        Assert.Equal("nvram", runtime.Properties["serial"].Source);
        Assert.Equal("SN", runtime.Properties["serial"].Key);
        Assert.Null(runtime.Properties["serial"].Command);
        Assert.Equal("uci", runtime.Properties["hostname"].Source);
        Assert.Equal("uname -r", runtime.Properties["kernel"].Command);
        Assert.Null(runtime.Properties["kernel"].Source);
        Assert.Equal(["001", "router", "6.1"], compiler.Preview(project).Select(row => row.Value));
    }

    [Fact]
    public void RejectsMissingReferencesCyclesDuplicatesAndReservedKeys()
    {
        Assert.Contains("未定义", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Formula("missing + a"))).Message);
        var project = Formula("middle");
        project.Attributes.Add(Attribute("middle", AttributeSource.Expression, "answer"));
        Assert.Contains("循环引用", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
        project.Attributes[3] = Attribute("a", AttributeSource.Command, "printf 1");
        Assert.Contains("重复", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
        project.Attributes[3] = Attribute("device_id", AttributeSource.Command, "printf 1");
        Assert.Contains("保留", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
    }

    [Theory]
    [InlineData("a / b", "1", "0", "除数")]
    [InlineData("a + b", "hello", "1", "数字")]
    [InlineData("a + b", "1e309", "1", "有限")]
    [InlineData("a + b", "", "1", "数字")]
    [InlineData("a * a", "1e200", "1", "有限")]
    public void ReportsInvalidSamplesPerDisplay(string formula, string a, string b, string error) => Assert.Contains(error, compiler.Preview(Formula(formula, a, b))[0].Error);

    [Fact]
    public void AcceptsBooleanSamplesAndPrototypeLikeKeys()
    {
        Assert.Equal("1", compiler.Preview(Formula("a && !b", "true", "false"))[0].Value);
        var project = Formula("constructor + b");
        project.Attributes[0].Key = "constructor";
        Assert.Equal("14", compiler.Preview(project)[0].Value);
        Assert.Contains("awk", compiler.Compile(project).Properties["answer"].Command);
    }

    [Theory]
    [InlineData("a; system(\"x\")")]
    [InlineData("a ? 1 : 2")]
    [InlineData("a = 3")]
    [InlineData("a +")]
    [InlineData("(a + b")]
    [InlineData("a ** b")]
    [InlineData("a == \"1\"")]
    public void RejectsNonLanguageSyntax(string expression) => Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Formula(expression)));

    [Fact]
    public void EnforcesLanguageComplexityAndFiniteLimits()
    {
        Assert.Contains("1024", Assert.ThrowsAny<InvalidOperationException>(() => ExpressionParser.Parse(new string('1', 1025))).Message);
        Assert.Contains("256", Assert.ThrowsAny<InvalidOperationException>(() => ExpressionParser.Parse(string.Join('+', Enumerable.Repeat("1", 129)))).Message);
        Assert.Contains("32", Assert.ThrowsAny<InvalidOperationException>(() => ExpressionParser.Parse(new string('(', 34) + "1" + new string(')', 34))).Message);
        Assert.Contains("有限", Assert.ThrowsAny<InvalidOperationException>(() => ExpressionParser.Parse("1e309")).Message);
    }

    [Fact]
    public void EnforcesCommandsDisplayCountKeysAndSerializedSize()
    {
        var project = Formula("a + b");
        project.Attributes[0].Input = new string('x', 4000);
        Assert.Contains("4096", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
        project.Attributes = Enumerable.Range(0, 33).Select(i => Attribute("prop" + i, AttributeSource.Command, "printf 1", visibility: AttributeVisibility.Display)).ToList();
        Assert.Contains("32", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
        project.Attributes = [Attribute("value", AttributeSource.Uci, "system.@system[0];id.hostname", visibility: AttributeVisibility.Display)];
        Assert.Contains("UCI", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
        project.Attributes = [Attribute("value", AttributeSource.Nvram, "-x", visibility: AttributeVisibility.Display)];
        Assert.Contains("NVRAM", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
        project.Attributes = Enumerable.Range(0, 20).Select(i => Attribute("prop" + i, AttributeSource.Command, new string('<', 1000), visibility: AttributeVisibility.Display)).ToList();
        Assert.Contains("48 KiB", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
    }

    [Fact]
    public void FieldValidationReturnsAllLocalErrorsAndGraphOwner()
    {
        var project = Formula("a + b");
        project.Name = "";
        project.Attributes[0].Timeout = 1.5;
        project.Attributes[1].Key = "device_id";
        var issues = compiler.Validate(project);
        Assert.Contains(issues, issue => issue.AttributeId is null && issue.Field == "Name");
        Assert.Contains(issues, issue => issue.AttributeId == project.Attributes[0].Id && issue.Field == "Timeout");
        Assert.Contains(issues, issue => issue.AttributeId == project.Attributes[1].Id && issue.Field == "Key");
        project = Formula("missing + b");
        var graphIssue = Assert.Single(compiler.Validate(project));
        Assert.Equal(project.Attributes[2].Id, graphIssue.AttributeId);
        Assert.Equal("Input", graphIssue.Field);
    }

    [Fact]
    public void CopiesSettingsAndIndependentRulesWithoutChangingReferences()
    {
        var project = Formula("a / b");
        var source = project.Attributes[2];
        source.Timeout = 12;
        var copy = TemplateCompiler.CopyAttribute(project.Attributes, source.Id);
        Assert.NotEqual(source.Id, copy.Id);
        Assert.Equal("answer_copy", copy.Key);
        Assert.Equal("answer（副本）", copy.Name);
        Assert.Equal(source.Input, copy.Input);
        Assert.Equal(12, copy.Timeout);
        copy.Input = "a + b";
        copy.Sample = "99";
        Assert.Equal("a / b", source.Input);
        Assert.Equal("", source.Sample);
        var virtualCopy = TemplateCompiler.CopyAttribute(project.Attributes, project.Attributes[0].Id);
        Assert.Equal(AttributeVisibility.Virtual, virtualCopy.Visibility);
        Assert.Equal("10", virtualCopy.Sample);
    }

    [Fact]
    public void CopyKeepsUniqueBoundedIdentifiersAndUnicodeNames()
    {
        var source = Attribute(new string('x', 64), AttributeSource.Command, "printf 1", "1", AttributeVisibility.Display);
        source.Name = string.Concat(Enumerable.Repeat("路😀", 30));
        var attributes = new List<TemplateAttribute> { source };
        attributes.Add(TemplateCompiler.CopyAttribute(attributes, source.Id));
        var second = TemplateCompiler.CopyAttribute(attributes, source.Id);
        Assert.Equal(64, second.Key.Length);
        Assert.EndsWith("_copy_2", second.Key);
        Assert.True(Encoding.UTF8.GetByteCount(second.Name) <= 128);
        Assert.DoesNotContain('\ufffd', second.Name);
        source.Name = "路";
        attributes.Add(second);
        Assert.Equal(3, compiler.Compile(new() { Attributes = attributes }).Properties.Count);
        Assert.ThrowsAny<InvalidOperationException>(() => TemplateCompiler.CopyAttribute(attributes, "missing"));
        Assert.ThrowsAny<InvalidOperationException>(() => TemplateCompiler.CopyAttribute(Enumerable.Repeat(source, 128).ToArray(), source.Id));
    }

    [Fact]
    public void ProjectFilesPreserveCurrentInvalidEditableDraft()
    {
        var project = TemplateCompiler.ExampleProject();
        var restored = files.ReadProject(files.SerializeProject(project));
        Assert.NotEqual(project.Attributes[0].Id, restored.Attributes[0].Id);
        Assert.Equal(files.SerializeTemplate(compiler.Compile(project)), files.SerializeTemplate(compiler.Compile(restored)));
        var json = JsonNode.Parse(files.SerializeProject(project))!;
        json["schema_version"] = 8;
        json["attributes"]![0]!["timeout"] = 1.5;
        json["attributes"]![0]!["input"] = "";
        restored = files.ReadProject(json.ToJsonString());
        Assert.Equal(8,restored.SchemaVersion);
        Assert.Equal(1.5, restored.Attributes[0].Timeout);
        Assert.Contains(compiler.Validate(restored), issue => issue.Field == "Timeout");
        Assert.ThrowsAny<InvalidOperationException>(() => files.ReadProject("{\"format\":\"router-agent-template-project\",\"schema_version\":2}"));
        Assert.ThrowsAny<InvalidOperationException>(() => files.ReadProject(new string(' ', 1024 * 1024 + 1)));
    }

    [Fact]
    public void ImportsOldRuntimeTemplatesAndNormalizesDefaultTimeout()
    {
        const string legacy = """{"name":"旧模板","properties":{"kernel":{"name":"内核","command":"uname -r","timeout_seconds":5}}}""";
        var runtime = compiler.Compile(files.ReadProject(legacy));
        Assert.True(JsonNode.DeepEquals(JsonNode.Parse(legacy), JsonNode.Parse(files.SerializeTemplate(runtime))));
        var explicitCommand = legacy.Replace("\"command\":", "\"source\":\"command\",\"command\":", StringComparison.Ordinal).Replace(":5", ":0", StringComparison.Ordinal);
        Assert.Equal(5, compiler.Compile(files.ReadProject(explicitCommand)).Properties["kernel"].TimeoutSeconds);
        Assert.Equal("uname -r", compiler.Compile(files.FromTemplate(runtime)).Properties["kernel"].Command);
    }
}

public sealed class BusyBoxFactAttribute : FactAttribute
{
    public BusyBoxFactAttribute()
    {
        if (string.IsNullOrWhiteSpace(Environment.GetEnvironmentVariable("RMP_GENERATOR_WSL"))) Skip = "Set RMP_GENERATOR_WSL to an existing BusyBox test distribution.";
    }
}
public sealed class BusyBoxTheoryAttribute : TheoryAttribute
{
    public BusyBoxTheoryAttribute()
    {
        if (string.IsNullOrWhiteSpace(Environment.GetEnvironmentVariable("RMP_GENERATOR_WSL"))) Skip = "Set RMP_GENERATOR_WSL to an existing BusyBox test distribution.";
    }
}

public class BusyBoxCompilerTests
{
    private readonly TemplateCompiler compiler = new();
    internal static async Task<(int ExitCode, string Output, string Error)> Execute(TemplateProject project)
    {
        var start = new ProcessStartInfo("wsl.exe")
        {
            RedirectStandardInput = true, RedirectStandardOutput = true, RedirectStandardError = true,
            StandardInputEncoding = new UTF8Encoding(false), StandardOutputEncoding = Encoding.UTF8, StandardErrorEncoding = Encoding.UTF8,
            UseShellExecute = false, CreateNoWindow = true
        };
        foreach (var argument in new[] { "-d", Environment.GetEnvironmentVariable("RMP_GENERATOR_WSL")!, "-u", "root", "--", "/bin/sh", "-s" }) start.ArgumentList.Add(argument);
        using var process = Process.Start(start)!;
        var output = process.StandardOutput.ReadToEndAsync();
        var error = process.StandardError.ReadToEndAsync();
        await process.StandardInput.WriteAsync(new TemplateCompiler().Compile(project).Properties["answer"].Command);
        process.StandardInput.Close();
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(15));
        try { await process.WaitForExitAsync(timeout.Token); }
        catch (OperationCanceledException) { process.Kill(entireProcessTree: true); throw; }
        return (process.ExitCode, await output, await error);
    }

    [BusyBoxTheory]
    [InlineData("(a - b) / b * 100")]
    [InlineData("a + b * 2")]
    [InlineData("a > b && !(b == 0)")]
    [InlineData("非 (a < b) 与 true")]
    [InlineData("false && (1 / 0)")]
    [InlineData("-a / (b * .5)")]
    [InlineData("a + 1e2")]
    public async Task GeneratedCommandMatchesPreview(string expression)
    {
        var project = TemplateCompilerTests.Formula(expression, "10.5", "4");
        var result = await Execute(project);
        Assert.True(result.ExitCode == 0, result.Error);
        Assert.Equal(compiler.Preview(project)[0].Value, result.Output.Trim());
    }

    [BusyBoxTheory]
    [InlineData("a / b", "1", "0")]
    [InlineData("a + b", "oops", "1")]
    [InlineData("a + b", "1e309", "2")]
    [InlineData("a * a", "1e200", "1")]
    public async Task InvalidArithmeticFailsWithoutOutput(string expression, string a, string b)
    {
        var result = await Execute(TemplateCompilerTests.Formula(expression, a, b));
        Assert.NotEqual(0, result.ExitCode);
        Assert.Equal("", result.Output);
    }

    [BusyBoxFact]
    public async Task TreatsSourceOutputAsDataAndPropagatesSourceFailure()
    {
        var project = TemplateCompilerTests.Formula("a + b");
        project.Attributes[0].Input = "printf '%s' '1; system(\"echo INJECTED\")'";
        var result = await Execute(project);
        Assert.NotEqual(0, result.ExitCode);
        Assert.DoesNotContain("INJECTED", result.Output);
        project.Attributes[0].Input = "exit 7";
        result = await Execute(project);
        Assert.Equal(64, result.ExitCode);
        Assert.Equal("", result.Output);
    }

    [BusyBoxFact]
    public async Task CollectsSharedDependenciesOnceInsideEachCommand()
    {
        var project = TemplateCompilerTests.Formula("middle + a");
        project.Attributes.Add(TemplateCompilerTests.Attribute("middle", AttributeSource.Expression, "a * 2"));
        var result = await Execute(project);
        Assert.Equal(0, result.ExitCode);
        Assert.Equal("30", result.Output.Trim());
        var command = compiler.Compile(project).Properties["answer"].Command!;
        Assert.Equal(1, command.Split("RA_v0=$(", StringSplitOptions.None).Length - 1);
    }
}
