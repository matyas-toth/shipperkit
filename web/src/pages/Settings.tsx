import { useLocation } from "@solidjs/router";
import { For, Show, createSignal, onMount } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import type { SystemInfo, ProxyConfig } from "../types";
import { useAuth } from "../context";
import { bytes } from "../data";
import { Button, ErrorNotice } from "../components/Controls";
import Connections from "../components/Connections";
import Backups from "../components/Backups";
import { Icon, type IconName } from "../components/Icon";
import AccountSecurity from "../components/AccountSecurity";
import DashboardSettings from "../components/DashboardSettings";

type SettingsGroup = {
  id: string;
  label: string;
  icon: IconName;
  sections: { id: string; label: string; icon: IconName }[];
};

const groups: SettingsGroup[] = [
  { id: "account", label: "Account settings", icon: "user", sections: [
    { id: "account", label: "Account", icon: "user" },
  ] },
  { id: "server", label: "Server settings", icon: "server", sections: [
    { id: "instance", label: "Instance", icon: "sliders" },
    { id: "dashboard", label: "Dashboard", icon: "globe" },
    { id: "sources", label: "Connections", icon: "git" },
    { id: "recovery", label: "Recovery", icon: "database" },
    { id: "security", label: "Access policy", icon: "shield" },
  ] },
];
const sections = groups.flatMap(group => group.sections);

export default function Settings() {
  const auth = useAuth();
  const location = useLocation();
  const section = () => sections.find(item => item.id === location.hash.slice(1))?.id ?? "instance";
  const [actionError, setActionError] = createSignal("");
  onMount(() => { document.title = "Settings | Shipper"; });
  async function logout() {
    setActionError("");
    try { await auth.logout(); } catch (error) { setActionError((error as Error).message); }
  }
  const query = createQuery(() => ({ queryKey: ["system"], queryFn: () => api<SystemInfo>("/system"), staleTime: 30_000, enabled: section() === "instance" }));
  const proxy = createQuery(() => ({ queryKey: ["proxy"], queryFn: () => api<ProxyConfig>("/proxy"), enabled: section() === "dashboard" }));
  const dockerRows = () => {
    const value = query.data?.docker;
    return [["Connection", value?.connected ? "Connected" : "Disconnected"], ["Host", value?.engine.name || "Unavailable"],
      ["Docker Engine", value?.engine.version || "Unavailable"], ["Operating system", value?.engine.os || "Unavailable"],
      ["Architecture", value?.engine.architecture || "Unavailable"], ["CPUs", String(value?.engine.cpus ?? 0)],
      ["Memory", bytes(value?.engine.memory ?? 0)]];
  };
  return <div class="content content-wide settings-page">
    <header class="page-heading"><h1>Settings</h1></header>
    <div class="settings-layout">
      <nav class="settings-navigation" aria-label="Settings sections">
        <For each={groups}>{group => <section class="settings-nav-group">
          <h2 class="settings-nav-heading"><Icon name={group.icon} />{group.label}</h2>
          <For each={group.sections}>{item => <a href={"/settings#" + item.id} class="settings-link"
            aria-current={section() === item.id ? "page" : undefined}><Icon name={item.icon} />{item.label}</a>}</For>
        </section>}</For>
      </nav>
      <div class="settings-content">
        <Show when={query.error}><ErrorNotice message={query.error!.message} retry={() => void query.refetch()} /></Show>
        <Show when={actionError()}><ErrorNotice message={actionError()} /></Show>
        <Show when={section() === "sources"}><Connections /></Show>
        <Show when={section() === "recovery"}><section class="settings-section" id="recovery"><Backups /></section></Show>
        <Show when={section() === "account"}><section class="settings-section" id="account"><h2>Account</h2><dl class="settings-group">
          <div class="setting-row"><dt>Email</dt><dd>{auth.session()?.user?.email}</dd></div>
          <div class="setting-row"><dt>Session</dt><dd><Button small onClick={() => void logout()}>Sign out</Button></dd></div>
        </dl><AccountSecurity /></section></Show>
        <Show when={section() === "security"}><section class="settings-section" id="security"><h2>Access policy</h2><AccountSecurity scope="instance" /></section></Show>
        <Show when={section() === "dashboard"}><DashboardSettings proxy={proxy.data} refresh={() => proxy.refetch()} /></Show>
        <Show when={section() === "instance"}>
          <section class="settings-section" id="docker"><h2>Docker</h2><dl class="settings-group">
            <For each={dockerRows()}>{([label, value]) => <div class="setting-row"><dt>{label}</dt><dd>{query.isPending ? <span class="loading-line" /> : value}</dd></div>}</For>
          </dl></section>
          <section class="settings-section" id="storage"><h2>Storage</h2><dl class="settings-group">
            <div class="setting-row"><dt>Database</dt><dd>SQLite</dd></div>
            <div class="setting-row"><dt>Journal mode</dt><dd>{query.data?.database.journal ?? "WAL"}</dd></div>
            <div class="setting-row"><dt>Applications</dt><dd>{query.data?.database.counts.applications ?? 0}</dd></div>
            <div class="setting-row"><dt>Containers</dt><dd>{query.data?.database.counts.containers ?? 0}</dd></div>
          </dl></section>
          <section class="settings-section"><h2>Shipper</h2><dl class="settings-group">
            <div class="setting-row"><dt>Version</dt><dd>{query.data?.version ?? "Loading"}</dd></div>
            <div class="setting-row"><dt>Go</dt><dd>{query.data?.runtime.go ?? "Loading"}</dd></div>
            <div class="setting-row"><dt>Heap</dt><dd>{bytes(query.data?.runtime.heapBytes ?? 0)}</dd></div>
          </dl></section>
        </Show>
      </div>
    </div>
  </div>;
}
