using ProbeTemplateGenerator.Models;
using System.Text;
using System.Text.RegularExpressions;
namespace ProbeTemplateGenerator.Services;
public sealed partial class TemplateCompiler
{
    private static void ValidatePresentation(TemplateProject project,List<ValidationIssue> issues)
    {
        void Error(string message)=>issues.Add(new(null,"Presentation",message));
        bool Key(string k)=>Regex.IsMatch(k,"^[a-z][a-z0-9_]{0,63}$");
        if(project.Presentation is {} p){
            if(p.Groups is null||p.Fields is null||p.BuiltinVisibility is null){Error("展示配置集合不能为空");return;}
            if(p.Groups.Count>64||p.Fields.Count>1024||p.BuiltinVisibility.Count>7)Error("展示配置超过分组/字段/分类数量上限");
            if(p.Groups.Select(g=>g.Id).Distinct().Count()!=p.Groups.Count||p.Groups.Select(g=>g.Name).Distinct().Count()!=p.Groups.Count)Error("分组标识和名称不能重复");
            foreach(var g in p.Groups)if(!Key(g.Id)||DisplayLayout.IsBuiltinGroup(g.Id)||DisplayLayout.BuiltinGroups.Values.Contains(g.Name)||string.IsNullOrWhiteSpace(g.Name)||Encoding.UTF8.GetByteCount(g.Name)>128||g.Order is <0 or >1000000)Error("自定义分组需要独立名称、有效标识和0～1000000的排序数字");
            foreach(var (key,f) in p.Fields)if(!Key(key)||f.Order is <0 or >1000000||f.GroupId!=""&&!DisplayLayout.IsBuiltinGroup(f.GroupId)&&!p.Groups.Any(g=>g.Id==f.GroupId))Error($"属性 {key} 的分组或排序无效");
            foreach(var key in p.BuiltinVisibility.Keys)if(!DisplayLayout.Categories.ContainsKey(key))Error("未知的内置显示分类");
        }
        if(project.SwitchProbe is {} s){
            void PortError(string message)=>issues.Add(new(null,"SwitchProbe",message));
            if(s.Backend is not ("auto" or "dsa" or "swconfig" or "command"))PortError("交换机采集方式无效");
            if(s.Backend=="command"&&(string.IsNullOrWhiteSpace(s.Command)||Encoding.UTF8.GetByteCount(s.Command)>4096))PortError("厂商采集命令需要1～4096字节");
            if(s.Backend!="command"&&!string.IsNullOrEmpty(s.Command))PortError("仅厂商命令模式支持采集命令");
            if(s.Ports is null){PortError("端口集合不能为空");return;}
            if(s.Counters is {} c){
                if(s.Ports.Count is <1 or >16)PortError("启用流量时须配置1～16个真实端口");
                if(c.Bits is not (32 or 64)||string.IsNullOrWhiteSpace(c.Basis)||Encoding.UTF8.GetByteCount(c.Basis)>128)PortError("计数位宽须为32/64，统计口径须为1～128字节");
                if(c.Backend=="swconfig_mib"){
                    if(!string.IsNullOrEmpty(c.Command)||string.IsNullOrWhiteSpace(c.RxField)||string.IsNullOrWhiteSpace(c.TxField)||Encoding.UTF8.GetByteCount(c.RxField??"")>64||Encoding.UTF8.GetByteCount(c.TxField??"")>64||c.RxField==c.TxField)PortError("MIB需要不同的RX/TX字段名，各1～64字节，不配置额外命令");
                    foreach(var port in s.Ports)if(!Regex.IsMatch(port.SwitchId,"^[A-Za-z0-9_.-]{1,32}$")||port.SwitchId is "." or ".."||port.Port is null)PortError("MIB流量来源需要真实交换机实例与端口号");
                }else if(c.Backend=="command"){
                    if(string.IsNullOrWhiteSpace(c.Command)||Encoding.UTF8.GetByteCount(c.Command)>4096||!string.IsNullOrEmpty(c.RxField)||!string.IsNullOrEmpty(c.TxField))PortError("厂商流量命令需要1～4096字节，输出三列原始计数，不配置MIB字段名");
                }else PortError("不支持的物理口流量来源");
            }
            var sources=new HashSet<string>();foreach(var port in s.Ports){var source=port.SwitchId.Length>0&&port.Port is not null?port.SwitchId+":"+port.Port:"system:"+port.SystemName;if(source!="system:"&&!sources.Add(source))PortError("同一物理端口不能映射多次");}
            if(s.Ports.Count>64||s.Ports.Select(p=>p.Id).Distinct().Count()!=s.Ports.Count)PortError("端口最多64项，标识不能重复");
            foreach(var port in s.Ports){
                if(port.SwitchId.Length>0 && port.Port is null)PortError($"端口 {port.Id}：填写交换机实例后，须填写芯片端口号。");
                if(s.Backend=="swconfig" && (port.SwitchId.Length==0 || port.Port is null))PortError($"端口 {port.Id}：swconfig映射需要交换机实例和芯片端口号。");
                if(s.Backend is "auto" or "dsa" && port.SwitchId.Length==0 && port.SystemName.Length==0)PortError($"端口 {port.Id}：请填写系统物理接口，或交换机实例与芯片端口号。");
            }
            foreach(var port in s.Ports)if(port.Role is not ("" or "external" or "cpu")||Encoding.UTF8.GetByteCount(port.SwitchId)>64||!Key(port.Id)||port.Id.Length>32||port.Port is <0 or >255||port.SystemName.Length>15||port.Uplink.Length>15||Encoding.UTF8.GetByteCount(port.DisplayName)>128)PortError("端口标识、端口号或接口名称无效");
        }
    }
}
