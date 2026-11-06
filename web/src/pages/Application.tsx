import { useNavigate, useParams, useSearchParams } from "@solidjs/router";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { For, Show, createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import { APIError, api, post } from "../api";
import { mediaQuery } from "../media";
import { useAuth } from "../context";
import type { ApplicationDetail, Deployment, ProxyConfig, StaticConfig, Container, Domain, ChangePlan, LogLine } from "../types";
import { repositoryLabel, applicationState, methodLabel, domainURL, primaryDomain, deploymentLabel, deploymentRef, operationTime, operationPending, statusOf, isStaticApplication, deploymentSeconds, formatDuration } from "../data";
import { ActionMenu, Button, ErrorNotice, Status } from "../components/Controls";
import { Icon } from "../components/Icon";
import LogViewer from "../components/LogViewer";
import ContainerDrawer from "../components/ContainerDrawer";
import ApplicationSettings from "../components/ApplicationSettings";
import Tasks from "../components/Tasks";
import Backups from "../components/Backups";
import ChangeReview from "../components/ChangeReview";
import Domains from "../components/Domains";
import Variables from "../components/Variables";
import RuntimeLogs from "../components/RuntimeLogs";
import Diagnostics, { ApplicationIssue, applicationDiagnostics, diagnosticIssue } from "../components/Diagnostics";
import { ServiceList, ServicePanel } from "../components/Services";

const appTabs = [
  { id: "overview", label: "Overview" }, { id: "deployments", label: "Deployments" }, { id: "logs", label: "Logs" },
  { id: "domains", label: "Domains" }, { id: "env", label: ".env" }, { id: "tasks", label: "Tasks" },
  { id: "backups", label: "Backups" }, { id: "settings", label: "Settings" },
];

export default function ApplicationPage() {
  const params = useParams(), [search, setSearch] = useSearchParams(), auth = useAuth(), client = useQueryClient(), navigate = useNavigate();
  const [actionError, setActionError] = createSignal(""), [pending, setPending] = createSignal(false), [deleting, setDeleting] = createSignal(false), [selected, setSelected] = createSignal<Container>();
  const [editorScope, setEditorScope] = createSignal<string>();
  const [historyExpanded, setHistoryExpanded] = createSignal(false);
  const [observedResult, setObservedResult] = createSignal<{ id: string; line: LogLine }>();
  const query = createQuery(() => ({ queryKey: ["applications", params.id], queryFn: () => api<ApplicationDetail>("/applications/" + params.id), reconcile: "id" }));
  const proxy = createQuery(() => ({ queryKey: ["proxy"], queryFn: () => api<ProxyConfig>("/proxy") }));
  const [rollback,setRollback]=createSignal<{target:Deployment;serving?:Deployment;plan:ChangePlan}>();
  const [review,setReview]=createSignal<{config:StaticConfig;plan:ChangePlan;resolve:(applied:boolean)=>void}>();
 onCleanup(()=>review()?.resolve(false));
 const [now, setNow] = createSignal(Date.now()/1000);
  const app = () => query.data?.application;
  const compact = mediaQuery("(width < 48rem)");
  const tab = () => search.tab === "variables" ? "env" : appTabs.some(item => item.id === search.tab) ? String(search.tab) : "overview";
  const serviceView = () => tab() === "overview" && app()?.method === "compose" && !!search.service;
  const selectTab = (value: string) => setSearch({ tab: value, service: undefined, section: undefined, route: undefined });
  createEffect(() => { if (tab() !== "deployments" || !query.data?.deployments.some(d => operationPending(d.state))) return;setNow(Date.now()/1000);const timer=setInterval(() => setNow(Date.now()/1000),1000);onCleanup(() => clearInterval(timer)); });
  const busy = () => pending() || operationPending(app()?.latestDeployment?.state);
  createEffect(() => { document.title = (app()?.name ?? "Application") + " | Shipper"; if (deleting() && query.error instanceof APIError && query.error.status === 404) navigate("/"); });
  const active = createMemo(() => query.data?.activeDeployment ?? query.data?.deployments.find(d => d.id === app()?.activeDeploymentId));
  const managed = () => ["git","compose","image","inline","local"].includes(app()?.sourceKind||"") && !!app()?.method;
  const domain = () => app() && primaryDomain(app()!);
  const diagnostic = applicationDiagnostics(() => params.id ?? "", () => tab() === "overview" && !serviceView() && managed());
  const hasIssue = () => !!diagnosticIssue(diagnostic.data) || diagnostic.data?.latestOperation?.state === "failed";
  const viewDeployment = (id: string) => setSearch({ tab: "deployments", deployment: id, section: undefined });
  const routingIssue = () => tab() === "overview" ? diagnostic.data?.checks.find(check =>
    ["route", "certificate"].includes(check.layer) && ["failed", "unreachable", "degraded", "warning", "unhealthy"].includes(check.state)) : undefined;
  const openDomains = (hostname: string) => setSearch({ tab: "domains", service: undefined, section: undefined, route: hostname });
  function focusDiagnostics() {
    setSearch({ tab: "overview", section: "diagnostics", service: undefined });
    document.querySelector(".diagnostics-section")?.scrollIntoView({ block: "start" });
  }

  const historical=createQuery(()=>({queryKey:["applications",params.id,"deployment",search.deployment],enabled:!!search.deployment&&!!query.data&&!query.data.deployments.some(d=>d.id===search.deployment),queryFn:()=>api<Deployment>(`/applications/${params.id}/deployments/${search.deployment}`)}));
  const currentDeployment=createMemo(()=>search.deployment?query.data?.deployments.find(d=>d.id===search.deployment)??historical.data:query.data?.deployments[0]);
  const deployments = () => query.data?.deployments ?? [];
  const visibleDeployments = createMemo(() => {
    const history = deployments();
    if (!compact() || historyExpanded()) return history;
    const index = history.findIndex(deployment => deployment.id === currentDeployment()?.id);
    const start = Math.max(0, index - 1);
    return history.slice(start, start + 2);
  });
  function displayDeployment(deployment: Deployment) {
    const observed = observedResult();
    return observed?.id === deployment.id && operationPending(deployment.state)
      ? { ...deployment, state: observed.line.step, finishedAt: observed.line.createdAt }
      : deployment;
  }
  function displayApplicationState(application: ApplicationDetail["application"]) {
    const latest = application.latestDeployment;
    return latest ? applicationState({ ...application, latestDeployment: displayDeployment(latest) }) : applicationState(application);
  }
  async function deploy(body: { config?: StaticConfig; noCache?: boolean; revision?: string } = {}) {
    if (busy()) throw new Error("An application operation is already in progress");
    setPending(true);setActionError("");
    try { const d = await post<Deployment>(`/applications/${params.id}/deployments`, body, auth.session()?.csrfToken); await client.invalidateQueries({ queryKey: ["applications", params.id] });setSearch({ tab: "deployments", deployment: d.id }); }
    finally { setPending(false); }
  }

  async function saveConfig(config:StaticConfig):Promise<boolean> {
    const plan=await post<ChangePlan>(`/applications/${params.id}/changes`,{config},auth.session()?.csrfToken);
    return new Promise(resolve=>setReview({config,plan,resolve}));
  }
  function closeReview(){review()?.resolve(false);setReview(undefined);}
  async function applyReview(){const r=review();if(!r)return;await deploy({config:r.config,revision:r.plan.revision});r.resolve(true);setReview(undefined);}
  async function deploymentAction(id:string,action:"cancel"|"retry"){setActionError("");try{const result=await post<Deployment>(`/applications/${params.id}/deployments/${id}/${action}`,{},auth.session()?.csrfToken);await client.invalidateQueries({queryKey:["applications",params.id]});if(action==="retry")setSearch({tab:"deployments",deployment:result.id});}catch(e){setActionError((e as Error).message);}}
  async function restore(target:Deployment){setActionError("");try{const plan=await post<ChangePlan>(`/applications/${params.id}/deployments/${target.id}/rollback`,{review:true},auth.session()?.csrfToken);setRollback({target,serving:active(),plan});}catch(e){setActionError((e as Error).message);}}
  async function applyRollback(){const value=rollback();if(!value)return;const d=await post<Deployment>(`/applications/${params.id}/deployments/${value.target.id}/rollback`,{revision:value.plan.revision},auth.session()?.csrfToken);setRollback(undefined);await client.invalidateQueries({queryKey:["applications",params.id]});setSearch({tab:"deployments",deployment:d.id});}
  async function rename(name: string) { await post(`/applications/${params.id}/name`, { name }, auth.session()?.csrfToken);await client.invalidateQueries({ queryKey: ["applications"] }); }
  async function domains(domains: Domain[]) { if (busy()) return;setPending(true);try { await post(`/applications/${params.id}/domains`, { domains }, auth.session()?.csrfToken);await client.invalidateQueries({ queryKey: ["applications"] }); } finally { setPending(false); } }
  async function lifecycle(action: string,service?:string) { if (busy()) return;setPending(true);setActionError("");try { await post(`/applications/${params.id}/lifecycle`, { action,service }, auth.session()?.csrfToken);await client.invalidateQueries({ queryKey: ["applications"] }); } catch(e) { setActionError((e as Error).message); } finally { setPending(false); } }
  async function remove() { await post(`/applications/${params.id}/delete`, { confirmName: app()!.name }, auth.session()?.csrfToken);setDeleting(true);setSearch({ tab: "deployments", deployment: undefined });await client.invalidateQueries({ queryKey: ["applications", params.id] }); }
  return <div class="content content-narrow application-detail">
    <Show when={query.error && !deleting()}><ErrorNotice message={query.error!.message} retry={() => void query.refetch()} /></Show>
    <Show when={app()} fallback={<span class="loading-line" aria-label="Loading application" />}>{a => <>
      <Show when={!serviceView()}>
      <div class="app-context" classList={{ "has-editor-scope": !!editorScope() }}>
      <header class="page-heading app-heading"><div class="application-title"><Icon name={a().method === "static" ? "code" : "box"} /><h1>{a().name}</h1><span class="application-title-states">
          <Status value={displayApplicationState(a())} label={routingIssue() && ["running", "serving"].includes(displayApplicationState(a())) ? "Runtime running" : undefined} />
        </span>
        <Show when={editorScope()}><span class="app-editor-scope"><Icon name="chevron" />{editorScope()}</span></Show></div>
        <div class="heading-actions"><Show when={active() && domain() && a().desiredState !== "stopped"}><a class="button ghost visit-app" href={domainURL(domain()!, proxy.data)} target="_blank" rel="noreferrer" aria-label="Visit application"><span>Visit</span><Icon name="external" /></a></Show><Show when={managed()}><Button variant="neutral" icon="deploy" disabled={busy()} onClick={() => void deploy().catch(e => setActionError((e as Error).message))}>Redeploy</Button><ActionMenu label="Application actions" icon="more" scope={tab()}><Show when={active() && domain()}><button onClick={() => window.open(domainURL(domain()!, proxy.data), "_blank", "noopener")}>Visit application</button></Show><button disabled={busy()} onClick={() => void deploy({ noCache: true }).catch(e => setActionError((e as Error).message))}>Rebuild without cache</button><Show when={active()}><button disabled={busy()} onClick={() => void lifecycle(a().desiredState === "stopped" ? "start" : "stop")}>{a().desiredState === "stopped" ? "Start application" : "Stop application"}</button><button disabled={busy()} onClick={() => void lifecycle("restart")}>Restart application</button></Show></ActionMenu></Show></div>
      </header>
      <nav class="application-tabs" aria-label="Application navigation">
        <For each={appTabs.slice(0, managed() ? compact() ? 3 : appTabs.length : 1)}>{item =>
          <button classList={{ active: tab() === item.id }} aria-current={tab() === item.id ? "page" : undefined} onClick={() => selectTab(item.id)}>{item.label}</button>
        }</For>
        <Show when={compact() && managed()}>
          <select class="input app-section-select" aria-label="Other application sections"
            classList={{ selected: appTabs.slice(3).some(item => item.id === tab()) }}
            value={appTabs.slice(3).some(item => item.id === tab()) ? tab() : ""}
            onChange={e => selectTab(e.currentTarget.value)}>
            <option value="" disabled>More</option>
            <For each={appTabs.slice(3)}>{item => <option value={item.id}>{item.label}</option>}</For>
          </select>
        </Show>
      </nav>
      </div>
      </Show>
      <Show when={actionError()}><ErrorNotice message={actionError()} /></Show>
      <Show when={tab() === "overview"}>
        <Show when={!search.service || a().method !== "compose"} fallback={<Show when={search.service} keyed>{name => <ServicePanel application={a()} services={active()?.services ?? a().latestDeployment?.services ?? []} name={String(name)} proxy={proxy.data} busy={busy()} save={saveConfig} saveDomains={domains} close={() => setSearch({ service: undefined })} lifecycle={action=>lifecycle(action,String(search.service))} editCompose={() => setSearch({ tab: "settings", service: undefined, section: "source" })} />}</Show>}>
        <Show when={managed()}><ApplicationIssue diagnostic={diagnostic.data} domains={openDomains} details={focusDiagnostics} view={viewDeployment} /></Show>
        <dl class="application-overview"><div><dt>Domain</dt><dd><Show when={domain()} fallback="None">{d => <a class="inline-link" href={domainURL(d(), proxy.data)} target="_blank" rel="noreferrer">{d().hostname}<Icon name="external" /></a>}</Show></dd></div><Show when={a().sourceKind === "git" || a().sourceKind === "compose" && a().config.composeSource !== "paste"}><div><dt>Repository</dt><dd><Show when={a().sourceRef} fallback="None"><a href={a().sourceRef} target="_blank" rel="noreferrer" class="inline-link repository-reference" title={a().sourceRef}><Icon name="git" />{repositoryLabel(a().sourceRef)}<Icon name="external" /></a></Show></dd></div></Show><div><dt>Method</dt><dd>{methodLabel(a())}</dd></div><Show when={a().sourceKind === "git" || a().sourceKind === "compose" && a().config.composeSource !== "paste"}><div><dt>Commit</dt><dd class="mono">{active()?.commit.slice(0, 12) || "None"}</dd></div></Show></dl>
        <Show when={a().method !== "compose"} fallback={<ServiceList application={a()} services={active()?.services ?? a().latestDeployment?.services ?? []} select={name => setSearch({ service: name })} />}><section class="containers-section"><h2>{isStaticApplication(a()) ? "Runtime" : "Containers"}</h2><div class="table-tray"><table class="inventory-table services-table"><thead><tr><th>{isStaticApplication(a()) ? "Runtime" : "Container"}</th><th>Status</th><th>{isStaticApplication(a()) ? "Container" : "Image"}</th><th aria-label="Details" /></tr></thead><tbody><For each={a().containers}>{c => <tr role="button" tabindex="0" class="container-row" aria-label={"Inspect " + c.name} onClick={() => setSelected(c)} onKeyDown={e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault();setSelected(c); } }}><td><span class="identity"><Icon name="box" /><span class="truncate">{isStaticApplication(a()) ? "Apache" : c.name}</span></span></td><td><Status value={statusOf(c)} /></td><td><span class="truncate image-name" title={isStaticApplication(a()) ? c.name : c.image}>{isStaticApplication(a()) ? compact() && c.name.length > 34 ? c.name.slice(0, 17) + "…" + c.name.slice(-12) : c.name : c.image}</span></td><td class="row-chevron"><Icon name="chevron" /></td></tr>}</For><Show when={!a().containers.length}><tr class="empty-row"><td colspan="4">{isStaticApplication(a()) ? (a().activeDeploymentId ? "Static site unavailable" : "Not deployed") : "No containers"}</td></tr></Show></tbody></table></div></section></Show></Show>
      </Show>
      <Show when={tab() === "deployments"}><div class="deployments-layout"><aside class="deployment-history-rail" aria-label="Deployment history"><header><h2>Release history <span>{deployments().length}</span></h2><Show when={compact() && deployments().length > 2}><Button small variant="ghost" aria-expanded={historyExpanded()} onClick={() => setHistoryExpanded(value => !value)}>{historyExpanded() ? "Show fewer" : "Show all"}</Button></Show></header><div class="deployment-history" classList={{ "history-expanded": historyExpanded() }}><For each={visibleDeployments()}>{d => <button class="deployment-entry" aria-pressed={currentDeployment()?.id === d.id} classList={{ selected: currentDeployment()?.id === d.id }} onClick={() => setSearch({ deployment: d.id })}><span class="deployment-entry-state"><Status value={displayDeployment(d).state} label={deploymentLabel(displayDeployment(d))} /><Show when={d.id === a().activeDeploymentId}><span class="release-current">Current</span></Show></span><span class="mono" title={d.commit || d.id}>{deploymentRef(d)}</span><div class="deployment-meta"><time datetime={new Date(d.createdAt*1000).toISOString()}>{operationTime(d.createdAt)}</time><time class="deployment-duration" aria-label="Deployment duration" datetime={"PT"+deploymentSeconds(displayDeployment(d), now())+"S"}>{formatDuration(deploymentSeconds(displayDeployment(d), now()))}</time></div></button>}</For></div></aside><Show when={currentDeployment()} keyed>{d => <LogViewer deployment={d} current={active()} diagnostics={managed() ? focusDiagnostics : undefined} onTerminalLog={line => setObservedResult({ id: d.id, line })} restore={!busy() && d.state==="succeeded" && !!d.imageId && d.id!==a().activeDeploymentId && ["deploy","apply","rollback"].includes(d.operation)?()=>void restore(d):undefined} cancel={()=>void deploymentAction(d.id,"cancel")} retry={()=>void deploymentAction(d.id,"retry")}/>}</Show><Show when={!currentDeployment()&&historical.isFetching}><span class="loading-line" aria-label="Loading deployment" /></Show><Show when={search.deployment&&historical.error}><ErrorNotice message={historical.error!.message} /></Show><Show when={!currentDeployment()&&!search.deployment}><div class="empty-state">No deployments</div></Show></div></Show>
      <Show when={tab()==="overview"&&!search.service&&managed()}><Diagnostics applicationId={a().id} query={diagnostic} focused={search.section === "diagnostics"} view={viewDeployment}/></Show>
      <Show when={tab() === "logs"}><RuntimeLogs application={a()} containerId={active()?.containerId} /></Show>
      <Show when={tab() === "domains"}><Domains application={a()} services={active()?.services} proxy={proxy.data} busy={busy()} save={domains} selectedHostname={typeof search.route === "string" ? search.route : undefined} /></Show>
      <Show when={tab() === "env"}><Variables application={a()} busy={busy()} save={saveConfig} /></Show>
      <Show when={tab() === "backups"}><Backups application={a()} services={active()?.services??[]} save={saveConfig} editorScope={setEditorScope}/></Show>
      <Show when={tab() === "tasks"}><Tasks application={a()} services={active()?.services??[]} busy={busy()} deployments={query.data?.deployments??[]} editorScope={setEditorScope} view={id=>setSearch({tab:"deployments",deployment:id})}/></Show>
      <Show when={tab() === "settings"}><ApplicationSettings application={a()} services={active()?.services?.map(service => service.name) ?? a()?.latestDeployment?.services?.map(service => service.name)} showSource={search.section === "source"} busy={busy()} save={saveConfig} rename={rename} remove={remove} /></Show>
    </>}</Show>
    <Show when={review()}>{r=><ChangeReview application={app()?.name} compose={app()?.method === "compose"} hooks={{ before: app()?.config.hooks, after: r().config.hooks }} variables={{ before: app()?.config.variables, after: r().config.variables }} plan={r().plan} close={closeReview} apply={applyReview}/>}</Show>
    <Show when={rollback()}>{value=><ChangeReview title="Restore release" confirm="Restore release" application={app()?.name} release={value()} plan={value().plan} close={()=>setRollback(undefined)} apply={applyRollback}/>}</Show>
    <Show when={selected()}>{c => <ContainerDrawer container={c()} close={() => setSelected(undefined)} />}</Show>
  </div>;
}
