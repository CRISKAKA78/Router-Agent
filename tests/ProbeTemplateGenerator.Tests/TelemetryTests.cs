using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
using ProbeTemplateGenerator.Features.Projects;
namespace ProbeTemplateGenerator.Tests;
public class TelemetryTests
{
    [Fact] public void CyclesSurviveCompileCopyAndBothFileFormats()
    {
        var p = new TemplateProject {Name="Monitoring",Monitoring=new(){CpuSeconds=2,MemorySeconds=3,DiskSeconds=0,NetworkSeconds=1},Attributes=[new(){Key="cpu_usage",Name="CPU",Input="printf 25",IntervalSeconds=7}]};
        var files=new ProjectFiles();var compiler=new TemplateCompiler();var compiled=compiler.Compile(p);
        Assert.Equal(7,compiled.Properties["cpu_usage"].IntervalSeconds);Assert.Equal(0,compiled.Monitoring!.DiskSeconds);
        var restored=files.ReadProject(files.SerializeProject(p));Assert.Equal(8,restored.SchemaVersion);Assert.Equal(7,restored.Attributes[0].IntervalSeconds);Assert.Equal(3,restored.Monitoring!.MemorySeconds);
        restored=files.ReadProject(files.SerializeTemplate(compiled));Assert.Equal(7,restored.Attributes[0].IntervalSeconds);Assert.Equal(1,restored.Monitoring!.NetworkSeconds);
        p.Attributes[0].IntervalSeconds=86401;Assert.NotEmpty(compiler.Validate(p));p.Attributes[0].IntervalSeconds=0;p.Monitoring.CpuSeconds=-1;Assert.NotEmpty(compiler.Validate(p));
    }
    [Theory] [InlineData(1)] [InlineData(2)] public void RejectsPreviousProjectVersions(int version)
    {
        var files=new ProjectFiles();var p=new TemplateProject {SchemaVersion=version,Name="legacy",Attributes=[new(){Key="x",Name="X",Input="printf x"}]};
        Assert.Throws<InvalidOperationException>(()=>files.ReadProject(files.SerializeProject(p)));
    }
}
