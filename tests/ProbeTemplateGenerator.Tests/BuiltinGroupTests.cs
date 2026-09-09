using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
using ProbeTemplateGenerator.Features.Projects;
namespace ProbeTemplateGenerator.Tests;
public sealed class BuiltinGroupTests
{
    [Fact] public void DefaultGroupsAndExplicitAssignmentsSurviveExport()
    {
        var project=new TemplateProject{Name="分组",Presentation=new(){Groups=[new(){Id="custom",Name="业务信息",Order=10}],Fields=new(){["cpu_usage"]=new(){GroupId="builtin_system"},["memory_usage"]=new(){GroupId="custom"},["disk_example_usage"]=new(){GroupId="other"},["session_id"]=new(){Visible=true}}}};
        Assert.Equal(new[]{"builtin_system","builtin_resources","custom","other"},DisplayLayout.Groups(project).Select(g=>g.Id));
        var files=new ProjectFiles();var runtime=new TemplateCompiler().Compile(project);
        foreach(var copy in new[]{files.ReadProject(files.SerializeProject(project)),files.ReadProject(files.SerializeTemplate(runtime))}){
            Assert.Equal("builtin_system",DisplayLayout.GroupId(copy,"cpu_usage"));
            Assert.Equal("custom",DisplayLayout.GroupId(copy,"memory_usage"));
            Assert.Equal("other",DisplayLayout.GroupId(copy,"disk_example_usage"));
            Assert.Equal("builtin_resources",DisplayLayout.GroupId(copy,"memory_total_bytes"));
            Assert.Equal("other",DisplayLayout.GroupId(copy,"net_example_state"));
            Assert.True(DisplayLayout.IsVisible(copy,"session_id"));
        }
        Assert.False(DisplayLayout.IsVisible(new TemplateProject(),"session_id"));
        Assert.Equal(3,DisplayLayout.Groups(new TemplateProject()).Count());
        project.Presentation.Groups[0].Id="builtin_system";Assert.NotEmpty(new TemplateCompiler().Validate(project));
    }
}
