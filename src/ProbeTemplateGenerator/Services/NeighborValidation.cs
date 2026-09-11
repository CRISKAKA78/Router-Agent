using System.Text;
using System.Text.RegularExpressions;
using ProbeTemplateGenerator.Models;
namespace ProbeTemplateGenerator.Services;
public sealed partial class TemplateCompiler
{
 private static void ValidateNeighbors(TemplateProject project,List<ValidationIssue> issues)
 {
  if(project.NeighborProbe is not {} n)return;
  void Invalid(string text,string field="NeighborProbe")=>issues.Add(new(null,field,text));
  if(n.FdbPreset is not (null or "fnr100") || (n.FdbPreset is not null&&!string.IsNullOrEmpty(n.FdbCommand)))Invalid("型号预设与自定义命令不能同时使用。","neighbor_probe.fdb_preset");
  if(n.IntervalSeconds is <10 or >86400)Invalid("邻居采集周期须为10～86400秒。","neighbor_probe.interval_seconds");
  if(n.Domains is null||n.Domains.Count is <1 or >8){Invalid("请配置1～8个广播域。");return;}
  if(n.FdbCommand is {} command&&(Encoding.UTF8.GetByteCount(command)>4096||command.Contains('\0')))Invalid("物理端口识别命令最多4096字节，不可包含NUL。","neighbor_probe.fdb_command");
  var ids=new HashSet<string>(StringComparer.Ordinal);
  foreach(var d in n.Domains){
   if(d is null){Invalid("广播域记录不能为空。");continue;}
   var path=$"neighbor_probe.domains[{n.Domains.IndexOf(d)}].";
   if(d.Id is null||!Regex.IsMatch(d.Id,"^[a-z][a-z0-9_]{0,31}$")||!ids.Add(d.Id))Invalid("广播域标识须为小写字母开头的1～32字符，不可重复。",path+"id");
   if(d.Scope is not ("lan" or "broadcast"))Invalid("请选择LAN下接或本机广播域。",path+"scope");
   if(d.Interface is null||d.Interface is "." or ".."||!Regex.IsMatch(d.Interface,"^[A-Za-z0-9_.-]{1,15}$"))Invalid("请输入本机IP所在的原始Linux接口名，不根据名称自动判断用途。",path+"interface");
   if(d.LeaseFile is {Length:>0} file&&(!file.StartsWith('/')||Encoding.UTF8.GetByteCount(file)>256||file.IndexOfAny(['\0','\r','\n'])>=0))Invalid("租约文件须为不超过256字节的绝对路径。",path+"lease_file");
   if(d.Ports is {} ports&&(ports.Count>64||ports.Distinct(StringComparer.Ordinal).Count()!=ports.Count||ports.Any(p=>string.IsNullOrEmpty(p)||Encoding.UTF8.GetByteCount(p)>128||p.IndexOfAny(['\0','\r','\n'])>=0)))Invalid("转发端口最多64项，不可重复，每项1～128字节。",path+"ports");
   if(d.Scope=="lan"&&d.Ports is not {Count:>0})Invalid("LAN下接清单须指定本机LAN转发端口；未匹配的记录另列，不猜测端口角色。",path+"ports");
   foreach(var other in n.Domains.TakeWhile(x=>!ReferenceEquals(x,d)).Where(x=>x is not null&&x.Interface==d.Interface&&x.Scope==d.Scope))
    if(d.Scope=="broadcast"||d.Ports is not {Count:>0}||other.Ports is not {Count:>0}||d.Ports.Intersect(other.Ports,StringComparer.Ordinal).Any())Invalid("同一接口只配置一个本机广播域清单；多个LAN清单的转发端口须互不重叠。");
  }
 }
}
