import { For, Show, createSignal, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import type { Container } from "../types";
import { createdDate, portLabel, statusOf } from "../data";
import { ErrorNotice, IconButton, Status } from "./Controls";
import { Icon } from "./Icon";

export default function ContainerDrawer(props: { container: Container; close: () => void }) {
  const [copied, setCopied] = createSignal(false);
  const [copyError, setCopyError] = createSignal("");
  let panel!: HTMLDivElement;
  let closeButton!: HTMLButtonElement;
  const previous = document.activeElement as HTMLElement | null;
  onMount(() => {
    closeButton.focus();
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") props.close();
      if (event.key === "Tab") {
        const nodes = panel.querySelectorAll<HTMLElement>("button,a[href],input,[tabindex='0']");
        const first = nodes[0], last = nodes[nodes.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    };
    document.addEventListener("keydown", keyboard);
    onCleanup(() => { document.removeEventListener("keydown", keyboard); previous?.focus(); });
  });
  const rows = () => [
    ["Image", props.container.image],
    ["Container ID", props.container.id],
    ["State", props.container.status],
    ["Health", props.container.health || "Not configured"],
    ["Ports", portLabel(props.container.ports) || "None"],
    ["Created", createdDate(props.container.createdAt)],
    ["Ownership", props.container.managed ? "Shipper" : "External"],
    ...(props.container.project ? [["Compose project", props.container.project]] : []),
    ...(props.container.service ? [["Service", props.container.service]] : []),
    ...(props.container.applicationId ? [["Application ID", props.container.applicationId]] : []),
  ];
  async function copy() { try { await navigator.clipboard.writeText(props.container.id); setCopied(true); } catch { setCopyError("The container ID could not be copied"); } }
  return <Portal>
    <div class="drawer-scrim" onClick={props.close} />
    <div ref={panel} class="drawer" role="dialog" aria-modal="true" aria-labelledby="container-title">
      <div class="drawer-toolbar">
        <Status value={statusOf(props.container)} />
        <div class="drawer-actions">
          <IconButton icon={copied() ? "check" : "copy"} label={copied() ? "Container ID copied" : "Copy container ID"} ghost onClick={() => void copy()} />
          <button ref={closeButton} class="button icon-button ghost" aria-label="Close container details" onClick={props.close}>
            <Icon name="close" />
          </button>
        </div>
      </div>
      <div class="drawer-scroll">
        <Show when={copyError()}><ErrorNotice message={copyError()} /></Show>
        <h2 id="container-title">{props.container.name}</h2>
        <dl class="details-list"><For each={rows()}>{([label, value]) =>
          <div><dt>{label}</dt><dd classList={{ mono: label === "Container ID" || label === "Application ID" }}>{value}</dd></div>
        }</For></dl>
      </div>
      <Show when={copied()}><span class="sr-only" role="status">Container ID copied</span></Show>
    </div>
  </Portal>;
}

