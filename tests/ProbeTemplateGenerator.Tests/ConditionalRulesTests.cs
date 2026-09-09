using System.Text.Json.Nodes;
using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;

namespace ProbeTemplateGenerator.Tests;

public class ConditionalRulesTests
{
    private readonly TemplateCompiler compiler = new();
    private readonly ProjectFiles files = new();
    internal static List<ResultRule> Mapping() =>
    [
        new() { Condition = "a == 0", Value = "4G" },
        new() { Condition = "a == 1", Value = "5G" },
        new() { Condition = "a == 2", Value = "有线连接" }
    ];
    internal static TemplateProject Fixture(List<ResultRule> rules, string a = "0", string b = "0", string fallback = "其他情况")
    {
        var project = TemplateCompilerTests.Formula("0", a, b);
        project.Attributes[2].Source = AttributeSource.Rules;
        project.Attributes[2].Rules = rules;
        project.Attributes[2].Fallback = fallback;
        return project;
    }
    [Theory]
    [InlineData("0", "4G", 1)]
    [InlineData("1", "5G", 2)]
    [InlineData("2", "有线连接", 3)]
    [InlineData("9", "其他情况", null)]
    public void MapsValuesAndReportsMatchedRule(string sample, string value, int? matched)
    {
        var project = Fixture(Mapping(), sample);
        var preview = Assert.Single(compiler.Preview(project));
        Assert.Equal(value, preview.Value);
        Assert.Equal(matched, preview.MatchedRule);
        Assert.True(preview.UsesRules);
        Assert.Equal(["answer"], compiler.Compile(project).Properties.Keys);
    }
    [Theory]
    [InlineData("1", "0", "仅第一项")]
    [InlineData("0", "1", "仅第二项")]
    [InlineData("2", "3", "两项均有")]
    [InlineData("0", "0", "其他情况")]
    public void CombinesMultipleInputs(string a, string b, string expected)
    {
        var project = Fixture([
            new() { Condition = "a != 0 && b == 0", Value = "仅第一项" },
            new() { Condition = "a == 0 && b != 0", Value = "仅第二项" },
            new() { Condition = "a != 0 && b != 0", Value = "两项均有" }
        ], a, b);
        Assert.Equal(expected, compiler.Preview(project)[0].Value);
    }
    [Fact]
    public void UsesNumericIntermediatesAndTheFirstMatchingRule()
    {
        var project = Fixture([
            new() { Condition = "middle >= 4", Value = "较高" },
            new() { Condition = "middle >= 2", Value = "中间" }
        ], "1", "1");
        project.Attributes.Add(TemplateCompilerTests.Attribute("middle", AttributeSource.Expression, "(a + b) * 2"));
        Assert.Equal("较高", compiler.Preview(project)[0].Value);
        project.Attributes[2].Rules!.Reverse();
        Assert.Equal("中间", compiler.Preview(project)[0].Value);
        Assert.Equal(1, compiler.Preview(project)[0].MatchedRule);
    }
    [Theory]
    [InlineData("01", "0", "保留前导零")]
    [InlineData("1", "0", "其他协议")]
    [InlineData(" dhcp\n", "1", "自动获取")]
    [InlineData("pppoe", "0", "其他情况")]
    [InlineData("", "0", "未配置")]
    public void ComparesRawTextWithoutNumericCoercion(string a, string b, string expected)
    {
        var project = Fixture([
            new() { Condition = "a == \"01\"", Value = "保留前导零" },
            new() { Condition = "a == \"dhcp\" && b != 0", Value = "自动获取" },
            new() { Condition = "a == \"\"", Value = "未配置" },
            new() { Condition = "a != \"pppoe\"", Value = "其他协议" }
        ], a, b);
        Assert.Equal(expected, compiler.Preview(project)[0].Value);
    }
    [Fact]
    public void NeverConvertsNumericErrorsOrUnknownReferencesToDefault()
    {
        Assert.Contains("数字", compiler.Preview(Fixture(Mapping(), "dhcp"))[0].Error);
        Assert.Contains("除数", compiler.Preview(Fixture([new() { Condition = "a / b > 0", Value = "结果" }], "1", "0"))[0].Error);
        Assert.Contains("未定义", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Fixture([new() { Condition = "missing == 0", Value = "结果" }]))).Message);
        Assert.Contains("循环引用", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Fixture([new() { Condition = "answer == 0", Value = "结果" }]))).Message);
        // All numeric dependencies are validated before selection, even if an earlier rule is true.
        var project = Fixture([new() { Condition = "true", Value = "第一项" }, new() { Condition = "a == 0", Value = "后续" }], "bad");
        Assert.Contains("数字", compiler.Preview(project)[0].Error);
    }
    [Fact]
    public void ValidatesRuleCountsResultsAndStandardOutputLimits()
    {
        Assert.Contains("1～32", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Fixture([]))).Message);
        Assert.Contains("1～32", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Fixture(Enumerable.Repeat(new ResultRule { Condition = "true", Value = "值" }, 33).ToList()))).Message);
        Assert.Contains("第 1 条条件", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Fixture([new() { Condition = "", Value = "值" }]))).Message);
        Assert.Contains("显示文本", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Fixture([new() { Condition = "true", Value = "  " }]))).Message);
        Assert.Contains("默认", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Fixture(Mapping(), fallback: ""))).Message);
        Assert.Contains("4096", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Fixture([new() { Condition = "true", Value = new string('x', 4097) }]))).Message);
        var project = Fixture([new() { Condition = "true", Value = "中文" }]);
        project.Attributes[2].Key = "libc";
        Assert.Contains("ASCII", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
        project.Attributes[2].Key = "hostname";
        project.Attributes[2].Rules![0].Value = "line1\nline2";
        Assert.Contains("单行", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
        project = Fixture([new() { Condition = "a == 0", Value = string.Concat(Enumerable.Repeat("中文", 600)) }]);
        Assert.Contains("4096", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
    }
    [Theory]
    [InlineData("\"dhcp\"")]
    [InlineData("a + \"1\"")]
    [InlineData("a > \"0\"")]
    [InlineData("system(\"echo\")")]
    [InlineData("a == \"x\"; exit")]
    [InlineData("(a + 1) == \"2\"")]
    public void RejectsUnsupportedTextExpressions(string condition) =>
        Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(Fixture([new() { Condition = condition, Value = "结果" }])));

    [Fact]
    public void RulesRemainFinalAndNumericFormulasRemainNumeric()
    {
        var project = Fixture(Mapping());
        project.Attributes[2].Visibility = AttributeVisibility.Virtual;
        Assert.Contains("仅用于展示", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
        project.Attributes[2].Visibility = AttributeVisibility.Display;
        project.Attributes.Add(TemplateCompilerTests.Attribute("other", AttributeSource.Expression, "answer + 1", visibility: AttributeVisibility.Display));
        Assert.Contains("最终展示文本", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
        project.Attributes.RemoveAt(3);
        project.Attributes[2].Rules = [new() { Condition = "middle == \"1\"", Value = "结果" }];
        project.Attributes.Add(TemplateCompilerTests.Attribute("middle", AttributeSource.Expression, "1"));
        Assert.Contains("不带引号", Assert.ThrowsAny<InvalidOperationException>(() => compiler.Compile(project)).Message);
    }
    [Fact]
    public void CopiesRulesDeeplyAndRoundTripsWithoutMutatingOriginal()
    {
        var project = Fixture(Mapping());
        var copy = TemplateCompiler.CopyAttribute(project.Attributes, project.Attributes[2].Id);
        copy.Rules![0].Value = "副本";
        copy.Rules.Reverse();
        Assert.Equal("4G", project.Attributes[2].Rules![0].Value);
        var restored = files.ReadProject(files.SerializeProject(project));
        Assert.Equal(files.SerializeTemplate(compiler.Compile(project)), files.SerializeTemplate(compiler.Compile(restored)));
        Assert.Equal(8,restored.SchemaVersion);
    }
    [Theory]
    [InlineData("null")]
    [InlineData("{}")]
    [InlineData("[null]")]
    [InlineData("[{\"condition\":\"true\",\"value\":5}]")]
    public void RejectsMalformedRuleFiles(string rules)
    {
        var node = JsonNode.Parse(files.SerializeProject(Fixture(Mapping())))!;
        node["attributes"]![2]!["rules"] = JsonNode.Parse(rules);
        Assert.ThrowsAny<InvalidOperationException>(() => files.ReadProject(node.ToJsonString()));
    }
    [Fact]
    public void RejectsRulesInVersionOneAndUnknownVersions()
    {
        var node = JsonNode.Parse(files.SerializeProject(Fixture(Mapping())))!;
        node["schema_version"] = 1;
        Assert.Contains("版本", Assert.ThrowsAny<InvalidOperationException>(() => files.ReadProject(node.ToJsonString())).Message);
        node["schema_version"] = 999;
        Assert.Contains("版本", Assert.ThrowsAny<InvalidOperationException>(() => files.ReadProject(node.ToJsonString())).Message);
    }
}

public class BusyBoxRulesTests
{
    private readonly TemplateCompiler compiler = new();
    [BusyBoxTheory]
    [InlineData("0")]
    [InlineData("1")]
    [InlineData("2")]
    [InlineData("9")]
    public async Task MatchesNumericMapping(string sample) => await Matches(ConditionalRulesTests.Fixture(ConditionalRulesTests.Mapping(), sample));
    [BusyBoxTheory]
    [InlineData("1", "0")]
    [InlineData("0", "1")]
    [InlineData("1", "1")]
    [InlineData("0", "0")]
    public async Task MatchesCombinationRules(string a, string b) => await Matches(ConditionalRulesTests.Fixture([
        new() { Condition = "a != 0 && b == 0", Value = "仅第一项" },
        new() { Condition = "a == 0 && b != 0", Value = "仅第二项" },
        new() { Condition = "a != 0 && b != 0", Value = "两项均有" }
    ], a, b));
    [BusyBoxTheory]
    [InlineData("dhcp")]
    [InlineData("pppoe")]
    [InlineData("01")]
    [InlineData("1")]
    [InlineData("")]
    public async Task MatchesLiteralText(string a) => await Matches(ConditionalRulesTests.Fixture([
        new() { Condition = "a == \"dhcp\"", Value = "自动获取" },
        new() { Condition = "a == \"01\"", Value = "前导零" },
        new() { Condition = "a != \"pppoe\" && b == 0", Value = "其他" }
    ], a));
    [BusyBoxFact]
    public async Task OutputsSpecialCharactersAsLiteralData()
    {
        const string value = "中文 \"双引号\" '单引号' 100% \\路径\n第二行 $(printf BAD) `printf BAD`";
        await Matches(ConditionalRulesTests.Fixture([new() { Condition = "a == \"a\\\\b\\\"c\"", Value = value }], "a\\b\"c"));
    }
    [BusyBoxFact]
    public async Task ComputesIntermediatesAndSkipsLaterConditionArithmeticAfterMatch()
    {
        var project = ConditionalRulesTests.Fixture([
            new() { Condition = "middle == 4", Value = "第一结果" },
            new() { Condition = "a / b > 0", Value = "不应执行" }
        ], "2", "0");
        project.Attributes.Add(TemplateCompilerTests.Attribute("middle", AttributeSource.Expression, "a * 2"));
        await Matches(project);
    }
    [BusyBoxTheory]
    [InlineData("bad-number")]
    [InlineData("zero-division")]
    [InlineData("source-failure")]
    public async Task NeverUsesDefaultForFailures(string reason)
    {
        var project = ConditionalRulesTests.Fixture([new() { Condition = "a / b > 0", Value = "正常" }], reason == "bad-number" ? "oops" : "1", reason == "zero-division" ? "0" : "1");
        if (reason == "source-failure") project.Attributes[0].Input = "exit 7";
        var result = await BusyBoxCompilerTests.Execute(project);
        Assert.NotEqual(0, result.ExitCode);
        Assert.Equal("", result.Output);
    }
    private async Task Matches(TemplateProject project)
    {
        var result = await BusyBoxCompilerTests.Execute(project);
        Assert.True(result.ExitCode == 0, result.Error);
        Assert.Equal(compiler.Preview(project)[0].Value, result.Output.Trim());
    }
}
