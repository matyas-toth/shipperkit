import { For, Show, createSignal } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice, Status } from "./Controls";
import { bytes, createdDate } from "../data";

interface Plan { logLines:number; revision: string; images: { id: string; bytes: number }[]; sources: { id: string; bytes: number }[]; protected: string }
export default function Retention() {
  const auth=useAuth(),[open,setOpen]=createSignal(false),[busy,setBusy]=createSignal(false),[error,setError]=createSignal("");
  const query=createQuery(()=>({queryKey:["system","retention"],enabled:open(),queryFn:()=>api<{plan:Plan;automatic:boolean;runs:{id:string;state:string;createdAt:number;error:string}[]}>("/system/retention"),refetchInterval:open()?15000:false}));
  async function apply(body:unknown){setBusy(true);setError("");try{await post("/system/retention",body,auth.session()?.csrfToken);await query.refetch()}catch(e){setError((e as Error).message)}finally{setBusy(false)}}
  return <section class="settings-section"><div class="section-toolbar"><h2>Retention</h2><Button small variant="ghost" disabled={busy()} onClick={()=>{setOpen(true);void query.refetch()}}>Review cleanup</Button></div>
    <Show when={error()||query.error}><ErrorNotice message={error()||query.error!.message}/></Show>
    <Show when={open()&&query.data}>{data=><>
      <dl class="settings-group"><div class="setting-row"><dt>Eligible images</dt><dd>{data().plan.images.length}</dd></div><div class="setting-row"><dt>Abandoned uploads</dt><dd>{data().plan.sources.length}</dd></div><div class="setting-row"><dt>Older deployment log lines</dt><dd>{data().plan.logLines}</dd></div><div class="setting-row"><dt>Maximum reclaim</dt><dd>{bytes([...data().plan.images,...data().plan.sources].reduce((n,v)=>n+v.bytes,0))}</dd></div><div class="setting-row"><dt>Preserved</dt><dd>{data().plan.protected}</dd></div></dl>
      <details class="configuration"><summary>Cleanup candidates</summary><For each={[...data().plan.images,...data().plan.sources]}>{v=><div class="setting-row"><span class="mono truncate">{v.id}</span><span>{bytes(v.bytes)}</span></div>}</For></details>
      <div class="form-actions"><label class="check-field"><input type="checkbox" disabled={busy()} checked={data().automatic} onChange={e=>void apply({automatic:e.currentTarget.checked})}/>Daily eligible cleanup</label><Button disabled={busy()||!data().plan.images.length&&!data().plan.sources.length&&!data().plan.logLines} onClick={()=>void apply({revision:data().plan.revision})}>Clean reviewed items</Button></div>
      <For each={data().runs}>{run=><div class="incident-row"><Status value={run.state}/><time>{createdDate(run.createdAt)}</time><span>{run.error}</span></div>}</For>
    </>}</Show>
  </section>;
}
