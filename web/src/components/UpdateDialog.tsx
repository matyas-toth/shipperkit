import { For, Show, createMemo, createSignal, onCleanup, onMount } from "solid-js";
import type { UpdateStatus } from "../types";
import { Button, IconButton } from "./Controls";
import { docs } from "../docs";
import { Icon } from "./Icon";

const idle: UpdateStatus = { state: "queued", message: "Preparing the update", fromVersion: "", toVersion: "", startedAt: 0, finishedAt: 0, requestId: "", log: "" };
const steps = ["Prepare", "Download", "Checkpoint", "Install", "Verify"];

export default function UpdateDialog(props: { open: boolean; close: () => void; done: () => void; initial?: UpdateStatus }) {
  return <Show when={props.open}><DialogPanel close={props.close} done={props.done} initial={props.initial ?? idle} /></Show>;
}

function DialogPanel(props: { close: () => void; done: () => void; initial: UpdateStatus }) {
  const [status, setStatus] = createSignal(props.initial);
  let panel!: HTMLDivElement, timer: ReturnType<typeof setInterval> | undefined;
  const previous = document.activeElement as HTMLElement | null;
  const expectedRequest = props.initial.requestId;
  let stopped = false;

  const busy = createMemo(() => !["succeeded", "failed"].includes(status().state));
  const stage = createMemo(() => {
    switch (status().state) {
      case "queued": case "preparing": return 0;
      case "pulling": return 1;
      case "checkpoint": return 2;
      case "updating": case "starting": case "restarting": return 3;
      case "verifying": return 4;
      case "succeeded": return steps.length;
      default: return 3;
    }
  });
  const title = createMemo(() => status().state === "succeeded" ? "Shipper is up to date" : status().state === "failed" ? "Update failed" : status().state === "restarting" ? "Shipper is restarting" : "Updating Shipper");
  const message = createMemo(() => status().state === "restarting" ? "Shipper is restarting. This usually takes under a minute." : status().message || "Preparing the update");

  async function read() {
    try {
      const response = await fetch("/api/updates/status", { credentials: "same-origin", headers: { Accept: "application/json" } });
      if (response.status === 401) { setStatus({ ...status(), state: "failed", message: "Your session expired. Sign in again." }); stop(); return; }
      if (response.status === 503) { setStatus({ ...status(), state: "restarting", message: "" }); return; }
      if (!response.ok) { setStatus({ ...status(), state: "restarting", message: "" }); return; }
      const next = await response.json() as UpdateStatus;
      if (stopped) return;
      if (next.requestId && expectedRequest && next.requestId !== expectedRequest) return;
      setStatus(next);
      if (next.state === "succeeded" || next.state === "failed") { stop(); props.done(); }
    } catch {
      if (!stopped) setStatus({ ...status(), state: "restarting", message: "" });
    }
  }
  function stop() {
    stopped = true;
    if (timer) { clearInterval(timer); timer = undefined; }
  }
  function start() {
    stopped = false;
    void read();
    timer = setInterval(() => void read(), 1500);
  }
  onMount(() => {
    start();
    panel.querySelector<HTMLButtonElement>("button")?.focus();
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape") props.close();
      if (event.key !== "Tab") return;
      const nodes = Array.from(panel.querySelectorAll<HTMLElement>("button:not(:disabled),a[href],input,summary"));
      if (!nodes.length) return;
      const first = nodes[0], last = nodes[nodes.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener("keydown", key);
    onCleanup(() => { stop(); document.removeEventListener("keydown", key); previous?.focus(); });
  });
  onCleanup(stop);

  return <>
    <div class="drawer-scrim" onClick={props.close} />
    <div ref={panel} class="confirm-dialog update-dialog" role="dialog" aria-modal="true" aria-labelledby="update-title">
      <header>
        <div class="update-dialog-heading">
          <span classList={{ "update-orbit": true, "is-running": busy(), "is-done": status().state === "succeeded", "is-failed": status().state === "failed" }}>
            <Show when={status().state === "succeeded"} fallback={<Show when={status().state === "failed"} fallback={<span class="update-spinner" />}><Icon name="warning" /></Show>}><Icon name="check" /></Show>
          </span>
          <div><h2 id="update-title">{title()}</h2><Show when={status().fromVersion || status().toVersion}><p class="update-version-line"><span>{status().fromVersion || "Current release"}</span><Icon name="chevron" /><strong>{status().toVersion || "Next release"}</strong></p></Show></div>
        </div>
        <IconButton icon="close" label="Hide update status" ghost onClick={props.close} />
      </header>
      <p class="update-dialog-copy">{status().state === "succeeded" ? `Shipper ${status().toVersion} is ready. Reload to use the new release.` : status().state === "failed" ? message() : "Applications stay online while Shipper checkpoints its data and restarts the control plane."}</p>
      <Show when={status().state !== "succeeded" && status().state !== "failed"}>
        <ol class="update-steps" aria-label="Update progress">
          <For each={steps}>{(label, index) => <li data-state={index() < stage() ? "done" : index() === stage() ? "active" : "pending"} aria-current={index() === stage() ? "step" : undefined}>
            <span class="update-step-mark"><Show when={index() < stage()} fallback={<Show when={index() === stage()} fallback={<span />}><span class="update-step-dot" /></Show>}><Icon name="check" /></Show></span>
            <span>{label}</span>
          </li>}</For>
        </ol>
      </Show>
      <div class="update-status-line" role="status"><span classList={{ "status-dot": true, "is-running": busy() }} /><span>{message()}</span></div>
      <Show when={status().log}><details class="update-log"><summary>Update log</summary><pre>{status().log}</pre></details></Show>
      <Show when={status().state === "failed"}>
        <a class="docs-link update-dialog-docs" href={docs("updates/")} target="_blank" rel="noreferrer">Update troubleshooting</a>
      </Show>
      <div class="form-actions">
        <Button variant="ghost" onClick={props.close}>{busy() ? "Hide" : "Close"}</Button>
        <Show when={status().state === "succeeded"}><Button variant="primary" icon="refresh" onClick={() => location.reload()}>Reload Shipper</Button></Show>
      </div>
    </div>
  </>;
}
