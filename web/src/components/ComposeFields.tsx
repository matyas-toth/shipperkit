import { For, Show, createMemo, createSignal } from "solid-js";
import type { Service, StaticConfig } from "../types";
import { post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice } from "./Controls";
import { Icon } from "./Icon";

export default function ComposeFields(props: { config: StaticConfig; change: (key: keyof StaticConfig, value: unknown) => void; applicationId?: string }) {
  const auth = useAuth(), [loading, setLoading] = createSignal(false), [error, setError] = createSignal(""), [services, setServices] = createSignal<Service[]>([]), [files, setFiles] = createSignal<string[]>([]), [detected, setDetected] = createSignal("");
  const fingerprint = createMemo(() => JSON.stringify([props.config.localSource, props.config.repository, props.config.branch, props.config.rootDirectory, props.config.composeSource, props.config.composeFile, props.config.composeYAML, props.config.variables]));
  const current = () => detected() === fingerprint();
  async function inspect() {
    if (loading()) return;setLoading(true);setError("");const start = fingerprint();
    try { const result = await post<{ services: Service[]; files: string[]; composeFile: string }>("/compose/preview", { config: props.config, applicationId: props.applicationId }, auth.session()?.csrfToken);if (start !== fingerprint()) return;props.change("composeFile", result.composeFile);setFiles(result.files);setServices(result.services);setDetected(fingerprint()); }
    catch (e) { setError((e as Error).message); }finally { setLoading(false); }
  }
  async function edit() {
    setLoading(true);setError("");try { const value = await post<{ content: string }>(`/applications/${props.applicationId}/compose/source`, {}, auth.session()?.csrfToken);props.change("composeYAML", value.content);props.change("composeKeep", false); }
    catch (e) { setError((e as Error).message); }finally { setLoading(false); }
  }
  return <>
    <Show when={props.config.composeSource === "paste"} fallback={<><label for="compose-file">Compose file</label><input class="input mono" id="compose-file" list="compose-files" value={props.config.composeFile || "compose.yaml"} onInput={e => props.change("composeFile", e.currentTarget.value)} /><datalist id="compose-files"><For each={files()}>{file => <option value={file} />}</For></datalist></>}>
      <label for="compose-yaml">Compose YAML</label><Show when={!props.config.composeKeep} fallback={<Button variant="ghost" disabled={loading()} onClick={() => void edit()}>Edit Compose</Button>}><textarea id="compose-yaml" class="input compose-editor mono" rows={12} spellcheck={false} required value={props.config.composeYAML || ""} onInput={e => props.change("composeYAML", e.currentTarget.value)} /></Show>
    </Show>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <div class="compose-detect"><Button icon={loading() ? "refresh" : "search"} variant="ghost" disabled={loading() || (props.config.composeSource !== "paste" && !props.config.repository&&!props.config.localSource) || (props.config.composeSource === "paste" && !props.config.composeYAML && !props.config.composeKeep)} onClick={() => void inspect()}>{loading() ? "Detecting services" : "Detect services"}</Button></div>
    <Show when={current() && services().length}><div class="table-tray compose-preview"><table class="inventory-table"><thead><tr><th>Service</th><th>Image</th><th>HTTP port</th></tr></thead><tbody><For each={services()}>{service => <tr><td><span class="identity"><Icon name="box" />{service.name}</span></td><td class="image-name">{service.image || "Build from source"}</td><td class="mono">{service.port || "Internal"}</td></tr>}</For></tbody></table></div></Show>
    <Show when={props.config.composeSource !== "paste"}><details class="configuration"><summary>Configuration<Icon name="down" /></summary><div class="form-fields configuration-fields"><label for="root-directory">Project directory</label><input class="input" id="root-directory" value={props.config.rootDirectory || "."} onInput={e => props.change("rootDirectory", e.currentTarget.value)} /></div></details></Show>
  </>;
}
