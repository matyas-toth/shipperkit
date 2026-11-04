import { Show, createSignal, createEffect, on, createMemo } from "solid-js";
import type { Domain } from "../types";
import { Icon } from "./Icon";

export default function DomainOptions(props: {
  domain: Domain;
  stored?: Domain;
  change: (value: Partial<Domain>) => void;
}) {
  const [headers, setHeaders] = createSignal(
    Object.entries(props.domain.responseHeaders ?? {})
      .map(([key, value]) => key + ": " + value)
      .join("\n"),
  );
  const domainID = createMemo(() => props.domain.id);
  createEffect(
    on(
      domainID,
      () =>
        setHeaders(
          Object.entries(props.domain.responseHeaders ?? {})
            .map(([key, value]) => key + ": " + value)
            .join("\n"),
        ),
      { defer: true },
    ),
  );
  function parseHeaders(input: HTMLTextAreaElement) {
    const next: Record<string, string> = {};
    for (const line of input.value.split("\n")) {
      if (!line.trim()) continue;
      const colon = line.indexOf(":");
      if (colon < 1) {
        input.setCustomValidity("Enter each header as Name: value");
        return;
      }
      next[line.slice(0, colon).trim()] = line.slice(colon + 1).trim();
    }
    input.setCustomValidity("");
    props.change({ responseHeaders: next });
  }
  return (
    <details class="domain-route-options">
      <summary aria-label={"Routing options for " + props.domain.hostname}>
        Routing options
        <Icon name="down" />
      </summary>
      <div class="form-fields configuration-fields">
        <label for={"route-path-" + props.domain.id}>Path prefix</label>
        <input
          id={"route-path-" + props.domain.id}
          class="input mono"
          value={props.domain.pathPrefix ?? ""}
          placeholder="/"
          onInput={(e) =>
            props.change({
              pathPrefix: e.currentTarget.value,
              stripPrefix:
                e.currentTarget.value && e.currentTarget.value !== "/"
                  ? props.domain.stripPrefix
                  : false,
            })
          }
        />
        <label class="check-field">
          <input
            type="checkbox"
            checked={props.domain.stripPrefix ?? false}
            disabled={
              !props.domain.pathPrefix || props.domain.pathPrefix === "/"
            }
            onChange={(e) =>
              props.change({ stripPrefix: e.currentTarget.checked })
            }
          />
          Strip prefix
        </label>
        <label class="check-field">
          <input
            type="checkbox"
            checked={props.domain.compress ?? false}
            onChange={(e) =>
              props.change({ compress: e.currentTarget.checked })
            }
          />
          Compression
        </label>
        <label class="check-field">
          <input
            type="checkbox"
            checked={props.domain.maintenance ?? false}
            onChange={(e) =>
              props.change({ maintenance: e.currentTarget.checked })
            }
          />
          Maintenance mode
        </label>
        <label for={"route-headers-" + props.domain.id}>Response headers</label>
        <textarea
          id={"route-headers-" + props.domain.id}
          class="input mono command-input"
          value={headers()}
          placeholder="Cache-Control: no-store"
          onInput={(e) => {
            setHeaders(e.currentTarget.value);
            parseHeaders(e.currentTarget);
          }}
        />
        <Show when={props.stored?.certificateExpiresAt}>
          <dl class="settings-group">
            <div class="setting-row">
              <dt>Certificate issuer</dt>
              <dd>{props.stored?.certificateIssuer}</dd>
            </div>
            <div class="setting-row">
              <dt>Certificate expires</dt>
              <dd>
                {new Date(
                  props.stored!.certificateExpiresAt! * 1000,
                ).toLocaleString()}
              </dd>
            </div>
          </dl>
        </Show>
      </div>
    </details>
  );
}
