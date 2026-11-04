import { createSignal, Show } from "solid-js";
import type { ProxyConfig } from "../types";
import { post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice } from "./Controls";

export default function ProxyCheck(props: { proxy?: ProxyConfig }) {
  const auth = useAuth(),
    [busy, setBusy] = createSignal(false),
    [error, setError] = createSignal(""),
    [result, setResult] = createSignal<{
      state: string;
      message: string;
      certificateExpiresAt?: number;
      certificateIssuer?: string;
    }>();
  async function check() {
    setBusy(true);
    setError("");
    try {
      setResult(await post("/proxy/check", {}, auth.session()?.csrfToken));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div class="proxy-check">
      <Show when={error()}>
        <ErrorNotice message={error()} />
      </Show>
      <dl class="settings-group">
        <div class="setting-row">
          <dt>Dashboard HTTPS</dt>
          <dd>{props.proxy?.dashboardURL || "Not configured"}</dd>
        </div>
        <div class="setting-row">
          <dt>Certificate renewal</dt>
          <dd>
            {props.proxy?.acmeEmail
              ? "Automatic through ACME"
              : "Not configured"}
          </dd>
        </div>
      </dl>
      <Show when={props.proxy?.dashboardURL}>
        <Button icon="refresh" disabled={busy()} onClick={() => void check()}>
          {busy() ? "Checking" : "Verify dashboard"}
        </Button>
      </Show>
      <Show when={result()}>
        {(value) => (
          <dl class="settings-group">
            <div class="setting-row">
              <dt>Verification</dt>
              <dd>{value().message}</dd>
            </div>
            <Show when={value().certificateExpiresAt}>
              <div class="setting-row">
                <dt>Certificate issuer</dt>
                <dd>{value().certificateIssuer}</dd>
              </div>
              <div class="setting-row">
                <dt>Certificate expires</dt>
                <dd>
                  {new Date(
                    value().certificateExpiresAt! * 1000,
                  ).toLocaleString()}
                </dd>
              </div>
            </Show>
          </dl>
        )}
      </Show>
    </div>
  );
}
