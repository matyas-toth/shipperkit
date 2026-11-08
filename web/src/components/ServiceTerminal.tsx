import { createEffect, createSignal, For, onCleanup, Show } from "solid-js";
import { post } from "../api";
import { useAuth } from "../context";
import type { Application, Container } from "../types";
import type { Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import "./ServiceTerminal.css";

interface TerminalTicket { id: string; ticket: string; expiresAt: number }
export default function ServiceTerminal(props: { application: Application; container?: Container }) {
  const auth = useAuth();
  const [selected, setSelected] = createSignal("");
  const [minutes, setMinutes] = createSignal(15);
  const [state, setState] = createSignal<"closed" | "opening" | "connected">("closed");
  const [error, setError] = createSignal("");
  let viewport!: HTMLDivElement;
  let term: Terminal | undefined;
  let fit: FitAddon | undefined;
  let socket: WebSocket | undefined;
  let current: { id: string; applicationId: string } | undefined;
  let observer: ResizeObserver | undefined;
  let generation = 0;
  let disposed = false;
  const containers = () => props.application.containers.filter(c => c.managed && c.state === "running" && c.applicationId === props.application.id);
  const send = (data: unknown) => { if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(data)); };
  const revoke = async (session: { id: string; applicationId: string }) => {
    await post(`/applications/${session.applicationId}/terminal/${session.id}/revoke`, {}, auth.session()?.csrfToken);
  };
  const disconnect = async () => {
    generation++;
    const session = current; current = undefined;
    const connection = socket; socket = undefined;
    observer?.disconnect(); observer = undefined;
    term?.dispose(); term = undefined; fit = undefined;
    setState("closed");
    try { if (session) await revoke(session); } catch (e) { if (!disposed) setError((e as Error).message); } finally { connection?.close(); }
  };
  createEffect(() => {
    const list = containers();
    const preferred = props.container?.id;
    const next = list.find(c => c.id === preferred)?.id ?? list.find(c => c.id === selected())?.id ?? list[0]?.id ?? "";
    if (next !== selected()) { void disconnect(); setSelected(next); }
    if (current && current.applicationId !== props.application.id) void disconnect();
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
      const style = getComputedStyle(viewport);
      term = new xterm.Terminal({
        cursorBlink: true, scrollback: 1500, convertEol: false, fontSize: 13,
        fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", monospace',
        theme: { background: style.getPropertyValue("--surface").trim() || "#212121", foreground: style.getPropertyValue("--text").trim() || "#f2f2f2", cursor: "#f2f2f2", selectionBackground: "#676767" },
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
        if (message.type === "end") { setError(message.reason); ws.close(); }
      };
      ws.onerror = () => { if (attempt === generation) setError("The terminal connection failed"); };
      ws.onclose = () => { if (attempt === generation) { current = undefined; setState("closed"); term?.options && (term.options.disableStdin = true); } };
      term.onData(data => {
        // Bound paste frames while preserving ordinary keyboard and control input.
        for (let offset = 0; offset < data.length; offset += 2048) send({ type: "input", data: data.slice(offset, offset + 2048) });
      });
      observer = new ResizeObserver(() => { fit?.fit(); send({ type: "resize", cols: Math.min(300, Math.max(2, term?.cols ?? 80)), rows: Math.min(120, Math.max(2, term?.rows ?? 24)) }); });
      observer.observe(viewport);
    } catch (e) { if (attempt === generation) { setError((e as Error).message); await disconnect(); } }
  };
  const changeContainer = (id: string) => { void disconnect(); setError(""); setSelected(id); };
  return <section class="service-terminal">
    <div class="service-terminal-controls">
      <select class="input" aria-label="Terminal container" value={selected()} onChange={event => changeContainer(event.currentTarget.value)} disabled={state() === "opening"}>
        <Show when={containers().length > 0} fallback={<option value="">No running containers</option>}>
          <For each={containers()}>{container => <option value={container.id}>{container.service || container.name}</option>}</For>
        </Show>
      </select>
      <select class="input terminal-timeout" aria-label="Terminal timeout" value={minutes()} disabled={state() !== "closed"} onChange={event => setMinutes(Number(event.currentTarget.value))}>
        <option value={5}>5 minutes</option><option value={15}>15 minutes</option><option value={30}>30 minutes</option>
      </select>
      <Show when={state() === "connected"} fallback={<button class="button" disabled={!selected() || state() === "opening"} onClick={() => { term?.dispose(); term = undefined; observer?.disconnect(); void open(); }}>{state() === "opening" ? "Opening terminal…" : "Open terminal"}</button>}>
        <button class="button" onClick={() => void disconnect()}>Close terminal</button>
      </Show>
    </div>
    <Show when={error()}><p class="error" role="alert">{error()}</p></Show>
    <div class="service-terminal-viewport" classList={{ "terminal-active": state() !== "closed" || !!term }} ref={viewport} role="region" aria-label="Interactive container terminal" />
  </section>;
}
