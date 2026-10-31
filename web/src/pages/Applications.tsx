import { For, Show, createMemo, createSignal, onMount } from "solid-js";
import { useInventory } from "../context";
import { filterContainers, statusOf, portLabel, createdDate } from "../data";
import type { Container } from "../types";
import { Icon } from "../components/Icon";
import { Button, ErrorNotice, IconButton, Status } from "../components/Controls";
import ContainerDrawer from "../components/ContainerDrawer";

const ROW_HEIGHT = 48;
const WINDOW_ROWS = 32;
export default function Applications() {
  const inventory = useInventory();
  onMount(() => { document.title = "Applications | Shipper"; });
  const [search, setSearch] = createSignal("");
  const [scope, setScope] = createSignal("all");
  const [status, setStatus] = createSignal("all");
  const [selectedID, setSelectedID] = createSignal<string | null>(null);
  const [scrollTop, setScrollTop] = createSignal(0);
  const [actionError, setActionError] = createSignal("");
  let viewport!: HTMLDivElement;
  const items = createMemo(() => filterContainers(inventory.data()?.items ?? [], search(), scope(), status()));
  const start = createMemo(() => Math.min(Math.max(0, Math.floor((scrollTop() - 32) / ROW_HEIGHT) - 5), Math.max(0, items().length - WINDOW_ROWS)));
  const visible = createMemo(() => items().slice(start(), start() + WINDOW_ROWS));
  const selected = createMemo(() => inventory.data()?.items.find(item => item.id === selectedID()));
  const searching = () => !!search() || status() !== "all";
  function resetScroll() { if (viewport) viewport.scrollTop = 0; setScrollTop(0); }
  async function refresh() { setActionError(""); try { await inventory.refresh(); } catch (error) { setActionError((error as Error).message); } }
  const selectRow = (event: KeyboardEvent, item: Container) => {
    if (event.key === "Enter" || event.key === " ") { event.preventDefault(); setSelectedID(item.id); }
  };
  return <div class="content content-narrow">
    <header class="page-heading"><h1>Applications</h1>
      <Button icon="refresh" disabled={inventory.refreshing()} onClick={() => void refresh()}>Refresh</Button>
    </header>
    <Show when={inventory.error()}><ErrorNotice message={inventory.error()!.message} retry={() => void refresh()} /></Show>
    <Show when={actionError()}><ErrorNotice message={actionError()} /></Show>
    <Show when={inventory.data() && !inventory.data()!.docker.connected}>
      <ErrorNotice message={inventory.data()!.docker.error || "Docker is unavailable"} retry={() => void refresh()} />
    </Show>
    <div class="table-toolbar">
      <div class="segments" role="group" aria-label="Container scope">
        <button aria-pressed={scope() === "all"} onClick={() => { setScope("all"); resetScroll(); }}>All containers</button>
        <button aria-pressed={scope() === "managed"} onClick={() => { setScope("managed"); resetScroll(); }}>Shipper</button>
      </div>
      <div class="search-field">
        <Icon name="search" /><input id="application-search" aria-label="Search applications" placeholder="Search applications" autocomplete="off"
          value={search()} onInput={event => { setSearch(event.currentTarget.value); resetScroll(); }} />
        <Show when={search()}><IconButton icon="close" label="Clear search" ghost onClick={() => { setSearch(""); resetScroll(); }} /></Show>
      </div>
      <select class="input status-filter" aria-label="Filter by status" value={status()} onChange={event => { setStatus(event.currentTarget.value); resetScroll(); }}>
        <option value="all">All statuses</option><option value="running">Running</option><option value="exited">Stopped</option>
        <option value="unhealthy">Unhealthy</option><option value="restarting">Restarting</option><option value="paused">Paused</option>
        <option value="created">Created</option><option value="dead">Failed</option><option value="starting">Starting</option>
      </select>
    </div>
    <div ref={viewport} class="table-tray" onScroll={event => setScrollTop(event.currentTarget.scrollTop)} aria-busy={inventory.pending()}>
      <table class="inventory-table">
        <thead><tr><th>Application</th><th>Status</th><th>Image</th><th>Ports</th><th>Created</th><th aria-label="Details" /></tr></thead>
        <tbody>
          <Show when={start() > 0}><tr class="virtual-spacer" aria-hidden="true"><td colspan="6" style={{ height: start() * ROW_HEIGHT + "px" }} /></tr></Show>
          <For each={visible()}>{item =>
            <tr class="container-row" role="button" tabindex="0" aria-label={"View " + item.name}
              onClick={() => setSelectedID(item.id)} onKeyDown={event => selectRow(event, item)}>
              <td><span class="identity"><span class={"container-mark " + (item.managed ? "owned" : "")}><Icon name={item.managed ? "ship" : "box"} /></span><span class="truncate" title={item.name}>{item.name}</span></span></td>
              <td><Status value={statusOf(item)} /></td>
              <td><span class="truncate image-name" title={item.image}>{item.image}</span></td>
              <td class="mono ports-cell"><span class="truncate" title={portLabel(item.ports)}>{portLabel(item.ports) || "None"}</span></td>
              <td class="date-cell">{createdDate(item.createdAt)}</td>
              <td class="row-chevron"><Icon name="chevron" /></td>
            </tr>
          }</For>
          <Show when={items().length > start() + WINDOW_ROWS}>
            <tr class="virtual-spacer" aria-hidden="true"><td colspan="6" style={{ height: (items().length - start() - WINDOW_ROWS) * ROW_HEIGHT + "px" }} /></tr>
          </Show>
          <Show when={!items().length}>
            <tr class="empty-row"><td colspan="6">
              <Show when={!inventory.pending()} fallback={<span class="loading-line" aria-label="Loading applications" />}>
                <div class="empty-state"><Icon name={searching() ? "search" : "box"} /><span>{searching() ? "No matching applications" : scope() === "managed" ? "No Shipper applications" : "No containers"}</span></div>
              </Show>
            </td></tr>
          </Show>
          <tr class="table-footer"><td colspan="6"><span>{items().length} {items().length === 1 ? "container" : "containers"}</span></td></tr>
        </tbody>
      </table>
    </div>
    <Show when={selected()}>{item => <ContainerDrawer container={item()} close={() => setSelectedID(null)} />}</Show>
  </div>;
}

