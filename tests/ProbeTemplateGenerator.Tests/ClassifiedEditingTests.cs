using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
using ProbeTemplateGenerator.Features.Projects;

namespace ProbeTemplateGenerator.Tests;
public sealed class ClassifiedEditingTests
{
    [Fact] public void StorageVisibilitySurvivesBothFormatsAndDoesNotDisableCollection()
    {
        var project=new TemplateProject{Name="磁盘显示",Monitoring=new(){DiskSeconds=60},Presentation=new(){StorageVisible=false,BuiltinVisibility=new(){["disk"]=true}}};
        var files=new ProjectFiles();var runtime=new TemplateCompiler().Compile(project);
        foreach(var copy in new[]{files.ReadProject(files.SerializeProject(project)),files.ReadProject(files.SerializeTemplate(runtime))}){
            Assert.False(copy.Presentation!.StorageVisible);Assert.True(DisplayLayout.IsVisible(copy,"disk_a_usage"));Assert.Equal(60,copy.Monitoring!.DiskSeconds);
        }
        Assert.Null(new PresentationSettings().StorageVisible);
    }
    [Fact] public void EditingCategoriesAreIndependentOfDisplayGrouping()
    {
        var project=new TemplateProject{Presentation=new(){Fields=new(){["cpu_usage"]=new(){GroupId="other"},["net_eth0_state"]=new(){GroupId="builtin_interfaces"}}}};
        Assert.Equal("cpu",RouterWorkbench.Display.DevicePresentation.EditCategory("cpu_usage"));Assert.Equal("other",DisplayLayout.GroupId(project,"cpu_usage"));
        Assert.Equal("other",DisplayLayout.GroupId(project,"net_eth0_state"));
        foreach(var field in DisplayLayout.Fields(project))Assert.True(RouterWorkbench.Display.DevicePresentation.EditCategories.ContainsKey(RouterWorkbench.Display.DevicePresentation.EditCategory(field.Key)));
    }
}
