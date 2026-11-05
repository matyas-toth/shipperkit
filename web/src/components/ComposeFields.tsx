import { For, Show, batch, createMemo, createResource, createSignal } from "solid-js";
import type { Service, StaticConfig } from "../types";
import { post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice } from "./Controls";
import { Icon } from "./Icon";

export default function ComposeFields(props: {
  config: StaticConfig;
  change: (key: keyof StaticConfig, value: unknown) => void;
  applicationId: string;
  expanded: boolean;
  hasSavedCompose: boolean;
}) {
  const auth = useAuth(), [loading, setLoading] = createSignal(false), [error, setError] = createSignal(""), [services, setServices] = createSignal<Service[]>([]), [files, setFiles] = createSignal<string[]>([]), [detected, setDetected] = createSignal("");
  const [saved, { refetch }] = createResource(
    () => props.expanded && props.hasSavedCompose && props.config.composeSource === "paste" && props.applicationId,
    async (id): Promise<{ content: string; error?: never } | { error: string; content?: never }> => {
      try {
        return await post<{ content: string }>(`/applications/${id}/compose/source`, {}, auth.session()?.csrfToken);
      } catch (e) {
        return { error: (e as Error).message };
      }
    },
  );
  const needsSaved = () => !!props.config.composeKeep && props.hasSavedCompose;
  const editorValue = () => needsSaved() ? saved.latest?.content ?? "" : props.config.composeYAML ?? "";
  const editorDisabled = () => needsSaved() && saved.latest?.content === undefined;
  function edit(content: string) {
    const keep = props.hasSavedCompose && content === saved.latest?.content;
    batch(() => {
      props.change("composeYAML", keep ? "" : content);
      props.change("composeKeep", keep);
    });
  }
  const fingerprint = createMemo(() => JSON.stringify([props.config.localSource, props.config.repository, props.config.branch, props.config.rootDirectory, props.config.composeSource, props.config.composeFile, props.config.composeYAML, props.config.variables]));
  const current = () => detected() === fingerprint();
  async function inspect() {
    if (loading()) return;setLoading(true);setError("");const start = fingerprint();
    try { const result = await post<{ services: Service[]; files: string[]; composeFile: string }>("/compose/preview", { config: props.config, applicationId: props.applicationId }, auth.session()?.csrfToken);if (start !== fingerprint()) return;props.change("composeFile", result.composeFile);setFiles(result.files);setServices(result.services);setDetected(fingerprint()); }
    catch (e) { setError((e as Error).message); }finally { setLoading(false); }
  }
  return <>
    <Show when={props.config.composeSource === "paste"} fallback={<><label for="compose-file">Compose file</label><input class="input mono" id="compose-file" list="compose-files" value={props.config.composeFile || "compose.yaml"} onInput={e => props.change("composeFile", e.currentTarget.value)} /><datalist id="compose-files"><For each={files()}>{file => <option value={file} />}</For></datalist></>}>
      <label for="compose-yaml">Compose YAML</label>
      <textarea id="compose-yaml" class="input compose-editor mono" rows={12} spellcheck={false} required
        aria-busy={saved.loading} disabled={editorDisabled()} placeholder={saved.loading ? "Loading Compose YAML" : undefined}
        value={editorValue()} onInput={e => edit(e.currentTarget.value)} />
      <Show when={needsSaved() && !saved.loading && saved.latest?.error}>{message => <ErrorNotice message={message()} retry={() => void refetch()} />}</Show>
    </Show>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <div class="compose-detect"><Button icon={loading() ? "refresh" : "search"} disabled={loading() || (props.config.composeSource !== "paste" && !props.config.repository && !props.config.localSource) || (props.config.composeSource === "paste" && (!editorValue().trim() || editorDisabled()))} onClick={() => void inspect()}>{loading() ? "Detecting services" : "Detect services"}</Button></div>
    <Show when={current() && services().length}><div class="table-tray compose-preview"><table class="inventory-table"><thead><tr><th>Service</th><th>Image</th><th>HTTP port</th></tr></thead><tbody><For each={services()}>{service => <tr><td><span class="identity"><Icon name="box" />{service.name}</span></td><td class="image-name">{service.image || "Build from source"}</td><td class="mono">{service.port || "Internal"}</td></tr>}</For></tbody></table></div></Show>
    <Show when={props.config.composeSource !== "paste"}><details class="configuration"><summary>Configuration<Icon name="down" /></summary><div class="form-fields configuration-fields"><label for="root-directory">Project directory</label><input class="input" id="root-directory" value={props.config.rootDirectory || "."} onInput={e => props.change("rootDirectory", e.currentTarget.value)} /></div></details></Show>
  </>;
}
