using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
using ProbeTemplateGenerator.Features.Projects;
using Xunit;
namespace ProbeTemplateGenerator.Tests;
public class PresentationTests {
 [Fact]public void PresentationAndPortsSurviveBothFormatsAndCompile(){
  var project=new TemplateProject{Name="分组模板",Attributes=[new(){Key="signal",Name="信号",Input="printf 90"}],Presentation=new(){Groups=[new(){Id="radio",Name="4G/5G信息",Order=30}],Fields=new(){["signal"]=new(){GroupId="radio",Order=10},["device_id"]=new(){GroupId="radio",Order=20}},BuiltinVisibility=new(){["network"]=false}},SwitchProbe=new(){Backend="swconfig",Ports=[new(){Id="lan1",SwitchId="switch0",Port=1,SystemName="vlan3",Uplink="eth0",DisplayName="LAN1"}]}};
  var compiler=new TemplateCompiler();var files=new ProjectFiles();var runtime=compiler.Compile(project);
  foreach(var copy in new[]{files.ReadProject(files.SerializeProject(project)),files.ReadProject(files.SerializeTemplate(runtime)),files.FromTemplate(runtime)}){Assert.Equal(8,copy.SchemaVersion);Assert.False(copy.Presentation!.BuiltinVisibility["network"]);Assert.Equal(30,copy.Presentation.Groups[0].Order);Assert.Equal("eth0",copy.SwitchProbe!.Ports[0].Uplink);Assert.Equal("radio",copy.Presentation.Fields["device_id"].GroupId);}
  runtime.Presentation!.Groups[0].Name="changed";Assert.Equal("4G/5G信息",project.Presentation.Groups[0].Name);
 }
 [Fact]public void ModelWritesUseExistingDurableRetryEnvelope(){var q=new PendingTemplateMutation("http://localhost:8080","型号","PUT","device-models/m","{\"name\":\"M\",\"version\":0}","same-key");TemplatePublishingService.ValidatePending(q);Assert.Throws<System.IO.InvalidDataException>(()=>TemplatePublishingService.ValidatePending(q with{Method="DELETE"}));}
 [Fact]public void BuiltinOnlyLayoutNeedsNoArtificialCommand(){var p=new TemplateProject{Name="布局",Presentation=new(){Groups=[new(){Id="basic",Name="基本信息"}]}};var runtime=new TemplateCompiler().Compile(p);Assert.Empty(runtime.Properties);Assert.NotNull(runtime.Presentation);}
 [Fact]public void MissingGroupAndDuplicateNamesAreRejected(){var p=new TemplateProject{Name="T",Attributes=[new(){Key="x",Name="X",Input="true"}],Presentation=new(){Fields=new(){["x"]=new(){GroupId="missing"}}}};Assert.NotEmpty(new TemplateCompiler().Validate(p));}
}
