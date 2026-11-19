import { For, Show, createSignal } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import { operationTime } from "../data";
import type { Observation } from "./Diagnostics";
import { Button, ErrorNotice, Status } from "./Controls";
import { SegmentedChoice } from "./Choices";

interface NotificationSettings { enabled: boolean; kind: string; events: string[]; configured?: boolean; webhook?: string; private: boolean; smtp?: string; username?: string; password?: string; from?: string; to?: string }
interface FailedDelivery { sequence: number; kind: string; summary: string; createdAt: number; attempts: number; error: string }
const events = ["deployment", "service", "backup", "storage", "certificate", "daemon", "proxy", "task", "update"];
const eventLabels: Record<string, string> = { daemon: "Docker", certificate: "HTTPS", service: "Services", deployment: "Deployments", backup: "Backups", task: "Tasks", update: "Updates" };

export default function Notifications() {
  const auth = useAuth();
  const [draft, setDraft] = createSignal<NotificationSettings>();
  const [busy, setBusy] = createSignal(false), [error, setError] = createSignal(""), [result, setResult] = createSignal("");
  const [inspectDeliveries, setInspectDeliveries] = createSignal(false);
  const operations = createQuery(() => ({ queryKey: ["system", "operations"], queryFn: () => api<{ items: Observation[]; notifications: { pending: number; failed: number } }>("/system/operations"), refetchInterval: 15000 }));
  const settings = createQuery(() => ({ queryKey: ["notifications"], queryFn: () => api<NotificationSettings>("/notifications") }));
  const deliveries = createQuery(() => ({ queryKey: ["notification-deliveries"], queryFn: () => api<{ items: FailedDelivery[] }>("/notifications/deliveries"), enabled: inspectDeliveries(), refetchInterval: 15000 }));
  const config = () => draft() ?? settings.data;
  const change = (value: Partial<NotificationSettings>) => setDraft({ ...config()!, ...value });
  async function action(work: () => Promise<unknown>) {
    if (busy()) return; setBusy(true); setError(""); setResult("");
    try { await work(); } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  async function save(e: SubmitEvent) {
    e.preventDefault();
    await action(async () => { setDraft(await post<NotificationSettings>("/notifications", config(), auth.session()?.csrfToken)); await settings.refetch(); setResult("Saved"); });
  }
  return <section class="settings-section notifications-settings" id="notifications">
    <h2>Notifications</h2>
    <Show when={error() || settings.error}><ErrorNotice message={error() || settings.error!.message} /></Show>
    <Show when={config()} fallback={<span class="loading-line" aria-label="Loading notifications" />}>{c => <form onSubmit={save} class="notification-editor">
      <label class="checkbox-label"><input type="checkbox" checked={c().enabled} onChange={e => change({ enabled: e.currentTarget.checked })} />Enabled</label>
      <Show when={c().enabled}><div class="form-fields field-pairs notification-fields">
        <SegmentedChoice label="Delivery" value={c().kind} choices={[{ value: "webhook", label: "Webhook" }, { value: "email", label: "Email" }]} change={kind => change({ kind })} />
        <Show when={c().kind === "webhook"} fallback={<>
          <div class="field field-wide"><label for="smtp-host">SMTP host</label><input id="smtp-host" class="input" placeholder="smtp.example.com:587" value={c().smtp || ""} onInput={e => change({ smtp: e.currentTarget.value })} /></div>
          <div class="field"><label for="smtp-user">Username</label><input id="smtp-user" class="input" value={c().username || ""} onInput={e => change({ username: e.currentTarget.value })} /></div>
          <div class="field"><label for="smtp-password">Password</label><input id="smtp-password" class="input" type="password" autocomplete="new-password" value={c().password || ""} onInput={e => change({ password: e.currentTarget.value })} /></div>
          <div class="field"><label for="smtp-from">From</label><input id="smtp-from" class="input" type="email" value={c().from || ""} onInput={e => change({ from: e.currentTarget.value })} /></div>
          <div class="field"><label for="smtp-to">To</label><input id="smtp-to" class="input" type="email" value={c().to || ""} onInput={e => change({ to: e.currentTarget.value })} /></div>
        </>}><div class="field field-wide"><label for="notification-url">Webhook URL</label><input id="notification-url" class="input" type="url" placeholder={c().configured ? "Saved endpoint" : "https://"} value={c().webhook || ""} onInput={e => change({ webhook: e.currentTarget.value })} /></div></Show>
      </div><details class="configuration"><summary>Events</summary><div class="capability-choices event-choices">
        <For each={events}>{kind => <label><input type="checkbox" checked={c().events.includes(kind)} onChange={e => change({ events: e.currentTarget.checked ? [...c().events, kind] : c().events.filter(v => v !== kind) })} />{eventLabels[kind] ?? kind[0].toUpperCase() + kind.slice(1)}</label>}</For>
      </div></details><details class="configuration"><summary>Network</summary><div class="capability-choices"><label><input type="checkbox" checked={c().private} onChange={e => change({ private: e.currentTarget.checked })} />Allow private destinations</label>
      </div></details></Show>
      <div class="form-actions"><Show when={result()}><span role="status">{result()}</span></Show>
        <Button variant="ghost" disabled={busy() || !c().enabled} onClick={() => void action(async () => { await post("/notifications/test", { config: config() }, auth.session()?.csrfToken); setResult("Delivered"); })}>Send test</Button>
        <Button variant="primary" type="submit" disabled={busy()}>Save notifications</Button>
      </div>
    </form>}</Show>
    <Show when={operations.data?.notifications.failed}><details class="configuration health-inspect" onToggle={e => setInspectDeliveries(e.currentTarget.open)}>
      <summary><Status value="failed" label={operations.data!.notifications.failed + (operations.data!.notifications.failed === 1 ? " failed delivery" : " failed deliveries")} /><span>Inspect deliveries</span></summary>
      <div class="health-inspect-body">
        <Show when={deliveries.error}><ErrorNotice message={deliveries.error!.message} retry={() => void deliveries.refetch()} /></Show>
        <Show when={deliveries.isPending}><span class="loading-line" aria-label="Loading deliveries" /></Show>
        <For each={deliveries.data?.items}>{delivery => <section class="health-detail">
          <div class="section-toolbar"><strong>{delivery.summary}</strong><time class="operator-time">{operationTime(delivery.createdAt)}</time></div>
          <p>{delivery.error || "Delivery failed"}</p>
          <div class="toolbar-actions"><Status value="failed" label={delivery.attempts + " attempts"} /><Button small disabled={busy() || !settings.data?.enabled} onClick={() => void action(async () => {
            await post(`/notifications/deliveries/${delivery.sequence}/retry`, {}, auth.session()?.csrfToken);
            await Promise.all([deliveries.refetch(), operations.refetch()]); setResult("Delivery queued");
          })}>Retry delivery</Button></div>
        </section>}</For>
      </div>
    </details></Show>
  </section>;
}
