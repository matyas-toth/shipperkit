import { createStore } from "solid-js/store";
import { For, Show, createEffect, createMemo, createSignal, createUniqueId, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import type { Application, StaticConfig } from "../types";
import DeploymentTriggers from "./DeploymentTriggers";
import { LifecycleCommands } from "./Commands";
import StaticFields, { normalizedConfig } from "./StaticFields";
import { Button, Disclosure, ErrorNotice, IconButton, validateForm } from "./Controls";
import { clearDraft, readDraft, safeConfigDraft, saveDraft, saveVolatileDraft } from "../drafts";
import { useAuth } from "../context";
import Volumes from "./Volumes";
import RuntimeFields from "./RuntimeFields";
import { MemoryLimitField, RuntimeConnectivity } from "./DeployFields";
import { Icon } from "./Icon";
import EditorActions from "./EditorActions";
import { createdDate, deploymentLabel, methodLabel, primaryDomain } from "../data";

const areas = [
  { id: "general", label: "General", icon: "settings" },
  { id: "source", label: "Source and build", icon: "layers" },
  { id: "runtime", label: "Runtime", icon: "terminal" },
  { id: "storage", label: "Storage", icon: "database" },
  { id: "commands", label: "Deployment commands", icon: "code" },
] as const;
type SettingsArea = (typeof areas)[number]["id"];

function configFingerprint(config: StaticConfig) {
  return JSON.stringify(safeConfigDraft(config), (_, value) =>
    value && typeof value === "object" && !Array.isArray(value)
      ? Object.fromEntries(Object.keys(value).sort().map(key => [key, value[key]]))
      : value);
}

export default function ApplicationSettings(props: {
  application: Application;
  showSource?: boolean;
  busy: boolean;
  save: (config: StaticConfig) => Promise<boolean | void>;
  rename: (name: string) => Promise<void>;
  remove: () => Promise<void>;
}) {
  const auth = useAuth(), formId = createUniqueId();
  const key = "settings:" + auth.session()?.user?.id + ":" + props.application.id;
  const [config, setConfig] = createStore(normalizedConfig(readDraft<StaticConfig>(key) ?? props.application.config));
  const [name, setName] = createSignal(props.application.name);
  const [error, setError] = createSignal("");
  const [renameError, setRenameError] = createSignal("");
  const [renaming, setRenaming] = createSignal(false);
  const [reviewing, setReviewing] = createSignal(false);
  const [confirm, setConfirm] = createSignal(false);
  const inlineCompose = () => config.method === "compose" && config.composeSource === "paste";
  const [area, setArea] = createSignal<SettingsArea>(props.showSource || inlineCompose() ? "source" : "general");
  const visibleAreas = () => areas.filter(item => config.method !== "compose" || !["runtime", "storage"].includes(item.id));
  const hasSavedCompose = () => props.application.config.composeSource === "paste" &&
    !!(props.application.config.composeKeep || props.application.config.composeHasValue);
  const composeEdited = () => inlineCompose() && (config.composeKeep === false && hasSavedCompose() || !!config.composeYAML);
  const savedFingerprint = createMemo(() => configFingerprint(normalizedConfig(props.application.config)));
  const dirty = createMemo(() => composeEdited() || configFingerprint(config) !== savedFingerprint());
  createEffect(() => {
    if (!dirty()) {
      clearDraft(key);
      return;
    }
    const safe = safeConfigDraft(config);
    saveDraft(key, safe);
    // Inline source can contain credentials. Keep edited YAML in memory only.
    saveVolatileDraft(key, composeEdited() ? { ...safe, composeYAML: config.composeYAML, composeKeep: config.composeKeep } : safe);
  });
  createEffect(() => { if (props.showSource) setArea("source"); });
  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (props.busy || reviewing() || !dirty()) return;
    const form = e.currentTarget as HTMLFormElement;
    const invalidArea = form.querySelector(":invalid")?.closest<HTMLElement>("[data-settings-section]")?.dataset.settingsSection as SettingsArea | undefined;
    if (invalidArea) setArea(invalidArea);
    if (!validateForm(form)) return;
    setError("");
    setReviewing(true);
    try {
      const draft = { ...config, variables: props.application.config.variables, services: props.application.config.services };
      if (await props.save(draft) !== false) clearDraft(key);
    } catch (e) { setError((e as Error).message); }
    finally { setReviewing(false); }
  }
  async function rename(e: SubmitEvent) {
    e.preventDefault();
    if (renaming() || name().trim() === props.application.name) return;
    setRenaming(true);
    setRenameError("");
    try { await props.rename(name().trim()); }
    catch (e) { setRenameError((e as Error).message); }
    finally { setRenaming(false); }
  }
  return <div class="application-settings">
    <div class="app-settings-workspace">
      <nav class="app-settings-navigation" aria-label="Application settings sections">
        <For each={visibleAreas()}>{item => <button type="button" aria-current={area() === item.id ? "page" : undefined} onClick={() => setArea(item.id)}>
          <Icon name={item.icon} />{item.label}
        </button>}</For>
      </nav>
      <select class="input app-settings-select" aria-label="Application settings section" value={area()} onChange={e => setArea(e.currentTarget.value as SettingsArea)}>
        <For each={visibleAreas()}>{item => <option value={item.id}>{item.label}</option>}</For>
      </select>
      <div class="app-settings-main">
        <section class="app-settings-pane" hidden={area() !== "general"}>
          <h3>General</h3>
          <form onSubmit={rename} class="rename-form">
            <label for="application-name">Name</label>
            <div>
              <input id="application-name" class="input" value={name()} required maxlength={80} onInput={e => setName(e.currentTarget.value)} />
              <Button type="submit" disabled={renaming() || !name().trim() || name().trim() === props.application.name}>Save name</Button>
            </div>
          </form>
          <Show when={renameError()}><ErrorNotice message={renameError()} /></Show>
          <dl class="settings-group app-settings-facts">
            <div class="setting-row"><dt>Domain</dt><dd>{primaryDomain(props.application)?.hostname || "None"}</dd></div>
            <div class="setting-row"><dt>Method</dt><dd>{methodLabel(props.application)}</dd></div>
            <div class="setting-row"><dt>Created</dt><dd>{createdDate(props.application.createdAt)}</dd></div>
            <div class="setting-row"><dt>Last release</dt><dd>{props.application.latestDeployment ? deploymentLabel(props.application.latestDeployment) : "None"}</dd></div>
          </dl>
          <div class="app-settings-secondary">
            <DeploymentTriggers applicationId={props.application.id} />
            <Disclosure label="Danger zone" icon="warning" class="delete-section">
              <div class="app-settings-disclosure-body">
                <Button icon="trash" class="danger-button" disabled={props.busy} onClick={() => setConfirm(true)}>Delete application</Button>
              </div>
            </Disclosure>
          </div>
        </section>
        <form id={formId} noValidate onSubmit={save} class="app-config-form" classList={{ "is-general": area() === "general" && !dirty() }}>
          <section class="app-settings-pane" data-settings-section="source" hidden={area() !== "source"}>
            <h3>Source and build</h3>
            <div class="form-fields app-settings-fields">
              <StaticFields config={config} change={(key, value) => setConfig(key, value as never)} applicationId={props.application.id}
                expanded={area() === "source"} hasSavedCompose={hasSavedCompose()} />
            </div>
          </section>
          <Show when={config.method !== "compose"}>
            <section class="app-settings-pane" data-settings-section="runtime" hidden={area() !== "runtime"}>
              <h3>Runtime</h3>
              <RuntimeFields value={config.runtime} change={runtime => setConfig("runtime", runtime)} expanded
                connectivity={<RuntimeConnectivity config={config} change={(key, value) => setConfig(key, value as never)} method={config.method} withoutMemory />}
                memory={<MemoryLimitField config={config} change={(key, value) => setConfig(key, value as never)} />} />
            </section>
            <section class="app-settings-pane" data-settings-section="storage" hidden={area() !== "storage"}>
              <h3>Storage</h3>
              <Volumes values={config.volumes} change={volumes => setConfig("volumes", volumes)} />
            </section>
          </Show>
          <section class="app-settings-pane" data-settings-section="commands" hidden={area() !== "commands"}>
            <h3>Deployment commands</h3>
            <LifecycleCommands compact config={config} services={props.application.latestDeployment?.services?.map(s => s.name)}
              change={hooks => setConfig("hooks", hooks)} />
          </section>
          <Show when={error()}><ErrorNotice message={error()} /></Show>
          <EditorActions docked={dirty()} measure="form" inlineWhenFits>
            <Button variant="primary" type="submit" form={formId} icon="deploy" disabled={props.busy || !dirty()} aria-disabled={reviewing() || undefined}>
              {reviewing() ? "Reviewing changes" : "Review changes"}
            </Button>
          </EditorActions>
        </form>
      </div>
    </div>
    <Show when={confirm()}><DeleteDialog name={props.application.name} close={() => setConfirm(false)} remove={props.remove} /></Show>
  </div>;
}
function DeleteDialog(props: { name: string; close: () => void; remove: () => Promise<void> }) {
  const [value, setValue] = createSignal(""), [busy, setBusy] = createSignal(false), [error, setError] = createSignal("");
  let panel!: HTMLDivElement, input!: HTMLInputElement;const previous = document.activeElement as HTMLElement | null;
  onMount(() => { input.focus(); const key = (e: KeyboardEvent) => { if (e.key === "Escape" && !busy()) props.close(); if (e.key === "Tab") { const nodes = Array.from(panel.querySelectorAll<HTMLElement>("button:not(:disabled),input")); const first = nodes[0], last = nodes[nodes.length - 1]; if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); } else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); } } };document.addEventListener("keydown", key);onCleanup(() => { document.removeEventListener("keydown", key); previous?.focus(); }); });
  async function remove(e: SubmitEvent) { e.preventDefault(); if (value() !== props.name || busy()) return; setBusy(true);try { await props.remove(); props.close(); } catch (e) { setError((e as Error).message); }finally { setBusy(false); } }
  return <Portal><div class="drawer-scrim" onClick={() => { if (!busy()) props.close(); }} /><div ref={panel} class="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-title"><header><h2 id="delete-title">Delete {props.name}?</h2><IconButton icon="close" label="Close deletion" ghost disabled={busy()} onClick={props.close} /></header><p>Deleting this application removes its containers and retains its images and volumes.</p><form onSubmit={remove}><Show when={error()}><ErrorNotice message={error()} /></Show><div class="form-fields"><label for="confirm-name">Application name</label><input ref={input} id="confirm-name" class="input" value={value()} onInput={e => setValue(e.currentTarget.value)} autocomplete="off" /></div><div class="form-actions"><Button variant="ghost" disabled={busy()} onClick={props.close}>Cancel</Button><Button type="submit" class="danger-button" icon="trash" disabled={busy() || value() !== props.name}>{busy() ? "Deleting application" : "Delete application"}</Button></div></form></div></Portal>;
}
