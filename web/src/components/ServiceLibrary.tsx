import { For, Show, createEffect, createMemo, createSignal, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import type { CatalogDomain, CatalogField, CatalogIndex, CatalogService, CatalogServiceSummary, Domain, StaticConfig, Variable } from "../types";
import { Button } from "./Controls";
import { Icon, type IconName } from "./Icon";
import ServiceLogo from "./ServiceLogo";
import { emptyConfig } from "./StaticFields";

type Props = {
  open: boolean;
  close: () => void;
  use: (request: { name: string; config: StaticConfig }) => Promise<void>;
  acme: boolean;
};

const categoryIcons: Record<string, IconName> = {
  Automation: "deploy", AI: "deploy", Analytics: "sliders", Auth: "lock", Backend: "code",
  CMS: "edit", Databases: "database", Devtools: "terminal", Git: "git", Media: "eye",
  Messaging: "plug", Monitoring: "clock", Productivity: "check", Security: "shield",
  Storage: "folder", Finance: "sliders", RSS: "plug", Featured: "layers", Other: "box",
};

function iconFor(category: string): IconName { return categoryIcons[category] ?? "box"; }
function roleIcon(role: string): IconName {
  switch (role) {
    case "Database": return "database";
    case "Cache": return "database";
    case "Queue": return "layers";
    case "Storage": return "folder";
    case "Proxy": return "globe";
    case "Worker": return "terminal";
    default: return "box";
  }
}

function slug(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 48) || "service";
}

function random(kind: string) {
  const bytes = crypto.getRandomValues(new Uint8Array(kind === "hex" ? 32 : 32));
  if (kind === "hex") return Array.from(bytes, b => b.toString(16).padStart(2, "0")).join("");
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", "");
}

function initialValue(field: CatalogField) {
  if (field.generate) return random(field.generate);
  return field.default ?? "";
}

function optionLabel(value: string) {
  if (value === "true") return "Enabled";
  if (value === "false") return "Disabled";
  return value;
}

function hostFor(id: string, index: number, port: number) {
  const base = slug(id);
  if (index === 0) return base + ".localhost";
  return base + "-" + port + ".localhost";
}

export default function ServiceLibrary(props: Props) {
  const [search, setSearch] = createSignal("");
  const [category, setCategory] = createSignal("Featured");
  const [selected, setSelected] = createSignal<string>();
  const [values, setValues] = createSignal<Record<string, string>>({});
  const [hostnames, setHostnames] = createSignal<Record<string, string>>({});
  const [secure, setSecure] = createSignal<Record<string, boolean>>({});
  const [name, setName] = createSignal("");
  const [advanced, setAdvanced] = createSignal(false);
  const [revealed, setRevealed] = createSignal<Record<string, boolean>>({});
  const [error, setError] = createSignal("");
  const [using, setUsing] = createSignal(false);
  let searchInput: HTMLInputElement | undefined;
  let dialog: HTMLElement | undefined;
  let results: HTMLDivElement | undefined;
  let previousFocus: HTMLElement | undefined;
  let backButton: HTMLButtonElement | undefined;
  let initialized = "";

  const index = createQuery(() => ({ queryKey: ["catalog"], queryFn: () => api<CatalogIndex>("/catalog"), enabled: props.open, staleTime: 5 * 60_000 }));
  const detail = createQuery(() => ({ queryKey: ["catalog", selected()], queryFn: () => api<CatalogService>("/catalog/" + selected()!), enabled: !!selected() && props.open, staleTime: 10 * 60_000 }));

  const items = createMemo(() => index.data?.items ?? []);
  const categories = createMemo(() => [{ name: "All", count: items().length }, ...(index.data?.categories ?? []).filter(c => c.count > 0)]);
  const filtered = createMemo(() => {
    const query = search().trim().toLowerCase();
    const active = category();
    return items().filter(item => {
      if (active === "Featured" && !item.featured) return false;
      if (active !== "All" && active !== "Featured" && item.category !== active) return false;
      if (!query) return true;
      return (item.name + " " + item.description + " " + item.category + " " + (item.tags ?? []).join(" ")).toLowerCase().includes(query);
    });
  });
  const service = () => detail.data;
  const fields = createMemo(() => service()?.fields ?? []);
  const domainFields = createMemo(() => fields().filter(field => field.kind === "domain"));
  const accessFields = createMemo(() => fields().filter(field => field.group === "access" && field.kind !== "domain"));
  const configList = createMemo(() => fields().filter(field => field.group !== "access" && field.kind !== "domain" && !field.advanced && field.group !== "advanced"));
  const configFields = createMemo(() => configList().slice(0, 10));
  const advancedFields = createMemo(() => {
    const explicit = fields().filter(field => field.kind !== "domain" && (field.advanced || field.group === "advanced"));
    const seen = new Set(explicit.map(field => field.key));
    return [...explicit, ...configList().slice(10).filter(field => !seen.has(field.key))];
  });
  const domainRows = createMemo<CatalogDomain[]>(() => {
    const value = service();
    if (!value) return [];
    const rows: CatalogDomain[] = [];
    const seen = new Set<string>();
    for (const domain of value.domains ?? []) {
      if (seen.has(domain.service)) continue;
      seen.add(domain.service);
      rows.push(domain);
    }
    return rows;
  });

  function reset() {
    initialized = "";
    setSearch("");
    setCategory("Featured");
    setSelected(undefined);
    setValues({});
    setHostnames({});
    setSecure({});
    setName("");
    setAdvanced(false);
    setRevealed({});
    setError("");
    setUsing(false);
  }

  function openService(id: string) {
    setSelected(id);
    setError("");
    setAdvanced(false);
    setRevealed({});
    requestAnimationFrame(() => backButton?.focus({ preventScroll: true }));
  }
  function closeService() {
    setSelected(undefined);
    requestAnimationFrame(() => searchInput?.focus({ preventScroll: true }));
  }
  function chooseCategory(name: string) {
    setCategory(name);
    results?.scrollTo({ top: 0 });
  }

  createEffect(() => {
    const value = detail.data;
    if (!value || value.id !== selected()) return;
    if (value.id === initialized) return;
    initialized = value.id;
    if (detail.isFetched && detail.dataUpdatedAt === 0) return;
    const nextValues: Record<string, string> = {};
    for (const field of value.fields) {
      if (field.kind !== "domain") nextValues[field.key] = initialValue(field);
    }
    const nextHosts: Record<string, string> = {};
    const nextSecure: Record<string, boolean> = {};
    domainRows().forEach((domain, index) => {
      const key = "service:" + domain.service;
      nextHosts[key] = hostFor(value.id, index, domain.port);
      nextSecure[key] = false;
    });
    setValues(nextValues);
    setHostnames(nextHosts);
    setSecure(nextSecure);
    setName(value.name);
    setError("");
  });

  createEffect(() => {
    if (!props.open) { reset(); return; }
    previousFocus = document.activeElement as HTMLElement | undefined;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    queueMicrotask(() => searchInput?.focus());
    const keydown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !using()) {
        event.preventDefault();
        if (selected()) closeService(); else props.close();
      }
      if (event.key === "Tab" && dialog) {
        const focusable = [...dialog.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], [tabindex]:not([tabindex="-1"])')].filter(element => element.offsetParent !== null);
        if (!focusable.length) return;
        const first = focusable[0], last = focusable[focusable.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
      }
    };
    document.addEventListener("keydown", keydown);
    onCleanup(() => {
      document.body.style.overflow = overflow;
      document.removeEventListener("keydown", keydown);
      previousFocus?.focus?.({ preventScroll: true });
    });
  });

  onCleanup(() => {
    previousFocus?.focus?.({ preventScroll: true });
  });

  function value(key: string) { return values()[key] ?? ""; }
  function setValue(key: string, next: string) { setValues(current => ({ ...current, [key]: next })); }
  function regenerate(field: CatalogField) {
    if (!field.generate) return;
    setValue(field.key, random(field.generate));
    setRevealed(current => ({ ...current, [field.key]: true }));
  }
  function hostKey(domain: CatalogDomain) { return "service:" + domain.service; }
  function domainValue(domain: CatalogDomain) { return hostnames()[hostKey(domain)] ?? ""; }
  function setDomainValue(domain: CatalogDomain, next: string) {
    setHostnames(current => ({ ...current, [hostKey(domain)]: next.toLowerCase().replace(/\s+/g, "") }));
  }
  function domainSecure(domain: CatalogDomain) { return !!secure()[hostKey(domain)]; }
  function setDomainSecure(domain: CatalogDomain, next: boolean) {
    setSecure(current => ({ ...current, [hostKey(domain)]: next }));
  }

  function generatedConfig(service: CatalogService): StaticConfig {
    const base = emptyConfig();
    const variables: Variable[] = [];
    for (const field of service.fields) {
      if (field.kind === "domain") continue;
      const current = value(field.key);
      const changed = current !== (field.default ?? "");
      if (!current && !field.required && !field.generate) continue;
      if (!changed && !field.required && !field.generate && !field.secret && field.group !== "access") continue;
      variables.push({ name: field.key, value: current, secret: !!field.secret || field.kind === "password", scope: "runtime", hasValue: !!current });
    }
    const domains: Domain[] = [];
    for (const domain of domainRows()) {
      const hostname = domainValue(domain).trim();
      if (!hostname) continue;
      const https = domainSecure(domain) && !hostname.endsWith(".localhost");
      domains.push({
        id: "catalog-" + domain.service + (domains.length ? "-" + domains.length : ""),
        hostname, service: domain.service, https, primary: true, redirect: false,
      });
    }
    // Every FQDN variable for a service receives the same public URL. Some
    // templates declare one variable per exposed port.
    for (const field of domainFields()) {
      const row = domainRows().find(domain => domain.service === field.service);
      if (!row) continue;
      const hostname = domainValue(row).trim();
      if (!hostname) continue;
      const secure = domainSecure(row) && !hostname.endsWith(".localhost");
      const url = (secure ? "https://" : "http://") + hostname;
      variables.push({ name: field.key, value: url, secret: false, scope: "runtime", hasValue: true });
    }
    // A primary route exists once per service, not once per hostname.
    const primaries = new Set<string>();
    for (const domain of domains) {
      const name = domain.service ?? "";
      if (domain.primary && primaries.has(name)) domain.primary = false;
      if (domain.primary) primaries.add(name);
    }
    for (const item of service.services) {
      if (!primaries.has(item.name)) {
        const existing = domains.find(domain => domain.service === item.name);
        if (existing) { existing.primary = true; primaries.add(item.name); }
      }
    }
    const primary = domains[0];
    return {
      ...base,
      method: "compose",
      composeSource: "paste",
      composeFile: "compose.yaml",
      composeYAML: service.compose,
      startupPolicy: "http",
      port: 0,
      healthPath: service.healthPath || "/",
      memoryMB: 512,
      variables,
      volumes: [],
      domains,
      domain: primary?.hostname ?? "",
      https: primary?.https ?? false,
      services: service.services.map(item => ({
        name: item.name,
        port: item.name === service.mainService
          ? (service.port || item.port || undefined)
          : (["Application", "Service", "Proxy"].includes(item.role) && item.port ? item.port : undefined),
        healthPath: "/",
        memoryMB: 0,
        variables: [],
      })),
      catalogService: service.id,
      catalogVersion: index.data?.version ?? "",
    };
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    const item = service();
    if (!item || using()) return;
    setError("");
    if (!name().trim()) { setError("Name the service"); return; }
    for (const field of fields()) {
      if (field.kind === "domain") continue;
      if (field.required && !value(field.key)) { setError("Add " + field.label.toLowerCase()); return; }
    }
    setUsing(true);
    try {
      await props.use({ name: name().trim(), config: generatedConfig(item) });
      reset();
      props.close();
    } catch (reason) {
      setError((reason as Error).message || "The service could not be prepared");
    } finally {
      setUsing(false);
    }
  }

  return <Show when={props.open}><Portal><div class="library-backdrop" role="presentation" onPointerDown={event => { if (event.target === event.currentTarget && !using()) props.close(); }}>
    <section ref={dialog} class="library-dialog" role="dialog" aria-modal="true" aria-label="Service Library">
      <header class="library-heading">
        <div class="library-heading-title"><span class="library-heading-mark"><Icon name="layers" /></span><div><h2>Service Library</h2><span>{items().length ? items().length + " services" : "Loading services"}</span></div></div>
        <button type="button" class="library-close" aria-label="Close Service Library" disabled={using()} onClick={props.close}><Icon name="close" /></button>
      </header>
      <Show when={!selected()} fallback={<form class="library-detail" onSubmit={submit}>
        <Show when={detail.isPending}><div class="library-detail-loading" aria-label="Loading service"><span class="loading-line" /></div></Show>
        <Show when={detail.error}><div class="library-alert" role="alert">{detail.error?.message}<Button small variant="ghost" onClick={() => void detail.refetch()}>Retry</Button></div></Show>
        <Show when={service()}>{item => <>
          <div class="library-detail-scroll">
            <button ref={backButton} type="button" class="library-back" disabled={using()} onClick={closeService}><Icon name="back" />Service Library</button>
            <header class="library-service-heading">
              <ServiceLogo logo={item().logo} mark={item().logomark} class="library-mark large" />
              <div class="library-service-copy">
                <div class="library-service-title"><h3>{item().name}</h3><span class={"library-support " + item().support}>{item().support === "ready" ? "Ready" : "Advanced"}</span></div>
                <p>{item().description}</p>
                <div class="library-service-meta">
                  <span><Icon name={iconFor(item().category)} />{item().category}</span>
                  <span>{item().services.length} {item().services.length === 1 ? "service" : "services"}</span>
                  <Show when={item().documentation}><a href={item().documentation} target="_blank" rel="noreferrer">Documentation<Icon name="external" /></a></Show>
                </div>
              </div>
            </header>
            <Show when={item().support === "advanced"}><div class="library-note"><Icon name="warning" /><span>{item().supportNote || "Review this template after deploy"}</span></div></Show>
            <section class="library-includes" aria-label="Included services">
              <For each={item().services}>{part => <div class="library-include"><span class="library-include-icon"><Icon name={roleIcon(part.role)} /></span><span><strong>{part.name}</strong><small>{part.role}</small></span></div>}</For>
            </section>
            <div class="library-config">
              <label class="library-field"><span>Name</span><input class="input" value={name()} maxlength={80} onInput={event => setName(event.currentTarget.value)} autocomplete="off" /></label>
              <Show when={domainRows().length}>
                <section class="library-group"><h4>Domain</h4><div class="library-domain-list">
                  <For each={domainRows()}>{domain => <div class="library-domain">
                    <label class="library-field"><span>{domain.service}</span><input class="input mono" value={domainValue(domain)} placeholder="app.example.com" autocomplete="off" onInput={event => setDomainValue(domain, event.currentTarget.value)} /></label>
                    <label class="check-field"><input type="checkbox" checked={domainSecure(domain)} disabled={!props.acme || domainValue(domain).endsWith(".localhost")} onChange={event => setDomainSecure(domain, event.currentTarget.checked)} />HTTPS</label>
                  </div>}</For>
                </div></section>
              </Show>
              <Show when={accessFields().length}>
                <section class="library-group"><h4>Access</h4><div class="library-field-grid">
                  <For each={accessFields()}>{field => <label class="library-field"><span>{field.label}</span>
                    <div class="library-secret">
                      <input class="input" type={field.secret && !revealed()[field.key] ? "password" : "text"} value={value(field.key)} placeholder={field.placeholder} autocomplete="off" onInput={event => setValue(field.key, event.currentTarget.value)} />
                      <Show when={field.secret}><button type="button" class="library-icon-button" aria-label={revealed()[field.key] ? "Hide " + field.label : "Show " + field.label} onClick={() => setRevealed(current => ({ ...current, [field.key]: !current[field.key] }))}><Icon name="eye" /></button></Show>
                      <Show when={field.generate}><button type="button" class="library-icon-button" aria-label={"Generate " + field.label} onClick={() => regenerate(field)}><Icon name="refresh" /></button></Show>
                    </div>
                  </label>}</For>
                </div></section>
              </Show>
              <Show when={configFields().length}>
                <section class="library-group"><h4>Configuration</h4><div class="library-field-grid">
                  <For each={configFields()}>{field => <label class="library-field" classList={{ wide: field.kind === "textarea" }}><span>{field.label}</span>
                    <Show when={field.kind === "select"} fallback={<Show when={field.kind === "textarea"} fallback={<input class="input" type={field.kind === "number" ? "number" : field.kind === "email" ? "email" : "text"} value={value(field.key)} placeholder={field.placeholder ?? field.default} autocomplete="off" onInput={event => setValue(field.key, event.currentTarget.value)} />}><textarea class="input" rows={3} value={value(field.key)} onInput={event => setValue(field.key, event.currentTarget.value)} /></Show>}>
                      <select class="input" value={value(field.key)} onChange={event => setValue(field.key, event.currentTarget.value)}><For each={field.options ?? []}>{option => <option value={option}>{optionLabel(option)}</option>}</For></select>
                    </Show>
                  </label>}</For>
                </div></section>
              </Show>
              <Show when={advancedFields().length}><details class="library-advanced" open={advanced()} onToggle={event => setAdvanced(event.currentTarget.open)}>
                <summary>Advanced<Icon name="down" /></summary>
                <div class="library-field-grid"><For each={advancedFields()}>{field => <label class="library-field"><span>{field.label}</span><input class="input" type={field.kind === "number" ? "number" : "text"} value={value(field.key)} placeholder={field.placeholder} onInput={event => setValue(field.key, event.currentTarget.value)} /></label>}</For></div>
              </details></Show>
            </div>
          </div>
          <footer class="library-footer">
            <Show when={error()}><div class="library-alert" role="alert">{error()}</div></Show>
            <div class="library-footer-actions"><Button variant="ghost" type="button" disabled={using()} onClick={props.close}>Cancel</Button><Button variant="primary" type="submit" disabled={using() || detail.isPending}>{using() ? "Preparing deployment" : "Review deployment"}</Button></div>
          </footer>
        </>}</Show>
      </form>}>
        <div class="library-browse">
          <div class="library-search"><Icon name="search" /><input ref={searchInput} type="search" value={search()} placeholder="Search services" aria-label="Search services" onInput={event => { setSearch(event.currentTarget.value); results?.scrollTo({ top: 0 }); }} /><Show when={search()}><button type="button" aria-label="Clear search" onClick={() => { setSearch(""); results?.scrollTo({ top: 0 }); searchInput?.focus(); }}><Icon name="close" /></button></Show></div>
          <div class="library-layout">
            <nav class="library-categories" aria-label="Service categories">
              <For each={categories()}>{entry => <button type="button" classList={{ active: category() === entry.name }} aria-pressed={category() === entry.name} onClick={() => chooseCategory(entry.name)}><span>{entry.name}</span><small>{entry.count}</small></button>}</For>
            </nav>
            <div ref={results} class="library-results">
              <Show when={index.error}><div class="library-alert" role="alert">{index.error?.message}<Button small variant="ghost" onClick={() => void index.refetch()}>Retry</Button></div></Show>
              <Show when={index.isPending}><div class="library-skeletons"><For each={[0,1,2,3,4,5]}>{() => <span class="library-skeleton" />}</For></div></Show>
              <Show when={!index.isPending && !filtered().length}><div class="library-empty"><Icon name="search" /><strong>No matching services</strong><span>Try a different category or search.</span></div></Show>
              <div class="library-grid" role="list">
                <For each={filtered()}>{item => <button type="button" class="library-tile" role="listitem" onClick={() => openService(item.id)}>
                  <ServiceLogo logo={item.logo} mark={item.logomark} class="library-mark" />
                  <span class="library-tile-copy"><strong>{item.name}</strong><small>{item.description}</small></span>
                  <span class="library-tile-meta"><span><Icon name={iconFor(item.category)} />{item.category}</span><Show when={item.support === "advanced"}><span class="library-support advanced">Advanced</span></Show><Show when={item.services > 1}><span>{item.services} services</span></Show></span>
                </button>}</For>
              </div>
            </div>
          </div>
        </div>
      </Show>
    </section>
  </div></Portal></Show>;
}

export function ServiceLibraryEntry(props: { open: () => void; count?: number }) {
  return <button type="button" class="library-entry" onClick={props.open}>
    <span class="library-entry-icon"><Icon name="layers" /></span>
    <span><strong>Service Library</strong><small>{props.count ? props.count + " ready-made services" : "360+ ready-made services"}</small></span>
    <Icon name="chevron" />
  </button>;
}

export function serviceLibrarySummary(items: CatalogServiceSummary[] | undefined) { return items?.length ?? 0; }
