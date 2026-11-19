import { createStore, reconcile } from "solid-js/store";
import { For, Show, createEffect, createMemo, createSignal, createUniqueId, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import type { Application, StaticConfig } from "../types";
import { LifecycleCommands } from "./Commands";
import StaticFields, { normalizedConfig } from "./StaticFields";
import { Button, Disclosure, ErrorNotice, IconButton, validateForm } from "./Controls";
import Volumes from "./Volumes";
import RuntimeFields from "./RuntimeFields";
import { MemoryLimitField, ReadinessField, RuntimeConnectivity } from "./DeployFields";
import { Icon } from "./Icon";
import EditorActions from "./EditorActions";
import { createdDate } from "../data";

const areas = [
  { id: "general", label: "General", icon: "settings" },
  { id: "source", label: "Source and build", icon: "layers" },
  { id: "runtime", label: "Runtime", icon: "terminal" },
  { id: "storage", label: "Storage", icon: "database" },
  { id: "commands", label: "Deployment commands", icon: "code" },
] as const;
type SettingsArea = (typeof areas)[number]["id"];

// The editor owns a deep copy. Nested objects shared with the query cache would let edits
// rewrite the saved config and let background refetches overwrite edits in progress.
function editableConfig(config: Partial<StaticConfig>) {
  return JSON.parse(JSON.stringify(normalizedConfig(config))) as StaticConfig;
}

// Inline sources and .env values are compared elsewhere, so they stay out of the settings fingerprint.
function configFingerprint(config: StaticConfig) {
  const comparable = { ...config, inlineDockerfile: "", inlineKeep: config.inlineKeep || config.inlineHasValue || undefined, composeYAML: "", composeKeep: config.composeKeep || config.composeHasValue || undefined, services: config.services?.map(service => ({ ...service, variables: undefined })), variables: undefined };
  return JSON.stringify(comparable, (_, value) =>
    value && typeof value === "object" && !Array.isArray(value)
      ? Object.fromEntries(Object.keys(value).sort().map(key => [key, value[key]]))
      : value);
}

export default function ApplicationSettings(props: {
  application: Application;
  services?: string[];
  showSource?: boolean;
  busy: boolean;
  save: (config: StaticConfig) => Promise<boolean | void>;
  rename: (name: string) => Promise<void>;
  remove: () => Promise<void>;
}) {
  const formId = createUniqueId();
  // Edits live only in this view until they are saved. Leaving or reloading the page discards them.
  const [config, setConfig] = createStore(editableConfig(props.application.config));
  const [name, setName] = createSignal(props.application.name);
  const [error, setError] = createSignal("");
  const [reviewing, setReviewing] = createSignal(false);
  const [confirm, setConfirm] = createSignal(false);
  const inlineCompose = () => config.method === "compose" && config.composeSource === "paste";
  const [area, setArea] = createSignal<SettingsArea>(props.showSource || inlineCompose() ? "source" : "general");
  const visibleAreas = () => areas.filter(item => config.method !== "compose" || !["runtime", "storage"].includes(item.id));
  const hasSavedCompose = () => props.application.config.composeSource === "paste" &&
    !!(props.application.config.composeKeep || props.application.config.composeHasValue);
  const composeEdited = () => inlineCompose() && (config.composeKeep === false && hasSavedCompose() || !!config.composeYAML);
  const savedFingerprint = createMemo(() => configFingerprint(normalizedConfig(props.application.config)));
  const configDirty = createMemo(() => composeEdited() || configFingerprint(config) !== savedFingerprint());
  const renamed = () => !!name().trim() && name().trim() !== props.application.name;
  const dirty = () => configDirty() || renamed();
  createEffect(() => { if (props.showSource) setArea("source"); });
  function revert() { setConfig(reconcile(editableConfig(props.application.config))); setName(props.application.name); setError(""); }
  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (props.busy || reviewing() || !dirty()) return;
    const form = e.currentTarget as HTMLFormElement;
    const invalidArea = form.querySelector(":invalid")?.closest<HTMLElement>("[data-settings-section]")?.dataset.settingsSection as SettingsArea | undefined;
    if (invalidArea) setArea(invalidArea);
    if (!validateForm(form)) return;
    if (!name().trim()) { setArea("general"); setError("Name the application"); return; }
    setError("");
    setReviewing(true);
    try {
      // A new name applies right away; configuration changes still go through the review.
      if (renamed()) await props.rename(name().trim());
      if (!configDirty()) return;
      await props.save({ ...config, variables: props.application.config.variables, services: props.application.config.services });
    } catch (e) { setError((e as Error).message); }
    finally { setReviewing(false); }
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
          <div class="form-fields rename-form">
            <label for="application-name">Name</label>
            <input id="application-name" class="input" value={name()} required maxlength={80} form={formId} onInput={e => setName(e.currentTarget.value)} />
          </div>
          <p class="app-settings-created">Created {createdDate(props.application.createdAt)}</p>
          <div class="app-settings-secondary">
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
                connectivity={<RuntimeConnectivity config={config} change={(key, value) => setConfig(key, value as never)} method={config.method} withoutMemory withoutReadiness />}
                memory={<MemoryLimitField config={config} change={(key, value) => setConfig(key, value as never)} />}
                advanced={config.method !== "static" ? <ReadinessField config={config} change={(key, value) => setConfig(key, value as never)} /> : undefined} />
            </section>
            <section class="app-settings-pane" data-settings-section="storage" hidden={area() !== "storage"}>
              <h3>Storage</h3>
              <Volumes values={config.volumes} change={volumes => setConfig("volumes", volumes)} />
            </section>
          </Show>
          <section class="app-settings-pane" data-settings-section="commands" hidden={area() !== "commands"}>
            <h3>Deployment commands</h3>
            <LifecycleCommands compact config={config} services={props.services}
              change={hooks => setConfig("hooks", hooks)} />
          </section>
          <Show when={error()}><ErrorNotice message={error()} /></Show>
          <EditorActions docked={dirty()} measure="form" inlineWhenFits revert={dirty() ? revert : undefined}>
            <Button variant="primary" type="submit" form={formId} icon="deploy" disabled={props.busy || !dirty()} aria-disabled={reviewing() || undefined}>
              {reviewing() ? "Saving" : renamed() && !configDirty() ? "Save changes" : "Save and redeploy"}
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
  return <Portal><div class="drawer-scrim" onClick={() => { if (!busy()) props.close(); }} /><div ref={panel} class="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-title"><header><h2 id="delete-title">Delete {props.name}?</h2><IconButton icon="close" label="Close deletion" ghost disabled={busy()} onClick={props.close} /></header><p>Deleting removes its containers and keeps its images. Its volumes stay listed under Recovery in Settings.</p><form onSubmit={remove}><Show when={error()}><ErrorNotice message={error()} /></Show><div class="form-fields"><label for="confirm-name">Application name</label><input ref={input} id="confirm-name" class="input" value={value()} onInput={e => setValue(e.currentTarget.value)} autocomplete="off" /></div><div class="form-actions"><Button variant="ghost" disabled={busy()} onClick={props.close}>Cancel</Button><Button type="submit" class="danger-button" icon="trash" disabled={busy() || value() !== props.name}>{busy() ? "Deleting application" : "Delete application"}</Button></div></form></div></Portal>;
}
