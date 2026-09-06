import {useEffect,useRef,useState,type ReactNode} from 'react';
import {Bell,Check,ChevronRight,CircleHelp,Copy,Ellipsis,Folder,History,LayoutGrid,Moon,Package,PanelLeftClose,RefreshCw,Router,Search,Settings,ShieldCheck,SlidersHorizontal,Sun,Terminal,X} from 'lucide-react';
import {useWorkbench,stamp} from './useWorkbench';
import {describe,leaseBody,mutation,segment} from './api';
import {platform} from './platform';
import {FormDialog} from './components';
import {OverviewView} from './ui/OverviewView';
import {MaintenanceView,type ShellProtocol} from './ui/MaintenanceView';
import {TasksView} from './ui/TasksView';
import {RepositoryView} from './ui/RepositoryView';
import {SettingsPage} from './pages/SettingsPage';
import {DetailsPage} from './pages/DetailsPage';
function Badge({online,children}:{online?:boolean;children:ReactNode}){return <span className={`badge ${online?'green':'neutral'}`}><i/>{children}</span>;}
export function App(){
 const model=useWorkbench();
 const {page,setPage,selected,setSelected,query,setQuery,state,owner,busy,error,setError,notice,setNotice,dialog,setDialog,inspect,setInspect,run,form,field}=model;
 const globalPage=['tasks','files','tools','settings'].includes(page);
 const [filter,setFilter]=useState('all'),[collapsed,setCollapsed]=useState(false),[mobileList,setMobileList]=useState(false);
 const [systemDark,setSystemDark]=useState(matchMedia('(prefers-color-scheme: dark)').matches);
 const dark=model.profile.theme==='Dark'||(model.profile.theme==='Default'&&systemDark);
 useEffect(()=>{const media=matchMedia('(prefers-color-scheme: dark)');const change=()=>setSystemDark(media.matches);media.addEventListener('change',change);return()=>media.removeEventListener('change',change);},[]);
 const toggleTheme=()=>{const p={...model.profile,theme:dark?'Light' as const:'Dark' as const};model.setProfile(p);void platform.saveProfile(p).catch(e=>setError(describe(e)));};
 const [shellRequest,setShellRequest]=useState<{protocol:ShellProtocol;id:number}|null>(null);
 const nextRequest=useRef(0);
 const devices=state.snapshot.devices.map(d=>({id:d.device_id,name:d.registration.hostname||d.device_id,host:d.registration.hostname||'—',model:d.registration.kernel||'—',arch:d.registration.arch||'—',online:d.status==='online',location:d.registration.model||'—'}));
 const device=devices.find(d=>d.id===selected)??{id:'—',name:'设备工作台',host:'—',model:'—',arch:'—',online:false,location:'等待选择设备'};
 const online=devices.filter(d=>d.online).length;
 const filtered=devices.filter(d=>(filter==='all'||(filter==='online'?d.online:!d.online))&&`${d.name} ${d.id} ${d.host}`.toLowerCase().includes(query.toLowerCase()));
 const notify=setNotice;
 const navigate=(target:string)=>setPage(target as typeof page);
 const open=(title:string,kind='info')=>{
  if(kind==='maintenance'){
   const active=model.active;
   if(active&&!active.released){form('关闭远程维护',[],async()=>{await platform.closeTerminals();await model.write(mutation('关闭维护',`maintenance/${segment(active.maintenance_id)}/close`));});}
   else form('开启远程维护',[field('minutes','维护时长（分钟）','240')],async v=>{await model.write(mutation('开启维护','maintenance',leaseBody(selected,v.minutes)));});
  }else if(kind==='endpoint'){
   if(title.startsWith('Web')){if(model.active)void run('打开 Web 管理',()=>model.open(model.active!,'web'));}
   else {navigate('maintenance');setShellRequest({protocol:title.startsWith('SSH')?'SSH':'Telnet',id:++nextRequest.current});}
  }else if(kind==='exec')model.exec();
  else if(kind==='details'||kind==='history')navigate('details');
  else if(kind==='tasks')navigate('tasks');
  else if(kind==='files'||title==='文件传输'||title==='文件管理')navigate('files');
  else if(title==='工具部署')navigate('tools');
  else setInspect({title,value:kind==='help'?'配置服务器后选择在线设备。远程维护默认使用内置 Shell，可在新建会话菜单选择外部工具。任务、文件和工具操作通过服务器执行。':'当前 API 未提供此项能力。'});
 };
 useEffect(()=>{if(!notice)return;const timer=setTimeout(()=>setNotice(''),5000);return()=>clearTimeout(timer);},[notice]);
  return <div className={`preview overview-layout ${globalPage ? "tasks-layout" : ""} ${dark ? "dark" : ""} ${collapsed ? "collapsed" : ""}`}>
    <header className="appbar">
      <div className="brand"><span className="brand-symbol"><Router size={25} strokeWidth={1.7} /></span><div><strong>设备远程维护工作台</strong><small>Router Maintenance Workbench</small></div></div>
      <div className="appbar-actions"><span className="preview-tag">正式工作台</span><button className="demo-connection text-button" onClick={()=>navigate("settings")}><i style={{background:state.synchronized?"var(--green)":"var(--muted)"}}/>{state.status}</button><span className="bar-divider"/><button className="icon" aria-label="通知" onClick={() => notify(state.error||"暂无新通知")}><Bell size={19}/></button><button className="icon" aria-label="切换浅色或深色主题" onClick={() => toggleTheme()}>{dark ? <Sun size={19}/> : <Moon size={19}/>}</button><span className="user-avatar">运</span></div>
    </header>
    <div className="workspace">
      <nav className="rail" aria-label="主导航">
        <div className="nav-label">工作空间</div>
        {[[LayoutGrid,"设备管理"],[Terminal,"任务中心"],[Folder,"文件管理"],[Package,"工具仓库"]].map(([Icon, label], i) => { const Glyph = Icon as typeof LayoutGrid; return <button key={String(label)} title={String(label)} className={`nav-item ${(i === 0 && !globalPage) || page === ["overview", "tasks", "files", "tools"][i] ? "selected" : ""}`} onClick={() => i === 0 ? (globalPage ? navigate("overview") : setMobileList(!mobileList)) : navigate(["overview", "tasks", "files", "tools"][i])}><Glyph size={20}/><span>{String(label)}</span>{i === 0 && <span className="nav-count">{devices.length}</span>}</button>; })}
        <div className="rail-bottom"><button className={`nav-item ${page==="settings"?"selected":""}`} title="设置" onClick={() => navigate("settings")}><Settings size={20}/><span>设置</span></button><button className="nav-item" title="帮助与反馈" onClick={() => open("帮助与反馈", "help")}><CircleHelp size={20}/><span>帮助与反馈</span></button><div className="rail-foot"><span>v0.6.0</span><button className="icon" aria-label="折叠或展开导航" onClick={() => setCollapsed(!collapsed)}><PanelLeftClose size={17}/></button></div></div>
      </nav>
      <aside className={`device-list ${mobileList ? "mobile-open" : ""}`} aria-label="设备列表">
        <div className="list-heading"><h2>设备列表 <span>{devices.length}</span></h2><button className="icon" aria-label="刷新设备列表" onClick={() => owner.current?.invalidate()}><RefreshCw size={17}/></button></div>
        <label className="search"><Search size={17}/><input placeholder="搜索设备名称或 ID" aria-label="搜索设备" value={query} onChange={e => setQuery(e.target.value)}/><span>⌕</span></label>
        <div className="filter-row"><div className="filter-tabs">{[["all","全部",devices.length],["online","在线",online],["offline","离线",devices.length-online]].map(([key,label,count]) => <button key={key} className={filter === key ? "selected" : ""} onClick={() => setFilter(String(key))}>{label}<span>{count}</span></button>)}</div><SlidersHorizontal size={15} aria-hidden="true"/></div>
        <div className="device-items">{filtered.map(d => <button key={d.id} disabled={busy} className={`device-item ${d.id === selected ? "selected" : ""}`} onClick={() => { setSelected(d.id); setShellRequest(null); setMobileList(false); }}><span className={`device-icon ${d.online ? "" : "offline"}`}><Router size={22}/><i/></span><span className="device-text"><strong>{d.name}</strong><small>{d.id}</small></span><span className={`device-status ${d.online ? "" : "offline"}`}>{d.online ? "在线" : "离线"}</span></button>)}{!filtered.length && <div className="empty"><Search/><p>{owner.current?"没有匹配的设备":"连接服务器后显示设备"}</p><button className="text-button" onClick={() => { setQuery(""); setFilter("all"); }}>清除筛选</button>{!owner.current&&<button onClick={()=>navigate("settings")}>配置连接</button>}</div>}</div>
        <div className="list-footer"><span><i/>{online} 台在线</span><span>共 {devices.length} 台设备</span></div>
      </aside>
      <main hidden={globalPage} className={`main-panel device-main ${page === "maintenance" ? "maintenance-page" : ""}`}>
        <div className="breadcrumb">设备管理<ChevronRight size={13}/><span>设备工作台</span><button className="mobile-device-picker text-button" onClick={() => setMobileList(!mobileList)}>切换设备</button></div>
        <section className="device-header"><div className="header-top"><div className="device-title-icon"><Router size={30} strokeWidth={1.6}/></div><div className="header-name"><div className="title-line"><h1>{device.name}</h1><Badge online={device.online}>{model.device?(device.online?"在线":"离线"):"未选择"}</Badge></div><p>{device.location}<span>·</span>{device.host}</p></div><div className="header-actions"><button onClick={() => owner.current?.invalidate()}><RefreshCw size={15}/>刷新</button><button className="icon" aria-label="更多设备操作" onClick={() => open("设备详情", "details")}><Ellipsis size={20}/></button></div></div><div className="device-meta"><span>设备 ID <b>{device.id}</b><button className="icon copy" aria-label="复制设备 ID" onClick={() => { void navigator.clipboard.writeText(device.id).then(() => notify("设备 ID 已复制"), () => notify("复制失败，请从设备详情手动复制")); }}><Copy size={12}/></button></span><span>系统 <b>{device.model} / {device.arch}</b></span><span>探针版本 <b>{model.device?.registration.probe_version||"—"}</b></span><span>最近上线 <b>{stamp(model.device?.last_online_at)}</b></span></div></section>
        <div className="page-tabs" role="tablist" aria-label="设备功能">{[[LayoutGrid,"设备概览"],[ShieldCheck,"远程维护"],[SlidersHorizontal,"配置管理"],[History,"任务记录"],[Bell,"告警事件"],[Folder,"文件管理"],[Settings,"设备设置"]].map(([Icon,label],i) => { const Glyph = Icon as typeof LayoutGrid; const selectedTab = i === 0 ? page === "overview" : i === 1 && page === "maintenance"; return <button role="tab" aria-selected={selectedTab} key={String(label)} className={selectedTab ? "selected" : ""} onClick={() => i < 2 ? navigate(i === 0 ? "overview" : "maintenance") : open(String(label), i === 3 ? "tasks" : i === 5 ? "files" : i === 6 ? "details" : "prototype")}><Glyph size={16}/>{String(label)}</button>; })}</div>
        <div className="page-content">
          <div hidden={page !== "overview"}>{page==="overview"&&<OverviewView model={model} open={open}/>}</div>
          <div hidden={page !== "maintenance"}>{page==="maintenance"&&<MaintenanceView key={device.id+":"+(model.device?.current_session?.session_id??"")+":"+(model.active?.maintenance_id??"")} model={model} request={shellRequest} open={open}/>}</div><div hidden={page!=="details"}>{page==="details"&&<DetailsPage model={model}/>}</div>
          <footer className="workspace-footer"><ShieldCheck size={13}/><span>每一次连接，都让维护更简单</span><span className="footer-demo">{state.synchronized?"数据来自当前服务器":"等待服务器连接"}</span></footer>
    </div>
      </main>
      <div className="tasks-page-host" hidden={page !== "tasks"}>{page==="tasks"&&<TasksView model={model}/>}</div>
      <div className="repo-page-host" hidden={!["files", "tools"].includes(page)}>{["files","tools"].includes(page)&&<RepositoryView model={model}/>}</div>
      {page==="settings"&&<main className="production-settings"><SettingsPage model={model}/></main>}
    </div>

    {(notice||error||state.error)&&<div className={`toast ${(error||state.error)?'production-error':''}`} role={(error||state.error)?'alert':'status'}><Check size={18}/>{error||state.error||notice}<button className="icon" aria-label="关闭提示" onClick={()=>{setError('');setNotice('');}}><X size={15}/></button></div>}
    <div className="production-pending">            {state.pending && !state.busy && (
              <div className="alert warning">
                <span>
                  “{state.pending.label}”响应不确定。确认 Server
                  没有重启后才能重试原请求。
                </span>
                <button
                  disabled={busy}
                  onClick={() =>
                    form(
                      "确认原请求重试",
                      [field("confirmed", "已确认服务器未重启（输入 是）")],
                      async (v) => {
                        if (v.confirmed !== "是")
                          throw Error("需明确确认 Server 未重启");
                        await owner.current!.execute(state.pending!, true);
                      },
                    )
                  }
                >
                  核对并重试
                </button>
                <button
                  onClick={() =>
                    form(
                      "放弃原请求",
                      [field("confirmed", "已核对服务器结果（输入 是）")],
                      async (v) => {
                        if (v.confirmed !== "是") throw Error("请先核对");
                        owner.current?.abandon();
                      },
                    )
                  }
                >
                  核对后放弃
                </button>
              </div>
            )}
</div>
    {dialog&&<FormDialog key={dialog.title} error={error} title={dialog.title} fields={dialog.fields} busy={busy} onClose={()=>setDialog(null)} onSubmit={v=>void run(dialog.title,async()=>{await dialog.run(v);setDialog(null);})}/>}
    {inspect&&<div className="modal-backdrop"><section className="modal" role="dialog" aria-modal="true" aria-label={inspect.title}><div className="card-heading"><h2>{inspect.title}</h2><button aria-label="关闭详情" onClick={()=>setInspect(null)}><X/></button></div><pre>{typeof inspect.value==='string'?inspect.value:JSON.stringify(inspect.value,null,2)}</pre></section></div>}
  </div>;
}
