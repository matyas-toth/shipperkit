import { Show } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import { bytes } from "../data";
import type { SystemOverview } from "../types";
import { Sparkline } from "./MetricChart";

export type PulseMetric = "cpu" | "memory" | "disk";

function percent(value: number | null | undefined) {
  return value == null ? "—" : Math.round(value) + "%";
}

export default function SystemPulse(props: { open: (metric: PulseMetric) => void }) {
  const query = createQuery(() => ({
    queryKey: ["system", "overview"],
    queryFn: () => api<SystemOverview>("/system/overview"),
    refetchInterval: 30000,
    staleTime: 10000,
  }));
  const data = () => query.data;
  const cpuValue = () => data()?.measuring ? "Measuring" : percent(data()?.cpu.percent);
  const memoryValue = () => percent(data()?.memory.percent);
  const diskValue = () => percent(data()?.disk.percent);
  const cpuState = () => data()?.thresholds.cpu ?? "unknown";
  const memoryState = () => data()?.thresholds.memory ?? "unknown";
  const diskState = () => data()?.thresholds.disk ?? "unknown";
  const memoryTitle = () => {
    const value = data();
    return value?.memory.total ? bytes(value.memory.used ?? 0) + " of " + bytes(value.memory.total) : "Memory is not measured on this host";
  };
  const diskTitle = () => {
    const value = data();
    return value?.disk.total ? bytes(value.disk.used ?? 0) + " of " + bytes(value.disk.total) : "Disk is not measured on this host";
  };
  const status = () => query.error ? "Unavailable" : data()?.stale ? "Stale" : data()?.measuring ? "Measuring" : "Live";
  return (
    <section class="system-pulse" classList={{ loading: query.isPending, stale: data()?.stale || !!query.error }} aria-label="Host health">
      <button type="button" class="pulse-segment" classList={{ warning: cpuState() === "warning", critical: cpuState() === "critical" }} aria-label={`CPU ${cpuValue()}`} title={data()?.unavailableCpu ? "CPU is not measured on this host" : "Open CPU history"} onClick={() => props.open("cpu")}>
        <span class="pulse-label">CPU</span>
        <span class="pulse-value">{cpuValue()}</span>
        <Sparkline values={data()?.series.cpu ?? []} state={cpuState()} />
      </button>
      <button type="button" class="pulse-segment" classList={{ warning: memoryState() === "warning", critical: memoryState() === "critical" }} aria-label={`Memory ${memoryValue()}`} title={memoryTitle()} onClick={() => props.open("memory")}>
        <span class="pulse-label">Memory</span>
        <span class="pulse-value">{memoryValue()}</span>
        <Sparkline values={data()?.series.memory ?? []} state={memoryState()} />
      </button>
      <button type="button" class="pulse-segment" classList={{ warning: diskState() === "warning", critical: diskState() === "critical" }} aria-label={`Disk ${diskValue()}`} title={diskTitle()} onClick={() => props.open("disk")}>
        <span class="pulse-label">Disk</span>
        <span class="pulse-value">{diskValue()}</span>
        <Sparkline values={data()?.series.disk ?? []} state={diskState()} />
      </button>
      <Show when={status() !== "Live"}>
        <span class="pulse-status" role="status">{status()}</span>
      </Show>
    </section>
  );
}
