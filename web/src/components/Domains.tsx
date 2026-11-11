import EditorActions from "./EditorActions";
import DomainOptions, { DomainCertificate } from "./DomainOptions";
import { createUniqueId, For, Show, createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api, post } from "../api";
import { docs } from "../docs";
import { useAuth } from "../context";
import type { Application, Domain, ProxyConfig, Service } from "../types";
import { domainURL } from "../data";
import { Button, ErrorNotice, IconButton, Status, validateForm } from "./Controls";
import { Icon } from "./Icon";

export default function Domains(props: { application: Application; proxy?: ProxyConfig; services?: Service[]; service?: string; busy: boolean; save: (domains: Domain[]) => Promise<void>; configureService?: () => void; selectedHostname?: string }) {
  const saved = () => (props.application.domains ?? []).filter(d => !props.service || d.service === props.service);
  const formId = createUniqueId(), auth = useAuth(), [domains, setDomains] = createSignal<Domain[]>(saved()), [hostname, setHostname] = createSignal(""), [lookup, setLookup] = createSignal(""), [httpsChoice, setHTTPS] = createSignal<boolean>(), [www, setWWW] = createSignal(false), [error, setError] = createSignal(""), [checks, setChecks] = createSignal<Record<string, boolean>>({});
  const groups = createMemo(() => [...new Set(domains().map(d => d.service || ""))]);
  const groupIDs = (service: string) => domains().filter(domain => (domain.service || "") === service).map(domain => domain.id);
  const [editing, setEditing] = createSignal<string | undefined>(saved().find(domain => domain.hostname === props.selectedHostname)?.id);
  const editorId = formId + "-route-editor";
  let editor: HTMLElement | undefined;
  const domainFor = (id: string) => createMemo<Domain>(previous => domains().find(d => d.id === id) ?? previous, domains().find(d => d.id === id)!);
  function edit(id: string) {
    const invalid = editing() ? editor?.querySelector<HTMLInputElement | HTMLTextAreaElement>(":invalid") : undefined;
    if (invalid) {
      const section = invalid.closest("details");
      if (section) section.open = true;
      invalid.reportValidity();
      return;
    }
    setEditing(editing() === id ? undefined : id);
  }
  const route = (d: Domain) => JSON.stringify([d.id, d.hostname, d.service || "", d.https, d.primary, d.redirect, d.pathPrefix || "", !!d.stripPrefix, !!d.compress, !!d.maintenance, Object.entries(d.responseHeaders ?? {}).sort()]);
  const fingerprint = (items: Domain[]) => items.map(route).sort().join("\n");
  const dirty = () => fingerprint(domains()) !== fingerprint(saved());
  const compose = () => props.application.method === "compose";
  const availableServices = () => props.services?.filter(s => s.port > 0 && !s.oneShot) ?? [];
  const pickService = () => compose() && !props.service;
  const canAdd = () => !compose() || (props.service ? availableServices().some(s => s.name === props.service) : availableServices().length > 0);
  const https = () => httpsChoice() ?? (!!props.proxy?.autoHTTPS && !!props.proxy?.acmeEmail);
  createEffect(() => { const name = hostname();const timer = setTimeout(() => setLookup(name), 250);onCleanup(() => clearTimeout(timer)); });
  const [selectedService, setService] = createSignal("");
  const service = () => props.service || selectedService() || availableServices()[0]?.name || "";
  const options = createQuery(() => ({ queryKey: ["domain-options", lookup()], queryFn: () => api<{ canIncludeWWW: boolean }>("/domain-options?hostname=" + encodeURIComponent(lookup())), enabled: lookup().includes("."), staleTime: 60000 }));
  function update(id: string, value: Partial<Domain>) { setDomains(current => current.map(d => d.id === id ? { ...d, ...value, verificationState: "", message: "" } : d)); }
  function primary(id: string) { setDomains(current => { const target = current.find(d => d.id === id);return current.map(d => d.service === target?.service ? { ...d, primary: d.id === id, redirect: d.id === id ? false : d.redirect } : d); }); }
  function add() {
    const name = hostname().trim().toLowerCase();
    if (!name || !canAdd() || props.busy) return;
    setError("");
    const alias = www() && options.data?.canIncludeWWW && lookup() === hostname();
    if ((props.application.domains ?? []).some(d => props.service && d.service !== props.service && (d.hostname === name || alias && d.hostname === "www." + name)) ||
      domains().some(d => d.hostname === name || alias && d.hostname === "www." + name)) {
      setError("This domain is already listed"); return;
    }
    const target = compose() ? service() || undefined : undefined;
    const entries: Domain[] = [{ id: crypto.randomUUID(), hostname: name, https: https() && !name.endsWith(".localhost"), primary: !domains().some(d => d.service === target), service: target, redirect: false }];
    if (alias) entries.push({ id: crypto.randomUUID(), hostname: "www." + name, https: https(), service: target, primary: false, redirect: true });
    setDomains(current => [...current, ...entries]); setHostname(""); setWWW(false); setHTTPS(undefined);
  }
  async function save(e: SubmitEvent) { e.preventDefault(); if (!dirty() || props.busy || !validateForm(e.currentTarget as HTMLFormElement)) return;setError("");try { await props.save(props.service ? [...(props.application.domains ?? []).filter(d => d.service !== props.service), ...domains()] : domains()); } catch (e) { setError((e as Error).message); } }
  async function verify(domain: Domain) { setChecks(value => ({ ...value, [domain.id]: true }));setError("");try { await post(`/applications/${props.application.id}/domain-check`, { domainId: domain.id }, auth.session()?.csrfToken); } catch (e) { setError((e as Error).message); }finally { setChecks(value => ({ ...value, [domain.id]: false })); } }
  const savedHost = (domain: Domain) => saved().find(d => d.id === domain.id && d.hostname === domain.hostname && d.https === domain.https && d.service === domain.service);
  const stored = (domain: Domain) => savedHost(domain) && saved().find(d => route(d) === route(domain));
  function remove(id: string) {
    const item = domains().find(d => d.id === id);
    if (!item) return;
    if (editing() === id) setEditing(undefined);
    setDomains(current => {
      const next = current.filter(d => d.id !== id), group = next.filter(d => d.service === item.service);
      return group.length && !group.some(d => d.primary)
        ? next.map(d => d.id === group[0].id ? { ...d, primary: true, redirect: false } : d) : next;
    });
  }
  function move(id: string, target: string) {
    const previous = domains().find(d => d.id === id);
    if (!previous || previous.service === target) return;
    const remaining = domains().find(d => d.id !== id && d.service === previous.service);
    const destination = domains().some(d => d.id !== id && d.service === target);
    setDomains(items => items.map(d => {
      if (d.id === id) return { ...d, service: target, primary: !destination, redirect: false };
      if (previous.primary && d.id === remaining?.id) return { ...d, primary: true, redirect: false };
      return d;
    }));
  }
  return <form id={formId} noValidate onSubmit={save} class="domains-panel"><Show when={error()}><ErrorNotice message={error()} /></Show><Show when={props.application.latestDeployment?.operation === "domains" && props.application.latestDeployment.state === "failed"}><ErrorNotice message={props.application.latestDeployment!.error} /></Show>
    <Show when={canAdd()} fallback={<div class="domain-unavailable"><span>No HTTP port configured</span><Show when={props.configureService}><Button icon="settings" onClick={() => props.configureService?.()}>Service settings</Button></Show></div>}>
      <div class="domain-add">
        <Show when={pickService()}><select class="input domain-service" aria-label="Domain service" value={service()} onChange={e => setService(e.currentTarget.value)}><option value="" disabled>Choose service</option><For each={availableServices()}>{s => <option value={s.name}>{s.name}</option>}</For></select></Show>
        <div class="field domain-add-field"><label for="new-domain">New domain</label><input class="input" id="new-domain" placeholder="app.example.com" value={hostname()} onInput={e => { setHostname(e.currentTarget.value); setWWW(false); }} onKeyDown={e => { if (e.key === "Enter") { e.preventDefault(); add(); } }} /></div>
        <Show when={hostname() && props.proxy?.acmeEmail && !hostname().endsWith(".localhost")}><label class="check-field"><input type="checkbox" checked={https()} onChange={e => setHTTPS(e.currentTarget.checked)} />HTTPS</label></Show>
        <Show when={options.data?.canIncludeWWW && lookup() === hostname()}><label class="check-field"><input type="checkbox" checked={www()} onChange={e => setWWW(e.currentTarget.checked)} />Include www</label></Show>
        <Button icon="plus" onClick={add} disabled={!hostname().trim() || props.busy || compose() && !service()}>Add domain</Button>
      </div>
    </Show>
    <div class="domain-workspace" classList={{ "has-route": !!editing() }}><div class="domain-list"><For each={groups()}>{group => <div class="domain-group">
      <Show when={pickService() && group}><h3>{group}</h3></Show>
      <For each={groupIDs(group)}>{id => {
      const domain = domainFor(id);
      const target = () => domains().find(d => d.service === domain().service && d.primary)?.hostname;
      const verified = () => stored(domain())?.verificationState === "verified";
      return <><article class="domain-card" classList={{ "route-selected": editing() === id }}>
        <header class="domain-card-heading">
          <div class="domain-identity">
            <Show when={stored(domain()) && props.application.activeDeploymentId} fallback={<strong>{domain().hostname}</strong>}>
              <a class="inline-link" href={domainURL(domain(), props.proxy)} target="_blank" rel="noreferrer">{domain().hostname}<Icon name="external" /></a>
            </Show>
            <div class="domain-facts">
              <span>{domain().https ? "HTTPS" : "HTTP"}</span>
              <Show when={domain().primary}><span>Primary</span></Show>
              <Show when={domain().redirect} fallback={<code>{domain().pathPrefix || "/"}</code>}><span>Redirect to <strong>{target() || "primary domain"}</strong></span></Show>
              <Show when={verified()}><Status value="running" label="Verified" /></Show>
              <Show when={!stored(domain())}><Status value="queued" label="Unsaved" /></Show>
            </div>
          </div>
          <div class="toolbar-actions">
            <Show when={stored(domain()) && props.application.activeDeploymentId}>
              <Button small aria-label={"Verify " + domain().hostname} icon="refresh" disabled={checks()[id] || props.busy} onClick={() => void verify(domain())}>{checks()[id] ? "Checking" : verified() ? "Recheck" : "Verify"}</Button>
            </Show>
            <Button small icon="sliders" variant="ghost" aria-label={"Edit " + domain().hostname}
              aria-expanded={editing() === id} aria-controls={editing() === id ? editorId : undefined} onClick={() => edit(id)}>{editing() === id ? "Close" : "Edit"}</Button>
            <IconButton icon="trash" label={"Remove " + domain().hostname} onClick={() => remove(id)} />
          </div>
        </header>
        <Show when={stored(domain())?.message && !verified()}><div class="domain-row-error" role="status">{stored(domain())?.message}</div></Show>
      </article>
      <Show when={editing() === id}>
        <section ref={editor} id={editorId} class="domain-editor" aria-labelledby={editorId + "-title"}>
          <h3 id={editorId + "-title"}>Edit route</h3>
          <div class="form-fields field-pairs domain-editor-grid">
            <div class="field" classList={{ "field-wide": !pickService() }}><label for={"host-" + id}>Hostname</label><input id={"host-" + id} class="input" aria-label={"Hostname " + domain().hostname} value={domain().hostname} required onInput={e => update(id, { hostname: e.currentTarget.value.trim().toLowerCase(), ...(e.currentTarget.value.endsWith(".localhost") ? { https: false } : {}) })} /></div>
            <Show when={pickService()}><div class="field"><label for={"service-" + id}>Service</label><select id={"service-" + id} class="input" aria-label={"Service for " + domain().hostname} value={domain().service || ""} onChange={e => move(id, e.currentTarget.value)}><For each={availableServices()}>{s => <option value={s.name}>{s.name}</option>}</For></select></div></Show>
            <div class="domain-toggles">
              <Show when={props.proxy?.acmeEmail || domain().https}><label class="check-field"><input type="checkbox" aria-label={"HTTPS for " + domain().hostname} checked={domain().https} disabled={domain().hostname.endsWith(".localhost")} onChange={e => update(id, { https: e.currentTarget.checked })} />HTTPS</label></Show>
              <label class="check-field"><input type="radio" name={"primary-domain-" + (domain().service || "app")} aria-label={"Primary " + domain().hostname} checked={domain().primary} onChange={() => primary(id)} />Primary domain</label>
              <Show when={!domain().primary}><label class="check-field"><input type="checkbox" aria-label={"Redirect " + domain().hostname} checked={domain().redirect} onChange={e => update(id, { redirect: e.currentTarget.checked })} />Redirect to primary</label></Show>
            </div>
          </div>
          <DomainOptions domain={domain()} change={value => update(id, value)} />
          <DomainCertificate domain={savedHost(domain())} />
        </section>
      </Show>
      </>;
    }}</For></div>}</For><Show when={!domains().length && canAdd()}><div class="empty-state">No domains <a class="docs-link" href={docs("domains-tls/")} target="_blank" rel="noreferrer">Domain guide</a></div></Show></div></div>
    <Show when={domains().length || dirty()}><EditorActions docked={dirty()} measure="form" inlineWhenFits><Button variant="primary" type="submit" form={formId} icon="check" disabled={props.busy || !dirty()}>Save domains</Button></EditorActions></Show>
  </form>;
}
