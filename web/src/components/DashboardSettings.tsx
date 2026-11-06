import { createEffect, createSignal, Show } from "solid-js";
import type { ProxyConfig } from "../types";
import { post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice } from "./Controls";
import ProxyCheck from "./ProxyCheck";

export default function DashboardSettings(props: { proxy?: ProxyConfig; refresh: () => Promise<unknown> }) {
  const auth = useAuth();
  const [domain, setDomain] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  const [saved, setSaved] = createSignal("");
  const current = () => {
    try {
      return props.proxy?.dashboardURL ? new URL(props.proxy.dashboardURL).hostname : "";
    } catch {
      return "";
    }
  };
  createEffect(() => {
    if (props.proxy) setDomain(current());
  });
  async function save(value: string) {
    if (busy()) return;
    const target = value.trim().toLowerCase();
    setBusy(true);
    setError("");
    setSaved("");
    try {
      await post("/proxy", { domain: target }, auth.session()?.csrfToken);
      for (let attempt = 0; attempt < 20; attempt++) {
        await new Promise(resolve => setTimeout(resolve, 400));
        await props.refresh();
        if (current() === target) break;
      }
      setSaved(target ? "Domain saved" : "Domain removed");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return <section class="settings-section" id="dashboard">
    <h2>Dashboard</h2>
    <form class="dashboard-domain" onSubmit={event => { event.preventDefault(); void save(domain()); }}>
      <div class="form-fields">
        <label for="dashboard-domain">Domain</label>
        <input id="dashboard-domain" class="input mono" placeholder="shipper.example.com" disabled={!props.proxy}
          value={domain()} onInput={event => setDomain(event.currentTarget.value)} />
      </div>
      <div class="form-actions">
        <Show when={current()}><Button type="button" variant="ghost" disabled={busy() || !props.proxy} onClick={() => void save("")}>Remove domain</Button></Show>
        <Button type="submit" variant="primary" disabled={busy() || !props.proxy || domain().trim().toLowerCase() === current()}>{busy() ? "Applying" : "Save domain"}</Button>
      </div>
    </form>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <Show when={saved()}><p class="settings-result" role="status">{saved()}</p></Show>
    <ProxyCheck proxy={props.proxy} />
  </section>;
}
