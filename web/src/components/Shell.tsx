import Logo from "./Logo";
import { EditorActionsContext } from "./EditorActions";
import { A, useLocation, useNavigate, type RouteSectionProps } from "@solidjs/router";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { Show, createEffect, createSignal, onCleanup, onMount } from "solid-js";
import { api, post } from "../api";
import { InventoryContext, UpdatesContext, useAuth } from "../context";
import type { Inventory, UpdateChannel, UpdateState, UpdateStatus } from "../types";
import { IconButton } from "./Controls";
import { Icon } from "./Icon";
import UpdateDialog from "./UpdateDialog";

export default function Shell(props: RouteSectionProps) {
  const auth = useAuth();
  const client = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  const query = createQuery(() => ({ queryKey: ["inventory"], queryFn: () => api<Inventory>("/applications"), staleTime: 60_000, reconcile: "id" }));
  const updates = createQuery(() => ({ queryKey: ["updates"], queryFn: () => api<UpdateState>("/updates"), staleTime: 15_000, refetchInterval: 30_000 }));
  const [editorActions, setEditorActions] = createSignal<HTMLDivElement>();
  const [live, setLive] = createSignal(false);
  const [menu, setMenu] = createSignal(false);
  const [refreshing, setRefreshing] = createSignal(false);
  const [actionError, setActionError] = createSignal("");
  const [updateOpen, setUpdateOpen] = createSignal(false);
  const [updateInitial, setUpdateInitial] = createSignal<UpdateStatus>();
  const [dismissedUpdate, setDismissedUpdate] = createSignal("");
  const [loadedVersion, setLoadedVersion] = createSignal("");
  createEffect(() => {
    const version = updates.data?.currentVersion;
    if (version && !loadedVersion()) setLoadedVersion(version);
  });
  const updateInProgress = (status?: UpdateStatus) => !!status && ["queued", "preparing", "pulling", "checkpoint", "updating", "starting", "verifying", "restarting"].includes(status.state);
  const updateBadge = () => !!updates.data?.enabled && !!updates.data?.available && !updateInProgress(updates.data?.status);
  const updating = () => updateInProgress(updates.data?.status);
  createEffect(() => {
    const status = updates.data?.status;
    if (!status || !updateInProgress(status) || status.requestId && status.requestId === dismissedUpdate()) return;
    if (updateOpen()) return;
    setUpdateInitial(status);
    setUpdateOpen(true);
  });
  function closeUpdate() {
    setDismissedUpdate(updates.data?.status?.requestId || "");
    setUpdateOpen(false);
  }
  function openUpdate() {
    setDismissedUpdate("");
    setUpdateInitial(updates.data?.status);
    setUpdateOpen(true);
  }
  async function checkUpdates() {
    client.setQueryData<UpdateState | undefined>(["updates"], old => old ? { ...old, checking: true, error: "" } : old);
    try {
      const state = await post<UpdateState>("/updates/check", {}, auth.session()?.csrfToken);
      client.setQueryData(["updates"], state);
    } catch (error) {
      await updates.refetch();
      throw error;
    }
  }
  async function saveUpdates(channel: UpdateChannel, auto: boolean) {
    const previous = updates.data;
    client.setQueryData<UpdateState | undefined>(["updates"], old => old ? { ...old, channel, auto, available: old.channel === channel ? old.available : null, error: "" } : old);
    try {
      const state = await post<UpdateState>("/updates/settings", { channel, auto }, auth.session()?.csrfToken);
      client.setQueryData(["updates"], state);
      return state;
    } catch (error) {
      if (previous) client.setQueryData(["updates"], previous);
      throw error;
    }
  }
  async function applyUpdate() {
    const status = await post<UpdateStatus>("/updates/apply", {}, auth.session()?.csrfToken);
    setDismissedUpdate("");
    setUpdateInitial(status);
    setUpdateOpen(true);
    await updates.refetch();
  }
  let accountButton!: HTMLButtonElement;
  let menuElement!: HTMLDivElement;
  let stream: EventSource | undefined;
  const invalidate = () => {
    void client.invalidateQueries({ queryKey: ["inventory"] });
    void client.invalidateQueries({ queryKey: ["system"] });
    void client.invalidateQueries({ queryKey: ["applications"] });
    void client.invalidateQueries({ queryKey: ["deployment-logs"] });
  };
  const closeMenu = () => { setMenu(false); accountButton?.focus(); };
  onMount(() => {
    stream = new EventSource("/api/events");
    stream.onopen = () => { setLive(true); void client.invalidateQueries({ queryKey: ["session"] }); invalidate(); };
    stream.addEventListener("inventory", invalidate);
    stream.onerror = () => { setLive(false); void client.invalidateQueries({ queryKey: ["session"] }); };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && menu()) closeMenu();
      if (event.key === "/" && !(event.target instanceof HTMLInputElement) && !(event.target instanceof HTMLTextAreaElement)) {
        event.preventDefault(); focusSearch();
      }
    };
    const outside = (event: PointerEvent) => {
      if (menu() && !menuElement?.contains(event.target as Node) && !accountButton.contains(event.target as Node)) setMenu(false);
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("pointerdown", outside);
    onCleanup(() => { document.removeEventListener("keydown", onKey); document.removeEventListener("pointerdown", outside); });
  });
  onCleanup(() => stream?.close());
  function focusSearch() {
    navigate("/");
    requestAnimationFrame(() => document.querySelector<HTMLInputElement>("#application-search")?.focus());
  }
  async function refresh() {
    if (refreshing()) return;
    setRefreshing(true);
    try { await post("/refresh", {}, auth.session()?.csrfToken); }
    finally { setRefreshing(false); }
  }
  async function logout() {
    setActionError("");
    try { await auth.logout(); } catch (error) { setActionError((error as Error).message); }
  }
  const connected = () => query.data?.docker.connected && live();
  const section = () => location.pathname === "/settings" ? "settings" : location.pathname === "/system" ? "system" : "applications";
  return <UpdatesContext.Provider value={{ data: () => updates.data, pending: () => updates.isPending, error: () => updates.error,
    loadedVersion, check: checkUpdates, save: saveUpdates, apply: applyUpdate, open: openUpdate }}>
  <InventoryContext.Provider value={{ data: () => query.data, pending: () => query.isPending, error: () => query.error,
    live, refresh, refreshing }}>
    <div class="app-frame">
      <header class="topbar">
        <A class="brand" href="/" aria-label="Shipper"><span class="brand-mark"><Logo /></span><span>Shipper</span></A>
        <nav class="primary-navigation" aria-label="Main navigation">
          <A href="/" end activeClass="" class="nav-link" classList={{ active: !["/settings", "/system"].includes(location.pathname) }} aria-current={!["/settings", "/system"].includes(location.pathname) ? "page" : undefined}><Icon name="layers" />Applications</A>
          <A href="/system" activeClass="active" class="nav-link"><Icon name="terminal" />System</A>
          <A href="/settings" activeClass="active" class="nav-link"><Icon name="settings" />Settings<Show when={updateBadge()}><span class="nav-update-badge" title={updates.data?.available ? "Update to " + updates.data.available.version : "Update available"}>Update</span></Show><Show when={updating()}><span class="nav-update-badge is-updating" title="Shipper is updating">Updating</span></Show></A>
        </nav>
        <div class="topbar-actions">
          <span class={"connection " + (connected() ? "is-connected" : "")} title={query.data?.docker.error ?? "Docker connection"}>
            <span class="status-dot" /><span class="connection-label">{connected() ? "Connected" : "Reconnecting"}</span>
          </span>
          <IconButton icon="search" label="Search applications" class="topbar-search" ghost onClick={focusSearch} />
          <div class="account">
            <button ref={accountButton} class="account-trigger" aria-label="Account" aria-expanded={menu()} aria-haspopup="menu" onClick={() => setMenu(!menu())}>
              <span class="avatar">{auth.session()?.user?.email.slice(0, 1)}</span>
            </button>
            <Show when={menu()}>
              <div ref={menuElement} class="menu account-dropdown" role="menu" aria-label="Account" onKeyDown={event => {
                if (event.key !== "ArrowDown" && event.key !== "ArrowUp" && event.key !== "Home" && event.key !== "End") return;
                event.preventDefault();
                const items = Array.from(menuElement.querySelectorAll<HTMLButtonElement>("[role=menuitem]"));
                const current = items.indexOf(document.activeElement as HTMLButtonElement);
                const next = event.key === "Home" ? 0 : event.key === "End" ? items.length - 1 : (current + (event.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
                items[next]?.focus();
              }}>
                <div class="account-email">{auth.session()?.user?.email}</div>
                <button role="menuitem" onClick={() => { closeMenu(); navigate("/settings"); }}><Icon name="sliders" />Settings</button>
                <button role="menuitem" onClick={() => void logout()}><Icon name="logout" />Sign out</button>
                <Show when={actionError()}><div role="alert" class="menu-error">{actionError()}</div></Show>
              </div>
            </Show>
          </div>
        </div>
      </header>
      <EditorActionsContext.Provider value={editorActions}>
        <main class="page-scroll">{props.children}</main>
        <div ref={setEditorActions} class="page-actions" />
      </EditorActionsContext.Provider>
      <nav class="mobile-bottom-nav" data-active={section()} aria-label="Main navigation">
        <div class="mobile-nav-pill">
          <span class="mobile-nav-indicator" aria-hidden="true" />
          <A href="/" end activeClass="" class="mobile-nav-link" classList={{ active: section() === "applications" }} aria-current={section() === "applications" ? "page" : undefined}>
            <span class="mobile-nav-icon"><Icon name="layers" /></span>
            <span class="mobile-nav-label">Applications</span>
          </A>
          <A href="/system" activeClass="" class="mobile-nav-link" classList={{ active: section() === "system" }} aria-current={section() === "system" ? "page" : undefined}>
            <span class="mobile-nav-icon"><Icon name="terminal" /></span>
            <span class="mobile-nav-label">System</span>
          </A>
          <A href="/settings" activeClass="" class="mobile-nav-link" classList={{ active: section() === "settings" }} aria-current={section() === "settings" ? "page" : undefined}>
            <span class="mobile-nav-icon"><Icon name="settings" /></span>
            <span class="mobile-nav-label">Settings</span>
            <Show when={updateBadge() || updating()}>
              <span classList={{ "mobile-nav-badge": true, "is-updating": updating() }} title={updating() ? "Shipper is updating" : updates.data?.available ? "Update to " + updates.data.available.version : "Update available"} aria-hidden="true" />
            </Show>
          </A>
        </div>
        <button type="button" class="mobile-nav-search" aria-label="Search applications" title="Search applications" onClick={focusSearch}>
          <Icon name="search" />
        </button>
      </nav>
    </div>
    <UpdateDialog open={updateOpen()} close={closeUpdate} done={() => void updates.refetch()} initial={updateInitial()} />
  </InventoryContext.Provider>
  </UpdatesContext.Provider>;
}

