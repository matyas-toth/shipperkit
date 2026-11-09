import { Show, createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import type { Application, NetworkAccess, NetworkingInfo, PublishedPort, ReviewSummaryItem, Service, StaticConfig } from "../types";
import { Button, ErrorNotice } from "./Controls";
import { Icon } from "./Icon";
import Networking from "./Networking";

const empty = (): NetworkAccess => ({ applications: [], ports: [] });
const normalize = (value?: NetworkAccess | null): NetworkAccess => ({ applications: value?.applications ?? [], ports: value?.ports ?? [] });
const portLabel = (port: PublishedPort) => port.hostPort + "/" + port.protocol + " to container port " + port.containerPort + " on " + (port.bindIP === "0.0.0.0" ? "all interfaces" : port.bindIP === "127.0.0.1" ? "loopback" : port.bindIP);
const samePort = (a: PublishedPort, b: PublishedPort) => a.protocol === b.protocol && a.hostPort === b.hostPort && a.containerPort === b.containerPort && a.bindIP === b.bindIP;

export default function ApplicationNetworking(props: {
  application: Application;
  services: Service[];
  busy: boolean;
  save: (config: StaticConfig, summary?: ReviewSummaryItem[], note?: string) => Promise<boolean | void>;
  onDirty?: (value: boolean) => void;
}) {
  const info = createQuery(() => ({ queryKey: ["applications", props.application.id, "networking"], queryFn: () => api<NetworkingInfo>(`/applications/${props.application.id}/networking`) }));
  const [value, setValue] = createSignal<NetworkAccess>(normalize(props.application.config.networking));
  const [tlsHostname, setTlsHostname] = createSignal(props.application.config.database?.tlsHostname ?? "");
  const [tlsOn, setTlsOn] = createSignal(!!props.application.config.database?.tlsHostname);
  const [error, setError] = createSignal("");
  const [saving, setSaving] = createSignal(false);
  const [conflict, setConflict] = createSignal<number>();
  const original = () => normalize(props.application.config.networking);
  const savedTLS = () => props.application.config.database?.tlsHostname ?? "";
  const tlsInvalid = () => tlsOn() && !tlsHostname().trim();
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
  const issues = () => (conflict() !== undefined ? 1 : 0) + (tlsInvalid() ? 1 : 0) + (portProblem() ? 1 : 0);
  const changed = () => JSON.stringify(value()) !== JSON.stringify(original()) || tlsOn() !== !!savedTLS() || tlsHostname() !== savedTLS();
  createEffect(() => props.onDirty?.(changed()));
  onCleanup(() => props.onDirty?.(false));
  function update(next: NetworkAccess) {
    setValue(next);
    setError("");
    setConflict(undefined);
  }
  function toggleTLS(on: boolean) {
    setTlsOn(on);
    if (!on) setTlsHostname("");
    setError("");
  }
  function reset() {
    setValue(normalize(props.application.config.networking));
    setTlsHostname(savedTLS());
    setTlsOn(!!savedTLS());
    setError("");
    setConflict(undefined);
  }
  const summary = createMemo<ReviewSummaryItem[]>(() => {
    const before = original();
    const after = value();
    const items: ReviewSummaryItem[] = [];
    const name = (id: string) => info.data?.applications.find(entry => entry.id === id)?.name ?? id;
    const added = after.applications.filter(id => !before.applications.includes(id));
    const removed = before.applications.filter(id => !after.applications.includes(id));
    if (added.length || removed.length) {
      items.push({ label: "Private connections", value: [added.length ? "Add " + added.map(name).join(", ") : "", removed.length ? "Remove " + removed.map(name).join(", ") : ""].filter(Boolean).join(". ") });
    }
    const addedPorts = after.ports.filter(port => !before.ports.some(previous => samePort(previous, port)));
    const removedPorts = before.ports.filter(port => !after.ports.some(next => samePort(port, next)));
    if (addedPorts.length || removedPorts.length) {
      items.push({ label: "Public ports", value: [addedPorts.length ? "Map " + addedPorts.map(portLabel).join(", ") : "", removedPorts.length ? "Remove " + removedPorts.map(portLabel).join(", ") : ""].filter(Boolean).join(". ") });
    }
    if (addedPorts.length) {
      items.push({ label: "Firewall", value: "Required: open " + addedPorts.map(port => port.hostPort + "/" + port.protocol).join(", ") + " on the host firewall" });
    }
    const previousTLS = savedTLS();
    const nextTLS = tlsOn() ? tlsHostname().trim() : "";
    if (nextTLS !== previousTLS) {
      const value = !previousTLS && nextTLS ? "Enable TLS for " + nextTLS : previousTLS && !nextTLS ? "Disable TLS for " + previousTLS : previousTLS + " to " + nextTLS;
      items.push({ label: "TLS hostname", value });
    }
    return items;
  });
  async function review(event: SubmitEvent) {
    event.preventDefault();
    if (!changed() || saving() || props.busy || tlsInvalid() || portProblem()) return;
    setError(""); setConflict(undefined); setSaving(true);
    const note = summary().some(item => item.label === "Public ports")
      ? "The database becomes reachable from the internet. Connections drop while the ports switch."
      : "Data is preserved; connections drop while the network changes.";
    try {
      await props.save({ ...props.application.config, networking: value(), database: props.application.config.database ? { ...props.application.config.database, tlsHostname: tlsOn() ? tlsHostname().trim() : undefined } : undefined }, summary(), note);
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
        <Networking value={value()} change={update} applicationId={props.application.id} services={props.services}
          applications={info.data!.applications} host={info.data!.host} port={props.application.config.port} database={!!props.application.config.database} tlsCapable={["postgresql", "mysql", "mariadb", "redis"].includes(props.application.config.database?.engine ?? "")}
          tlsHostname={tlsHostname()} changeTLS={setTlsHostname} conflictPort={conflict()} tlsOn={tlsOn()} changeTLSOn={toggleTLS} tlsInvalid={tlsInvalid()} />
      </fieldset>
    </Show>
    <Show when={error() && conflict() === undefined}><ErrorNotice message={error()} /></Show>
    <Show when={info.data && changed() && !saving()}><div class="form-actions network-review-bar">
      <Show when={changed()}><span class="network-unsaved-page"><span class="network-unsaved-dot" />Unsaved changes</span></Show>
      <Show when={changed()}><Button variant="neutral" type="button" disabled={saving()} onClick={reset}>Discard changes</Button></Show>
      <Show when={portProblem()}><span class="network-review-block" role="alert"><Icon name="warning" />{portProblem()}</span></Show>
      <Show when={conflict() !== undefined || tlsInvalid()}><button type="button" class="network-issue-count" onClick={() => document.querySelector<HTMLInputElement>(".network-field.is-conflict input, .network-tls .is-invalid")?.focus()}>{issues()} {issues() === 1 ? "issue" : "issues"}</button></Show>
      <Button type="submit" variant="primary" disabled={!changed() || props.busy || saving() || issues() > 0}>{saving() ? "Preparing review" : "Review changes"}</Button>
    </div></Show>
  </form>;
}
