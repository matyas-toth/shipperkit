import ProxyCheck from "../components/ProxyCheck";
import { For, Show, createSignal, onMount } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import type { SystemInfo, ProxyConfig } from "../types";
import { useAuth } from "../context";
import { bytes } from "../data";
import { Button, ErrorNotice } from "../components/Controls";
import Connections from "../components/Connections";
import Backups from "../components/Backups";
import { Icon } from "../components/Icon";
import AccountSecurity from "../components/AccountSecurity";

export default function Settings() {
  const auth = useAuth();
  const [section, setSection] = createSignal(location.hash.slice(1)||"general");
  const [actionError, setActionError] = createSignal("");
  onMount(() => { document.title = "Settings | Shipper"; });
  async function logout() {
    setActionError("");
    try { await auth.logout(); } catch (error) { setActionError((error as Error).message); }
  }
  const query = createQuery(() => ({ queryKey: ["system"], queryFn: () => api<SystemInfo>("/system"), staleTime: 30_000 }));
  const proxy = createQuery(() => ({ queryKey: ["proxy"], queryFn: () => api<ProxyConfig>("/proxy") }));
  const dockerRows = () => {
    const value = query.data?.docker;
    return [["Connection", value?.connected ? "Connected" : "Disconnected"], ["Host", value?.engine.name || "Unavailable"],
      ["Docker Engine", value?.engine.version || "Unavailable"], ["Operating system", value?.engine.os || "Unavailable"],
      ["Architecture", value?.engine.architecture || "Unavailable"], ["CPUs", String(value?.engine.cpus ?? 0)],
      ["Memory", bytes(value?.engine.memory ?? 0)]];
  };
  return <div class="content content-wide">
    <div class="settings-layout">
      <aside class="settings-navigation"><a href="#general" class="settings-link" classList={{ active: section() === "general" }} onClick={() => setSection("general")}><Icon name="sliders" />General</a>
        <a href="#sources" class="settings-link" classList={{active:section()==="sources"}} onClick={()=>setSection("sources")}><Icon name="git"/>Connections</a>
 <a href="#docker" class="settings-link" classList={{ active: section() === "docker" }} onClick={() => setSection("docker")}><Icon name="plug" />Docker</a>
        <a href="#storage" class="settings-link" classList={{ active: section() === "storage" }} onClick={() => setSection("storage")}><Icon name="database" />Storage</a>
        <a href="#recovery" class="settings-link" classList={{active:section()==="recovery"}} onClick={()=>setSection("recovery")}><Icon name="database"/>Recovery</a>
        <a href="#proxy" class="settings-link" classList={{ active: section() === "proxy" }} onClick={() => setSection("proxy")}><Icon name="globe" />Proxy</a>
        <a href="#account" class="settings-link" classList={{ active: section() === "account" }} onClick={() => setSection("account")}><Icon name="settings" />Account</a>
      </aside>
      <div class="settings-content" id="general">
        <header class="page-heading"><h1>Settings</h1></header>
        <Show when={query.error}><ErrorNotice message={query.error!.message} retry={() => void query.refetch()} /></Show>
        <Show when={actionError()}><ErrorNotice message={actionError()} /></Show>
        <Connections/>
 <section class="settings-section" id="docker"><h2>Docker</h2><dl class="settings-group">
          <For each={dockerRows()}>{([label, value]) => <div class="setting-row"><dt>{label}</dt><dd>{query.isPending ? <span class="loading-line" /> : value}</dd></div>}</For>
        </dl></section>
        <section class="settings-section" id="storage"><h2>Storage</h2><dl class="settings-group">
          <div class="setting-row"><dt>Database</dt><dd>SQLite</dd></div>
          <div class="setting-row"><dt>Journal mode</dt><dd>{query.data?.database.journal ?? "WAL"}</dd></div>
          <div class="setting-row"><dt>Applications</dt><dd>{query.data?.database.counts.applications ?? 0}</dd></div>
          <div class="setting-row"><dt>Containers</dt><dd>{query.data?.database.counts.containers ?? 0}</dd></div>
        </dl></section>
        <section class="settings-section" id="recovery"><Backups/></section>
        <section class="settings-section" id="account"><h2>Account</h2><dl class="settings-group">
          <div class="setting-row"><dt>Email</dt><dd>{auth.session()?.user?.email}</dd></div>
          <div class="setting-row"><dt>Session</dt><dd><Button small onClick={() => void logout()}>Sign out</Button></dd></div>
        </dl><AccountSecurity /></section>
        <section class="settings-section" id="proxy"><h2>Proxy</h2><dl class="settings-group">
          <div class="setting-row"><dt>Proxy</dt><dd>Traefik</dd></div><div class="setting-row"><dt>HTTP port</dt><dd>{proxy.data?.httpPort ?? "Loading"}</dd></div><div class="setting-row"><dt>HTTPS port</dt><dd>{proxy.data?.httpsPort ?? "Loading"}</dd></div><div class="setting-row"><dt>Bind address</dt><dd>{proxy.data?.bindIP ?? "Loading"}</dd></div><div class="setting-row"><dt>ACME email</dt><dd>{proxy.data?.acmeEmail || "Not configured"}</dd></div>
        </dl><ProxyCheck proxy={proxy.data}/></section>
        <section class="settings-section"><h2>Shipper</h2><dl class="settings-group">
          <div class="setting-row"><dt>Version</dt><dd>{query.data?.version ?? "Loading"}</dd></div>
          <div class="setting-row"><dt>Go</dt><dd>{query.data?.runtime.go ?? "Loading"}</dd></div>
          <div class="setting-row"><dt>Heap</dt><dd>{bytes(query.data?.runtime.heapBytes ?? 0)}</dd></div>
        </dl></section>
      </div>
    </div>
  </div>;
}

