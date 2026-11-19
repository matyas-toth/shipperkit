import { For, Show } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import { Button, Status } from "./Controls";
import { Icon } from "./Icon";
import { createdDate, statusLabel } from "../data";
import type { Deployment, DiagnosisAction, DiagnosisCause } from "../types";

export interface Observation {
  stale?:boolean; applicationId?: string; id: string; kind: string; state: string; summary: string; reference: string; observedAt: number;
  details: { cpuPercent?: number; memoryBytes?: number; memoryLimit?: number; exitCode?: number; restarts?: number; oomKilled?: boolean; service?: string; free?: number; total?: number; scope?: string };
}
export interface Diagnostic {
  servingRelease?: Deployment; latestOperation?: Deployment; failedStage: string; durationSeconds: number;
  checks: { layer: string; state: string; summary: string; observedAt: number }[];
  observations: Observation[]; changesSinceServing: string[]; unknowns: string[];
  cause?: DiagnosisCause; nextActions?: DiagnosisAction[];
}
const checkNames: Record<string, string> = {
  daemon: "Docker", runtime: "Runtime", route: "Routing", certificate: "HTTPS", proxy: "Proxy", storage: "Storage",
  host: "Host", service: "Service", build: "Build", readiness: "Readiness", task: "Task", "dependency-network": "Dependency network",
};
export const checkName = (kind: string) => checkNames[kind] ?? (kind ? kind[0].toUpperCase() + kind.slice(1).replaceAll("-", " ") : kind);
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

export function ApplicationIssue(props: { diagnostic?: Diagnostic; domains: (hostname: string) => void; deployments: () => void; view: (id: string) => void }) {
  const failed = () => props.diagnostic?.latestOperation?.state === "failed" ? props.diagnostic.latestOperation : undefined;
  // A failed deployment explains most runtime symptoms, so it leads unless Docker is down or an
  // older release still serves and its route needs attention on its own.
  const issue = () => {
    const found = diagnosticIssue(props.diagnostic);
    if (!failed() || !found || found.layer === "daemon") return found;
    return props.diagnostic?.servingRelease && ["route", "certificate"].includes(found.layer) ? found : undefined;
  };
  const route = () => ["route", "certificate"].includes(issue()?.layer ?? "");
  const issueTitle = () => issue() && checkName(issue()!.layer) +
    (route() ? " " + statusLabel(issue()!.state).toLowerCase() : " needs attention");
  return <Show when={issue() || failed()}>
    <section class="diagnostic-alert application-issue" aria-label="Application issue">
      <Icon name="warning" />
      <div class="application-issue-body">
        <strong>{issue() ? issueTitle() : "Deployment failed"}</strong>
        <span>{issue()?.summary || props.diagnostic?.cause?.summary || failed()?.error || props.diagnostic?.failedStage}</span>
      </div>
      <Button small onClick={() => route() ? props.domains(issue()!.summary) : !issue() && failed() ? props.view(failed()!.id) : props.deployments()}>
        {route() ? "Inspect route" : !issue() && failed() ? "View deployment" : "View deployments"}
      </Button>
    </section>
  </Show>;
}

export function ApplicationActivity(props: { applicationId: string }) {
  const incidents = createQuery(() => ({ queryKey: ["applications", props.applicationId, "incidents"], queryFn: () => api<{ items: { sequence: number; state: string; summary: string; createdAt: number }[] }>(`/applications/${props.applicationId}/incidents`), refetchInterval: 15000 }));
  return <Show when={incidents.data?.items.length}>
    <details class="configuration activity-disclosure"><summary>Activity</summary><div class="incident-list"><For each={[...(incidents.data?.items ?? [])].reverse()}>{event => <div class="incident-row"><Status value={event.state} /><span>{event.summary}</span><time>{createdDate(event.createdAt)}</time></div>}</For></div></details>
  </Show>;
}
