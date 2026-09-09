using ProbeTemplateGenerator.Features.Projects;
using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
using Xunit;
namespace ProbeTemplateGenerator.Tests;
public sealed class VisibilityTests
{
    [Fact] public void CategoryDefaultsAndFieldOverridesDoNotChangeCollection()
    {
        var project=new TemplateProject{Name="显示测试",Monitoring=new(){DiskSeconds=60},Presentation=new(){BuiltinVisibility=new(){["memory"]=false},Fields=new(){["memory_usage"]=new(){Visible=true},["device_id"]=new(){Visible=false},["net_65746830_rx_bytes"]=new(){Visible=true}}}};
        foreach(var key in new[]{"switch_lan1_state","net_aa_state","memory_total_bytes","device_id"})Assert.False(DisplayLayout.IsVisible(project,key));
        foreach(var key in new[]{"disk_aa_total_bytes","cpu_usage","memory_usage","net_65746830_rx_bytes"})Assert.True(DisplayLayout.IsVisible(project,key));
        var files=new ProjectFiles();var runtime=new TemplateCompiler().Compile(project);
        foreach(var copy in new[]{files.ReadProject(files.SerializeProject(project)),files.ReadProject(files.SerializeTemplate(runtime))}) {
            Assert.False(DisplayLayout.IsVisible(copy,"device_id"));Assert.True(DisplayLayout.IsVisible(copy,"memory_usage"));Assert.False(DisplayLayout.IsVisible(copy,"memory_total_bytes"));
            Assert.Equal(60,copy.Monitoring!.DiskSeconds);
        }
    }
    [Fact] public void RemovedMappingIsRejectedInProjectAndRuntime()
    {
        var files=new ProjectFiles();
        foreach(var json in new[]{"{\"schema_version\":8,\"name\":\"T\",\"attributes\":[],\"presentation\":{\"interface_aliases\":{\"eth0\":\"LAN\"}}}","{\"name\":\"T\",\"properties\":{},\"presentation\":{\"interface_aliases\":{}}}"})
            Assert.ThrowsAny<Exception>(()=>files.ReadProject(json));
    }
}
