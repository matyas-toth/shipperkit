import { For, Index, Show, createUniqueId } from "solid-js";
import type { Application, NetworkAccess, PublishedPort, Service } from "../types";
import { Button, IconButton } from "./Controls";
import "./Networking.css";

export default function Networking(props: { value: NetworkAccess; applications: Application[]; applicationId?: string; services?: Service[]; change: (value: NetworkAccess) => void }) {
  const id = createUniqueId();
  const applications = () => props.value?.applications ?? [];
  const ports = () => props.value?.ports ?? [];
  const available = () => props.applications.filter(app => app.id !== props.applicationId && app.sourceKind !== "external" && app.state !== "deleted");
  const update = (patch: Partial<NetworkAccess>) => props.change({ applications: applications(), ports: ports(), ...patch });
  const updatePort = (index: number, patch: Partial<PublishedPort>) => update({ ports: ports().map((port, i) => i === index ? { ...port, ...patch } : port) });
  return <div class="networking-settings">
    <section>
      <div class="section-toolbar"><h2>Private connections</h2><Button icon="plus" disabled={applications().length >= available().length} onClick={() => {
        const next = available().find(app => !applications().includes(app.id));
        if (next) update({ applications: [...applications(), next.id] });
      }}>Connect application</Button></div>
      <Index each={applications()}>{(application, index) => <div class="networking-connection">
        <select class="input" aria-label={"Connected application " + (index + 1)} value={application()} onChange={e => update({ applications: applications().map((value, i) => i === index ? e.currentTarget.value : value) })}>
          <For each={available().filter(app => app.id === application() || !applications().includes(app.id))}>{app => <option value={app.id}>{app.name}</option>}</For>
        </select>
        <code class="networking-host" title={"app-" + application()}>{"app-" + application()}</code>
        <IconButton icon="trash" ghost label={"Disconnect application " + (index + 1)} onClick={() => update({ applications: applications().filter((_, i) => i !== index) })} />
      </div>}</Index>
    </section>
    <section>
      <div class="section-toolbar"><h2>Public ports</h2><Button icon="plus" onClick={() => update({ ports: [...ports(), { service: props.services?.[0]?.name, protocol: "tcp", bindIP: "0.0.0.0", hostPort: 0, containerPort: 0 }] })}>Add port</Button></div>
      <Index each={ports()}>{(port, index) => <div class="networking-port">
        <Show when={props.services?.length}><div class="field"><label for={id + "-service-" + index}>Service</label><select id={id + "-service-" + index} class="input" value={port().service ?? ""} required onChange={e => updatePort(index, { service: e.currentTarget.value })}><option value="" disabled>Select service</option><For each={props.services}>{service => <option value={service.name}>{service.name}</option>}</For></select></div></Show>
        <div class="field"><label for={id + "-protocol-" + index}>Protocol</label><select id={id + "-protocol-" + index} class="input" value={port().protocol} onChange={e => updatePort(index, { protocol: e.currentTarget.value as "tcp" | "udp" })}><option value="tcp">TCP</option><option value="udp">UDP</option></select></div>
        <div class="field"><label for={id + "-bind-" + index}>Bind IP</label><input id={id + "-bind-" + index} class="input mono" value={port().bindIP} required onInput={e => updatePort(index, { bindIP: e.currentTarget.value })} /></div>
        <div class="field"><label for={id + "-public-" + index}>Public port</label><input id={id + "-public-" + index} class="input mono" type="number" min="1024" max="65535" required value={port().hostPort || ""} onInput={e => updatePort(index, { hostPort: Number(e.currentTarget.value) })} /></div>
        <div class="field"><label for={id + "-container-" + index}>Container port</label><input id={id + "-container-" + index} class="input mono" type="number" min="1" max="65535" required value={port().containerPort || ""} onInput={e => updatePort(index, { containerPort: Number(e.currentTarget.value) })} /></div>
        <IconButton icon="trash" ghost label={"Remove port " + (index + 1)} onClick={() => update({ ports: ports().filter((_, i) => i !== index) })} />
      </div>}</Index>
    </section>
  </div>;
}
