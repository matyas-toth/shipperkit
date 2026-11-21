import { Show, createSignal } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import { operationTime } from "../data";
import { ErrorNotice } from "./Controls";

interface TelemetryState { allowed: boolean; enabled: boolean; lastSent: number; report?: Record<string, unknown> }

export default function Telemetry() {
  const auth = useAuth();
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  const [optimistic, setOptimistic] = createSignal<boolean>();
  const state = createQuery(() => ({ queryKey: ["telemetry"], queryFn: () => api<TelemetryState>("/telemetry") }));
  const enabled = () => optimistic() ?? state.data?.enabled ?? false;

  async function toggle(next: boolean) {
    setBusy(true); setError(""); setOptimistic(next);
    try { await post<TelemetryState>("/telemetry", { enabled: next }, auth.session()?.csrfToken); await state.refetch(); }
    catch (e) { setError((e as Error).message); }
    finally { setOptimistic(undefined); setBusy(false); }
  }

  return <section class="settings-section" id="usage">
    <h2>Usage data</h2>
    <Show when={error() || state.error}><ErrorNotice message={error() || state.error!.message} retry={() => void state.refetch()} /></Show>
    <Show when={state.data} fallback={<span class="loading-line" aria-label="Loading usage data" />}>{data => <>
      <div class="update-auto-row">
        <strong>{data().allowed ? "Share anonymous usage data" : "Usage data is off for this installation"}</strong>
        <Show when={data().allowed}>
          <label class="update-switch">
            <input type="checkbox" role="switch" aria-label="Share anonymous usage data" checked={enabled()} disabled={busy()} onChange={e => void toggle(e.currentTarget.checked)} />
            <span class="update-switch-track" aria-hidden="true" />
          </label>
        </Show>
      </div>
      <Show when={data().allowed}>
        <dl class="settings-group">
          <div class="setting-row"><dt>Last report</dt><dd>{data().lastSent ? operationTime(data().lastSent, true) : "Never"}</dd></div>
          <div class="setting-row"><dt>Interval</dt><dd>Daily</dd></div>
        </dl>
      </Show>
      <Show when={data().report}>{report => <details class="configuration">
        <summary>Report</summary>
        <pre class="diagnosis-log">{JSON.stringify(report(), null, 2)}</pre>
      </details>}</Show>
    </>}</Show>
  </section>;
}
