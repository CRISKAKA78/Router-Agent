import { OverviewPreview } from "./OverviewPreview";
import { MaintenancePreview, type ShellProtocol } from "./MaintenancePreview";
import { TasksPreview } from "./TasksPreview";
import { RepositoryPreview } from "./RepositoryPreview";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Bell, Check, ChevronRight, CircleHelp, Copy, Ellipsis, Folder, History, LayoutGrid, Monitor, Moon, Package, PanelLeftClose, Play, RefreshCw, Router, Search, Settings, ShieldCheck, SlidersHorizontal, Sun, Terminal, X } from "lucide-react";

const devices = [
  { id: "RMP-SH-001", name: "上海总部 · 主路由", host: "sh-gateway-01", model: "OpenWrt", arch: "aarch64", online: true, location: "上海 · 总部机房" },
  { id: "RMP-SH-002", name: "上海总部 · 备用路由", host: "sh-gateway-02", model: "OpenWrt", arch: "aarch64", online: true, location: "上海 · 总部机房" },
  { id: "RMP-HZ-001", name: "杭州分部 · 办公网络", host: "hz-office-01", model: "Linux", arch: "mipsel", online: true, location: "杭州 · 办公区" },
  { id: "RMP-SZ-001", name: "深圳分部 · 核心路由", host: "sz-gateway-01", model: "OpenWrt", arch: "armv7", online: false, location: "深圳 · 核心机房" },
  { id: "RMP-BJ-001", name: "北京分部 · 测试设备", host: "bj-lab-01", model: "Linux", arch: "x86_64", online: true, location: "北京 · 测试实验室" },
  { id: "RMP-CD-001", name: "成都分部 · 边缘网关", host: "cd-edge-01", model: "OpenWrt", arch: "mipsel", online: false, location: "成都 · 边缘节点" },
];
type RecordRow = { state: string; start: string; end: string; duration: string; reason: string };
const initialRecords: RecordRow[] = [
  { state: "已关闭", start: "2026-09-06 09:30:00", end: "2026-09-06 10:12:36", duration: "42 分 36 秒", reason: "主动关闭" },
  { state: "已到期", start: "2026-09-05 14:00:00", end: "2026-09-05 18:00:00", duration: "4 小时", reason: "维护租期到期" },
  { state: "已关闭", start: "2026-09-05 10:16:24", end: "2026-09-05 10:48:02", duration: "31 分 38 秒", reason: "主动关闭" },
];
function Badge({ online, children }: { online?: boolean; children: ReactNode }) {
  return <span className={`badge ${online ? "green" : "neutral"}`}><i />{children}</span>;
}
export function WorkbenchPreview() {
  const [page, setPage] = useState("overview");
  const globalPage = ["tasks", "files", "tools"].includes(page);
  const [shellRequest, setShellRequest] = useState<{ protocol: ShellProtocol; id: number } | null>(null);
  const [selected, setSelected] = useState(devices[0].id);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("all");
  const [dark, setDark] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const [mobileList, setMobileList] = useState(false);
  const [activeByDevice, setActiveByDevice] = useState<Record<string, boolean>>({ [devices[0].id]: true });
  const [lease, setLease] = useState("240");
  const [previewMinutes, setPreviewMinutes] = useState<Record<string, number>>({});
  const [dialog, setDialog] = useState<{ title: string; kind: string; record?: RecordRow } | null>(null);
  const [toast, setToast] = useState("");
  const [command, setCommand] = useState("uptime");
  const [commandDone, setCommandDone] = useState(false);
  const dialogRef = useRef<HTMLDialogElement>(null);
  const device = devices.find(d => d.id === selected)!;
  const active = !!activeByDevice[selected];
  const remainingSeconds = previewMinutes[selected] === undefined ? 13338 : previewMinutes[selected] * 60;
  const filtered = devices.filter(d => (filter === "all" || (filter === "online" ? d.online : !d.online)) && `${d.name} ${d.id} ${d.host}`.toLowerCase().includes(query.toLowerCase()));
  useEffect(() => { if (!toast) return; const t = setTimeout(() => setToast(""), 3200); return () => clearTimeout(t); }, [toast]);
  useEffect(() => { if (dialog) dialogRef.current?.showModal(); else dialogRef.current?.close(); }, [dialog]);
  const navigate = (target: string) => { setPage(target); window.location.hash = target; };
  useEffect(() => {
    const syncPage = () => { const target = window.location.hash.slice(1); setPage(["tasks", "files", "tools", "maintenance"].includes(target) ? target : "overview"); };
    syncPage(); window.addEventListener("hashchange", syncPage);
    return () => window.removeEventListener("hashchange", syncPage);
  }, []);
  const open = (title: string, kind = "info") => {
    if (kind === "endpoint" && (title.startsWith("SSH") || title.startsWith("Telnet"))) {
      navigate("maintenance");
      setShellRequest(prev => ({ protocol: title.startsWith("SSH") ? "SSH" : "Telnet", id: (prev?.id ?? 0) + 1 }));
      return;
    }
    setCommandDone(false); setDialog({ title, kind });
  };
  const notify = (text: string) => setToast(text);
  const toggleMaintenance = () => {
    if (!active) setPreviewMinutes(prev => ({ ...prev, [selected]: Number(lease) }));
    setActiveByDevice(prev => ({ ...prev, [selected]: !active }));
    setDialog(null);
    notify(active ? "维护已关闭 · 演示状态" : `维护已开启，时长 ${lease} 分钟 · 演示状态`);
  };
  return <div className={`preview overview-layout ${globalPage ? "tasks-layout" : ""} ${dark ? "dark" : ""} ${collapsed ? "collapsed" : ""}`}>
    <header className="appbar">
      <div className="brand"><span className="brand-symbol"><Router size={25} strokeWidth={1.7} /></span><div><strong>设备远程维护工作台</strong><small>Router Maintenance Workbench</small></div></div>
      <div className="appbar-actions"><span className="preview-tag">Mock 视觉预览</span><span className="demo-connection"><i />演示工作区</span><span className="bar-divider"/><button className="icon" aria-label="通知" onClick={() => notify("暂无新通知")}><Bell size={19}/></button><button className="icon" aria-label="切换浅色或深色主题" onClick={() => setDark(!dark)}>{dark ? <Sun size={19}/> : <Moon size={19}/>}</button><span className="user-avatar">运</span></div>
    </header>
    <div className="workspace">
      <nav className="rail" aria-label="主导航">
        <div className="nav-label">工作空间</div>
        {[[LayoutGrid,"设备管理"],[Terminal,"任务中心"],[Folder,"文件管理"],[Package,"工具仓库"]].map(([Icon, label], i) => { const Glyph = Icon as typeof LayoutGrid; return <button key={String(label)} title={String(label)} className={`nav-item ${(i === 0 && !globalPage) || page === ["overview", "tasks", "files", "tools"][i] ? "selected" : ""}`} onClick={() => i === 0 ? (globalPage ? navigate("overview") : setMobileList(!mobileList)) : navigate(["overview", "tasks", "files", "tools"][i])}><Glyph size={20}/><span>{String(label)}</span>{i === 0 && <span className="nav-count">6</span>}</button>; })}
        <div className="rail-bottom"><button className="nav-item" title="设置" onClick={() => open("工作台设置", "settings")}><Settings size={20}/><span>设置</span></button><button className="nav-item" title="帮助与反馈" onClick={() => open("帮助与反馈", "help")}><CircleHelp size={20}/><span>帮助与反馈</span></button><div className="rail-foot"><span>v0.6.0</span><button className="icon" aria-label="折叠或展开导航" onClick={() => setCollapsed(!collapsed)}><PanelLeftClose size={17}/></button></div></div>
      </nav>
      <aside className={`device-list ${mobileList ? "mobile-open" : ""}`} aria-label="设备列表">
        <div className="list-heading"><h2>设备列表 <span>6</span></h2><button className="icon" aria-label="刷新设备列表" onClick={() => notify("设备列表已刷新 · Mock 数据")}><RefreshCw size={17}/></button></div>
        <label className="search"><Search size={17}/><input placeholder="搜索设备名称或 ID" aria-label="搜索设备" value={query} onChange={e => setQuery(e.target.value)}/><span>⌕</span></label>
        <div className="filter-row"><div className="filter-tabs">{[["all","全部",6],["online","在线",4],["offline","离线",2]].map(([key,label,count]) => <button key={key} className={filter === key ? "selected" : ""} onClick={() => setFilter(String(key))}>{label}<span>{count}</span></button>)}</div><SlidersHorizontal size={15} aria-hidden="true"/></div>
        <div className="device-items">{filtered.map(d => <button key={d.id} className={`device-item ${d.id === selected ? "selected" : ""}`} onClick={() => { setSelected(d.id); setShellRequest(null); setMobileList(false); }}><span className={`device-icon ${d.online ? "" : "offline"}`}><Router size={22}/><i/></span><span className="device-text"><strong>{d.name}</strong><small>{d.id}</small></span><span className={`device-status ${d.online ? "" : "offline"}`}>{d.online ? "在线" : "离线"}</span></button>)}{!filtered.length && <div className="empty"><Search/><p>没有匹配的设备</p><button className="text-button" onClick={() => { setQuery(""); setFilter("all"); }}>清除筛选</button></div>}</div>
        <div className="list-footer"><span><i/>4 台在线</span><span>共 6 台设备</span></div>
      </aside>
      <main hidden={globalPage} className={`main-panel device-main ${page === "maintenance" ? "maintenance-page" : ""}`}>
        <div className="breadcrumb">设备管理<ChevronRight size={13}/><span>设备工作台</span><button className="mobile-device-picker text-button" onClick={() => setMobileList(!mobileList)}>切换设备</button></div>
        <section className="device-header"><div className="header-top"><div className="device-title-icon"><Router size={30} strokeWidth={1.6}/></div><div className="header-name"><div className="title-line"><h1>{device.name}</h1><Badge online={device.online}>{device.online ? "在线" : "离线"}</Badge></div><p>{device.location}<span>·</span>{device.host}</p></div><div className="header-actions"><button onClick={() => notify("设备信息已刷新 · Mock 数据")}><RefreshCw size={15}/>刷新</button><button className="icon" aria-label="更多设备操作" onClick={() => open("设备详情", "details")}><Ellipsis size={20}/></button></div></div><div className="device-meta"><span>设备 ID <b>{device.id}</b><button className="icon copy" aria-label="复制设备 ID" onClick={() => { void navigator.clipboard.writeText(device.id).then(() => notify("设备 ID 已复制"), () => notify("复制失败，请从设备详情手动复制")); }}><Copy size={12}/></button></span><span>系统 <b>{device.model} / {device.arch}</b></span><span>探针版本 <b>v0.6.0</b></span><span>最近上线 <b>今天 09:24:16</b></span></div></section>
        <div className="page-tabs" role="tablist" aria-label="设备功能">{[[LayoutGrid,"设备概览"],[ShieldCheck,"远程维护"],[SlidersHorizontal,"配置管理"],[History,"任务记录"],[Bell,"告警事件"],[Folder,"文件管理"],[Settings,"设备设置"]].map(([Icon,label],i) => { const Glyph = Icon as typeof LayoutGrid; const selectedTab = i === 0 ? page === "overview" : i === 1 && page === "maintenance"; return <button role="tab" aria-selected={selectedTab} key={String(label)} className={selectedTab ? "selected" : ""} onClick={() => i < 2 ? navigate(i === 0 ? "overview" : "maintenance") : open(String(label), i === 3 ? "exec" : i === 6 ? "details" : "prototype")}><Glyph size={16}/>{String(label)}</button>; })}</div>
        <div className="page-content">
          <div hidden={page !== "overview"}><OverviewPreview device={device} active={active} remainingSeconds={remainingSeconds} open={open}/></div>
          <div hidden={page !== "maintenance"}><MaintenancePreview key={device.id} device={device} active={active} remainingSeconds={remainingSeconds} request={shellRequest} open={open} notify={notify}/></div>
          <footer className="workspace-footer"><ShieldCheck size={13}/><span>每一次连接，都让维护更简单</span><span className="footer-demo">当前内容为演示数据</span></footer>
        </div>
      </main>
      <div className="tasks-page-host" hidden={page !== "tasks"}><TasksPreview devices={devices} notify={notify}/></div>
      <div className="repo-page-host" hidden={!["files", "tools"].includes(page)}><RepositoryPreview page={page} devices={devices} notify={notify}/></div>
    </div>
    {toast && <div className="toast" role="status"><Check size={18}/>{toast}<button className="icon" aria-label="关闭提示" onClick={() => setToast("")}><X size={15}/></button></div>}
    <dialog ref={dialogRef} onCancel={() => setDialog(null)} onClick={e => { if(e.target === e.currentTarget) setDialog(null); }}><div className="dialog-header"><h2>{dialog?.title}</h2><button className="icon" aria-label="关闭对话框" onClick={() => setDialog(null)}><X size={20}/></button></div><div className="dialog-body">
      {dialog?.kind === "maintenance" ? <><p>{active ? "关闭后，Web、SSH 和 Telnet 入口将不可用。" : `为「${device.name}」开启三个维护入口。`}</p>{!active && <label className="lease-input">维护时长（分钟）<input type="number" min="1" step="1" value={lease} onChange={e => setLease(e.target.value)}/><small>默认 240 分钟，可自定义正整数分钟。</small></label>}<div className="dialog-actions"><button onClick={() => setDialog(null)}>取消</button><button className={active ? "danger-soft" : "primary"} disabled={!active && (!/^\d+$/.test(lease) || Number(lease) < 1)} onClick={toggleMaintenance}>{active ? "关闭维护" : "开启维护"}</button></div></> : dialog?.kind === "exec" ? <><p>在 {device.name} 上预览命令执行交互。</p><label className="lease-input">命令<input value={command} onChange={e => {setCommand(e.target.value);setCommandDone(false);}}/></label><button className="primary" disabled={!command.trim()} onClick={() => setCommandDone(true)}><Play size={14}/>运行演示</button>{commandDone && <pre>$ {command}{"\n"}演示结果：命令已完成（退出码 0）{"\n"}此输出为 Mock 数据。</pre>}</> : dialog?.kind === "details" ? <dl><dt>设备 ID</dt><dd>{device.id}</dd><dt>主机名</dt><dd>{device.host}</dd><dt>系统</dt><dd>{device.model} / {device.arch}</dd><dt>位置</dt><dd>{device.location}</dd><dt>会话编号</dt><dd>demo-session-001</dd></dl> : dialog?.kind === "record" ? <dl><dt>状态</dt><dd>{dialog.record?.state}</dd><dt>设备</dt><dd>{device.name}</dd><dt>维护编号</dt><dd>demo-maintenance-001</dd><dt>开始时间</dt><dd>{dialog.record?.start}</dd><dt>结束时间</dt><dd>{dialog.record?.end}</dd><dt>释放原因</dt><dd>{dialog.record?.reason}</dd></dl> : dialog?.kind === "settings" ? <><p>外观</p><div className="dialog-actions"><button onClick={() => setDark(false)}><Sun size={16}/>浅色</button><button onClick={() => setDark(true)}><Moon size={16}/>深色</button></div><p>当前为本地 Mock 视觉预览。</p></> : dialog?.kind === "history" ? <div className="history-preview">{initialRecords.map((r,i) => <div key={i}><Badge>{r.state}</Badge><span>{r.start}</span><span>{r.duration}</span></div>)}</div> : dialog?.kind === "endpoint" ? <><div className="endpoint-preview-icon"><Monitor size={36}/></div><p>入口已就绪，正式交互将从这里打开设备管理页或外部终端。</p><small>本轮为 Mock 预览，当前不发起真实连接。</small></> : <><p>{dialog?.kind === "help" ? "通过设备列表选择设备，再从远程维护卡片进入 Web、SSH 或 Telnet。" : `${dialog?.title}的入口位置与弹窗交互预览。`}</p><small>本轮聚焦设备远程维护工作台主页面。</small></>}
    </div></dialog>
  </div>;
}
