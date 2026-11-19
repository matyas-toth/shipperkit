import { For, Show, createSignal, createUniqueId } from "solid-js";
import type { StaticConfig } from "../types";
import BuildMethod from "./BuildMethod";
import ComposeFields from "./ComposeFields";
import { BuildDetails, type ConfigFields } from "./DeployFields";
import { AutomaticDeployFields } from "./DeploymentTriggers";
import GitFields from "./GitFields";
import LocalSource from "./LocalSource";
import { ImageFields, InlineFields, useConnections } from "./SourceFields";
import type { Choice } from "./Choices";

type EditSource = "git" | "local" | "image" | "inline" | "paste";
const editSources: readonly Choice<EditSource>[] = [
  { value: "git", label: "Public Git repository", icon: "git" },
  { value: "image", label: "Container image", icon: "box" },
  { value: "local", label: "Local files", icon: "folder" },
  { value: "inline", label: "Inline Dockerfile", icon: "code" },
  { value: "paste", label: "Inline Docker Compose", icon: "layers" },
];

export const emptyConfig = (): StaticConfig => ({ sourceId:"",repositoryId:0,autoDeploy:{enabled:false,supersede:true,paths:[]},image:"",registryId:"",inlineDockerfile:"",repository: "", branch: "", publishDirectory: ".", routing: "files", htaccess: false, domain: "", https: false, method: "auto", rootDirectory: ".", dockerfile: "Dockerfile", installCommand: "", buildCommand: "", startCommand: "", port: 0, healthPath: "/", variables: [], volumes: [], memoryMB: 512, composeSource: "git", composeFile: "compose.yaml", composeYAML: "", services: [] });
export function normalizedConfig(config?: Partial<StaticConfig>): StaticConfig { const method = config?.method || "static";return { ...emptyConfig(), ...config, method, rootDirectory: config?.rootDirectory || ".", dockerfile: config?.dockerfile || "Dockerfile", healthPath: config?.healthPath || "/", publishDirectory: config?.publishDirectory || ".", routing: config?.routing || "files", memoryMB: config?.memoryMB || (method === "static" ? 128 : 512), variables: config?.variables ?? [], volumes: config?.volumes ?? [] }; }
export default function StaticFields(props: ConfigFields & { applicationId: string; expanded: boolean; hasSavedCompose: boolean }) {
  const sourceId = createUniqueId();
  const [source, setSource] = createSignal<EditSource>(props.config.method === "image" ? "image" : props.config.method === "inline" ? "inline" :
    props.config.localSource ? "local" : props.config.composeSource === "paste" ? "paste" : "git");
  const connections = useConnections();
  const gitConnections = () => connections.data?.items.filter(c => c.kind !== "registry") ?? [];
  const compose = () => props.config.method === "compose";
  // One list names every place the code can come from, connected Git accounts included.
  const sourceValue = () => source() === "git" && props.config.sourceId ? "connection:" + props.config.sourceId : source();
  function chooseSource(value: string) {
    if (!value.startsWith("connection:")) {
      if (value === "git" && source() === "git" && props.config.sourceId) {
        props.change("repository", ""); props.change("repositoryId", 0); props.change("branch", ""); props.change("sourceId", "");
        props.change("autoDeploy", { enabled: false, paths: [], supersede: true });
        return;
      }
      changeSource(value as EditSource);
      return;
    }
    const id = value.slice("connection:".length);
    if (source() !== "git") changeSource("git");
    if (props.config.sourceId === id) return;
    props.change("repository", ""); props.change("repositoryId", 0); props.change("branch", ""); props.change("sourceId", id);
  }
  const files = () => source() === "git" || source() === "local";
  function build(method: StaticConfig["method"]) {
    if (method === props.config.method) return;
    props.change("method", method);
    props.change("port", 0);
    props.change("memoryMB", method === "static" ? 128 : 512);
    if (["dockerfile", "image", "inline"].includes(method)) {
      props.change("installCommand", "");
      props.change("buildCommand", "");
    }
    if (method === "static") {
      props.change("startCommand", "");
      props.change("startupPolicy", "http");
    }
  }
  function changeSource(next: EditSource) {
    setSource(next);
    if (next !== "local") props.change("localSource", "");
    if (next !== "git") {
      props.change("repository", "");
      props.change("branch", "");
      props.change("sourceId", "");
      props.change("repositoryId", 0);
      props.change("autoDeploy", { enabled: false, paths: [], supersede: true });
    }
    if (compose()) props.change("composeSource", next === "paste" ? "paste" : "git");
    else if (next === "image" || next === "inline") build(next);
    else if (!["auto", "static", "dockerfile", "railpack"].includes(props.config.method)) build("auto");
  }
  return <>
    <div class="field source-type-field"><label for={sourceId}>Source</label>
      <select id={sourceId} class="input" value={sourceValue()} onChange={e => chooseSource(e.currentTarget.value)}>
        <For each={gitConnections()}>{c => <option value={"connection:" + c.id}>{c.name + " (GitHub)"}</option>}</For>
        <For each={editSources.filter(item => compose() ? ["git", "local", "paste"].includes(item.value) : item.value !== "paste")}>{item => <option value={item.value}>{item.label}</option>}</For>
      </select>
    </div>
    <Show when={source() === "git"}><GitFields config={props.config} change={props.change} withoutConnection /></Show>
    <Show when={source() === "local"}>
      <LocalSource value={props.config.localSource} applicationId={props.applicationId} change={id => props.change("localSource", id)} />
    </Show>
    <Show when={source() === "image"}><ImageFields config={props.config} change={props.change} /></Show>
    <Show when={source() === "inline"}><InlineFields config={props.config} change={props.change} applicationId={props.applicationId} /></Show>
    <Show when={compose()} fallback={<>
      <BuildDetails config={props.config} change={props.change} method={props.config.method} splitRuntime
        leading={files() ? <BuildMethod value={props.config.method} visibleLegend change={build} /> : undefined} />
    </>}>
      <ComposeFields config={props.config} change={props.change} applicationId={props.applicationId} expanded={props.expanded} hasSavedCompose={props.hasSavedCompose} />
    </Show>
    <AutomaticDeployFields config={props.config} change={props.change} applicationId={props.applicationId} push={source() === "git"} />
  </>;
}
