namespace RouterWorkbench.Display;

public static class DevicePresentation
{
    public static readonly IReadOnlyDictionary<string, string> Builtins = new Dictionary<string, string>
    {
        ["device_id"]="设备标识", ["managed_name"]="管理名称", ["hostname"]="主机名称", ["model"]="探测型号", ["managed_model"]="管理型号",
        ["firmware"]="固件版本", ["serial"]="序列号", ["arch"]="架构", ["kernel"]="内核", ["libc"]="C运行库", ["probe_version"]="探针版本",
        ["boot_id"]="启动标识", ["uptime"]="开机时长", ["status"]="设备状态", ["capabilities"]="能力", ["cpu_model"]="CPU型号",
        ["cpu_soc"]="芯片平台", ["cpu_arch"]="CPU架构", ["cpu_hardware_bits"]="CPU位数", ["kernel_arch"]="内核架构", ["kernel_bits"]="内核位数",
        ["probe_bits"]="探针位数", ["cpu_max_mhz"]="CPU最高频率", ["cpu_usage"]="CPU使用率", ["cpu_current_mhz"]="CPU当前频率",
        ["memory_method"]="内存计算方式", ["memory_usage"]="内存使用率", ["memory_total_bytes"]="内存总量", ["memory_used_bytes"]="已用内存", ["memory_available_bytes"]="可用内存",
        ["egress_ipv4"]="出口IPv4", ["egress_ipv6"]="出口IPv6", ["configuration_state"]="配置状态", ["last_heartbeat"]="最近心跳", ["source_ip"]="连接来源IP",
        ["egress_ipv4_location"]="IPv4归属地", ["egress_ipv4_isp"]="IPv4运营商", ["egress_ipv6_location"]="IPv6归属地", ["egress_ipv6_isp"]="IPv6运营商",
        ["first_seen_at"]="首次发现", ["last_online_at"]="最近上线", ["last_offline_at"]="最近离线", ["session_id"]="会话标识",
        ["template_id"]="模板标识", ["template_name"]="模板名称", ["template_version"]="模板版本"
    };

    public static string DefaultGroup(string key) {
        if(key is "cpu_usage" or "cpu_current_mhz" || key.StartsWith("memory_") || key.StartsWith("disk_")) return "builtin_resources";
        if(key.StartsWith("net_") || key.StartsWith("switch_")) return "other";
        return Builtins.ContainsKey(key) ? "builtin_system" : "other";
    }

    public static readonly IReadOnlyDictionary<string,string> EditCategories = new Dictionary<string,string> {
        ["system"]="设备与系统", ["cpu"]="CPU", ["memory"]="内存", ["disk"]="磁盘",
        ["network"]="系统接口", ["switch"]="物理端口", ["egress"]="出口信息", ["connection"]="连接与模板", ["custom"]="自定义属性"
    };
    public static string EditCategory(string key) {
        if(key.StartsWith("cpu_") || key is "kernel_arch" or "kernel_bits" or "probe_bits") return "cpu";
        foreach(var prefix in new[]{"memory","disk","switch","egress"}) if(key.StartsWith(prefix+"_")) return prefix;
        if(key.StartsWith("net_")) return "network";
        if(key.StartsWith("template_") || key is "session_id" or "configuration_state" or "last_heartbeat" or "source_ip" or "first_seen_at" or "last_online_at" or "last_offline_at") return "connection";
        return Builtins.ContainsKey(key)?"system":"custom";
    }
}
