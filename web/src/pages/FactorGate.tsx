import { For, Match, Show, Switch, createEffect, createSignal } from "solid-js";
import { useAuth } from "../context";
import { post } from "../api";
import type { Session } from "../types";
import { Button, ErrorNotice } from "../components/Controls";
import { FactorEnrollment } from "../components/AccountSecurity";
import Logo from "../components/Logo";

export default function FactorGate() {
  const auth = useAuth();
  const [code, setCode] = createSignal("");
  const [recovery, setRecovery] = createSignal(false);
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  const codes = () => auth.session()?.recoveryCodes ?? [];
  const title = () => codes().length ? "Save recovery codes" : auth.session()?.authStep === "enroll" ? "Set up two-factor authentication" : "Verify sign-in";
  createEffect(() => { document.title = title() + " | Shipper"; });
  async function verify(event: SubmitEvent) {
    event.preventDefault();
    if (busy()) return;
    setBusy(true); setError("");
    try { auth.accept(await post<Session>("/auth/factor/verify", { code: code() }, auth.session()?.csrfToken)); }
    catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function logout() {
    try { await auth.logout(); }
    catch (e) { setError((e as Error).message); }
  }
  return <div class="app-frame auth-frame">
    <div class="topbar"><span class="brand"><Logo /><span>Shipper</span></span></div>
    <main class="app-panel auth-panel">
      <section class="auth-form factor-gate">
        <h1>{title()}</h1>
        <Switch>
          <Match when={codes().length}>
            <ul class="factor-codes mono"><For each={codes()}>{value => <li>{value}</li>}</For></ul>
            <div class="form-actions">
              <Button variant="ghost" icon="copy" onClick={() => void navigator.clipboard.writeText(codes().join("\n")).catch(() => setError("The codes could not be copied"))}>Copy codes</Button>
              <Button onClick={() => auth.accept({ ...auth.session()!, recoveryCodes: undefined })}>Done</Button>
            </div>
          </Match>
          <Match when={auth.session()?.authStep === "enroll"}>
            <FactorEnrollment cancel={() => void logout()} cancelLabel="Sign out" />
          </Match>
          <Match when={auth.session()?.authStep === "verify"}>
            <form class="factor-form" onSubmit={verify}>
              <div class="form-fields">
                <label for="login-code">{recovery() ? "Recovery code" : "Authenticator code"}</label>
                <input id="login-code" class="input mono" inputmode={recovery() ? "text" : "numeric"} autocomplete="one-time-code"
                  required maxlength={recovery() ? 26 : 6} pattern={recovery() ? undefined : "[0-9]{6}"}
                  value={code()} onInput={e => setCode(e.currentTarget.value)} />
              </div>
              <div class="form-actions">
                <Button variant="ghost" disabled={busy()} onClick={() => { setRecovery(v => !v); setCode(""); setError(""); }}>{recovery() ? "Use authenticator" : "Use recovery code"}</Button>
                <Button type="submit" disabled={busy()}>Verify</Button>
              </div>
            </form>
            <Button variant="ghost" disabled={busy()} onClick={() => void logout()}>Sign out</Button>
          </Match>
        </Switch>
        <Show when={error()}><ErrorNotice message={error()} /></Show>
      </section>
    </main>
  </div>;
}
