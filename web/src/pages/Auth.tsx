import Logo from "../components/Logo";
import { For, Show, createEffect, createMemo, createSignal, onMount } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { Button, ErrorNotice, IconButton } from "../components/Controls";
import { Icon, type IconName } from "../components/Icon";
import { api, post, APIError } from "../api";
import type { AuthMethods, Session } from "../types";

const providerNames: { id: keyof AuthMethods; label: string; icon: IconName }[] = [
  { id: "google", label: "Continue with Google", icon: "google" },
  { id: "github", label: "Continue with GitHub", icon: "github" },
  { id: "discord", label: "Continue with Discord", icon: "discord" },
];

const authErrors: Record<string, string> = {
  unprovisioned: "Your account has not been provisioned. Ask an administrator to add you in Shipper.",
  email_unverified: "The provider did not confirm a verified email address.",
  email_missing: "The provider did not return an email address.",
  ambiguous: "More than one account matches that email. Ask an administrator to resolve the identity link.",
  identity_conflict: "That sign-in identity is already linked to another account.",
  state_invalid: "The sign-in attempt expired. Start again.",
  access_denied: "Sign-in was cancelled.",
  provider_disabled: "That sign-in method is not enabled.",
  provider_not_found: "That sign-in method is not available.",
  account_disabled: "This account is disabled.",
  wrong_account: "Sign in with the account that opened the test.",
  password_change_required: "Set a permanent password before continuing.",
};

export default function AuthPage(props: { setup: boolean; onAuthenticated: (session: Session) => void; refreshSession: () => Promise<unknown> }) {
  createEffect(() => { document.title = (props.setup ? "Create administrator" : "Sign in") + " | Shipper"; });
  const [email, setEmail] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [token, setToken] = createSignal("");
  const [visible, setVisible] = createSignal(false);
  const [pending, setPending] = createSignal(false);
  const [error, setError] = createSignal("");
  const [callbackError, setCallbackError] = createSignal("");
  const methods = createQuery(() => ({
    queryKey: ["auth-methods"],
    queryFn: () => api<AuthMethods>("/auth/methods"),
    staleTime: 10_000,
    enabled: !props.setup,
  }));
  onMount(() => {
    const query = new URLSearchParams(window.location.search);
    const code = query.get("auth_error");
    if (!code) return;
    setCallbackError(authErrors[code] ?? "The sign-in provider could not complete the request.");
    query.delete("auth_error");
    const search = query.toString();
    window.history.replaceState({}, "", window.location.pathname + (search ? "?" + search : "") + window.location.hash);
  });
  const providers = createMemo(() => providerNames.filter(provider => methods.data?.[provider.id]));
  const hasMethods = createMemo(() => props.setup || !!methods.data?.password || providers().length > 0);
  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (pending()) return;
    setPending(true); setError("");
    try {
      const body = props.setup ? { email: email(), password: password(), setupToken: token().trim() } : { email: email(), password: password() };
      const session = await post<Session>(props.setup ? "/auth/setup" : "/auth/login", body);
      setPassword(""); setToken(""); props.onAuthenticated(session);
    } catch (error) {
      if (error instanceof APIError && error.code === "setup_complete") await props.refreshSession();
      else setError((error as Error).message);
    }
    finally { setPending(false); }
  }
  return <div class="app-frame auth-frame">
    <div class="topbar"><a class="brand" href="/" aria-label="Shipper"><Logo /><span>Shipper</span></a></div>
    <main class="app-panel auth-panel">
      <form class="auth-form" onSubmit={submit}>
        <h1>{props.setup ? "Create administrator" : "Sign in"}</h1>
        <Show when={callbackError()}><ErrorNotice message={callbackError()} /></Show>
        <Show when={!props.setup && methods.data}>
          <Show when={!hasMethods()}>
            <ErrorNotice message="No sign-in methods are enabled. Use shipperctl claim to restore access." />
          </Show>
          <Show when={providers().length}>
            <div class="auth-providers">
              <For each={providers()}>{provider =>
                <button type="button" class="button neutral auth-provider" onClick={() => window.location.assign("/auth/oauth/" + provider.id + "/start")}>
                  <Icon name={provider.icon} />{provider.label}
                </button>
              }</For>
            </div>
          </Show>
          <Show when={methods.data?.password && providers().length}><div class="auth-divider" role="separator" /></Show>
        </Show>
        <Show when={!props.setup && methods.isPending}><span class="loading-line" aria-label="Loading sign-in methods" /></Show>
        <Show when={props.setup || methods.data?.password}>
          <div class="form-fields">
            <Show when={props.setup}>
              <label for="setup-token">Setup token</label>
              <input id="setup-token" name="setupToken" class="input" type="password" autocomplete="off" required
                value={token()} onInput={event => setToken(event.currentTarget.value)} />
            </Show>
            <label for="email">Email</label>
            <input id="email" name="email" class="input" type="email" autocomplete="username" required
              value={email()} onInput={event => setEmail(event.currentTarget.value)} />
            <label for="password">Password</label>
            <div class="password-field">
              <input id="password" name="password" class="input" type={visible() ? "text" : "password"}
                autocomplete={props.setup ? "new-password" : "current-password"} minlength={props.setup ? 12 : undefined} maxlength={72} required
                value={password()} onInput={event => setPassword(event.currentTarget.value)} />
              <IconButton icon="eye" label={visible() ? "Hide password" : "Show password"} ghost onClick={() => setVisible(!visible())} />
            </div>
          </div>
        </Show>
        <Show when={error()}><ErrorNotice message={error()} /></Show>
        <Show when={props.setup || methods.data?.password}>
          <Button type="submit" variant="primary" disabled={pending()} class="auth-submit">
            {pending() ? (props.setup ? "Creating administrator" : "Signing in") : (props.setup ? "Create administrator" : "Sign in")}
          </Button>
        </Show>
      </form>
    </main>
  </div>;
}
