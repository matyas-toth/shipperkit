import { For, Show, createMemo, createSignal, onMount } from "solid-js";
import { useNavigate } from "@solidjs/router";
import type { Application } from "../types";
import { useInventory } from "../context";
import { useAuth } from "../context";
import { applicationState, methodLabel, operationTime, primaryDomain, deploymentLabel } from "../data";
import { Icon } from "../components/Icon";
import { Button, ErrorNotice, IconButton, Status } from "../components/Controls";
import { mediaQuery } from "../media";
import { docs } from "../docs";
import HighlightedText from "../components/HighlightedText";

const WINDOW_ROWS = 32;
export default function Applications() {
  const inventory = useInventory(), navigate = useNavigate();
  const auth = useAuth();
  const [search, setSearch] = createSignal(""), [status, setStatus] = createSignal("all"), [scrollTop, setScrollTop] = createSignal(0);
  let viewport!: HTMLDivElement;
  const compact = mediaQuery("(width < 48rem)");
  // Keep the virtualization stride in sync with the desktop and compact rows.
  const rowHeight = () => compact() ? 88 : 56;
  onMount(() => { document.title = "Applications | Shipper"; });
  const items = createMemo(() => (inventory.data()?.items ?? []).filter(app =>
    (status() === "all" || applicationState(app) === status() || status() === "running" && applicationState(app) === "serving") && (!search().trim() || [app.name, primaryDomain(app)?.hostname, methodLabel(app), app.sourceRef, ...(app.domains ?? []).map(d => d.hostname)].some(value => value?.toLowerCase().includes(search().trim().toLowerCase())))));
  const start = createMemo(() => Math.min(Math.max(0, Math.floor(scrollTop() / rowHeight()) - 5), Math.max(0, items().length - WINDOW_ROWS)));
  const visible = createMemo(() => items().slice(start(), start() + WINDOW_ROWS));
  function resetScroll() { if (viewport) viewport.scrollTop = 0; setScrollTop(0); }
  function open(app: Application) {
    const path = "/applications/" + app.id;
    const failed = applicationState(app) === "failed" && app.latestDeployment?.state === "failed" ? app.latestDeployment : undefined;
    navigate(failed ? path + "?tab=deployments&deployment=" + encodeURIComponent(failed.id) : path);
  }
  return <div class="content content-narrow applications-page">
    <header class="page-heading"><h1>Applications</h1></header>
    <Show when={inventory.error()}><ErrorNotice message={inventory.error()!.message} retry={() => void inventory.refresh()} /></Show>
    <Show when={inventory.data() && !inventory.data()!.docker.connected}><ErrorNotice message={inventory.data()!.docker.error || "Docker is unavailable"} retry={() => void inventory.refresh()} /></Show>
    <div class="table-toolbar app-toolbar">
      <div class="search-field"><Icon name="search" /><input id="application-search" aria-label="Search applications" placeholder="Search applications" autocomplete="off" value={search()} onInput={e => { setSearch(e.currentTarget.value); resetScroll(); }} />
        <Show when={search()}><IconButton icon="close" label="Clear search" ghost onClick={() => { setSearch(""); resetScroll(); }} /></Show>
      </div>
      <select class="input status-filter" aria-label="Filter by status" value={status()} onChange={e => { setStatus(e.currentTarget.value); resetScroll(); }}>
        <option value="all">All statuses</option><option value="running">Running</option><option value="exited">Stopped</option><option value="failed">Failed</option><option value="degraded">Degraded</option><option value="queued">Queued</option><option value="cloning">Cloning</option><option value="building">Building</option><option value="routing">Updating domains</option><option value="operating">Applying action</option><option value="packaging">Packaging</option><option value="starting">Starting</option><option value="checking">Checking</option><option value="promoting">Promoting</option><option value="unavailable">Unavailable</option><option value="missing">Missing</option>
      </select>
      <Show when={auth.session()?.user?.role !== "viewer"}><Button class="inventory-create" variant="primary" icon="plus" onClick={() => navigate("/new")}>New application</Button></Show>
    </div>
    <div ref={viewport} class="table-tray inventory-viewport" style={{ "--inventory-row": rowHeight() + "px" }} onScroll={e => setScrollTop(e.currentTarget.scrollTop)} aria-busy={inventory.pending()}>
      <table class="inventory-table applications-table"><thead><tr><th>Application</th><th>Status</th><th>Domain</th><th>Method</th><th>Last activity</th><th aria-label="Details" /></tr></thead><tbody>
        <Show when={start() > 0}><tr class="virtual-spacer" aria-hidden="true"><td colspan="6" style={{ height: start() * rowHeight() + "px" }} /></tr></Show>
        <For each={visible()}>{app => <tr class="container-row" role="button" tabindex="0" aria-label={"View " + app.name} onClick={() => open(app)} onKeyDown={e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); open(app); } }}>
          <td><span class="identity"><Icon name={app.config?.database ? "database" : app.method === "static" ? "code" : "box"} /><span class="truncate" title={app.name}><HighlightedText text={app.name} query={search()} /></span></span></td>
          <td><Status value={applicationState(app)} /></td><td class="domain-cell"><Show when={primaryDomain(app)?.hostname} fallback={<span class="no-value">None</span>}>{hostname => <span class="truncate" title={hostname()}><HighlightedText text={hostname()} query={search()} /></span>}</Show></td><td class="muted"><HighlightedText text={methodLabel(app)} query={search()} /></td>
          <td class="date-cell"><Show when={app.latestDeployment} fallback={<span class="no-value">None</span>}>{d => <span class="deployment-cell"><span class="activity-label">{deploymentLabel(d())}</span><time datetime={new Date(d().createdAt * 1000).toISOString()} title={new Date(d().createdAt * 1000).toLocaleString()}>{operationTime(d().createdAt)}</time></span>}</Show></td><td class="row-chevron"><Icon name="chevron" /></td>
        </tr>}</For>
        <Show when={items().length > start() + WINDOW_ROWS}><tr class="virtual-spacer" aria-hidden="true"><td colspan="6" style={{ height: (items().length - start() - WINDOW_ROWS) * rowHeight() + "px" }} /></tr></Show>
        <Show when={!items().length}><tr class="empty-row"><td colspan="6"><Show when={!inventory.pending()} fallback={<span class="loading-line" aria-label="Loading applications" />}><div class="empty-state"><Icon name={search() ? "search" : "code"} /><span>{search() || status() !== "all" ? "No matching applications" : "No applications"}</span><Show when={search() || status() !== "all"}><Button small onClick={() => { setSearch(""); setStatus("all"); resetScroll(); }}>Clear filters</Button></Show><Show when={!search() && status() === "all"}><a class="docs-link" href={docs("first-deploy/")} target="_blank" rel="noreferrer">Deployment guide</a></Show></div></Show></td></tr></Show>
        <tr class="table-footer"><td colspan="6">{items().length} {items().length === 1 ? "application" : "applications"}</td></tr>
      </tbody></table>
    </div>
  </div>;
}
