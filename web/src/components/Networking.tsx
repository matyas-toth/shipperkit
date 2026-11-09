import { For, Index, Show, createSignal, createUniqueId } from "solid-js";
import type { NetworkAccess, NetworkApplication, PublishedPort, Service } from "../types";
import { Button, IconButton } from "./Controls";
import { Icon } from "./Icon";
import "./Networking.css";

export default function Networking(props: {
  value: NetworkAccess;
  applications: NetworkApplication[];
  host: string;
  port?: number;
  applicationId?: string;
  services?: Service[];
  database?: boolean;
  tlsHostname: string;
  tlsOn?: boolean;
  tlsInvalid?: boolean;
  changeTLSOn?: (value: boolean) => void;
  change: (value: NetworkAccess) => void;
  changeTLS: (value: string) => void;
  conflictPort?: number;
}) {
  const id = createUniqueId();
  const connections = () => props.value?.applications ?? [];
  const ports = () => props.value?.ports ?? [];
  const applications = () => props.applications ?? [];
  const available = () => applications().filter(app => !connections().includes(app.id));
  const tcpPorts = () => ports().filter(port => port.protocol === "tcp");
  const update = (patch: Partial<NetworkAccess>) => props.change({ applications: connections(), ports: ports(), ...patch });
  const updatePort = (index: number, patch: Partial<PublishedPort>) => update({ ports: ports().map((port, i) => i === index ? { ...port, ...patch } : port) });
  const app = (applicationId: string) => props.applications.find(entry => entry.id === applicationId);
  const [copiedHost, setCopiedHost] = createSignal("");
  async function copyHost(value: string) { try { await navigator.clipboard.writeText(value); setCopiedHost(value); setTimeout(() => setCopiedHost(""), 1500); } catch {} }
  const endpoint = () => tcpPorts().map(port => (props.tlsHostname.trim() || location.hostname) + ":" + port.hostPort).join(", ");
  const [copiedEndpoint, setCopiedEndpoint] = createSignal(false);
  async function copyEndpoint() { if (!endpoint()) return; try { await navigator.clipboard.writeText(endpoint()); setCopiedEndpoint(true); setTimeout(() => setCopiedEndpoint(false), 1500); } catch {} }
  const target = () => props.host + (props.port ? ":" + props.port : "");
  const canConnect = () => available().length > 0;
  function connect() {
    const next = available()[0];
    if (next) update({ applications: [...connections(), next.id] });
  }
  return <div class="networking-settings">
    <section class="network-section" aria-labelledby="private-connections-title">
      <div class="section-toolbar network-toolbar">
        <div class="network-heading">
          <h2 id="private-connections-title">Private connections</h2>
        </div>
        <Show when={connections().length}><Button small icon="plus" disabled={!canConnect()} onClick={connect}>Connect application</Button></Show>
      </div>
      <Show when={connections().length} fallback={<div class="network-empty">
        <span class="network-empty-icon"><Icon name="plug" /></span>
        <div class="network-empty-copy"><strong>No private connections</strong></div>
        <Button small icon="plus" variant="primary" disabled={!canConnect()} onClick={connect}>Connect application</Button>
      </div>}>
        <div class="network-connection-head"><span>Application</span><span>Can reach</span><span /></div>
        <Index each={connections()}>{(applicationId, index) => <div class="network-connection-row">
          <label class="network-field">
            <span>Application</span>
            <select class="input" aria-label={"Connected application " + (index + 1)} value={applicationId()} onChange={e => update({ applications: connections().map((value, i) => i === index ? e.currentTarget.value : value) })}>
              <Show when={!app(applicationId())}><option value={applicationId()}>{applicationId()}</option></Show>
              <For each={applications().filter(entry => entry.id === applicationId() || !connections().includes(entry.id))}>{entry => <option value={entry.id}>{entry.name}</option>}</For>
            </select>
          </label>
          <div class="network-host-cell"><code class="network-host">{target()}</code><IconButton icon={copiedHost() === target() ? "check" : "copy"} ghost label="Copy internal host" onClick={() => void copyHost(target())} /></div>
          <div class="network-row-actions"><button type="button" class="network-remove" aria-label={"Disconnect application " + (index + 1)} onClick={() => update({ applications: connections().filter((_, i) => i !== index) })}>Remove</button></div>
        </div>}</Index>
      </Show>
    </section>

    <section class="network-section" aria-labelledby="public-ports-title">
      <div class="section-toolbar network-toolbar">
        <div class="network-heading"><h2 id="public-ports-title">Public ports</h2></div>
        <Show when={ports().length}><Button small icon="plus" onClick={() => update({ ports: [...ports(), { service: props.services?.[0]?.name, protocol: "tcp", bindIP: "0.0.0.0", hostPort: 0, containerPort: 0 }] })}>Add port</Button></Show>
      </div>
      <Show when={ports().length} fallback={<div class="network-empty">
        <span class="network-empty-icon"><Icon name="globe" /></span>
        <div class="network-empty-copy"><strong>No public ports</strong></div>
        <Button small icon="plus" variant="primary" onClick={() => update({ ports: [{ service: props.services?.[0]?.name, protocol: "tcp", bindIP: "0.0.0.0", hostPort: 0, containerPort: 0 }] })}>Add port</Button>
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
                <For each={props.services}>{service => <option value={service.name}>{service.name}</option>}</For>
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
    </section>

    <Show when={props.database && tcpPorts().length}>
      <section class="network-section" aria-labelledby="public-tls-title">
        <div class="section-toolbar network-toolbar">
          <div class="network-heading">
            <h2 id="public-tls-title"><Icon name="lock" />Public TLS</h2>
            <button type="button" class="network-switch" role="switch" aria-label="Public TLS" aria-checked={!!props.tlsOn} onClick={() => props.changeTLSOn?.(!props.tlsOn)}>
              <span class="network-switch-track"><span /></span>{props.tlsOn ? "On" : "Off"}
            </button>
          </div>
        </div>
        <Show when={props.tlsOn}><div class="network-tls-body">
          <div class="form-fields"><label for={id + "-tls"}>TLS hostname</label>
            <input id={id + "-tls"} class="input" classList={{ "is-invalid": props.tlsInvalid }} aria-invalid={!!props.tlsInvalid} placeholder="db.example.com" value={props.tlsHostname} onInput={e => props.changeTLS(e.currentTarget.value)} />
            <Show when={props.tlsInvalid}><span class="network-field-error" role="alert"><Icon name="warning" />Enter a TLS hostname.</span></Show>
          </div>
          <div class="network-tls-endpoint">
            <span>Endpoint</span>
            <div class="network-endpoint-value"><code>{endpoint()}</code><Show when={!props.conflictPort}><IconButton icon={copiedEndpoint() ? "check" : "copy"} ghost label="Copy endpoint" onClick={() => void copyEndpoint()} /></Show></div>
          </div>
        </div></Show>
      </section>
    </Show>
  </div>;
}
