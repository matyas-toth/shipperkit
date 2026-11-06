import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { For, Show, createEffect, createSignal, createUniqueId, onCleanup } from "solid-js";
import { api, post } from "../api";
import { useAuth } from "../context";
import type { Application, Deployment, SavedTask, Service, TaskRun } from "../types";
import { formatDuration, operationTime } from "../data";
import { Button, ErrorNotice, IconButton, Status } from "./Controls";
import { CommandFields, commandStageLabel, emptyCommand } from "./Commands";
import { Icon } from "./Icon";
import EditorActions from "./EditorActions";
import SchedulePicker from "./SchedulePicker";
import { describeSchedule } from "../schedule";
export default function Tasks(props:{application:Application;services:Service[];busy:boolean;view:(id:string)=>void;deployments:Deployment[];editorScope?:(name?:string)=>void}){
 const formId = createUniqueId(), auth=useAuth(),client=useQueryClient(),base=()=>`/applications/${props.application.id}`;
 const query=createQuery(()=>({queryKey:["applications",props.application.id,"tasks"],queryFn:()=>api<{items:SavedTask[];runs:TaskRun[]}>(base()+"/tasks"),refetchInterval:3000}));
 const [editing,setEditing]=createSignal<SavedTask>(),[error,setError]=createSignal(""),[saving,setSaving]=createSignal(false),[now,setNow]=createSignal(Date.now()/1000);
 createEffect(()=>props.editorScope?.(editing() ? editing()?.id ? "Edit task" : "New task" : undefined));
 onCleanup(()=>props.editorScope?.());
 const timer=setInterval(()=>setNow(Date.now()/1000),1000);onCleanup(()=>clearInterval(timer));
 const update=(v:Partial<SavedTask>)=>{setEditing(t=>t?{...t,...v}:t);};
 function newTask(){setError("");setEditing({id:"",applicationId:props.application.id,name:"",command:{...emptyCommand(),service:props.application.method==="compose"?props.services[0]?.name??"":""},enabled:true,schedule:"",timezone:Intl.DateTimeFormat().resolvedOptions().timeZone||"UTC",overlap:"skip",missed:"skip",nextAt:0});}
 async function refresh(){await client.invalidateQueries({queryKey:["applications",props.application.id]});}
 async function save(e:SubmitEvent){e.preventDefault();if(saving())return;setSaving(true);setError("");try{await post(base()+"/tasks",editing(),auth.session()?.csrfToken);setEditing(undefined);await refresh();}catch(e){setError((e as Error).message);}finally{setSaving(false);}}
 async function run(t:SavedTask){setSaving(true);setError("");try{const d=await post<Deployment>(base()+`/tasks/${t.id}/run`,{},auth.session()?.csrfToken);await refresh();props.view(d.id);}catch(e){setError((e as Error).message);}finally{setSaving(false);}}
 async function remove(){const t=editing();if(!t?.id)return;setSaving(true);setError("");try{await post(base()+`/tasks/${t.id}/delete`,{},auth.session()?.csrfToken);setEditing(undefined);await refresh();}catch(e){setError((e as Error).message);}finally{setSaving(false);}}

 const when=(seconds:number,zone?:string)=>new Date(seconds*1000).toLocaleString(undefined,{month:"short",day:"numeric",hour:"2-digit",minute:"2-digit",...(zone?{timeZone:zone}:{})});
 const runName=(r:TaskRun)=>{const d=props.deployments.find(d=>d.id===r.deploymentId);return d?.taskId?query.data?.items.find(t=>t.id===d.taskId)?.name??"Task":commandStageLabel(r.stage).replace(":"," · ");};
 return <section class="tasks-panel"><Show when={error()||query.error}><ErrorNotice message={error()||query.error!.message}/></Show>
 <Show when={editing()} fallback={<><div class="section-toolbar"><h2>Tasks</h2><Button variant="primary" icon="plus" onClick={newTask}>New task</Button></div><div class="table-tray"><table class="inventory-table tasks-table task-definitions"><thead><tr><th>Name</th><th>Schedule</th><th>Next run</th><th aria-label="Run"/></tr></thead><tbody><For each={query.data?.items}>{t=><tr><td><button class="task-name" onClick={()=>{setError("");setEditing(JSON.parse(JSON.stringify(t)));}}>{t.name}<Show when={!t.enabled}><span class="muted">Disabled</span></Show></button></td><td title={t.schedule || undefined}>{describeSchedule(t.schedule)}</td><td class="task-next"><Show when={t.enabled && t.schedule && t.nextAt}><time datetime={new Date(t.nextAt * 1000).toISOString()} title={when(t.nextAt, t.timezone) + " " + t.timezone}>{when(t.nextAt, t.timezone)}</time></Show></td><td class="task-actions"><Button icon="play" variant="ghost" disabled={props.busy||saving()||!t.enabled||!props.application.activeDeploymentId||props.application.desiredState==="stopped"} onClick={()=>void run(t)}>Run now</Button></td></tr>}</For><Show when={!query.data?.items.length}><tr class="empty-row"><td colspan="4">No tasks</td></tr></Show></tbody></table></div></>}>{task=><form id={formId} class="task-editor" onSubmit={save}><div class="section-toolbar"><h2>{task().id?"Edit task":"New task"}</h2><div class="editor-heading-actions"><label class="check-field"><input type="checkbox" checked={task().enabled} onChange={e=>update({enabled:e.currentTarget.checked})}/>Enabled</label><IconButton icon="close" label="Close task" ghost onClick={()=>setEditing(undefined)}/></div></div><div class="operational-editor-grid"><div class="task-definition"><div class="form-fields"><label for="task-name">Name</label><input id="task-name" class="input" required maxlength="80" value={task().name} onInput={e=>update({name:e.currentTarget.value})}/></div><CommandFields value={task().command} services={props.application.method==="compose"?props.services.map(s=>s.name):undefined} execution change={command=>update({command})}/></div>
 <div class="task-schedule"><h3>Schedule</h3><SchedulePicker value={task().schedule} timezone={task().timezone} change={(schedule,timezone)=>update({schedule,timezone})}/><Show when={task().schedule}><details class="configuration"><summary>Schedule options<Icon name="down"/></summary><div class="form-fields configuration-fields"><label for="task-overlap">When busy</label><select id="task-overlap" class="input" value={task().overlap} onChange={e=>update({overlap:e.currentTarget.value as SavedTask["overlap"]})}><option value="skip">Skip this run</option><option value="queue">Wait for the application</option></select><label for="task-missed">After downtime</label><select id="task-missed" class="input" value={task().missed} onChange={e=>update({missed:e.currentTarget.value as SavedTask["missed"]})}><option value="skip">Skip missed runs</option><option value="once">Run once</option></select></div></details></Show></div></div>
 <EditorActions measure="form" inlineWhenFits><Show when={task().id}><Button icon="trash" variant="ghost" disabled={saving()} onClick={()=>void remove()}>Delete task</Button></Show><Button variant="ghost" onClick={()=>setEditing(undefined)}>Cancel</Button><Button type="submit" form={formId} variant="primary" disabled={saving()}>{saving()?"Saving":"Save task"}</Button></EditorActions></form>}</Show>
 <Show when={query.data?.runs.length}>
   <section class="task-history"><h2>Run history</h2><div class="table-tray">
     <table class="inventory-table tasks-table task-runs">
       <thead><tr><th>Task</th><th>Status</th><th>Started</th><th>Duration</th><th aria-label="Actions" /></tr></thead>
       <tbody><For each={query.data?.runs}>{r =>
         <tr class="task-run-row">
           <td><strong class="task-run-name">{runName(r)}</strong>
             <Show when={r.state === "failed" && r.error}><span class="task-run-error">{r.error}</span></Show>
           </td>
           <td><Status value={r.state} label={r.state === "succeeded" ? "Completed" : undefined} /></td>
           <td class="task-start"><Show when={r.startedAt} fallback="Not started"><time datetime={new Date(r.startedAt * 1000).toISOString()} title={operationTime(r.startedAt, true)}>{operationTime(r.startedAt)}</time></Show></td>
           <td class="mono">{r.startedAt ? formatDuration(Math.max(0, (r.finishedAt || Math.floor(now())) - r.startedAt)) : ""}</td>
           <td><Button small icon="terminal" variant="ghost" aria-label={"View run " + r.id} onClick={() => props.view(r.deploymentId)}>View logs</Button></td>
         </tr>
       }</For></tbody>
     </table>
   </div></section>
 </Show></section>;
}
