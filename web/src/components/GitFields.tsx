import { For, Show, createEffect, createMemo, createSignal, createUniqueId } from "solid-js";
import type { StaticConfig } from "../types";
import { post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice } from "./Controls";
import { GitConnectionFields } from "./SourceFields";

export default function GitFields(props: {
  config: StaticConfig;
  change: (key: keyof StaticConfig, value: unknown) => void;
}) {
  const auth = useAuth();
  const [branches, setBranches] = createSignal<string[]>([]);
  const [branchDefault, setBranchDefault] = createSignal("");
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal("");
  const [open, setOpen] = createSignal(false), [loaded, setLoaded] = createSignal(false);
  const [filter, setFilter] = createSignal("");
  const listId = createUniqueId();
  const matches = createMemo(() => branches().filter(branch => branch.toLowerCase().includes(filter().toLowerCase())));
  let branchInput!: HTMLInputElement;
  const sourceKey = () => JSON.stringify([props.config.repository, props.config.sourceId]);
  createEffect(() => { sourceKey(); setBranches([]); setBranchDefault(""); setError(""); setOpen(false); setLoaded(false); setFilter(""); });
  async function loadBranches() {
    if (loading() || !props.config.repository) return;
    const key = sourceKey();
    setLoading(true);
    setError("");
    try {
      const result = await post<{ items: string[]; default: string }>("/repositories/branches", {
        repository: props.config.repository, sourceId: props.config.sourceId,
      }, auth.session()?.csrfToken);
      if (key !== sourceKey() || !branchInput.isConnected) return;
      setBranches(result.items);
      setBranchDefault(result.default);
      setLoaded(true);
    } catch (e) {
      if (key === sourceKey()) setError((e as Error).message);
    } finally { setLoading(false); }
  }
  return <>
    <GitConnectionFields config={props.config} change={props.change} />
    <Show when={!props.config.sourceId}>
      <div class="field git-field"><label for="repository">Repository URL</label>
      <input class="input" id="repository" type="url" required placeholder="https://github.com/owner/application"
        autocomplete="off" value={props.config.repository} onInput={e => props.change("repository", e.currentTarget.value)} /></div>
    </Show>
    <div class="field git-field"><label for="branch">Branch or commit</label>
    <div class="branch-field">
      <input ref={branchInput} class="input" id="branch"
        placeholder={branchDefault() ? "Default branch (" + branchDefault() + ")" : "Default branch"}
        value={props.config.branch}
        onInput={e => props.change("branch", e.currentTarget.value)} />
      <Button icon="down" aria-expanded={open()} aria-controls={listId}
        disabled={!props.config.repository} onClick={() => {
          const next = !open(); setOpen(next);
          if (next && !loaded()) void loadBranches();
        }}>Browse branches</Button>
    </div>
    <Show when={open()}><div class="branch-picker" id={listId} role="group" aria-label="Branches"
      onKeyDown={e => { if (e.key === "Escape") { setOpen(false); branchInput.focus(); } }}>
      <Show when={loading()}><p role="status">Loading branches</p></Show>
      <Show when={error()}><ErrorNotice message={error()} retry={() => void loadBranches()} /></Show>
      <Show when={loaded()}>
        <Show when={branches().length > 6}><input class="input" aria-label="Search branches" placeholder="Search branches" value={filter()} onInput={e => setFilter(e.currentTarget.value)} /></Show>
        <div class="branch-options"><For each={matches()} fallback={<p>No matching branches</p>}>{branch =>
          <button type="button" class="branch-option" onClick={() => { props.change("branch", branch); setOpen(false); branchInput.focus(); }}>{branch}</button>
        }</For></div>
      </Show>
    </div></Show></div>
  </>;
}
