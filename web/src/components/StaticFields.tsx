import { Show, createSignal } from "solid-js";
import type { StaticConfig } from "../types";
import BuildMethod from "./BuildMethod";
import ComposeFields from "./ComposeFields";
import { BuildDetails, type ConfigFields } from "./DeployFields";
import { AutomaticDeployFields } from "./DeploymentTriggers";
import GitFields from "./GitFields";
import LocalSource from "./LocalSource";
import RuntimeFields from "./RuntimeFields";
import { ImageFields, InlineFields } from "./SourceFields";

export const emptyConfig = (): StaticConfig => ({ sourceId:"",repositoryId:0,autoDeploy:{enabled:false,supersede:true,paths:[]},image:"",registryId:"",inlineDockerfile:"",repository: "", branch: "", publishDirectory: ".", routing: "files", htaccess: false, domain: "", https: false, method: "auto", rootDirectory: ".", dockerfile: "Dockerfile", installCommand: "", buildCommand: "", startCommand: "", port: 0, healthPath: "/", variables: [], volumes: [], memoryMB: 512, composeSource: "git", composeFile: "compose.yaml", composeYAML: "", services: [] });
export function normalizedConfig(config?: Partial<StaticConfig>): StaticConfig { const method = config?.method || "static";return { ...emptyConfig(), ...config, method, rootDirectory: config?.rootDirectory || ".", dockerfile: config?.dockerfile || "Dockerfile", healthPath: config?.healthPath || "/", publishDirectory: config?.publishDirectory || ".", routing: config?.routing || "files", memoryMB: config?.memoryMB || (method === "static" ? 128 : 512), variables: config?.variables ?? [], volumes: config?.volumes ?? [] }; }
export default function StaticFields(props: ConfigFields & { applicationId: string }) {
  const [source, setSource] = createSignal(props.config.method === "image" ? "image" : props.config.method === "inline" ? "inline" :
    props.config.localSource ? "local" : props.config.composeSource === "paste" ? "paste" : "git");
  const compose = () => props.config.method === "compose";
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
  function changeSource(next: string) {
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
    <label for="settings-source">Source</label>
    <select id="settings-source" class="input" value={source()} onChange={e => changeSource(e.currentTarget.value)}>
      <option value="git">Git repository</option><option value="local">Local files</option>
      <Show when={compose()} fallback={<><option value="image">Container image</option><option value="inline">Inline Dockerfile</option></>}>
        <option value="paste">Paste Compose YAML</option>
      </Show>
    </select>
    <Show when={source() === "git"}><GitFields config={props.config} change={props.change} /></Show>
    <Show when={source() === "local"}>
      <LocalSource value={props.config.localSource} applicationId={props.applicationId} change={id => props.change("localSource", id)} />
    </Show>
    <Show when={source() === "image"}><ImageFields config={props.config} change={props.change} /></Show>
    <Show when={source() === "inline"}><InlineFields config={props.config} change={props.change} applicationId={props.applicationId} /></Show>
    <Show when={compose()} fallback={<>
      <Show when={files()}><BuildMethod value={props.config.method} change={build} /></Show>
      <BuildDetails config={props.config} change={props.change} method={props.config.method} open />
      <RuntimeFields value={props.config.runtime} change={runtime => props.change("runtime", runtime)} />
    </>}>
      <ComposeFields config={props.config} change={props.change} applicationId={props.applicationId} />
    </Show>
    <Show when={source() === "git"}><AutomaticDeployFields config={props.config} change={props.change} /></Show>
  </>;
}
