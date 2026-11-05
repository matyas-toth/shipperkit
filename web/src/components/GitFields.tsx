import { For, Show, createEffect, createSignal } from "solid-js";
import type { StaticConfig } from "../types";
import { post } from "../api";
import { useAuth } from "../context";
import { ErrorNotice, IconButton } from "./Controls";
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
  let branchInput!: HTMLInputElement;
  const sourceKey = () => JSON.stringify([props.config.repository, props.config.sourceId]);
  createEffect(() => { sourceKey(); setBranches([]); setBranchDefault(""); setError(""); });
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
      if (document.activeElement === branchInput) try { branchInput.showPicker?.(); } catch {}
    } catch (e) {
      if (key === sourceKey()) setError((e as Error).message);
    } finally { setLoading(false); }
  }
  return <>
    <GitConnectionFields config={props.config} change={props.change} />
    <Show when={!props.config.sourceId}>
      <label for="repository">Repository URL</label>
      <input class="input" id="repository" type="url" required placeholder="https://github.com/owner/application"
        autocomplete="off" value={props.config.repository} onInput={e => props.change("repository", e.currentTarget.value)} />
    </Show>
    <label for="branch">Branch or commit</label>
    <div class="branch-field">
      <input ref={branchInput} class="input" id="branch" list="repository-branches"
        placeholder={branchDefault() ? "Default branch (" + branchDefault() + ")" : "Default branch"}
        value={props.config.branch}
        onFocus={() => { if (!branches().length && !loading() && !error()) void loadBranches(); }}
        onInput={e => props.change("branch", e.currentTarget.value)} />
      <IconButton icon={loading() ? "refresh" : "down"} label="Load branches" ghost
        disabled={loading() || !props.config.repository} onClick={() => { branchInput.focus(); void loadBranches(); }} />
      <datalist id="repository-branches"><For each={branches()}>{branch => <option value={branch} />}</For></datalist>
    </div>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
  </>;
}
