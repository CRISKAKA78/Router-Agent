using System.Net;
using System.Net.Sockets;
using System.Text.Json.Serialization;
namespace RouterAgent.Neighbors;

public sealed record DetectedNetwork(
 [property:JsonPropertyName("interface")] string Interface,
 [property:JsonPropertyName("bridge")] bool Bridge,
 [property:JsonPropertyName("vlan")] bool Vlan,
 [property:JsonPropertyName("master")] string Master,
 [property:JsonPropertyName("eligible")] bool Eligible,
 [property:JsonPropertyName("reason")] string Reason,
 [property:JsonPropertyName("ipv4")] string[] Ipv4,
 [property:JsonPropertyName("networks")] string[] Networks,
 [property:JsonPropertyName("ports")] string[] Ports)
{
 [JsonIgnore] public string Label => Interface+" · "+(Networks.Length==0?"无IPv4":string.Join(" / ",Networks));
 [JsonIgnore] public string PortHint => Ports.Contains("vlan3")?"交换机聚合路径，无法区分机壳端口":"内核FDB端口："+string.Join("、",Ports);
 public override string ToString()=>Label;
}
public sealed record NetworkDiscovery(
 [property:JsonPropertyName("networks")] DetectedNetwork[] Networks,
 [property:JsonPropertyName("preset")] string Preset,
 [property:JsonPropertyName("preset_status")] string PresetStatus,
 [property:JsonPropertyName("raw_summary")] string RawSummary,
 [property:JsonPropertyName("ports")] string[] Ports,
 [property:JsonPropertyName("config_revision")] ulong ConfigRevision,
 [property:JsonPropertyName("session_id")] string SessionId,
 [property:JsonPropertyName("detected_at")] DateTimeOffset DetectedAt,
 [property:JsonPropertyName("stale")] bool Stale)
{
 public bool Expired(DateTimeOffset now)=>Stale || now-DetectedAt>TimeSpan.FromSeconds(90);
}
public sealed record ScanRange(string Interface,string Cidr,string ConnectedNetwork,int Hosts,int Seconds)
{
 public string Label=>$"{Interface} · {Cidr} · {Hosts}个主机地址";
 public override string ToString()=>Label;
}
public static class NeighborNetworks
{
 public static bool TryPrefix(string text,out uint network,out int bits){network=0;bits=0;var split=text.Trim().Split('/');if(split.Length!=2||!int.TryParse(split[1],out bits)||bits is <0 or >32||!IPAddress.TryParse(split[0],out var ip)||ip.AddressFamily!=AddressFamily.InterNetwork)return false;var b=ip.GetAddressBytes();network=((uint)b[0]<<24)|((uint)b[1]<<16)|((uint)b[2]<<8)|b[3];network&=Mask(bits);return true;}
 private static uint Mask(int bits)=>bits==0?0:uint.MaxValue<<(32-bits);
 private static string Format(uint ip,int bits)=>$"{ip>>24}.{(ip>>16)&255}.{(ip>>8)&255}.{ip&255}/{bits}";
 public static string Normalize(string input)=>TryPrefix(input,out var ip,out var bits)?Format(ip,bits):input.Trim();
 public static int Hosts(int bits)=>bits<24||bits>32?0:bits>=31?1<<(32-bits):(1<<(32-bits))-2;
 public static ScanRange[] Ranges(DetectedNetwork n)=>!n.Eligible?[]:n.Ipv4.Select(a=>{if(!TryPrefix(a,out var network,out var bits))return null;var chosen=bits<24?Normalize(a.Split('/')[0]+"/24"):Format(network,bits);var b=Math.Max(bits,24);return new ScanRange(n.Interface,chosen,Format(network,bits),Hosts(b),(int)Math.Ceiling(Hosts(b)/16d)+2);}).OfType<ScanRange>().Distinct().ToArray();
 public static string? Validate(string input,DetectedNetwork n,out ScanRange? range){range=null;if(!n.Eligible)return "该接口是桥成员或不支持以太网发现，请选择本机IP所在网络。";if(!TryPrefix(input,out var ip,out var bits)||bits<24)return "仅允许IPv4 /24～/32范围，每次最多256个地址。";var parent=n.Networks.FirstOrDefault(s=>TryPrefix(s,out var network,out var prefix)&&bits>=prefix&&(ip&Mask(prefix))==network);if(parent is null)return $"该范围不属于 {n.Interface} 当前直连网络 {string.Join("、",n.Networks)}";range=new(n.Interface,Format(ip,bits),parent,Hosts(bits),(int)Math.Ceiling(Hosts(bits)/16d)+2);return null;}
 // Hex encoding is reversible, collision-free for the allowed 15-byte interface names.
 public static string DomainId(string scope,string iface,bool multiple)=>!multiple?(scope=="lan"?"lan":"local"):(scope=="lan"?"l_":"b_")+Convert.ToHexString(System.Text.Encoding.ASCII.GetBytes(iface)).ToLowerInvariant();
}
