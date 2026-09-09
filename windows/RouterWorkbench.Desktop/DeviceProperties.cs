using RouterWorkbench.Client;
namespace RouterWorkbench.Desktop;
public static class DeviceProperties
{
    public static string Text(string? value) => string.IsNullOrEmpty(value) || value == "unknown" ? "—" : value;
    public static string Reason(string? reason) => reason switch {
        "port_counter_failed"=>"物理口字节计数读取失败、字段缺失或输出无效", "counter_baseline"=>"等待两次有效采样；首次采样、来源改变或计数重置后重新统计",
        "counter_out_of_range"=>"原始计数超出配置位宽", "unsupported_port_counters"=>"当前探针未声明物理端口计数能力",
        "command_failed" => "采集命令执行失败或固件未提供命令", "timeout" => "采集或查询超时",
        "empty" => "采集结果为空", "invalid_output" => "采集输出无效或超过长度限制",
        "budget_exhausted" => "本轮采集时间预算已耗尽", "model_prefix_missing" => "固件版本中未找到 v 前的设备型号",
        "switch_command_failed" => "交换机采集命令失败、超时或输出格式无效", "switch_not_supported" => "当前固件未提供可识别的物理端口，请配置匹配的交换机采集方式和端口映射",
        "interface_missing" => "配置的接口当前不存在或无法读取计数",
        "dns_busy" => "上一轮域名解析尚未完成", "dns_failed" => "出口服务域名解析失败",
        "connect_failed" => "无法连接出口探测服务", "cancelled" => "探测已取消",
        "tls_setup_failed" => "无法初始化安全连接", "tls_certificate_failed" => "出口服务证书校验失败，请检查设备时间",
        "tls_handshake_failed" => "出口服务安全连接失败", "invalid_http_response" => "出口服务响应无效或超过长度限制",
        "egress_request_failed" => "出口服务连接中断或响应读取失败",
        "payload_limit" => "监控报文超过上限，请缩小采集接口范围", "collection_disabled" => "探针配置已关闭此项采集",
        "invalid_ip" => "出口探测未返回有效的对应协议 IP 地址", null or "" => "设备未提供该参数", _ => reason };
    public static PropertyRow Field(Device d, string key, string name, string? fallback)
    {
        if(d.EffectiveMetrics?.TryGetValue(key,out var m)==true)
            return new("上报属性",name,TelemetryPresentation.Value(m,d.Online),TelemetryPresentation.Tip(m,d.Online)){Key=key,MetricGroup=m.Source=="template"?"template":m.Group};
        return new("上报属性",name,Text(fallback),Text(fallback)=="—"?"设备未提供该参数":""){Key=key,MetricGroup=key is "model" or "firmware"?"hardware":""};
    }
    public static PropertyRow Uptime(Device d) => new("运行状态","开机时长",FormatUptime(d.Runtime?.UptimeSeconds),
        d.Runtime==null?"尚未收到心跳":d.Runtime.UptimeSeconds==null?"设备无法读取系统开机时长":!d.Online?"设备离线，显示最后一次心跳上报值":""){Key="uptime"};
    public static PropertyRow[] Rows(Device device)
    {
        var r=device.Registration;
        var rows=new List<PropertyRow>{new("设备","设备 ID",device.DeviceId){Key="device_id"},new("设备","设备名称",device.DisplayName){Key="managed_name"},new("设备","状态",device.StatusText){Key="status"}};
        var standard=new[] {("hostname","主机名",r.Hostname),("serial","序列号",r.Serial),("model","设备型号",r.Model),
            ("firmware","固件版本",r.Firmware),("arch","工具兼容架构",r.Arch),("libc","C 库",r.Libc),
            ("kernel","内核版本",r.Kernel),("probe_version","探针版本",r.ProbeVersion),("boot_id","Boot ID",r.BootId)};
        foreach(var (key,name,value) in standard) rows.Add(Field(device,key,name,value));
        rows.Add(Uptime(device));
        rows.AddRange(TelemetryPresentation.Rows(device).Where(p=>!standard.Any(v=>p.Key==v.Item1)));
        rows.Add(new("能力","支持操作",Text(string.Join(", ",r.Capabilities))){Key="capabilities"});
        if(device.ActiveTemplate is {} t)rows.AddRange([new("模板","名称",t.Name){Key="template_name"},new("模板","ID",t.TemplateId){Key="template_id"},new("模板","版本",t.Version.ToString()){Key="template_version"}]);
        rows.AddRange([new("连接","当前 Session",device.CurrentSession?.SessionId??"—",device.CurrentSession==null?"当前没有在线会话":""){Key="session_id"},
            new("连接","首次发现",Labels.Time(device.FirstSeenAt)){Key="first_seen_at"},new("连接","最近上线",Labels.Time(device.LastOnlineAt)){Key="last_online_at"},
            new("连接","最近心跳",Labels.Time(device.Runtime?.ReportedAt),device.Runtime==null?"尚未收到心跳":""){Key="last_heartbeat"},new("连接","最近下线",Labels.Time(device.LastOfflineAt)){Key="last_offline_at"}]);
 return rows.ToArray();
    }
    public static string FormatUptime(long? seconds)
    {
        if(seconds is null or <0)return "—";
        var remaining=seconds.Value;var parts=new List<string>();
        foreach(var (size,unit) in new (long,string)[]{(365L*86400,"年"),(30L*86400,"月"),(86400,"天"),(3600,"小时"),(60,"分钟"),(1,"秒")}){
            var value=remaining/size;remaining%=size;
            if(value!=0||parts.Count!=0||size==1)parts.Add($"{value}{unit}");
        }
        return string.Concat(parts);
    }
}
