import { useEffect, useRef, useImperativeHandle, forwardRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { platform } from "../platform";
import { describe } from "../api";
import type { Workbench } from "../useWorkbench";

export interface ShellControl {clear():void;insert(text:string):void;copy():Promise<void>}
export const EmbeddedShell=forwardRef<ShellControl,{model:Workbench;protocol:'SSH'|'Telnet';fontSize:number;visible:boolean;onState:(open:boolean)=>void}>(function EmbeddedShell({model,protocol,fontSize,visible,onState},ref){
  const host=useRef<HTMLDivElement>(null),terminal=useRef<Terminal|null>(null),fit=useRef<FitAddon|null>(null);
  const handle=useRef(''),latest=useRef({model,onState});latest.current={model,onState};
  const input=useRef<(text:string)=>void>(()=>{});
  useImperativeHandle(ref,()=>({clear:()=>terminal.current?.clear(),insert:text=>{input.current(text);terminal.current?.focus();},copy:async()=>{const t=terminal.current;if(!t)return;await navigator.clipboard.writeText(t.getSelection()||Array.from({length:t.buffer.active.length},(_,i)=>t.buffer.active.getLine(i)?.translateToString(true)??'').join('\n'));}}),[]);
  useEffect(()=>{
    const t=new Terminal({fontSize,fontFamily:'Consolas, "Cascadia Mono", monospace',cursorBlink:true,scrollback:2000,allowProposedApi:false,theme:{background:'#122031',foreground:'#d9e5f4',cursor:'#d9e5f4',selectionBackground:'#315477'},convertEol:false});
    const addon=new FitAddon();t.loadAddon(addon);t.open(host.current!);terminal.current=t;fit.current=addon;
    let cancelled=false,timer:ReturnType<typeof setTimeout>|undefined,sending=false,queued='';
    const fail=(e:unknown)=>{if(!cancelled){latest.current.onState(false);latest.current.model.setError(describe(e));}};
    const send=async()=>{
      if(sending||!handle.current)return;sending=true;
      try{while(queued&&!cancelled){const end=queued.length>4000&&/[\uD800-\uDBFF]/.test(queued[3999])?3999:4000;const chunk=queued.slice(0,end);queued=queued.slice(chunk.length);await platform.terminalWrite(handle.current,chunk);}}
      catch(e){queued='';fail(e);}finally{sending=false;}
    };
    input.current=text=>{if(!handle.current||cancelled)return;if(queued.length+text.length>65536){fail(Error('终端输入过大，请分段粘贴。'));return;}queued+=text;void send();};
    const inputSubscription=t.onData(text=>input.current(text));
    let resizing=false,resizeAgain=false;
    const resize=async()=>{
      if(cancelled||!host.current?.clientWidth||!host.current.clientHeight)return;
      addon.fit();if(!handle.current)return;
      if(resizing){resizeAgain=true;return;}resizing=true;
      try{do{resizeAgain=false;await platform.terminalResize(handle.current,Math.min(500,t.cols),Math.min(300,t.rows));}while(resizeAgain&&!cancelled);}catch(e){fail(e);}finally{resizing=false;}
    };
    const observer=new ResizeObserver(()=>void resize());observer.observe(host.current!);
    const poll=async()=>{
      if(cancelled)return;
      try{
        const value=await platform.terminalRead(handle.current);if(cancelled)return;
        if(value.base64){const bytes=Uint8Array.from(atob(value.base64),c=>c.charCodeAt(0));await new Promise<void>(resolve=>t.write(bytes,resolve));}
        if(cancelled)return;
        if(value.exited){t.writeln(`\r\n[客户端已退出：${value.exit_code}]`);latest.current.onState(false);await platform.terminalClose(handle.current);handle.current='';return;}
        timer=setTimeout(()=>void poll(),value.base64?16:80);
      }catch(e){fail(e);}
    };
    void (async()=>{
      try{
        if(!platform.embeddedTerminal)throw Error('内置 Shell 需要 Windows 工作台。浏览器可查看真实业务数据；终端请在 Windows 客户端打开。');
        const m=latest.current.model,connection=m.owner.current,maintenance=m.active;
        if(!maintenance)throw Error('请先开启维护');
        const {endpoint,current}=await m.endpointFor(maintenance,protocol.toLowerCase());
        if(cancelled||connection!==m.owner.current)return;
        addon.fit();
        const id=await platform.terminalOpen(endpoint,current.expires_at,Math.max(2,Math.min(500,t.cols)),Math.max(2,Math.min(300,t.rows)));
        if(cancelled||connection!==m.owner.current){await platform.terminalClose(id);return;}
        handle.current=id;latest.current.onState(true);t.focus();void poll();
      }catch(e){if(!cancelled)t.writeln(describe(e));fail(e);}
    })();
    return()=>{cancelled=true;clearTimeout(timer);observer.disconnect();inputSubscription.dispose();queued='';const id=handle.current;handle.current='';if(id)void platform.terminalClose(id).catch(()=>{});t.dispose();terminal.current=null;fit.current=null;};
  },[protocol]);
  useEffect(()=>{if(terminal.current){terminal.current.options.fontSize=fontSize;if(visible&&host.current?.clientWidth)fit.current?.fit();}},[fontSize,visible]);
  return <div className="production-terminal-host" ref={host} aria-label={`${protocol} 终端输出`}/>;
});
