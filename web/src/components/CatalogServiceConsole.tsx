import { For, Show, createEffect, createMemo, createSignal } from "solid-js";
import type { Application, CatalogDomain, CatalogField, CatalogService, Domain, ProxyConfig, StaticConfig, Variable } from "../types";
import { domainURL, primaryDomain, statusOf } from "../data";
import { Button, ErrorNotice, Status, ActionMenu } from "./Controls";
import { Icon } from "./Icon";
import { useAuth } from "../context";
import { post } from "../api";

function fieldValue(values: Map<string, Variable>, field: CatalogField) {
  return values.get(field.key)?.value ?? field.default ?? "";
}

function domainKey(domain: CatalogDomain) {
  return "service:" + domain.service;
}

function variableURL(hostname: string, https: boolean) {
  return (https ? "https://" : "http://") + hostname;
}

function optionLabel(value: string) {
  if (value === "true") return "Enabled";
  if (value === "false") return "Disabled";
  return value;
}

export function CatalogSummary(props: { service: CatalogService; proxy?: ProxyConfig; application: Application; manage: () => void }) {
  const domain = () => primaryDomain(props.application);
  return <section class="catalog-summary">
    <span class="catalog-mark">{props.service.logomark}</span>
    <div class="catalog-summary-copy">
      <div class="catalog-summary-title"><strong>{props.service.name}</strong><span class="catalog-chip">{props.service.category}</span></div>
      <p>{props.service.description}</p>
    </div>
    <div class="catalog-summary-actions">
      <Show when={domain()}><a class="button ghost small" href={domainURL(domain()!, props.proxy)} target="_blank" rel="noreferrer">Open<Icon name="external" /></a></Show>
      <Button small variant="neutral" onClick={props.manage}>Manage<Icon name="chevron" /></Button>
    </div>
  </section>;
}

export function CatalogServiceConsole(props: {
  application: Application;
  service: CatalogService;
  proxy?: ProxyConfig;
  busy: boolean;
  save: (config: StaticConfig) => Promise<boolean | void>;
  selectService?: (name: string) => void;
  lifecycle?: (action: string, service?: string) => void;
}) {
  const auth = useAuth();
  const [values, setValues] = createSignal<Map<string, Variable>>(new Map());
  const [hostnames, setHostnames] = createSignal<Record<string, string>>({});
  const [https, setHTTPS] = createSignal<Record<string, boolean>>({});
  const [revealed, setRevealed] = createSignal<Record<string, string>>({});
  const [error, setError] = createSignal("");
  const [saving, setSaving] = createSignal(false);
  const [saved, setSaved] = createSignal(false);
  let initialized = "";

  const fields = createMemo(() => props.service.fields ?? []);
  const domains = createMemo(() => props.service.domains ?? []);
  const domainRows = createMemo(() => {
    const rows: CatalogDomain[] = [];
    const seen = new Set<string>();
    for (const domain of domains()) {
      if (seen.has(domain.service)) continue;
      seen.add(domain.service);
      rows.push(domain);
    }
    return rows;
  });
  const access = createMemo(() => fields().filter(field => field.group === "access" && field.kind !== "domain"));
  const configList = createMemo(() => fields().filter(field => field.group !== "access" && field.kind !== "domain" && !field.advanced && field.group !== "advanced"));
  const config = createMemo(() => configList().slice(0, 10));
  const advanced = createMemo(() => {
    const explicit = fields().filter(field => field.kind !== "domain" && (field.advanced || field.group === "advanced"));
    const seen = new Set(explicit.map(field => field.key));
    return [...explicit, ...configList().slice(10).filter(field => !seen.has(field.key))];
  });
  const primary = () => primaryDomain(props.application);
  const mainImage = () => props.application.containers.find(container => container.service === props.service.mainService)?.image || props.service.services.find(part => part.name === props.service.mainService)?.image || "";

  createEffect(() => {
    const signature = props.service.id + JSON.stringify(props.application.config.variables ?? []) + JSON.stringify(props.application.domains ?? []);
    if (signature === initialized) return;
    initialized = signature;
    const current = new Map<string, Variable>();
    for (const variable of props.application.config.variables ?? []) current.set(variable.name, variable);
    setValues(current);
    const nextHosts: Record<string, string> = {};
    const nextHTTPS: Record<string, boolean> = {};
    domainRows().forEach(domain => {
      const key = domainKey(domain);
      const existing = (props.application.domains ?? []).find(item => item.service === domain.service && item.hostname)
        ?? (props.application.domains ?? []).find(item => item.service === domain.service);
      nextHosts[key] = existing?.hostname ?? (domain.service === props.application.config.domain ? props.application.config.domain : "");
      nextHTTPS[key] = existing?.https ?? (props.application.config.https && domain.service === props.application.config.domain);
    });
    setHostnames(nextHosts);
    setHTTPS(nextHTTPS);
    setRevealed({});
    setError("");
  });

  function update(field: CatalogField, value: string) {
    setValues(current => {
      const next = new Map(current);
      const previous = next.get(field.key);
      next.set(field.key, { name: field.key, value, secret: !!field.secret || field.kind === "password", scope: "runtime", keep: false, hasValue: !!value || !!previous?.hasValue });
      return next;
    });
    setSaved(false);
  }
  function updateHost(domain: CatalogDomain, value: string) {
    const key = domainKey(domain);
    setHostnames(current => ({ ...current, [key]: value.trim().toLowerCase() }));
    setSaved(false);
  }
  function updateHTTPS(domain: CatalogDomain, value: boolean) {
    setHTTPS(current => ({ ...current, [domainKey(domain)]: value }));
    setSaved(false);
  }
  async function reveal(field: CatalogField) {
    setError("");
    try {
      const result = await post<{ value: string }>(`/applications/${props.application.id}/variables/reveal`, { name: field.key }, auth.session()?.csrfToken);
      setRevealed(current => ({ ...current, [field.key]: result.value }));
      setValues(current => {
        const next = new Map(current);
        const previous = next.get(field.key);
        next.set(field.key, { ...(previous ?? { name: field.key, secret: true, scope: "runtime" }), value: result.value, hasValue: true });
        return next;
      });
    } catch (reason) { setError((reason as Error).message); }
  }
  async function copy(value: string) {
    try { await navigator.clipboard.writeText(value); setSaved(true); setTimeout(() => setSaved(false), 1600); } catch { /* Clipboard access is optional. */ }
  }

  function configForSave(): StaticConfig {
    const current = props.application.config;
    const managed = new Set(fields().map(field => field.key));
    const variables = (current.variables ?? []).filter(variable => !managed.has(variable.name));
    for (const field of fields()) {
      if (field.kind === "domain") continue;
      const variable = values().get(field.key);
      if (!variable) continue;
      const changed = variable.value !== (field.default ?? "");
      if (!variable.value && !field.required && !field.generate) continue;
      if (!changed && !field.required && !field.generate && !field.secret && field.group !== "access") continue;
      const next: Variable = { ...variable, name: field.key, scope: "runtime", secret: !!field.secret || field.kind === "password" };
      if (next.secret && !next.value && variable.hasValue) { next.keep = true; next.keepFrom = variable.keepFrom || field.key; }
      variables.push(next);
    }
    for (const domain of fields().filter(field => field.kind === "domain")) {
      const entry = domainRows().find(item => item.service === domain.service);
      if (!entry) continue;
      const host = hostnames()[domainKey(entry)] ?? "";
      if (!host) continue;
      const secure = !!https()[domainKey(entry)] && !host.endsWith(".localhost");
      variables.push({ name: domain.key, value: variableURL(host, secure), secret: false, scope: "runtime", hasValue: true });
    }
    const nextDomains: Domain[] = [];
    const primaries = new Set<string>();
    domainRows().forEach(domain => {
      const host = (hostnames()[domainKey(domain)] ?? "").trim();
      if (!host) return;
      const secure = !!https()[domainKey(domain)] && !host.endsWith(".localhost");
      const isPrimary = !primaries.has(domain.service);
      if (isPrimary) primaries.add(domain.service);
      nextDomains.push({ id: "catalog-" + domain.service + (nextDomains.length ? "-" + nextDomains.length : ""), hostname: host, service: domain.service, https: secure, primary: isPrimary, redirect: false });
    });
    for (const existing of props.application.domains ?? []) {
      if (!existing.hostname || nextDomains.some(domain => domain.hostname === existing.hostname)) continue;
      nextDomains.push({ ...existing, primary: false, redirect: false });
    }
    if (!nextDomains.length) {
      // Keep the existing route when the form has no domain rows.
      nextDomains.push(...(props.application.domains ?? []));
    }
    const primaryDomain = nextDomains.find(domain => domain.primary) ?? nextDomains[0];
    return {
      ...current,
      variables,
      domains: nextDomains,
      domain: primaryDomain?.hostname ?? current.domain,
      https: primaryDomain?.https ?? current.https,
      catalogService: props.service.id,
      catalogVersion: props.service.updated ?? current.catalogVersion,
    };
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (props.busy || saving()) return;
    setError("");
    for (const field of fields()) {
      if (field.kind === "domain" || !field.required) continue;
      if (!values().get(field.key)?.value && !revealed()[field.key]) { setError("Add " + field.label.toLowerCase()); return; }
    }
    setSaving(true);
    try {
      const applied = await props.save(configForSave());
      if (applied !== false) { setSaved(true); setTimeout(() => setSaved(false), 2200); }
    } catch (reason) { setError((reason as Error).message); }
    finally { setSaving(false); }
  }

  return <div class="catalog-console">
    <header class="catalog-console-head">
      <span class="catalog-mark xl">{props.service.logomark}</span>
      <div class="catalog-console-copy">
        <div class="catalog-console-title"><h2>{props.service.name}</h2><span class={"catalog-support " + props.service.support}>{props.service.support === "ready" ? "Ready" : "Advanced"}</span></div>
        <p>{props.service.description}</p>
        <div class="catalog-console-meta">
          <span><Icon name="box" />{props.service.category}</span>
          <span>{props.service.services.length} {props.service.services.length === 1 ? "service" : "services"}</span>
          <Show when={mainImage()}><span class="mono truncate" title={mainImage()}>{mainImage()}</span></Show>
          <Show when={props.service.documentation}><a href={props.service.documentation} target="_blank" rel="noreferrer">Documentation<Icon name="external" /></a></Show>
        </div>
      </div>
      <div class="catalog-console-actions">
        <Show when={primary()}><a class="button ghost" href={domainURL(primary()!, props.proxy)} target="_blank" rel="noreferrer">Open<Icon name="external" /></a></Show>
        <ActionMenu label="Service actions" icon="more" scope="catalog">
          <Show when={props.lifecycle}><button disabled={props.busy} onClick={() => props.lifecycle!("restart")}>Restart service</button></Show>
          <Show when={props.lifecycle}><button disabled={props.busy} onClick={() => props.lifecycle!("stop")}>Stop service</button></Show>
        </ActionMenu>
      </div>
    </header>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <div class="catalog-console-grid">
      <section class="catalog-card">
        <div class="catalog-card-heading"><h3>Access</h3><span>{access().length} {access().length === 1 ? "credential" : "credentials"}</span></div>
        <Show when={access().length} fallback={<p class="catalog-card-empty">No credentials required</p>}>
          <div class="catalog-access-list">
            <For each={access()}>{field => {
              const variable = () => values().get(field.key);
              const shown = () => revealed()[field.key] ?? (field.secret ? "" : variable()?.value ?? "");
              return <div class="catalog-access-row">
                <div><span>{field.label}</span><code classList={{ masked: !!field.secret && !revealed()[field.key] }}>{field.secret && !revealed()[field.key] ? (variable()?.hasValue ? "••••••••••••" : "Not set") : shown() || "Not set"}</code></div>
                <div class="catalog-access-actions">
                  <Show when={field.secret}><Button small variant="ghost" icon="eye" aria-label={(revealed()[field.key] ? "Hide " : "Reveal ") + field.label} onClick={() => void reveal(field)}>{revealed()[field.key] ? "Hide" : "Reveal"}</Button></Show>
                  <Show when={shown()}><Button small variant="ghost" icon="copy" aria-label={"Copy " + field.label} onClick={() => void copy(shown())} /></Show>
                </div>
              </div>;
            }}</For>
          </div>
        </Show>
      </section>
      <section class="catalog-card">
        <div class="catalog-card-heading"><h3>Endpoints</h3><span>{props.service.services.length}</span></div>
        <div class="catalog-endpoint-list">
          <For each={props.service.services}>{part => <button type="button" class="catalog-endpoint" disabled={!props.selectService} onClick={() => props.selectService?.(part.name)}>
            <span class="catalog-endpoint-icon"><Icon name={part.role === "Database" || part.role === "Cache" ? "database" : part.role === "Storage" ? "folder" : "box"} /></span>
            <span><strong>{part.name}</strong><small>{part.role} · {part.port ? "port " + part.port : "internal"}</small></span>
            <Show when={props.application.containers.find(container => container.service === part.name)}>{container => <Status value={statusOf(container())} />}</Show>
            <Icon name="chevron" />
          </button>}</For>
        </div>
      </section>
    </div>
    <form class="catalog-card catalog-settings" onSubmit={save}>
      <div class="catalog-card-heading"><h3>Configuration</h3><Show when={saved()}><span class="catalog-saved"><Icon name="check" />Saved</span></Show></div>
      <Show when={domainRows().length}>
        <section class="catalog-form-group"><h4>Domain</h4><div class="catalog-domain-list">
          <For each={domainRows()}>{domain => <div class="catalog-domain">
            <label class="catalog-field"><span>{domain.service}</span><input class="input mono" value={hostnames()[domainKey(domain)] ?? ""} placeholder="app.example.com" onInput={event => updateHost(domain, event.currentTarget.value)} /></label>
            <label class="check-field"><input type="checkbox" checked={!!https()[domainKey(domain)]} disabled={(hostnames()[domainKey(domain)] ?? "").endsWith(".localhost")} onChange={event => updateHTTPS(domain, event.currentTarget.checked)} />HTTPS</label>
          </div>}</For>
        </div></section>
      </Show>
      <Show when={config().length}><section class="catalog-form-group"><h4>Options</h4><div class="catalog-field-grid">
        <For each={config()}>{field => <label class="catalog-field"><span>{field.label}</span>
          <Show when={field.kind === "select"} fallback={<input class="input" type={field.kind === "number" ? "number" : field.kind === "email" ? "email" : "text"} value={fieldValue(values(), field)} placeholder={field.placeholder} onInput={event => update(field, event.currentTarget.value)} />}>
            <select class="input" value={fieldValue(values(), field)} onChange={event => update(field, event.currentTarget.value)}><For each={field.options ?? []}>{option => <option value={option}>{optionLabel(option)}</option>}</For></select>
          </Show>
        </label>}</For>
      </div></section></Show>
      <Show when={advanced().length}><details class="catalog-advanced"><summary>Advanced<Icon name="down" /></summary><div class="catalog-field-grid"><For each={advanced()}>{field => <label class="catalog-field"><span>{field.label}</span><input class="input" type={field.kind === "number" ? "number" : "text"} value={fieldValue(values(), field)} placeholder={field.placeholder} onInput={event => update(field, event.currentTarget.value)} /></label>}</For></div></details></Show>
      <Show when={!config().length && !advanced().length && !domainRows().length}><p class="catalog-card-empty">No settings to configure.</p></Show>
      <div class="catalog-settings-actions"><Button variant="primary" type="submit" disabled={props.busy || saving()}>{saving() ? "Preparing review" : "Review changes"}</Button></div>
    </form>
  </div>;
}
