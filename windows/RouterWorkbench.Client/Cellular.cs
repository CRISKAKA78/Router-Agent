namespace RouterWorkbench.Client;
public sealed record CellularSettings(uint IntervalSeconds=30,[property:System.Text.Json.Serialization.JsonIgnore(Condition=System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingDefault)] bool Telemetry=false,[property:System.Text.Json.Serialization.JsonIgnore(Condition=System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingDefault)] bool Details=false);
public sealed record AtIdentity(string Command,string Value,string Status);
public sealed record CellularPort(string Path,string DeviceKey,string Status,string Reason,bool Selected,
 ulong AgeMs,AtIdentity ATI,AtIdentity IMEI,DateTimeOffset? SampledAt, string Profile="", AtIdentity[]? Queries=null, CellularField[]? Fields=null, CellularSignal[]? Signals=null)
{
 public override string ToString()=>Path+(Selected?" · 已选择":"")+" · "+CellularLabels.Status(Status);
}
public sealed record CellularSnapshot(ulong ConfigRevision,uint IntervalSeconds,string Status,string Reason,
 bool Limited,CellularPort[] Ports,DateTimeOffset? SampledAt,bool Stale,bool Telemetry=false,bool Details=false);
public sealed record CellularField(string Key,string Value,string Name="",string Group="",string Source="");
public sealed record CellularSignal(string Key,string Rat,double Value,double Minimum,double Maximum,string Unit,string Qualifier);
public static class CellularLabels
{
 public static string Status(string? status)=>status switch {
  "ok"=>"正常","partial"=>"部分成功","unavailable"=>"未找到可用AT端口","no_ports"=>"未发现USB串口",
  "busy"=>"已占用 · 跳过","not_at"=>"AT未确认","pending"=>"等待后续探测","alternate"=>"同模块已有可用端口",
  "not_queried"=>"未查询","rejected"=>"模块拒绝查询","invalid_value"=>"返回值不符合身份格式",
  "invalid_response"=>"返回格式无效","timeout"=>"查询超时","io_error"=>"串口读取失败",
  "overflow"=>"响应超过上限","cancelled"=>"已取消","error"=>"检测失败",_=>status??"未提供"};
 public static string Reason(string? reason)=>reason switch {
  "port_busy"=>"其他程序正在使用该端口","occupancy_unknown"=>"无法确认端口占用情况，已跳过",
  "occupancy_timeout"=>"本轮占用检查超时，等待后续探测","occupancy_limit"=>"占用检查达到上限，已跳过","permission_denied"=>"无权打开串口",
  "open_failed"=>"串口当前无法打开","port_changed"=>"探测期间串口发生变化，等待重新识别",
  "exclusive_unavailable"=>"无法取得独占访问","not_tty"=>"不是可查询的TTY串口","configure_failed"=>"无法配置本次读取",
  "unsolicited_data"=>"端口持续输出或无法恢复安静状态","device_port_selected"=>"同一USB设备已选择其他端口",
  "identity_incomplete"=>"ATI或IMEI尚未完整读取；不代表模块支持所有命令",
  "sysfs_unavailable"=>"无法读取USB串口清单","payload_limit"=>"响应超出控制帧容量",
  null or ""=>"",_=>Status(reason)};
}
