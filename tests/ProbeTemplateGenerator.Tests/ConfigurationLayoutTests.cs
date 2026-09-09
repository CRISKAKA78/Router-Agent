using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
using ProbeTemplateGenerator.Features.Projects;

namespace ProbeTemplateGenerator.Tests;

public sealed partial class EditorWorkspaceTests
{
    [Fact] public async Task LayoutFollowsRenameCopyDeleteAndGroupDeletion()
    {
        await using var f=await WorkspaceFixture.CreateAsync();var s=f.State;
        s.LoadExample(false);await s.AcceptConfirmationAsync();
        var row=s.Project.Attributes.First(a=>a.Visibility==AttributeVisibility.Display);s.Select(row.Id);
        s.AddGroup();s.SetGroup(row.Key,"group_1");s.SetOrder(row.Key,30);
        var old=row.Key;s.ChangeAttributeKey(row,"renamed_field");
        Assert.False(s.Project.Presentation!.Fields.ContainsKey(old));Assert.Equal(30,s.Placement(row.Key).Order);
        s.SetVisible(row.Key,false);s.CopyAttribute();var copy=s.Selected!;Assert.False(s.Placement(copy.Key).Visible);Assert.Equal("group_1",s.Placement(copy.Key).GroupId);Assert.True(s.Placement(copy.Key).Order>30);
        s.MoveField(copy.Key,-1);Assert.Equal(copy.Key,DisplayLayout.InGroup(s.Project,"group_1").First().Key);
        s.RemoveAttribute();await s.AcceptConfirmationAsync();Assert.False(s.Project.Presentation.Fields.ContainsKey(copy.Key));
        s.RemoveGroup(s.Project.Presentation.Groups[0]);Assert.Equal("other",s.Placement(row.Key).GroupId);
        s.ChangeVisibility(row,AttributeVisibility.Virtual);Assert.DoesNotContain(DisplayLayout.Fields(s.Project),x=>x.Key==row.Key);
    }
    [Fact] public async Task DuplicateKeyDoesNotOverwriteAnotherFieldsPlacement()
    {
        await using var f=await WorkspaceFixture.CreateAsync();var s=f.State;s.LoadExample(false);await s.AcceptConfirmationAsync();
        var rows=s.Project.Attributes.Take(2).ToArray();s.AddGroup();s.SetGroup(rows[0].Key,"group_1");s.SetOrder(rows[1].Key,50);
        s.ChangeAttributeKey(rows[0],rows[1].Key);Assert.Equal(50,s.Placement(rows[1].Key).Order);
        s.ChangeAttributeKey(rows[0],"renamed");Assert.Equal("group_1",s.Placement("renamed").GroupId);
    }
    [Fact] public async Task ConfigNavigationAndGroupMovesDoNotChangeSelectedAttribute()
    {
        await using var f=await WorkspaceFixture.CreateAsync();var s=f.State;s.LoadExample(false);await s.AcceptConfirmationAsync();var id=s.SelectedId;
        s.AddGroup();s.AddGroup();s.MoveGroup(s.Project.Presentation!.Groups[1],-1);
        Assert.Equal("group_2",DisplayLayout.CustomGroups(s.Project).First().Id);s.Configure(2);Assert.Equal(0,s.Stage);Assert.Equal(2,s.ConfigurationTab);
        s.SetStage(1);Assert.Equal(id,s.SelectedId);s.GoToIssue(new(null,"SwitchProbe","端口错误"));Assert.Equal(0,s.Stage);Assert.Equal(2,s.ConfigurationTab);
    }
}
public sealed class ConfigurationFormatTests
{
    [Fact] public void OptionalPortNumberIsOmittedAndZeroRemainsRealPort()
    {
        var p=new TemplateProject{Name="DSA",SwitchProbe=new(){Backend="dsa",Ports=[new(){Id="p1",SystemName="lan1"}]}};
        var c=new TemplateCompiler();var files=new ProjectFiles();var json=files.SerializeTemplate(c.Compile(p));Assert.DoesNotContain("\"port\":",json);
        foreach(var copy in new[]{files.ReadProject(files.SerializeProject(p)),files.ReadProject(json),files.FromTemplate(c.Compile(p))})Assert.Null(copy.SwitchProbe!.Ports[0].Port);
        p.SwitchProbe.Ports[0].Port=0;p.SwitchProbe.Ports[0].SwitchId="switch0";Assert.Contains("\"port\": 0",files.SerializeTemplate(c.Compile(p)));
    }
    [Fact] public void CurrentProjectKeepsPortMappingAndDisplayMetadata()
    {
        var p=new TemplateProject{Name="当前工程",SchemaVersion=8,SwitchProbe=new(){Backend="swconfig",Ports=[new(){Id="p1",SwitchId="switch0",Port=3,DisplayName="LAN1"}]},Presentation=new(){Groups=[new(){Id="basic",Name="基本信息"}],Fields=new(){["cpu_model"]=new(){GroupId="basic",Order=20}}}};
        var files=new ProjectFiles();var copy=files.ReadProject(files.SerializeProject(p));Assert.Equal(8,copy.SchemaVersion);Assert.Equal(3,copy.SwitchProbe!.Ports[0].Port);Assert.Equal("basic",copy.Presentation!.Fields["cpu_model"].GroupId);Assert.Empty(new TemplateCompiler().Validate(copy));
    }
    [Fact] public void BackendRequirementsAndDynamicFieldsAreValidated()
    {
        var p=new TemplateProject{Name="T",SwitchProbe=new(){Backend="swconfig",Ports=[new(){Id="p1"}]}};var c=new TemplateCompiler();Assert.Contains(c.Validate(p),i=>i.Message.Contains("swconfig映射"));
        p.SwitchProbe.Backend="dsa";p.SwitchProbe.Ports[0].SystemName="lan1";Assert.Empty(c.Validate(p));
        p.SwitchProbe.Ports[0].SwitchId="switch0";Assert.Contains(c.Validate(p),i=>i.Message.Contains("须填写芯片端口号"));
        p.SwitchProbe.Ports[0].SwitchId="";p.SwitchProbe.Backend="command";p.SwitchProbe.Command="true";Assert.Empty(c.Validate(p));
        p.Presentation=new(){Fields=new(){["network_eth0_rx_bytes"]=new()}};
        Assert.Contains(DisplayLayout.Fields(p),f=>f.Key=="cpu_model"&&f.Name=="CPU型号");Assert.Contains(DisplayLayout.Fields(p),f=>f.Key=="network_eth0_rx_bytes");
    }
}
