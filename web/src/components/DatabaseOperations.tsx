import { Show, createSignal, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import { useNavigate } from "@solidjs/router";
import { useQueryClient } from "@tanstack/solid-query";
import { api } from "../api";
import { useAuth } from "../context";
import type { Application } from "../types";
import { Button, ErrorNotice, IconButton } from "./Controls";
import { Icon } from "./Icon";
import "./DatabaseResource.css";

export default function DatabaseOperations(props: { application: Application; openTerminal?: () => void }) {
  const auth = useAuth(), client = useQueryClient(), navigate = useNavigate();
  const [open, setOpen] = createSignal(false), [busy, setBusy] = createSignal(false), [error, setError] = createSignal("");
  const [file, setFile] = createSignal<File>(), [target, setTarget] = createSignal(""), [confirm, setConfirm] = createSignal(""), [revision, setRevision] = createSignal("");
  const base = () => `/applications/${props.application.id}/database`;
  const redis = () => props.application.config.database?.engine === "redis";
  const importLabel = () => redis() ? "Restore snapshot" : "Create from dump";
  const defaultTarget = () => ((props.application.name.toLowerCase().replace(/[^a-z0-9_]+/g, "_").replace(/^[^a-z_]+/, "").slice(0, 48)) || "database") + "_copy";
  async function showImport() {
    setError(""); setBusy(true);
    try {
      const review = await api<{ revision: string }>(`/applications/${props.application.id}/protection`);
      setRevision(review.revision);
      if (!redis() && !target()) setTarget(defaultTarget());
      setOpen(true);
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function downloadCA() {
    setBusy(true); setError("");
    try {
      const response = await fetch("/api" + base() + "/ca", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": auth.session()?.csrfToken ?? "" }, body: "{}" });
      if (!response.ok) { const body = await response.json(); throw new Error(body.error?.message ?? body.message ?? "CA download failed"); }
      const url = URL.createObjectURL(await response.blob()), link = document.createElement("a");
      link.href = url; link.download = "database-ca.crt"; link.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function importDump(event: SubmitEvent) {
    event.preventDefault();
    if (!file() || busy()) return;
    setBusy(true); setError("");
    try {
      const form = new FormData();
      form.append("file", file()!);
      form.append("target", target());
      form.append("confirm", confirm());
      form.append("revision", revision());
      const response = await fetch("/api" + base() + "/import", { method: "POST", credentials: "same-origin", headers: { "X-CSRF-Token": auth.session()?.csrfToken ?? "" }, body: form });
      const result = await response.json();
      if (!response.ok) throw new Error(result.error?.message ?? result.message ?? "Database import failed");
      setOpen(false); setFile(undefined); setTarget(""); setConfirm("");
      await client.invalidateQueries({ queryKey: ["applications", props.application.id, "protection"] });
      navigate(`/applications/${props.application.id}?tab=backups`);
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  return <section class="database-operations" aria-label="Database operations">
    <div class="section-toolbar"><h2>Operations</h2></div>
    <div class="database-operation-grid">
      <Show when={props.openTerminal}>
        <button type="button" class="database-operation" onClick={() => props.openTerminal?.()}>
          <Icon name="terminal" /><span>Open terminal</span>
        </button>
      </Show>
      <button type="button" class="database-operation" disabled={busy() || !props.application.activeDeploymentId} onClick={() => void showImport()}>
        <Icon name="upload" /><span>{importLabel()}</span>
      </button>
      <Show when={props.application.config.database?.tlsHostname}>
        <button type="button" class="database-operation" disabled={busy() || !props.application.activeDeploymentId} onClick={() => void downloadCA()}>
          <Icon name="download" /><span>Download CA</span>
        </button>
      </Show>
    </div>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <Show when={open()}><ImportDialog application={props.application} redis={redis()} busy={busy()} error={error()}
      file={file()} target={target()} confirm={confirm()}
      setFile={setFile} setTarget={setTarget} setConfirm={setConfirm}
      close={() => { if (!busy()) { setOpen(false); setError(""); } }} submit={importDump} /></Show>
  </section>;
}

function ImportDialog(props: {
  application: Application; redis: boolean; busy: boolean; error: string;
  file?: File; target: string; confirm: string;
  setFile: (file?: File) => void; setTarget: (value: string) => void; setConfirm: (value: string) => void;
  close: () => void; submit: (event: SubmitEvent) => void;
}) {
  let panel!: HTMLFormElement;
  const previous = document.activeElement as HTMLElement | null;
  const importLabel = () => props.redis ? "Restore snapshot" : "Create from dump";
  onMount(() => {
    panel.querySelector<HTMLInputElement>("input:not([type=file])")?.focus();
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !props.busy) props.close();
      if (event.key !== "Tab") return;
      const nodes = [...panel.querySelectorAll<HTMLElement>("input,button:not(:disabled)")].filter(node => node.offsetParent !== null);
      const first = nodes[0], last = nodes[nodes.length - 1];
      if (!first || !last) return;
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener("keydown", key);
    onCleanup(() => { document.removeEventListener("keydown", key); previous?.focus({ preventScroll: true }); });
  });
  return <Portal>
    <div class="drawer-scrim" onClick={props.close} />
    <form ref={panel} onSubmit={props.submit} class="confirm-dialog database-import-dialog" role="dialog" aria-modal="true" aria-labelledby="database-import-title">
      <header><h2 id="database-import-title">{importLabel()}</h2><IconButton icon="close" label="Close import" ghost disabled={props.busy} onClick={props.close} /></header>
      <div class="database-import-copy" classList={{ "is-warning": props.redis }}>{props.redis && <Icon name="warning" />}<span>{props.redis ? "The snapshot is loaded into this Redis service. Current keys can be replaced." : "The dump is restored into a new database on this instance. The current database stays online."}</span></div>
      <Show when={props.error}><ErrorNotice message={props.error} /></Show>
      <label class="database-file-input" classList={{ selected: !!props.file }}>
        <input type="file" required accept={props.redis ? ".rdb" : props.application.config.database?.engine === "postgresql" ? ".dump,.backup" : ".sql"}
          onChange={e => props.setFile(e.currentTarget.files?.[0])} />
        <Icon name="upload" />
        <span>{props.file?.name ?? (props.redis ? "Choose an RDB snapshot" : "Choose a database dump")}</span>
        <small>{props.redis ? ".rdb" : props.application.config.database?.engine === "postgresql" ? ".dump or .backup" : ".sql"}</small>
      </label>
      <fieldset disabled={props.busy} class="form-fields">
        <Show when={!props.redis}>
          <label for="native-database-target">New database name</label>
          <input id="native-database-target" class="input" required pattern="[a-zA-Z_][a-zA-Z0-9_]{0,62}" value={props.target} onInput={e => props.setTarget(e.currentTarget.value)} />
        </Show>
        <Show when={props.redis}>
          <label for="native-database-confirm">Enter <code class="database-confirm-token">{props.application.name}</code> to confirm</label>
          <input id="native-database-confirm" class="input" required autocomplete="off" value={props.confirm} onInput={e => props.setConfirm(e.currentTarget.value)} />
        </Show>
      </fieldset>
      <div class="form-actions">
        <Button variant="ghost" disabled={props.busy} onClick={props.close}>Cancel</Button>
        <Button type="submit" variant="primary" icon="upload" disabled={props.busy || !props.file || (props.redis && props.confirm !== props.application.name)}>{props.busy ? (props.redis ? "Restoring snapshot" : "Creating copy") : importLabel()}</Button>
      </div>
    </form>
  </Portal>;
}
