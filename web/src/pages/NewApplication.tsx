import { A, useNavigate } from "@solidjs/router";
import { createQuery } from "@tanstack/solid-query";
import { For, Show, batch, createEffect, createMemo, createSignal, onCleanup, onMount } from "solid-js";
import { createStore, reconcile } from "solid-js/store";
import { api, post } from "../api";
import type { Application, Deployment, ProxyConfig, Service, StaticConfig } from "../types";
import { useAuth } from "../context";
import { clearDraft, readDraft, safeConfigDraft, saveDraft, saveVolatileDraft } from "../drafts";
import { emptyVariable } from "../env";
import { Button, ErrorNotice, validateForm } from "../components/Controls";
import { Icon } from "../components/Icon";
import { ChoiceCards, type Choice } from "../components/Choices";
import BuildMethod, { methodLabel } from "../components/BuildMethod";
import { BuildDetails, DomainFields, PortField } from "../components/DeployFields";
import { LifecycleCommands } from "../components/Commands";
import { AutomaticDeployFields } from "../components/DeploymentTriggers";
import GitFields from "../components/GitFields";
import LocalSource from "../components/LocalSource";
import { ReviewFact, ReviewPanel } from "../components/ReviewPanel";
import RuntimeFields from "../components/RuntimeFields";
import ServiceSummary from "../components/ServiceSummary";
import { ImageFields, InlineFields } from "../components/SourceFields";
import { emptyConfig } from "../components/StaticFields";
import Variables from "../components/Variables";
import Volumes from "../components/Volumes";

type Source = "git" | "image" | "local" | "inline" | "compose";
type Step = "source" | "configure" | "review";
type Inspection = {
  method: "dockerfile" | "railpack" | "static";
  commit?: string;
  variables: string[];
  installCommand: string;
  buildCommand: string;
  startCommand: string;
};
type Preview = { services: Service[]; files: string[]; composeFile: string; commit?: string };
type CreatePlan = { applicationId: string; revision: string; name: string; config: StaticConfig; commit?: string };
type Draft = { name: string; source?: Source; composeInput?: "git" | "local" | "paste"; config: StaticConfig };
const sources: readonly Choice<Source>[] = [
  { value: "git", label: "Git repository", icon: "git" },
  { value: "image", label: "Container image", icon: "box" },
  { value: "local", label: "Local files", icon: "folder" },
  { value: "inline", label: "Inline Dockerfile", icon: "code" },
  { value: "compose", label: "Inline Docker Compose", icon: "layers" },
];
const steps = [{ value: "source", label: "Source" }, { value: "configure", label: "Configure" }, { value: "review", label: "Review" }] as const;

function draftSource(draft?: Draft): Source {
  const config = draft?.config;
  if (draft?.source === "compose" || config?.method === "compose") {
    if (draft?.source === "git" || draft?.source === "local") return draft.source;
    if (draft?.composeInput === "git" || draft?.composeInput === "local") return draft.composeInput;
    if (config?.composeSource === "paste") return "compose";
    return config?.localSource ? "local" : "git";
  }
  if (draft?.source) return draft.source;
  if (config?.method === "image" || config?.method === "inline") return config.method;
  return config?.localSource ? "local" : "git";
}

function persistedConfig(config: StaticConfig): StaticConfig {
  const safe = safeConfigDraft(config);
  // An unfinished URL or shell command may contain credentials even when it
  // never reached server validation. Keep these inputs only in memory.
  try {
    const url = new URL(safe.repository);
    if (url.username || url.password || url.search || url.hash) safe.repository = "";
  } catch { safe.repository = ""; }
  return { ...safe, image: safe.image?.includes("://") ? "" : safe.image,
    hooks: [], installCommand: "", buildCommand: "", startCommand: "" };
}

export default function NewApplication() {
  const auth = useAuth(), navigate = useNavigate();
  const key = "new:" + auth.session()?.user?.id;
  const draft = readDraft<Draft>(key);
  const [config, setConfig] = createStore<StaticConfig>({ ...emptyConfig(), ...draft?.config });
  const [source, setSource] = createSignal<Source>(draftSource(draft));
  const [name, setName] = createSignal(draft?.name ?? "");
  const [step, setStep] = createSignal<Step>("source");
  const [pending, setPending] = createSignal(false);
  const [error, setError] = createSignal("");
  const [inspection, setInspection] = createSignal<{ key: string; result: Inspection }>();
  const [inspecting, setInspecting] = createSignal(false);
  const [inspectionError, setInspectionError] = createSignal("");
  const [preview, setPreview] = createSignal<{ key: string; result: Preview }>();
  const [previewing, setPreviewing] = createSignal(false);
  const [review, setReview] = createSignal<{ plan: CreatePlan; inputs: StaticConfig; commit?: string; requestKey: string }>();
  const [envKey, setEnvKey] = createSignal(1);
  const [envOpen, setEnvOpen] = createSignal(false);
  const proxy = createQuery(() => ({ queryKey: ["proxy"], queryFn: () => api<ProxyConfig>("/proxy") }));
  const acme = () => !!proxy.data?.acmeEmail;
  const compose = () => config.method === "compose";
  const inlineCompose = () => source() === "compose";
  const git = () => source() === "git";
  const local = () => source() === "local";
  const fileBuild = () => git() || local();
  const sourceKey = createMemo(() => JSON.stringify([source(), config.repository,
    config.sourceId, config.repositoryId, config.branch, config.localSource, config.rootDirectory]));
  const composeKey = createMemo(() => JSON.stringify([sourceKey(), config.composeSource, config.composeFile, config.composeYAML, config.variables]));
  const detected = () => inspection()?.key === sourceKey() ? inspection()?.result : undefined;
  const stack = () => preview()?.key === composeKey() ? preview()?.result : undefined;
  const method = () => config.method === "auto" ? detected()?.method ?? "auto" : config.method;
  const worker = () => method() !== "static" && config.startupPolicy === "process";
  const suggestedName = () => {
    if (git()) return config.repository.replace(/\/$/, "").split("/").pop()?.replace(/\.git$/, "") || "";
    if (source() === "image") return config.image?.split("/").pop()?.split(/[:@]/)[0] || "";
    return compose() ? "Compose application" : local() ? "Local application" : "Dockerfile application";
  };
  const missing = () => (detected()?.variables ?? []).filter(v => !config.variables.some(row => row.name === v));
  const staleDomains = () => (config.domains ?? []).filter(d => !stack()?.services.some(s => s.name === d.service && s.port && !s.oneShot));

  let heading!: HTMLHeadingElement;
  let disposed = false;
  let inspectionRequest: { key: string; promise: Promise<void> } | undefined;
  const sourceDrafts = new Map<Source, StaticConfig>();
  onCleanup(() => { disposed = true; });
  onMount(() => { document.title = "New application | Shipper"; });
  createEffect(() => {
    const current = { name: name(), source: source(), config };
    saveVolatileDraft(key, current);
    saveDraft(key, { ...current, config: persistedConfig(config) });
  });
  createEffect(() => { sourceKey(); setInspectionError(""); });

  function change(k: keyof StaticConfig, value: unknown) { setConfig(k, value as never); }
  function go(next: Step) {
    setError("");
    setStep(next);
    queueMicrotask(() => {
      if (disposed) return;
      heading?.closest(".page-scroll")?.scrollTo(0, 0);
      heading?.focus({ preventScroll: true });
    });
  }
  function build(next: StaticConfig["method"]) {
    if (next === config.method) return;
    batch(() => {
      setConfig("method", next);
      if (next === "compose") {
        setConfig("composeSource", inlineCompose() ? "paste" : "git");
        setConfig("startupPolicy", "http");
      }
      if (next === "static") {
        setConfig("port", 0);
        setConfig("startupPolicy", "http");
        setConfig("memoryMB", 128);
        setConfig("startCommand", "");
      } else if (config.memoryMB === 128) setConfig("memoryMB", 512);
      if (["dockerfile", "image", "inline", "compose"].includes(next)) {
        setConfig("installCommand", "");
        setConfig("buildCommand", "");
      }
    });
  }
  function chooseSource(next: Source) {
    if (next === source()) return;
    batch(() => {
      sourceDrafts.set(source(), JSON.parse(JSON.stringify(config)));
      setSource(next);
      setConfig(reconcile(sourceDrafts.get(next) ?? { ...emptyConfig(),
        method: next === "git" || next === "local" ? "auto" : next,
        composeSource: next === "compose" ? "paste" : "git",
      }));
      setInspectionError("");
      setError("");
    });
  }
  function sourceReady() {
    if (git()) return !!config.repository.trim();
    if (local()) return !!config.localSource;
    if (inlineCompose()) return !!config.composeYAML?.trim();
    if (source() === "image") return !!config.image?.trim();
    return !!config.inlineDockerfile?.trim();
  }
  function snapshot(): StaticConfig {
    const values: StaticConfig = JSON.parse(JSON.stringify(config));
    if (method() === "static") {
      values.startupPolicy = "http"; values.port = 0; values.startCommand = "";
    }
    if (worker() || compose()) {
      values.domain = ""; values.https = false;
    }
    if (compose()) {
      values.composeSource = inlineCompose() ? "paste" : "git";
      values.port = 0; values.startCommand = ""; values.volumes = []; values.runtime = undefined;
    } else {
      values.domains = undefined;
      values.composeSource = "git"; values.composeYAML = ""; values.services = [];
    }
    if (!git()) {
      values.repository = ""; values.branch = ""; values.sourceId = "";
      values.repositoryId = 0; values.autoDeploy = emptyConfig().autoDeploy;
    }
    return values;
  }
  function inspect(): Promise<void> {
    if (!fileBuild() || !sourceReady() || detected()) return Promise.resolve();
    const current = sourceKey();
    if (inspectionRequest?.key === current) return inspectionRequest.promise;
    setInspecting(true); setInspectionError("");
    const promise = post<Inspection>("/repositories/inspect", { config: snapshot() }, auth.session()?.csrfToken)
      .then(result => {
        if (!disposed && current === sourceKey()) setInspection({ key: current, result });
      }).catch(e => {
        if (!disposed && current === sourceKey()) setInspectionError((e as Error).message);
      }).finally(() => {
        if (inspectionRequest?.key === current) {
          inspectionRequest = undefined;
          if (!disposed) setInspecting(false);
        }
      });
    inspectionRequest = { key: current, promise };
    return promise;
  }
  async function inspectStack() {
    if (stack()) return stack()!;
    const current = composeKey();
    const result = await post<Preview>("/compose/preview", { config: snapshot() }, auth.session()?.csrfToken);
    if (disposed || current !== composeKey()) throw Error("The Compose inputs changed; review them again");
    setConfig("composeFile", result.composeFile);
    setPreview({ key: composeKey(), result });
    return result;
  }
  async function previewStack() {
    if (previewing()) return;
    setPreviewing(true); setError("");
    try { await inspectStack(); } catch (e) { if (!disposed) setError((e as Error).message); }
    finally { if (!disposed) setPreviewing(false); }
  }
  async function plan() {
    if (pending() || previewing()) return;
    setPending(true); setError("");
    try {
      if (fileBuild() && !compose() && !inspectionError()) await inspect();
      const services = compose() ? await inspectStack() : undefined;
      if (compose() && staleDomains().length) throw Error("Remove domains for services without an HTTP port");
      const inputs = snapshot();
      const commit = git() ? (services?.commit || detected()?.commit) : undefined;
      const result = await post<CreatePlan>("/plan", {
        name: name().trim() || suggestedName(), config: inputs, commit,
      }, auth.session()?.csrfToken);
      if (!disposed) {
        const requestKey = Array.from(crypto.getRandomValues(new Uint8Array(16)), b => b.toString(16).padStart(2, "0")).join("");
        setReview({ plan: result, inputs, commit, requestKey });
        go("review");
      }
    } catch (e) { if (!disposed) setError((e as Error).message); }
    finally { if (!disposed) setPending(false); }
  }
  async function deploy() {
    const approved = review();
    if (!approved || pending()) return;
    setPending(true); setError("");
    try {
      const d = await post<Deployment>("/applications", {
        id: approved.plan.applicationId, revision: approved.plan.revision, name: approved.plan.name,
        config: approved.inputs, commit: approved.commit,
      }, auth.session()?.csrfToken, approved.requestKey);
      clearDraft(key);
      clearDraft("variables:" + auth.session()?.user?.id + ":new");
      navigate("/applications/" + d.applicationId + "?tab=deployments");
    } catch (e) { if (!disposed) setError((e as Error).message); }
    finally { if (!disposed) setPending(false); }
  }
  function addVariables() {
    setConfig("variables", [...config.variables, ...missing().map(v => ({ ...emptyVariable(), name: v }))].slice(0, 128));
    setEnvKey(v => v + 1);
    setEnvOpen(true);
  }
  function serviceDomain(service: string, hostname: string, https?: boolean) {
    const previous = config.domains?.find(d => d.service === service);
    const next = (config.domains ?? []).filter(d => d.service !== service);
    if (hostname.trim()) next.push({ id: previous?.id ?? "service-" + service, hostname, service,
      https: !hostname.toLowerCase().endsWith(".localhost") && (https ?? previous?.https ?? (!!proxy.data?.autoHTTPS && acme())), primary: true, redirect: false });
    setConfig("domains", next);
  }

  return <div class="content creation-page">
    <A href="/" class="back-link"><Icon name="back" />Applications</A>
    <header class="page-heading"><h1>New application</h1></header>
    <ol class="creation-steps" aria-label="Creation steps">
      <For each={steps}>{(entry, index) => <li>
        <button type="button" aria-current={step() === entry.value ? "step" : undefined}
          disabled={pending() || previewing() || steps.findIndex(s => s.value === step()) <= index()}
          onClick={() => go(entry.value)}>
          <span class="step-number">{index() + 1}</span>{entry.label}
        </button>
      </li>}</For>
    </ol>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <Show when={step() === "source"}>
      <form onSubmit={e => { e.preventDefault(); go("configure"); if (!compose()) void inspect(); }}>
        <h2 ref={heading} class="sr-only" tabindex="-1">Source</h2>
        <fieldset class="creation-fields" disabled={pending()}>
          <ChoiceCards<Source> label="Source type" value={source()} choices={sources} change={chooseSource} />
          <div class="form-fields creation-inputs">
            <Show when={git()}><GitFields config={config} change={change} /></Show>
            <Show when={local()}>
              <LocalSource value={config.localSource} change={id => setConfig("localSource", id)} />
            </Show>
            <Show when={inlineCompose()}>
              <label for="compose-yaml">Compose YAML</label>
              <textarea id="compose-yaml" class="input compose-editor mono" rows={8} spellcheck={false} required
                value={config.composeYAML} onInput={e => setConfig("composeYAML", e.currentTarget.value)} />
            </Show>
            <Show when={source() === "image"}><ImageFields config={config} change={change} /></Show>
            <Show when={source() === "inline"}><InlineFields config={config} change={change} /></Show>
            <label for="name">Name</label>
            <input class="input" id="name" maxlength={80} placeholder={suggestedName() || "Application name"}
              value={name()} onInput={e => setName(e.currentTarget.value)} />
          </div>
        </fieldset>
        <div class="form-actions creation-actions">
          <A href="/" class="button ghost">Cancel</A>
          <Show when={!compose()}>
            <Button variant="ghost" disabled={pending() || !sourceReady()} onClick={e => {
              if (e.currentTarget.form?.reportValidity()) void plan();
            }}>Review with defaults</Button>
          </Show>
          <Button variant="primary" type="submit" disabled={pending() || !sourceReady()}>Continue<Icon name="chevron" /></Button>
        </div>
      </form>
    </Show>
    <Show when={step() === "configure"}>
      <form noValidate onSubmit={e => { e.preventDefault(); if (validateForm(e.currentTarget)) void plan(); }}>
        <h2 ref={heading} class="sr-only" tabindex="-1">Configure</h2>
        <fieldset class="creation-fields creation-config" disabled={pending()}>
          <Show when={fileBuild()}>
            <section>
              <h2>Build method</h2>
              <BuildMethod value={config.method} detected={detected()?.method} allowCompose change={build} />
              <Show when={!compose() && inspecting()}><div class="inspection-status" role="status">Inspecting source</div></Show>
              <Show when={config.method === "auto" && inspectionError()}><ErrorNotice message={inspectionError()} retry={() => void inspect()} /></Show>
            </section>
          </Show>
          <Show when={compose()} fallback={<>
            <Show when={method() !== "static"}>
              <section>
                <h2>Runtime</h2>
                <ChoiceCards<"http" | "process"> label="Runtime role" value={config.startupPolicy || "http"} choices={[
                  { value: "http", label: "Web service", icon: "globe" },
                  { value: "process", label: "Worker", icon: "terminal" },
                ]} change={role => setConfig("startupPolicy", role)} />
              </section>
            </Show>
            <Show when={!worker()}>
              <div class="form-fields creation-inputs">
                <Show when={method() !== "static"}><PortField config={config} change={change} /></Show>
                <DomainFields config={config} change={change} acme={acme()} autoHTTPS={proxy.data?.autoHTTPS} />
              </div>
            </Show>
            <BuildDetails config={config} change={change} method={method()} hints={detected()} essentials />
            <RuntimeFields value={config.runtime} change={runtime => setConfig("runtime", runtime)} />
            <details class="configuration"><summary>Storage<Icon name="down" /></summary>
              <Volumes values={config.volumes} change={volumes => setConfig("volumes", volumes)} />
            </details>
          </>}>
            <section class="creation-services">
              <Show when={!inlineCompose()}>
                <div class="form-fields compose-file-fields">
                  <label for="compose-file">Compose file</label>
                  <input class="input mono" id="compose-file" placeholder="compose.yaml" value={config.composeFile} onInput={e => setConfig("composeFile", e.currentTarget.value)} />
                </div>
              </Show>
              <div class="section-toolbar"><h2>Services</h2>
                <Show when={!stack()}>
                  <Button icon="search" variant="ghost" disabled={previewing()} onClick={() => void previewStack()}>
                    {previewing() ? "Inspecting Compose" : "Preview services"}
                  </Button>
                </Show>
              </div>
              <Show when={stack()}>{result => <ServiceSummary services={result().services} routes={service =>
                <Show when={service.port && !service.oneShot}>
                  <div class="service-route">
                    <input class="input" aria-label={"Domain for " + service.name} placeholder="Add domain"
                      value={config.domains?.find(d => d.service === service.name)?.hostname ?? ""}
                      onInput={e => serviceDomain(service.name, e.currentTarget.value)} />
                    <Show when={config.domains?.some(d => d.service === service.name && !d.hostname.toLowerCase().endsWith(".localhost"))}>
                      <label class="check-field"><input type="checkbox" aria-label={"HTTPS for " + service.name}
                        disabled={!acme() || !!config.domains?.find(d => d.service === service.name)?.hostname.toLowerCase().endsWith(".localhost")}
                        checked={config.domains?.find(d => d.service === service.name)?.https ?? false}
                        onChange={e => serviceDomain(service.name, config.domains!.find(d => d.service === service.name)!.hostname, e.currentTarget.checked)} />HTTPS</label>
                    </Show>
                  </div>
                </Show>
              } />}</Show>
              <Show when={stack() && staleDomains().length}>
                <Button variant="ghost" icon="trash" onClick={() => setConfig("domains", rows => rows?.filter(d => !staleDomains().includes(d)))}>
                  Remove domains without a matching service
                </Button>
              </Show>
            </section>
            <Show when={!inlineCompose()}><details class="configuration"><summary>Project directory<Icon name="down" /></summary>
              <div class="form-fields configuration-fields"><label for="project-directory">Project directory</label>
                <input id="project-directory" class="input" value={config.rootDirectory} onInput={e => setConfig("rootDirectory", e.currentTarget.value)} />
              </div>
            </details></Show>
          </Show>
          <Show when={git()}><div class="form-fields"><AutomaticDeployFields config={config} change={change} /></div></Show>
          <details class="configuration" open={envOpen()} onToggle={e => setEnvOpen(e.currentTarget.open)}>
            <summary>.env<Icon name="down" /></summary>
            <Show when={missing().length}><Button variant="ghost" icon="plus" onClick={addVariables}>Add detected variables</Button></Show>
            <Show when={envKey()} keyed>{(_key) => <Variables application={{ id: "new", config } as Application} busy={pending()}
              embedded change={variables => setConfig("variables", variables)} />}</Show>
          </details>
          <details class="configuration"><summary>Deployment commands<Icon name="down" /></summary>
            <LifecycleCommands compact config={config} services={stack()?.services.map(s => s.name)} change={hooks => setConfig("hooks", hooks)} />
          </details>
        </fieldset>
        <div class="form-actions creation-actions">
          <Button variant="ghost" icon="back" disabled={pending() || previewing()} onClick={() => go("source")}>Back</Button>
          <Button variant="primary" type="submit" disabled={pending() || previewing()}>{pending() ? "Preparing review" : "Review deployment"}</Button>
        </div>
      </form>
    </Show>
    <Show when={step() === "review" && review()}>{approved => <>
      <h2 ref={heading} tabindex="-1">Review deployment</h2>
      <div class="creation-review">
        <ReviewPanel title="Source" edit={pending() ? undefined : () => go("source")}>
          <dl>
            <ReviewFact label="Application">{approved().plan.name}</ReviewFact>
            <ReviewFact label={git() ? "Repository" : source() === "image" ? "Image" : "Source"}>
              <span class="mono">{git() ? approved().plan.config.repository : local() ? "Snapshot " + config.localSource : source() === "image" ? config.image : compose() ? "Pasted Compose YAML" : "Inline Dockerfile"}</span>
            </ReviewFact>
            <Show when={git()}><ReviewFact label="Revision">
              {approved().plan.config.branch || "Default branch"}<Show when={approved().commit}><code class="mono" title={approved().commit}>{approved().commit!.slice(0, 12)}</code></Show>
            </ReviewFact></Show>
            <Show when={(git() || local()) && approved().plan.config.rootDirectory !== "."}>
              <ReviewFact label="Directory"><code>{approved().plan.config.rootDirectory}</code></ReviewFact>
            </Show>
            <Show when={compose() && !inlineCompose()}>
              <ReviewFact label="Compose file"><code>{approved().plan.config.composeFile}</code></ReviewFact>
            </Show>
          </dl>
        </ReviewPanel>
        <ReviewPanel title="Configuration" edit={pending() ? undefined : () => go("configure")}>
          <dl>
            <ReviewFact label="Method">{methodLabel(method())}{config.method === "auto" && detected() ? " (automatic)" : ""}</ReviewFact>
            <Show when={method() === "static" && approved().plan.config.publishDirectory !== "."}>
              <ReviewFact label="Publish"><code>{approved().plan.config.publishDirectory}</code></ReviewFact>
            </Show>
            <Show when={!compose()}>
              <ReviewFact label="Runtime">{method() === "static" ? "Static site" : worker() ? "Worker" : "Web service"}</ReviewFact>
              <Show when={!worker()}>
                <ReviewFact label="Port">{method() === "static" ? 8080 : config.port || "Automatic"}</ReviewFact>
                <ReviewFact label="Domain">{approved().plan.config.domain ? (approved().plan.config.https ? "https://" : "http://") + approved().plan.config.domain : "None"}</ReviewFact>
              </Show>
            </Show>
            <Show when={compose()}><ReviewFact label="Public routes">
              <Show when={approved().plan.config.domains?.length} fallback="None"><For each={approved().plan.config.domains}>{d => <span>{d.service}: {d.https ? "https://" : "http://"}{d.hostname}</span>}</For></Show>
            </ReviewFact></Show>
            <ReviewFact label="Storage">
              <Show when={config.volumes.length || stack()?.services.some(s => s.volumes?.length)} fallback="None">
                <For each={config.volumes}>{v => <span>{v.name}: <code>{v.path}</code></span>}</For>
                <For each={stack()?.services}>{s => <For each={s.volumes}>{v => <span>{s.name}: {v.name} <code>{v.path}</code></span>}</For>}</For>
              </Show>
            </ReviewFact>
            <ReviewFact label=".env">{config.variables.length ? config.variables.length + (config.variables.length === 1 ? " variable" : " variables") : "None"}</ReviewFact>
            <ReviewFact label="Commands">
              <Show when={config.installCommand || config.buildCommand || approved().inputs.startCommand || config.hooks?.length} fallback="Source defaults">
                <Show when={config.installCommand}><span>Install <code>{config.installCommand}</code></span></Show>
                <Show when={config.buildCommand}><span>Build <code>{config.buildCommand}</code></span></Show>
                <Show when={approved().inputs.startCommand}><span>Start <code>{approved().inputs.startCommand}</code></span></Show>
                <For each={config.hooks}>{hook => <span>{hook.stage.replaceAll("_", " ")}{hook.command.service ? " · " + hook.command.service : ""} <code>{hook.command.shell || hook.command.argv?.join(" ")}</code></span>}</For>
              </Show>
            </ReviewFact>
          </dl>
        </ReviewPanel>
        <Show when={stack()}>{result => <section class="creation-services"><h2>Services</h2><ServiceSummary services={result().services} /></section>}</Show>
      </div>
      <div class="form-actions creation-actions">
        <Button variant="ghost" icon="back" disabled={pending()} onClick={() => go("configure")}>Back</Button>
        <Button variant="primary" icon="deploy" disabled={pending()} onClick={() => void deploy()}>{pending() ? "Creating application" : "Deploy"}</Button>
      </div>
    </>}</Show>
  </div>;
}
