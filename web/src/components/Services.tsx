import ServiceTerminal from "./ServiceTerminal";
import Backups from "./Backups";
import RuntimeFields from "./RuntimeFields";
import { For, Show, createMemo, createSignal } from "solid-js";
import type { Application, Domain, ProxyConfig, Service, ServiceConfig, StaticConfig, RuntimePolicy } from "../types";
import { ActionMenu, Button, ErrorNotice, Status } from "./Controls";
import { Icon } from "./Icon";
import { statusOf } from "../data";
import Variables from "./Variables";
import RuntimeLogs from "./RuntimeLogs";
import Domains from "./Domains";
import ServiceSummary from "./ServiceSummary";

export function ServiceList(props: { application: Application; services: Service[]; stats?: (service: string) => { cpuPercent?: number; memoryBytes?: number } | undefined; select: (name: string) => void }) {
  const byService = createMemo(() => {
    const groups = new Map<string, Application["containers"]>();
    for (const container of props.application.containers) {
      if (!container.service) continue;
      const group = groups.get(container.service) ?? [];
      group.push(container);
      groups.set(container.service, group);
    }
    return groups;
  });
  function status(service: Service) {
    const containers = byService().get(service.name) ?? [];
    if (service.oneShot && containers[0]?.state === "exited" && containers[0].status.startsWith("Exited (0)")) return "completed";
    if (containers.some(c => statusOf(c) === "unhealthy")) return "unhealthy";
    return containers[0] ? statusOf(containers[0]) : props.application.activeDeploymentId ? "unavailable" : "created";
  }
  return <section class="containers-section"><h2>Services</h2>
    <Show when={props.services.length} fallback={<div class="empty-state">Not deployed</div>}>
      <ServiceSummary services={props.services} stats={props.stats} open={props.select} status={status} />
    </Show>
  </section>;
}

export function ServicePanel(props: { application: Application; services: Service[]; name: string; proxy?: ProxyConfig; busy: boolean; save: (config: StaticConfig) => Promise<boolean | void>; saveDomains: (domains: Domain[]) => Promise<void>; close: () => void; lifecycle:(action:string)=>Promise<void>; editCompose: () => void }) {
  const [tab, setTab] = createSignal("logs"), [error, setError] = createSignal("");
  const service = () => props.services.find(s => s.name === props.name);
  const override = () => props.application.config.services?.find(s => s.name === props.name);
  const [port, setPort] = createSignal(override()?.port ?? service()?.port ?? 0), [health, setHealth] = createSignal(override()?.healthPath || service()?.healthPath || "/"), [memory, setMemory] = createSignal(override()?.memoryMB || 0);
  const [runtime,setRuntime]=createSignal<RuntimePolicy>(override()?.runtime??{});
  const variables = createMemo(() => { const rows = new Map((service()?.variables ?? []).map(v => [v.name, v]));for (const v of override()?.variables ?? []) rows.set(v.name, v);return [...rows.values()]; });
  const app = createMemo(() => ({ ...props.application, containers: props.application.containers.filter(c => c.service === props.name), config: { ...props.application.config, variables: variables() } }));
  async function save(value: Partial<ServiceConfig>) {
    const config = props.application.config, current = override() ?? { name: props.name, healthPath: "/", memoryMB: 0, variables: [] };
    return props.save({ ...config, services: [...(config.services ?? []).filter(s => s.name !== props.name), { ...current, ...value, name: props.name }] });
  }
  async function settings(e: SubmitEvent) { e.preventDefault();setError("");try { await save({ port: port(), healthPath: health(), memoryMB: memory(),runtime:runtime() }); }catch (e) { setError((e as Error).message); } }
  return <section class="service-detail"><div class="app-context"><header class="service-heading"><Button icon="back" variant="ghost" class="service-parent" onClick={props.close}>{props.application.name}</Button><Icon name="chevron" /><h1><Icon name="box" />{props.name}</h1><ActionMenu label="Service actions" icon="more" class="service-actions" scope={tab()}><For each={["start","stop","restart"]}>{action=><button disabled={props.busy} onClick={()=>void props.lifecycle(action)}>{action[0].toUpperCase()+action.slice(1)} service</button>}</For></ActionMenu></header>
    <nav class="service-tabs" aria-label="Service navigation"><For each={["logs", "terminal", "env", "domains", "backups", "settings"]}>{value => <button classList={{ active: tab() === value }} aria-current={tab() === value ? "page" : undefined} onClick={() => setTab(value)}>{value === "env" ? ".env" : value[0].toUpperCase() + value.slice(1)}</button>}</For></nav>
    </div><Show when={error()}><ErrorNotice message={error()} /></Show>
    <Show when={tab() === "backups"}><Backups application={props.application} services={props.services} service={props.name} save={props.save}/></Show>
    <Show when={tab() === "terminal"}><ServiceTerminal application={app()} /></Show>
    <Show when={tab() === "logs"}><RuntimeLogs application={app()} /></Show>
    <Show when={tab() === "env"}><Variables application={app()} service={props.name} busy={props.busy} save={config => { const rows = config.variables, names = new Set(rows.map(v => v.name)), inherited = new Set((service()?.variables ?? []).map(v => v.name)), owned = new Set((override()?.variables ?? []).map(v => v.name));return save({ variables: rows.filter(v => owned.has(v.name) || !inherited.has(v.name) || !v.keep || !!v.keepFrom || !v.secret).map(v => ({ ...v, scope: "runtime" })), unsetVariables: [...new Set([...(override()?.unsetVariables ?? []), ...inherited])].filter(name => !names.has(name)) }); }} /></Show>
    <Show when={tab() === "domains"}><Domains application={props.application} services={props.services} service={props.name} proxy={props.proxy} busy={props.busy} save={props.saveDomains} configureService={() => setTab("settings")} /></Show>
    <Show when={tab() === "settings"}><form onSubmit={settings} class="service-settings"><div class="form-fields"><label for="service-port">HTTP port</label><input id="service-port" class="input" type="number" min="0" max="65535" value={port() || ""} placeholder="Internal service" onInput={e => setPort(Number(e.currentTarget.value) || 0)} /><label for="service-health">Health check path</label><input id="service-health" class="input" value={health()} onInput={e => setHealth(e.currentTarget.value)} /><label for="service-memory">Memory limit (MiB)</label><input id="service-memory" class="input" type="number" min="64" max="8192" placeholder="Compose configuration" value={memory() || ""} onInput={e => setMemory(Number(e.currentTarget.value) || 0)} /></div><RuntimeFields value={runtime()} change={setRuntime} service/><Show when={service()?.volumes.length}><section class="volume-settings"><h2>Volumes</h2><div class="table-tray"><table class="inventory-table"><thead><tr><th>Source</th><th>Mount path</th></tr></thead><tbody><For each={service()?.volumes}>{volume => <tr><td class="mono">{volume.name || "Persistent volume"}</td><td class="mono">{volume.path}</td></tr>}</For></tbody></table></div></section></Show><div class="form-actions"><Button variant="ghost" icon="code" onClick={props.editCompose}>Compose source</Button><Button type="submit" variant="primary" icon="deploy" disabled={props.busy}>Review changes</Button></div></form></Show>
  </section>;
}
