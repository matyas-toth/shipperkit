import { Show, createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import type { Application, NetworkAccess, NetworkingInfo, ProxyConfig, PublishedPort, ReviewSummaryItem, Service, StaticConfig } from "../types";
import { Button, ErrorNotice } from "./Controls";
import { Icon } from "./Icon";
import Networking, { domainPorts } from "./Networking";

// Copies keep edits independent of the refetched application data.
const normalize = (value?: NetworkAccess | null): NetworkAccess => ({ ports: (value?.ports ?? []).map(port => ({ ...port })) });
const portLabel = (port: PublishedPort) => port.hostPort + "/" + port.protocol + " to container port " + port.containerPort + " on " + (port.bindIP === "0.0.0.0" ? "all interfaces" : port.bindIP === "127.0.0.1" ? "loopback" : port.bindIP);
const samePort = (a: PublishedPort, b: PublishedPort) => a.protocol === b.protocol && a.hostPort === b.hostPort && a.containerPort === b.containerPort && a.bindIP === b.bindIP;
const domainPattern = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/;
const cleanDomain = (value: string) => value.trim().toLowerCase().replace(/\.$/, "");

export default function ApplicationNetworking(props: {
  application: Application;
  services: Service[];
  busy: boolean;
  save: (config: StaticConfig, summary?: ReviewSummaryItem[], note?: string) => Promise<boolean | void>;
  onDirty?: (value: boolean) => void;
}) {
  const info = createQuery(() => ({ queryKey: ["applications", props.application.id, "networking"], queryFn: () => api<NetworkingInfo>(`/applications/${props.application.id}/networking`), reconcile: "id" }));
  const proxy = createQuery(() => ({ queryKey: ["proxy"], queryFn: () => api<ProxyConfig>("/proxy") }));
  const [value, setValue] = createSignal<NetworkAccess>(normalize(props.application.config.networking));
  const savedDomain = () => props.application.config.database?.domain ?? "";
  const [domain, setDomain] = createSignal(savedDomain());
  const [error, setError] = createSignal("");
  const [saving, setSaving] = createSignal(false);
  const [conflict, setConflict] = createSignal<number>();
  const engine = () => props.application.config.database?.engine ?? "";
  const domainPort = () => domainPorts[engine()];
  const original = () => normalize(props.application.config.networking);
  const domainError = () => {
    const next = cleanDomain(domain());
    if (next && !domainPattern.test(next)) return "Enter a domain such as db.example.com.";
    if (next && value().ports.some(port => port.protocol === "tcp" && port.hostPort === domainPort())) return `Port ${domainPort()} serves this domain. Remove it from published ports.`;
    return "";
  };
  const portProblem = () => {
    const list = value().ports;
    for (const port of list) {
      if (!port.hostPort || port.hostPort < 1024 || port.hostPort > 65535) return "Public ports must be between 1024 and 65535.";
      if (!port.containerPort || port.containerPort < 1 || port.containerPort > 65535) return "Container ports must be between 1 and 65535.";
    }
    for (let i = 0; i < list.length; i++) for (let j = i + 1; j < list.length; j++) {
      if (list[i].protocol === list[j].protocol && list[i].hostPort === list[j].hostPort) return "Each public port can be mapped once per protocol.";
    }
    return "";
  };
  const issues = () => (conflict() !== undefined ? 1 : 0) + (domainError() ? 1 : 0) + (portProblem() ? 1 : 0);
  const changed = () => JSON.stringify(value()) !== JSON.stringify(original()) || cleanDomain(domain()) !== savedDomain();
  createEffect(() => props.onDirty?.(changed()));
  onCleanup(() => props.onDirty?.(false));
  function update(next: NetworkAccess) {
    setValue(next);
    setError("");
    setConflict(undefined);
  }
  function changeDomain(next: string) {
    setDomain(next);
    setError("");
  }
  function reset() {
    setValue(normalize(props.application.config.networking));
    setDomain(savedDomain());
    setError("");
    setConflict(undefined);
  }
  const summary = createMemo<ReviewSummaryItem[]>(() => {
    const before = original();
    const after = value();
    const items: ReviewSummaryItem[] = [];
    const previous = savedDomain();
    const next = cleanDomain(domain());
    if (next !== previous) {
      const target = (host: string) => host + ":" + domainPort();
      items.push({ label: "Public domain", value: !previous ? "Serve on " + target(next) : !next ? "Stop serving " + target(previous) : target(previous) + " to " + target(next) });
    }
    const addedPorts = after.ports.filter(port => !before.ports.some(existing => samePort(existing, port)));
    const removedPorts = before.ports.filter(port => !after.ports.some(candidate => samePort(port, candidate)));
    if (addedPorts.length || removedPorts.length) {
      items.push({ label: "Public ports", value: [addedPorts.length ? "Map " + addedPorts.map(portLabel).join(", ") : "", removedPorts.length ? "Remove " + removedPorts.map(portLabel).join(", ") : ""].filter(Boolean).join(". ") });
    }
    const opened = addedPorts.map(port => port.hostPort + "/" + port.protocol);
    if (next && !previous) opened.unshift(domainPort() + "/tcp");
    if (opened.length) items.push({ label: "Firewall", value: "Required: open " + opened.join(", ") + " on the host firewall" });
    return items;
  });
  async function review(event: SubmitEvent) {
    event.preventDefault();
    if (!changed() || saving() || props.busy || issues() > 0) return;
    setError(""); setConflict(undefined); setSaving(true);
    const exposed = summary().some(item => item.label === "Public ports" || item.label === "Public domain" && cleanDomain(domain()));
    const note = exposed
      ? "The database becomes reachable from the internet. Connections drop while it restarts."
      : "Data is preserved; connections drop while the database restarts.";
    const config = props.application.config;
    try {
      await props.save({ ...config, networking: value(), database: config.database ? { ...config.database, domain: cleanDomain(domain()) || undefined } : undefined }, summary(), config.database ? note : undefined);
    } catch (e) {
      const message = (e as Error).message;
      setError(message);
      const match = /Port (\d+)\/(?:tcp|udp)/i.exec(message);
      setConflict(match ? Number(match[1]) : undefined);
    }
    finally { setSaving(false); }
  }
  return <form class="application-networking" onSubmit={review}>
    <Show when={info.error}><ErrorNotice message={info.error!.message} retry={() => void info.refetch()} /></Show>
    <Show when={info.data} fallback={<Show when={!info.error}><span class="loading-line" aria-label="Loading networking" /></Show>}>
      <fieldset disabled={props.busy || saving()} class="creation-fields">
        <Networking value={value()} change={update} host={info.data!.host} port={props.application.config.port} services={props.services} conflictPort={conflict()}
          engine={domainPort() ? engine() : undefined} domain={domain()} changeDomain={changeDomain} domainError={domainError()} publicAddress={proxy.data?.publicAddress} />
      </fieldset>
    </Show>
    <Show when={error() && conflict() === undefined}><ErrorNotice message={error()} /></Show>
    <Show when={info.data && changed() && !saving()}><div class="form-actions network-review-bar">
      <span class="network-unsaved-page"><span class="network-unsaved-dot" />You have unsaved changes</span>
      <Button variant="ghost" type="button" disabled={saving()} onClick={reset}>Revert</Button>
      <Show when={portProblem()}><span class="network-review-block" role="alert"><Icon name="warning" />{portProblem()}</span></Show>
      <Show when={conflict() !== undefined || domainError()}><button type="button" class="network-issue-count" onClick={() => document.querySelector<HTMLInputElement>(".network-field.is-conflict input, .networking-settings .input.is-invalid")?.focus()}>{issues()} {issues() === 1 ? "issue" : "issues"}</button></Show>
      <Button type="submit" variant="primary" disabled={!changed() || props.busy || saving() || issues() > 0}>{saving() ? "Saving" : "Save and redeploy"}</Button>
    </div></Show>
  </form>;
}
