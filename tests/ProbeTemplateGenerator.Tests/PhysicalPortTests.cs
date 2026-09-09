using ProbeTemplateGenerator.Models;
using ProbeTemplateGenerator.Services;
using ProbeTemplateGenerator.Features.Projects;

namespace ProbeTemplateGenerator.Tests;
public sealed class PhysicalPortTests
{
    [Fact] public void PresetPreservesConfirmedBoardMappingAndRoundtrips()
    {
        var s=PhysicalPortProfiles.Fnr100();Assert.Equal(new int?[]{1,2,3,4,5,0},s.Ports.Select(p=>p.Port));Assert.Equal("cpu",s.Ports[^1].Role);
        var p=new TemplateProject{Name="FNR100",SwitchProbe=s};var files=new ProjectFiles();var restored=files.ReadProject(files.SerializeProject(p));
        Assert.Equal(8,restored.SchemaVersion);Assert.Equal("RxGoodByte",restored.SwitchProbe!.Counters!.RxField);Assert.Equal("swconfig",restored.SwitchProbe.Backend);
        Assert.Equal("eth1",restored.SwitchProbe.Ports[0].Uplink);Assert.Equal("eth0",restored.SwitchProbe.Ports[4].Uplink);
    }
    [Fact] public void RawSamplePreviewIsExactAndRejectsPacketCountsDuplicatesAndOverflow()
    {
        var s=PhysicalPortProfiles.Fnr100();var v=PhysicalPortProfiles.Preview("Port 1 MIB counters\nRxGoodByte : 9007199254740993\nTxByte : 18446744073709551615\n",s).Single();
        Assert.Equal("9007199254740993",v.Receive);Assert.Equal("18446744073709551615",v.Transmit);
        foreach(var raw in new[]{"recv_good: 10\nTxByte: 30","RxGoodByte: 1\nRxGoodByte: 2\nTxByte: 3","RxGoodByte: -1\nTxByte: 3","RxGoodByte: 18446744073709551616\nTxByte: 3"})Assert.Throws<InvalidOperationException>(()=>PhysicalPortProfiles.Preview(raw,s));
    }
    [Fact] public void VendorSamplesMatchOnlyConfiguredStableIds()
    {
        var s=PhysicalPortProfiles.Fnr100();s.Counters=new(){Backend="command",RxField=null,TxField=null,Command="read counters"};
        Assert.Equal("0",PhysicalPortProfiles.Preview("lan1\t0\t42",s).Single().Receive);
        Assert.Throws<InvalidOperationException>(()=>PhysicalPortProfiles.Preview("unknown\t1\t2",s));
        Assert.Throws<InvalidOperationException>(()=>PhysicalPortProfiles.Preview("lan1\t1\t2\nlan1\t3\t4",s));
    }
}
