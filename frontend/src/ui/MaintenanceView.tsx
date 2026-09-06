import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { ArrowDownToLine, ArrowLeft, ArrowUpFromLine, ChevronDown, ChevronLeft, ChevronRight, Clock3, Copy, EllipsisVertical, File, FileCode2, Folder, FolderOpen, Globe, HardDrive, History, LocateFixed, Maximize2, Minimize2, Monitor, PlugZap, Plus, Power, RefreshCw, Search, Settings, ShieldCheck, Terminal, Trash2, UploadCloud, X } from "lucide-react";
import "./maintenance.css";
import { type Workbench } from "../useWorkbench";
import { mutation, importMutation, describe } from "../api";
import { EmbeddedShell, type ShellControl } from "./EmbeddedShell";
import { directoryCommand, parseDirectory, waitTask, type RemoteEntry } from "./remoteDirectory";
export type ShellProtocol='SSH'|'Telnet';
const resolvePath=(cwd:string,path:string)=>{const parts:string[]=[];for(const p of (path.startsWith('/')?path:cwd+'/'+path).split('/')){if(p==='..')parts.pop();else if(p&&p!=='.')parts.push(p);}return '/'+parts.join('/');};
const formatSize=(entry:RemoteEntry)=>entry.folder?'—':entry.size<1024?`${entry.size} B`:`${(entry.size/1024).toFixed(1)} KB`;
export function MaintenanceView({model,request,open}:{model:Workbench;request:{protocol:ShellProtocol;id:number}|null;open:(title:string,kind?:string)=>void}) {
  const device={id:model.selected,host:model.device?.registration.hostname||model.selected,online:model.device?.status==='online'};
  const active=!!model.active&&!model.active.released&&model.active.state==='ready'&&model.seconds>0&&model.active.session_id===model.device?.current_session?.session_id;
  const enabled=device.online&&active&&model.state.synchronized;
  const remainingSeconds=model.seconds,notify=model.setNotice;
  const [sessions,setSessions]=useState<Partial<Record<ShellProtocol,{id:number;connected:boolean}>>>({});
  const [current,setCurrent]=useState<ShellProtocol>('SSH');
  const [expanded,setExpanded]=useState(false),[fontSize,setFontSize]=useState(13);
  const [directory,setDirectory]=useState('/etc/config'),[pathInput,setPathInput]=useState('/etc/config');
  const [fs,setFs]=useState<Record<string,RemoteEntry[]>>({});
  const [selected,setSelected]=useState(''),[checked,setChecked]=useState<string[]>([]);
  const [filePage,setFilePage]=useState(0),[fileQuery,setFileQuery]=useState('');
  const [menu,setMenu]=useState<'session'|'terminal'|'files'|null>(null),[dragging,setDragging]=useState(false);
  const [preview,setPreview]=useState<{name:string;text:string}|null>(null);
  const [directoryMessage,setDirectoryMessage]=useState('点击刷新读取设备目录');
  const uploadInput=useRef<HTMLInputElement>(null),previewDialog=useRef<HTMLDialogElement>(null);
  const shellRefs=useRef<Partial<Record<ShellProtocol,ShellControl|null>>>({}),sessionCounter=useRef(0);
  const lifecycle=useRef(new AbortController());
  useEffect(()=>()=>lifecycle.current.abort(),[]);
  const session=sessions[current],connected=enabled&&!!session?.connected;
  const entries=(fs[directory]??[]).filter(e=>e.name.toLowerCase().includes(fileQuery.toLowerCase()));
  const selection=fs[directory]?.find(e=>e.name===selected);
  const downloadEntries=checked.length?(fs[directory]??[]).filter(e=>checked.includes(e.name)&&!e.folder):selection&&!selection.folder?[selection]:[];
  const pageCount=Math.max(1,Math.ceil(entries.length/8)),visiblePage=Math.min(filePage,pageCount-1),pageEntries=entries.slice(visiblePage*8,visiblePage*8+8);
  const sessionCount=Object.keys(sessions).length;
  const connect=(protocol:ShellProtocol,restart=false)=>{if(!enabled)return;setSessions(prev=>({...prev,[protocol]:!restart&&prev[protocol]?prev[protocol]:{id:++sessionCounter.current,connected:false}}));setCurrent(protocol);setMenu(null);};
  const closeSession=(protocol:ShellProtocol)=>setSessions(prev=>{const next={...prev};delete next[protocol];return next;});
  useEffect(()=>{if(request)connect(request.protocol);},[request]);
  useEffect(()=>{if(!enabled)setSessions({});},[enabled]);
  useEffect(()=>{if(preview)previewDialog.current?.showModal();else previewDialog.current?.close();},[preview]);
  const insert=(text:string)=>shellRefs.current[current]?.insert(text);
  const goTo=(path:string)=>{
    if(!model.enabled)return;
    const normalized=resolvePath(directory,path),c=model.owner.current;
    if(!c)return;
    setDirectory(normalized);setPathInput(normalized);setFileQuery('');setSelected('');setChecked([]);setFilePage(0);setDirectoryMessage('正在读取设备目录…');
    void model.run('读取目录',async()=>{
      try{
        const result=await model.write(mutation('读取目录','tasks',{device_id:device.id,command:directoryCommand(normalized),timeout_seconds:15}));
        const task=await c.track(()=>waitTask(c,result.task_id,lifecycle.current.signal));
        if(lifecycle.current.signal.aborted||c!==model.owner.current)return;
        if(task.state!=='success'||!task.result||task.result.exit_code!==0)throw Error(task.result?.stderr||`目录读取未成功：${task.state}`);
        if(task.result.truncated)throw Error('目录结果已截断，请在终端中查看，未显示不完整列表');
        const value=parseDirectory(task.result.stdout);setFs(prev=>({...prev,[normalized]:value.entries}));setDirectoryMessage(value.limited?'仅显示前 250 项，可在终端中查看完整目录':'这个目录暂时为空');
      }catch(e){if(!lifecycle.current.signal.aborted)setDirectoryMessage(describe(e));throw e;}
    });
  };
  const uploadFiles=(files:FileList|File[])=>{
    if(!model.enabled)return;
    void model.run('上传文件',async()=>{for(const file of Array.from(files)){
      lifecycle.current.signal.throwIfAborted();
      const asset=await model.write(await importMutation(file));
      lifecycle.current.signal.throwIfAborted();
      await model.write(mutation('上传文件','uploads',{device_id:device.id,asset_id:asset.asset_id,remote_path:resolvePath(directory,file.name),mode:'0644',overwrite:false,timeout_seconds:120}));
    }notify('已创建上传任务，请在任务中心查看结果后刷新目录');});
  };
  const downloadFile=()=>void model.run('创建下载',async()=>{for(const entry of downloadEntries){lifecycle.current.signal.throwIfAborted();await model.write(mutation('下载文件','downloads',{device_id:device.id,remote_path:resolvePath(directory,entry.name),name:entry.name,timeout_seconds:120}));}model.setPage('tasks');});
  const inspectFile=(entry:RemoteEntry)=>{if(entry.folder)goTo(entry.name);else setPreview({name:entry.name,text:`路径：${resolvePath(directory,entry.name)}\n大小：${formatSize(entry)}\n修改时间：${entry.modified}\n\n选择下载后，在任务中心导入已提交文件，再从文件仓库保存到本地。`});};


  const duration = `${String(Math.floor(remainingSeconds / 3600)).padStart(2, "0")}:${String(Math.floor(remainingSeconds / 60) % 60).padStart(2, "0")}:${String(remainingSeconds % 60).padStart(2, "0")}`;

  return <section className={`remote-maintenance ${expanded ? "terminal-expanded" : ""}`}>
    <div className="maintenance-toolbar">
      <div className="maintenance-heading"><span><ShieldCheck size={25}/></span><div><h2>远程维护</h2><p>{connected ? "本地终端会话已打开，在同一工作区完成运维操作" : "连接设备终端，在同一工作区完成运维操作"}</p></div></div>
      <div className="maintenance-lease"><span className={`overview-status ${active ? "" : "offline"}`}>{active ? "维护已开启" : "未开启"}</span><span><Clock3 size={17}/>剩余 <b>{active ? duration : "—"}</b></span><button className="text-button" disabled={!device.online} onClick={() => open(active ? "关闭远程维护" : "开启远程维护", "maintenance")}>{active ? "关闭维护" : "开启维护"}</button></div>
    </div>
    <div className="remote-workspace">
      <section className="shell-panel" aria-label="内置 Shell">
        <div className="shell-connectionbar">
          <div className="protocol-buttons" role="tablist" aria-label="终端会话">
            <button role="tab" aria-selected={!!session && current === "SSH"} className={session && current === "SSH" ? "primary" : ""} disabled={!enabled} onClick={() => connect("SSH")}><Terminal size={17}/>SSH<span className="protocol-description">安全终端</span></button>
            <button role="tab" aria-selected={!!session && current === "Telnet"} className={session && current === "Telnet" ? "primary" : ""} disabled={!enabled} onClick={() => connect("Telnet")}><Monitor size={17}/>Telnet<span className="protocol-description">远程终端</span></button>
          </div>
          <button className="shell-web" disabled={!enabled} onClick={() => open("Web 管理页面", "endpoint")}><Globe size={18}/>Web 管理<ChevronDown size={13}/></button>
          <div className="shell-session-menu"><button className="new-shell-session" disabled={!enabled} aria-expanded={menu === "session"} onClick={() => setMenu(menu === "session" ? null : "session")}><Plus size={16}/><span>新建会话</span><ChevronDown size={13}/></button>{menu === "session" && <div className="maintenance-menu dark-menu">{(["SSH","Telnet"] as const).map(protocol => <button key={protocol} onClick={() => connect(protocol,true)}><Terminal size={14}/>{sessions[protocol] ? `重建 ${protocol} 会话` : `${protocol} 会话`}</button>)}{(["SSH","Telnet"] as const).map(protocol=><button key={"external"+protocol} onClick={()=>{setMenu(null);if(model.active)void model.run(`打开外部 ${protocol}`,()=>model.open(model.active!,protocol.toLowerCase()));}}>外部 {protocol} 工具</button>)}</div>}</div>
        </div>
        <div className="shell-context">
          <span className={connected ? "connected-label" : ""}><i className={connected ? "connected" : ""}/>{connected ? "会话已打开" : session ? "已断开" : "等待连接"}</span><span className="shell-host">{model.profile.ssh_user}@{device.host}</span><span className="shell-encoding">{current} / UTF-8</span>
          <div className="shell-tools"><button aria-label="减小终端字号" onClick={() => setFontSize(Math.max(11,fontSize-1))}>A−</button><button aria-label="增大终端字号" onClick={() => setFontSize(Math.min(18,fontSize+1))}>A+</button><button aria-label="清空终端" disabled={!session} onClick={() => shellRefs.current[current]?.clear()}><Trash2 size={16}/></button><div className="terminal-menu-anchor"><button aria-label="终端设置" aria-expanded={menu === "terminal"} onClick={() => setMenu(menu === "terminal" ? null : "terminal")}><Settings size={16}/></button>{menu === "terminal" && <div className="maintenance-menu dark-menu"><button disabled={!session} onClick={() => {setMenu(null);void shellRefs.current[current]?.copy().then(()=>notify('终端输出已复制'),()=>notify('复制未成功'));}}><Copy size={14}/>复制终端输出</button><button disabled={!enabled || !session} onClick={() => {setMenu(null);if(session)closeSession(current);else connect(current);}}><Power size={14}/>{connected ? "断开连接" : "重新连接"}</button><button disabled={!session} onClick={() => {setMenu(null);closeSession(current);}}><X size={14}/>关闭当前会话</button></div>}</div><button aria-label={expanded ? "恢复终端布局" : "展开终端"} onClick={() => setExpanded(!expanded)}>{expanded ? <Minimize2 size={16}/> : <Maximize2 size={16}/>}</button></div>
        </div>
        <div className="terminal-output production-terminal-output" hidden={!session}>
          {(['SSH','Telnet'] as const).map(protocol=>sessions[protocol]&&<div key={protocol+sessions[protocol]!.id} className="production-terminal-slot" hidden={current!==protocol}><EmbeddedShell ref={value=>{shellRefs.current[protocol]=value;}} model={model} protocol={protocol} fontSize={fontSize} visible={current===protocol} onState={opened=>setSessions(prev=>prev[protocol]?{...prev,[protocol]:{...prev[protocol]!,connected:opened}}:prev)}/></div>)}
        </div>{session ? null : <div className="shell-welcome"><span className="welcome-terminal-icon"><Terminal size={43}/></span><h3>从这里连接你的设备</h3><p>选择 SSH 或 Telnet，在内置终端中开始维护</p><div><button disabled={!enabled} onClick={()=>connect("SSH")}><Terminal size={17}/>连接 SSH<ChevronRight size={14}/></button><button disabled={!enabled} onClick={()=>connect("Telnet")}><Monitor size={17}/>连接 Telnet<ChevronRight size={14}/></button></div><small>{enabled ? "默认内置 Shell，也可从新建会话菜单调用外部工具" : "请先开启维护，或等待设备上线"}</small></div>}
        <div className="shell-bottom"><div className="shell-shortcuts">{["uptime","ip addr","df -h","free -m"].map(command=><button key={command} disabled={!connected} onClick={()=>{insert(command);}}>{command}</button>)}</div><label className="command-picker"><select aria-label="常用命令" disabled={!connected} value="" onChange={e=>{insert(e.target.value);}}><option value="" disabled>常用命令</option>{["help","pwd","ls -l","uname -a","whoami"].map(command=><option key={command}>{command}</option>)}</select><ChevronDown size={13}/></label><div className="shell-statusbar"><span className="cursor-position">Windows ConPTY</span><span>UTF-8</span><span className={connected ? "connected-label" : ""}><i className={connected ? "connected" : ""}/>{connected ? `${current} 会话已打开` : "未连接"}</span><span className="session-count">{sessionCount} 个会话</span></div></div>
      </section>
      <aside className={`remote-files ${dragging ? "is-dragging" : ""}`} aria-label="远程文件管理" onDragOver={e=>{e.preventDefault();if(enabled)setDragging(true);}} onDragLeave={e=>{if(!e.currentTarget.contains(e.relatedTarget as Node))setDragging(false);}} onDrop={e=>{e.preventDefault();setDragging(false);uploadFiles(e.dataTransfer.files);}}>
        <div className="file-panel-heading"><h2><Folder size={19}/>文件管理</h2><div className="file-menu-anchor"><button className="icon" aria-label="文件管理工具" aria-expanded={menu === "files"} onClick={()=>setMenu(menu === "files" ? null : "files")}><Settings size={18}/></button>{menu === "files" && <div className="maintenance-menu"><button onClick={()=>{setMenu(null);goTo('/');}}><LocateFixed size={14}/>打开设备根目录</button><button onClick={()=>{setMenu(null);model.setPage('tasks');}}><History size={14}/>传输记录</button></div>}</div></div>
        <div className="file-actionbar"><button className="primary" disabled={!model.enabled} onClick={()=>uploadInput.current?.click()}><ArrowUpFromLine size={16}/>上传文件</button><button disabled={!downloadEntries.length || !model.enabled} onClick={downloadFile}><ArrowDownToLine size={16}/>下载{checked.length>1 ? ` (${downloadEntries.length})` : ""}</button><button className="icon" aria-label="刷新远程文件列表" disabled={!model.enabled} onClick={()=>goTo(directory)}><RefreshCw size={16}/></button><input ref={uploadInput} aria-label="选择上传文件" type="file" multiple hidden onChange={e=>{if(e.target.files)uploadFiles(e.target.files);e.target.value="";}}/></div>
        <form className="file-path" onSubmit={e=>{e.preventDefault();goTo(pathInput);}}><button type="button" aria-label="返回上级目录" disabled={directory === "/"} onClick={()=>goTo("..")}><ArrowLeft size={16}/></button><Folder size={15}/><input aria-label="当前文件目录" value={pathInput} onChange={e=>setPathInput(e.target.value)}/><button type="button" aria-label="复制当前目录" onClick={()=>{void navigator.clipboard.writeText(directory).then(()=>notify("目录路径已复制"),()=>notify("复制未成功"));}}><Copy size={14}/></button></form>
        <div className="directory-shortcuts">{["/","/etc/config","/tmp","/var/log"].map(path=><button key={path} className={directory === path ? "selected" : ""} onClick={()=>goTo(path)}>{path === "/" ? "全部" : path}</button>)}</div>
        <label className="file-search"><Search size={15}/><input aria-label="搜索当前目录" placeholder="搜索当前目录..." value={fileQuery} onChange={e=>{setFileQuery(e.target.value);setFilePage(0);}}/></label>
        <div className="remote-file-list"><div className="file-columns"><input type="checkbox" aria-label="选择本页全部文件" checked={!!pageEntries.length && pageEntries.every(e=>checked.includes(e.name))} onChange={e=>setChecked(e.target.checked ? [...new Set([...checked,...pageEntries.map(entry=>entry.name)])] : checked.filter(name=>!pageEntries.some(entry=>entry.name===name)))}/><span>名称</span><span>大小</span><span>修改时间</span><span/></div>
          {pageEntries.map(entry=><div key={entry.name} className={`remote-file-row ${entry.name===selected ? "selected" : ""}`}><input type="checkbox" aria-label={`选择 ${entry.name}`} checked={checked.includes(entry.name)} onChange={e=>{setSelected(entry.name);setChecked(e.target.checked ? [...checked,entry.name] : checked.filter(name=>name!==entry.name));}}/><button className={`file-name ${entry.folder ? "folder" : ""}`} onClick={()=>setSelected(entry.name)} onDoubleClick={()=>inspectFile(entry)} onKeyDown={e=>{if(e.key==="Enter"){e.preventDefault();inspectFile(entry);}}} title={entry.folder ? `双击打开 ${entry.name}` : `双击预览 ${entry.name}`}>{entry.folder ? <Folder size={16}/> : <FileCode2 size={16}/>}<span>{entry.name}</span></button><span>{formatSize(entry)}</span><time>{entry.modified}</time><button className="file-row-more" aria-label={`${entry.folder ? "打开" : "预览"} ${entry.name}`} onClick={()=>inspectFile(entry)}><EllipsisVertical size={16}/></button></div>)}
          {!entries.length && <div className="files-empty"><FolderOpen size={29}/><p>{fileQuery ? "没有匹配的文件" : directoryMessage}</p></div>}
        </div>
        <div className="file-selection-info"><span>共 {entries.length} 个项目{checked.length ? ` · 已选 ${checked.length}` : ""}</span><div className="file-pagination"><button aria-label="文件上一页" disabled={visiblePage === 0} onClick={()=>setFilePage(visiblePage-1)}><ChevronLeft size={13}/></button><span>{visiblePage+1}</span><button aria-label="文件下一页" disabled={visiblePage+1 >= pageCount} onClick={()=>setFilePage(visiblePage+1)}><ChevronRight size={13}/></button></div></div>
        <button className="file-dropzone" disabled={!model.enabled} onClick={()=>uploadInput.current?.click()}><UploadCloud size={29}/><span>拖拽文件到这里上传<small>也可以点击选择文件</small></span></button>
        <div className="file-storage"><HardDrive size={14}/><span>设备存储</span><i><b style={{width:0}}/></i><span>—</span><span>API 未提供设备容量</span></div>
        {dragging && <div className="drop-overlay"><UploadCloud size={38}/><strong>松开以导入并创建上传任务</strong><span>{directory}</span></div>}
      </aside>
    </div>
    <dialog ref={previewDialog} className="remote-file-preview" onCancel={()=>setPreview(null)}><div className="dialog-header"><h2><File size={17}/>{preview?.name}</h2><button className="icon" aria-label="关闭文件预览" onClick={()=>setPreview(null)}><X size={18}/></button></div><pre>{preview?.text}</pre></dialog>
  </section>;
}