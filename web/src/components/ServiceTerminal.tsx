import { createEffect, createSignal, For, onCleanup, Show } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import { operationTime } from "../data";
import type { Application, Container, TerminalAudit } from "../types";
import type { Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import { Button } from "./Controls";
import { Icon } from "./Icon";
import "@xterm/xterm/css/xterm.css";
import "./ServiceTerminal.css";

interface TerminalTicket { id: string; ticket: string; expiresAt: number }
const auditLabel = (entry: TerminalAudit) => entry.event === "start" ? "Session opened" : entry.event === "revoke" ? "Session revoked" : entry.reason === "session_expired" ? "Session expired" : entry.reason === "ticket_expired" ? "Session expired" : "Session closed";
const auditReason = (entry: TerminalAudit) => entry.reason && !["user_revoked", "session_expired", "ticket_expired"].includes(entry.reason) ? entry.reason.replaceAll("_", " ") : "";
export default function ServiceTerminal(props: { application: Application; container?: Container }) {
  const auth = useAuth();
  const [selected, setSelected] = createSignal("");
  const [minutes, setMinutes] = createSignal(15);
  const [state, setState] = createSignal<"closed" | "opening" | "connected">("closed");
  const [error, setError] = createSignal("");
  const [expiresAt, setExpiresAt] = createSignal(0);
  const [currentId, setCurrentId] = createSignal("");
  const [now, setNow] = createSignal(Math.floor(Date.now() / 1000));
  const remaining = () => Math.max(0, expiresAt() - now());
  const remainingLabel = () => { const seconds = remaining(); return seconds >= 60 ? "Expires in " + Math.ceil(seconds / 60) + " minutes" : "Expires in " + seconds + "s"; };
  let viewport!: HTMLDivElement;
  let term: Terminal | undefined;
  let fit: FitAddon | undefined;
  let socket: WebSocket | undefined;
  let current: { id: string; applicationId: string } | undefined;
  let observer: ResizeObserver | undefined;
  let generation = 0;
  let disposed = false;
  const audit = createQuery(() => ({ queryKey: ["applications", props.application.id, "terminal", "audit"], queryFn: () => api<{ items: TerminalAudit[] }>(`/applications/${props.application.id}/terminal/audit`), refetchInterval: 15_000 }));
  const containers = () => props.application.containers.filter(c => c.managed && c.state === "running" && c.applicationId === props.application.id);
  const selectedContainer = () => containers().find(c => c.id === selected());
  const selectedName = () => selectedContainer()?.service || selectedContainer()?.name || "No running container";
  const containerName = (id: string) => props.application.containers.find(c => c.id === id)?.name ?? id.slice(0, 12);
  const auditDuration = (entry: TerminalAudit) => {
    if (entry.event === "start") return "";
    const start = audit.data?.items.filter(item => item.sessionId === entry.sessionId && item.event === "start").sort((a, b) => a.createdAt - b.createdAt)[0]?.createdAt;
    if (!start || entry.createdAt <= start) return "";
    const seconds = entry.createdAt - start;
    return seconds >= 60 ? Math.round(seconds / 60) + " min" : seconds + "s";
  };
  const send = (data: unknown) => { if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(data)); };
  const revoke = async (session: { id: string; applicationId: string }) => {
    await post(`/applications/${session.applicationId}/terminal/${session.id}/revoke`, {}, auth.session()?.csrfToken);
  };
  function clearTerminal() {
    observer?.disconnect(); observer = undefined;
    term?.dispose(); term = undefined; fit = undefined;
  }
  const disconnect = async () => {
    generation++;
    const session = current; current = undefined;
    const connection = socket; socket = undefined;
    clearTerminal();
    setState("closed");
    setExpiresAt(0);
    setCurrentId("");
    try { if (session) await revoke(session); } catch (e) { if (!disposed) setError((e as Error).message); } finally { connection?.close(); if (session) void audit.refetch(); }
  };
  createEffect(() => {
    const list = containers();
    const preferred = props.container?.id;
    const next = list.find(c => c.id === preferred)?.id ?? list.find(c => c.id === selected())?.id ?? list[0]?.id ?? "";
    if (next !== selected()) { void disconnect(); setSelected(next); }
    if (current && current.applicationId !== props.application.id) void disconnect();
  });
  createEffect(() => {
    if (state() !== "connected" || !expiresAt()) return;
    setNow(Math.floor(Date.now() / 1000));
    const timer = setInterval(() => setNow(Math.floor(Date.now() / 1000)), 15_000);
    onCleanup(() => clearInterval(timer));
  });
  onCleanup(() => { disposed = true; void disconnect(); });
  const open = async () => {
    if (state() !== "closed" || !selected()) return;
    setState("opening"); setError("");
    const attempt = ++generation;
    const applicationId = props.application.id;
    try {
      // Load the emulator alongside permission checks to keep opening responsive.
      const [ticket, xterm, addon] = await Promise.all([
        post<TerminalTicket>(`/applications/${applicationId}/terminal`, { containerId: selected(), minutes: minutes() }, auth.session()?.csrfToken),
        import("@xterm/xterm"), import("@xterm/addon-fit"),
      ]);
      if (disposed || attempt !== generation) { await revoke({ id: ticket.id, applicationId }); return; }
      current = { id: ticket.id, applicationId };
      setCurrentId(ticket.id);
      setExpiresAt(ticket.expiresAt);
      const style = getComputedStyle(viewport);
      term = new xterm.Terminal({
        cursorBlink: true, scrollback: 1500, convertEol: false, fontSize: 14, lineHeight: 1.35,
        fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", monospace',
        theme: { background: style.getPropertyValue("--deep").trim() || "#0e0f0f", foreground: style.getPropertyValue("--text").trim() || "#f2f2f2", cursor: "#f2f2f2", selectionBackground: "#676767" },
        allowProposedApi: false,
      });
      fit = new addon.FitAddon(); term.loadAddon(fit); term.open(viewport); fit.fit();
      const ws = new WebSocket(`${location.protocol === "https:" ? "wss:" : "ws:"}//${location.host}/api/applications/${applicationId}/terminal/${ticket.id}`);
      socket = ws;
      ws.onopen = () => send({ type: "connect", ticket: ticket.ticket, cols: Math.min(300, Math.max(2, term?.cols ?? 80)), rows: Math.min(120, Math.max(2, term?.rows ?? 24)) });
      ws.onmessage = event => {
        if (attempt !== generation) return;
        const message = JSON.parse(event.data);
        if (message.type === "ready") { setState("connected"); term?.focus(); }
        if (message.type === "output") term?.write(Uint8Array.from(atob(message.data), character => character.charCodeAt(0)));
        if (message.type === "end") { setError(message.reason); void audit.refetch(); ws.close(); }
      };
      ws.onerror = () => { if (attempt === generation) setError("The terminal connection failed"); };
      ws.onclose = () => { if (attempt === generation) { current = undefined; setCurrentId(""); setExpiresAt(0); setState("closed"); } };
      term.onData(data => {
        // Bound paste frames while preserving ordinary keyboard and control input.
        for (let offset = 0; offset < data.length; offset += 2048) send({ type: "input", data: data.slice(offset, offset + 2048) });
      });
      observer = new ResizeObserver(() => { fit?.fit(); send({ type: "resize", cols: Math.min(300, Math.max(2, term?.cols ?? 80)), rows: Math.min(120, Math.max(2, term?.rows ?? 24)) }); });
      observer.observe(viewport);
    } catch (e) { if (attempt === generation) { setError((e as Error).message); await disconnect(); } }
  };
  function start() {
    if (state() !== "closed" || !selected()) return;
    clearTerminal();
    void open();
  }
  const changeContainer = (id: string) => { void disconnect(); setError(""); setSelected(id); };
  return <section class="terminal-panel">
    <div class="terminal-toolbar">
      <label class="terminal-field"><span>Container</span>
        <Show when={containers().length > 1} fallback={<span class="terminal-container-static">{selectedName()}</span>}>
          <select class="input" aria-label="Terminal container" value={selected()} onChange={event => changeContainer(event.currentTarget.value)} disabled={state() !== "closed"}>
            <Show when={containers().length > 0} fallback={<option value="">No running containers</option>}>
              <For each={containers()}>{container => <option value={container.id}>{container.service || container.name}</option>}</For>
            </Show>
          </select>
        </Show>
      </label>
      <label class="terminal-field"><span>Session lifetime</span>
        <select class="input terminal-timeout" aria-label="Session lifetime" value={minutes()} disabled={state() !== "closed"} onChange={event => setMinutes(Number(event.currentTarget.value))}>
          <option value={5}>5 minutes</option><option value={15}>15 minutes</option><option value={30}>30 minutes</option>
        </select>
      </label>
    </div>
    <Show when={error() && state() !== "closed"}><div class="terminal-notice" role="alert"><Icon name="warning" /><span>{error()}</span></div></Show>
    <Show when={state() !== "closed"} fallback={<div class="terminal-frame terminal-frame-idle" classList={{ "is-ended": !!error() }} role={error() ? "alert" : undefined}>
      <header><span class="terminal-frame-dot" /><span class="terminal-frame-state">{error() ? "Session expired" : "Not connected"}</span></header>
      <div class="terminal-idle-body">
        <Show when={error()}><span class="terminal-idle-reason">{error() === "Terminal session expired" ? "Session expired after " + minutes() + " minutes." : error()}</span></Show>
        <Button variant="primary" icon={error() ? "refresh" : "terminal"} disabled={!selected()} onClick={start}>{error() ? "Reconnect" : "Open terminal"}</Button>
      </div>
    </div>}>
      <div class="terminal-frame">
        <header><span class="terminal-frame-dot" classList={{ live: state() === "connected" }} /><span class="terminal-frame-state">{state() === "connected" ? "Live" : "Opening"}</span><code class="terminal-frame-container">{selectedName()}</code><Show when={state() === "connected" && remaining() > 0}><span class="terminal-remaining">{remainingLabel()}</span></Show><Button small variant="ghost" disabled={state() !== "connected"} onClick={() => void disconnect()}>End session</Button></header>
        <div class="service-terminal-viewport" ref={viewport} role="region" aria-label="Interactive container terminal" />
      </div>
    </Show>
    <Show when={audit.data?.items.length}><details class="configuration terminal-audit">
      <summary>Session history<Show when={audit.data?.items.length}>{count => <span class="terminal-audit-count">{count()} {count() === 1 ? "session" : "sessions"}</span>}</Show></summary>
      <div class="terminal-audit-list"><div class="terminal-audit-head" classList={{ "single-container": containers().length <= 1 }}><span>Event</span><Show when={containers().length > 1}><span>Container</span></Show><span>Duration</span><span>Time</span></div><For each={audit.data!.items.slice(0, 8)}>{entry =>
        <div class="terminal-audit-row" classList={{ "single-container": containers().length <= 1 }}>
          <span class="terminal-audit-event" data-event={entry.event}>{auditLabel(entry)}<Show when={currentId() === entry.sessionId}><span class="terminal-audit-current">Current</span></Show></span>
          <Show when={containers().length > 1}><code>{containerName(entry.containerId)}</code></Show>
          <Show when={auditReason(entry) || auditDuration(entry)}><span class="terminal-audit-reason">{[auditReason(entry), auditDuration(entry)].filter(Boolean).join(" · ")}</span></Show>
          <time datetime={new Date(entry.createdAt * 1000).toISOString()}>{operationTime(entry.createdAt)}</time>
        </div>
      }</For></div>
    </details></Show>
  </section>;
}
