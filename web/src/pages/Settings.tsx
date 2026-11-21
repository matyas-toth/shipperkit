import { useLocation, useNavigate } from "@solidjs/router";
import { For, Show, createEffect, createMemo, createSignal, onMount } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import type { ProxyConfig } from "../types";
import { useAuth, useUpdates } from "../context";
import { Button, ErrorNotice } from "../components/Controls";
import Connections from "../components/Connections";
import Backups from "../components/Backups";
import { Icon, type IconName } from "../components/Icon";
import AccountSecurity from "../components/AccountSecurity";
import DashboardSettings from "../components/DashboardSettings";
import Updates from "../components/Updates";
import Users from "../components/Users";
import Authentication from "../components/Authentication";
import Notifications from "../components/Notifications";
import AccessTokens from "../components/AccessTokens";
import Telemetry from "../components/Telemetry";

type SettingsGroup = {
  id: string;
  label: string;
  sections: { id: string; label: string; icon: IconName }[];
};

const accountGroup: SettingsGroup = { id: "account", label: "Account", sections: [
  { id: "account", label: "Account", icon: "user" },
] };
const peopleGroup: SettingsGroup = { id: "people", label: "People and sign-in", sections: [
  { id: "users", label: "Users", icon: "users" },
  { id: "authentication", label: "Authentication", icon: "security" },
] };
const instanceGroup: SettingsGroup = { id: "instance", label: "Instance", sections: [
  { id: "dashboard", label: "Dashboard", icon: "globe" },
  { id: "sources", label: "Connections", icon: "git" },
  { id: "tokens", label: "Access tokens", icon: "key" },
  { id: "notifications", label: "Notifications", icon: "mail" },
  { id: "updates", label: "Updates", icon: "refresh" },
  { id: "usage", label: "Usage data", icon: "chart" },
  { id: "recovery", label: "Recovery", icon: "database" },
] };
// Older links point at sections that were merged into another one.
const merged: Record<string, string> = { security: "authentication" };
const updating = ["queued", "preparing", "pulling", "checkpoint", "updating", "starting", "verifying", "restarting"];

export default function Settings() {
  const auth = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const updates = useUpdates();
  const groups = createMemo(() => auth.session()?.user?.role === "admin" ? [accountGroup, peopleGroup, instanceGroup] : [accountGroup]);
  createEffect(() => { const target = merged[location.hash.slice(1)]; if (target) navigate("/settings#" + target, { replace: true }); });
  const updateReady = () => { const data = updates.data(); return !!data?.enabled && !!data.available && !updating.includes(data.status?.state ?? ""); };
  const sections = createMemo(() => groups().flatMap(group => group.sections));
  const section = () => sections().find(item => item.id === location.hash.slice(1))?.id ?? "account";
  const [actionError, setActionError] = createSignal("");
  onMount(() => { document.title = "Settings | Shipper"; });
  async function logout() {
    setActionError("");
    try { await auth.logout(); } catch (error) { setActionError((error as Error).message); }
  }
  const proxy = createQuery(() => ({ queryKey: ["proxy"], queryFn: () => api<ProxyConfig>("/proxy"), enabled: section() === "dashboard" }));
  return <div class="content content-wide settings-page">
    <header class="page-heading"><h1>Settings</h1></header>
    <div class="settings-layout">
      <nav class="settings-navigation" aria-label="Settings sections">
        <For each={groups()}>{group => <section class="settings-nav-group">
          <Show when={group.sections.length > 1 || group.sections[0].label !== group.label}><h2 class="settings-nav-heading">{group.label}</h2></Show>
          <For each={group.sections}>{item => <a href={"/settings#" + item.id} class="settings-link"
            aria-current={section() === item.id ? "page" : undefined}><Icon name={item.icon} />{item.label}<Show when={item.id === "updates" && updateReady()}><span class="settings-link-badge">New</span></Show></a>}</For>
        </section>}</For>
      </nav>
      <div class="settings-content">
        <Show when={actionError()}><ErrorNotice message={actionError()} /></Show>
        <Show when={section() === "sources"}><Connections /></Show>
        <Show when={section() === "users"}><Users /></Show>
        <Show when={section() === "authentication"}><Authentication /></Show>
        <Show when={section() === "updates"}><Updates /></Show>
        <Show when={section() === "recovery"}><section class="settings-section" id="recovery"><Backups /></section></Show>
        <Show when={section() === "account"}><section class="settings-section" id="account"><h2>Account</h2><dl class="settings-group">
          <div class="setting-row"><dt>Email</dt><dd>{auth.session()?.user?.email}</dd></div>
          <div class="setting-row"><dt>Session</dt><dd><Button small onClick={() => void logout()}>Sign out</Button></dd></div>
        </dl><AccountSecurity /></section></Show>
        <Show when={section() === "dashboard"}><DashboardSettings proxy={proxy.data} refresh={() => proxy.refetch()} /></Show>
        <Show when={section() === "notifications"}><Notifications /></Show>
        <Show when={section() === "tokens"}><AccessTokens /></Show>
        <Show when={section() === "usage"}><Telemetry /></Show>
      </div>
    </div>
  </div>;
}
