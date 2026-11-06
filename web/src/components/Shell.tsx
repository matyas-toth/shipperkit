import Logo from "./Logo";
import { EditorActionsContext } from "./EditorActions";
import { A, useLocation, useNavigate, type RouteSectionProps } from "@solidjs/router";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { Show, createSignal, onCleanup, onMount } from "solid-js";
import { api, post } from "../api";
import { InventoryContext, useAuth } from "../context";
import type { Inventory } from "../types";
import { IconButton } from "./Controls";
import { Icon } from "./Icon";

export default function Shell(props: RouteSectionProps) {
  const auth = useAuth();
  const client = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  const query = createQuery(() => ({ queryKey: ["inventory"], queryFn: () => api<Inventory>("/applications"), staleTime: 60_000, reconcile: "id" }));
  const [editorActions, setEditorActions] = createSignal<HTMLDivElement>();
  const [live, setLive] = createSignal(false);
  const [menu, setMenu] = createSignal(false);
  const [refreshing, setRefreshing] = createSignal(false);
  const [actionError, setActionError] = createSignal("");
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
    stream.onopen = () => { setLive(true); invalidate(); };
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
  return <InventoryContext.Provider value={{ data: () => query.data, pending: () => query.isPending, error: () => query.error,
    live, refresh, refreshing }}>
    <div class="app-frame">
      <header class="topbar">
        <A class="brand" href="/" aria-label="Shipper"><span class="brand-mark"><Logo /></span><span>Shipper</span></A>
        <nav class="primary-navigation" aria-label="Main navigation">
          <A href="/" end activeClass="" class="nav-link" classList={{ active: !["/settings", "/system"].includes(location.pathname) }} aria-current={!["/settings", "/system"].includes(location.pathname) ? "page" : undefined}><Icon name="layers" />Applications</A>
          <A href="/system" activeClass="active" class="nav-link"><Icon name="terminal" />System</A>
          <A href="/settings" activeClass="active" class="nav-link"><Icon name="settings" />Settings</A>
        </nav>
        <div class="topbar-actions">
          <span class={"connection " + (connected() ? "is-connected" : "")} title={query.data?.docker.error ?? "Docker connection"}>
            <span class="status-dot" /><span class="connection-label">{connected() ? "Connected" : "Reconnecting"}</span>
          </span>
          <IconButton icon="search" label="Search applications" ghost onClick={focusSearch} />
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
    </div>
  </InventoryContext.Provider>;
}

