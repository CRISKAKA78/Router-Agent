using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
namespace ProbeTemplateGenerator.Tests;
public class NeighborTests
{
 [Fact]public void OverlappingLanAndBroadcastViewsRoundTrip(){
  var p=new TemplateProject{Name="邻居",NeighborProbe=new(){Domains=[new(){Id="lan",Scope="lan",Interface="br0",Ports=["LAN1","LAN2"]},new(){Id="local",Scope="broadcast",Interface="br0"}]}};
  var files=new ProjectFiles();var compiler=new TemplateCompiler();var runtime=compiler.Compile(p);p.NeighborProbe.Domains[0].Ports![0]="changed";Assert.Equal("LAN1",runtime.NeighborProbe!.Domains[0].Ports![0]);
  var restored=files.ReadProject(files.SerializeTemplate(runtime));Assert.Equal(2,restored.NeighborProbe!.Domains.Count);Assert.Equal("broadcast",restored.NeighborProbe.Domains[1].Scope);
  Assert.Equal("LAN2",files.ReadProject(files.SerializeProject(restored)).NeighborProbe!.Domains[0].Ports![1]);
 }
 [Theory][InlineData("uplink","br0")][InlineData("lan","br0")][InlineData("broadcast","../x")]
 public void InvalidDomainRejected(string scope,string iface){var p=new TemplateProject{Name="邻居",NeighborProbe=new(){Domains=[new(){Scope=scope,Interface=iface}]}};Assert.Contains(new TemplateCompiler().Validate(p),i=>i.Field=="NeighborProbe");}
}
