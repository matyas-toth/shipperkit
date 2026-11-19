import { For, Show, createEffect, createSignal, onMount } from "solid-js";
import { A, useLocation, useNavigate } from "@solidjs/router";
import { createQuery } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import { checkName, type Observation } from "../components/Diagnostics";
import { Button, ErrorNotice, IconButton, Status } from "../components/Controls";
import { bytes, operationTime } from "../data";
import Retention from "../components/Retention";
import ChangeReview from "../components/ChangeReview";
import { MetricChart } from "../components/MetricChart";
import type { ChangePlan, SystemInfo, SystemOverview } from "../types";

interface HostStatus { observedAt: number; stale: boolean; dockerAtBoot: boolean; shipperAtBoot: boolean; rebootRequired: boolean; securityServiceEnabled: boolean; clockSynchronized: boolean }
// Notifications and access tokens moved to Settings; keep their old links working.
const movedToSettings = new Set(["notifications", "tokens"]);

export default function System() {
  const auth = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  createEffect(() => { const hash = location.hash.slice(1); if (movedToSettings.has(hash)) navigate("/settings#" + hash, { replace: true }); });
  const [repair, setRepair] = createSignal<ChangePlan>(), [cleanup, setCleanup] = createSignal(false);
  const system = createQuery(() => ({ queryKey: ["system"], queryFn: () => api<SystemInfo>("/system"), staleTime: 30_000 }));
  const overview = createQuery(() => ({ queryKey: ["system", "overview"], queryFn: () => api<SystemOverview>("/system/overview"), refetchInterval: 30000 }));
  const [error, setError] = createSignal(""), [busy, setBusy] = createSignal(false);
  const [inspected, setInspected] = createSignal<Observation>();
  const query = createQuery(() => ({ queryKey: ["system", "operations"], queryFn: () => api<{ items: Observation[]; host?: HostStatus; notifications: { pending: number; failed: number } }>("/system/operations"), refetchInterval: 15000 }));
  onMount(() => {
    document.title = "System | Shipper";
    if (location.hash === "#maintenance") requestAnimationFrame(() => document.getElementById("maintenance")?.scrollIntoView({ block: "start" }));
  });
  async function action(work: () => Promise<unknown>) {
    if (busy()) return; setBusy(true); setError("");
    try { await work(); } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  const instance = () => {
    const data = system.data, docker = data?.docker;
    return [
      { title: "Shipper", rows: [["Version", data?.version], ["Go", data?.runtime.go], ["Heap", data && bytes(data.runtime.heapBytes)]] },
      { title: "Docker", rows: [["Connection", docker && (docker.connected ? "Connected" : "Disconnected")], ["Host", docker?.engine.name], ["Engine", docker?.engine.version],
        ["Operating system", docker?.engine.os], ["Architecture", docker?.engine.architecture], ["CPUs", docker && String(docker.engine.cpus ?? 0)], ["Memory", docker && bytes(docker.engine.memory ?? 0)]] },
      { title: "Database", rows: [["Engine", data && "SQLite"], ["Journal mode", data?.database.journal], ["Applications", data && String(data.database.counts.applications ?? 0)], ["Containers", data && String(data.database.counts.containers ?? 0)]] },
    ];
  };
  const hostChecks = () => {
    const host = query.data?.host;
    if (!host) return [];
    const row = (name: string, ok: boolean, good: string, bad: string, badState = "warning") =>
      ({ name, state: host.stale ? "stale" : ok ? "healthy" : badState, label: host.stale ? "Not observed" : ok ? good : bad });
    return [
      row("Boot services", host.dockerAtBoot && host.shipperAtBoot, "Ready", "Check Docker and Shipper"),
      row("Security updates", host.securityServiceEnabled, "Enabled", "Operator managed", "neutral"),
      row("Reboot", !host.rebootRequired, "Not required", "Required"),
      row("System clock", host.clockSynchronized, "Synchronized", "Not synchronized"),
    ];
  };
  const unmeasured = () => { const data = overview.data; return !!data && !data.measuring && data.cpu.percent == null && data.memory.percent == null && data.disk.percent == null; };
  return <div class="content content-wide system-page">
    <header class="page-heading"><h1>System</h1><Button icon="refresh" disabled={busy()} onClick={() => void action(async () => { await post("/system/recheck", {}, auth.session()?.csrfToken); await query.refetch(); })}>Recheck</Button></header>
    <Show when={error() || query.error}><ErrorNotice message={error() || query.error!.message} /></Show>
    <div class="system-overview">
      <section class="settings-section"><h2>Health</h2><div class="table-tray"><table class="inventory-table health-table"><thead><tr><th>Component</th><th>Status</th><th>Last checked</th><th aria-label="Details" /></tr></thead><tbody><For each={query.data?.items}>{fact => <tr><td>{checkName(fact.kind)}</td><td><Status value={fact.stale ? "stale" : fact.state} /></td><td>{fact.observedAt ? <time class="operator-time" datetime={new Date(fact.observedAt * 1000).toISOString()} title={operationTime(fact.observedAt, true)}>{operationTime(fact.observedAt)}</time> : "Not checked"}</td><td><Button small variant="ghost" aria-label={"Inspect " + checkName(fact.kind)} onClick={() => setInspected(fact)}>Inspect</Button></td></tr>}</For>
        <For each={hostChecks()}>{check => <tr><td>{check.name}</td><td><Status value={check.state} label={check.label} /></td><td>{query.data?.host && !query.data.host.stale ? <time class="operator-time" datetime={new Date(query.data.host.observedAt * 1000).toISOString()} title={operationTime(query.data.host.observedAt, true)}>{operationTime(query.data.host.observedAt)}</time> : "Not checked"}</td><td /></tr>}</For></tbody></table></div>
        <Show when={inspected()}>{fact => <section class="health-detail" aria-label={checkName(fact().kind) + " details"}>
          <div class="section-toolbar"><h3>{checkName(fact().kind)}</h3><IconButton icon="close" label="Close health details" ghost onClick={() => setInspected(undefined)} /></div>
          <p>{fact().summary}</p>
          <Show when={fact().reference}><p class="mono">{fact().reference}</p></Show>
          <div class="toolbar-actions">
            <Show when={fact().applicationId}><A class="button neutral" href={"/applications/" + encodeURIComponent(fact().applicationId!) + "?tab=" + (fact().kind === "certificate" ? "domains" : "overview")}>{fact().kind === "certificate" ? "Open domains" : "Open application"}</A></Show>
            <Show when={!fact().applicationId && ["proxy", "daemon", "host"].includes(fact().kind)}><Button onClick={() => document.getElementById("maintenance")?.scrollIntoView({ behavior: "smooth", block: "start" })}>Open maintenance</Button></Show>
            <Show when={fact().kind === "certificate" && !fact().applicationId}><A class="button neutral" href="/settings#dashboard">Open HTTPS settings</A></Show>
          </div>
        </section>}</Show>
        <For each={query.data?.items.filter(v => v.kind === "storage")}>{fact => <dl class="settings-group"><div class="setting-row"><dt>{fact.details.scope}</dt><dd>{bytes(fact.details.free ?? 0)} free of {bytes(fact.details.total ?? 0)}</dd></div></dl>}</For>
      </section>
      <section class="settings-section"><h2>Capacity</h2>
        <Show when={overview.data} fallback={<span class="loading-line" aria-label="Loading host capacity" />}>{data => <Show when={!unmeasured()} fallback={<dl class="settings-group"><div class="setting-row"><dt>CPU, memory and disk</dt><dd>Not measured</dd></div></dl>}>
          <div class="capacity-charts">
            <MetricChart label="CPU" value={data().measuring ? "Measuring" : data().cpu.percent == null ? "Not measured" : Math.round(data().cpu.percent!) + "%"} state={data().thresholds.cpu} values={data().series.cpu} />
            <MetricChart label="Memory" value={data().memory.percent == null ? "Not measured" : Math.round(data().memory.percent!) + "%"} state={data().thresholds.memory} values={data().series.memory} detail={data().memory.total ? `${bytes(data().memory.used ?? 0)} of ${bytes(data().memory.total!)}` : undefined} />
            <MetricChart label="Disk" value={data().disk.percent == null ? "Not measured" : Math.round(data().disk.percent!) + "%"} state={data().thresholds.disk} values={data().series.disk} detail={data().disk.total ? `${bytes(data().disk.used ?? 0)} of ${bytes(data().disk.total!)}` : undefined} />
          </div>
        </Show>}</Show>
      </section>
      <section class="settings-section instance-section"><h2>Instance</h2>
        <Show when={system.error}><ErrorNotice message={system.error!.message} retry={() => void system.refetch()} /></Show>
        <div class="instance-groups"><For each={instance()}>{group => <div class="instance-group"><h3>{group.title}</h3><dl class="settings-group">
          <For each={group.rows}>{([label, value]) => <div class="setting-row"><dt>{label}</dt><dd>{system.isPending ? <span class="loading-line" /> : value || "Unavailable"}</dd></div>}</For>
        </dl></div>}</For></div>
      </section>
      <section class="settings-section" id="maintenance"><div class="section-toolbar"><h2>Maintenance</h2><div class="toolbar-actions">
        <Button icon="refresh" disabled={busy()} onClick={() => void action(async () => setRepair(await post<ChangePlan>("/system/repair", { review: true }, auth.session()?.csrfToken)))}>Repair proxy</Button>
        <Show when={!cleanup()}><Button onClick={() => setCleanup(true)}>Review cleanup</Button></Show>
      </div></div>
        <Show when={cleanup()}><Retention close={() => setCleanup(false)} /></Show>
      </section>
    </div>
    <Show when={repair()}>{plan => <ChangeReview plan={plan()} title="Repair proxy" confirm="Repair proxy" close={() => setRepair(undefined)} apply={async () => { await post("/system/repair", { revision: plan().revision }, auth.session()?.csrfToken); setRepair(undefined); await query.refetch(); }} />}</Show>
  </div>;
}
