import { For, Show, createSignal } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice, IconButton, Status } from "./Controls";
import { bytes, createdDate } from "../data";

interface Plan { logLines:number; revision: string; images: { id: string; bytes: number }[]; sources: { id: string; bytes: number }[]; protected: string }
export default function Retention(props: { close: () => void }) {
  const auth=useAuth(),[busy,setBusy]=createSignal(false),[error,setError]=createSignal("");
  const query=createQuery(()=>({queryKey:["system","retention"],queryFn:()=>api<{plan:Plan;automatic:boolean;runs:{id:string;state:string;createdAt:number;error:string}[]}>("/system/retention"),refetchInterval:15000}));
  async function apply(body:unknown){setBusy(true);setError("");try{await post("/system/retention",body,auth.session()?.csrfToken);await query.refetch()}catch(e){setError((e as Error).message)}finally{setBusy(false)}}
  return <div class="retention-plan" aria-label="Cleanup">
    <div class="section-toolbar"><h3>Cleanup</h3><div class="toolbar-actions"><Button small variant="ghost" disabled={busy()} onClick={()=>void query.refetch()}>Refresh plan</Button><IconButton icon="close" label="Close cleanup" ghost onClick={props.close}/></div></div>
    <Show when={error()||query.error}><ErrorNotice message={error()||query.error!.message}/></Show>
    <Show when={query.data} fallback={<span class="loading-line" aria-label="Loading cleanup plan"/>}>{data=><>
      <dl class="settings-group"><div class="setting-row"><dt>Eligible images</dt><dd>{data().plan.images.length}</dd></div><div class="setting-row"><dt>Abandoned uploads</dt><dd>{data().plan.sources.length}</dd></div><div class="setting-row"><dt>Older deployment log lines</dt><dd>{data().plan.logLines}</dd></div><div class="setting-row"><dt>Maximum reclaim</dt><dd>{bytes([...data().plan.images,...data().plan.sources].reduce((n,v)=>n+v.bytes,0))}</dd></div><div class="setting-row"><dt>Preserved</dt><dd>{data().plan.protected}</dd></div></dl>
      <details class="configuration"><summary>Cleanup candidates</summary><For each={[...data().plan.images,...data().plan.sources]}>{v=><div class="setting-row"><span class="mono truncate">{v.id}</span><span>{bytes(v.bytes)}</span></div>}</For></details>
      <div class="form-actions"><label class="check-field"><input type="checkbox" disabled={busy()} checked={data().automatic} onChange={e=>void apply({automatic:e.currentTarget.checked})}/>Daily eligible cleanup</label><Button disabled={busy()||!data().plan.images.length&&!data().plan.sources.length&&!data().plan.logLines} onClick={()=>void apply({revision:data().plan.revision})}>Clean reviewed items</Button></div>
      <For each={data().runs}>{run=><div class="incident-row"><Status value={run.state} label={run.state==="succeeded"?"Completed":undefined}/><time>{createdDate(run.createdAt)}</time><span>{run.error}</span></div>}</For>
    </>}</Show>
  </div>;
}
