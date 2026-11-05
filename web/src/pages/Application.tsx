import { A, useNavigate, useParams, useSearchParams } from "@solidjs/router";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { For, Show, createEffect, createMemo, createSignal, onCleanup, onMount } from "solid-js";
import { APIError, api, post } from "../api";
import { useAuth } from "../context";
import type { ApplicationDetail, Deployment, ProxyConfig, StaticConfig, Container, Domain, ChangePlan } from "../types";
import { applicationState, methodLabel, domainURL, primaryDomain, deploymentLabel, createdDate, operationPending, statusOf, isStaticApplication, deploymentSeconds, formatDuration } from "../data";
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
import Diagnostics from "../components/Diagnostics";
import { ServiceList, ServicePanel } from "../components/Services";

export default function ApplicationPage() {
  const params = useParams(), [search, setSearch] = useSearchParams(), auth = useAuth(), client = useQueryClient(), navigate = useNavigate();
  const [actionError, setActionError] = createSignal(""), [pending, setPending] = createSignal(false), [deleting, setDeleting] = createSignal(false), [selected, setSelected] = createSignal<Container>();
  const query = createQuery(() => ({ queryKey: ["applications", params.id], queryFn: () => api<ApplicationDetail>("/applications/" + params.id), reconcile: "id" }));
  const proxy = createQuery(() => ({ queryKey: ["proxy"], queryFn: () => api<ProxyConfig>("/proxy") }));
  const [rollback,setRollback]=createSignal<{id:string;plan:ChangePlan}>();
  const [review,setReview]=createSignal<{config:StaticConfig;plan:ChangePlan;resolve:(applied:boolean)=>void}>();
 onCleanup(()=>review()?.resolve(false));
 const [now, setNow] = createSignal(Date.now()/1000);
  const app = () => query.data?.application;
  let tabs!:HTMLElement;
  const appIdentity=createMemo(()=>app()?.id);
  const tab = () => search.tab === "variables" ? "env" : ["overview", "deployments", "logs", "domains", "env", "tasks", "backups", "settings"].includes(String(search.tab)) ? String(search.tab) : "overview";
  function positionTabs(){const active=tabs?.querySelector<HTMLElement>("button.active");if(!active||!tabs)return;const frame=tabs.getBoundingClientRect(),item=active.getBoundingClientRect();if(item.left<frame.left||item.right>frame.right)tabs.scrollLeft+=item.left-frame.left-(frame.width-item.width)/2;}
  createEffect(()=>{tab();appIdentity();queueMicrotask(positionTabs);});
  onMount(()=>{window.addEventListener("resize",positionTabs);onCleanup(()=>window.removeEventListener("resize",positionTabs));});
  createEffect(() => { if (tab() !== "deployments" || !query.data?.deployments.some(d => operationPending(d.state))) return;setNow(Date.now()/1000);const timer=setInterval(() => setNow(Date.now()/1000),1000);onCleanup(() => clearInterval(timer)); });
  const busy = () => pending() || operationPending(app()?.latestDeployment?.state);
  createEffect(() => { document.title = (app()?.name ?? "Application") + " | Shipper"; if (deleting() && query.error instanceof APIError && query.error.status === 404) navigate("/"); });
  const active = createMemo(() => query.data?.activeDeployment ?? query.data?.deployments.find(d => d.id === app()?.activeDeploymentId));
  const managed = () => ["git","compose","image","inline","local"].includes(app()?.sourceKind||"") && !!app()?.method;
  const domain = () => app() && primaryDomain(app()!);
  const historical=createQuery(()=>({queryKey:["applications",params.id,"deployment",search.deployment],enabled:!!search.deployment&&!!query.data&&!query.data.deployments.some(d=>d.id===search.deployment),queryFn:()=>api<Deployment>(`/applications/${params.id}/deployments/${search.deployment}`)}));
  const currentDeployment=createMemo(()=>search.deployment?query.data?.deployments.find(d=>d.id===search.deployment)??historical.data:query.data?.deployments[0]);
  async function deploy(body: { config?: StaticConfig; noCache?: boolean; revision?: string } = {}) {
    if (busy()) return; setPending(true);setActionError("");
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
  async function restore(id:string){setActionError("");try{const plan=await post<ChangePlan>(`/applications/${params.id}/deployments/${id}/rollback`,{review:true},auth.session()?.csrfToken);setRollback({id,plan});}catch(e){setActionError((e as Error).message);}}
  async function applyRollback(){const value=rollback();if(!value)return;const d=await post<Deployment>(`/applications/${params.id}/deployments/${value.id}/rollback`,{revision:value.plan.revision},auth.session()?.csrfToken);setRollback(undefined);await client.invalidateQueries({queryKey:["applications",params.id]});setSearch({tab:"deployments",deployment:d.id});}
  async function rename(name: string) { await post(`/applications/${params.id}/name`, { name }, auth.session()?.csrfToken);await client.invalidateQueries({ queryKey: ["applications"] }); }
  async function domains(domains: Domain[]) { if (busy()) return;setPending(true);try { await post(`/applications/${params.id}/domains`, { domains }, auth.session()?.csrfToken);await client.invalidateQueries({ queryKey: ["applications"] }); } finally { setPending(false); } }
  async function lifecycle(action: string,service?:string) { if (busy()) return;setPending(true);setActionError("");try { await post(`/applications/${params.id}/lifecycle`, { action,service }, auth.session()?.csrfToken);await client.invalidateQueries({ queryKey: ["applications"] }); } catch(e) { setActionError((e as Error).message); } finally { setPending(false); } }
  async function remove() { await post(`/applications/${params.id}/delete`, { confirmName: app()!.name }, auth.session()?.csrfToken);setDeleting(true);setSearch({ tab: "deployments", deployment: undefined });await client.invalidateQueries({ queryKey: ["applications", params.id] }); }
  return <div class="content content-narrow application-detail">
    <A href="/" class="back-link"><Icon name="back" />Applications</A>
    <Show when={query.error && !deleting()}><ErrorNotice message={query.error!.message} retry={() => void query.refetch()} /></Show>
    <Show when={app()} fallback={<span class="loading-line" aria-label="Loading application" />}>{a => <>
      <header class="page-heading app-heading"><div class="application-title"><Icon name={a().method === "static" ? "code" : "box"} /><h1>{a().name}</h1><Status value={applicationState(a())} /></div>
        <div class="heading-actions"><Show when={active() && domain() && a().desiredState !== "stopped"}><a class="button ghost" href={domainURL(domain()!, proxy.data)} target="_blank" rel="noreferrer">Visit<Icon name="external" /></a></Show><Show when={managed()}><Button icon="deploy" disabled={busy()} onClick={() => void deploy().catch(e => setActionError((e as Error).message))}>Redeploy</Button><ActionMenu label="Application actions" scope={tab() + String(search.service ?? "")}><button disabled={busy()} onClick={() => void deploy({ noCache: true }).catch(e => setActionError((e as Error).message))}>Rebuild without cache</button><Show when={active()}><button disabled={busy()} onClick={() => void lifecycle(a().desiredState === "stopped" ? "start" : "stop")}>{a().desiredState === "stopped" ? "Start application" : "Stop application"}</button><button disabled={busy()} onClick={() => void lifecycle("restart")}>Restart application</button></Show></ActionMenu></Show></div>
      </header>
      <Show when={actionError()}><ErrorNotice message={actionError()} /></Show>
      <nav ref={tabs} class="application-tabs" aria-label="Application navigation"><For each={["overview", "deployments", "logs", "domains", "env", "tasks", "backups", "settings"].filter(value => managed() || value === "overview")}>{value => <button classList={{ active: tab() === value }} aria-current={tab() === value ? "page" : undefined} onClick={() => setSearch({ tab: value, service: undefined, section: undefined })}>{value === "env" ? ".env" : value[0].toUpperCase() + value.slice(1)}</button>}</For></nav>
      <Show when={tab() === "overview"}>
        <Show when={!search.service || a().method !== "compose"} fallback={<Show when={search.service} keyed>{name => <ServicePanel application={a()} services={active()?.services ?? a().latestDeployment?.services ?? []} name={String(name)} proxy={proxy.data} busy={busy()} save={saveConfig} saveDomains={domains} close={() => setSearch({ service: undefined })} lifecycle={action=>lifecycle(action,String(search.service))} editCompose={() => setSearch({ tab: "settings", service: undefined, section: "source" })} />}</Show>}>
        <dl class="application-overview"><div><dt>Domain</dt><dd><Show when={domain()} fallback="None">{d => <a class="inline-link" href={domainURL(d(), proxy.data)} target="_blank" rel="noreferrer">{d().hostname}<Icon name="external" /></a>}</Show></dd></div><Show when={a().sourceKind === "git" || a().sourceKind === "compose" && a().config.composeSource !== "paste"}><div><dt>Repository</dt><dd><Show when={a().sourceRef} fallback="None"><a href={a().sourceRef} target="_blank" rel="noreferrer" class="inline-link"><Icon name="git" />{a().sourceRef.replace("https://", "").replace(/\.git$/, "")}<Icon name="external" /></a></Show></dd></div></Show><div><dt>Method</dt><dd>{methodLabel(a())}</dd></div><Show when={a().sourceKind === "git" || a().sourceKind === "compose" && a().config.composeSource !== "paste"}><div><dt>Commit</dt><dd class="mono">{active()?.commit.slice(0, 12) || "None"}</dd></div></Show></dl>
        <Show when={a().method !== "compose"} fallback={<ServiceList application={a()} services={active()?.services ?? a().latestDeployment?.services ?? []} select={name => setSearch({ service: name })} />}><section class="containers-section"><h2>{isStaticApplication(a()) ? "Serving" : "Containers"}</h2><div class="table-tray"><table class="inventory-table services-table"><thead><tr><th>{isStaticApplication(a()) ? "Runtime" : "Container"}</th><th>Status</th><th>{isStaticApplication(a()) ? "Container" : "Image"}</th><th aria-label="Details" /></tr></thead><tbody><For each={a().containers}>{c => <tr role="button" tabindex="0" class="container-row" aria-label={"Inspect " + c.name} onClick={() => setSelected(c)} onKeyDown={e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault();setSelected(c); } }}><td><span class="identity"><Icon name="box" /><span class="truncate">{isStaticApplication(a()) ? "Apache" : c.name}</span></span></td><td><Status value={statusOf(c)} /></td><td><span class="truncate image-name">{isStaticApplication(a()) ? c.name : c.image}</span></td><td class="row-chevron"><Icon name="chevron" /></td></tr>}</For><Show when={!a().containers.length}><tr class="empty-row"><td colspan="4">{isStaticApplication(a()) ? (a().activeDeploymentId ? "Static site unavailable" : "Not deployed") : "No containers"}</td></tr></Show></tbody></table></div></section></Show></Show>
      </Show>
      <Show when={tab() === "deployments"}><div class="deployments-layout"><aside class="deployment-history" aria-label="Deployment history"><For each={query.data?.deployments}>{d => <button class="deployment-entry" classList={{ selected: currentDeployment()?.id === d.id }} onClick={() => setSearch({ deployment: d.id })}><Status value={d.state} label={deploymentLabel(d)} /><span class="mono">{d.commit.slice(0, 8) || d.id.slice(0, 8)}</span><div class="deployment-meta"><time datetime={new Date(d.createdAt*1000).toISOString()}>{createdDate(d.createdAt)}</time><time class="deployment-duration" aria-label="Deployment duration" datetime={"PT"+deploymentSeconds(d, now())+"S"}>{formatDuration(deploymentSeconds(d, now()))}</time></div></button>}</For></aside><Show when={currentDeployment()} keyed>{d => <LogViewer deployment={d} restore={!busy() && d.state==="succeeded" && !!d.imageId && d.id!==a().activeDeploymentId && ["deploy","apply","rollback"].includes(d.operation)?()=>void restore(d.id):undefined} cancel={()=>void deploymentAction(d.id,"cancel")} retry={()=>void deploymentAction(d.id,"retry")}/>}</Show><Show when={!currentDeployment()&&historical.isFetching}><span class="loading-line" aria-label="Loading deployment" /></Show><Show when={search.deployment&&historical.error}><ErrorNotice message={historical.error!.message} /></Show><Show when={!currentDeployment()&&!search.deployment}><div class="empty-state">No deployments</div></Show></div></Show>
      <Show when={tab()==="overview"&&!search.service&&managed()}><Diagnostics applicationId={a().id} view={id=>setSearch({tab:"deployments",deployment:id})}/></Show>
      <Show when={tab() === "logs"}><RuntimeLogs application={a()} containerId={active()?.containerId} /></Show>
      <Show when={tab() === "domains"}><Domains application={a()} services={active()?.services} proxy={proxy.data} busy={busy()} save={domains} /></Show>
      <Show when={tab() === "env"}><Variables application={a()} busy={busy()} save={saveConfig} /></Show>
      <Show when={tab() === "backups"}><Backups application={a()} services={active()?.services??[]} save={saveConfig}/></Show>
      <Show when={tab() === "tasks"}><Tasks application={a()} services={active()?.services??[]} busy={busy()} deployments={query.data?.deployments??[]} view={id=>setSearch({tab:"deployments",deployment:id})}/></Show>
      <Show when={tab() === "settings"}><ApplicationSettings application={a()} showSource={search.section === "source"} busy={busy()} save={saveConfig} rename={rename} remove={remove} /></Show>
    </>}</Show>
    <Show when={review()}>{r=><ChangeReview plan={r().plan} close={closeReview} apply={applyReview}/>}</Show>
    <Show when={rollback()}>{value=><ChangeReview title="Restore release" confirm="Restore release" plan={value().plan} close={()=>setRollback(undefined)} apply={applyRollback}/>}</Show>
    <Show when={selected()}>{c => <ContainerDrawer container={c()} close={() => setSelected(undefined)} />}</Show>
  </div>;
}
