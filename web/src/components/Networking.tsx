import { For, Index, Show, createSignal, createUniqueId, type JSX } from "solid-js";
import type { NetworkAccess, PublishedPort, Service } from "../types";
import { Button, IconButton } from "./Controls";
import DnsTarget from "./DnsTarget";
import { Icon } from "./Icon";
import "./Networking.css";

// Engines the proxy serves on a public domain, at their standard port.
export const domainPorts: Record<string, number> = { postgresql: 5432, redis: 6379, mongodb: 27017 };

export default function Networking(props: {
  value: NetworkAccess;
  change: (value: NetworkAccess) => void;
  host: string;
  port?: number;
  services?: Service[];
  conflictPort?: number;
  // A managed database the proxy can serve on a public domain.
  engine?: string;
  domain?: string;
  changeDomain?: (value: string) => void;
  domainError?: string;
  publicAddress?: string;
}) {
  const id = createUniqueId();
  const ports = () => props.value?.ports ?? [];
  const update = (patch: Partial<NetworkAccess>) => props.change({ ports: ports(), ...patch });
  const updatePort = (index: number, patch: Partial<PublishedPort>) => update({ ports: ports().map((port, i) => i === index ? { ...port, ...patch } : port) });
  const newPort = (): PublishedPort => ({ service: props.services?.[0]?.name, protocol: "tcp", bindIP: "0.0.0.0", hostPort: 0, containerPort: 0 });
  const [copied, setCopied] = createSignal("");
  async function copy(value: string) { try { await navigator.clipboard.writeText(value); setCopied(value); setTimeout(() => setCopied(""), 1500); } catch { /* clipboard unavailable */ } }
  const address = () => props.host + (props.port ? ":" + props.port : "");
  const domainPort = () => domainPorts[props.engine ?? ""];
  const endpoint = () => (props.domain ?? "").trim() ? (props.domain ?? "").trim().toLowerCase() + ":" + domainPort() : "";
  // A database with a domain keeps raw ports for the rare client that needs one.
  const [advanced, setAdvanced] = createSignal(ports().length > 0);

  const publicPorts = (): JSX.Element => <section class="network-section" aria-labelledby={id + "-ports"}>
    <div class="section-toolbar network-toolbar">
      <div class="network-heading"><h2 id={id + "-ports"}>{domainPort() ? "Published ports" : "Public ports"}</h2></div>
      <Show when={ports().length}><Button small icon="plus" onClick={() => update({ ports: [...ports(), newPort()] })}>Add port</Button></Show>
    </div>
    <Show when={ports().length} fallback={<div class="network-empty">
      <span class="network-empty-icon"><Icon name="globe" /></span>
      <div class="network-empty-copy"><strong>No public ports</strong></div>
      <Button small icon="plus" variant={domainPort() ? "neutral" : "primary"} onClick={() => update({ ports: [newPort()] })}>Add port</Button>
    </div>}>
      <div class="network-port-list" classList={{ "has-service": !!props.services?.length }}>
        <div class="network-port-head">
          <Show when={props.services?.length}><span>Service</span></Show>
          <span>Protocol</span><span>Public port</span><span>Container port</span><span>Bind</span><span />
        </div>
        <Index each={ports()}>{(port, index) => <div class="network-port-row">
          <Show when={props.services?.length}><label class="network-field network-field-service">
            <span>Service</span>
            <select class="input" aria-label={"Service for port " + (index + 1)} value={port().service ?? ""} required onChange={e => updatePort(index, { service: e.currentTarget.value })}>
              <option value="" disabled>Select service</option>
              <For each={props.services?.map(service => service.name)}>{name => <option value={name}>{name}</option>}</For>
            </select>
          </label></Show>
          <label class="network-field">
            <span>Protocol</span>
            <select class="input" aria-label="Protocol" value={port().protocol} onChange={e => updatePort(index, { protocol: e.currentTarget.value as "tcp" | "udp" })}>
              <option value="tcp">TCP</option><option value="udp">UDP</option>
            </select>
          </label>
          <label class="network-field" classList={{ "is-conflict": props.conflictPort === port().hostPort }}>
            <span>Public port</span>
            <input class="input mono" aria-label="Public port" aria-invalid={props.conflictPort === port().hostPort} type="number" min="1024" max="65535" required value={port().hostPort || ""} onInput={e => updatePort(index, { hostPort: Number(e.currentTarget.value) })} />
            <Show when={props.conflictPort === port().hostPort}><span class="network-field-error" role="alert"><Icon name="warning" />Public port {port().hostPort} is already in use. Choose another port.</span></Show>
          </label>
          <label class="network-field">
            <span>Container port</span>
            <input class="input mono" aria-label="Container port" type="number" min="1" max="65535" required value={port().containerPort || ""} onInput={e => updatePort(index, { containerPort: Number(e.currentTarget.value) })} />
          </label>
          <label class="network-field">
            <span>Bind</span>
            <select class="input" aria-label="Bind" value={port().bindIP} onChange={e => updatePort(index, { bindIP: e.currentTarget.value })}>
              <option value="0.0.0.0">All interfaces (0.0.0.0)</option>
              <option value="127.0.0.1">Loopback (127.0.0.1)</option>
              <Show when={!["0.0.0.0", "127.0.0.1"].includes(port().bindIP)}><option value={port().bindIP}>{port().bindIP}</option></Show>
            </select>
          </label>
          <div class="network-row-actions"><button type="button" class="network-remove" aria-label={"Remove port " + (index + 1)} onClick={() => update({ ports: ports().filter((_, i) => i !== index) })}>Remove</button></div>
        </div>}</Index>
      </div>
      <Show when={!props.conflictPort}><div class="network-hint"><Icon name="globe" /><span>Open {ports().map(port => port.hostPort + "/" + port.protocol).join(", ")} in your firewall before clients connect.</span></div></Show>
    </Show>
  </section>;

  return <div class="networking-settings">
    <section class="network-section network-private" aria-labelledby={id + "-private"}>
      <div class="network-heading">
        <h2 id={id + "-private"}>Private address</h2>
        <button type="button" class="network-host network-host-button mono" aria-label={"Copy private address " + address()} onClick={() => void copy(address())}>
          {address()}<Icon name={copied() === address() ? "check" : "copy"} />
        </button>
      </div>
    </section>

    <Show when={domainPort()} fallback={publicPorts()}>
      <section class="network-section" aria-labelledby={id + "-domain"}>
        <div class="section-toolbar network-toolbar">
          <div class="network-heading"><h2 id={id + "-domain"}><Icon name="lock" />Public domain</h2></div>
        </div>
        <div class="network-tls-body">
          <div class="form-fields">
            <label class="sr-only" for={id + "-domain-input"}>Domain</label>
            <input id={id + "-domain-input"} class="input mono" classList={{ "is-invalid": !!props.domainError }} aria-invalid={!!props.domainError} placeholder="db.example.com" autocomplete="off" spellcheck={false} value={props.domain ?? ""} onInput={e => props.changeDomain?.(e.currentTarget.value)} />
            <Show when={props.domainError}><span class="network-field-error" role="alert"><Icon name="warning" />{props.domainError}</span></Show>
          </div>
          <Show when={endpoint() && !props.domainError}>
            <div class="network-tls-endpoint">
              <div class="network-endpoint-value"><code>{endpoint()}</code><IconButton icon={copied() === endpoint() ? "check" : "copy"} ghost label="Copy endpoint" onClick={() => void copy(endpoint())} /></div>
            </div>
          </Show>
        </div>
        <Show when={endpoint() && !props.domainError}><DnsTarget address={props.publicAddress} /></Show>
      </section>
      <details class="network-advanced" open={advanced()} onToggle={e => setAdvanced(e.currentTarget.open)}>
        <summary><Icon name="chevron" />Advanced</summary>
        {publicPorts()}
      </details>
    </Show>
  </div>;
}
