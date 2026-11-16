import { For, Show, createMemo, createSignal, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import { createQuery } from "@tanstack/solid-query";
import { useNavigate } from "@solidjs/router";
import { api, post } from "../api";
import { useAuth } from "../context";
import { bytes } from "../data";
import { Button, ErrorNotice, IconButton, Status } from "./Controls";
import { Icon } from "./Icon";
import type { DeploymentDiagnosis, DiagnosisAction } from "../types";

export default function DiagnosisDrawer(props: { applicationId: string; deploymentId: string; close: () => void }) {
  const auth = useAuth();
  const navigate = useNavigate();
  const canEdit = () => auth.session()?.user?.role !== "viewer";
  const [copied, setCopied] = createSignal(false);
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  let panel!: HTMLDivElement;
  let closeButton!: HTMLButtonElement;
  const previous = document.activeElement as HTMLElement | null;
  const query = createQuery(() => ({
    queryKey: ["applications", props.applicationId, "deployments", props.deploymentId, "diagnosis"],
    queryFn: () => api<DeploymentDiagnosis>(`/applications/${props.applicationId}/deployments/${props.deploymentId}/diagnosis`),
    staleTime: 15000,
  }));
  const diagnosis = () => query.data;
  const confidence = createMemo(() => {
    const value = diagnosis()?.cause.confidence;
    if (value === "high") return "High confidence";
    if (value === "medium") return "Medium confidence";
    return "Low confidence";
  });
  const logs = () => diagnosis()?.logs.flatMap(group => group.lines).slice(-20) ?? [];
  onMount(() => {
    closeButton.focus();
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") props.close();
      if (event.key === "Tab") {
        const nodes = panel.querySelectorAll<HTMLElement>("button,a[href],input,[tabindex='0']");
        const first = nodes[0], last = nodes[nodes.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    };
    document.addEventListener("keydown", keyboard);
    onCleanup(() => { document.removeEventListener("keydown", keyboard); previous?.focus(); });
  });
  async function copy() {
    try {
      await navigator.clipboard.writeText(JSON.stringify(diagnosis(), null, 2));
      setCopied(true);
      setError("");
    } catch {
      setError("The diagnosis could not be copied");
    }
  }
  function download() {
    const blob = new Blob([JSON.stringify(diagnosis(), null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "shipper-diagnosis-" + props.deploymentId.slice(0, 8) + ".json";
    link.click();
    URL.revokeObjectURL(url);
  }
  async function action(value: DiagnosisAction) {
    if (busy()) return;
    if (value.kind === "copy") { await copy(); return; }
    if (value.kind === "navigate") { props.close(); navigate(value.target); return; }
    if (value.kind !== "retry") return;
    setBusy(true);
    setError("");
    try {
      const next = await post<{ id: string }>(`/applications/${props.applicationId}/deployments/${props.deploymentId}/retry`, {}, auth.session()?.csrfToken);
      props.close();
      navigate(`/applications/${props.applicationId}?tab=deployments&deployment=${encodeURIComponent(next.id)}`);
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }
  return <Portal>
    <div class="drawer-scrim" onClick={props.close} />
    <div ref={panel} class="drawer diagnosis-drawer" role="dialog" aria-modal="true" aria-labelledby="diagnosis-title">
      <div class="drawer-toolbar">
        <h2 id="diagnosis-title">Why this failed</h2>
        <div class="drawer-actions">
          <IconButton icon={copied() ? "check" : "copy"} label={copied() ? "Diagnosis copied" : "Copy diagnosis"} ghost onClick={() => void copy()} />
          <button ref={closeButton} class="button icon-button ghost" aria-label="Close diagnosis" onClick={props.close}><Icon name="close" /></button>
        </div>
      </div>
      <div class="drawer-scroll">
        <Show when={query.error}><ErrorNotice message={query.error!.message} retry={() => void query.refetch()} /></Show>
        <Show when={error()}><ErrorNotice message={error()} /></Show>
        <Show when={!diagnosis() && query.isPending}><span class="loading-line" aria-label="Loading diagnosis" /></Show>
        <Show when={diagnosis()}>{value => <>
          <section class="diagnosis-cause" classList={{ medium: value().cause.confidence !== "high" }}>
            <div class="diagnosis-cause-meta">
              <span>{value().cause.confidence === "high" ? "Cause" : "Likely cause"}</span>
              <span>{confidence()}</span>
            </div>
            <h3>{value().cause.summary}</h3>
            <Show when={value().cause.detail}><p>{value().cause.detail}</p></Show>
            <Show when={value().cause.evidence.length}>
              <ul class="diagnosis-evidence"><For each={value().cause.evidence}>{line => <li>{line}</li>}</For></ul>
            </Show>
          </section>
          <section class="diagnosis-section">
            <h3>Checks</h3>
            <div class="diagnosis-checks"><For each={value().checks}>{check => (
              <div class="diagnosis-check">
                <Status value={check.state} />
                <span>{check.layer === "daemon" ? "Docker" : check.layer === "readiness" ? "Readiness" : check.layer}</span>
                <span class="diagnosis-check-summary">{check.summary}</span>
              </div>
            )}</For></div>
          </section>
          <section class="diagnosis-section">
            <h3>Services</h3>
            <div class="diagnosis-services"><For each={value().services}>{service => (
              <article class="diagnosis-service">
                <header><strong>{service.name}</strong><Status value={service.health && service.health !== "none" ? service.health : service.state} /></header>
                <dl>
                  <div><dt>State</dt><dd>{service.state}</dd></div>
                  <div><dt>Exit</dt><dd>{service.exitCode ?? 0}</dd></div>
                  <div><dt>Restarts</dt><dd>{service.restarts ?? 0}</dd></div>
                  <Show when={service.oomKilled}><div><dt>Memory</dt><dd>OOM killed</dd></div></Show>
                  <Show when={service.cpuPercent != null}><div><dt>CPU</dt><dd>{service.cpuPercent!.toFixed(1)}%</dd></div></Show>
                  <Show when={service.memoryBytes != null}><div><dt>Memory</dt><dd>{bytes(service.memoryBytes!)}</dd></div></Show>
                  <Show when={service.healthPath}><div><dt>Health path</dt><dd class="mono">{service.healthPath}</dd></div></Show>
                </dl>
              </article>
            )}</For></div>
          </section>
          <section class="diagnosis-section">
            <div class="diagnosis-section-head"><h3>Last lines</h3><button type="button" class="inline-link" onClick={props.close}>Show full log</button></div>
            <pre class="diagnosis-log">{logs().length ? logs().join("\n") : "No deployment lines were retained."}</pre>
          </section>
          <Show when={value().unknowns.length}>
            <details class="diagnosis-unknowns">
              <summary>Not measured</summary>
              <ul><For each={value().unknowns}>{line => <li>{line}</li>}</For></ul>
            </details>
          </Show>
          <div class="diagnosis-actions">
            <For each={value().nextActions}>{next => <Show when={next.kind !== "retry" || canEdit()}><Button small={next.kind === "copy"} variant={next.kind === "retry" ? "primary" : "neutral"} disabled={busy()} onClick={() => void action(next)}>{next.label}</Button></Show>}</For>
          </div>
          <div class="diagnosis-footer">
            <Button small variant="ghost" icon="download" onClick={download}>Download</Button>
            <span class="diagnosis-id mono" title={value().deployment.id}>{value().deployment.id.slice(0, 12)}</span>
          </div>
        </>}</Show>
      </div>
    </div>
  </Portal>;
}
