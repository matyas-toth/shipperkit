import { A, useNavigate, useParams, useSearchParams } from "@solidjs/router";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { For, Show, createEffect, createMemo, createSignal } from "solid-js";
import { createStore } from "solid-js/store";
import { APIError, api, post } from "../api";
import { useAuth } from "../context";
import type { ApplicationDetail, Deployment, ProxyConfig, StaticConfig, Container } from "../types";
import { applicationState, methodLabel, applicationURL, createdDate, operationPending, statusOf } from "../data";
import { Button, ErrorNotice, Status } from "../components/Controls";
import { Icon } from "../components/Icon";
import LogViewer from "../components/LogViewer";
import ContainerDrawer from "../components/ContainerDrawer";
import ApplicationSettings from "../components/ApplicationSettings";

export default function ApplicationPage() {
  const params = useParams(), [search, setSearch] = useSearchParams(), auth = useAuth(), client = useQueryClient(), navigate = useNavigate();
  const [actionError, setActionError] = createSignal(""), [pending, setPending] = createSignal(false), [deleting, setDeleting] = createSignal(false), [selected, setSelected] = createSignal<Container>();
  const query = createQuery(() => ({ queryKey: ["applications", params.id], queryFn: () => api<ApplicationDetail>("/applications/" + params.id), reconcile: "id" }));
  const proxy = createQuery(() => ({ queryKey: ["proxy"], queryFn: () => api<ProxyConfig>("/proxy") }));
  const app = () => query.data?.application;
  const tab = () => ["overview", "deployments", "domains", "settings"].includes(String(search.tab)) ? String(search.tab) : "overview";
  const busy = () => pending() || operationPending(app()?.latestDeployment?.state);
  createEffect(() => { document.title = (app()?.name ?? "Application") + " | Shipper"; if (deleting() && query.error instanceof APIError && query.error.status === 404) navigate("/"); });
  const active = createMemo(() => query.data?.deployments.find(d => d.id === app()?.activeDeploymentId));
  const currentDeployment = createMemo(() => query.data?.deployments.find(d => d.id === search.deployment) ?? query.data?.deployments[0]);
  async function deploy(body: { name?: string; config?: StaticConfig } = {}) {
    if (busy()) return; setPending(true);setActionError("");
    try { const d = await post<Deployment>(`/applications/${params.id}/deployments`, body, auth.session()?.csrfToken); await client.invalidateQueries({ queryKey: ["applications", params.id] });setSearch({ tab: "deployments", deployment: d.id }); }
    finally { setPending(false); }
  }
  async function remove() { await post(`/applications/${params.id}/delete`, { confirmName: app()!.name }, auth.session()?.csrfToken);setDeleting(true);setSearch({ tab: "deployments", deployment: undefined });await client.invalidateQueries({ queryKey: ["applications", params.id] }); }
  return <div class="content content-narrow application-detail">
    <A href="/" class="back-link"><Icon name="back" />Applications</A>
    <Show when={query.error && !deleting()}><ErrorNotice message={query.error!.message} retry={() => void query.refetch()} /></Show>
    <Show when={app()} fallback={<span class="loading-line" aria-label="Loading application" />}>{a => <>
      <header class="page-heading app-heading"><div class="application-title"><Icon name={a().method === "static" ? "code" : "box"} /><h1>{a().name}</h1><Status value={applicationState(a())} /></div>
        <div class="heading-actions"><Show when={active()}>{d => <a class="button ghost" href={applicationURL(d().config, proxy.data)} target="_blank" rel="noreferrer">Visit<Icon name="external" /></a>}</Show><Show when={a().method === "static"}><Button icon="deploy" disabled={busy()} onClick={() => void deploy().catch(e => setActionError((e as Error).message))}>Redeploy</Button></Show></div>
      </header>
      <Show when={actionError()}><ErrorNotice message={actionError()} /></Show>
      <nav class="application-tabs" aria-label="Application navigation"><For each={["overview", "deployments", "domains", "settings"].filter(value => a().method === "static" || value === "overview")}>{value => <button classList={{ active: tab() === value }} onClick={() => setSearch({ tab: value })}>{value[0].toUpperCase() + value.slice(1)}</button>}</For></nav>
      <Show when={tab() === "overview"}>
        <dl class="application-overview"><div><dt>Domain</dt><dd><Show when={active()} fallback={a().config.domain || "None"}>{d => <a class="inline-link" href={applicationURL(d().config, proxy.data)} target="_blank" rel="noreferrer">{d().config.domain}<Icon name="external" /></a>}</Show></dd></div><div><dt>Repository</dt><dd><Show when={a().sourceRef} fallback="None"><a href={a().sourceRef} target="_blank" rel="noreferrer" class="inline-link"><Icon name="git" />{a().sourceRef.replace("https://", "").replace(/\.git$/, "")}<Icon name="external" /></a></Show></dd></div><div><dt>Method</dt><dd>{methodLabel(a())}</dd></div><div><dt>Commit</dt><dd class="mono">{active()?.commit.slice(0, 12) || "None"}</dd></div></dl>
        <section class="containers-section"><h2>Containers</h2><div class="table-tray"><table class="inventory-table services-table"><thead><tr><th>Container</th><th>Status</th><th>Image</th><th aria-label="Details" /></tr></thead><tbody><For each={a().containers}>{c => <tr role="button" tabindex="0" class="container-row" aria-label={"Inspect " + c.name} onClick={() => setSelected(c)} onKeyDown={e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault();setSelected(c); } }}><td><span class="identity"><Icon name="box" /><span class="truncate">{c.name}</span></span></td><td><Status value={statusOf(c)} /></td><td><span class="truncate image-name">{c.image}</span></td><td class="row-chevron"><Icon name="chevron" /></td></tr>}</For><Show when={!a().containers.length}><tr class="empty-row"><td colspan="4">No containers</td></tr></Show></tbody></table></div></section>
      </Show>
      <Show when={tab() === "deployments"}><div class="deployments-layout"><aside class="deployment-history" aria-label="Deployment history"><For each={query.data?.deployments}>{d => <button class="deployment-entry" classList={{ selected: currentDeployment()?.id === d.id }} onClick={() => setSearch({ deployment: d.id })}><Status value={d.state} /><span class="mono">{d.commit.slice(0, 8) || d.id.slice(0, 8)}</span><time>{createdDate(d.createdAt)}</time></button>}</For></aside><Show when={currentDeployment()} keyed>{d => <LogViewer deployment={d} />}</Show><Show when={!currentDeployment()}><div class="empty-state">No deployments</div></Show></div></Show>
      <Show when={tab() === "domains"}><DomainForm application={a()} active={active()} proxy={proxy.data} busy={busy()} save={config => deploy({ config })} /></Show>
      <Show when={tab() === "settings"}><ApplicationSettings application={a()} acme={!!proxy.data?.acmeEmail} busy={busy()} save={(name, config) => deploy({ name, config })} remove={remove} /></Show>
    </>}</Show>
    <Show when={selected()}>{c => <ContainerDrawer container={c()} close={() => setSelected(undefined)} />}</Show>
  </div>;
}
function DomainForm(props: { application: ApplicationDetail["application"]; active?: Deployment; proxy?: ProxyConfig; busy: boolean; save: (config: StaticConfig) => Promise<void> }) {
  const auth = useAuth(), [config, setConfig] = createStore({ ...props.application.config }), [error, setError] = createSignal(""), [result, setResult] = createSignal<{ state: string; message: string }>(), [checking, setChecking] = createSignal(false);
  async function submit(e: SubmitEvent) { e.preventDefault();setError("");try { await props.save(config); }catch (e) { setError((e as Error).message); } }
  async function check() { setChecking(true);setError("");try { setResult(await post(`/applications/${props.application.id}/domain-check`, {}, auth.session()?.csrfToken)); }catch (e) { setError((e as Error).message); }finally { setChecking(false); } }
  return <div class="domain-settings"><Show when={error()}><ErrorNotice message={error()} /></Show><form onSubmit={submit}><div class="form-fields"><label for="domain">Domain</label><input class="input" id="domain" required value={config.domain} onInput={e => { setConfig("domain", e.currentTarget.value); if (e.currentTarget.value.endsWith(".localhost")) setConfig("https", false); }} /><label class="check-field"><input type="checkbox" checked={config.https} disabled={!props.proxy?.acmeEmail || config.domain.endsWith(".localhost")} onChange={e => setConfig("https", e.currentTarget.checked)} />HTTPS</label></div><div class="form-actions"><Button type="submit" icon="deploy" disabled={props.busy}>Save and deploy</Button></div></form>
    <Show when={props.active}><div class="domain-check"><a href={applicationURL(props.active!.config, props.proxy)} target="_blank" rel="noreferrer" class="inline-link"><Icon name="globe" />{props.active!.config.domain}<Icon name="external" /></a><Button icon="refresh" disabled={checking()} onClick={() => void check()}>{checking() ? "Checking domain" : "Verify domain"}</Button></div></Show>
    <Show when={result()}>{r => <div role="status" class={"domain-result " + (r().state === "verified" ? "success" : "warning")}><Icon name={r().state === "verified" ? "check" : "warning"} />{r().message}</div>}</Show>
  </div>;
}
