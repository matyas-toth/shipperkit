import { createQuery } from "@tanstack/solid-query";
import { For, Show, createEffect, createSignal, createUniqueId, onCleanup } from "solid-js";
import { post } from "../api";
import { useAuth } from "../context";
import type { Connection, StaticConfig } from "../types";
import { Button, ErrorNotice } from "./Controls";
import { Icon } from "./Icon";

export function GitHubAccountPicker(props: { accounts: Connection[]; selected?: string; select: (id: string) => void }) {
  const id = createUniqueId();
  const [open, setOpen] = createSignal(false);
  const [position, setPosition] = createSignal({ left: "0px", top: "0px", "max-height": "320px" });
  let popover!: HTMLDivElement;
  let trigger!: HTMLButtonElement;
  createEffect(() => {
    if (!open()) return;
    const reposition = () => place(trigger);
    window.addEventListener("resize", reposition);
    document.addEventListener("scroll", reposition, true);
    onCleanup(() => { window.removeEventListener("resize", reposition); document.removeEventListener("scroll", reposition, true); });
  });
  function place(trigger: HTMLButtonElement) {
    const rect = trigger.getBoundingClientRect();
    const width = Math.min(300, window.innerWidth - 24);
    const below = window.innerHeight - rect.bottom - 16;
    const above = rect.top - 16;
    const height = Math.min(320, Math.max(below, above));
    setPosition({ left: Math.max(12, Math.min(rect.left, window.innerWidth - width - 12)) + "px",
      top: (below >= Math.min(320, above) ? rect.bottom + 8 : Math.max(12, rect.top - height - 8)) + "px", "max-height": height + "px" });
  }
  return <>
    <Button ref={trigger} small variant="ghost" icon="git" popovertarget={id} aria-haspopup="dialog" aria-expanded={open()} onClick={e => place(e.currentTarget)}>GitHub Repository<Icon name="down" /></Button>
    <div ref={popover} id={id} popover="auto" role="dialog" aria-label="GitHub accounts" class="github-account-popover" style={position()}
      onToggle={e => setOpen((e as ToggleEvent).newState === "open")}>
      <For each={props.accounts}>{(account, index) => <button type="button" autofocus={account.id === props.selected || !props.selected && index() === 0}
        aria-pressed={account.id === props.selected} onClick={() => {
          props.select(account.id); popover.hidePopover();
          queueMicrotask(() => document.querySelector<HTMLElement>(".connected-source-fields")?.focus({ preventScroll: true }));
        }}><Icon name="git" /><span>{account.name}</span><Show when={account.id === props.selected}><Icon name="check" /></Show></button>}</For>
    </div>
  </>;
}

export function RepositoryBranchPicker(props: { config: StaticConfig; change: (key: keyof StaticConfig, value: unknown) => void }) {
  const auth = useAuth(), id = createUniqueId();
  const query = createQuery(() => ({
    queryKey: ["repository-branches", props.config.sourceId, props.config.repository],
    queryFn: () => post<{ items: string[]; default: string }>("/repositories/branches", {
      repository: props.config.repository, sourceId: props.config.sourceId,
    }, auth.session()?.csrfToken),
    enabled: !!props.config.repository, staleTime: 60000, retry: false,
  }));
  const value = () => props.config.branch || query.data?.default || "";
  return <div class="connected-branch-field">
    <label for={id}>Branch</label>
    <select id={id} class="input" value={query.isPending || query.isError ? "" : value()}
      disabled={query.isPending || query.isError} aria-busy={query.isPending} onChange={e => props.change("branch", e.currentTarget.value)}>
      <Show when={query.isPending || query.isError} fallback={<>
        <Show when={!value()}><option value="">Default branch</option></Show>
        <Show when={value() && !query.data?.items.includes(value())}><option value={value()}>{value()}</option></Show>
        <For each={query.data?.items}>{branch => <option value={branch}>{branch}</option>}</For>
      </>}><option value="">{query.isPending ? "Loading branches" : "Branches unavailable"}</option></Show>
    </select>
    <Show when={query.error}><ErrorNotice message={query.error!.message} retry={() => void query.refetch()} /></Show>
  </div>;
}
