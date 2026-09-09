using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
namespace ProbeTemplateGenerator.Tests;
public class InterfaceTests
{
    [Fact] public void InterfacesAndEgressSurviveCompileAndRoundTrip()
    {
        var p=new TemplateProject{Name="接口",Monitoring=new(){NetworkInterfaces="eth0,br0",EgressSeconds=600},Attributes=[new(){Key="firmware",Name="固件",Input="nvram get softver"}]};
        var files=new ProjectFiles();var compiled=new TemplateCompiler().Compile(p);
        var restored=files.ReadProject(files.SerializeTemplate(compiled));
        Assert.Equal("eth0,br0",restored.Monitoring!.NetworkInterfaces);Assert.Equal(600,restored.Monitoring.EgressSeconds);
        Assert.Equal("eth0,br0",files.ReadProject(files.SerializeProject(p)).Monitoring!.NetworkInterfaces);
        Assert.Equal(8,restored.SchemaVersion);
        p.Monitoring.NetworkInterfaces="";Assert.Empty(new TemplateCompiler().Validate(p));
        p.Monitoring.NetworkInterfaces=null;Assert.DoesNotContain("network_interfaces",files.SerializeTemplate(new TemplateCompiler().Compile(p)));
    }
    [Theory][InlineData("eth0,eth0")][InlineData("eth*")][InlineData("../eth0")][InlineData("eth0,")][InlineData("abcdefghijklmnop")]
    public void InvalidNamesRejected(string names)
    {
        var p=new TemplateProject{Name="接口",Monitoring=new(){NetworkInterfaces=names},Attributes=[new(){Key="x",Name="X",Input="echo x"}]};
        Assert.Contains(new TemplateCompiler().Validate(p),i=>i.Field=="Monitoring");
    }
}
