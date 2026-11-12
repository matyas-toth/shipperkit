import AuthShell from "../components/AuthShell";
import { For, Show, createEffect, createMemo, createSignal, onCleanup, onMount } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { Button, ErrorNotice, IconButton } from "../components/Controls";
import { Icon, type IconName } from "../components/Icon";
import { api, post, APIError } from "../api";
import { docs } from "../docs";
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
  const [mode, setMode] = createSignal<"create" | "restore">("create");
  const [archive, setArchive] = createSignal<File>();
  const [recoveryKey, setRecoveryKey] = createSignal("");
  const [restoring, setRestoring] = createSignal(false);
  let poll = 0;
  onCleanup(() => window.clearInterval(poll));

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
  const restoreMode = () => props.setup && mode() === "restore";

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

  async function restore(event: SubmitEvent) {
    event.preventDefault();
    if (pending()) return;
    if (!token().trim() || !archive() || !recoveryKey().trim()) {
      setError("Add the setup token, archive and recovery key.");
      return;
    }
    setPending(true); setError("");
    try {
      const response = await fetch("/api/auth/restore", {
        method: "POST",
        headers: {
          "Content-Type": "application/octet-stream",
          "X-Setup-Token": token().trim(),
          "X-Recovery-Key": recoveryKey().trim(),
        },
        body: archive(),
      });
      if (!response.ok) {
        const body = await response.json().catch(() => undefined);
        throw new Error(body?.error?.message ?? "The archive could not be restored.");
      }
      setRestoring(true);
      const deadline = Date.now() + 180_000;
      poll = window.setInterval(async () => {
        if (Date.now() > deadline) {
          window.clearInterval(poll);
          setRestoring(false); setPending(false);
          setError("Shipper is taking longer to restart. Reload this page to check.");
          return;
        }
        try {
          const health = await fetch("/api/health", { cache: "no-store" });
          if (!health.ok) return;
          const session = await fetch("/api/auth/session", { cache: "no-store" }).then(value => value.json());
          window.clearInterval(poll);
          if (session?.setupRequired) {
            setRestoring(false); setPending(false);
            setError("The restore did not complete. Check the archive and recovery key, then try again.");
            return;
          }
          window.location.assign("/");
        } catch {}
      }, 2000);
    } catch (error) {
      setError((error as Error).message);
      setPending(false);
    }
  }

  function handleSubmit(event: SubmitEvent) {
    if (restoreMode()) void restore(event);
    else void submit(event);
  }

  return <AuthShell>
    <form class="auth-form auth-card" onSubmit={handleSubmit}>
      <header class="auth-card-head">
        <span class="auth-badge"><Icon name={restoreMode() ? "upload" : props.setup ? "deploy" : "key"} /></span>
        <h1>{restoreMode() ? "Load a save." : props.setup ? "Begin the journey." : "Welcome back aboard."}</h1>
        <p class="auth-lede">{restoreMode() ? "Bring an installation back from its archive and recovery key." : props.setup ? "Claim this Shipper instance and give your applications a place to land." : "Your applications kept sailing while you were away."}</p>
      </header>
      <Show when={callbackError()}><ErrorNotice message={callbackError()} /></Show>
      <Show when={restoring()} fallback={
        <>
          <Show when={!props.setup && methods.data}>
            <Show when={!hasMethods()}>
              <ErrorNotice message="No sign-in methods are enabled. Ask the server operator to restore access." />
            </Show>
            <Show when={providers().length}>
              <div class="auth-providers">
                <For each={providers()}>{provider =>
                  <button type="button" class="auth-provider" onClick={() => window.location.assign("/auth/oauth/" + provider.id + "/start")}>
                    <span class="auth-provider-icon"><Icon name={provider.icon} /></span>
                    <span>{provider.label}</span>
                    <Icon name="chevron" class="auth-provider-arrow" />
                  </button>
                }</For>
              </div>
            </Show>
            <Show when={methods.data?.password && providers().length}><div class="auth-divider" role="separator"><span>or</span></div></Show>
          </Show>
          <Show when={!props.setup && methods.isPending}><span class="loading-line" aria-label="Loading sign-in methods" /></Show>
          <Show when={props.setup || methods.data?.password}>
            <div class="form-fields">
              <Show when={props.setup}>
                <label class="auth-label" for="setup-token">Setup token</label>
                <div class="auth-field">
                  <Icon name="key" />
                  <input id="setup-token" name="setupToken" class="input" type="password" autocomplete="off" required
                    value={token()} onInput={event => setToken(event.currentTarget.value)} />
                </div>
              </Show>
              <Show when={!restoreMode()}>
                <label class="auth-label" for="email">Email</label>
                <div class="auth-field">
                  <Icon name="mail" />
                  <input id="email" name="email" class="input" type="email" autocomplete="username" required
                    value={email()} onInput={event => setEmail(event.currentTarget.value)} />
                </div>
                <label class="auth-label" for="password">Password</label>
                <div class="auth-field password-field">
                  <Icon name="lock" />
                  <input id="password" name="password" class="input" type={visible() ? "text" : "password"}
                    autocomplete={props.setup ? "new-password" : "current-password"} minlength={props.setup ? 12 : undefined} maxlength={72} required
                    value={password()} onInput={event => setPassword(event.currentTarget.value)} />
                  <IconButton icon="eye" label={visible() ? "Hide password" : "Show password"} ghost onClick={() => setVisible(!visible())} />
                </div>
              </Show>
              <Show when={restoreMode()}>
                <label class="auth-label" for="restore-archive">Instance archive</label>
                <input id="restore-archive" class="input auth-file" type="file" accept=".age" required
                  onChange={event => setArchive(event.currentTarget.files?.[0])} />
                <label class="auth-label" for="restore-key">Recovery key</label>
                <textarea id="restore-key" class="input mono auth-key" rows={3} autocomplete="off" spellcheck={false} required
                  value={recoveryKey()} onInput={event => setRecoveryKey(event.currentTarget.value)} />
                <div class="auth-warning">
                  <Icon name="warning" />
                  <span>Restoring replaces this instance. Applications start stopped, and the saved administrator signs in.</span>
                </div>
              </Show>
            </div>
          </Show>
          <Show when={error()}><ErrorNotice message={error()} /></Show>
          <Show when={props.setup || methods.data?.password}>
            <Button type="submit" variant="primary" disabled={pending()} class="auth-submit">
              <span>{pending() ? (restoreMode() ? "Restoring Shipper" : props.setup ? "Creating administrator" : "Signing in") : (restoreMode() ? "Restore Shipper" : props.setup ? "Create administrator" : "Sign in")}</span>
              <Icon name="chevron" />
            </Button>
          </Show>
          <Show when={props.setup}>
            <button type="button" class="auth-switch" onClick={() => { setError(""); setMode(restoreMode() ? "create" : "restore"); }}>
              {restoreMode() ? "Create a new administrator instead" : "Restore a save"}
            </button>
          </Show>
        </>
      }>
        <div class="auth-restoring" role="status">
          <span class="loading-line" />
          <p>Restoring Shipper and restarting the dashboard. Keep this page open.</p>
        </div>
      </Show>
      <Show when={!props.setup}><p class="auth-foot">Need access? Ask your Shipper administrator.</p></Show>
      <Show when={props.setup}>
        <a class="auth-docs-link" href={docs("install/")} target="_blank" rel="noreferrer">Installation guide</a>
      </Show>
    </form>
  </AuthShell>;
}
