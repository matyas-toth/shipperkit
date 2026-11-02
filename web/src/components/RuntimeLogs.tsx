import { For, Show, createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import type { Application } from "../types";
import { ErrorNotice, IconButton } from "./Controls";
import { Icon } from "./Icon";

export default function RuntimeLogs(props: { application: Application; containerId?: string }) {
  const [selected, setSelected] = createSignal(""), [lines, setLines] = createSignal<{ id: number; text: string }[]>([]), [search, setSearch] = createSignal(""), [error, setError] = createSignal(""), [live, setLive] = createSignal(false), [scroll, setScroll] = createSignal(0), [copied, setCopied] = createSignal(false);
  const [revision, setRevision] = createSignal("");
  const containerID = createMemo(() => props.application.containers.some(c => c.id === selected()) ? selected() : (props.application.containers.some(c => c.id === props.containerId) ? props.containerId! : props.application.containers[0]?.id || ""));
  let viewport!: HTMLDivElement, follow = true, sequence = 0;
  createEffect(() => { const operation = props.application.latestDeployment;if (operation?.state === "succeeded" && ["start", "restart"].includes(operation.operation)) setRevision(operation.id); });
  createEffect(() => {
    revision();props.application.desiredState;
    const id = containerID();props.application.containers.find(c => c.id === id)?.state;if (!id) {setLines([]);setError("");setLive(false);return;}
    const stream = new EventSource(`/api/applications/${props.application.id}/logs?container=${encodeURIComponent(id)}&tail=200`);setError("");
    stream.onopen = () => setLive(true);stream.addEventListener("reset", () => { setLines([]);sequence = 0; });
    stream.addEventListener("log", event => { const value = JSON.parse((event as MessageEvent).data) as { line: string };setLines(current => [...current, { id: ++sequence, text: value.line }].slice(-5000));queueMicrotask(() => { if (follow && viewport) viewport.scrollTop = viewport.scrollHeight; }); });
    stream.addEventListener("failure", event => { const value = JSON.parse((event as MessageEvent).data) as { message: string };setError(value.message);setLive(false);stream.close(); });
    stream.addEventListener("end", () => { setLive(false);stream.close(); });stream.onerror = () => setLive(false);onCleanup(() => { stream.close();setLive(false); });
  });
  const filtered = createMemo(() => lines().filter(line => !search() || line.text.toLowerCase().includes(search().toLowerCase())));
  const start = createMemo(() => Math.max(0, Math.min(Math.floor(scroll() / 22) - 5, filtered().length - 64)));
  async function copy() { try { await navigator.clipboard.writeText(lines().map(line => line.text).join("\n"));setCopied(true); } catch { setError("The logs could not be copied"); } }
  return <div class="log-panel runtime-log-panel"><div class="log-heading"><span class={"status " + (live() ? "status-running" : "")}><span class="status-dot" />{live() ? "Live" : "Disconnected"}</span><Show when={props.application.containers.length > 1}><select class="input" aria-label="Log container" value={containerID()} onChange={e => setSelected(e.currentTarget.value)}><For each={props.application.containers}>{c => <option value={c.id}>{c.service || c.name}</option>}</For></select></Show><div class="search-field log-search"><Icon name="search" /><input aria-label="Search runtime logs" placeholder="Search logs" value={search()} onInput={e => { setSearch(e.currentTarget.value);if (viewport) viewport.scrollTop = 0; }} /></div><IconButton icon={copied() ? "check" : "copy"} label="Copy runtime logs" ghost onClick={() => void copy()} /></div>
    <Show when={error()}><ErrorNotice message={error()} /></Show><div ref={viewport} class="log-viewport mono" role="log" aria-label="Runtime logs" aria-live="off" tabindex="0" onScroll={e => { setScroll(e.currentTarget.scrollTop);follow = e.currentTarget.scrollHeight - e.currentTarget.scrollTop - e.currentTarget.clientHeight < 44; }}><div style={{ height: start() * 22 + "px" }} /><For each={filtered().slice(start(), start() + 64)}>{line => <div class="log-line"><span class="log-number">{line.id}</span><span>{line.text}</span></div>}</For><div style={{ height: Math.max(0, filtered().length - start() - 64) * 22 + "px" }} /><Show when={!lines().length}><div class="log-empty">{containerID() ? "Waiting for logs" : "No runtime logs"}</div></Show></div>
  </div>;
}
