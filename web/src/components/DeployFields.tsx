import { Show, createSignal } from "solid-js";
import type { StaticConfig } from "../types";
import { post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice } from "./Controls";
import { Icon } from "./Icon";

export type ConfigFields = {
  config: StaticConfig;
  change: (key: keyof StaticConfig, value: unknown) => void;
};

export function DomainFields(props: ConfigFields & { acme: boolean; autoHTTPS?: boolean }) {
  const [override, setOverride] = createSignal<boolean>();
  return <>
    <label for="domain">Domain</label>
    <input class="input" id="domain" placeholder={props.autoHTTPS ? "Optional" : "Automatic .localhost address"}
      value={props.config.domain} onInput={e => {
        const value = e.currentTarget.value;
        props.change("domain", value);
        if (!value || value.toLowerCase().endsWith(".localhost")) props.change("https", false);
        else if (props.autoHTTPS && props.acme) props.change("https", override() ?? true);
      }} />
    <Show when={props.config.domain && !props.config.domain.toLowerCase().endsWith(".localhost")}>
      <label class="check-field"><input type="checkbox" checked={props.config.https} disabled={!props.acme}
        onChange={e => { setOverride(e.currentTarget.checked); props.change("https", e.currentTarget.checked); }} />HTTPS</label>
    </Show>
  </>;
}

export function PortField(props: ConfigFields) {
  return <>
    <label for="container-port">Container port</label>
    <input class="input" id="container-port" type="number" min="1" max="65535" placeholder="Automatic"
      value={props.config.port || ""} onInput={e => props.change("port", Number(e.currentTarget.value) || 0)} />
  </>;
}

export function BuildDetails(props: ConfigFields & {
  method: StaticConfig["method"];
  open?: boolean;
  essentials?: boolean;
  hints?: { installCommand: string; buildCommand: string; startCommand: string };
}) {
  const auth = useAuth(), [loading, setLoading] = createSignal(false), [error, setError] = createSignal("");
  const fileBased = () => !["image", "inline"].includes(props.method);
  const commands = () => ["auto", "static", "railpack"].includes(props.method);
  const sourceKey = () => JSON.stringify([props.config.repository, props.config.sourceId, props.config.branch, props.config.localSource, props.config.rootDirectory, props.method]);
  async function detectCommands() {
    if (loading()) return;
    const key = sourceKey();
    setLoading(true);
    setError("");
    try {
      const result = props.hints ?? await post<NonNullable<typeof props.hints>>("/repositories/inspect", {config: props.config}, auth.session()?.csrfToken);
      if (sourceKey() !== key) return;
      for (const name of ["installCommand", "buildCommand", "startCommand"] as const) {
        if (!props.config[name] && result[name] && (name !== "startCommand" || props.method !== "static")) props.change(name, result[name]);
      }
    } catch (e) { if (sourceKey() === key) setError((e as Error).message); }
    finally { setLoading(false); }
  }
  return <details class="configuration" open={props.open}>
    <summary>Configuration<Icon name="down" /></summary>
    <div class="form-fields configuration-fields">
      <Show when={fileBased()}>
        <label for="root-directory">Application directory</label>
        <input class="input" id="root-directory" value={props.config.rootDirectory}
          onInput={e => props.change("rootDirectory", e.currentTarget.value)} />
      </Show>
      <Show when={props.method === "dockerfile"}>
        <label for="dockerfile">Dockerfile path</label>
        <input class="input" id="dockerfile" value={props.config.dockerfile}
          onInput={e => props.change("dockerfile", e.currentTarget.value)} />
      </Show>
      <Show when={props.method === "static"}>
        <label for="publish-directory">Publish directory</label>
        <input class="input" id="publish-directory" required value={props.config.publishDirectory}
          onInput={e => props.change("publishDirectory", e.currentTarget.value)} />
        <label for="routing">Routing</label>
        <select class="input" id="routing" value={props.config.routing} onChange={e => props.change("routing", e.currentTarget.value)}>
          <option value="files">File based</option><option value="spa">SPA fallback</option><option value="strict">Strict</option>
        </select>
        <label class="check-field"><input type="checkbox" checked={props.config.htaccess}
          onChange={e => props.change("htaccess", e.currentTarget.checked)} />.htaccess rewrites</label>
      </Show>
      <Show when={commands()}>
        <div class="detected-commands"><Button variant="ghost" icon="refresh"
          disabled={loading() || !props.config.repository && !props.config.localSource}
          onClick={() => void detectCommands()}>{loading() ? "Detecting" : props.hints ? "Use detected commands" : "Detect commands"}</Button></div>
        <Show when={error()}><ErrorNotice message={error()} /></Show>
        <label for="install-command">Install command</label>
        <input class="input mono" id="install-command" placeholder={props.method === "static" ? "None" : "Automatic"}
          value={props.config.installCommand} onInput={e => props.change("installCommand", e.currentTarget.value)} />
        <label for="build-command">Build command</label>
        <input class="input mono" id="build-command" placeholder={props.method === "static" ? "None" : "Automatic"}
          value={props.config.buildCommand} onInput={e => props.change("buildCommand", e.currentTarget.value)} />
      </Show>
      <Show when={props.method !== "static"}>
        <label for="start-command">Start command</label>
        <input class="input mono" id="start-command" placeholder="Automatic"
          value={props.config.startCommand} onInput={e => props.change("startCommand", e.currentTarget.value)} />
        <Show when={!props.essentials}>
          <Show when={props.config.startupPolicy !== "process"}><PortField config={props.config} change={props.change} /></Show>
          <label for="startup-policy">Readiness</label>
          <select id="startup-policy" class="input" value={props.config.startupPolicy || "http"}
            onChange={e => props.change("startupPolicy", e.currentTarget.value)}>
            <option value="http">HTTP</option><option value="process">Running process</option>
          </select>
        </Show>
      </Show>
      <Show when={props.config.startupPolicy !== "process"}>
        <label for="health-path">Health check path</label>
        <input class="input" id="health-path" value={props.config.healthPath}
          onInput={e => props.change("healthPath", e.currentTarget.value)} />
      </Show>
      <label for="memory-limit">Memory limit (MiB)</label>
      <input class="input" id="memory-limit" type="number" min="64" max="8192" value={props.config.memoryMB}
        onInput={e => props.change("memoryMB", Number(e.currentTarget.value))} />
    </div>
  </details>;
}
