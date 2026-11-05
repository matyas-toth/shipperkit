import { For, Show } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import { Button, ErrorNotice, Status } from "./Controls";
import { createdDate, formatDuration, bytes, logDateTime } from "../data";
import type { Deployment } from "../types";
import { ReviewFact, ReviewPanel } from "./ReviewPanel";

export interface Observation {
  stale?:boolean; id: string; kind: string; state: string; summary: string; reference: string; observedAt: number;
  details: { cpuPercent?: number; memoryBytes?: number; memoryLimit?: number; exitCode?: number; restarts?: number; oomKilled?: boolean; service?: string; free?: number; total?: number; scope?: string };
}
interface Diagnostic {
  servingRelease?: Deployment; latestOperation?: Deployment; failedStage: string; durationSeconds: number;
  checks: { layer: string; state: string; summary: string; observedAt: number }[];
  observations: Observation[]; changesSinceServing: string[]; unknowns: string[];
}
export default function Diagnostics(props: { applicationId: string; view: (id: string) => void }) {
  const query = createQuery(() => ({ queryKey: ["applications", props.applicationId, "diagnostics"], queryFn: () => api<Diagnostic>(`/applications/${props.applicationId}/diagnostics`), staleTime: 15000, refetchInterval: 15000 }));
  const incidents = createQuery(() => ({ queryKey: ["applications", props.applicationId, "incidents"], queryFn: () => api<{ items: { sequence: number; state: string; summary: string; createdAt: number }[] }>(`/applications/${props.applicationId}/incidents`), refetchInterval: 15000 }));
  return <section class="diagnostics-section">
    <div class="section-toolbar"><h2>Diagnostics</h2><Button small variant="ghost" icon="refresh" disabled={query.isFetching} onClick={() => void query.refetch()}>Recheck</Button></div>
    <Show when={query.error}><ErrorNotice message={query.error!.message} /></Show>
    <Show when={query.data}>{data => <>
      <ReviewPanel title="Deployment">
        <dl>
          <ReviewFact label="Serving release"><code class="mono">{data().servingRelease?.id.slice(0, 12) || "None"}</code></ReviewFact>
          <ReviewFact label="Latest operation">
            <Show when={data().latestOperation} fallback="None">{d =>
              <button class="inline-link diagnostic-operation" onClick={() => props.view(d().id)}>
                <Status value={d().state} /><span class="mono">{formatDuration(data().durationSeconds)}</span>
              </button>}</Show>
          </ReviewFact>
          <Show when={data().latestOperation?.state === "failed"}><ReviewFact label="Failed stage">
            <Button small variant="ghost" class="diagnostic-operation" onClick={() => props.view(data().latestOperation!.id)}>{data().failedStage || "Open deployment logs"}</Button>
          </ReviewFact></Show>
        </dl>
      </ReviewPanel>
      <div class="table-tray"><table class="inventory-table diagnostic-table"><thead><tr><th>Check</th><th>Status</th><th>Observed</th></tr></thead><tbody>
        <For each={data().checks}>{check => <tr><td><span class="diagnostic-layer">{({daemon:"Docker",runtime:"Runtime",route:"Routing",certificate:"HTTPS",build:"Build",readiness:"Readiness",task:"Task","dependency-network":"Dependency network"} as Record<string,string>)[check.layer]||check.layer}</span><Show when={["route","certificate","dependency-network","task"].includes(check.layer)}><span class="mono">{check.summary}</span></Show></td><td><Status value={check.state} /></td><td>{check.observedAt ? logDateTime(check.observedAt) : "Not observed"}</td></tr>}</For>
      </tbody></table></div>
      <Show when={data().observations.some(v=>v.kind==="service")}><div class="table-tray resource-table"><table class="inventory-table"><thead><tr><th>Service</th><th>CPU</th><th>Memory</th><th>Restarts</th><th>Exit</th></tr></thead><tbody><For each={data().observations.filter(v=>v.kind==="service")}>{v=><tr><td>{v.details.service||"Application"}</td><td>{v.details.cpuPercent==null?"Unobserved":v.details.cpuPercent.toFixed(1)+"%"}</td><td>{v.details.memoryBytes==null?"Unobserved":bytes(v.details.memoryBytes)}</td><td>{v.details.restarts??"Unobserved"}</td><td>{v.details.oomKilled?"OOM":v.details.exitCode??"None"}</td></tr>}</For></tbody></table></div></Show>
      <Show when={data().changesSinceServing.length}><details class="configuration"><summary>Pending changes</summary><ul><For each={data().changesSinceServing}>{change => <li>{change}</li>}</For></ul></details></Show>
      <details class="configuration"><summary>Evidence</summary><div class="table-tray"><table class="inventory-table"><thead><tr><th>Observation</th><th>Status</th><th>Time</th></tr></thead><tbody><For each={data().observations}>{fact => <tr><td>{fact.summary}</td><td><Status value={fact.stale?"stale":fact.state} /></td><td>{logDateTime(fact.observedAt)}</td></tr>}</For></tbody></table></div><For each={data().unknowns}>{unknown => <div class="setting-row">{unknown}</div>}</For></details>
    </>}</Show>
    <Show when={incidents.data?.items.length}><details class="configuration"><summary>Activity</summary><div class="incident-list"><For each={[...(incidents.data?.items ?? [])].reverse()}>{event => <div class="incident-row"><Status value={event.state} /><span>{event.summary}</span><time>{createdDate(event.createdAt)}</time></div>}</For></div></details></Show>
  </section>;
}
