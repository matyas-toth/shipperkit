import { For, Show, createEffect, createMemo, createSignal, on } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import { Button, ErrorNotice, Status } from "./Controls";
import { Icon } from "./Icon";
import { statusLabel, createdDate, deploymentRef, deploymentLabel, operationTime, formatDuration, bytes, logDateTime } from "../data";
import type { Deployment } from "../types";

export interface Observation {
  stale?:boolean; id: string; kind: string; state: string; summary: string; reference: string; observedAt: number;
  details: { cpuPercent?: number; memoryBytes?: number; memoryLimit?: number; exitCode?: number; restarts?: number; oomKilled?: boolean; service?: string; free?: number; total?: number; scope?: string };
}
export interface Diagnostic {
  servingRelease?: Deployment; latestOperation?: Deployment; failedStage: string; durationSeconds: number;
  checks: { layer: string; state: string; summary: string; observedAt: number }[];
  observations: Observation[]; changesSinceServing: string[]; unknowns: string[];
}
const checkNames: Record<string, string> = {
  daemon: "Docker", runtime: "Runtime", route: "Routing", certificate: "HTTPS",
  build: "Build", readiness: "Readiness", task: "Task", "dependency-network": "Dependency network",
};
const attention = new Set(["failed", "unreachable", "degraded", "warning", "unhealthy"]);
export function applicationDiagnostics(id: () => string, enabled: () => boolean) {
  return createQuery(() => ({ queryKey: ["applications", id(), "diagnostics"], queryFn: () => api<Diagnostic>(`/applications/${id()}/diagnostics`), enabled: enabled(), staleTime: 15000, refetchInterval: 15000 }));
}

export function diagnosticIssue(data?: Diagnostic) {
  const checks = data?.checks.filter(check => attention.has(check.state)) ?? [];
  return checks.find(check => check.layer === "daemon")
    ?? checks.find(check => check.layer === "route" || check.layer === "certificate")
    ?? checks[0];
}

export function ApplicationIssue(props: { diagnostic?: Diagnostic; domains: (hostname: string) => void; details: () => void; view: (id: string) => void }) {
  const issue = () => diagnosticIssue(props.diagnostic);
  const failed = () => props.diagnostic?.latestOperation?.state === "failed" ? props.diagnostic.latestOperation : undefined;
  const route = () => ["route", "certificate"].includes(issue()?.layer ?? "");
  const issueTitle = () => issue() && (checkNames[issue()!.layer] ?? issue()!.layer) +
    (route() ? " " + statusLabel(issue()!.state).toLowerCase() : " needs attention");
  return <Show when={issue() || failed()}>
    <section class="diagnostic-alert application-issue" aria-label="Application issue">
      <Icon name="warning" />
      <div class="application-issue-body">
        <strong>{issue() ? issueTitle() : "Deployment failed"}</strong>
        <span>{issue()?.summary || failed()?.error || props.diagnostic?.failedStage}</span>
      </div>
      <Button small onClick={() => route() ? props.domains(issue()!.summary) : !issue() && failed() ? props.view(failed()!.id) : props.details()}>
        {route() ? "Inspect route" : !issue() && failed() ? "View deployment" : "View diagnostics"}
      </Button>
    </section>
  </Show>;
}

export default function Diagnostics(props: { applicationId: string; query: ReturnType<typeof applicationDiagnostics>; focused?: boolean; view: (id: string) => void }) {
  const query = props.query;
  const incidents = createQuery(() => ({ queryKey: ["applications", props.applicationId, "incidents"], queryFn: () => api<{ items: { sequence: number; state: string; summary: string; createdAt: number }[] }>(`/applications/${props.applicationId}/incidents`), refetchInterval: 15000 }));
  const checks = createMemo(() => [...(query.data?.checks ?? [])].sort((a, b) => Number(attention.has(b.state)) - Number(attention.has(a.state))));
  const checksHealthy = createMemo(() => checks().length > 0 && checks().every(check => !attention.has(check.state) && check.state !== "unknown"));
  const [showChecks, setShowChecks] = createSignal(false);
  const checksExpanded = () => showChecks() || !checksHealthy();
  const lastChecked = () => Math.max(0, ...checks().map(check => check.observedAt));
  let section!: HTMLElement;
  createEffect(on(() => !!props.focused && !!query.data, ready => {
    if (ready) queueMicrotask(() => section.scrollIntoView({ block: "start" }));
  }));
  return <section ref={section} class="diagnostics-section">
    <div class="section-toolbar"><h2>Diagnostics</h2><Button small variant="ghost" icon="refresh" disabled={query.isFetching} onClick={() => void query.refetch()}>Recheck</Button></div>
    <Show when={query.error}><ErrorNotice message={query.error!.message} /></Show>
    <Show when={query.data}>{data => <>
      <div class="diagnostic-strip" classList={{ "has-issue": !checksHealthy() }}>
        <div class="diagnostic-health">
          <span class="diagnostic-health-icon"><Icon name={checksHealthy() ? "check" : "warning"} /></span>
          <div class="diagnostic-health-copy">
            <strong>{checksHealthy() ? "No issues detected" : diagnosticIssue(data()) ? (checkNames[diagnosticIssue(data())!.layer] ?? diagnosticIssue(data())!.layer) + " needs attention" : "Checks need attention"}</strong>
            <span>{checks().length} {checks().length === 1 ? "check" : "checks"}</span>
          </div>
        </div>
        <dl class="diagnostic-facts">
          <div><dt>Release</dt><dd><Show when={data().servingRelease} fallback="None">{release =>
            <button class="inline-link diagnostic-operation mono" title={release().commit || release().id} onClick={() => props.view(release().id)}>{deploymentRef(release())}</button>
          }</Show></dd></div>
          <div><dt>Operation</dt><dd><Show when={data().latestOperation} fallback="None">{d =>
            <button class="inline-link diagnostic-operation" onClick={() => props.view(d().id)}>
              <span>{deploymentLabel(d())}</span><span class="mono">{formatDuration(data().durationSeconds)}</span>
            </button>
          }</Show></dd></div>
          <div><dt>Checks</dt><dd>{checksHealthy() ? checks().length + " healthy" : checks().filter(check => attention.has(check.state)).length + " need attention"}</dd></div>
          <Show when={lastChecked()}><div><dt>Updated</dt><dd>{operationTime(lastChecked())}</dd></div></Show>
        </dl>
        <Show when={checks().length}><Button small variant="ghost" onClick={() => setShowChecks(value => !value)}>{checksExpanded() ? "Hide checks" : "Show checks"}</Button></Show>
      </div>
      <Show when={checks().length && checksExpanded()}><div class="table-tray diagnostic-checks-tray"><table class="inventory-table diagnostic-table">
        <thead><tr><th>Check</th><th>Status</th><th>Last checked</th></tr></thead>
        <tbody><For each={checks()}>{check =>
          <tr classList={{ "check-attention": attention.has(check.state) }}>
            <td><span class="diagnostic-layer">{checkNames[check.layer] ?? check.layer}</span>
              <Show when={["route", "certificate", "dependency-network", "task"].includes(check.layer)}><span class="mono diagnostic-reference">{check.summary}</span></Show>
            </td>
            <td><Status value={check.state} label={check.state === "unknown" ? "Not checked" : undefined} /></td>
            <td><Show when={check.observedAt} fallback="Not checked">{observed =>
              <time class="operator-time" datetime={new Date(observed() * 1000).toISOString()} title={operationTime(observed(), true)}>{operationTime(observed())}</time>
            }</Show></td>
          </tr>
        }</For></tbody>
      </table></div></Show>
      <Show when={data().observations.some(v=>v.kind==="service")}><div class="table-tray resource-table"><table class="inventory-table"><thead><tr><th>Service</th><th>CPU</th><th>Memory</th><th>Restarts</th><th>Exit</th></tr></thead><tbody><For each={data().observations.filter(v=>v.kind==="service")}>{v=><tr><td>{v.details.service||"Application"}</td><td>{v.details.cpuPercent==null?"Unobserved":v.details.cpuPercent.toFixed(1)+"%"}</td><td>{v.details.memoryBytes==null?"Unobserved":bytes(v.details.memoryBytes)}</td><td>{v.details.restarts??"Unobserved"}</td><td>{v.details.oomKilled?"OOM":v.details.exitCode??"None"}</td></tr>}</For></tbody></table></div></Show>
      <Show when={data().changesSinceServing.length}><details class="configuration"><summary>Pending changes</summary><ul><For each={data().changesSinceServing}>{change => <li>{change}</li>}</For></ul></details></Show>
      <details class="configuration"><summary>Evidence</summary><div class="table-tray"><table class="inventory-table evidence-table"><thead><tr><th>Observation</th><th>Status</th><th>Time</th></tr></thead><tbody><For each={data().observations}>{fact => <tr><td>{fact.summary}</td><td><Status value={fact.stale?"stale":fact.state} /></td><td>{logDateTime(fact.observedAt)}</td></tr>}</For><For each={data().unknowns}>{unknown => <tr class="evidence-unknown"><td>{unknown}</td><td><Status value="unknown" label="Not measured" /></td><td>—</td></tr>}</For></tbody></table></div></details>
    </>}</Show>
    <Show when={incidents.data?.items.length}><details class="configuration"><summary>Activity</summary><div class="incident-list"><For each={[...(incidents.data?.items ?? [])].reverse()}>{event => <div class="incident-row"><Status value={event.state} /><span>{event.summary}</span><time>{createdDate(event.createdAt)}</time></div>}</For></div></details></Show>
  </section>;
}
