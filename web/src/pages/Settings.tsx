import { useLocation } from "@solidjs/router";
import { For, Show, createMemo, createSignal, onMount } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import type { ProxyConfig } from "../types";
import { useAuth } from "../context";
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

type SettingsGroup = {
  id: string;
  label: string;
  sections: { id: string; label: string; icon: IconName }[];
};

const accountGroup: SettingsGroup = { id: "account", label: "Account settings", sections: [
  { id: "account", label: "Account", icon: "user" },
] };
const serverGroup: SettingsGroup = { id: "server", label: "Server settings", sections: [
  { id: "users", label: "Users", icon: "users" },
  { id: "authentication", label: "Authentication", icon: "security" },
  { id: "security", label: "Access policy", icon: "shield" },
  { id: "tokens", label: "Access tokens", icon: "key" },
  { id: "dashboard", label: "Dashboard", icon: "globe" },
  { id: "sources", label: "Connections", icon: "git" },
  { id: "notifications", label: "Notifications", icon: "mail" },
  { id: "updates", label: "Updates", icon: "refresh" },
  { id: "recovery", label: "Recovery", icon: "database" },
] };

export default function Settings() {
  const auth = useAuth();
  const location = useLocation();
  const groups = createMemo(() => auth.session()?.user?.role === "admin" ? [accountGroup, serverGroup] : [accountGroup]);
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
          <h2 class="settings-nav-heading">{group.label}</h2>
          <For each={group.sections}>{item => <a href={"/settings#" + item.id} class="settings-link"
            aria-current={section() === item.id ? "page" : undefined}><Icon name={item.icon} />{item.label}</a>}</For>
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
        <Show when={section() === "security"}><section class="settings-section" id="security"><h2>Access policy</h2><AccountSecurity scope="instance" /></section></Show>
        <Show when={section() === "dashboard"}><DashboardSettings proxy={proxy.data} refresh={() => proxy.refetch()} /></Show>
        <Show when={section() === "notifications"}><Notifications /></Show>
        <Show when={section() === "tokens"}><AccessTokens /></Show>
      </div>
    </div>
  </div>;
}
