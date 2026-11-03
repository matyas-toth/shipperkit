import { createStore } from "solid-js/store";
import { Index, Show, createEffect, createMemo, createSignal, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import type { Application, StaticConfig } from "../types";
import DeploymentTriggers from "./DeploymentTriggers";
import { LifecycleCommands } from "./Commands";
import StaticFields, { normalizedConfig } from "./StaticFields";
import { Button, ErrorNotice, IconButton } from "./Controls";
import { clearDraft, readDraft, safeConfigDraft, saveDraft } from "../drafts";
import { useAuth } from "../context";

export default function ApplicationSettings(props: { application: Application; acme: boolean; busy: boolean; save: (config: StaticConfig) => Promise<boolean | void>; rename: (name: string) => Promise<void>; remove: () => Promise<void> }) {
  const auth = useAuth(), key = "settings:" + auth.session()?.user?.id + ":" + props.application.id;
  const [config, setConfig] = createStore(normalizedConfig(readDraft<StaticConfig>(key) ?? props.application.config)), [name, setName] = createSignal(props.application.name), [error, setError] = createSignal(""), [renaming, setRenaming] = createSignal(false), [confirm, setConfirm] = createSignal(false);
  const dirty=createMemo(()=>JSON.stringify(safeConfigDraft(config))!==JSON.stringify(safeConfigDraft(normalizedConfig(props.application.config))));
  createEffect(() => saveDraft(key, safeConfigDraft(config)));
  async function save(e: SubmitEvent) { e.preventDefault(); setError(""); try { if(await props.save({ ...config, variables: props.application.config.variables, services: props.application.config.services })!==false)clearDraft(key); } catch (e) { setError((e as Error).message); } }
  async function rename(e: SubmitEvent) { e.preventDefault();setRenaming(true);setError("");try { await props.rename(name()); } catch (e) { setError((e as Error).message); } finally { setRenaming(false); } }
  return <div class="application-settings"><Show when={error()}><ErrorNotice message={error()} /></Show><form onSubmit={rename} class="rename-form"><label for="application-name">Name</label><div><input id="application-name" class="input" value={name()} required maxlength={80} onInput={e => setName(e.currentTarget.value)} /><Button type="submit" disabled={renaming() || name() === props.application.name}>Save name</Button></div></form><form onSubmit={save} class="app-config-form"><div class="form-fields">
    <StaticFields config={config} change={(key, value) => setConfig(key, value as never)} advanced applicationId={props.application.id} acme={props.acme} /></div>
    <Show when={config.method !== "compose"}><section class="volume-settings"><div class="section-toolbar"><h2>Persistent volumes</h2><Button icon="plus" variant="ghost" onClick={() => setConfig("volumes", rows => [...rows, { name: "", path: "" }])}>Add volume</Button></div><Index each={config.volumes}>{(volume, index) => <div class="volume-row"><input class="input" aria-label={"Volume name " + (index + 1)} placeholder="data" value={volume().name} required onInput={e => setConfig("volumes", index, "name", e.currentTarget.value)} /><input class="input mono" aria-label={"Mount path " + (index + 1)} placeholder="/app/data" value={volume().path} required onInput={e => setConfig("volumes", index, "path", e.currentTarget.value)} /><IconButton icon="trash" label={"Remove volume " + (index + 1)} ghost onClick={() => setConfig("volumes", rows => rows.filter((_, i) => i !== index))} /></div>}</Index></section></Show>
    <LifecycleCommands config={config} services={props.application.latestDeployment?.services?.map(s=>s.name)} change={hooks=>setConfig("hooks",hooks)} />
 <div class="form-actions"><Button type="submit" icon="deploy" disabled={props.busy||!dirty()}>Review changes</Button></div></form>
    <DeploymentTriggers applicationId={props.application.id}/>
    <section class="delete-section"><h2>Delete application</h2><Button icon="trash" class="danger-button" disabled={props.busy} onClick={() => setConfirm(true)}>Delete application</Button></section>
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
