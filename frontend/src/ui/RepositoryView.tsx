import { useEffect, useRef, useState, type ReactNode } from "react";
import { Archive, ArrowDownToLine, ArrowRight, ArrowUpFromLine, Box, Check, ChevronRight, CircleAlert, CircleCheck, Clock3, Copy, Cpu, FileCode2, FileText, Files, Folder, FolderInput, HardDrive, History, Layers3, Network, Package, Plus, RefreshCw, Search, Send, ShieldCheck, Terminal, UploadCloud, X } from "lucide-react";
import "./repository.css";
import { describe, importMutation, mutation, segment } from "../api";
import { type Workbench, stamp, positive } from "../useWorkbench";
import { useQuery } from "../useQuery";
import { stateName } from "../components";
import { platform } from "../platform";
import { readRepository } from "./repositoryData";
const classify=(name:string)=> /\.(conf|cfg|json|yaml|yml|ini)$/i.test(name)?'配置文件':/\.(sh|py|pl)$/i.test(name)?'脚本文件':/\.(log|txt)$/i.test(name)?'日志文件':'其他文件';


type Device = { id: string; name: string; arch: string; online: boolean };
type Asset = { id: string; name: string; size: number; kind: string; created: string; archived: boolean; source: string; sha: string };
type Artifact = { id: string; asset: string; arch: string; libc: string; mode: string };
type Version = { label: string; created: string; archived: boolean; artifacts: Artifact[] };
type Tool = { id: string; name: string; description: string; category: string; tone: string; archived: boolean; versions: Version[] };
type Transfer = { id: string; name: string; device: string; path: string; kind: "upload" | "download" | "deployment"; state: string; size: number; committed: boolean; released: boolean; imported?: string; cleaned?: boolean };
const sizeText = (n:number) => n < 1024 ? `${n} B` : n < 1048576 ? `${(n/1024).toFixed(1)} KB` : `${(n/1048576).toFixed(1)} MB`;
const kindIcon = (kind:string) => kind === "配置文件" ? FileCode2 : kind === "日志文件" ? FileText : kind === "脚本文件" ? Terminal : Box;
function Tag({children,tone="neutral"}:{children:ReactNode;tone?:string}) {return <span className={`repo-tag ${tone}`}>{children}</span>;}
function Facts({rows}:{rows:[string,ReactNode][]}) {return <dl className="repo-facts">{rows.map(([label,value])=><div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>;}

export function RepositoryView({model}:{model:Workbench}) {
  const page=model.page,notify=model.setNotice;
  const devices=model.state.snapshot.devices.map(d=>({id:d.device_id,name:d.registration.hostname||d.device_id,arch:d.registration.arch,online:d.status==='online'}));
  const data=useQuery(model.owner.current,JSON.stringify([model.state.snapshot.tools.map(t=>t.tool_id),model.state.snapshot.tasks.filter(t=>t.type!=='exec').slice(0,50).map(t=>t.task_id)]),model.state.snapshot.fetchedAt,c=>readRepository(c,model.state.snapshot.tools,model.state.snapshot.tasks),model.setError);
  const assets:Asset[]=model.state.snapshot.assets.map(a=>({id:a.asset_id,name:a.name,size:a.size,kind:classify(a.name),created:stamp(a.created_at),archived:a.archived,source:'共享仓库',sha:a.sha256}));
  const tools:Tool[]=model.state.snapshot.tools.map(t=>({id:t.tool_id,name:t.name,description:t.description,category:'未分类',tone:'blue',archived:t.archived,versions:(data?.versions??[]).filter(v=>v.tool_id===t.tool_id).map(v=>({label:v.version,created:stamp(v.created_at),archived:v.archived,artifacts:v.artifacts.map(a=>({id:a.artifact_id,asset:a.asset_id,arch:a.rules.arch.join(', '),libc:a.rules.libc.join(', '),mode:a.mode}))}))}));
  const transfers:Transfer[]=(data?.transfers??[]).map(({detail:t,transfer:f,operation:o})=>({id:t.task_id,name:String(t.params?.name||t.params?.remote_path||t.task_id),device:t.device_id,path:String(t.params?.remote_path??''),kind:o.tool_id?'deployment':t.type as 'upload'|'download',state:stateName(t.state),size:f.size,committed:f.committed,released:f.released,imported:t.type==='download'?(o.asset_id||undefined):undefined}));
  const [fileTab,setFileTab]=useState("assets");
  const [assetId,setAssetId]=useState("");
  const toolId=model.toolId,setToolId=model.setToolId;
  const versionLabel=model.version,setVersionLabel=model.setVersion;
  const [artifactId,setArtifactId]=useState("");
  const deviceId=model.selected,setDeviceId=model.setSelected;
  const [fileQuery,setFileQuery]=useState("");
  const [toolQuery,setToolQuery]=useState("");
  const [fileKind,setFileKind]=useState("全部文件");
  const [toolCategory,setToolCategory]=useState("全部工具");
  const [showArchive,setShowArchive]=useState(false);
  const [showToolArchive,setShowToolArchive]=useState(false);
  const [modal,setModal]=useState("");
  const [form,setForm]=useState<Record<string,string>>({});
  const [pickedFile,setPickedFile]=useState<File|null>(null);
  const [error,setError]=useState("");
  const [advancedArtifacts,setAdvancedArtifacts]=useState("");
  const dialogRef=useRef<HTMLDialogElement>(null);
  const pageRef=useRef<HTMLElement>(null);

  const isFiles=page==="files";
  const actualAsset=assets.find(a=>a.id===assetId)??assets[0];
  const asset=actualAsset??{id:"",name:"暂无文件",kind:"",size:0,created:"—",source:"—",sha:"—",archived:true};
  const AssetIcon=kindIcon(asset.kind);
  const actualTool=tools.find(t=>t.id===toolId);
  const tool=actualTool??{id:"",name:"选择工具查看详情",description:"",category:"",tone:"blue",archived:true,versions:[]};
  const version=tool.versions.find(v=>v.label===versionLabel);
  const selectedArtifact=version?.artifacts.find(a=>a.id===artifactId);
  const device=devices.find(d=>d.id===deviceId)??{id:"",name:"暂无设备",arch:"未提供",online:false};
  const references=(id:string)=>tools.flatMap(t=>t.versions.filter(v=>!t.archived&&!v.archived&&v.artifacts.some(a=>a.asset===id)).map(v=>`${t.name} · ${v.label}`));
  const visibleAssets=assets.filter(a=>(showArchive||!a.archived)&&(fileKind==="全部文件"||(fileKind==="工具文件"?references(a.id).length>0:a.kind===fileKind))&&`${a.name} ${a.id}`.toLowerCase().includes(fileQuery.toLowerCase()));
  const visibleTools=tools.filter(t=>(showToolArchive||!t.archived)&&(toolCategory==="全部工具"||t.category===toolCategory)&&`${t.name} ${t.description}`.toLowerCase().includes(toolQuery.toLowerCase()));
  const activeTools=tools.filter(t=>!t.archived);
  const versions=activeTools.flatMap(t=>t.versions.filter(v=>!v.archived));
  const readyDownloads=transfers.filter(t=>t.kind==="download"&&t.committed&&t.released&&!t.imported).length;
  const compatibility=(a:Artifact)=>{const match=model.matches.find(m=>m.artifact.artifact_id===a.id);return match?.status==='compatible'?'兼容':match?.status==='incompatible'?'不兼容':'信息不足';};
  const canDeploy=model.enabled&&!!selectedArtifact&&compatibility(selectedArtifact)==="兼容"&&device.online&&!tool.archived&&!version?.archived&&!assets.find(a=>a.id===selectedArtifact.asset)?.archived;
  useEffect(()=>{if(modal)dialogRef.current?.showModal();else dialogRef.current?.close();},[modal]);
  useEffect(()=>{pageRef.current?.scrollTo({top:0});},[page]);
  const field=(key:string,value:string)=>setForm(prev=>({...prev,[key]:value}));
  const open=(kind:string)=>{setError("");setAdvancedArtifacts("");setPickedFile(null);setForm({name:"",description:"",device:deviceId,path:kind==="download"?"/var/log/messages":kind==="deploy"?`/tmp/tools/${tool.name.toLowerCase().replaceAll(" ","-")}`:`/tmp/${asset.name}`,timeout:"120",mode:kind==="publish"?"0755":"0644",overwrite:"false",version:"",asset:assets.find(a=>!a.archived)?.id??"",arch:"aarch64",libc:"any"});setModal(kind);};
  const chooseTool=(t:Tool)=>{setToolId(t.id);setVersionLabel("");setArtifactId("");};
  const copy=(text:string)=>void navigator.clipboard.writeText(text).then(()=>notify("已复制"),()=>notify("复制未成功"));
  const submit=()=>void model.run(`仓库操作：${modal}`,async()=>{
    setError("");
    try {
    if(["upload","download","deploy"].includes(modal)) {
      if(!form.path.trim()||!/^\d+$/.test(form.timeout)||Number(form.timeout)<1||Number(form.timeout)>4294967295||!devices.find(d=>d.id===form.device)?.online){setError("请选择在线设备，填写目标路径和正整数超时秒数。");return;}
      if(modal==="upload"&&!/^0[0-7]{3}$/.test(form.mode)){setError("文件权限请使用四位八进制，例如 0644。");return;}
      if(modal==="deploy"&&!canDeploy){setError("请先选择兼容的产物与在线设备。");return;}
      await model.write(mutation('创建传输',modal==='deploy'?'deployments':modal==='upload'?'uploads':'downloads',{
        device_id:form.device,remote_path:form.path,timeout_seconds:positive(form.timeout),
        ...(modal==='download'?{name:form.name||form.path.split('/').pop()||'download'}:{overwrite:form.overwrite==='true'}),
        ...(modal==='upload'?{asset_id:asset.id,mode:form.mode}:{}),
        ...(modal==='deploy'?{tool_id:tool.id,version:versionLabel,artifact_id:selectedArtifact!.id}:{})
      }));
      setFileTab('transfers');
    } else if(modal==="import") {
      if(!pickedFile)throw Error('请选择要导入的文件');
      const file=form.name.trim()?new File([pickedFile],form.name.trim(),{type:pickedFile.type}):pickedFile;
      const result=await model.write(await importMutation(file));setAssetId(result.asset_id);setFileTab('assets');setFileKind('全部文件');setFileQuery('');
    } else if(modal==="create-tool") {
      if(!form.name.trim()){setError("请填写工具名称。");return;}
      const result=await model.write(mutation('创建工具','tools',{name:form.name.trim(),description:form.description}));setToolId(result.tool_id);setToolCategory('全部工具');setToolQuery('');
    } else if(modal==="publish") {
      if(!form.version.trim()||!form.asset||!form.arch.trim()||!form.libc.trim()||!/^0[0-7]{3}$/.test(form.mode)){setError("请填写版本标签、文件、架构、libc 和四位八进制权限。");return;}
      if(tool.versions.some(v=>v.label===form.version.trim())){setError("此版本标签已存在，请使用新标签；已归档版本也不能覆盖。");return;}
      await model.write(mutation('发布版本',`tools/${segment(tool.id)}/versions/${segment(form.version.trim())}`,{artifacts:advancedArtifacts.trim()?JSON.parse(advancedArtifacts):[{asset_id:form.asset,platform:'linux',mode:form.mode,rules:{arch:[form.arch],libc:[form.libc]}}]},'PUT'));
      setVersionLabel(form.version.trim());
    } else if(modal==='archive-asset') await model.write(mutation('归档文件',`assets/${segment(asset.id)}/archive`));
    else if(modal==='archive-tool') await model.write(mutation('归档工具',`tools/${segment(tool.id)}/archive`));
    else if(modal==='archive-version') await model.write(mutation('归档版本',`tools/${segment(tool.id)}/versions/${segment(versionLabel)}/archive`));
    setModal('');
    } catch(e) {setError(describe(e));throw e;}
  });
  const importDownload=(t:Transfer)=>void model.run('导入已提交下载',async()=>{const result=await model.write(mutation('导入下载',`downloads/${segment(t.id)}/complete`));model.setInspect({title:'下载导入结果',value:result});});
  const saveAsset=()=>void model.run('保存到本地',async()=>{const a=model.state.snapshot.assets.find(a=>a.asset_id===asset.id),c=model.owner.current;if(a&&c){const blob=await c.track(()=>c.api.content(a));if(c===model.owner.current)await platform.saveFile(a.name,blob);}});
  const title=isFiles?"文件管理":"工具仓库";
  const PageIcon=isFiles?Folder:Package;
  const statItems=isFiles?[
    [Files,"文件资产",String(assets.length),"含已归档记录","blue"],
    [HardDrive,"文件总大小",sizeText(assets.reduce((n,a)=>n+a.size,0)),"按资产累加，非磁盘占用","cyan"],
    [Layers3,"工具引用",data?String(assets.filter(a=>references(a.id).length).length):"—","被可用版本引用的文件","purple"],
    [FolderInput,"待导入下载",data?String(readyDownloads):"—","已提交且已释放的文件","amber"],
  ]:[
    [Package,"可用工具",String(activeTools.length),"集中维护工具与说明","blue"],
    [Layers3,"可用版本",data?String(versions.length):"—","明确版本，保留历史","purple"],
    [Box,"版本产物",data?String(versions.flatMap(v=>v.artifacts).length):"—","关联仓库中的文件资产","cyan"],
    [Cpu,"目标架构",String(new Set(versions.flatMap(v=>v.artifacts.map(a=>a.arch)).filter(a=>a!=="any")).size),"另含通用脚本产物","green"],
  ];
  return <main ref={pageRef} className={`repo-center ${isFiles?"repo-files":"repo-tools"}`}>
    <div className="repo-breadcrumb">工作空间<ChevronRight size={13}/><span>{title}</span></div>
    <header className="repo-page-heading"><div><div className="repo-title"><span><PageIcon size={27}/></span><h1>{title}</h1><Tag>共享仓库</Tag></div><p>{isFiles?"集中管理配置、日志与工具文件，按需传输到设备":"管理工具版本与架构产物，为设备选择合适的维护工具"}</p></div><div className="repo-heading-actions"><button onClick={()=>model.owner.current?.invalidate()}><RefreshCw size={15}/>刷新</button>{isFiles&&<button onClick={()=>open("download")}><ArrowDownToLine size={15}/>从设备下载</button>}<button className="primary" onClick={()=>open(isFiles?"import":"create-tool")}>{isFiles?<UploadCloud size={17}/>:<Plus size={17}/>} {isFiles?"导入文件":"创建工具"}</button></div></header>
    <div className="repo-stats">{statItems.map(([Icon,label,value,caption,tone])=>{const Glyph=Icon as typeof Files;return <div className="repo-stat" key={String(label)}><span className={`repo-glyph ${tone}`}><Glyph size={23}/></span><div><span>{String(label)}</span><strong>{String(value)}</strong><small>{String(caption)}</small></div></div>;})}</div>
    {isFiles?<>
      <div className="repo-page-tabs" role="tablist" aria-label="文件管理视图"><button role="tab" aria-selected={fileTab==="assets"} className={fileTab==="assets"?"selected":""} onClick={()=>setFileTab("assets")}><Folder size={16}/>文件资产</button><button role="tab" aria-selected={fileTab==="transfers"} className={fileTab==="transfers"?"selected":""} onClick={()=>setFileTab("transfers")}><History size={16}/>传输记录<Tag>{transfers.length}</Tag></button><span>文件入库后可重复使用</span></div>
      {fileTab==="assets"?<div className="repo-workspace">
        <section className="repo-panel repo-library" aria-label="文件资产列表">
          <div className="repo-panel-heading"><h2>文件资产<Tag>{visibleAssets.length}</Tag></h2><label className="repo-checkbox"><input type="checkbox" checked={showArchive} onChange={e=>setShowArchive(e.target.checked)}/>显示已归档</label></div>
          <div className="repo-filter"><label className="repo-search"><Search size={16}/><input aria-label="搜索仓库文件" placeholder="搜索文件名或资产 ID…" value={fileQuery} onChange={e=>setFileQuery(e.target.value)}/></label></div>
          <div className="repo-chips">{["全部文件","配置文件","脚本文件","日志文件","工具文件"].map(k=><button key={k} className={fileKind===k?"selected":""} onClick={()=>setFileKind(k)}>{k}</button>)}</div>
          <div className="repo-table-scroll"><table className="repo-table"><thead><tr><th>文件名称</th><th>大小</th><th>工具引用</th><th>入库时间</th><th>状态</th><th/></tr></thead><tbody>{visibleAssets.map(a=>{const Icon=kindIcon(a.kind);return <tr key={a.id} className={a.id===assetId?"selected":""} onClick={()=>setAssetId(a.id)}><td><button className="repo-file-name" onClick={()=>setAssetId(a.id)}><span className={`repo-file-icon ${a.kind==="工具文件"?"purple":a.kind==="日志文件"?"amber":"blue"}`}><Icon size={21}/></span><span><strong>{a.name}</strong><small>{a.kind} · {a.id}</small></span></button></td><td>{sizeText(a.size)}</td><td>{references(a.id).length?<Tag tone="purple">{references(a.id).length} 个版本</Tag>:<span className="repo-muted">—</span>}</td><td>{a.created}</td><td><Tag tone={a.archived?"neutral":"green"}>{a.archived?"已归档":"可用"}</Tag></td><td><button className="icon" aria-label={`查看文件 ${a.name}`} onClick={()=>setAssetId(a.id)}><ChevronRight size={15}/></button></td></tr>;})}</tbody></table>{!visibleAssets.length&&<div className="repo-empty"><Search size={30}/><h3>没有匹配的文件</h3><button onClick={()=>{setFileQuery("");setFileKind("全部文件");}}>清除筛选</button></div>}</div>
          <div className="repo-panel-footer"><span>共 {visibleAssets.length} 个文件</span><span>共享仓库</span></div>
          <button className="repo-import-strip" onClick={()=>open("import")}><span className="repo-glyph blue"><UploadCloud size={23}/></span><span><strong>将本地文件加入共享仓库</strong><small>导入后可上传至设备，或用于发布工具版本</small></span><Plus size={18}/></button>
        </section>
        <aside className="repo-panel repo-detail" aria-label="文件详情"><div className="repo-panel-heading"><h2><FileText size={17}/>文件详情</h2><Tag tone={asset.archived?"neutral":"green"}>{actualAsset?(asset.archived?"已归档":"可用"):"未选择"}</Tag></div><div className="repo-detail-body"><div className="repo-asset-hero"><span className="repo-glyph blue"><AssetIcon size={33}/></span><h3>{asset.name}</h3><p>{asset.kind} · {sizeText(asset.size)}</p></div><Facts rows={[["资产 ID",<button className="repo-copy" disabled={!actualAsset} onClick={()=>copy(asset.id)}>{asset.id}<Copy size={12}/></button>],["入库时间",asset.created],["来源",asset.source]]}/><div className="repo-info-box"><ShieldCheck size={19}/><div><strong>文件内容与资产身份</strong><p>{actualAsset?"已按大小与 SHA-256 校验入库。":"选择文件后查看大小与 SHA-256。"}</p><code className="production-digest">{asset.sha}</code></div></div><div className="repo-detail-section"><h3><Layers3 size={16}/>工具版本引用<Tag>{references(asset.id).length}</Tag></h3>{references(asset.id).length?references(asset.id).map(ref=><div className="repo-reference" key={ref}><Package size={16}/>{ref}</div>):<p className="repo-muted">{data?"暂无可用工具版本引用，可独立传输。":"正在读取工具版本引用…"}</p>}</div><div className="repo-detail-section"><h3><ArrowUpFromLine size={16}/>发送到设备</h3><p className="repo-muted">选择在线设备与目标路径，创建一次文件上传任务。</p><button className="repo-wide primary" disabled={asset.archived} onClick={()=>open("upload")}><Send size={15}/>上传到设备<ArrowRight size={15}/></button></div></div><div className="repo-detail-footer"><button disabled={asset.archived} onClick={saveAsset}><ArrowDownToLine size={14}/>保存到本地</button><button disabled={asset.archived} onClick={()=>open("archive-asset")}><Archive size={14}/>归档</button><small>归档保留文件，不释放存储空间。</small></div></aside>
      </div>:<section className="repo-panel repo-transfers"><div className="repo-panel-heading"><h2>传输记录<Tag>{transfers.length}</Tag></h2><span className="repo-muted">本次服务运行期间</span></div><div className="repo-transfer-banner"><FolderInput size={22}/><div><strong>{readyDownloads} 个下载文件待导入仓库</strong><p>下载文件完整提交并释放后可导入，任务执行结果单独保留。</p></div></div><div className="repo-table-scroll"><table className="repo-table transfer-table"><thead><tr><th>文件 / 传输任务</th><th>设备与路径</th><th>方向</th><th>任务结果</th><th>文件状态</th><th>操作</th></tr></thead><tbody>{transfers.map(t=><tr key={t.id}><td><strong>{t.name}</strong><small>{t.id}</small></td><td>{devices.find(d=>d.id===t.device)?.name}<small>{t.path}</small></td><td>{t.kind==="download"?"设备 → 仓库":t.kind==="deployment"?"工具 → 设备":"仓库 → 设备"}</td><td><Tag tone={t.state==="成功"?"green":t.state==="失败"?"red":"blue"}>{t.state}</Tag></td><td>{t.imported?<Tag tone="green">已入库</Tag>:t.committed?<Tag tone="blue">已完整提交</Tag>:t.released?"已释放":"等待传输确认"}<small>{t.size?sizeText(t.size):"大小待确认"} · {t.cleaned?"暂存已清理":t.released?"句柄已释放":"尚未释放"}</small></td><td>{t.kind==="download"&&t.committed&&t.released&&!t.imported?<button className="primary" onClick={()=>importDownload(t)}><FolderInput size={14}/>导入仓库</button>:t.kind==="download"&&t.released&&(!t.committed||t.imported)?<button disabled={t.cleaned} onClick={()=>void model.run("清理暂存",async()=>{await model.write(mutation("清理暂存",`downloads/${segment(t.id)}/cleanup`));})}>{t.cleaned?"已清理":"清理暂存"}</button>:<span className="repo-muted">—</span>}</td></tr>)}</tbody></table></div><div className="repo-panel-footer"><span>共 {transfers.length} 项 · 最近 50 条</span><span>任务结果与文件提交状态分别展示</span></div></section>}
    </>:<div className="repo-workspace tool-workspace">
      <section className="repo-panel repo-tool-catalog" aria-label="工具列表"><div className="repo-panel-heading"><h2>工具目录<Tag>{visibleTools.length}</Tag></h2><label className="repo-checkbox"><input type="checkbox" checked={showToolArchive} onChange={e=>setShowToolArchive(e.target.checked)}/>显示已归档</label></div><div className="repo-filter"><label className="repo-search"><Search size={16}/><input aria-label="搜索工具" placeholder="搜索工具名称或用途…" value={toolQuery} onChange={e=>setToolQuery(e.target.value)}/></label></div><div className="repo-chips">{["全部工具","未分类"].map(c=><button key={c} className={toolCategory===c?"selected":""} onClick={()=>setToolCategory(c)}>{c}</button>)}</div><div className="repo-tool-scroll"><div className="repo-tool-grid">{visibleTools.map(t=><button className={`repo-tool-card ${toolId===t.id?"selected":""}`} key={t.id} onClick={()=>chooseTool(t)}><div className="repo-tool-top"><span className={`repo-glyph ${t.tone}`}>{t.category==="运维脚本"?<Terminal size={25}/>:t.category==="网络诊断"?<Network size={25}/>:<Package size={25}/>}</span><Tag tone={t.archived?"neutral":"green"}>{t.archived?"已归档":"可用"}</Tag></div><h3>{t.name}</h3><p>{t.description||"尚未填写工具说明"}</p><div className="repo-tool-arches">{[...new Set(t.versions.filter(v=>!v.archived).flatMap(v=>v.artifacts.map(a=>a.arch)))].map(arch=><Tag key={arch}>{arch==="any"?"通用":arch}</Tag>)}{!t.versions.length&&<Tag>待发布版本</Tag>}</div><div className="repo-tool-card-footer"><span>{t.category}</span><span>{t.versions.length} 个版本<ChevronRight size={14}/></span></div></button>)}</div>{!visibleTools.length&&<div className="repo-empty"><Search size={30}/><h3>没有匹配的工具</h3><button onClick={()=>{setToolQuery("");setToolCategory("全部工具");}}>清除筛选</button></div>}</div><div className="repo-catalog-note"><ShieldCheck size={18}/><div><strong>文件、版本、设备，一次明确选择</strong><p>版本产物引用文件资产；选择目标设备后查看兼容性，再发起投放。</p></div></div><div className="repo-panel-footer"><span>共 {visibleTools.length} 个工具</span><span>当前 API 未提供用途分类</span></div></section>
      <aside className="repo-panel repo-detail tool-detail" aria-label="工具详情"><div className="repo-panel-heading"><h2><Package size={17}/>工具详情</h2><button className="icon" aria-label="归档当前工具" disabled={tool.archived} onClick={()=>open("archive-tool")}><Archive size={16}/></button></div><div className="repo-detail-body"><div className="repo-tool-hero"><span className={`repo-glyph ${tool.tone}`}><Package size={28}/></span><div><h3>{tool.name}</h3><small>{tool.id}</small></div><Tag tone={tool.archived?"neutral":"green"}>{actualTool?(tool.archived?"已归档":"可用"):"未选择"}</Tag></div><p className="repo-muted tool-description">{tool.description||"暂无工具说明"}</p><div className="repo-version-heading"><h3><Layers3 size={16}/>版本与产物</h3><button className="text-button" disabled={tool.archived} onClick={()=>open("publish")}><Plus size={14}/>发布版本</button></div><label className="repo-version-select"><select aria-label="选择工具版本" value={versionLabel} onChange={e=>{setVersionLabel(e.target.value);setArtifactId(tool.versions.find(v=>v.label===e.target.value)?.artifacts[0]?.id??"");}}>{!tool.versions.length&&<option value="">暂无已发布版本</option>}{tool.versions.map(v=><option value={v.label} key={v.label}>{v.label}{v.archived?" · 已归档":""}</option>)}</select>{version&&<button className="icon" aria-label="归档当前版本" disabled={tool.archived||version.archived} onClick={()=>open("archive-version")}><Archive size={15}/></button>}</label>{version&&<div className="repo-version-meta"><span>{version.created} 发布</span><span>{version.artifacts.length} 个产物 · 版本不可覆盖</span></div>}
      {version?.artifacts.map(a=>{const match=compatibility(a);const file=assets.find(f=>f.id===a.asset);return <button title={model.matches.find(m=>m.artifact.artifact_id===a.id)?.checks.map(c=>`${c.field}: ${c.status} · ${c.reason}`).join("\n")} className={`repo-artifact ${artifactId===a.id?"selected":""}`} key={a.id} onClick={()=>setArtifactId(a.id)} aria-pressed={artifactId===a.id}><div><span className="repo-radio">{artifactId===a.id&&<i/>}</span><Cpu size={17}/><strong>Linux · {a.arch==="any"?"通用":a.arch}</strong><Tag tone={match==="兼容"?"green":match==="不兼容"?"red":"amber"}>{match}</Tag></div><p>{file?.name}</p><small>libc: {a.libc} <span>权限 {a.mode}</span><span>{sizeText(file?.size??0)}</span></small></button>;})}
      {!version&&<div className="repo-empty compact"><Box size={27}/><p>发布首个版本，关联文件与兼容条件</p></div>}
      <div className="repo-detail-section"><h3><Network size={16}/>目标设备</h3><select className="repo-device-select" aria-label="工具目标设备" value={deviceId} onChange={e=>setDeviceId(e.target.value)}>{devices.map(d=><option key={d.id} value={d.id}>{d.name}{!d.online?" · 离线":""}</option>)}</select><div className="repo-device-meta"><Tag tone={device.online?"green":"neutral"}>{device.online?"在线":"离线"}</Tag><span>{device.arch} · {model.device?.registration.libc || "libc 未提供"}</span></div>{selectedArtifact&&<div className={`repo-match-message ${canDeploy?"green":"amber"}`}>{canDeploy?<CircleCheck size={18}/>:<CircleAlert size={18}/>}<p>{tool.archived||version?.archived?"工具或版本已归档，不可新投放。":!device.online?"设备离线，可查看最近资料，暂不可投放。":compatibility(selectedArtifact)==="兼容"?"服务端判定兼容；投放只上传文件，不自动执行。":compatibility(selectedArtifact)==="不兼容"?"产物架构与设备不匹配，请选择其他产物。":"兼容性尚未确认，请查看产物判断详情。"}</p></div>}</div></div><div className="repo-detail-footer"><button className="primary repo-wide" disabled={!canDeploy} onClick={()=>open("deploy")}><Send size={16}/>投放到设备<ArrowRight size={16}/></button><small>明确选择版本与产物后，创建文件上传任务。</small></div></aside>
    </div>}
    <footer className="repo-page-footer"><ShieldCheck size={13}/><span>文件与工具资料保存在共享仓库</span><span>{model.state.status}</span></footer>
    <dialog ref={dialogRef} className="repo-dialog" onCancel={()=>setModal("")}><form onSubmit={e=>{e.preventDefault();submit();}}><div className="dialog-header"><h2>{{import:"导入文件",upload:"上传到设备",download:"从设备下载",deploy:"投放工具",publish:"发布工具版本","create-tool":"创建工具","archive-asset":"归档文件","archive-tool":"归档工具","archive-version":"归档版本"}[modal]??"操作"}</h2><button type="button" className="icon" aria-label="关闭仓库弹窗" onClick={()=>setModal("")}><X size={19}/></button></div><div className="repo-form-body">
    {modal==="import"&&<><label className="repo-pick-file"><UploadCloud size={35}/><strong>{pickedFile?.name??"选择要加入仓库的文件"}</strong><small>{pickedFile?sizeText(pickedFile.size):"读取文件并校验后导入共享仓库"}</small><input type="file" aria-label="选择导入文件" onChange={e=>{const f=e.target.files?.[0];if(f)setPickedFile(f);}}/></label><label>文件名称<input value={form.name} placeholder={pickedFile?.name??"默认使用原文件名"} onChange={e=>field("name",e.target.value)}/></label></>}
    {modal==="create-tool"&&<><label>工具名称<input aria-label="新工具名称" value={form.name} onChange={e=>field("name",e.target.value)} placeholder="例如：Network Check"/></label><label>工具说明<textarea aria-label="新工具说明" rows={3} value={form.description} onChange={e=>field("description",e.target.value)} placeholder="描述工具用途与适用场景"/></label><div className="repo-form-note">创建后添加版本，将架构产物关联到仓库文件。</div></>}
    {["upload","download","deploy"].includes(modal)&&<>{modal==="deploy"?<div className="repo-form-note"><Package size={17}/>{tool.name} · {versionLabel} · {selectedArtifact?.arch}</div>:modal==="upload"?<div className="repo-form-note"><FileText size={17}/>{asset.name} · {sizeText(asset.size)}</div>:null}<label>目标设备<select aria-label="传输目标设备" disabled={modal==="deploy"} value={form.device} onChange={e=>field("device",e.target.value)}>{devices.map(d=><option key={d.id} disabled={!d.online} value={d.id}>{d.name}{d.online?"":" · 离线"}</option>)}</select></label><label>{modal==="download"?"设备源文件路径":"设备目标路径"}<input aria-label="传输文件路径" value={form.path} onChange={e=>field("path",e.target.value)}/></label>{modal==="download"&&<label>入库文件名<input value={form.name} placeholder="messages.log" onChange={e=>field("name",e.target.value)}/></label>}<div className="repo-form-row"><label>超时时间（秒）<input aria-label="传输超时秒数" type="number" min="1" max="4294967295" value={form.timeout} onChange={e=>field("timeout",e.target.value)}/></label>{modal==="upload"&&<label>文件权限<input aria-label="上传文件权限" value={form.mode} onChange={e=>field("mode",e.target.value)}/></label>}</div>{modal!=="download"&&<label className="repo-checkbox"><input type="checkbox" checked={form.overwrite==="true"} onChange={e=>field("overwrite",String(e.target.checked))}/>覆盖设备上的同名文件</label>}</>}
    {modal==="publish"&&<><div className="repo-form-note"><Package size={17}/>{tool.name} · 添加产物并发布不可变版本</div><label>版本标签<input aria-label="发布版本标签" value={form.version} onChange={e=>field("version",e.target.value)} placeholder="例如：1.3.0"/></label><label>关联仓库文件<select aria-label="版本关联文件" value={form.asset} onChange={e=>field("asset",e.target.value)}>{assets.filter(a=>!a.archived).map(a=><option key={a.id} value={a.id}>{a.name} · {sizeText(a.size)}</option>)}</select></label><div className="repo-form-row"><label>架构<select aria-label="产物架构" value={form.arch} onChange={e=>field("arch",e.target.value)}>{["aarch64","mipsel","x86_64","armv7","any"].map(a=><option key={a}>{a}</option>)}</select></label><label>libc<select aria-label="产物 libc" value={form.libc} onChange={e=>field("libc",e.target.value)}>{["any","musl","uclibc","glibc"].map(a=><option key={a}>{a}</option>)}</select></label><label>权限<input aria-label="产物权限" value={form.mode} onChange={e=>field("mode",e.target.value)}/></label></div><details><summary>多产物与完整匹配条件</summary><label>产物列表 JSON（填写后替代上方单产物）<textarea aria-label="完整产物列表 JSON" rows={6} value={advancedArtifacts} onChange={e=>setAdvancedArtifacts(e.target.value)}/></label></details><p className="repo-muted">平台 Linux；any 表示明确不限制该字段。版本发布后修改内容需使用新标签。</p></>}
    {modal.startsWith("archive")&&<div className="repo-archive-copy"><Archive size={32}/><h3>{modal==="archive-asset"?asset.name:modal==="archive-version"?`${tool.name} · ${versionLabel}`:tool.name}</h3><p>归档后停止新的引用或投放，保留文件、版本和身份；已经派发的任务不受影响。</p>{modal==="archive-asset"&&references(asset.id).length>0&&<p className="repo-form-error">此文件仍被 {references(asset.id).length} 个可用版本引用，暂不可归档。</p>}</div>}
    {error&&<p role="alert" className="repo-form-error">{error}</p>}<div className="repo-form-note"><CircleAlert size={15}/>操作将写入当前服务器，执行结果通过快照更新。</div></div><div className="repo-form-actions"><button type="button" onClick={()=>setModal("")}>取消</button><button type="submit" className="primary" disabled={model.busy || !!model.state.pending || !model.state.synchronized || (modal==="archive-asset"&&references(asset.id).length>0)}>{modal.startsWith("archive")?"归档":modal==="publish"?"发布版本":modal==="create-tool"?"创建工具":modal==="import"?"导入文件":"创建传输"}</button></div></form></dialog>
  </main>;
}
