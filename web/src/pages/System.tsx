import { For, Show, createSignal, onMount } from "solid-js";
import { A, useLocation } from "@solidjs/router";
import { createQuery } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import type { Application } from "../types";
import type { Observation } from "../components/Diagnostics";
import { Button, Disclosure, ErrorNotice, Status } from "../components/Controls";
import { SegmentedChoice } from "../components/Choices";
import { bytes, createdDate, logDateTime } from "../data";
import Retention from "../components/Retention";
import ChangeReview from "../components/ChangeReview";
import type {ChangePlan,SystemInfo} from "../types";

interface HostStatus { observedAt: number; stale: boolean; dockerAtBoot: boolean; shipperAtBoot: boolean; rebootRequired: boolean; securityServiceEnabled: boolean; clockSynchronized: boolean }
interface Credential { id: string; name: string; scopes: string[]; applications: string[]; expiresAt: number }
interface Notifications { enabled: boolean; kind: string; events: string[]; configured?: boolean; webhook?: string; private: boolean; smtp?: string; username?: string; password?: string; from?: string; to?: string }
const permissionLabel:Record<string,string>={read:"Read",logs:"Logs",deploy:"Deploy",execute:"Run tasks",secrets:"Reveal secrets",destructive:"Delete applications","host-admin":"Host maintenance"};
export default function System() {
  const auth = useAuth();
  const location = useLocation();
  const sections = [{ id: "overview", label: "Overview" }, { id: "maintenance", label: "Maintenance" },
    { id: "notifications", label: "Notifications" }, { id: "tokens", label: "Access tokens" }] as const;
  const section = () => sections.find(s => s.id === location.hash.slice(1))?.id ?? "overview";
 const [repair,setRepair]=createSignal<ChangePlan>();
 const system=createQuery(()=>({queryKey:["system"],queryFn:()=>api<SystemInfo>("/system")}));
  const [error, setError] = createSignal(""), [busy, setBusy] = createSignal(false), [token, setToken] = createSignal("");
  const [name, setName] = createSignal(""), [scope, setScope] = createSignal(["read"]), [app, setApp] = createSignal(""), [days, setDays] = createSignal(30);
  const [notification, setNotification] = createSignal<Notifications>(), [result, setResult] = createSignal("");
  const query = createQuery(() => ({ queryKey: ["system", "operations"], queryFn: () => api<{ items: Observation[]; host?: HostStatus; notifications: { pending: number; failed: number } }>("/system/operations"), refetchInterval: 15000 }));
  const credentials = createQuery(() => ({ queryKey: ["operation-tokens"], queryFn: () => api<{ items: Credential[] }>("/operation-tokens"), enabled: section() === "tokens" }));
  const applications = createQuery(() => ({ queryKey: ["system", "applications"], queryFn: () => api<{ items: Application[] }>("/applications"), enabled: section() === "tokens" }));
  const notifications = createQuery(() => ({ queryKey: ["notifications"], queryFn: () => api<Notifications>("/notifications"), enabled: section() === "notifications" }));
  const config = () => notification() ?? notifications.data;
  const change = (value: Partial<Notifications>) => setNotification({ ...config()!, ...value });
  onMount(() => { document.title = "System | Shipper"; });
  async function action(work: () => Promise<unknown>) {
    if (busy()) return; setBusy(true); setError(""); setResult("");
    try { await work(); } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  async function createToken(e: SubmitEvent) {
    e.preventDefault(); await action(async () => {
      const value = await post<{ token: string }>("/operation-tokens", { name: name(), scopes: scope(), applications: app() ? [app()] : [], days: days() }, auth.session()?.csrfToken);
      setToken(value.token); setName(""); await credentials.refetch();
    });
  }
  async function saveNotifications(e: SubmitEvent) {
    e.preventDefault(); await action(async () => { setNotification(await post<Notifications>("/notifications", config(), auth.session()?.csrfToken)); await notifications.refetch(); setResult("Saved"); });
  }
  return <div class="content content-wide system-page">
    <header class="page-heading"><h1>System</h1><Show when={section() === "overview"}><Button icon="refresh" disabled={busy()} onClick={() => void action(async () => { await post("/system/recheck", {}, auth.session()?.csrfToken); await query.refetch(); })}>Recheck</Button></Show></header>
    <nav class="section-navigation" aria-label="System sections">
      <For each={sections}>{item => <a href={"/system#" + item.id} aria-current={section() === item.id ? "page" : undefined}>{item.label}</a>}</For>
    </nav>
    <Show when={error() || query.error}><ErrorNotice message={error() || query.error!.message} /></Show>
    <Show when={section() === "overview"}>
    <section class="settings-section"><h2>Health</h2><div class="table-tray"><table class="inventory-table"><thead><tr><th>Component</th><th>Status</th><th>Observed</th></tr></thead><tbody><For each={query.data?.items}>{fact => <tr><td>{fact.kind}</td><td><Status value={fact.stale?"stale":fact.state} /></td><td>{logDateTime(fact.observedAt)}</td></tr>}</For></tbody></table></div>
      <For each={query.data?.items.filter(v => v.kind === "storage")}>{fact => <dl class="settings-group"><div class="setting-row"><dt>{fact.details.scope}</dt><dd>{bytes(fact.details.free ?? 0)} free / {bytes(fact.details.total ?? 0)}</dd></div></dl>}</For>
    </section>
    <section class="settings-section"><h2>Runtime</h2><dl class="settings-group"><div class="setting-row"><dt>Shipper</dt><dd>{system.data?.version||"Loading"}</dd></div><div class="setting-row"><dt>Docker Engine</dt><dd>{system.data?.docker.engine.version||"Unavailable"}</dd></div><div class="setting-row"><dt>Docker host</dt><dd>{system.data?.docker.engine.os||"Unavailable"}</dd></div><div class="setting-row"><dt>Shipper heap</dt><dd>{bytes(system.data?.runtime.heapBytes||0)}</dd></div><div class="setting-row"><dt>Recovery</dt><dd><A class="button neutral small" href="/settings#recovery">Open recovery</A></dd></div></dl></section>
    </Show>
    <Show when={repair()}>{plan=><ChangeReview plan={plan()} title="Repair proxy" confirm="Repair proxy" close={()=>setRepair(undefined)} apply={async()=>{await post("/system/repair",{revision:plan().revision},auth.session()?.csrfToken);setRepair(undefined);await query.refetch()}}/>}</Show>
    <Show when={section() === "maintenance"}>
    <section class="settings-section"><div class="section-toolbar"><h2>Proxy</h2><Button icon="refresh" disabled={busy()} onClick={()=>void action(async()=>setRepair(await post<ChangePlan>("/system/repair",{review:true},auth.session()?.csrfToken)))}>Repair proxy</Button></div></section>
    <Show when={query.data?.host}>{host => <section class="settings-section"><h2>Host maintenance</h2>
      <dl class="settings-group">
        <div class="setting-row"><dt>Boot services</dt><dd>{host().stale ? "Not observed" : host().dockerAtBoot && host().shipperAtBoot ? "Ready" : "Check Docker and Shipper"}</dd></div>
        <div class="setting-row"><dt>Security updates</dt><dd>{host().stale ? "Not observed" : host().securityServiceEnabled ? "System service enabled" : "Operator managed"}</dd></div>
        <div class="setting-row"><dt>Reboot</dt><dd>{host().stale ? "Not observed" : host().rebootRequired ? "Required" : "Not required"}</dd></div>
        <div class="setting-row"><dt>System clock</dt><dd>{host().stale ? "Not observed" : host().clockSynchronized ? "Synchronized" : "Not synchronized"}</dd></div>
      </dl>
    </section>}</Show>
    <Retention/>
    </Show>
    <Show when={section() === "notifications"}>
    <section class="settings-section"><h2>Notifications</h2><Show when={config()}>{c => <form onSubmit={saveNotifications}>
      <label class="checkbox-label"><input type="checkbox" checked={c().enabled} onChange={e => change({ enabled: e.currentTarget.checked })} />Enabled</label>
      <Show when={c().enabled}><div class="form-fields">
        <SegmentedChoice label="Delivery" value={c().kind} choices={[{value:"webhook",label:"Webhook"},{value:"email",label:"Email"}]} change={kind => change({kind})} />
        <Show when={c().kind === "webhook"} fallback={<>
          <label for="smtp-host">SMTP host</label><input id="smtp-host" class="input" placeholder="smtp.example.com:587" value={c().smtp || ""} onInput={e => change({ smtp: e.currentTarget.value })} />
          <label for="smtp-user">Username</label><input id="smtp-user" class="input" value={c().username || ""} onInput={e => change({ username: e.currentTarget.value })} />
          <label for="smtp-password">Password</label><input id="smtp-password" class="input" type="password" autocomplete="new-password" value={c().password || ""} onInput={e => change({ password: e.currentTarget.value })} />
          <label for="smtp-from">From</label><input id="smtp-from" class="input" type="email" value={c().from || ""} onInput={e => change({ from: e.currentTarget.value })} />
          <label for="smtp-to">To</label><input id="smtp-to" class="input" type="email" value={c().to || ""} onInput={e => change({ to: e.currentTarget.value })} />
        </>}><label for="notification-url">Webhook URL</label><input id="notification-url" class="input" type="url" placeholder={c().configured ? "Saved endpoint" : "https://"} value={c().webhook || ""} onInput={e => change({ webhook: e.currentTarget.value })} /></Show>
      </div><details class="configuration"><summary>Events and network</summary><div class="capability-choices">
        <For each={["deployment", "service", "backup", "storage", "certificate", "daemon", "proxy", "task", "update"]}>{kind => <label><input type="checkbox" checked={c().events.includes(kind)} onChange={e => change({ events: e.currentTarget.checked ? [...c().events, kind] : c().events.filter(v => v !== kind) })} />{kind}</label>}</For>
        <label><input type="checkbox" checked={c().private} onChange={e => change({ private: e.currentTarget.checked })} />Allow private destinations</label>
      </div></details></Show>
      <div class="form-actions"><Show when={result()}><span role="status">{result()}</span></Show><Button variant="ghost" disabled={busy() || !c().enabled} onClick={() => void action(async () => { await post("/notifications/test", { config: config() }, auth.session()?.csrfToken); setResult("Delivered"); })}>Test delivery</Button><Button type="submit" disabled={busy()}>Save</Button></div>
    </form>}</Show><Show when={query.data?.notifications.failed}><div class="deployment-error">{query.data!.notifications.failed} failed deliveries</div></Show></section>
    </Show>
    <Show when={section() === "tokens"}>
    <section class="settings-section"><h2>Access tokens</h2><Show when={credentials.error}><ErrorNotice message={credentials.error!.message} /></Show>
      <div class="table-tray"><table class="inventory-table"><thead><tr><th>Name</th><th>Permissions</th><th>Expires</th><th /></tr></thead><tbody><For each={credentials.data?.items}>{credential => <tr><td>{credential.name}</td><td>{credential.scopes.map(v=>permissionLabel[v]||v).join(", ")}</td><td>{createdDate(credential.expiresAt)}</td><td><Button small variant="ghost" disabled={busy()} onClick={() => void action(async () => { await post(`/operation-tokens/${credential.id}/revoke`, {}, auth.session()?.csrfToken); await credentials.refetch(); })}>Revoke</Button></td></tr>}</For></tbody></table></div>
      <Show when={token()}><div class="token-reveal"><input class="input mono" aria-label="New access token" readonly value={token()} /><Button onClick={() => void navigator.clipboard.writeText(token()).catch(() => setError("The token could not be copied"))}>Copy</Button><Button variant="ghost" onClick={() => setToken("")}>Dismiss</Button></div></Show>
      <Disclosure label="Create token" icon="plus"><form onSubmit={createToken}><div class="form-fields">
        <label for="token-name">Name</label><input id="token-name" class="input" required maxlength="80" value={name()} onInput={e => setName(e.currentTarget.value)} />
        <label for="token-app">Application</label><select id="token-app" class="input" value={app()} onChange={e => setApp(e.currentTarget.value)}><option value="">All applications</option><For each={applications.data?.items}>{a => <option value={a.id}>{a.name}</option>}</For></select>
        <label for="token-days">Expires in days</label><input id="token-days" class="input" type="number" min="1" max="365" value={days()} onInput={e => setDays(Number(e.currentTarget.value))} />
      </div><div class="capability-choices"><For each={["read", "logs", "deploy", "execute", "secrets", "destructive", "host-admin"]}>{permission => <label><input type="checkbox" checked={scope().includes(permission)} onChange={e => setScope(e.currentTarget.checked ? [...scope(), permission] : scope().filter(v => v !== permission))} />{permissionLabel[permission]}</label>}</For></div><div class="form-actions"><Button variant="primary" type="submit" disabled={busy() || !scope().length}>Create token</Button></div></form></Disclosure>
    </section>
    </Show>
  </div>;
}
