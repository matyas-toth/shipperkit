import { Index, Show, createEffect, createMemo, createSignal, createUniqueId, onCleanup } from "solid-js";
import { Dynamic, Portal } from "solid-js/web";
import { post } from "../api";
import { useAuth } from "../context";
import type { Application, StaticConfig, Variable } from "../types";
import { clearDraft, readDraft, saveDraft, saveVolatileDraft } from "../drafts";
import { emptyVariable, looksLikeEnvironment, mergeEnvironment, parseEnvironment, variableHasContent, variableSignature, variablesForSave } from "../env";
import { Button, ErrorNotice, IconButton } from "./Controls";

type Options = { index: number; left: number; top: number; trigger: HTMLButtonElement };

export default function Variables(props: { application: Application; busy: boolean; save?: (config: StaticConfig) => Promise<boolean | void>; embedded?: boolean; service?: string; change?: (variables: Variable[]) => void }) {
  const auth = useAuth(), id = createUniqueId(), key = "variables:" + auth.session()?.user?.id + ":" + props.application.id + (props.service ? ":" + props.service : "");
  const initial = (props.embedded ? props.application.config.variables : readDraft<Variable[]>(key) ?? props.application.config.variables) ?? [];
  const [items, setItems] = createSignal<Variable[]>([...variablesForSave(initial), emptyVariable()]);
  const [error, setError] = createSignal(""), [saving, setSaving] = createSignal(false), [revealing, setRevealing] = createSignal(false);
  const [visible, setVisible] = createSignal<Record<number, boolean>>({}), [options, setOptions] = createSignal<Options>();
  let root!: HTMLElement, panel!: HTMLDivElement;
  const dirty = createMemo(() => variableSignature(items()) !== variableSignature(props.application.config.variables ?? []));

  createEffect(() => {if(props.embedded)return;if(!dirty()){clearDraft(key);return;}const rows=variablesForSave(items());saveVolatileDraft(key,rows.map(v=>v.secret&&v.keep?{...v,value:""}:v));saveDraft(key,rows.filter(v=>!v.secret||v.keep).map(v=>v.secret?{...v,value:""}:v));});
  createEffect(() => props.change?.(variablesForSave(items())));
  function update(index: number, value: Partial<Variable>) {
    setItems(rows => {
      const next = rows.map((row, i) => i === index ? { ...row, ...value } : row);
      if (variableHasContent(next[next.length - 1])) next.push(emptyVariable());
      return next;
    });
    setError("");
  }
  function paste(event: ClipboardEvent, nameField: boolean) {
    const text = event.clipboardData?.getData("text/plain") ?? "";
    if ((!nameField && !text.includes("\n")) || !looksLikeEnvironment(text)) return;
    event.preventDefault();setError("");
    try { const merged = mergeEnvironment(items(), parseEnvironment(text));setOptions(undefined);setVisible({});setItems([...merged, emptyVariable()]); }
    catch (e) { setError((e as Error).message); }
  }
  function closeOptions(restoreFocus = false) {
    const trigger = options()?.trigger;setOptions(undefined);
    if (restoreFocus) trigger?.focus();
  }
  function openOptions(index: number, trigger: HTMLButtonElement) {
    if (options()?.index === index) { closeOptions();return; }
    const rect = trigger.getBoundingClientRect();
    setOptions({ index, trigger, left: Math.max(12, Math.min(rect.right - 240, window.innerWidth - 252)), top: Math.max(12, Math.min(rect.bottom + 6, window.innerHeight - 250)) });
  }
  createEffect(() => {
    const opened = options();if (!opened) return;
    const outside = (event: Event) => { const target = event.target as Node;if (!panel?.contains(target) && !opened.trigger.contains(target)) closeOptions(); };
    const keydown = (event: KeyboardEvent) => { if (event.key === "Escape") { event.preventDefault();closeOptions(true); } };
    const dismiss = () => closeOptions();
    const scroll = (event: Event) => { if (!panel?.contains(event.target as Node)) dismiss(); };
    document.addEventListener("pointerdown", outside);document.addEventListener("focusin", outside);document.addEventListener("keydown", keydown);
    window.addEventListener("resize", dismiss);window.addEventListener("scroll", scroll, true);
    queueMicrotask(() => panel?.querySelector<HTMLElement>("select,input")?.focus());
    onCleanup(() => { document.removeEventListener("pointerdown", outside);document.removeEventListener("focusin", outside);document.removeEventListener("keydown", keydown);window.removeEventListener("resize", dismiss);window.removeEventListener("scroll", scroll, true); });
  });
  async function reveal(index: number) {
    const variable = items()[index];setRevealing(true);setError("");
    try {
      if (!visible()[index] && variable.keep && !variable.value) {
        const result = await post<{ value: string }>(`/applications/${props.application.id}/variables/reveal`, { name: variable.keepFrom || variable.name, ...(props.service ? { service: props.service } : {}) }, auth.session()?.csrfToken);
        if (items()[index] !== variable) return;
        update(index, { value: result.value });
      }
      setVisible(value => ({ ...value, [index]: !value[index] }));closeOptions(true);
    } catch (e) { setError((e as Error).message); }
    finally { setRevealing(false); }
  }
  function generate(index:number){const bytes=crypto.getRandomValues(new Uint8Array(32));const value=btoa(String.fromCharCode(...bytes)).replaceAll("+","-").replaceAll("/","_").replaceAll("=","");update(index,{value,secret:true,keep:false,keepFrom:undefined,hasValue:false});setVisible(v=>({...v,[index]:false}));closeOptions(true);}
  function remove(index: number) {
    closeOptions();setItems(rows => [...variablesForSave(rows.filter((_, i) => i !== index)), emptyVariable()]);setVisible({});
    queueMicrotask(() => root.querySelectorAll<HTMLInputElement>(".env-name")[Math.min(index, items().length - 1)]?.focus());
  }
  async function findMissing(){setSaving(true);setError("");try{const result=await post<{variables:string[]}>("/repositories/inspect",{config:props.application.config},auth.session()?.csrfToken);const names=new Set(items().map(v=>v.name).filter(Boolean));const found=result.variables.filter(name=>!names.has(name)).slice(0,Math.max(0,128-names.size)).map(name=>({...emptyVariable(),name}));setItems([...variablesForSave(items()),...found,emptyVariable()]);}catch(e){setError((e as Error).message);}finally{setSaving(false);}}
  async function save(event: SubmitEvent) {
    event.preventDefault();if (props.busy || saving() || !dirty()) return;setError("");
    const variables = variablesForSave(items()), names = new Set<string>();
    if (variables.length > 128) { setError("Applications support up to 128 variables");return; }
    for (const variable of variables) {
      if (names.has(variable.name)) { setError(`Variable ${variable.name} appears more than once`);return; }
      names.add(variable.name);
    }
    setSaving(true);
    try { if(await props.save?.({ ...props.application.config, variables })!==false)clearDraft(key); }
    catch (e) { setError((e as Error).message); }
    finally { setSaving(false); }
  }
  return <Dynamic ref={(element: HTMLFormElement | HTMLDivElement) => { root = element; }} component={props.embedded ? "div" : "form"} onSubmit={props.embedded ? undefined : save} class="variables-panel" aria-label=".env">
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <div class="table-tray variables-tray"><div class="variables-header"><span>Name</span><span>Value</span><span class="sr-only">Options</span></div><div class="variables-body"><Index each={items()}>{(variable, index) => <div class="variable-row">
      <input class="input mono env-name" aria-label={"Variable name " + (index + 1)} placeholder="NAME" value={variable().name} required={variableHasContent(variable())} pattern="[A-Za-z_][A-Za-z0-9_]*" maxlength={128} autocomplete="off" autocapitalize="off" spellcheck={false} onPaste={event => paste(event, true)} onInput={event => update(index, { name: event.currentTarget.value, keepFrom: variable().keep ? variable().keepFrom || variable().name : undefined })} />
      <input class="input mono" aria-label={"Value for " + (variable().name || "variable " + (index + 1))} type={variable().secret && !visible()[index] ? "password" : "text"} value={variable().value} placeholder={variable().keep ? "••••••••" : "Value"} autocomplete="off" spellcheck={false} onPaste={event => paste(event, false)} onInput={event => update(index, { value: event.currentTarget.value, keep: false, keepFrom: undefined })} />
      <div class="variable-options"><Show when={variableHasContent(variable())}><IconButton icon="more" label={"Options for " + (variable().name || "variable " + (index + 1))} ghost aria-haspopup="dialog" aria-expanded={options()?.index === index} onClick={event => openOptions(index, event.currentTarget)} /></Show></div>
    </div>}</Index></div></div>
    <Show when={!props.embedded}><div class="form-actions"><Show when={!props.service&&props.application.config.repository}><Button variant="ghost" disabled={props.busy||saving()} onClick={()=>void findMissing()}>Find missing variables</Button></Show><Button type="submit" icon="deploy" disabled={props.busy || saving() || !dirty()}>{saving() ? "Saving" : "Review changes"}</Button></div></Show>
    <Show when={options()}>{opened => <Portal><div ref={panel} class="variable-menu" role="dialog" aria-label={"Options for " + (items()[opened().index]?.name || "variable")} style={{ left: opened().left + "px", top: opened().top + "px" }}>
      <Show when={!props.service && props.application.config.method !== "compose"}><label for={id + "-scope"}>Available during</label><select id={id + "-scope"} class="input" value={items()[opened().index]?.scope || "both"} onChange={event => update(opened().index, { scope: event.currentTarget.value as Variable["scope"] })}><option value="both">Build and runtime</option><option value="runtime">Runtime</option><option value="build">Build</option></select></Show>
      <label class="check-field"><input type="checkbox" checked={items()[opened().index]?.secret} onChange={event => update(opened().index, { secret: event.currentTarget.checked })} />Secret</label>
      <div class="variable-menu-actions"><Show when={items()[opened().index]?.secret || (items()[opened().index]?.keep && !items()[opened().index]?.value)}><Button variant="ghost" icon="eye" disabled={revealing()} onClick={() => void reveal(opened().index)}>{visible()[opened().index] ? "Hide value" : "Reveal value"}</Button></Show><Button variant="ghost" icon="refresh" onClick={()=>generate(opened().index)}>Generate value</Button><Button variant="ghost" icon="trash" class="danger-button" onClick={() => remove(opened().index)}>Remove variable</Button></div>
    </div></Portal>}</Show>
  </Dynamic>;
}
