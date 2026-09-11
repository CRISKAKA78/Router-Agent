using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
using RouterAgent.Neighbors;
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
 public void InvalidDomainRejected(string scope,string iface){var p=new TemplateProject{Name="邻居",NeighborProbe=new(){Domains=[new(){Scope=scope,Interface=iface}]}};Assert.Contains(new TemplateCompiler().Validate(p),i=>i.Field.StartsWith("neighbor",StringComparison.OrdinalIgnoreCase));}

 [Theory][InlineData("192.168.5.222/24","192.168.5.0/24",254)][InlineData("192.168.5.130/25","192.168.5.128/25",126)][InlineData("192.168.5.223/32","192.168.5.223/32",1)][InlineData("192.168.5.222/31","192.168.5.222/31",2)]
 public void NormalizeAndCount(string input,string expected,int count){var n=Network();Assert.Null(NeighborNetworks.Validate(input,n,out var range));Assert.Equal(expected,range!.Cidr);Assert.Equal(count,range.Hosts);Assert.Equal(expected,NeighborNetworks.Normalize(input));}
 [Theory][InlineData("192.168.6.0/24")][InlineData("192.168.4.0/23")][InlineData("2001:db8::/64")][InlineData("bad")]
 public void RejectInvalidOrOffLink(string input){Assert.NotNull(NeighborNetworks.Validate(input,Network(),out _));}
 [Fact] public void AutoRangeAndStableIds(){var n=Network();Assert.Equal("192.168.5.0/24",NeighborNetworks.Ranges(n)[0].Cidr);Assert.Equal(18,NeighborNetworks.Ranges(n)[0].Seconds);Assert.Empty(NeighborNetworks.Ranges(n with{Ipv4=[]}));Assert.Empty(NeighborNetworks.Ranges(n with{Eligible=false,Master="br0"}));Assert.Equal("local",NeighborNetworks.DomainId("broadcast","br0",false));Assert.Equal("lan",NeighborNetworks.DomainId("lan","br0",false));Assert.NotEqual(NeighborNetworks.DomainId("lan","a_b",true),NeighborNetworks.DomainId("lan","a.b",true));Assert.True(NeighborNetworks.DomainId("broadcast","abcdefghijklmno",true).Length<=32);Assert.Equal(2,NeighborNetworks.Ranges(n with{Ipv4=["192.168.5.222/24","198.51.100.2/25"]}).Length);}
 [Fact] public void Fnr100PresetRoundTripsWithoutExecutingCommands(){var p=new TemplateProject{Name="预设",NeighborProbe=new(){FdbPreset="fnr100",Domains=[new(){Id="local",Interface="br0"}]}};var files=new ProjectFiles();var runtime=new TemplateCompiler().Compile(p);Assert.Equal("fnr100",files.ReadProject(files.SerializeTemplate(runtime)).NeighborProbe!.FdbPreset);p.NeighborProbe.FdbCommand="anything";Assert.NotEmpty(new TemplateCompiler().Validate(p));}
 private static DetectedNetwork Network()=>new("br0",true,false,"",true,"",["192.168.5.222/24"],["192.168.5.0/24"],["eth0","vlan3"]);
}
