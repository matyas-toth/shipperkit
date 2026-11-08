import { Show, createEffect, createMemo, createSignal } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import type { Deployment, LogLine } from "../types";
import { operationTime, deploymentLabel, deploymentRef, logDateTime, operationPending } from "../data";
import { Icon } from "./Icon";
import { downloadLogs } from "../logs";
import { Button, ErrorNotice, IconButton, Status } from "./Controls";
import LogViewport, { logPresentation } from "./LogViewport";

export default function LogViewer(props: {
  deployment: Deployment;
  current?: Deployment;
  diagnostics?: () => void;
  cancel?: () => void;
  retry?: () => void;
  restore?: () => void;
  onTerminalLog?: (log: LogLine) => void;
}) {
  const [lines, setLines] = createSignal<LogLine[]>([]), [cursor, setCursor] = createSignal(0);
  const [search, setSearch] = createSignal(""), [copied, setCopied] = createSignal(false), [copyError, setCopyError] = createSignal("");
  const [paused, setPaused] = createSignal(false);
  const [contextBusy, setContextBusy] = createSignal(false), [contextCopied, setContextCopied] = createSignal(false);
  const presentation = logPresentation();
  let viewport!: HTMLDivElement, follow = true;
  const query = createQuery(() => ({
    queryKey: ["deployment-logs", props.deployment.id, cursor()],
    queryFn: () => api<{ items: LogLine[] }>(`/applications/${props.deployment.applicationId}/deployments/${props.deployment.id}/logs?after=${cursor()}`),
    enabled: !paused(), refetchInterval: operationPending(props.deployment.state) ? 5000 : false, gcTime: 0,
  }));
  createEffect(() => {
    if (paused()) return;
    const batch = query.data?.items;
    if (batch?.length) {
      const last = batch[batch.length - 1].sequence;
      if (last <= cursor()) return;
      setLines(current => [...current, ...batch.filter(line => line.sequence > cursor())].slice(-10000));
      setCursor(last);
      queueMicrotask(() => { if (follow && viewport) viewport.scrollTop = viewport.scrollHeight; });
    }
  });
  const terminal = createMemo(() => lines().find(line => line.step === "failed" || line.step === "cancelled" || line.step === "interrupted"));
  createEffect(() => {
    const log = terminal();
    if (log && operationPending(props.deployment.state)) props.onTerminalLog?.(log);
  });
  const state = () => operationPending(props.deployment.state) ? terminal()?.step ?? props.deployment.state : props.deployment.state;
  const message = () => props.deployment.error || (terminal()?.step === state() ? terminal()?.line : "");
  const failedStep = createMemo(() => [...lines()].reverse().find(line => !["failed", "cancelled", "interrupted"].includes(line.step))?.step);
  const filtered = createMemo(() => {
    const text = search().toLowerCase();
    return text ? lines().filter(line => (line.line + " " + line.step).toLowerCase().includes(text)) : lines();
  });
  const logText = () => filtered().map(line => logDateTime(line.createdAt) + " " + line.step + " " + line.line).join("\n");
  const cleanupMarker = "Candidate cleanup failed:";
  const cleanupFailure = () => lines().find(line => line.line.startsWith(cleanupMarker));
  const retryable = () => props.retry && ["deploy", "apply"].includes(props.deployment.operation) &&
    ["failed", "cancelled"].includes(props.deployment.state) &&
    (!!props.deployment.commit || !!props.deployment.config.localSource || props.deployment.config.method === "inline" ||
      props.deployment.config.method === "image" && !!(props.deployment.digest || props.deployment.config.image?.includes("@sha256:")));
  const cancellable = () => props.cancel && ["deploy", "apply"].includes(props.deployment.operation) &&
    ["queued", "cloning", "building", "packaging", "pulling"].includes(state());
  function filter(value: string) { setSearch(value); if (viewport) viewport.scrollTop = 0; }
  async function copy() {
    try { await navigator.clipboard.writeText(logText()); setCopied(true); }
    catch { setCopyError("The logs could not be copied"); }
  }
  function details(line: LogLine) {
    return <><time class="log-timestamp" datetime={new Date(line.createdAt * 1000).toISOString()} title={new Date(line.createdAt * 1000).toString()}>{logDateTime(line.createdAt)}</time><span class="log-step">{line.step}</span></>;
  }
  async function copyContext() {
    if (contextBusy()) return;
    setContextBusy(true); setCopyError(""); setContextCopied(false);
    try {
      const context = await api<unknown>(`/applications/${props.deployment.applicationId}/deployments/${props.deployment.id}/context`);
      const text = JSON.stringify(context, null, 2);
      if (navigator.clipboard) await navigator.clipboard.writeText(text);
      else {
        const field = document.createElement("textarea"), focused = document.activeElement as HTMLElement | null;
        field.value = text; field.style.cssText = "position:fixed;opacity:0;pointer-events:none";
        document.body.append(field); field.select();
        try { if (!document.execCommand("copy")) throw Error("Diagnostic context could not be copied"); }
        finally { field.remove(); focused?.focus({ preventScroll: true }); }
      }
      setContextCopied(true);
    } catch (e) { setCopyError((e as Error).message || "Diagnostic context could not be copied"); }
    finally { setContextBusy(false); }
  }

  return <div class="deployment-detail">
    <section class="release-summary" aria-label="Selected release">
      <div>
        <div class="log-context">
          <strong class="mono log-commit" title={props.deployment.commit || props.deployment.id}>{deploymentRef(props.deployment)}</strong>
          <Status value={state()} label={deploymentLabel({ ...props.deployment, state: state() })} />
          <time class="muted log-date" datetime={new Date(props.deployment.createdAt * 1000).toISOString()} title={operationTime(props.deployment.createdAt, true)}>{operationTime(props.deployment.createdAt)}</time>
        </div>
      </div>
      <Show when={props.current?.id !== props.deployment.id}><div class="release-serving">
        <Show when={props.current} fallback="No current release">{current =>
          <><span>Serving release</span><code title={current().commit || current().id}>{deploymentRef(current())}</code></>
        }</Show>
      </div></Show>
      <Show when={cancellable() || props.restore || retryable()}>
        <div class="log-operation-actions">
          <Show when={cancellable()}><Button small disabled={props.deployment.cancelRequested} onClick={props.cancel}>{props.deployment.cancelRequested ? "Cancelling" : "Cancel"}</Button></Show>
          <Show when={props.restore}><Button small icon="back" onClick={props.restore}>Restore release</Button></Show>
          <Show when={retryable()}><Button small variant="primary" onClick={props.retry}>Retry release</Button></Show>
        </div>
      </Show>
    </section>
    <Show when={message() || state() === "failed"}>
      <section class="deployment-error deployment-result" role="alert">
        <div class="deployment-result-body">
          <div class="deployment-result-title"><Icon name="warning" /><strong>{state() === "failed" ? "Deployment failed" : deploymentLabel({ ...props.deployment, state: state() })}</strong></div>
          <p>{message()}</p>
          <Show when={state() === "failed"}>
            <dl class="deployment-result-facts">
              <Show when={failedStep()}>{step => <div><dt>Last stage</dt><dd>{step().replaceAll("_", " ")}</dd></div>}</Show>
              <Show when={cleanupFailure()}>{cleanup => <div class="cleanup-result"><dt>Candidate container</dt><dd>{cleanup().line.includes("nothing was removed") ? "Cleanup failed and nothing was removed" : "Cleanup failed"} <button type="button" class="inline-link" onClick={() => filter(cleanupMarker)}>View log</button></dd></div>}</Show>
            </dl>
          </Show>
        </div>
        <div class="deployment-result-actions">
          <Button small variant="primary" icon={contextCopied() ? "check" : "copy"} disabled={contextBusy()} onClick={() => void copyContext()}>{contextBusy() ? "Preparing context" : contextCopied() ? "Context copied" : "Copy diagnostic context"}</Button>
          <Show when={failedStep()}><Button small variant="ghost" onClick={() => filter(failedStep()!)}>Show failed step</Button></Show>
          <Show when={props.diagnostics && state() === "failed"}><Button small variant="ghost" icon="terminal" onClick={props.diagnostics}>Health details</Button></Show>
        </div>
      </section>
    </Show>
    <div class="log-panel deployment-log-panel" style={{ "--terminal-log-height": operationPending(state()) ? undefined : Math.max(144, filtered().length * (presentation.wrap() ? 42 : 22) + 24) + "px" }}>
      <div class="log-heading">
        <strong class="log-title">Deployment logs</strong>
        <div class="log-tools">
          <div class="search-field log-search"><Icon name="search" /><input aria-label="Search logs" placeholder="Search logs" value={search()} onInput={e => filter(e.currentTarget.value)} /></div>
          <Button icon="wrap" small variant="ghost" aria-label="Wrap logs" aria-pressed={presentation.wrap()} onClick={presentation.toggleWrap}>Wrap</Button>
          <Button icon="sliders" small variant="ghost" aria-label="Log timestamps and steps" aria-pressed={presentation.details()} onClick={presentation.toggleDetails}>Details</Button>
          <Show when={operationPending(state())}><IconButton icon={paused() ? "play" : "pause"} label={paused() ? "Resume deployment logs" : "Pause deployment logs"} ghost onClick={() => setPaused(v => !v)} /></Show>
          <IconButton icon="download" label="Download deployment logs" ghost onClick={() => downloadLogs(logText(), "deployment.log")} />
          <IconButton icon={copied() ? "check" : "copy"} label="Copy logs" ghost onClick={() => void copy()} />
        </div>
        <Show when={search()}><span class="log-result-count" role="status">{filtered().length} of {lines().length} lines</span></Show>
      </div>
      <Show when={query.error || copyError()}><ErrorNotice message={copyError() || query.error!.message} /></Show>
      <LogViewport label="Deployment logs" rows={filtered()} lineNumber={line => line.number} message={line => line.line}
        tone={line => /(^|\s)(error|failed|failure|fatal|panic)\b/i.test(line.line) || /^(failed|failure)$/i.test(line.step) ? "error" : undefined}
        details={presentation.details() ? details : undefined} wrap={presentation.wrap()}
        empty={!lines().length ? query.isPending ? "Loading logs" : operationPending(state()) ? "Waiting for deployment" : "No deployment logs" : !filtered().length ? "No matching logs" : undefined}
        ref={node => viewport = node} onFollow={value => follow = value} />
    </div>
  </div>;
}
