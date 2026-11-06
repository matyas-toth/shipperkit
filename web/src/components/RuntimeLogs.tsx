import { For, Show, batch, createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import type { Application } from "../types";
import { Button, ErrorNotice, IconButton, Status } from "./Controls";
import { Icon } from "./Icon";
import { downloadLogs } from "../logs";
import LogViewport, { logPresentation } from "./LogViewport";

type RuntimeLine = { id: number; text: string; timestamp: string; message: string; tone?: "warning" | "error" };
const timestampPrefix = /^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\s+/;
const severityPrefix = /^\[?(WARNING|WARN|ERROR|ERR|FATAL|PANIC|CRITICAL)\b/i;
const failedRequest = /\b(?:GET|HEAD|POST|PUT|PATCH|DELETE|OPTIONS|CONNECT)\s+\S+\s+HTTP\/\d(?:\.\d)?"?\s+5\d\d\b/;
function runtimeLine(id: number, text: string): RuntimeLine {
  const timestamp = timestampPrefix.exec(text)?.[0] ?? "";
  const message = text.slice(timestamp.length);
  const level = severityPrefix.exec(message)?.[1].toUpperCase();
  const tone = level ? level.startsWith("WARN") ? "warning" : "error" : failedRequest.test(message) ? "error" : undefined;
  return { id, text, timestamp, message, tone };
}

export default function RuntimeLogs(props: { application: Application; containerId?: string }) {
  const [selected, setSelected] = createSignal(""), [lines, setLines] = createSignal<RuntimeLine[]>([]), [search, setSearch] = createSignal(""), [error, setError] = createSignal(""), [connection, setConnection] = createSignal("connecting"), [copied, setCopied] = createSignal(false), [issues, setIssues] = createSignal(false);
  const [revision, setRevision] = createSignal("");
  const [revealed, setRevealed] = createSignal<number>();
  const [paused,setPaused]=createSignal(false), [attempt, retry] = createSignal(0);
  const presentation = logPresentation();
  const containerID = createMemo(() => props.application.containers.some(c => c.id === selected()) ? selected() : (props.application.containers.some(c => c.id === props.containerId) ? props.containerId! : props.application.containers[0]?.id || ""));
  const containerState = createMemo(() => props.application.containers.find(c => c.id === containerID())?.state);
  const followsRuntime = () => containerState() === "running" || containerState() === "restarting";
  let viewport!: HTMLDivElement, follow = true, sequence = 0;
  let buffer:RuntimeLine[]=[],frame=0;
  function flush(){frame=0;if(paused()||!buffer.length)return;const batch=buffer;buffer=[];setLines(current=>[...current,...batch].slice(-5000));queueMicrotask(()=>{if(follow&&viewport)viewport.scrollTop=viewport.scrollHeight;});}
  createEffect(()=>{if(!paused())flush();});
  createEffect(() => { const operation = props.application.latestDeployment;if (operation?.state === "succeeded" && ["start", "restart", "service_start", "service_restart"].includes(operation.operation)) setRevision(operation.id); });
  // ServicePanel derives a fresh application object on inventory updates. Keep
  // the connection tied to primitive identity, not that object's reference.
  const streamIdentity = createMemo(() => JSON.stringify([props.application.id, containerID(), containerState(), props.application.desiredState, revision(), attempt()]));
  createEffect(() => {
    const [applicationId, id] = JSON.parse(streamIdentity()) as string[];
    if (!id) {setLines([]);setError("");setConnection("connecting");return;}
    let disposed=false;
    const stream = new EventSource(`/api/applications/${applicationId}/logs?container=${encodeURIComponent(id)}&tail=200`);setError("");setConnection("connecting");
    stream.onopen = () => {if(!disposed)setConnection("live");};stream.addEventListener("reset", () => {if(disposed)return; setLines([]);setRevealed(undefined);sequence = 0;buffer=[];if (viewport) viewport.scrollTop = 0; });
    stream.addEventListener("log", event => {if(disposed)return; const value = JSON.parse((event as MessageEvent).data) as { line: string };buffer.push(runtimeLine(++sequence,value.line));if(buffer.length>5000)buffer=buffer.slice(-5000);if(!frame)frame=requestAnimationFrame(flush); });
    stream.addEventListener("failure", event => {if(disposed)return; const value = JSON.parse((event as MessageEvent).data) as { message: string };setError(value.message);setConnection("failed");stream.close(); });
    stream.addEventListener("end", () => {if(disposed)return; setConnection("ended");stream.close(); });stream.onerror = () => {if(!disposed)setConnection("reconnecting");};onCleanup(() => { disposed=true;stream.close();cancelAnimationFrame(frame);frame=0;buffer=[]; });
  });
  const filtered = createMemo(() => {
    const term = search().toLowerCase();
    return !term && !issues() ? lines() : lines().filter(line => (!issues() || line.tone) && (!term || line.text.toLowerCase().includes(term)));
  });
  const issueCount = createMemo(() => lines().reduce((count, line) => count + Number(!!line.tone), 0));
  const mode = () => revealed() !== undefined ? "context" : issues() ? "issues" : "all";
  function scrollToResults() { setRevealed(undefined);  if (viewport) viewport.scrollTop = 0; }
  function selectMode(showIssues: boolean) { batch(() => { setIssues(showIssues); scrollToResults(); }); }
  function showContext(line: RuntimeLine) {
    follow = false;
    batch(() => { setSearch(""); setIssues(false); setRevealed(line.id); });
  }
  async function copy() { try { await navigator.clipboard.writeText(filtered().map(line => line.text).join("\n"));setCopied(true); } catch { setError("The logs could not be copied"); } }
  const logStatus = () => !containerID() ? "No container" : !followsRuntime() ? "Log history" :
    paused() ? "Logs paused" : ({
      live: "Live logs", connecting: "Connecting to logs", reconnecting: "Reconnecting logs",
      ended: "Log stream ended", failed: "Log stream disconnected",
    }[connection()] ?? "Log stream disconnected");
  const status = () => connection() === "failed" && followsRuntime() ? "failed" :
    followsRuntime() && !paused() && connection() === "live" ? "running" :
    followsRuntime() && ["reconnecting", "ended"].includes(connection()) ? "warning" : "unknown";
  return <div class="log-panel runtime-log-panel">
    <div class="log-heading">
      <div class="log-context runtime-log-status">
        <Status value={status()} label={logStatus()} />
        <Show when={followsRuntime() && ["failed", "ended", "reconnecting"].includes(connection())}>
          <Button small icon="refresh" onClick={() => retry(n => n + 1)}>Reconnect</Button>
        </Show>
        <span class="log-result-count">{search() || issues() ? filtered().length + " of " : ""}{lines().length} {lines().length === 1 ? "line" : "lines"}</span>
        <Show when={props.application.containers.length > 1}>
          <select class="input" aria-label="Log container" value={containerID()} onChange={e => setSelected(e.currentTarget.value)}>
            <For each={props.application.containers}>{c =>
              <option value={c.id}>{c.service && props.application.containers.filter(v => v.service === c.service).length === 1 ? c.service : c.name}</option>
            }</For>
          </select>
        </Show>
      </div>
      <div class="log-tools">
        <div class="runtime-modes" role="group" aria-label="Runtime log view">
          <Button small variant="ghost" aria-pressed={mode() === "all"} onClick={() => selectMode(false)}>All</Button>
          <Button small variant="ghost" aria-label="Show warnings and errors" aria-pressed={mode() === "issues"} onClick={() => selectMode(true)}>Issues <span>{issueCount()}</span></Button>
        </div>
        <div class="search-field log-search"><Icon name="search" /><input aria-label="Search runtime logs" placeholder="Search logs" value={search()} onInput={e => { setSearch(e.currentTarget.value); scrollToResults(); }} /></div>
        <Button icon="wrap" small variant="ghost" aria-label="Wrap logs" aria-pressed={presentation.wrap()} onClick={presentation.toggleWrap}>Wrap</Button>
        <Show when={followsRuntime()}><IconButton icon={paused() ? "play" : "pause"} label={paused() ? "Resume runtime logs" : "Pause runtime logs"} ghost onClick={() => setPaused(v => !v)} /></Show>
        <IconButton icon="download" label="Download runtime logs" ghost onClick={() => downloadLogs(filtered().map(l => l.text).join("\n"), "runtime.log")} />
        <IconButton icon={copied() ? "check" : "copy"} label="Copy runtime logs" ghost onClick={() => void copy()} />
      </div>
    </div>
    <Show when={revealed() !== undefined}>
      <div class="log-context-bar"><strong>Context for line {revealed()}</strong>
        <Button small variant="ghost" icon="back" onClick={() => selectMode(true)}>Back to issues</Button>
      </div>
    </Show>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <LogViewport label="Runtime logs" rows={filtered()} lineNumber={line => line.id} message={line => line.text} wrap={presentation.wrap()}
      tone={line => line.tone} renderMessage={line => <><span class="runtime-timestamp">{line.timestamp}</span><span class="runtime-message">{line.message}</span></>}
      reveal={revealed()} activate={issues() ? showContext : undefined}
      empty={!lines().length ? followsRuntime() ? "Waiting for logs" : "No runtime logs" : !filtered().length ? issues() && !search() ? "No warnings or errors" : "No matching logs" : undefined}
      ref={node => viewport = node} onFollow={value => follow = value} />
  </div>;
}
