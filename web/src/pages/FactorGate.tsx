import { For, Match, Show, Switch, createEffect, createSignal } from "solid-js";
import { useAuth } from "../context";
import { post } from "../api";
import type { Session } from "../types";
import { Button, ErrorNotice } from "../components/Controls";
import { FactorEnrollment } from "../components/AccountSecurity";
import AuthShell from "../components/AuthShell";
import { Icon, type IconName } from "../components/Icon";

export default function FactorGate() {
  const auth = useAuth();
  const [code, setCode] = createSignal("");
  const [recovery, setRecovery] = createSignal(false);
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  const [currentPassword, setCurrentPassword] = createSignal("");
  const [newPassword, setNewPassword] = createSignal("");
  const [confirmPassword, setConfirmPassword] = createSignal("");
  const codes = () => auth.session()?.recoveryCodes ?? [];
  const step = () => auth.session()?.authStep;
  const title = () => codes().length ? "Keep these somewhere safe." : step() === "password_change" ? "Make it yours." : step() === "enroll" ? "Add a second lock." : "One more check.";
  const lede = () => codes().length ? "Each recovery code works once if you lose your authenticator." : step() === "password_change" ? "Swap the temporary password for one only you know." : step() === "enroll" ? "Scan the code with your authenticator app." : "Confirm the code from your authenticator.";
  const badge = (): IconName => codes().length ? "key" : step() === "password_change" ? "lock" : step() === "enroll" ? "security" : "shield";
  createEffect(() => { document.title = title() + " | Shipper"; });
  async function verify(event: SubmitEvent) {
    event.preventDefault();
    if (busy()) return;
    setBusy(true); setError("");
    try { auth.accept(await post<Session>("/auth/factor/verify", { code: code() }, auth.session()?.csrfToken)); }
    catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function changePassword(event: SubmitEvent) {
    event.preventDefault();
    if (busy()) return;
    if (newPassword() !== confirmPassword()) {
      setError("The passwords do not match");
      return;
    }
    setBusy(true); setError("");
    try {
      auth.accept(await post<Session>("/auth/password", { currentPassword: currentPassword(), newPassword: newPassword() }, auth.session()?.csrfToken));
      setCurrentPassword(""); setNewPassword(""); setConfirmPassword("");
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function logout() {
    try { await auth.logout(); }
    catch (e) { setError((e as Error).message); }
  }
  return <AuthShell>
    <section class="auth-form auth-card factor-gate">
      <header class="auth-card-head">
        <span class="auth-badge"><Icon name={badge()} /></span>
        <h1>{title()}</h1>
        <p class="auth-lede">{lede()}</p>
      </header>
      <Switch>
        <Match when={codes().length}>
          <ul class="factor-codes mono"><For each={codes()}>{value => <li>{value}</li>}</For></ul>
          <div class="form-actions">
            <Button variant="ghost" icon="copy" onClick={() => void navigator.clipboard.writeText(codes().join("\n")).catch(() => setError("The codes could not be copied"))}>Copy codes</Button>
            <Button onClick={() => auth.accept({ ...auth.session()!, recoveryCodes: undefined })}>Done</Button>
          </div>
        </Match>
        <Match when={step() === "password_change"}>
          <form class="factor-form" onSubmit={changePassword}>
            <div class="form-fields">
              <label class="auth-label" for="current-password">Current password</label>
              <div class="auth-field">
                <Icon name="lock" />
                <input id="current-password" class="input" type="password" autocomplete="current-password" required
                  value={currentPassword()} onInput={e => setCurrentPassword(e.currentTarget.value)} />
              </div>
              <label class="auth-label" for="new-password">New password</label>
              <div class="auth-field">
                <Icon name="lock" />
                <input id="new-password" class="input" type="password" autocomplete="new-password" minlength={12} maxlength={72} required
                  value={newPassword()} onInput={e => setNewPassword(e.currentTarget.value)} />
              </div>
              <label class="auth-label" for="confirm-password">Confirm new password</label>
              <div class="auth-field">
                <Icon name="lock" />
                <input id="confirm-password" class="input" type="password" autocomplete="new-password" minlength={12} maxlength={72} required
                  value={confirmPassword()} onInput={e => setConfirmPassword(e.currentTarget.value)} />
              </div>
            </div>
            <div class="form-actions">
              <Button variant="ghost" disabled={busy()} onClick={() => void logout()}>Sign out</Button>
              <Button type="submit" variant="primary" class="auth-submit" disabled={busy()}>Save password</Button>
            </div>
          </form>
        </Match>
        <Match when={step() === "enroll"}>
          <FactorEnrollment cancel={() => void logout()} cancelLabel="Sign out" />
        </Match>
        <Match when={step() === "verify"}>
          <form class="factor-form" onSubmit={verify}>
            <div class="form-fields">
              <label class="auth-label" for="login-code">{recovery() ? "Recovery code" : "Authenticator code"}</label>
              <div class="auth-field">
                <Icon name={recovery() ? "key" : "shield"} />
                <input id="login-code" class="input mono" inputmode={recovery() ? "text" : "numeric"} autocomplete="one-time-code"
                  required maxlength={recovery() ? 26 : 6} pattern={recovery() ? undefined : "[0-9]{6}"}
                  value={code()} onInput={e => setCode(e.currentTarget.value)} />
              </div>
            </div>
            <div class="form-actions">
              <Button variant="ghost" disabled={busy()} onClick={() => { setRecovery(v => !v); setCode(""); setError(""); }}>{recovery() ? "Use authenticator" : "Use recovery code"}</Button>
              <Button type="submit" variant="primary" class="auth-submit" disabled={busy()}>Verify</Button>
            </div>
            <Button variant="ghost" disabled={busy()} onClick={() => void logout()}>Sign out</Button>
          </form>
        </Match>
      </Switch>
      <Show when={error()}><ErrorNotice message={error()} /></Show>
    </section>
  </AuthShell>;
}
