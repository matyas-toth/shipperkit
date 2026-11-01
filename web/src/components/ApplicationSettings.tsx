import { createStore } from "solid-js/store";
import { Show, createSignal, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import type { Application, StaticConfig } from "../types";
import StaticFields from "./StaticFields";
import { Button, ErrorNotice, IconButton } from "./Controls";

export default function ApplicationSettings(props: { application: Application; acme: boolean; busy: boolean; save: (name: string, config: StaticConfig) => Promise<void>; remove: () => Promise<void> }) {
  const [config, setConfig] = createStore({ ...props.application.config }), [name, setName] = createSignal(props.application.name), [error, setError] = createSignal(""), [confirm, setConfirm] = createSignal(false);
  async function save(e: SubmitEvent) { e.preventDefault(); setError(""); try { await props.save(name(), config); } catch (e) { setError((e as Error).message); } }
  return <div class="application-settings"><form onSubmit={save} class="app-config-form"><Show when={error()}><ErrorNotice message={error()} /></Show><div class="form-fields"><label for="application-name">Name</label><input id="application-name" class="input" value={name()} required maxlength={80} onInput={e => setName(e.currentTarget.value)} />
    <StaticFields config={config} change={(key, value) => setConfig(key, value as never)} advanced acme={props.acme} /></div><div class="form-actions"><Button type="submit" icon="deploy" disabled={props.busy}>Save and deploy</Button></div></form>
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
