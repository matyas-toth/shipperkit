import { For, Show, createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import type { Application } from "../types";
import { ErrorNotice, IconButton } from "./Controls";
import { Icon } from "./Icon";
import { downloadLogs } from "../logs";

export default function RuntimeLogs(props: { application: Application; containerId?: string }) {
  const [selected, setSelected] = createSignal(""), [lines, setLines] = createSignal<{ id: number; text: string }[]>([]), [search, setSearch] = createSignal(""), [error, setError] = createSignal(""), [live, setLive] = createSignal(false), [scroll, setScroll] = createSignal(0), [copied, setCopied] = createSignal(false);
  const [revision, setRevision] = createSignal("");
  const [paused,setPaused]=createSignal(false);
  const containerID = createMemo(() => props.application.containers.some(c => c.id === selected()) ? selected() : (props.application.containers.some(c => c.id === props.containerId) ? props.containerId! : props.application.containers[0]?.id || ""));
  const containerState = createMemo(() => props.application.containers.find(c => c.id === containerID())?.state);
  const followsRuntime = () => containerState() === "running" || containerState() === "restarting";
  let viewport!: HTMLDivElement, follow = true, sequence = 0;
  let buffer:{id:number;text:string}[]=[],frame=0;
  function flush(){frame=0;if(paused()||!buffer.length)return;const batch=buffer;buffer=[];setLines(current=>[...current,...batch].slice(-5000));queueMicrotask(()=>{if(follow&&viewport)viewport.scrollTop=viewport.scrollHeight;});}
  createEffect(()=>{if(!paused())flush();});
  createEffect(() => { const operation = props.application.latestDeployment;if (operation?.state === "succeeded" && ["start", "restart", "service_start", "service_restart"].includes(operation.operation)) setRevision(operation.id); });
  // ServicePanel derives a fresh application object on inventory updates. Keep
  // the connection tied to primitive identity, not that object's reference.
  const streamIdentity = createMemo(() => JSON.stringify([props.application.id, containerID(), containerState(), props.application.desiredState, revision()]));
  createEffect(() => {
    const [applicationId, id] = JSON.parse(streamIdentity()) as string[];
    if (!id) {setLines([]);setError("");setLive(false);return;}
    let disposed=false;
    const stream = new EventSource(`/api/applications/${applicationId}/logs?container=${encodeURIComponent(id)}&tail=200`);setError("");
    stream.onopen = () => {if(!disposed)setLive(true);};stream.addEventListener("reset", () => {if(disposed)return; setLines([]);sequence = 0;buffer=[];setScroll(0); });
    stream.addEventListener("log", event => {if(disposed)return; const value = JSON.parse((event as MessageEvent).data) as { line: string };buffer.push({id:++sequence,text:value.line});if(buffer.length>5000)buffer=buffer.slice(-5000);if(!frame)frame=requestAnimationFrame(flush); });
    stream.addEventListener("failure", event => {if(disposed)return; const value = JSON.parse((event as MessageEvent).data) as { message: string };setError(value.message);setLive(false);stream.close(); });
    stream.addEventListener("end", () => {if(disposed)return; setLive(false);stream.close(); });stream.onerror = () => {if(!disposed)setLive(false);};onCleanup(() => { disposed=true;stream.close();cancelAnimationFrame(frame);frame=0;buffer=[];setLive(false); });
  });
  const filtered = createMemo(() => lines().filter(line => !search() || line.text.toLowerCase().includes(search().toLowerCase())));
  const start = createMemo(() => Math.max(0, Math.min(Math.floor(scroll() / 22) - 5, filtered().length - 64)));
  async function copy() { try { await navigator.clipboard.writeText(filtered().map(line => line.text).join("\n"));setCopied(true); } catch { setError("The logs could not be copied"); } }
  return <div class="log-panel runtime-log-panel"><div class="log-heading"><span class={"status " + (followsRuntime() && live() ? "status-running" : "")}><span class="status-dot" />{!containerID() ? "No container" : !followsRuntime() ? "History" : paused() ? "Paused" : live() ? "Live" : "Disconnected"}</span><Show when={props.application.containers.length > 1}><select class="input" aria-label="Log container" value={containerID()} onChange={e => setSelected(e.currentTarget.value)}><For each={props.application.containers}>{c => <option value={c.id}>{c.service&&props.application.containers.filter(v=>v.service===c.service).length===1?c.service:c.name}</option>}</For></select></Show><div class="search-field log-search"><Icon name="search" /><input aria-label="Search runtime logs" placeholder="Search logs" value={search()} onInput={e => { setSearch(e.currentTarget.value);if (viewport) viewport.scrollTop = 0; }} /></div><Show when={followsRuntime()}><IconButton icon={paused()?"play":"pause"} label={paused()?"Resume runtime logs":"Pause runtime logs"} ghost onClick={()=>setPaused(v=>!v)}/></Show><IconButton icon="download" label="Download runtime logs" ghost onClick={()=>downloadLogs(filtered().map(l=>l.text).join("\n"),"runtime.log")}/><IconButton icon={copied() ? "check" : "copy"} label="Copy runtime logs" ghost onClick={() => void copy()} /></div>
    <Show when={error()}><ErrorNotice message={error()} /></Show><div ref={viewport} class="log-viewport mono" role="log" aria-label="Runtime logs" aria-live="off" tabindex="0" onScroll={e => { setScroll(e.currentTarget.scrollTop);follow = e.currentTarget.scrollHeight - e.currentTarget.scrollTop - e.currentTarget.clientHeight < 44; }}><div style={{ height: start() * 22 + "px" }} /><For each={filtered().slice(start(), start() + 64)}>{line => <div class="log-line"><span class="log-number">{line.id}</span><span>{line.text}</span></div>}</For><div style={{ height: Math.max(0, filtered().length - start() - 64) * 22 + "px" }} /><Show when={!lines().length}><div class="log-empty">{followsRuntime() ? "Waiting for logs" : "No runtime logs"}</div></Show></div>
  </div>;
}
