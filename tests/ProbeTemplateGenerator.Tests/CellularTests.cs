using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
using System.Text.Json;
namespace ProbeTemplateGenerator.Tests;
public class CellularTests
{
 [Fact] public void CellularOnlyTemplateCompilesAndCopies(){
  var p=new TemplateProject{Name="自动AT",CellularProbe=new(){IntervalSeconds=30}};
  var compiler=new TemplateCompiler();Assert.Empty(compiler.Validate(p));var t=compiler.Compile(p);
  p.CellularProbe.IntervalSeconds=60;Assert.Equal(30,t.CellularProbe!.IntervalSeconds);Assert.Empty(t.Properties);
  var files=new ProjectFiles();var restored=files.ReadProject(files.SerializeTemplate(t));Assert.Equal(30,restored.CellularProbe!.IntervalSeconds);
  restored=files.ReadProject(files.SerializeProject(restored));Assert.Equal(8,restored.SchemaVersion);Assert.Equal(30,restored.CellularProbe!.IntervalSeconds);
  var from=files.FromTemplate(t);from.CellularProbe!.IntervalSeconds=90;Assert.Equal(30,t.CellularProbe.IntervalSeconds);
 }
 [Theory][InlineData(0)][InlineData(9)][InlineData(-1)][InlineData(86401)]
 public void InvalidIntervalRejects(int interval){var p=new TemplateProject{Name="AT",CellularProbe=new(){IntervalSeconds=interval}};Assert.Contains(new TemplateCompiler().Validate(p),v=>v.Field=="cellular_probe.interval_seconds");}
 [Fact]public void DisabledConfigurationIsOmitted(){var p=new TemplateProject{Name="原模板",Monitoring=new()};var json=new ProjectFiles().SerializeTemplate(new TemplateCompiler().Compile(p));Assert.DoesNotContain("cellular_probe",json);}
 [Fact]public void ProjectAndRuntimeSerializationUseSameConfiguration(){var files=new ProjectFiles();var p=new TemplateProject{Name="AT",CellularProbe=new()};var runtime=new TemplateCompiler().Compile(p);using var json=JsonDocument.Parse(files.SerializeTemplate(runtime));Assert.Equal(30,json.RootElement.GetProperty("cellular_probe").GetProperty("interval_seconds").GetInt32());Assert.Null(files.ReadProject(files.SerializeProject(new TemplateProject{Name="disabled"})).CellularProbe);}
}
