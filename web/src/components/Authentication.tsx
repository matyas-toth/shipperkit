import { For, Show, createSignal, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import type { AuthProviderSettings, AuthSettings } from "../types";
import { Button, ErrorNotice, IconButton, Status } from "./Controls";
import { Icon, type IconName } from "./Icon";

const providerCopy: Record<string, { label: string; icon: IconName; scopes: string; help: string[] }> = {
  google: {
    label: "Google", icon: "google", scopes: "openid email profile",
    help: [
      "Create a Web application OAuth client in Google Cloud Console.",
      "Add the callback above to Authorized redirect URIs.",
      "Copy the client ID and client secret here.",
    ],
  },
  github: {
    label: "GitHub", icon: "github", scopes: "read:user user:email",
    help: [
      "Create an OAuth App under Settings > Developer settings.",
      "Use the dashboard origin as the homepage and add the callback above as the authorization callback URL.",
      "Copy the client ID and client secret here. Repository permissions are not needed.",
    ],
  },
  discord: {
    label: "Discord", icon: "discord", scopes: "identify email",
    help: [
      "Create an OAuth2 application in the Discord Developer Portal.",
      "Add the callback above under OAuth2 Redirects.",
      "Copy the client ID and client secret here. No bot or guild permissions are needed.",
    ],
  },
};
const testErrors: Record<string, string> = {
  wrong_account: "Sign in with the account that opened the test.",
  state_invalid: "The test expired. Start it again.",
  access_denied: "The test was cancelled.",
  unprovisioned: "That provider account has no provisioned Shipper user.",
  email_missing: "The provider did not return an email address.",
  email_unverified: "The provider did not confirm a verified email address.",
  ambiguous: "More than one Shipper user matches that email.",
  identity_conflict: "That sign-in identity is already linked to another account.",
  account_disabled: "This account is disabled.",
};

export default function Authentication() {
  const client = useQueryClient();
  const auth = useAuth();
  const [error, setError] = createSignal("");
  const [result, setResult] = createSignal("");
  const [removing, setRemoving] = createSignal<AuthProviderSettings>();
  const [busy, setBusy] = createSignal(false);
  const settings = createQuery(() => ({ queryKey: ["auth-settings"], queryFn: () => api<AuthSettings>("/auth/providers") }));
  onMount(() => {
    const query = new URLSearchParams(window.location.search);
    const provider = query.get("oauth");
    if (!provider) return;
    const label = providerCopy[provider]?.label ?? provider;
    if (query.get("oauth_result") === "success") setResult(label + " returned to Shipper");
    else setError(testErrors[query.get("oauth_code") ?? ""] ?? "The provider test failed. Check the credentials and callback.");
    query.delete("oauth"); query.delete("oauth_result"); query.delete("oauth_code");
    const search = query.toString();
    window.history.replaceState({}, "", window.location.pathname + (search ? "?" + search : "") + window.location.hash);
  });
  async function savePassword(enabled: boolean) {
    setBusy(true); setError("");
    try {
      await post("/auth/password-policy", { enabled }, auth.session()?.csrfToken);
      await client.invalidateQueries({ queryKey: ["auth-settings"] });
      await client.invalidateQueries({ queryKey: ["auth-methods"] });
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function remove(item: AuthProviderSettings) {
    setBusy(true); setError("");
    try {
      await post("/auth/providers/" + item.provider + "/remove", {}, auth.session()?.csrfToken);
      setRemoving(undefined);
      await client.invalidateQueries({ queryKey: ["auth-settings"] });
      await client.invalidateQueries({ queryKey: ["auth-methods"] });
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  return <section class="settings-section" id="authentication">
    <h2>Sign-in methods</h2>
    <Show when={settings.error}><ErrorNotice message={settings.error!.message} retry={() => void settings.refetch()} /></Show>
    <Show when={result()}><div class="provider-result" role="status"><Icon name="check" />{result()}</div></Show>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <dl class="settings-group">
      <div class="setting-row"><dt>Email and password</dt><dd class="method-control">
        <label class="update-switch"><input type="checkbox" aria-label="Enable email and password sign-in" checked={settings.data?.passwordEnabled ?? true} disabled={busy() || !settings.data} onChange={event => void savePassword(event.currentTarget.checked)} /><span class="update-switch-track" /></label>
        <span class="method-state">{settings.data?.passwordEnabled ? "Enabled" : "Disabled"}</span>
      </dd></div>
    </dl>
    <h2 class="settings-subheading">Providers</h2>
    <div class="provider-list">
      <For each={settings.data?.providers}>{item => <ProviderCard item={item} remove={() => setRemoving(item)} />}</For>
    </div>
    <Show when={removing()}>{item => <Portal>
      <div class="drawer-scrim" onClick={() => setRemoving(undefined)} />
      <div class="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="remove-provider-title">
        <header><h2 id="remove-provider-title">Remove {providerCopy[item().provider]?.label}</h2></header>
        <p>Remove the provider configuration and unlink every {providerCopy[item().provider]?.label} identity?</p>
        <Show when={error()}><ErrorNotice message={error()} /></Show>
        <div class="form-actions"><Button variant="ghost" disabled={busy()} onClick={() => setRemoving(undefined)}>Cancel</Button><Button variant="primary" disabled={busy()} onClick={() => void remove(item())}>Remove</Button></div>
      </div>
    </Portal>}</Show>
  </section>;
}

function ProviderCard(props: { item: AuthProviderSettings; remove: () => void }) {
  const client = useQueryClient();
  const auth = useAuth();
  const copy = () => providerCopy[props.item.provider];
  const [clientID, setClientID] = createSignal(props.item.clientId);
  const [secret, setSecret] = createSignal("");
  const [enabled, setEnabled] = createSignal(props.item.enabled);
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  const [copied, setCopied] = createSignal(false);
  async function save() {
    setBusy(true); setError("");
    try {
      await post("/auth/providers/" + props.item.provider, { clientId: clientID(), clientSecret: secret(), enabled: enabled() }, auth.session()?.csrfToken);
      setSecret("");
      await client.invalidateQueries({ queryKey: ["auth-settings"] });
      await client.invalidateQueries({ queryKey: ["auth-methods"] });
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function test() {
    setBusy(true); setError("");
    try {
      const response = await post<{ url: string }>("/auth/providers/" + props.item.provider + "/test", {}, auth.session()?.csrfToken);
      window.open(response.url, "_blank", "width=560,height=760");
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function copyCallback() {
    try { await navigator.clipboard.writeText(props.item.callback); setCopied(true); setTimeout(() => setCopied(false), 1600); }
    catch { setError("The callback could not be copied"); }
  }
  return <article class="provider-card" classList={{ "is-enabled": props.item.enabled }}>
    <header>
      <span class="provider-heading"><span class="provider-icon"><Icon name={copy().icon} /></span><strong>{copy().label}</strong></span>
      <Status value={props.item.enabled ? "enabled" : "disabled"} label={props.item.enabled ? "Enabled" : "Disabled"} />
    </header>
    <div class="provider-fields">
      <label for={"client-id-" + props.item.provider}>Client ID</label>
      <input id={"client-id-" + props.item.provider} class="input mono" autocomplete="off" value={clientID()} onInput={e => setClientID(e.currentTarget.value)} />
      <label for={"client-secret-" + props.item.provider}>Client secret</label>
      <div class="password-field">
        <input id={"client-secret-" + props.item.provider} class="input mono" type="password" autocomplete="new-password"
          value={secret()} onInput={e => setSecret(e.currentTarget.value)} />
        <span class="secret-state">{props.item.hasSecret ? "Saved" : "Not set"}</span>
      </div>
      <label for={"callback-" + props.item.provider}>Callback</label>
      <div class="callback-field">
        <code id={"callback-" + props.item.provider}>{props.item.callback}</code>
        <IconButton icon={copied() ? "check" : "copy"} label={copied() ? "Callback copied" : "Copy callback"} ghost onClick={() => void copyCallback()} />
      </div>
    </div>
    <p class="provider-scopes"><span>Scopes</span><code class="mono">{copy().scopes}</code></p>
    <details class="provider-help"><summary>Setup steps</summary><ol><For each={copy().help}>{step => <li>{step}</li>}</For></ol></details>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <footer>
      <span class="provider-enable"><label class="update-switch"><input type="checkbox" aria-label={"Enable " + copy().label + " sign-in"} checked={enabled()} onChange={e => setEnabled(e.currentTarget.checked)} /><span class="update-switch-track" /></label><span>{enabled() ? "Enabled" : "Disabled"}</span></span>
      <div class="provider-actions">
        <Button small variant="ghost" disabled={busy() || !props.item.configured || !props.item.enabled} onClick={() => void test()}>Test sign-in</Button>
        <Button small variant="ghost" disabled={busy()} onClick={props.remove}>Remove</Button>
        <Button small variant="primary" disabled={busy()} onClick={() => void save()}>Save</Button>
      </div>
    </footer>
  </article>;
}
