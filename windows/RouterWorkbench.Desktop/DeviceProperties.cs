using RouterWorkbench.Client;

namespace RouterWorkbench.Desktop;

public static class DeviceProperties
{
    public static PropertyRow[] Rows(Device device)
    {
        var r = device.Registration;
        var rows = new List<PropertyRow> { new("设备", "设备 ID", device.DeviceId), new("设备", "状态", device.StatusText) };
        foreach (var (key, name, value) in new[] {
            ("hostname", "主机名", r.Hostname), ("serial", "序列号", r.Serial), ("model", "型号", r.Model),
            ("firmware", "固件", r.Firmware), ("arch", "架构", r.Arch), ("libc", "C 库", r.Libc),
            ("kernel", "内核", r.Kernel), ("probe_version", "探针版本", r.ProbeVersion), ("boot_id", "Boot ID", r.BootId) })
            if (!string.IsNullOrEmpty(value) && r.CollectionErrors?.ContainsKey(key) != true) rows.Add(new("上报属性", name, value));
        foreach (var p in r.Attributes ?? []) rows.Add(new("模板属性", $"{p.Value.Name} ({p.Key})", p.Value.Value));
        foreach (var p in r.CollectionErrors ?? []) rows.Add(new("采集失败", $"{p.Value.Name} ({p.Key})", p.Value.Reason switch {
            "command_failed" => "采集命令失败", "timeout" => "采集超时", "empty" => "采集结果为空", "invalid_output" => "采集输出无效", "budget_exhausted" => "总采集时间已耗尽", _ => p.Value.Reason }));
        rows.Add(new("能力", "支持操作", string.Join(", ", r.Capabilities)));
        if (r.Template is { } t) rows.AddRange([new("模板", "名称", t.Name), new("模板", "ID", t.TemplateId), new("模板", "版本", t.Version.ToString())]);
        rows.AddRange([new("连接", "当前 Session", device.CurrentSession?.SessionId ?? "—"),
            new("连接", "首次发现", Labels.Time(device.FirstSeenAt)), new("连接", "最近上线", Labels.Time(device.LastOnlineAt)),
            new("连接", "最近心跳", Labels.Time(device.LastSeenAt)), new("连接", "最近下线", Labels.Time(device.LastOfflineAt))]);
        return rows.ToArray();
    }
}
