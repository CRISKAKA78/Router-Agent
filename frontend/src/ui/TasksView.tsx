import { useEffect, useRef, useState } from "react";
import { Activity, ArrowDownToLine, ArrowRight, ArrowUpFromLine, Check, CheckCheck, ChevronDown, ChevronLeft, ChevronRight, CircleAlert, CircleCheck, Clock3, Copy, FileText, Filter, Inbox, ListTodo, LoaderCircle, Package, Plus, RefreshCw, Search, Send, Settings2, ShieldCheck, Terminal, X } from "lucide-react";
import "./tasks.css";
import { mutation, segment } from "../api";
import { type TaskDetail, type TaskSummary } from "../models";
import { useQuery } from "../useQuery";
import type { Connection } from "../connection";
import {mapBounded,type Operation} from "./repositoryData";
import { type Workbench, stamp } from "../useWorkbench";
function asTask(t: TaskSummary, detail?: TaskDetail | null, operation?:Operation): ViewTask {
 const d=detail?.task_id===t.task_id?detail:null;
 return {id:t.task_id,device:t.device_id,kind:(operation?.tool_id?'deployment':t.type==='exec'?'exec':t.type==='download'?'download':'upload'),asset:operation?.tool_id?`${operation.tool_id} · ${operation.version}`:operation?.asset_id,imported:t.type==='download'&&!!operation?.asset_id,state:t.state as TaskState,title:d?.command || t.task_id,command:d?.command || String(d?.params?.remote_path ?? ''),created:stamp(t.created_at),timeout:d?.timeout_seconds??0,cwd:d?.cwd??'',env:d?.env??{},dispatches:d?.dispatch_count??0,session:d?.last_session_id,result:d?.result?{stdout:d.result.stdout,stderr:d.result.stderr,exitCode:d.result.exit_code,started:stamp(d.result.started_at),finished:stamp(d.result.finished_at),truncated:d.result.truncated}:undefined};
}

type Device = { id: string; name: string; online: boolean };
type TaskState = "received" | "queued" | "running" | "success" | "failed" | "timeout" | "rejected";
type TaskKind = "exec" | "upload" | "download" | "deployment";
type ViewTask = { session?: string | null; id: string; device: string; kind: TaskKind; state: TaskState; title: string; command: string; created: string; timeout: number; cwd: string; env: Record<string,string>; dispatches: number; result?: { stdout: string; stderr: string; exitCode: number; started: string; finished: string; truncated: boolean }; transfer?: { committed: boolean; released: boolean; failed: boolean; size: string }; asset?: string; imported?: boolean };
const states: Record<TaskState,{label:string;tone:string}> = { received:{label:"待确认",tone:"neutral"},queued:{label:"排队中",tone:"amber"},running:{label:"执行中",tone:"blue"},success:{label:"成功",tone:"green"},failed:{label:"失败",tone:"red"},timeout:{label:"已超时",tone:"red"},rejected:{label:"已拒绝",tone:"red"} };
const kinds = {exec:{label:"命令执行",icon:Terminal},upload:{label:"文件上传",icon:ArrowUpFromLine},download:{label:"文件下载",icon:ArrowDownToLine},deployment:{label:"工具投放",icon:Package}};
const pending = (t: ViewTask) => ["received","queued","running"].includes(t.state);
function TaskBadge({state}:{state:TaskState}) { const s=states[state]; return <span className={`task-badge ${s.tone}`}>{state === "running" ? <LoaderCircle size={12}/> : <i/>}{s.label}</span>; }
export function TasksView({model}:{model:Workbench}) {
  const devices=model.state.snapshot.devices.map(d=>({id:d.device_id,name:d.registration.hostname||d.device_id,online:d.status==='online'}));
  const notify=model.setNotice;
  const operationCache=useRef<{connection:Connection|null;values:Map<string,Operation>}>({connection:null,values:new Map()});
  const operationQuery=useQuery(model.owner.current,model.task?.task_id||'operations',model.state.snapshot.fetchedAt,async c=>{
    if(operationCache.current.connection!==c)operationCache.current={connection:c,values:new Map()};
    const cache=operationCache.current;
    const fileTasks=model.state.snapshot.tasks.filter(t=>t.type!=='exec');
    const present=new Set(fileTasks.map(t=>t.task_id));
    for(const id of cache.values.keys())if(!present.has(id))cache.values.delete(id);
    const values=await mapBounded(fileTasks.filter(t=>!cache.values.has(t.task_id)||t.task_id===model.task?.task_id),t=>c.api.get<Operation>('tasks/'+segment(t.task_id)+'/operation'));
    for(const value of values)cache.values.set(value.task_id,value);
    return {connection:c,values:new Map(cache.values)};
  },model.setError);
  const operations=operationQuery?.connection===model.owner.current?operationQuery.values:operationCache.current.connection===model.owner.current?operationCache.current.values:new Map<string,Operation>();
  const tasks=model.state.snapshot.tasks.map(t=>asTask(t,model.task,operations.get(t.task_id)));
  const selected=model.task?.task_id??'';
  const setSelected=(id:string)=>{if(model.task?.task_id===id){model.owner.current?.invalidate();return;}const t=model.state.snapshot.tasks.find(t=>t.task_id===id);if(t)model.setTask({...t,command:'',cwd:'',timeout_seconds:0,env:{},params:{},last_session_id:null,dispatch_count:0,result:null});};
  const [query,setQuery]=useState("");
  const [kind,setKind]=useState("all");
  const [status,setStatus]=useState("all");
  const [deviceFilter,setDeviceFilter]=useState("all");
  const [page,setPage]=useState(0);
  const [detailTab,setDetailTab]=useState("result");
  const [outputTab,setOutputTab]=useState("stdout");
  const [creating,setCreating]=useState(false);
  const [formDevice,setFormDevice]=useState(model.selected);
  const [command,setCommand]=useState("");
  const [timeout,setTimeoutValue]=useState("30");
  const [cwd,setCwd]=useState("/root");
  const [env,setEnv]=useState("{}");
  const [formError,setFormError]=useState("");
  const [advanced,setAdvanced]=useState(false);
  const dialogRef=useRef<HTMLDialogElement>(null);

  const visible=tasks.filter(t=>(kind === "all" || t.kind === kind) && (deviceFilter === "all" || t.device === deviceFilter) && (status === "all" || (status === "active" ? pending(t) : status === "attention" ? ["failed","timeout","rejected"].includes(t.state) : t.state === status)) && `${t.id} ${t.title} ${t.command} ${devices.find(d=>d.id===t.device)?.name}`.toLowerCase().includes(query.toLowerCase()));
  const currentPage=Math.min(page,Math.max(0,Math.ceil(visible.length/8)-1));
  const task=tasks.find(t=>t.id===selected) ?? null;
  if(task && model.transfer?.task_id===task.id) task.transfer={...model.transfer,size:`${model.transfer.size} B`};
  const taskDevice=devices.find(d=>d.id===task?.device)??{id:task?.device??'',name:task?.device??'—',online:false};
  const Glyph=kinds[task?.kind??'exec'].icon;
  const pageDetails=useQuery(model.owner.current,JSON.stringify(visible.slice(currentPage*8,currentPage*8+8).map(t=>t.id)),model.state.snapshot.fetchedAt,
    c=>Promise.all(visible.slice(currentPage*8,currentPage*8+8).map(t=>c.api.get<TaskDetail>('tasks/'+segment(t.id)))),model.setError);
  const refresh=()=>model.owner.current?.invalidate();
  useEffect(()=>{if(creating)dialogRef.current?.showModal();else dialogRef.current?.close();},[creating]);
  const choose=(t:ViewTask)=>{setSelected(t.id);setDetailTab("result");setOutputTab(t.state === "failed" || t.state === "timeout" ? "stderr" : "stdout");};
  const copy=(text:string)=>void navigator.clipboard.writeText(text).then(()=>notify("已复制"),()=>notify("复制未成功"));
  const resetFilters=()=>{setQuery("");setStatus("all");setKind("all");setDeviceFilter("all");setPage(0);};
  const createTask=()=>{
    if(!command.trim() || !/^\d+$/.test(timeout) || Number(timeout)<1 || Number(timeout)>4294967295 || !devices.find(d=>d.id===formDevice)?.online){setFormError("请选择在线设备，填写命令和有效的正整数超时秒数。");return;}
    let parsed:Record<string,string>;
    try {parsed=JSON.parse(env);if(!parsed || Array.isArray(parsed) || typeof parsed !== "object" || Object.values(parsed).some(value=>typeof value!=="string"))throw new Error();} catch {setFormError("环境变量应为 JSON 对象，键和值均使用字符串。");return;}
    void model.run('创建任务',async()=>{
      const result=await model.write(mutation('创建任务','tasks',{device_id:formDevice,command:command.trim(),timeout_seconds:Number(timeout),...(cwd?{cwd}:{}),env:parsed}));
      const c=model.owner.current;if(c){const detail=await c.track(()=>c.api.get<TaskDetail>('tasks/'+segment(result.task_id)));if(c===model.owner.current)model.setTask(detail);}
      resetFilters();setCreating(false);
    });
  };
  return <main className="task-center">
    <div className="task-breadcrumb">工作空间<ChevronRight size={13}/><span>任务中心</span></div>
    <header className="task-page-heading"><div><div className="task-title"><span><ListTodo size={27}/></span><h1>任务中心</h1><span className="task-scope-tag">本次服务运行</span></div><p>集中查看设备任务，跟踪执行状态与结果</p></div><div className="task-heading-actions"><button onClick={refresh}><RefreshCw size={15}/>刷新列表</button><button className="primary" onClick={()=>{setFormError("");setFormDevice(model.selected||devices.find(d=>d.online)?.id||"");setCreating(true);}}><Plus size={17}/>新建命令任务</button></div></header>
    <div className="task-stats">{[
      {label:"全部任务",value:tasks.length,caption:"当前列表统计",icon:ListTodo,tone:"blue",filter:"all"},
      {label:"进行中",value:tasks.filter(pending).length,caption:`${tasks.filter(t=>t.state==="running").length} 执行 · ${tasks.filter(t=>t.state==="queued").length} 排队 · ${tasks.filter(t=>t.state==="received").length} 待确认`,icon:Activity,tone:"blue",filter:"active"},
      {label:"执行成功",value:tasks.filter(t=>t.state==="success").length,caption:"任务状态为成功",icon:CircleCheck,tone:"green",filter:"success"},
      {label:"需要关注",value:tasks.filter(t=>["failed","timeout","rejected"].includes(t.state)).length,caption:"失败、超时或被拒绝",icon:CircleAlert,tone:"amber",filter:"attention"},
    ].map(stat=><button className={`task-stat ${stat.tone} ${status===stat.filter ? "active" : ""}`} key={stat.label} onClick={()=>{setStatus(stat.filter);setPage(0);}}><span className="task-stat-icon"><stat.icon size={23}/></span><div><span>{stat.label}</span><strong>{stat.value}<small>项</small></strong><p>{stat.caption}</p></div><ArrowRight size={15}/></button>)}</div>
    <div className="task-workspace">
      <section className="task-list-panel" aria-label="任务列表">
        <div className="task-list-heading"><h2>任务列表<span>{visible.length}</span></h2><span><i/>{model.state.synchronized?"快照已同步":"等待同步"}</span></div>
        <div className="task-kind-tabs" role="tablist" aria-label="任务类型">{[["all","全部任务"],...Object.entries(kinds).map(([key,value])=>[key,value.label])].map(([key,label])=><button role="tab" aria-selected={kind===key} className={kind===key ? "selected" : ""} key={key} onClick={()=>{setKind(key);setPage(0);}}>{label}<span>{key==="all" ? tasks.length : tasks.filter(t=>t.kind===key).length}</span></button>)}</div>
        <div className="task-filterbar"><label className="task-search"><Search size={16}/><input aria-label="搜索任务" placeholder="搜索任务 ID 或设备…" value={query} onChange={e=>{setQuery(e.target.value);setPage(0);}}/></label><label className="task-select"><select aria-label="筛选任务设备" value={deviceFilter} onChange={e=>{setDeviceFilter(e.target.value);setPage(0);}}><option value="all">全部设备</option>{devices.map(d=><option key={d.id} value={d.id}>{d.name}</option>)}</select><ChevronDown size={13}/></label><label className="task-select"><Filter size={13}/><select aria-label="筛选任务状态" value={status} onChange={e=>{setStatus(e.target.value);setPage(0);}}><option value="all">全部状态</option><option value="active">进行中</option><option value="attention">需要关注</option>{Object.entries(states).map(([key,s])=><option key={key} value={key}>{s.label}</option>)}</select><ChevronDown size={13}/></label></div>
        <div className="task-table-scroll"><table className="task-table"><thead><tr><th>任务 / 执行内容</th><th>目标设备</th><th>状态</th><th>创建时间</th><th/></tr></thead><tbody>{visible.slice(currentPage*8,currentPage*8+8).map(summary=>{const raw=pageDetails?.find(d=>d.task_id===summary.id);const t=raw?asTask(raw,raw,operations.get(raw.task_id)):summary;const Icon=kinds[t.kind].icon;const d=devices.find(d=>d.id===t.device)??{id:t.device,name:t.device,online:false};return <tr key={t.id} className={selected===t.id ? "selected" : ""} onClick={()=>choose(t)}><td><button className="task-row-title" onClick={()=>choose(t)}><span className={`task-type-icon ${t.kind}`}><Icon size={19}/></span><span><strong>{t.title}</strong><small>{t.kind==="exec" ? t.command : `${kinds[t.kind].label} · ${t.command}`}</small></span></button></td><td><span className="task-device-name"><i className={d.online ? "online" : ""}/>{d.name}</span><small>{d.id}</small></td><td><TaskBadge state={t.state}/></td><td><span>{t.created}</span></td><td><button className="icon" aria-label={`查看任务 ${t.id}`} onClick={()=>choose(t)}><ChevronRight size={15}/></button></td></tr>;})}</tbody></table>{!visible.length && <div className="task-empty"><Search size={32}/><h3>没有找到匹配任务</h3><p>调整搜索或筛选条件后再试</p><button onClick={resetFilters}>清除筛选</button></div>}</div>
        <div className="task-list-footer"><span>共 {visible.length} 项<span>·</span>每页 8 项</span><div><button className="icon" disabled={currentPage===0} aria-label="任务上一页" onClick={()=>setPage(currentPage-1)}><ChevronLeft size={15}/></button><b>{currentPage+1}</b><span>/ {Math.max(1,Math.ceil(visible.length/8))}</span><button className="icon" disabled={(currentPage+1)*8>=visible.length} aria-label="任务下一页" onClick={()=>setPage(currentPage+1)}><ChevronRight size={15}/></button></div></div>
      </section>
      {task ? <aside className="task-detail-panel" aria-label="任务详情">
        <div className="task-detail-heading"><h2><FileText size={17}/>任务详情</h2><button className="icon" aria-label="刷新任务详情" onClick={refresh}><RefreshCw size={15}/></button></div>
        <div className="task-detail-summary"><span className={`task-type-icon ${task.kind}`}><Glyph size={22}/></span><div><h3>{task.title}</h3><span>{kinds[task.kind].label}<b>·</b>{task.created} 创建</span></div><TaskBadge state={task.state}/></div>
        <div className="task-id-line"><span>{task.id}</span><button className="icon" aria-label="复制任务编号" onClick={()=>copy(task.id)}><Copy size={12}/></button></div>
        <div className="task-detail-tabs" role="tablist" aria-label="任务详情内容">{[["result","执行结果"],["params","任务参数"]].map(([key,label])=><button role="tab" aria-selected={detailTab===key} className={detailTab===key ? "selected" : ""} key={key} onClick={()=>setDetailTab(key)}>{label}</button>)}</div>
        <div className="task-detail-body">
          {detailTab==="result" ? <><div className="task-status-steps"><span className="done"><Check size={12}/>已创建</span><i/><span className={task.state!=="received" ? "done" : ""}>{task.state!=="received" ? <Check size={12}/> : <Clock3 size={12}/>}设备反馈</span><i/><span className={task.result ? "done" : ""}>{task.result ? <Check size={12}/> : <Clock3 size={12}/>}最终结果</span></div>
          <dl className="task-facts"><div><dt>目标设备</dt><dd>{taskDevice.name}</dd></div><div><dt>超时设置</dt><dd>{task.timeout||"—"} 秒</dd></div><div><dt>开始时间</dt><dd>{task.result?.started ?? "尚无最终结果"}</dd></div><div><dt>结束时间</dt><dd>{task.result?.finished ?? "—"}</dd></div></dl>
          {task.kind==="exec" && <div className="task-command"><span><Terminal size={13}/>执行命令</span><code>{task.command}</code></div>}
          {task.result ? task.kind==="exec" ? <div className="task-result-box"><div className="task-output-tabs"><button className={outputTab==="stdout" ? "selected" : ""} onClick={()=>setOutputTab("stdout")}>标准输出</button><button className={outputTab==="stderr" ? "selected" : ""} onClick={()=>setOutputTab("stderr")}>错误输出{task.result.stderr && <i/>}</button><button aria-label="复制任务输出" onClick={()=>copy(task.result![outputTab as "stdout"|"stderr"])}><Copy size={13}/></button></div><pre className={outputTab==="stderr" && task.result.stderr ? "stderr" : ""}>{task.result[outputTab as "stdout"|"stderr"] || "（无输出）"}</pre><div className="task-output-footer"><span>退出码 <b>{task.result.exitCode}</b></span><span>{task.result.truncated ? "输出已截断" : "输出未截断"}</span></div></div> : <div className={`task-result-notice ${task.state==="success" ? "green" : "amber"}`}><CircleAlert size={18}/><div><strong>{task.state==="success" ? "已收到成功结果" : "任务返回失败结果"}</strong><p>{task.result.stderr || "本次任务已结束，可查看下方文件状态。"}</p></div></div> : <div className="task-waiting"><span>{task.state==="rejected" ? <CircleAlert size={29}/> : <Inbox size={29}/>}</span><h3>{task.state==="rejected" ? "设备拒绝了此任务" : task.state==="running" ? "任务正在执行" : task.state==="queued" ? "任务等待执行" : task.state==="received" ? "等待设备确认" : "等待最终结果"}</h3><p>{task.state==="rejected" ? "未收到执行结果，可检查任务参数和设备能力。" : "完成后将展示最终结果，当前暂无输出。"}</p><small>状态通过任务快照同步</small></div>}
          {task.transfer && <div className="task-transfer-facts"><h4><ArrowDownToLine size={14}/>文件传输状态<span>{task.transfer.size}</span></h4><div><span>已提交<b>{task.transfer.committed ? "是" : "否"}</b></span><span>已释放<b>{task.transfer.released ? "是" : "否"}</b></span><span>传输失败<b>{task.transfer.failed ? "是" : "否"}</b></span></div>{task.kind==="download" && task.transfer.committed && <p>文件已完整提交；任务结果独立显示。</p>}{task.kind==="deployment" && <p>工具仅投放到设备，未自动执行。</p>}</div>}
          </> : <div className="task-params"><h4><Settings2 size={15}/>任务规格</h4><dl className="task-facts"><div><dt>任务类型</dt><dd>{task.kind==="deployment" ? "upload · 工具投放" : task.kind}</dd></div><div><dt>设备 ID</dt><dd>{task.device}</dd></div><div><dt>创建时间</dt><dd>{task.created}</dd></div><div><dt>超时</dt><dd>{task.timeout||"—"} 秒</dd></div><div><dt>派发次数</dt><dd>{task.dispatches}</dd></div><div><dt>最近设备会话</dt><dd>{task.session??"—"}</dd></div>{task.asset && <div><dt>关联资产 / 工具</dt><dd>{task.asset}</dd></div>}</dl><h4>{task.kind==="exec" ? "命令" : "设备目标路径"}</h4><pre>{task.command}</pre>{task.kind==="exec" && <><h4>工作目录</h4><pre>{task.cwd || "使用默认目录"}</pre><h4>环境变量</h4><pre>{JSON.stringify(task.env,null,2)}</pre></>}</div>}
        </div>
        <div className="task-detail-actions"><button disabled={!taskDevice.online || model.busy || !!model.state.pending || !model.state.synchronized} onClick={()=>void model.run('重发原任务',async()=>{await model.write(mutation('重发任务',`tasks/${segment(task.id)}/resend`));})}><RefreshCw size={14}/>重发 / 查询原任务</button>{task.kind==="download" && task.transfer?.committed && task.transfer.released && <button className="primary" disabled={task.imported || model.busy || !!model.state.pending} onClick={()=>void model.run('导入已提交文件',async()=>{const result=await model.write(mutation('导入下载',`downloads/${segment(task.id)}/complete`));model.setInspect({title:"下载导入结果",value:result});})}><CheckCheck size={14}/>{task.imported ? "已导入资产" : "导入已提交文件"}</button>}<small>重发保留原任务身份，用于查询或补报结果。</small></div>
      </aside> : <aside className="task-detail-panel"><div className="task-empty"><Inbox size={32}/><h3>选择任务查看详情</h3><p>暂无选中的任务</p></div></aside>}
    </div>
    <footer className="task-page-footer"><ShieldCheck size={13}/><span>任务记录保留在本次服务运行期间</span><span>{model.state.status}</span></footer>
    <dialog ref={dialogRef} className="task-create-dialog" onCancel={()=>setCreating(false)}><form onSubmit={e=>{e.preventDefault();createTask();}}><div className="dialog-header"><div><h2>新建命令任务</h2><p>在一台在线设备上执行一次命令</p></div><button type="button" className="icon" aria-label="关闭新建任务" onClick={()=>setCreating(false)}><X size={20}/></button></div><div className="task-create-body"><label>目标设备<select aria-label="新任务目标设备" value={formDevice} onChange={e=>setFormDevice(e.target.value)}>{devices.map(d=><option key={d.id} value={d.id} disabled={!d.online}>{d.name}{d.online ? "" : "（离线）"}</option>)}</select></label><label>执行命令<textarea aria-label="新任务执行命令" placeholder="例如：ip addr show" rows={4} value={command} onChange={e=>setCommand(e.target.value)}/></label><div className="task-command-presets"><span>常用命令</span>{["uptime","ip addr show","df -h"].map(value=><button type="button" key={value} onClick={()=>setCommand(value)}>{value}</button>)}</div><label>超时时间<span className="task-timeout-input"><input aria-label="新任务超时秒数" type="number" min="1" max="4294967295" step="1" value={timeout} onChange={e=>setTimeoutValue(e.target.value)}/><span>秒</span></span></label><button className="task-advanced-toggle" type="button" aria-expanded={advanced} onClick={()=>setAdvanced(!advanced)}><Settings2 size={14}/>高级参数<ChevronDown size={13}/></button>{advanced && <div className="task-advanced-fields"><label>工作目录<input aria-label="新任务工作目录" value={cwd} onChange={e=>setCwd(e.target.value)}/></label><label>环境变量（JSON）<textarea aria-label="新任务环境变量" rows={3} value={env} onChange={e=>setEnv(e.target.value)}/></label></div>}{(formError || model.error) && <p className="task-form-error" role="alert">{formError || model.error}</p>}<p className="task-create-note"><CircleAlert size={14}/>将通过服务器向所选设备派发命令。</p></div><div className="dialog-actions"><button type="button" onClick={()=>setCreating(false)}>取消</button><button className="primary" type="submit" disabled={!command.trim() || model.busy || !!model.state.pending || !model.state.synchronized}><Send size={15}/>创建任务</button></div></form></dialog>
  </main>;
}