import { Show, createSignal } from "solid-js";
import { post } from "../api";
import { useAuth } from "../context";
import type { StaticConfig } from "../types";
import { Button, Disclosure, ErrorNotice } from "./Controls";
export function AutomaticDeployFields(props: {
  config: StaticConfig;
  change: (key: keyof StaticConfig, value: unknown) => void;
}) {
  const value = () =>
    props.config.autoDeploy ?? { enabled: false, paths: [], supersede: true };
  return (
    <Show when={props.config.sourceId}>
      <details class="configuration">
        <summary>Automatic deployments</summary>
        <div class="form-fields configuration-fields">
          <label class="check-field">
            <input
              type="checkbox"
              checked={value().enabled}
              onChange={(e) =>
                props.change("autoDeploy", {
                  ...value(),
                  enabled: e.currentTarget.checked,
                })
              }
            />
            Deploy on push
          </label>
          <Show when={value().enabled}>
            <label for="watch-paths">Watch paths</label>
            <textarea
              id="watch-paths"
              class="input mono command-input"
              placeholder="All files"
              value={(value().paths ?? []).join("\n")}
              onInput={(e) =>
                props.change("autoDeploy", {
                  ...value(),
                  paths: e.currentTarget.value
                    .split("\n")
                    .map((p) => p.trim())
                    .filter(Boolean),
                })
              }
            />
            <label class="check-field">
              <input
                type="checkbox"
                checked={value().supersede}
                onChange={(e) =>
                  props.change("autoDeploy", {
                    ...value(),
                    supersede: e.currentTarget.checked,
                  })
                }
              />
              Replace older queued deployments
            </label>
          </Show>
        </div>
      </details>
    </Show>
  );
}
export default function DeploymentTriggers(props: { applicationId: string }) {
  const auth = useAuth(),
    [token, setToken] = createSignal(""),
    [busy, setBusy] = createSignal(false),
    [error, setError] = createSignal("");
  async function rotate(revoke = false) {
    setBusy(true);
    setError("");
    try {
      const result = await post<{ token: string }>(
        `/applications/${props.applicationId}/deploy-token`,
        { revoke },
        auth.session()?.csrfToken,
      );
      setToken(result.token);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Disclosure label="Deploy token" icon="plug" class="deploy-token">
      <div class="app-settings-disclosure-body">
      <Show when={error()}>
        <ErrorNotice message={error()} />
      </Show>
      <div class="form-fields configuration-fields">
        <label for="deploy-endpoint">Endpoint</label>
        <input
          id="deploy-endpoint"
          class="input mono"
          readonly
          value={
            location.origin +
            `/api/v1/applications/${props.applicationId}/deploy`
          }
        />
        <Show when={token()}>
          <label for="deploy-token-value">Token</label>
          <input
            id="deploy-token-value"
            class="input mono"
            readonly
            value={token()}
            autocomplete="off"
          />
        </Show>
      </div>
      <div class="form-actions">
        <Button
          variant="ghost"
          disabled={busy()}
          onClick={() => void rotate(true)}
        >
          Revoke token
        </Button>
        <Button disabled={busy()} onClick={() => void rotate()}>
          Generate token
        </Button>
      </div>
      </div>
    </Disclosure>
  );
}
