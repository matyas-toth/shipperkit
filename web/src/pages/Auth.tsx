import { Show, createSignal, createEffect } from "solid-js";
import { Button, ErrorNotice, IconButton } from "../components/Controls";
import { Icon } from "../components/Icon";
import { post, APIError } from "../api";
import type { Session } from "../types";

export default function AuthPage(props: { setup: boolean; onAuthenticated: (session: Session) => void; refreshSession: () => Promise<unknown> }) {
  createEffect(() => { document.title = (props.setup ? "Create administrator" : "Sign in") + " | Shipper"; });
  const [email, setEmail] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [token, setToken] = createSignal("");
  const [visible, setVisible] = createSignal(false);
  const [pending, setPending] = createSignal(false);
  const [error, setError] = createSignal("");
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
    <div class="topbar"><a class="brand" href="/" aria-label="Shipper"><Icon name="ship" /><span>Shipper</span></a></div>
    <main class="app-panel auth-panel">
      <form class="auth-form" onSubmit={submit}>
        <h1>{props.setup ? "Create administrator" : "Sign in"}</h1>
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
        <Show when={error()}><ErrorNotice message={error()} /></Show>
        <Button type="submit" disabled={pending()} class="auth-submit">
          {pending() ? (props.setup ? "Creating administrator" : "Signing in") : (props.setup ? "Create administrator" : "Sign in")}
        </Button>
      </form>
    </main>
  </div>;
}

