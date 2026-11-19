import { For, Show, createSignal } from "solid-js";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import { createdDate } from "../data";
import type { Application } from "../types";
import { Button, ConfirmDialog, ErrorNotice } from "./Controls";

interface Credential { id: string; name: string; scopes: string[]; applications: string[]; expiresAt: number }
const appScopes = ["read", "logs", "deploy", "execute", "secrets", "destructive"];
const permissionLabel: Record<string, string> = { read: "Read", logs: "Logs", deploy: "Deploy", execute: "Run tasks", secrets: "Reveal secrets", destructive: "Delete applications", "host-admin": "Host maintenance", full: "Full app access" };

export default function AccessTokens() {
  const auth = useAuth(), client = useQueryClient();
  const [error, setError] = createSignal(""), [busy, setBusy] = createSignal(false), [token, setToken] = createSignal(""), [copied, setCopied] = createSignal(false);
  const [name, setName] = createSignal(""), [scope, setScope] = createSignal(["read"]), [app, setApp] = createSignal(""), [days, setDays] = createSignal(30), [creating, setCreating] = createSignal(false);
  const [revoking, setRevoking] = createSignal<Credential>();
  const credentials = createQuery(() => ({ queryKey: ["operation-tokens"], queryFn: () => api<{ items: Credential[] }>("/operation-tokens") }));
  const applications = createQuery(() => ({ queryKey: ["system", "applications"], queryFn: () => api<{ items: Application[] }>("/applications") }));
  const full = () => scope().includes("full");
  // Full access already grants every application scope, and host maintenance only applies to unscoped tokens.
  const implied = (permission: string) => full() && appScopes.includes(permission);
  const unavailable = (permission: string) => permission === "host-admin" && !!app();
  const checked = (permission: string) => implied(permission) || scope().includes(permission) && !unavailable(permission);
  function toggle(permission: string, on: boolean) {
    setScope(current => on ? [...new Set([...current, permission])] : current.filter(value => value !== permission));
  }
  const requested = () => scope().filter(value => !unavailable(value) && !(full() && appScopes.includes(value)));
  const applicationName = (id: string) => applications.data?.items.find(item => item.id === id)?.name ?? id;
  async function create(e: SubmitEvent) {
    e.preventDefault();
    if (busy()) return;
    setBusy(true); setError("");
    try {
      const value = await post<{ token: string }>("/operation-tokens", { name: name(), scopes: requested(), applications: app() ? [app()] : [], days: days() }, auth.session()?.csrfToken);
      setToken(value.token); setCopied(false); setName(""); setScope(["read"]); setApp(""); setCreating(false);
      await credentials.refetch();
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  async function copy() {
    try { await navigator.clipboard.writeText(token()); setCopied(true); } catch { setError("The token could not be copied"); }
  }
  return <section class="settings-section" id="tokens">
    <div class="section-toolbar"><h2>Access tokens</h2><Show when={!creating()}><Button icon="plus" disabled={busy()} onClick={() => { setToken(""); setCreating(true); }}>Create token</Button></Show></div>
    <Show when={error() || credentials.error}><ErrorNotice message={error() || credentials.error!.message} /></Show>
    <Show when={token()}><div class="token-reveal" role="status">
      <strong>Copy the token now because it is shown only once</strong>
      <div class="token-reveal-row"><input class="input mono" aria-label="New access token" readonly value={token()} onFocus={e => e.currentTarget.select()} />
        <Button variant="primary" icon={copied() ? "check" : "copy"} onClick={() => void copy()}>{copied() ? "Copied" : "Copy"}</Button>
        <Button variant="ghost" onClick={() => setToken("")}>Done</Button></div>
    </div></Show>
    <Show when={creating()}><form onSubmit={create} class="token-form">
      <div class="section-toolbar"><h3>New token</h3><Button variant="ghost" disabled={busy()} onClick={() => setCreating(false)}>Cancel</Button></div>
      <div class="form-fields">
        <label for="token-name">Name</label><input id="token-name" class="input" required maxlength="80" value={name()} onInput={e => setName(e.currentTarget.value)} />
        <label for="token-app">Application</label><select id="token-app" class="input" value={app()} onChange={e => setApp(e.currentTarget.value)}><option value="">All applications</option><For each={applications.data?.items}>{a => <option value={a.id}>{a.name}</option>}</For></select>
        <label for="token-days">Expires in days</label><input id="token-days" class="input" type="number" min="1" max="365" value={days()} onInput={e => setDays(Number(e.currentTarget.value))} />
      </div>
      <div class="capability-choices"><For each={["full", ...appScopes, "host-admin"]}>{permission => <label classList={{ "is-implied": implied(permission) || unavailable(permission) }}>
        <input type="checkbox" checked={checked(permission)} disabled={implied(permission) || unavailable(permission)} onChange={e => toggle(permission, e.currentTarget.checked)} />{permissionLabel[permission]}
      </label>}</For></div>
      <div class="form-actions"><Button variant="primary" type="submit" disabled={busy() || !requested().length}>Create token</Button></div>
    </form></Show>
    <div class="table-tray"><table class="inventory-table access-tokens-table"><thead><tr><th>Name</th><th>Permissions</th><th>Applications</th><th>Expires</th><th aria-label="Actions" /></tr></thead><tbody>
      <For each={credentials.data?.items}>{credential => <tr>
        <td>{credential.name}</td>
        <td>{credential.scopes.map(v => permissionLabel[v] || v).join(", ")}</td>
        <td>{credential.applications.length ? credential.applications.map(applicationName).join(", ") : "All"}</td>
        <td>{createdDate(credential.expiresAt)}</td>
        <td><Button small variant="ghost" disabled={busy()} onClick={() => setRevoking(credential)}>Revoke</Button></td>
      </tr>}</For>
      <Show when={credentials.data && !credentials.data.items.length}><tr class="empty-row"><td colspan="5">No tokens</td></tr></Show>
    </tbody></table></div>
    <Show when={revoking()}>{item => <ConfirmDialog title={"Revoke " + item().name + "?"} message="Anything using this token loses access immediately." confirm="Revoke token" danger
      close={() => setRevoking(undefined)} run={async () => {
        await post(`/operation-tokens/${item().id}/revoke`, {}, auth.session()?.csrfToken);
        client.setQueryData<{ items: Credential[] } | undefined>(["operation-tokens"], old => old && { items: old.items.filter(value => value.id !== item().id) });
        void credentials.refetch();
      }} />}</Show>
  </section>;
}
