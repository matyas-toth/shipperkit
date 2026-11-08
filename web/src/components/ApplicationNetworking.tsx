import { createSignal, Show } from "solid-js";
import type { Application, NetworkAccess, Service, StaticConfig } from "../types";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import type { Inventory } from "../types";
import { Button, ErrorNotice } from "./Controls";
import Networking from "./Networking";

const empty = (): NetworkAccess => ({ applications: [], ports: [] });
export default function ApplicationNetworking(props: { application: Application; services: Service[]; busy: boolean; save: (config: StaticConfig) => Promise<boolean | void> }) {
  const inventory = createQuery(() => ({ queryKey: ["networking-applications"], queryFn: () => api<Inventory>("/applications") }));
  const [value, setValue] = createSignal<NetworkAccess>(JSON.parse(JSON.stringify(props.application.config.networking ?? empty())));
  const [tlsHostname, setTlsHostname] = createSignal(props.application.config.database?.tlsHostname ?? "");
  const [error, setError] = createSignal("");
  const [saving, setSaving] = createSignal(false);
  const changed = () => JSON.stringify(value()) !== JSON.stringify(props.application.config.networking ?? empty()) || tlsHostname() !== (props.application.config.database?.tlsHostname ?? "");
  async function review(event: SubmitEvent) {
    event.preventDefault();
    if (!changed() || saving() || props.busy) return;
    setError(""); setSaving(true);
    try { await props.save({ ...props.application.config, networking: value(), database: props.application.config.database ? { ...props.application.config.database, tlsHostname: tlsHostname().trim() || undefined } : undefined }); }
    catch (e) { setError((e as Error).message); }
    finally { setSaving(false); }
  }
  return <form class="application-networking" onSubmit={review}>
    <h2>Networking</h2>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <fieldset disabled={props.busy || saving()} class="creation-fields">
      <Show when={props.application.config.database}><div class="form-fields"><label for="database-tls-hostname">TLS hostname</label><input id="database-tls-hostname" class="input" placeholder="db.example.com" value={tlsHostname()} onInput={e => setTlsHostname(e.currentTarget.value)} /></div></Show>
      <Networking value={value()} change={setValue} applicationId={props.application.id} services={props.services} applications={inventory.data?.items ?? []} />
    </fieldset>
    <div class="form-actions"><Button variant="primary" type="submit" disabled={!changed() || props.busy || saving()}>{saving() ? "Preparing review" : "Review changes"}</Button></div>
  </form>;
}
