import type { ReactNode } from "react";
import { Activity, ArrowDown, ArrowUp, ArrowUpFromLine, ChevronRight, Clock3, Cpu, EthernetPort, FileText, Folder, Globe, HardDrive, History, Info, ListChecks, MemoryStick, Monitor, Package, Power, Radio, Router, ShieldCheck, Signal, Stethoscope, Terminal, Thermometer, Wrench } from "lucide-react";
import routerImage from "./assets/router-device.png";
import "./overview.css";

type PreviewDevice = { id: string; name: string; host: string; model: string; arch: string; online: boolean; location: string };
type Props = {
  device: PreviewDevice;
  active: boolean;
  remainingSeconds: number;
  open: (title: string, kind?: string) => void;
};
function Heading({ icon, children, action }: { icon: ReactNode; children: ReactNode; action?: ReactNode }) {
  return <div className="overview-heading"><h2><span>{icon}</span>{children}</h2>{action}</div>;
}
function Facts({ rows }: { rows: [string, ReactNode][] }) {
  return <dl className="overview-facts">{rows.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>;
}
const quickActions = [
  { Icon: Globe, title: "Web 管理", description: "打开设备管理页面", kind: "endpoint", entry: true },
  { Icon: Terminal, title: "SSH 连接", description: "安全的命令行连接", kind: "endpoint", entry: true },
  { Icon: Monitor, title: "Telnet 连接", description: "远程终端维护", kind: "endpoint", entry: true },
  { Icon: ListChecks, title: "执行命令", description: "下发命令与查看结果", kind: "exec" },
  { Icon: ArrowUpFromLine, title: "文件传输", description: "上传文件与下载日志", kind: "prototype" },
  { Icon: Package, title: "工具部署", description: "选择工具并投放设备", kind: "prototype" },
];

export function OverviewPreview({ device, active, remainingSeconds, open }: Props) {
  return <div className="device-overview">
    <section className="overview-card network-info">
      <Heading icon={<Globe size={19}/>}>网络信息</Heading>
      <div className="ports-label">端口状态</div>
      <div className="port-list">{["WAN", "LAN 1", "LAN 2", "LAN 3", "LAN 4"].map((name, i) => <div key={name} className={`port ${device.online && i < 3 ? "linked" : ""}`}><span><EthernetPort size={21}/></span><small>{name}</small></div>)}</div>
      <Facts rows={[
        ["设备名称", device.name], ["设备 ID", device.id], ["主机名", device.host],
        ["IP 地址", "192.168.1.1"], ["MAC 地址", "54:D0:B4:47:47:6C"],
        ["上行链路", <span className="overview-positive">{device.online ? "WAN · 已连接" : "未连接"}</span>],
      ]}/>
    </section>

    <section className="overview-card operation-summary">
      <Heading icon={<ShieldCheck size={19}/>} action={<span className="overview-chip">概况</span>}>连接与维护概况</Heading>
      <div className="connection-summary"><span className={`connection-emblem ${device.online ? "" : "offline"}`}><ShieldCheck size={28}/></span><div><strong>{device.online ? "设备连接正常" : "设备当前离线"}</strong><p>{device.online ? "探针在线，维护通道已就绪" : "等待探针重新建立连接"}</p></div><span className={`overview-status ${device.online ? "" : "offline"}`}>{device.online ? "在线" : "离线"}</span></div>
      <div className="summary-metrics">
        <div><span>本次在线</span><strong>{device.online ? "5 小时 11 分" : "—"}</strong></div>
        <div><span>最近心跳</span><strong>{device.online ? "3 秒前" : "2 小时前"}</strong></div>
        <div><span>维护连接</span><strong>{active ? "2" : "0"}<small>个</small></strong></div>
        <div><span>今日任务</span><strong>8<small>项已完成</small></strong></div>
      </div>
      <div className="recent-activity"><div><History size={14}/><span>最近动态</span><button className="text-button" onClick={() => open("全部维护记录", "history")}>查看记录<ChevronRight size={13}/></button></div><p><i/>文件传输完成 <time>14:12</time></p><p><i/>远程维护已开启 <time>14:00</time></p></div>
    </section>

    <section className="overview-card device-visual">
      <div className="device-visual-heading"><div><strong>{device.id}</strong><p>工业级多功能路由器</p></div><span className={`overview-status ${device.online ? "" : "offline"}`}>{device.online ? "稳定运行中" : "设备离线"}</span></div>
      <img src={routerImage} alt="黑色工业路由器设备示意图"/>
      <div className="device-visual-footer"><Router size={14}/><span>{device.model} / {device.arch}</span><small>设备示意图</small></div>
    </section>

    <section className="overview-card basic-info">
      <Heading icon={<Info size={19}/>}>基本信息</Heading>
      <Facts rows={[
        ["所属分组", device.name.split(" · ")[0]], ["设备位置", device.location],
        ["设备型号", "RGV120-01AD"], ["系统类型", device.model], ["CPU 架构", device.arch],
        ["硬件版本", "V1.0"], ["探针版本", "v0.6.0"], ["首次接入", "2026-08-24 10:24:36"],
      ]}/>
      <button className="overview-card-link" onClick={() => open("设备详情", "details")}>查看完整设备资料<ChevronRight size={14}/></button>
    </section>

    <section className="overview-card expanded-maintenance">
      <Heading icon={<Wrench size={19}/>} action={<span className={`overview-status ${active ? "" : "offline"}`}>{active ? "维护已开启" : "未开启"}</span>}>快速维护</Heading>
      <div className="maintenance-session"><Clock3 size={16}/><span>{active ? "剩余" : "默认时长"} <strong>{active ? `${String(Math.floor(remainingSeconds / 3600)).padStart(2,"0")}:${String(Math.floor(remainingSeconds / 60) % 60).padStart(2,"0")}:${String(remainingSeconds % 60).padStart(2,"0")}` : "240 分钟"}</strong></span><button className="text-button" disabled={!device.online} onClick={() => open(active ? "关闭远程维护" : "开启远程维护", "maintenance")}><Power size={13}/>{active ? "关闭维护" : "开启维护"}</button></div>
      <div className="maintenance-actions">{quickActions.map(({ Icon, title, description, kind, entry }) => <button key={title} disabled={entry ? !active : !device.online} onClick={() => open(title, kind)}><span className="action-icon"><Icon size={20}/></span><ChevronRight className="action-arrow" size={14}/><strong>{title}</strong><small>{description}</small></button>)}</div>
      <div className="maintenance-utilities"><span>更多操作</span>{[[Stethoscope,"网络诊断"],[FileText,"日志采集"],[Folder,"配置备份"]].map(([Icon,title]) => {const Glyph = Icon as typeof FileText;return <button key={String(title)} disabled={!device.online} onClick={() => open(String(title), "prototype")}><Glyph size={14}/>{String(title)}</button>;})}</div>
    </section>

    <section className="overview-card mobile-info">
      <Heading icon={<Radio size={19}/>} action={<span className="overview-status">在线</span>}>移动网络信息 <span className="network-type">4G</span></Heading>
      <Facts rows={[
        ["运营商", "中国移动"], ["网络制式", "4G LTE"], ["信号强度", <span className="signal-value"><Signal size={16}/>−72 dBm</span>],
        ["SIM 卡号", "460045540216387"], ["IMEI", "868517076674245"], ["ICCID", "89860485192000123456"],
      ]}/>
      <div className="data-usage"><div><span>本月流量</span><strong>498.4 MB <small>/ 3 GB</small></strong></div><div className="usage-track"><i/></div><small>已使用套餐流量的 16%</small></div>
    </section>

    <section className="overview-card runtime-info">
      <Heading icon={<Activity size={19}/>} action={<span className="runtime-note">Mock 数据 · 字段待调整</span>}>运行状态</Heading>
      <div className="runtime-metrics">{[
        { Icon: Thermometer, label: "设备温度", value: "42.0", unit: "°C", color: "orange" },
        { Icon: Cpu, label: "CPU 使用率", value: "4.0", unit: "%", color: "red" },
        { Icon: MemoryStick, label: "内存占用", value: "38.2", unit: "%", color: "blue" },
        { Icon: HardDrive, label: "磁盘占用", value: "26.2", unit: "%", color: "orange" },
        { Icon: Clock3, label: "运行时间", value: "12", unit: "天 6 小时", color: "green" },
        { Icon: ArrowUp, label: "上行速率", value: "8.1", unit: "KB/s", color: "blue" },
        { Icon: ArrowDown, label: "下行速率", value: "40.0", unit: "KB/s", color: "blue" },
      ].map(({Icon,label,value,unit,color}) => <div className="runtime-metric" key={label}><span className={`metric-icon ${color}`}><Icon size={22}/></span><div><span>{label}</span><strong>{value}<small>{unit}</small></strong></div></div>)}</div>
    </section>
  </div>;
}
