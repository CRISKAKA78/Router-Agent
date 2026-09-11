namespace RouterWorkbench.Client;
public sealed record ForwardInterface(string Name,string Address){public override string ToString()=>Name+" · "+Address;}
public sealed record ForwardCapabilities(ForwardInterface[] Interfaces,string[] Serials,bool Backend);
public sealed record ForwardMapping(string Id,string DeviceId,string Kind,string Protocol,string Interface,string TargetIp,int TargetPort,
 string Serial,int Baud,int DataBits,int StopBits,string Parity,int LeaseMinutes,string SessionId,string State,string Reason,
 string Host,int Port,string SourceIp,DateTimeOffset CreatedAt,DateTimeOffset? ExpiresAt,DateTimeOffset? ClosedAt,
 DateTimeOffset? ReusableAfter,bool Released,bool DeviceReleased,ForwardAuth? SerialAuth)
{
 public string ReasonText=>Reason switch{"closed_by_user"=>"用户关闭","expired"=>"租期到期","device_session_ended"=>"设备会话结束","device_helper_exited"=>"设备辅助程序退出","relay_exited"=>"Server GOST退出","gost_exited"=>"设备GOST退出","create_timeout"=>"等待数据通道就绪超时","target_not_on_selected_interface"=>"目标不属于所选接口直连网段","serial_missing_or_console"=>"串口不存在或属于系统控制台","serial_busy"=>"串口已被占用","gost_not_installed"=>"设备缺少GOST","helper_unavailable"=>"设备辅助程序不可用","device_dispatch_failed"=>"设备命令派发失败",_=>Reason};
 public bool CanConnect=>State=="active"&&!Released;
 public string Endpoint=>CanConnect?$"{Host}:{Port}":"不可用";
 public string Target=>Kind=="serial"?$"{Serial} · {Baud} / {DataBits} / {Parity} / {StopBits}":$"{Interface} → {TargetIp}:{TargetPort}";
 public string StateText=>State switch{"creating"=>"创建中","active"=>"运行中","closed"=>"已关闭","failed"=>"失败",_=>State};
 public string LeaseText=>ExpiresAt?.ToLocalTime().ToString("yyyy-MM-dd HH:mm:ss")??"不限时（失联仍关闭）";
 public string Traffic=>SerialAuth is {} a?$"发送 {a.BytesToBackend} / 接收 {a.BytesToClient} B":"未提供";
 public string Connections=>SerialAuth is {} a?(a.Owned?"1 已认证":"0 已认证")+ $" / {a.Pending} 待认证":"未提供";
}
public sealed record ForwardAuth(bool Closed,int Pending,bool Owned,ulong Accepted,ulong AuthFailures,ulong Busy,ulong Rejected,long BytesToBackend,long BytesToClient);
public sealed record ForwardCreated(ForwardMapping Mapping,string? Registration);

public sealed record ForwardCandidate(string Ip,string Label);
public static class ForwardingChoices
{
 public static ForwardCandidate[] Candidates(NeighborSnapshot? neighbors,string? iface)=>(neighbors?.Domains??[]).Where(d=>d.Interface==iface).SelectMany(d=>d.Rows.Select(r=>(Row:r,Scope:d.Scope))).Where(x=>System.Net.IPAddress.TryParse(x.Row.Ip,out var ip)&&ip.AddressFamily==System.Net.Sockets.AddressFamily.InterNetwork).GroupBy(x=>x.Row.Ip).OrderBy(g=>g.Key).Select(g=>new ForwardCandidate(g.Key,g.Key+" · "+g.First().Row.Mac+" · "+string.Join("/",g.Select(x=>x.Scope=="lan"?"LAN下接":"本机广播域").Distinct()))).ToArray();
}
