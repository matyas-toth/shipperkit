import { For, Show, createMemo, createSignal, onMount } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { useInventory } from "../context";
import { applicationState, methodLabel, createdDate, primaryDomain, deploymentLabel } from "../data";
import { Icon } from "../components/Icon";
import { Button, ErrorNotice, IconButton, Status } from "../components/Controls";

const ROW_HEIGHT = 48, WINDOW_ROWS = 32;
export default function Applications() {
  const inventory = useInventory(), navigate = useNavigate();
  const [search, setSearch] = createSignal(""), [status, setStatus] = createSignal("all"), [scrollTop, setScrollTop] = createSignal(0);
  let viewport!: HTMLDivElement;
  onMount(() => { document.title = "Applications | Shipper"; });
  const items = createMemo(() => (inventory.data()?.items ?? []).filter(app =>
    (status() === "all" || applicationState(app) === status()) && (!search().trim() || [app.name, app.config.domain, app.sourceRef, ...(app.domains ?? []).map(d => d.hostname)].some(value => value?.toLowerCase().includes(search().trim().toLowerCase())))));
  const start = createMemo(() => Math.min(Math.max(0, Math.floor((scrollTop() - 32) / ROW_HEIGHT) - 5), Math.max(0, items().length - WINDOW_ROWS)));
  const visible = createMemo(() => items().slice(start(), start() + WINDOW_ROWS));
  function resetScroll() { if (viewport) viewport.scrollTop = 0; setScrollTop(0); }
  return <div class="content content-narrow">
    <header class="page-heading"><h1>Applications</h1><Button variant="primary" icon="plus" onClick={() => navigate("/new")}>New application</Button></header>
    <Show when={inventory.error()}><ErrorNotice message={inventory.error()!.message} retry={() => void inventory.refresh()} /></Show>
    <Show when={inventory.data() && !inventory.data()!.docker.connected}><ErrorNotice message={inventory.data()!.docker.error || "Docker is unavailable"} retry={() => void inventory.refresh()} /></Show>
    <div class="table-toolbar app-toolbar">
      <div class="search-field"><Icon name="search" /><input id="application-search" aria-label="Search applications" placeholder="Search applications" autocomplete="off" value={search()} onInput={e => { setSearch(e.currentTarget.value); resetScroll(); }} />
        <Show when={search()}><IconButton icon="close" label="Clear search" ghost onClick={() => { setSearch(""); resetScroll(); }} /></Show>
      </div>
      <select class="input status-filter" aria-label="Filter by status" value={status()} onChange={e => { setStatus(e.currentTarget.value); resetScroll(); }}>
        <option value="all">All statuses</option><option value="serving">Serving</option><option value="running">Running</option><option value="exited">Stopped</option><option value="failed">Failed</option><option value="degraded">Degraded</option><option value="queued">Queued</option><option value="cloning">Cloning</option><option value="building">Building</option><option value="routing">Updating domains</option><option value="operating">Applying action</option><option value="packaging">Packaging</option><option value="starting">Starting</option><option value="checking">Checking</option><option value="promoting">Promoting</option><option value="unavailable">Unavailable</option><option value="missing">Missing</option>
      </select>
    </div>
    <div ref={viewport} class="table-tray" onScroll={e => setScrollTop(e.currentTarget.scrollTop)} aria-busy={inventory.pending()}>
      <table class="inventory-table applications-table"><thead><tr><th>Application</th><th>Status</th><th>Domain</th><th>Method</th><th>Last activity</th><th aria-label="Details" /></tr></thead><tbody>
        <Show when={start() > 0}><tr class="virtual-spacer" aria-hidden="true"><td colspan="6" style={{ height: start() * ROW_HEIGHT + "px" }} /></tr></Show>
        <For each={visible()}>{app => <tr class="container-row" role="button" tabindex="0" aria-label={"View " + app.name} onClick={() => navigate("/applications/" + app.id)} onKeyDown={e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); navigate("/applications/" + app.id); } }}>
          <td><span class="identity"><Icon name={app.method === "static" ? "code" : "box"} /><span class="truncate" title={app.name}>{app.name}</span></span></td>
          <td><Status value={applicationState(app)} /></td><td class="domain-cell"><span class="truncate">{primaryDomain(app)?.hostname || "None"}</span></td><td class="muted">{methodLabel(app)}</td>
          <td class="date-cell"><Show when={app.latestDeployment} fallback="None">{d => <span class="deployment-cell"><Status value={d().state} label={deploymentLabel(d())} /><span>{createdDate(d().createdAt)}</span></span>}</Show></td><td class="row-chevron"><Icon name="chevron" /></td>
        </tr>}</For>
        <Show when={items().length > start() + WINDOW_ROWS}><tr class="virtual-spacer" aria-hidden="true"><td colspan="6" style={{ height: (items().length - start() - WINDOW_ROWS) * ROW_HEIGHT + "px" }} /></tr></Show>
        <Show when={!items().length}><tr class="empty-row"><td colspan="6"><Show when={!inventory.pending()} fallback={<span class="loading-line" aria-label="Loading applications" />}><div class="empty-state"><Icon name={search() ? "search" : "code"} /><span>{search() || status() !== "all" ? "No matching applications" : "No applications"}</span></div></Show></td></tr></Show>
        <tr class="table-footer"><td colspan="6">{items().length} {items().length === 1 ? "application" : "applications"}</td></tr>
      </tbody></table>
    </div>
  </div>;
}
