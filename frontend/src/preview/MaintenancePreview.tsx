import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { ArrowDownToLine, ArrowLeft, ArrowUpFromLine, ChevronDown, ChevronLeft, ChevronRight, Clock3, Copy, EllipsisVertical, File, FileCode2, Folder, FolderOpen, Globe, HardDrive, History, LocateFixed, Maximize2, Minimize2, Monitor, PlugZap, Plus, Power, RefreshCw, Search, Settings, ShieldCheck, Terminal, Trash2, UploadCloud, X } from "lucide-react";
import "./maintenance.css";

export type ShellProtocol = "SSH" | "Telnet";
type Props = {
  device: { id: string; host: string; online: boolean };
  active: boolean;
  remainingSeconds: number;
  request: { protocol: ShellProtocol; id: number } | null;
  open: (title: string, kind?: string) => void;
  notify: (message: string) => void;
};
type Entry = { name: string; folder?: boolean; content?: string; file?: File; modified: string };
type FileSystem = Record<string, Entry[]>;
type ShellSession = { protocol: ShellProtocol; lines: string[]; history: string[]; cwd: string; connected: boolean };
type Transfer = { id: number; name: string; direction: "upload" | "download"; label: string; size: string };
const networkConfig = "# Mock configuration for visual preview\nconfig interface 'lan'\n    option device 'br-lan'\n    option proto 'static'\n    option ipaddr '192.168.1.1'\n    option netmask '255.255.255.0'\n";
const stamp = "09-06 14:12";
const folder = (name: string): Entry => ({ name, folder: true, modified: stamp });
const file = (name: string, content: string): Entry => ({ name, content, modified: stamp });
const initialFileSystem: FileSystem = {
  "/": [folder("etc"), folder("root"), folder("tmp"), folder("var")],
  "/etc": [folder("config"), file("hosts", "127.0.0.1 localhost\n192.168.1.1 sh-gateway-01\n"), file("openwrt_release", "DISTRIB_ID='OpenWrt'\nDISTRIB_RELEASE='23.05.5'\n# Mock data\n")],
  "/etc/config": [file("network", networkConfig), file("wireless", "# Mock wireless configuration\nconfig wifi-device 'radio0'\n    option type 'mac80211'\n"), file("firewall", "# Mock firewall configuration\nconfig defaults\n    option input 'ACCEPT'\n"), file("dhcp", "# Mock DHCP configuration\nconfig dhcp 'lan'\n    option start '100'\n    option limit '150'\n"), file("system", "# Mock system configuration\nconfig system\n    option hostname 'sh-gateway-01'\n"), file("dropbear", "# Mock SSH configuration\nconfig dropbear\n    option Port '22'\n")],
  "/root": [folder("scripts"), file("README.txt", "Router workbench demo filesystem.\nFiles and terminal responses exist only in this preview.\n")],
  "/root/scripts": [file("check-network.sh", "#!/bin/sh\n# Demo script — not executed by this preview\nip addr show\n")],
  "/tmp": [file("system.log", "[Mock] 14:00:00 probe connected\n[Mock] 14:12:00 file transfer completed\n")],
  "/var": [folder("log")],
  "/var/log": [file("messages", "[Mock] Sep 6 14:12:00 router: network ready\n"), file("maintenance.log", "[Mock] SSH session opened\n")],
};
function resolvePath(cwd: string, value: string) {
  const parts: string[] = [];
  for (const part of (value.startsWith("/") ? value : `${cwd}/${value}`).split("/")) {
    if (part === "..") parts.pop(); else if (part && part !== ".") parts.push(part);
  }
  return "/" + parts.join("/");
}
function formatSize(entry: Entry) {
  if (entry.folder) return "—";
  const size = entry.file?.size ?? new Blob([entry.content ?? ""]).size;
  return size >= 1024 ? `${(size / 1024).toFixed(1)} KB` : `${size} B`;
}
function newSession(protocol: ShellProtocol, host: string): ShellSession {
  return {
    protocol, cwd: "/root", connected: true, history: [],
    lines: [
      `${protocol} session · root@${host}`, "", "  OpenWrt 23.05.5  /  BusyBox v1.36.1", "  ──────────────────────────────────────────", "  欢迎使用设备远程维护终端", "  Mock 会话：命令仅返回本地示例结果。", "", "  输入 help 查看可体验的命令。", "",
    ],
  };
}

export function MaintenancePreview({ device, active, remainingSeconds, request, open, notify }: Props) {
  const [sessions, setSessions] = useState<Partial<Record<ShellProtocol, ShellSession>>>({});
  const [current, setCurrent] = useState<ShellProtocol>("SSH");
  const [input, setInput] = useState("");
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [expanded, setExpanded] = useState(false);
  const [fontSize, setFontSize] = useState(13);
  const [fs, setFs] = useState<FileSystem>(initialFileSystem);
  const [directory, setDirectory] = useState("/etc/config");
  const [pathInput, setPathInput] = useState(directory);
  const [selected, setSelected] = useState<string>("network");
  const [checked, setChecked] = useState<string[]>([]);
  const [menu, setMenu] = useState<"session" | "terminal" | "files" | null>(null);
  const [filePage, setFilePage] = useState(0);
  const [fileQuery, setFileQuery] = useState("");
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [dragging, setDragging] = useState(false);
  const [preview, setPreview] = useState<{ name: string; text: string } | null>(null);
  const terminalInput = useRef<HTMLInputElement>(null);
  const terminalOutput = useRef<HTMLDivElement>(null);
  const uploadInput = useRef<HTMLInputElement>(null);
  const previewDialog = useRef<HTMLDialogElement>(null);
  const transferId = useRef(0);
  const enabled = device.online && active;
  const session = sessions[current];
  const connected = enabled && !!session?.connected;
  const entries = (fs[directory] ?? []).filter(e => e.name.toLowerCase().includes(fileQuery.toLowerCase()));
  const selection = fs[directory]?.find(e => e.name === selected);
  const pageCount = Math.max(1, Math.ceil(entries.length / 8));
  const visiblePage = Math.min(filePage, pageCount - 1);
  const pageEntries = entries.slice(visiblePage * 8, visiblePage * 8 + 8);
  const downloadEntries = checked.length ? (fs[directory] ?? []).filter(e => checked.includes(e.name) && !e.folder) : selection && !selection.folder ? [selection] : [];
  const sessionCount = Object.values(sessions).filter(s => s?.connected).length;

  const connect = (protocol: ShellProtocol, restart = false) => {
    if (!enabled) return;
    setSessions(prev => ({ ...prev, [protocol]: !restart && prev[protocol]?.connected ? prev[protocol] : newSession(protocol, device.host) }));
    setCurrent(protocol); setInput(""); setHistoryIndex(-1); setMenu(null);
  };
  useEffect(() => { if (request) connect(request.protocol); }, [request]);
  useEffect(() => {
    if (!enabled) setSessions(prev => Object.fromEntries(Object.entries(prev).map(([key, value]) => [key, { ...value, connected: false }])));
  }, [enabled]);
  useEffect(() => { terminalOutput.current?.scrollTo({ top: terminalOutput.current.scrollHeight }); }, [session?.lines]);
  useEffect(() => { if (connected) terminalInput.current?.focus({ preventScroll: true }); }, [current, connected]);
  useEffect(() => { if (preview) previewDialog.current?.showModal(); else previewDialog.current?.close(); }, [preview]);

  const goTo = (path: string) => {
    const normalized = resolvePath(directory, path);
    if (!fs[normalized]) { notify("演示目录不存在"); setPathInput(directory); return; }
    setDirectory(normalized); setPathInput(normalized); setSelected(""); setFileQuery(""); setChecked([]); setFilePage(0);
  };
  const uploadFiles = (files: FileList | File[]) => {
    if (!enabled) return;
    const incoming = Array.from(files);
    // Keep the File objects locally; no request, credentials or remote filesystem is involved.
    setFs(prev => {
      const list = [...(prev[directory] ?? [])];
      for (const item of incoming) {
        let name = item.name;
        for (let suffix = 1; list.some(e => e.name === name); suffix++) name = `${item.name} (${suffix})`;
        list.push({ name, file: item, modified: "刚刚" });
      }
      return { ...prev, [directory]: list };
    });
    setTransfers(prev => [...incoming.map(item => ({ id: ++transferId.current, name: item.name, direction: "upload" as const, label: "已加入演示目录", size: formatSize({ name: item.name, file: item, modified: "" }) })), ...prev].slice(0, 12));
    notify(`${incoming.length} 个文件已加入 ${directory} · 仅本地演示`);
  };
  const downloadFile = () => {
    if (!downloadEntries.length || !enabled) return;
    for (const selection of downloadEntries) {
    const blob = selection.file ?? new Blob([selection.content ?? ""], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url; link.download = selection.name; link.click();
    // The click has already dispatched the browser download; release the temporary URL.
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    setTransfers(prev => [{ id: ++transferId.current, name: selection.name, direction: "download" as const, label: "已发起本地下载", size: formatSize(selection) }, ...prev].slice(0, 12));
    }
    notify(`已发起 ${downloadEntries.length} 个演示文件下载`);
  };
  const inspectFile = (entry: Entry) => {
    if (entry.folder) { goTo(resolvePath(directory, entry.name)); return; }
    setPreview({ name: entry.name, text: entry.file ? "本地上传文件。选择下载可取回原文件。" : entry.content ?? "" });
  };
  const runCommand = (value: string) => {
    if (!connected || !session) return;
    const command = value.trim();
    if (!command) return;
    const parts = command.split(/\s+/);
    let cwd = session.cwd;
    let output = "";
    switch (parts[0]) {
      case "help": output = "可体验：help  pwd  ls  cd  cat  uptime  uname  whoami\n        ip addr  df -h  free -m  echo  clear  exit\n↑ / ↓ 浏览命令历史，Ctrl+L 清屏，Ctrl+C 中断输入。\n所有输出均为 Mock 数据，不执行真实设备命令。"; break;
      case "pwd": output = cwd; break;
      case "whoami": output = "root"; break;
      case "uname": output = `Linux ${device.host} 5.15.167 #0 SMP aarch64 GNU/Linux`; break;
      case "uptime": output = " 14:17:42 up 12 days,  6:28,  load average: 0.04, 0.08, 0.06"; break;
      case "ip": output = "1: lo: <LOOPBACK,UP> mtu 65536\n    inet 127.0.0.1/8 scope host lo\n2: br-lan: <BROADCAST,MULTICAST,UP> mtu 1500\n    inet 192.168.1.1/24 brd 192.168.1.255 scope global br-lan"; break;
      case "df": output = "Filesystem        Size     Used    Available   Use%\n/dev/root         64.0M    16.8M       47.2M    26%\ntmpfs            128.0M     2.1M      125.9M     2%"; break;
      case "free": output = "              total        used        free\nMem:            256          98         158\nSwap:             0           0           0"; break;
      case "echo": output = command.slice(5); break;
      case "cd": {
        const path = resolvePath(cwd, parts[1] === "~" || !parts[1] ? "/root" : parts[1]);
        if (fs[path]) cwd = path; else output = `cd: ${parts[1]}: No such demo directory`;
        break;
      }
      case "ls": {
        const path = resolvePath(cwd, parts.find((p, i) => i > 0 && !p.startsWith("-")) ?? ".");
        output = fs[path] ? fs[path].map(e => `${e.folder ? "drwxr-xr-x" : "-rw-r--r--"}  root  ${formatSize(e).padStart(8)}  ${e.name}${e.folder ? "/" : ""}`).join("\n") : `ls: ${path}: No such demo directory`;
        break;
      }
      case "cat": {
        const path = resolvePath(cwd, parts[1] ?? "");
        const parent = path.slice(0, path.lastIndexOf("/")) || "/";
        const entry = fs[parent]?.find(e => e.name === path.split("/").pop() && !e.folder);
        output = entry ? entry.content ?? "[本地上传文件：请通过文件面板下载查看]" : "cat: file not found in demo filesystem";
        break;
      }
      case "clear": break;
      case "exit": output = `${current} 演示会话已断开。`; break;
      default: output = `演示终端暂不模拟「${parts[0]}」。输入 help 查看示例命令。`;
    }
    setSessions(prev => ({ ...prev, [current]: { ...session, cwd, connected: parts[0] !== "exit", history: [...session.history, command], lines: parts[0] === "clear" ? [] : [...session.lines, `root@${device.host}:${session.cwd}# ${command}`, ...(output ? output.split("\n") : []), ""] } }));
    setInput(""); setHistoryIndex(-1);
  };
  const terminalKeys = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.nativeEvent.isComposing) return;
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "l") {
      event.preventDefault(); if (session) setSessions(prev => ({ ...prev, [current]: { ...session, lines: [] } }));
    } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "c") {
      event.preventDefault(); setInput("");
    } else if (event.key === "ArrowUp" || event.key === "ArrowDown") {
      event.preventDefault();
      const next = event.key === "ArrowUp" ? Math.min(historyIndex + 1, (session?.history.length ?? 0) - 1) : Math.max(-1, historyIndex - 1);
      setHistoryIndex(next); setInput(next < 0 ? "" : session!.history[session!.history.length - 1 - next]);
    }
  };
  const closeSession = (protocol: ShellProtocol) => {
    setSessions(prev => { const next = { ...prev }; delete next[protocol]; return next; });
    setCurrent(protocol === "SSH" ? "Telnet" : "SSH"); setInput("");
  };
  const duration = `${String(Math.floor(remainingSeconds / 3600)).padStart(2, "0")}:${String(Math.floor(remainingSeconds / 60) % 60).padStart(2, "0")}:${String(remainingSeconds % 60).padStart(2, "0")}`;

  return <section className={`remote-maintenance ${expanded ? "terminal-expanded" : ""}`}>
    <div className="maintenance-toolbar">
      <div className="maintenance-heading"><span><ShieldCheck size={25}/></span><div><h2>远程维护</h2><p>{connected ? "当前设备已建立演示连接，在同一工作区完成运维操作" : "连接设备终端，在同一工作区完成运维操作"}</p></div></div>
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
          <div className="shell-session-menu"><button className="new-shell-session" disabled={!enabled} aria-expanded={menu === "session"} onClick={() => setMenu(menu === "session" ? null : "session")}><Plus size={16}/><span>新建会话</span><ChevronDown size={13}/></button>{menu === "session" && <div className="maintenance-menu dark-menu">{(["SSH","Telnet"] as const).map(protocol => <button key={protocol} onClick={() => connect(protocol,true)}><Terminal size={14}/>{sessions[protocol] ? `重建 ${protocol} 会话` : `${protocol} 会话`}</button>)}</div>}</div>
        </div>
        <div className="shell-context">
          <span className={connected ? "connected-label" : ""}><i className={connected ? "connected" : ""}/>{connected ? "已连接" : session ? "已断开" : "等待连接"}</span><span className="shell-host">root@{device.host}</span><span className="shell-encoding">{current} / UTF-8</span>
          <div className="shell-tools"><button aria-label="减小终端字号" onClick={() => setFontSize(Math.max(11,fontSize-1))}>A−</button><button aria-label="增大终端字号" onClick={() => setFontSize(Math.min(18,fontSize+1))}>A+</button><button aria-label="清空终端" disabled={!session} onClick={() => session && setSessions(prev => ({...prev,[current]:{...session,lines:[]}}))}><Trash2 size={16}/></button><div className="terminal-menu-anchor"><button aria-label="终端设置" aria-expanded={menu === "terminal"} onClick={() => setMenu(menu === "terminal" ? null : "terminal")}><Settings size={16}/></button>{menu === "terminal" && <div className="maintenance-menu dark-menu"><button disabled={!session} onClick={() => {setMenu(null);void navigator.clipboard.writeText(session?.lines.join("\n") ?? "").then(()=>notify("终端输出已复制"),()=>notify("复制未成功"));}}><Copy size={14}/>复制终端输出</button><button disabled={!enabled || !session} onClick={() => {setMenu(null);if(connected && session)setSessions(prev=>({...prev,[current]:{...session,connected:false}}));else connect(current);}}><Power size={14}/>{connected ? "断开连接" : "重新连接"}</button><button disabled={!session} onClick={() => {setMenu(null);closeSession(current);}}><X size={14}/>关闭当前会话</button></div>}</div><button aria-label={expanded ? "恢复终端布局" : "展开终端"} onClick={() => setExpanded(!expanded)}>{expanded ? <Minimize2 size={16}/> : <Maximize2 size={16}/>}</button></div>
        </div>
        {session ? <div className="terminal-output" ref={terminalOutput} style={{fontSize}} onClick={e=>{if(e.target === e.currentTarget)terminalInput.current?.focus({preventScroll:true});}}>
          <div className="terminal-lines" role="log" aria-label="终端输出" aria-live="polite">{session.lines.map((line,i)=><div key={i} className={line.startsWith("root@") ? "command-line" : line.includes("Mock") ? "mock-line" : ""}>{line || "\u00a0"}</div>)}</div>
          {connected && <form className="terminal-prompt" onSubmit={e=>{e.preventDefault();runCommand(input);}}><label htmlFor={`shell-input-${device.id}`}>root@{device.host}:<span>{session.cwd}</span>#</label><input id={`shell-input-${device.id}`} ref={terminalInput} aria-label="终端命令" autoComplete="off" spellCheck={false} value={input} onChange={e=>setInput(e.target.value)} onKeyDown={terminalKeys}/></form>}
          {!connected && <div className="terminal-disconnected"><PlugZap size={18}/><span>会话已断开</span><button disabled={!enabled} onClick={()=>connect(current)}>重新连接 {current}</button></div>}
        </div> : <div className="shell-welcome"><span className="welcome-terminal-icon"><Terminal size={43}/></span><h3>从这里连接你的设备</h3><p>选择 SSH 或 Telnet，在内置终端中开始维护</p><div><button disabled={!enabled} onClick={()=>connect("SSH")}><Terminal size={17}/>连接 SSH<ChevronRight size={14}/></button><button disabled={!enabled} onClick={()=>connect("Telnet")}><Monitor size={17}/>连接 Telnet<ChevronRight size={14}/></button></div><small>{enabled ? "本轮为交互预览，命令仅返回示例输出" : "请先开启维护，或等待设备上线"}</small></div>}
        <div className="shell-bottom"><div className="shell-shortcuts">{["uptime","ip addr","df -h","free -m"].map(command=><button key={command} disabled={!connected} onClick={()=>{setInput(command);terminalInput.current?.focus({preventScroll:true});}}>{command}</button>)}</div><label className="command-picker"><select aria-label="常用命令" disabled={!connected} value="" onChange={e=>{setInput(e.target.value);terminalInput.current?.focus({preventScroll:true});}}><option value="" disabled>常用命令</option>{["help","pwd","ls -l","uname -a","whoami"].map(command=><option key={command}>{command}</option>)}</select><ChevronDown size={13}/></label><div className="shell-statusbar"><span className="cursor-position">行 {(session?.lines.length ?? 0)+1}, 列 {input.length+1}</span><span>UTF-8</span><span className={connected ? "connected-label" : ""}><i className={connected ? "connected" : ""}/>{connected ? `${current} 已连接` : "未连接"}<small>Mock</small></span><span className="session-count">{sessionCount} 个会话</span></div></div>
      </section>
      <aside className={`remote-files ${dragging ? "is-dragging" : ""}`} aria-label="远程文件管理" onDragOver={e=>{e.preventDefault();if(enabled)setDragging(true);}} onDragLeave={e=>{if(!e.currentTarget.contains(e.relatedTarget as Node))setDragging(false);}} onDrop={e=>{e.preventDefault();setDragging(false);uploadFiles(e.dataTransfer.files);}}>
        <div className="file-panel-heading"><h2><Folder size={19}/>文件管理</h2><div className="file-menu-anchor"><button className="icon" aria-label="文件管理工具" aria-expanded={menu === "files"} onClick={()=>setMenu(menu === "files" ? null : "files")}><Settings size={18}/></button>{menu === "files" && <div className="maintenance-menu"><button disabled={!session} onClick={()=>{setMenu(null);if(session)goTo(session.cwd);}}><LocateFixed size={14}/>定位到终端目录</button><button onClick={()=>{setMenu(null);setPreview({name:"传输记录",text:transfers.length ? transfers.map(t=>`${t.direction === "upload" ? "↑" : "↓"} ${t.name}\n${t.label} · ${t.size}`).join("\n\n") : "暂无传输记录。上传或下载后，记录会显示在这里。"});}}><History size={14}/>传输记录<span>{transfers.length}</span></button></div>}</div></div>
        <div className="file-actionbar"><button className="primary" disabled={!enabled} onClick={()=>uploadInput.current?.click()}><ArrowUpFromLine size={16}/>上传文件</button><button disabled={!downloadEntries.length || !enabled} onClick={downloadFile}><ArrowDownToLine size={16}/>下载{checked.length>1 ? ` (${downloadEntries.length})` : ""}</button><button className="icon" aria-label="刷新远程文件列表" onClick={()=>notify("演示文件列表已刷新")}><RefreshCw size={16}/></button><input ref={uploadInput} aria-label="选择上传文件" type="file" multiple hidden onChange={e=>{if(e.target.files)uploadFiles(e.target.files);e.target.value="";}}/></div>
        <form className="file-path" onSubmit={e=>{e.preventDefault();goTo(pathInput);}}><button type="button" aria-label="返回上级目录" disabled={directory === "/"} onClick={()=>goTo("..")}><ArrowLeft size={16}/></button><Folder size={15}/><input aria-label="当前文件目录" value={pathInput} onChange={e=>setPathInput(e.target.value)}/><button type="button" aria-label="复制当前目录" onClick={()=>{void navigator.clipboard.writeText(directory).then(()=>notify("目录路径已复制"),()=>notify("复制未成功"));}}><Copy size={14}/></button></form>
        <div className="directory-shortcuts">{["/","/etc/config","/tmp","/var/log"].map(path=><button key={path} className={directory === path ? "selected" : ""} onClick={()=>goTo(path)}>{path === "/" ? "全部" : path}</button>)}</div>
        <label className="file-search"><Search size={15}/><input aria-label="搜索当前目录" placeholder="搜索当前目录..." value={fileQuery} onChange={e=>{setFileQuery(e.target.value);setFilePage(0);}}/></label>
        <div className="remote-file-list"><div className="file-columns"><input type="checkbox" aria-label="选择本页全部文件" checked={!!pageEntries.length && pageEntries.every(e=>checked.includes(e.name))} onChange={e=>setChecked(e.target.checked ? [...new Set([...checked,...pageEntries.map(entry=>entry.name)])] : checked.filter(name=>!pageEntries.some(entry=>entry.name===name)))}/><span>名称</span><span>大小</span><span>修改时间</span><span/></div>
          {pageEntries.map(entry=><div key={entry.name} className={`remote-file-row ${entry.name===selected ? "selected" : ""}`}><input type="checkbox" aria-label={`选择 ${entry.name}`} checked={checked.includes(entry.name)} onChange={e=>{setSelected(entry.name);setChecked(e.target.checked ? [...checked,entry.name] : checked.filter(name=>name!==entry.name));}}/><button className={`file-name ${entry.folder ? "folder" : ""}`} onClick={()=>setSelected(entry.name)} onDoubleClick={()=>inspectFile(entry)} onKeyDown={e=>{if(e.key==="Enter"){e.preventDefault();inspectFile(entry);}}} title={entry.folder ? `双击打开 ${entry.name}` : `双击预览 ${entry.name}`}>{entry.folder ? <Folder size={16}/> : <FileCode2 size={16}/>}<span>{entry.name}</span></button><span>{formatSize(entry)}</span><time>{entry.modified}</time><button className="file-row-more" aria-label={`${entry.folder ? "打开" : "预览"} ${entry.name}`} onClick={()=>inspectFile(entry)}><EllipsisVertical size={16}/></button></div>)}
          {!entries.length && <div className="files-empty"><FolderOpen size={29}/><p>{fileQuery ? "没有匹配的文件" : "这个目录暂时为空"}</p></div>}
        </div>
        <div className="file-selection-info"><span>共 {entries.length} 个项目{checked.length ? ` · 已选 ${checked.length}` : ""}</span><div className="file-pagination"><button aria-label="文件上一页" disabled={visiblePage === 0} onClick={()=>setFilePage(visiblePage-1)}><ChevronLeft size={13}/></button><span>{visiblePage+1}</span><button aria-label="文件下一页" disabled={visiblePage+1 >= pageCount} onClick={()=>setFilePage(visiblePage+1)}><ChevronRight size={13}/></button></div></div>
        <button className="file-dropzone" disabled={!enabled} onClick={()=>uploadInput.current?.click()}><UploadCloud size={29}/><span>拖拽文件到这里上传<small>也可以点击选择文件</small></span></button>
        <div className="file-storage"><HardDrive size={14}/><span>设备存储</span><i><b/></i><span>26.2%</span><span>已使用 3.1 GB / 12 GB</span></div>
        {dragging && <div className="drop-overlay"><UploadCloud size={38}/><strong>松开以加入演示目录</strong><span>{directory}</span></div>}
      </aside>
    </div>
    <dialog ref={previewDialog} className="remote-file-preview" onCancel={()=>setPreview(null)}><div className="dialog-header"><h2><File size={17}/>{preview?.name}</h2><button className="icon" aria-label="关闭文件预览" onClick={()=>setPreview(null)}><X size={18}/></button></div><pre>{preview?.text}</pre></dialog>
  </section>;
}