using System.Globalization;
using System.Text;
using ProbeTemplateGenerator.Models;

namespace ProbeTemplateGenerator.Services;

public sealed record CounterPreview(string Port,string Receive,string Transmit);
public static class PhysicalPortProfiles
{
    public static SwitchSettings Fnr100()
    {
        var settings=new SwitchSettings {Backend="swconfig",Counters=new()};
        for(var i=1;i<=5;i++)settings.Ports.Add(new(){Id=i==5?"wan":"lan"+i,SwitchId="switch0",Port=i,DisplayName=i==5?"WAN":"LAN"+i,Role="external",Uplink=i==5?"eth0":"eth1"});
        settings.Ports.Add(new(){Id="cpu",SwitchId="switch0",Port=0,DisplayName="CPU内部口",Role="cpu"});
        return settings;
    }

    public static CounterPreview[] Preview(string sample,SwitchSettings settings)
    {
        if(Encoding.UTF8.GetByteCount(sample)>32768)throw new InvalidOperationException("样本不能超过32 KiB");
        var c=settings.Counters??throw new InvalidOperationException("请先启用物理口流量采集");
        string Number(string value){if(value.Length==0||value.Any(ch=>ch<'0'||ch>'9')||!ulong.TryParse(value,NumberStyles.None,CultureInfo.InvariantCulture,out var n)||c.Bits==32&&n>uint.MaxValue)throw new InvalidOperationException("计数必须是位宽范围内的十进制非负整数");return n.ToString(CultureInfo.InvariantCulture);}
        var lines=sample.Split('\n').Select(s=>s.TrimEnd('\r')).Where(s=>s.Length>0).ToArray();
        if(c.Backend=="command"){
            var rows=new List<CounterPreview>();var ids=new HashSet<string>();
            foreach(var line in lines){var fields=line.Split('\t');if(fields.Length!=3||!settings.Ports.Any(p=>p.Id==fields[0])||!ids.Add(fields[0])||rows.Count>=64)throw new InvalidOperationException("每行须为已配置端口ID、RX字节、TX字节三列Tab分隔，ID不能重复");rows.Add(new(fields[0],Number(fields[1]),Number(fields[2].Trim())));}
            if(rows.Count==0)throw new InvalidOperationException("没有可解析的计数");return rows.ToArray();
        }
        if(c.Backend!="swconfig_mib")throw new InvalidOperationException("不支持的计数来源");
        string? rx=null,tx=null;
        foreach(var line in lines){var pos=line.IndexOf(':');if(pos<0)continue;var key=line[..pos].Trim();var value=line[(pos+1)..].Trim();
            if(key==c.RxField){if(rx!=null)throw new InvalidOperationException("RX字段重复");rx=Number(value);}
            if(key==c.TxField){if(tx!=null)throw new InvalidOperationException("TX字段重复");tx=Number(value);}
        }
        if(rx==null||tx==null)throw new InvalidOperationException("样本缺少配置的RX/TX字节字段；包计数不能代替字节");
        return [new("单口MIB样本",rx,tx)];
    }
}
