import { Show, createSignal } from "solid-js";
import { post } from "../api";
import { useAuth } from "../context";
import type { StaticConfig } from "../types";
import { Button, ErrorNotice } from "./Controls";

// Pushes and the deploy token both start deployments without opening Shipper, so they share one section.
export function AutomaticDeployFields(props: {
  config: StaticConfig;
  change: (key: keyof StaticConfig, value: unknown) => void;
  applicationId?: string;
  push?: boolean;
}) {
  const value = () =>
    props.config.autoDeploy ?? { enabled: false, paths: [], supersede: true };
  const push = () => (props.push ?? true) && !!props.config.sourceId;
  return (
    <Show when={push() || props.applicationId}>
      <details class="configuration automatic-deployments">
        <summary>Automatic deployments</summary>
        <Show when={push()}>
          <div class="form-fields configuration-fields">
            <label class="check-field">
              <input
                type="checkbox"
                checked={value().enabled}
                onChange={(e) =>
                  props.change("autoDeploy", {
                    ...value(),
                    paths: [],
                    enabled: e.currentTarget.checked,
                  })
                }
              />
              Deploy on push
            </label>
            <Show when={value().enabled}>
              <label class="check-field">
                <input
                  type="checkbox"
                  checked={value().supersede}
                  onChange={(e) =>
                    props.change("autoDeploy", {
                      ...value(),
                      paths: [],
                      supersede: e.currentTarget.checked,
                    })
                  }
                />
                Replace older queued deployments
              </label>
            </Show>
          </div>
        </Show>
        <Show when={props.applicationId}>{id => <DeployToken applicationId={id()} />}</Show>
      </details>
    </Show>
  );
}

function DeployToken(props: { applicationId: string }) {
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
    <div class="deploy-token">
      <h4>Deploy token</h4>
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
  );
}
