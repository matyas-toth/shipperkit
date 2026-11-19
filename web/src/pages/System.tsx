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
const sections = [{ id: "overview", label: "Overview" }, { id: "maintenance", label: "Maintenance" }] as const;
// Notifications and access tokens moved to Settings; keep their old links working.
const movedToSettings = new Set(["notifications", "tokens"]);

export default function System() {
  const auth = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const section = () => sections.find(s => s.id === location.hash.slice(1))?.id ?? "overview";
  createEffect(() => { const hash = location.hash.slice(1); if (movedToSettings.has(hash)) navigate("/settings#" + hash, { replace: true }); });
  const [repair, setRepair] = createSignal<ChangePlan>();
  const system = createQuery(() => ({ queryKey: ["system"], queryFn: () => api<SystemInfo>("/system"), staleTime: 30_000 }));
  const overview = createQuery(() => ({ queryKey: ["system", "overview"], queryFn: () => api<SystemOverview>("/system/overview"), refetchInterval: 30000 }));
  const [error, setError] = createSignal(""), [busy, setBusy] = createSignal(false);
  const [inspected, setInspected] = createSignal<Observation>();
  const query = createQuery(() => ({ queryKey: ["system", "operations"], queryFn: () => api<{ items: Observation[]; host?: HostStatus; notifications: { pending: number; failed: number } }>("/system/operations"), refetchInterval: 15000 }));
  onMount(() => { document.title = "System | Shipper"; });
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
  const unmeasured = () => { const data = overview.data; return !!data && !data.measuring && data.cpu.percent == null && data.memory.percent == null && data.disk.percent == null; };
  return <div class="content content-wide system-page">
    <header class="page-heading"><h1>System</h1><Show when={section() === "overview"}><Button icon="refresh" disabled={busy()} onClick={() => void action(async () => { await post("/system/recheck", {}, auth.session()?.csrfToken); await query.refetch(); })}>Recheck</Button></Show></header>
    <nav class="section-navigation" aria-label="System sections">
      <For each={sections}>{item => <a href={"/system#" + item.id} aria-current={section() === item.id ? "page" : undefined}>{item.label}</a>}</For>
    </nav>
    <Show when={error() || query.error}><ErrorNotice message={error() || query.error!.message} /></Show>
    <Show when={section() === "overview"}><div class="system-overview">
      <section class="settings-section"><h2>Health</h2><div class="table-tray"><table class="inventory-table health-table"><thead><tr><th>Component</th><th>Status</th><th>Last checked</th><th aria-label="Details" /></tr></thead><tbody><For each={query.data?.items}>{fact => <tr><td>{checkName(fact.kind)}</td><td><Status value={fact.stale ? "stale" : fact.state} /></td><td>{fact.observedAt ? <time class="operator-time" datetime={new Date(fact.observedAt * 1000).toISOString()} title={operationTime(fact.observedAt, true)}>{operationTime(fact.observedAt)}</time> : "Not checked"}</td><td><Button small variant="ghost" aria-label={"Inspect " + checkName(fact.kind)} onClick={() => setInspected(fact)}>Inspect</Button></td></tr>}</For></tbody></table></div>
        <Show when={inspected()}>{fact => <section class="health-detail" aria-label={checkName(fact().kind) + " details"}>
          <div class="section-toolbar"><h3>{checkName(fact().kind)}</h3><IconButton icon="close" label="Close health details" ghost onClick={() => setInspected(undefined)} /></div>
          <p>{fact().summary}</p>
          <Show when={fact().reference}><p class="mono">{fact().reference}</p></Show>
          <div class="toolbar-actions">
            <Show when={fact().applicationId}><A class="button neutral" href={"/applications/" + encodeURIComponent(fact().applicationId!) + "?tab=" + (fact().kind === "certificate" ? "domains" : "overview")}>{fact().kind === "certificate" ? "Open domains" : "Open application"}</A></Show>
            <Show when={!fact().applicationId && ["proxy", "daemon", "host"].includes(fact().kind)}><A class="button neutral" href="/system#maintenance">Open maintenance</A></Show>
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
    </div></Show>
    <Show when={repair()}>{plan => <ChangeReview plan={plan()} title="Repair proxy" confirm="Repair proxy" close={() => setRepair(undefined)} apply={async () => { await post("/system/repair", { revision: plan().revision }, auth.session()?.csrfToken); setRepair(undefined); await query.refetch(); }} />}</Show>
    <Show when={section() === "maintenance"}>
      <section class="settings-section"><h2>Proxy</h2><dl class="settings-group"><div class="setting-row"><dt>Traefik routing</dt><dd><Button icon="refresh" disabled={busy()} onClick={() => void action(async () => setRepair(await post<ChangePlan>("/system/repair", { review: true }, auth.session()?.csrfToken)))}>Repair proxy</Button></dd></div></dl></section>
      <Show when={query.data?.host}>{host => <section class="settings-section"><h2>Host maintenance</h2>
        <dl class="settings-group">
          <div class="setting-row"><dt>Boot services</dt><dd>{host().stale ? "Not observed" : host().dockerAtBoot && host().shipperAtBoot ? "Ready" : "Check Docker and Shipper"}</dd></div>
          <div class="setting-row"><dt>Security updates</dt><dd>{host().stale ? "Not observed" : host().securityServiceEnabled ? "System service enabled" : "Operator managed"}</dd></div>
          <div class="setting-row"><dt>Reboot</dt><dd>{host().stale ? "Not observed" : host().rebootRequired ? "Required" : "Not required"}</dd></div>
          <div class="setting-row"><dt>System clock</dt><dd>{host().stale ? "Not observed" : host().clockSynchronized ? "Synchronized" : "Not synchronized"}</dd></div>
        </dl>
      </section>}</Show>
      <Retention />
      <section class="settings-section"><h2>Recovery</h2><dl class="settings-group"><div class="setting-row"><dt>Backups and restore</dt><dd><A class="button neutral small" href="/settings#recovery">Open recovery</A></dd></div></dl></section>
    </Show>
  </div>;
}
