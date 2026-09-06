import type { Connection } from "../connection";
import { segment } from "../api";
import type { ToolVersion, TaskDetail, Transfer, TaskSummary, Tool } from "../models";

// Metadata fan-out is bounded; each call belongs to the connection cancellation scope.
export async function mapBounded<T,R>(items:T[], fn:(value:T)=>Promise<R>):Promise<R[]> {
  let index=0;
  const values:R[]=new Array(items.length);
  await Promise.all(Array.from({length:Math.min(4,items.length)},async()=>{
    while(index<items.length){const i=index++;values[i]=await fn(items[i]);}
  }));
  return values;
}
export interface Operation {task_id:string;transfer_id:string;device_id:string;session_id:string;tool_id:string;version:string;artifact_id:string;asset_id:string}
export async function readRepository(c:Connection,tools:Tool[],tasks:TaskSummary[]) {
  const versions=await mapBounded(tools,t=>c.api.list<ToolVersion>(`tools/${segment(t.tool_id)}/versions?include_archived=true`));
  const transfers=await mapBounded(tasks.filter(t=>t.type!=='exec').slice(0,50),async t=>{
    const detail=await c.api.get<TaskDetail>('tasks/'+segment(t.task_id));
    const transfer=await c.api.get<Transfer>('tasks/'+segment(t.task_id)+'/transfer');
    const operation=await c.api.get<Operation>('tasks/'+segment(t.task_id)+'/operation');
    return {detail,transfer,operation};
  });
  return {versions:versions.flat(),transfers};
}
