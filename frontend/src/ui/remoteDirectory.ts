import type { Connection } from "../connection";
import type { TaskDetail } from "../models";
import { segment } from "../api";

export type RemoteEntry={name:string;folder:boolean;size:number;modified:string};
export const shellQuote=(value:string)=>"'"+value.replaceAll("'","'\\''")+"'";
export function directoryCommand(path:string) {
  if(!path.startsWith('/')||path.includes('\0')||path.length>4096)throw Error('请输入有效的绝对目录路径');
  return `cd ${shellQuote(path)} || exit 1\nexport LC_ALL=C\nn=0\nfor f in .[!.]* ..?* *; do\n [ -e "./$f" ] || [ -L "./$f" ] || continue\n [ "$n" -lt 250 ] || { printf 'LIMIT\\0'; break; }\n kind=f; [ ! -d "./$f" ] || kind=d\n meta=$(stat -c '%s %Y' "./$f") || exit 2\n set -- $meta\n printf '%s\\0%s\\0%s\\0%s\\0' "$kind" "$1" "$2" "$f"\n n=$((n+1))\ndone`;
}
export function parseDirectory(output:string) {
  const fields=output.split('\0');if(fields.pop()!=='')throw Error('目录结果不完整');
  const limited=fields.at(-1)==='LIMIT';if(limited)fields.pop();
  if(fields.length%4)throw Error('设备返回了无法解析的目录格式');
  const entries:RemoteEntry[]=[];
  for(let i=0;i<fields.length;i+=4){const [kind,size,time,name]=fields.slice(i,i+4);if(!['f','d'].includes(kind)||!/^\d+$/.test(size)||!/^\d+$/.test(time)||!name||name.includes('/'))throw Error('设备目录字段无效');entries.push({name,folder:kind==='d',size:Number(size),modified:new Date(Number(time)*1000).toLocaleString('zh-CN',{hour12:false})});}
  return {entries,limited};
}
export async function waitTask(c:Connection,id:string,signal:AbortSignal) {
  const combined=AbortSignal.any([c.lifetime.signal,signal,AbortSignal.timeout(45000)]);
  while(true){
    combined.throwIfAborted();
    const task=await c.api.get<TaskDetail>('tasks/'+segment(id),combined);
    combined.throwIfAborted();
    if(!['received','queued','running'].includes(task.state))return task;
    await new Promise<void>((resolve,reject)=>{const abort=()=>{clearTimeout(timer);reject(combined.reason);};const timer=setTimeout(()=>{combined.removeEventListener('abort',abort);resolve();},600);combined.addEventListener('abort',abort,{once:true});if(combined.aborted)abort();});
  }
}
