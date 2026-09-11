namespace RouterWorkbench.Client;

public sealed record NeighborDomainConfig(string Id,string Scope,string Interface,string[]? Ports=null)
{
 public string Label=>Id+" · "+Interface;
 public override string ToString()=>Label;
}
public sealed record NeighborRow(string Ip,string Mac,string Port,string Hostname,string Source,string State,string? Interface=null, string? DomainId=null, string? Scope=null, DateTimeOffset? FirstSeen=null, DateTimeOffset? LastSeen=null, bool Current=true, DateTimeOffset? ActiveAt=null)
{
 public string IpText=>string.IsNullOrEmpty(Ip)?"未知":Ip;
 public string PortText=>string.IsNullOrEmpty(Port)?"未知":Port;
 public string NameText=>string.IsNullOrEmpty(Hostname)?"—":Hostname;
 public string FirstSeenText=>Labels.Time(FirstSeen);
 public string LastSeenText=>Labels.Time(LastSeen);
 public string StateText=>State=="recent" || (State=="responded" && ActiveAt is {} at && DateTimeOffset.UtcNow-at>=TimeSpan.FromSeconds(60)) ? $"最近发现于{Math.Max(0,(int)(DateTimeOffset.UtcNow-(LastSeen??ActiveAt??DateTimeOffset.UtcNow)).TotalMinutes)}分钟前" : State switch {"cached"=>"缓存记录","reachable"=>"内核最近可达","lease"=>"仅租约","mac_only"=>"仅MAC记录","responded"=>"刚刚响应（60秒内）",_=>"未知"};
 public string SourceText=>string.Join(" / ",Source.Split('+').Select(s=>s switch {"arp"=>"ARP","ndp"=>"NDP","dhcp"=>"DHCP","fdb"=>"MAC转发表","active_arp"=>"主动ARP","active_arp_history"=>"历史主动ARP",_=>s}));
}
public sealed record NeighborDomain(string Id,string Scope,string Interface,string Status,string Reason,bool Limited,NeighborRow[] Rows);
public sealed record NeighborSnapshot(ulong ConfigRevision,uint IntervalSeconds,NeighborDomain[] Domains,NeighborRow[] Unclassified,bool Limited,DateTimeOffset SampledAt,bool Stale);
public static class NeighborDiscovery
{
 public static string Reason(string? value)=>string.Join("；",(value??"").Split(';',StringSplitOptions.RemoveEmptyEntries).Select(s=>s switch {
  "interface_missing"=>"接口不存在","bridge_member_use_master"=>"请配置网桥接口，而非从属接口","vlan_bridge_requires_l3_interface"=>"VLAN网桥须指定对应三层子接口","not_ethernet"=>"接口不支持以太网邻居发现",
  "arp_unavailable"=>"ARP表不可读","ndp_unavailable"=>"NDP表不可读","leases_unavailable"=>"租约文件不可读","fdb_unavailable"=>"MAC转发表不可读","fdb_command_failed"=>"厂商MAC表命令失败",
  "range_not_on_link"=>"扫描范围不属于接口直连IPv4子网","raw_socket_unavailable"=>"无法创建ARP套接字，请检查探针权限","interface_ipv4_unavailable"=>"接口没有可用IPv4地址","scan_busy"=>"已有扫描正在运行","configuration_changed"=>"设备配置已变化","cancelled"=>"扫描已取消",_=>s}));
}

public sealed record NeighborScanSummary(int Responses,int Added,int Updated);
