import { Show, createSignal } from "solid-js";
import { post } from "../api";
import { useAuth } from "../context";
import type { Session } from "../types";
import { Button, ErrorNotice } from "./Controls";

export function FactorEnrollment(props: { cancel: () => void; cancelLabel: string }) {
  const auth = useAuth();
  const [password, setPassword] = createSignal("");
  const [code, setCode] = createSignal("");
  const [setup, setSetup] = createSignal<{ key: string; qr: string }>();
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  let codeInput!: HTMLInputElement;
  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (busy()) return;
    setBusy(true); setError("");
    try {
      const csrf = auth.session()?.csrfToken;
      if (setup()) auth.accept(await post<Session>("/auth/factor/confirm", { code: code() }, csrf));
      else {
        setSetup(await post("/auth/factor/start", { password: password() }, csrf));
        setPassword("");
        queueMicrotask(() => codeInput?.focus());
      }
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  return <form class="factor-form" onSubmit={submit}>
    <Show when={setup()} fallback={<div class="form-fields">
      <label for="factor-password">Current password</label>
      <input id="factor-password" class="input" type="password" autocomplete="current-password" required value={password()} onInput={e => setPassword(e.currentTarget.value)} />
    </div>}>{value => <div class="form-fields">
      <img class="factor-qr" src={value().qr} alt="Scan with your authenticator" width="184" height="184" />
      <label for="factor-key">Setup key</label>
      <input id="factor-key" class="input mono" readonly value={value().key} onFocus={e => e.currentTarget.select()} />
      <label for="factor-code">Authenticator code</label>
      <input id="factor-code" ref={codeInput} class="input mono" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" maxlength={6}
        required value={code()} onInput={e => setCode(e.currentTarget.value)} />
    </div>}</Show>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <div class="form-actions">
      <Button variant="ghost" disabled={busy()} onClick={props.cancel}>{props.cancelLabel}</Button>
      <Button type="submit" disabled={busy()}>{setup() ? "Enable 2FA" : "Continue"}</Button>
    </div>
  </form>;
}

type Change = "disable" | "recovery_codes" | "policy";
export default function AccountSecurity(props: { scope?: "account" | "instance" }) {
  const auth = useAuth();
  const [action, setAction] = createSignal<"enroll" | Change>();
  const [password, setPassword] = createSignal("");
  const [code, setCode] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  const instance = () => props.scope === "instance";
  const factor = () => auth.session()?.factor;
  const policyAction = () => factor()?.required ? "Make 2FA optional" : "Require 2FA";
  const label = () => action() === "disable" ? "Disable 2FA" : action() === "recovery_codes" ? "Replace recovery codes" : policyAction();
  function choose(value?: "enroll" | Change) {
    setAction(value); setPassword(""); setCode(""); setError("");
  }
  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (busy()) return;
    setBusy(true); setError("");
    try {
      auth.accept(await post<Session>("/auth/factor/change", {
        action: action(), password: password(), code: code(),
        required: action() === "policy" ? !factor()?.required : undefined,
      }, auth.session()?.csrfToken));
      choose();
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  return <div class="account-security">
    <dl class="settings-group">
      <Show when={!instance()}>
        <div class="setting-row"><dt>Two-factor authentication</dt><dd>
          <Show when={factor()?.enrolled} fallback={<Button small onClick={() => choose("enroll")}>Set up 2FA</Button>}>
            <span>Enabled</span>
            <Show when={!factor()?.required}><Button small variant="ghost" onClick={() => choose("disable")}>Disable</Button></Show>
          </Show>
        </dd></div>
        <Show when={factor()?.enrolled}><div class="setting-row"><dt>Recovery codes</dt><dd>
          <Button small variant="ghost" onClick={() => choose("recovery_codes")}>Replace codes</Button>
        </dd></div></Show>
      </Show>
      <Show when={instance()}>
        <div class="setting-row"><dt>Two-factor authentication</dt><dd>
          <span>{factor()?.required ? "Required" : "Optional"}</span>
          <Button small onClick={() => choose("policy")}>{policyAction()}</Button>
        </dd></div>
      </Show>
    </dl>
    <Show when={!instance() && action() === "enroll" && !factor()?.enrolled}><FactorEnrollment cancel={() => choose()} cancelLabel="Cancel" /></Show>
    <Show when={action() && action() !== "enroll"}>
      <form class="factor-form" onSubmit={submit}>
        <h2>{label()}</h2>
        <div class="form-fields">
          <label for="change-factor-password">Current password</label>
          <input id="change-factor-password" class="input" type="password" autocomplete="current-password" required value={password()} onInput={e => setPassword(e.currentTarget.value)} />
          <Show when={factor()?.enrolled}>
            <label for="change-factor-code">Authenticator code</label>
            <input id="change-factor-code" class="input mono" inputmode="numeric" autocomplete="one-time-code"
              pattern="[0-9]{6}" maxlength={6} required value={code()} onInput={e => setCode(e.currentTarget.value)} />
          </Show>
        </div>
        <Show when={error()}><ErrorNotice message={error()} /></Show>
        <div class="form-actions"><Button variant="ghost" disabled={busy()} onClick={() => choose()}>Cancel</Button><Button type="submit" disabled={busy()}>{label()}</Button></div>
      </form>
    </Show>
  </div>;
}
