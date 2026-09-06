import type { ReactNode } from "react";
import { Activity, ArrowDown, ArrowUp, ArrowUpFromLine, ChevronRight, Clock3, Cpu, EthernetPort, FileText, Folder, Globe, HardDrive, History, Info, ListChecks, MemoryStick, Monitor, Package, Power, Radio, Router, ShieldCheck, Signal, Stethoscope, Terminal, Thermometer, Wrench } from "lucide-react";
import routerImage from "../preview/assets/router-device.png";
import type { Workbench } from "../useWorkbench";
import { stamp } from "../useWorkbench";
import "./overview.css";

type Props = { model: Workbench; open: (title: string, kind?: string) => void };
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

export function OverviewView({ model, open }: Props) {
  const source=model.device;
  if(!source)return <div className="empty"><Router/><p>{model.state.synchronized?"暂无设备，等待探针连接":"连接服务器后查看设备"}</p></div>;
  const device={id:source.device_id,name:source.registration.hostname||source.device_id,host:source.registration.hostname||"未提供",model:source.registration.model||"未提供",arch:source.registration.arch||"未提供",online:source.status==="online"};
  const active=!!model.active&&!model.active.released&&model.active.state==="ready"&&model.active.session_id===source.current_session?.session_id;
  const remainingSeconds=model.seconds;
  const onlineMinutes=source.last_online_at?Math.max(0,Math.floor((Date.now()-Date.parse(source.last_online_at))/60000)):0;
  const todayTasks=model.state.snapshot.tasks.filter(t=>t.device_id===device.id&&new Date(t.created_at).toDateString()===new Date().toDateString()&&t.state==="success");
  const recentTask=model.state.snapshot.tasks.filter(t=>t.device_id===device.id).sort((a,b)=>b.created_at.localeCompare(a.created_at))[0];
  return <div className="device-overview">
    <section className="overview-card network-info">
      <Heading icon={<Globe size={19}/>}>网络信息</Heading>
      <div className="ports-label">端口状态 · 未提供</div>
      <div className="port-list">{["WAN", "LAN 1", "LAN 2", "LAN 3", "LAN 4"].map((name, i) => <div key={name} className={`port ${""}`}><span><EthernetPort size={21}/></span><small>{name}</small></div>)}</div>
      <Facts rows={[
        ["设备名称", device.name], ["设备 ID", device.id], ["主机名", device.host],
        ["IP 地址", "未提供"], ["MAC 地址", "未提供"],
        ["上行链路", "未提供"],
      ]}/>
    </section>

    <section className="overview-card operation-summary">
      <Heading icon={<ShieldCheck size={19}/>} action={<span className="overview-chip">概况</span>}>连接与维护概况</Heading>
      <div className="connection-summary"><span className={`connection-emblem ${device.online ? "" : "offline"}`}><ShieldCheck size={28}/></span><div><strong>{device.online ? "设备连接正常" : "设备当前离线"}</strong><p>{device.online ? (active?"探针在线，远程维护已开启":"探针在线，尚未开启维护") : "等待探针重新建立连接"}</p></div><span className={`overview-status ${device.online ? "" : "offline"}`}>{device.online ? "在线" : "离线"}</span></div>
      <div className="summary-metrics">
        <div><span>本次在线</span><strong>{device.online ? `${Math.floor(onlineMinutes/60)} 小时 ${onlineMinutes%60} 分` : "—"}</strong></div>
        <div><span>最近活动</span><strong>{stamp(source.last_seen_at)}</strong></div>
        <div><span>维护连接</span><strong>{active ? model.active!.connections : 0}<small>个</small></strong></div>
        <div><span>今日任务</span><strong>{todayTasks.length}<small>项状态成功</small></strong></div>
      </div>
      <div className="recent-activity"><div><History size={14}/><span>最近动态</span><button className="text-button" onClick={() => open("全部维护记录", "history")}>查看记录<ChevronRight size={13}/></button></div><p><i/>{recentTask?`任务状态：${recentTask.state}`:"暂无任务"}<time>{stamp(recentTask?.created_at)}</time></p><p><i/>{active?"远程维护已开启":"未开启维护"}<time>{stamp(model.active?.created_at)}</time></p></div>
    </section>

    <section className="overview-card device-visual">
      <div className="device-visual-heading"><div><strong>{device.id}</strong><p>工业级多功能路由器</p></div><span className={`overview-status ${device.online ? "" : "offline"}`}>{device.online ? "稳定运行中" : "设备离线"}</span></div>
      <img src={routerImage} alt="黑色工业路由器设备示意图"/>
      <div className="device-visual-footer"><Router size={14}/><span>{device.model} / {device.arch}</span><small>设备示意图</small></div>
    </section>

    <section className="overview-card basic-info">
      <Heading icon={<Info size={19}/>}>基本信息</Heading>
      <Facts rows={[
        ["所属分组", "未提供"], ["设备位置", "未提供"],
        ["设备型号", device.model], ["系统类型", source.registration.kernel || "未提供"], ["CPU 架构", device.arch],
        ["硬件版本", "未提供"], ["探针版本", source.registration.probe_version || "未提供"], ["首次接入", stamp(source.first_seen_at)],
      ]}/>
      <button className="overview-card-link" onClick={() => open("设备详情", "details")}>查看完整设备资料<ChevronRight size={14}/></button>
    </section>

    <section className="overview-card expanded-maintenance">
      <Heading icon={<Wrench size={19}/>} action={<span className={`overview-status ${active ? "" : "offline"}`}>{active ? "维护已开启" : "未开启"}</span>}>快速维护</Heading>
      <div className="maintenance-session"><Clock3 size={16}/><span>{active ? "剩余" : "默认时长"} <strong>{active ? `${String(Math.floor(remainingSeconds / 3600)).padStart(2,"0")}:${String(Math.floor(remainingSeconds / 60) % 60).padStart(2,"0")}:${String(remainingSeconds % 60).padStart(2,"0")}` : "240 分钟"}</strong></span><button className="text-button" disabled={!model.enabled} onClick={() => open(active ? "关闭远程维护" : "开启远程维护", "maintenance")}><Power size={13}/>{active ? "关闭维护" : "开启维护"}</button></div>
      <div className="maintenance-actions">{quickActions.map(({ Icon, title, description, kind, entry }) => <button key={title} disabled={!model.enabled || (entry ? !active : false)} onClick={() => open(title, kind)}><span className="action-icon"><Icon size={20}/></span><ChevronRight className="action-arrow" size={14}/><strong>{title}</strong><small>{description}</small></button>)}</div>
      <div className="maintenance-utilities"><span>更多操作</span>{[[Stethoscope,"网络诊断"],[FileText,"日志采集"],[Folder,"配置备份"]].map(([Icon,title]) => {const Glyph = Icon as typeof FileText;return <button key={String(title)} disabled={!model.enabled} onClick={() => open(String(title), "prototype")}><Glyph size={14}/>{String(title)}</button>;})}</div>
    </section>

    <section className="overview-card mobile-info">
      <Heading icon={<Radio size={19}/>} action={<span className="overview-status offline">未提供</span>}>移动网络信息 <span className="network-type">—</span></Heading>
      <Facts rows={[
        ["运营商", "未提供"], ["网络制式", "未提供"], ["信号强度", "未提供"],
        ["SIM 卡号", "未提供"], ["IMEI", "未提供"], ["ICCID", "未提供"],
      ]}/>
      <div className="data-usage"><div><span>本月流量</span><strong>— <small>/ 未提供</small></strong></div><div className="usage-track"><i style={{width:0}}/></div><small>暂无套餐与流量数据</small></div>
    </section>

    <section className="overview-card runtime-info">
      <Heading icon={<Activity size={19}/>} action={<span className="runtime-note">当前接口未提供设备遥测</span>}>运行状态</Heading>
      <div className="runtime-metrics">{[
        { Icon: Thermometer, label: "设备温度", value: "—", unit: "°C", color: "orange" },
        { Icon: Cpu, label: "CPU 使用率", value: "—", unit: "%", color: "red" },
        { Icon: MemoryStick, label: "内存占用", value: "—", unit: "%", color: "blue" },
        { Icon: HardDrive, label: "磁盘占用", value: "—", unit: "%", color: "orange" },
        { Icon: Clock3, label: "运行时间", value: "—", unit: "未提供", color: "green" },
        { Icon: ArrowUp, label: "上行速率", value: "—", unit: "KB/s", color: "blue" },
        { Icon: ArrowDown, label: "下行速率", value: "—", unit: "KB/s", color: "blue" },
      ].map(({Icon,label,value,unit,color}) => <div className="runtime-metric" key={label}><span className={`metric-icon ${color}`}><Icon size={22}/></span><div><span>{label}</span><strong>{value}<small>{unit}</small></strong></div></div>)}</div>
    </section>
  </div>;
}
