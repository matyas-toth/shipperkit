import { For, Show, createEffect, createMemo, createSignal } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import type { Deployment, LogLine } from "../types";
import { createdDate } from "../data";
import { Icon } from "./Icon";
import { ErrorNotice, IconButton, Status } from "./Controls";

export default function LogViewer(props: { deployment: Deployment }) {
  const [lines, setLines] = createSignal<LogLine[]>([]), [cursor, setCursor] = createSignal(0), [search, setSearch] = createSignal(""), [scroll, setScroll] = createSignal(0), [copied, setCopied] = createSignal(false), [copyError, setCopyError] = createSignal("");
  let viewport!: HTMLDivElement, follow = true;
  const query = createQuery(() => ({ queryKey: ["deployment-logs", props.deployment.id, cursor()], queryFn: () => api<{ items: LogLine[] }>(`/applications/${props.deployment.applicationId}/deployments/${props.deployment.id}/logs?after=${cursor()}`), refetchInterval: 5000, gcTime: 0 }));
  createEffect(() => {
    const batch = query.data?.items;
    if (batch?.length) {
      const last = batch[batch.length - 1].sequence;
      if (last > cursor()) { setLines(current => [...current, ...batch.filter(line => line.sequence > cursor())].slice(-10000)); setCursor(last); }
      queueMicrotask(() => { if (follow && viewport) viewport.scrollTop = viewport.scrollHeight; });
    }
  });
  const filtered = createMemo(() => lines().filter(line => !search() || line.line.toLowerCase().includes(search().toLowerCase())));
  const start = createMemo(() => Math.max(0, Math.min(Math.floor(scroll() / 22) - 5, filtered().length - 64)));
  const visible = createMemo(() => filtered().slice(start(), start() + 64));
  async function copy() { try { await navigator.clipboard.writeText(lines().map(line => line.line).join("\n")); setCopied(true); } catch { setCopyError("The logs could not be copied"); } }
  return <div class="log-panel">
    <div class="log-heading"><Status value={props.deployment.state} /><span class="mono log-commit">{props.deployment.commit.slice(0, 8) || props.deployment.id.slice(0, 8)}</span><span class="muted log-date">{createdDate(props.deployment.createdAt)}</span><IconButton icon={copied() ? "check" : "copy"} label="Copy logs" ghost onClick={() => void copy()} /></div>
    <div class="log-toolbar"><Icon name="terminal" /><div class="search-field"><Icon name="search" /><input aria-label="Search logs" placeholder="Search logs" value={search()} onInput={e => { setSearch(e.currentTarget.value); if (viewport) viewport.scrollTop = 0; }} /></div></div>
    <Show when={props.deployment.error}><div class="deployment-error" role="alert"><Icon name="warning" />{props.deployment.error}</div></Show>
    <Show when={query.error || copyError()}><ErrorNotice message={copyError() || query.error!.message} /></Show>
    <div ref={viewport} class="log-viewport mono" role="log" aria-label="Deployment logs" aria-live="off" tabindex="0" onScroll={e => { setScroll(e.currentTarget.scrollTop); follow = e.currentTarget.scrollHeight - e.currentTarget.clientHeight - e.currentTarget.scrollTop < 44; }}>
      <div style={{ height: start() * 22 + "px" }} />
      <For each={visible()}>{line => <div class="log-line"><span class="log-number">{line.sequence}</span><span class="log-step">{line.step}</span><span>{line.line}</span></div>}</For>
      <div style={{ height: Math.max(0, filtered().length - start() - 64) * 22 + "px" }} />
      <Show when={!lines().length}><div class="log-empty">{query.isPending ? "Loading logs" : "Waiting for deployment"}</div></Show>
    </div>
  </div>;
}
