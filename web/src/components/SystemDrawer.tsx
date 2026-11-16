import { For, Show, createSignal, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import { createQuery } from "@tanstack/solid-query";
import { useNavigate } from "@solidjs/router";
import { api } from "../api";
import { useAuth } from "../context";
import { bytes } from "../data";
import { Button, IconButton, Status } from "./Controls";
import { Icon } from "./Icon";
import { DiskDonut, MetricChart } from "./MetricChart";
import type { PulseMetric } from "./SystemPulse";
import type { SystemOverview } from "../types";
import type { Observation } from "./Diagnostics";

function value(value: number | null | undefined) {
  return value == null ? "—" : Math.round(value) + "%";
}

export default function SystemDrawer(props: { metric: PulseMetric; close: () => void }) {
  const auth = useAuth();
  const navigate = useNavigate();
  const isAdmin = () => auth.session()?.user?.role === "admin";
  const [copied, setCopied] = createSignal(false);
  const [copyError, setCopyError] = createSignal("");
  let panel!: HTMLDivElement;
  let closeButton!: HTMLButtonElement;
  const previous = document.activeElement as HTMLElement | null;
  const overview = createQuery(() => ({
    queryKey: ["system", "overview"],
    queryFn: () => api<SystemOverview>("/system/overview"),
    refetchInterval: 30000,
    staleTime: 10000,
  }));
  const operations = createQuery(() => ({
    queryKey: ["system", "operations"],
    queryFn: () => api<{ items: Observation[] }>("/system/operations"),
    enabled: isAdmin(),
    refetchInterval: 15000,
  }));
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
  const data = () => overview.data;
  const free = () => Math.max(0, (data()?.disk.total ?? 0) - (data()?.disk.used ?? 0));
  async function copy() {
    try {
      await navigator.clipboard.writeText(JSON.stringify(data(), null, 2));
      setCopied(true);
      setCopyError("");
    } catch {
      setCopyError("Host metrics could not be copied");
    }
  }
  return <Portal>
    <div class="drawer-scrim" onClick={props.close} />
    <div ref={panel} class="drawer system-drawer" role="dialog" aria-modal="true" aria-labelledby="system-drawer-title">
      <div class="drawer-toolbar">
        <h2 id="system-drawer-title">Host health</h2>
        <div class="drawer-actions">
          <IconButton icon={copied() ? "check" : "copy"} label={copied() ? "Host metrics copied" : "Copy host metrics"} ghost onClick={() => void copy()} />
          <button ref={closeButton} class="button icon-button ghost" aria-label="Close host health" onClick={props.close}><Icon name="close" /></button>
        </div>
      </div>
      <div class="drawer-scroll">
        <Show when={copyError()}><p class="drawer-error" role="alert">{copyError()}</p></Show>
        <Show when={overview.error}><p class="drawer-error" role="alert">{overview.error!.message}</p></Show>
        <Show when={data()?.stale}><p class="drawer-note">The last sample is {Math.max(1, Math.round((Date.now() / 1000 - (data()?.sampledAt ?? Date.now() / 1000)) / 60))} minutes old.</p></Show>
        <div class="capacity-charts">
          <MetricChart label="CPU" value={data()?.measuring ? "Measuring" : value(data()?.cpu.percent)} state={data()?.thresholds.cpu} values={data()?.series.cpu ?? []} />
          <MetricChart label="Memory" value={value(data()?.memory.percent)} state={data()?.thresholds.memory} values={data()?.series.memory ?? []} detail={data()?.memory.total ? `${bytes(data()!.memory.used ?? 0)} of ${bytes(data()!.memory.total!)}` : undefined} />
          <MetricChart label="Disk" value={value(data()?.disk.percent)} state={data()?.thresholds.disk} values={data()?.series.disk ?? []} detail={data()?.disk.total ? `${bytes(data()!.disk.used ?? 0)} of ${bytes(data()!.disk.total!)}` : undefined} />
        </div>
        <DiskDonut percent={data()?.disk.percent ?? null} used={bytes(data()?.disk.used ?? 0)} total={bytes(data()?.disk.total ?? 0)} free={bytes(free())} />
        <dl class="details-list system-facts">
          <div><dt>Load</dt><dd>{data()?.load1 ? data()!.load1.toFixed(2) : "—"}</dd></div>
          <div><dt>Containers</dt><dd>{data()?.containers.running ?? 0} running of {data()?.containers.total ?? 0}</dd></div>
          <div><dt>Vantage</dt><dd>{data()?.vantage === "limited" ? "Limited host metrics" : "Host"}</dd></div>
        </dl>
        <Show when={isAdmin() && operations.data?.items.length}>
          <section class="system-drawer-checks">
            <h3>Checks</h3>
            <div class="check-list">
              <For each={operations.data!.items.filter(item => ["daemon", "proxy", "storage", "host"].includes(item.kind))}>{item => (
                <div class="check-row">
                  <Status value={item.stale ? "stale" : item.state} />
                  <span>{item.kind === "storage" ? item.details.scope ?? "Storage" : item.kind}</span>
                  <span class="check-summary">{item.summary}</span>
                </div>
              )}</For>
            </div>
          </section>
        </Show>
        <Show when={isAdmin()}>
          <Button class="drawer-system-link" variant="neutral" onClick={() => { props.close(); navigate("/system"); }}>Open system</Button>
        </Show>
      </div>
    </div>
  </Portal>;
}
