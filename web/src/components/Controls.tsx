import { createEffect, createSignal, on, onCleanup, splitProps, type JSX } from "solid-js";
import { Icon, type IconName } from "./Icon";
import { statusLabel } from "../data";

export function Button(props: JSX.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "neutral" | "ghost"; small?: boolean; icon?: IconName }) {
  const [local, rest] = splitProps(props, ["variant", "small", "icon", "class", "children"]);
  return <button type="button" class={"button " + (local.variant ?? "neutral") + (local.small ? " small" : "") + " " + (local.class ?? "")} {...rest}>
    {local.icon && <Icon name={local.icon} />}{local.children}
  </button>;
}

export function Disclosure(props: JSX.DetailsHtmlAttributes<HTMLDetailsElement> & { label: string; icon?: IconName }) {
  const [local, rest] = splitProps(props, ["label", "icon", "class", "children"]);
  return <details class={"configuration " + (local.class ?? "")} {...rest}>
    <summary><span class="disclosure-title">{local.icon && <Icon name={local.icon} />}{local.label}</span></summary>
    {local.children}
  </details>;
}

export function validateForm(form: HTMLFormElement) {
  let section = form.querySelector(":invalid")?.closest("details");
  while (section) {
    section.open = true;
    section = section.parentElement?.closest("details") ?? null;
  }
  return form.reportValidity();
}

export function ActionMenu(props: { label: string; icon?: IconName; scope?: string; class?: string; children: JSX.Element }) {
  const [open, setOpen] = createSignal(false);
  let details!: HTMLDetailsElement, trigger!: HTMLElement;
  function close(restoreFocus = false) {
    details.open = false;
    setOpen(false);
    if (restoreFocus) trigger.focus();
  }
  createEffect(on(() => props.scope, () => close(), { defer: true }));
  createEffect(() => {
    if (!open()) return;
    const outside = (event: PointerEvent) => { if (event.target instanceof Node && !details.contains(event.target)) close(); };
    document.addEventListener("pointerdown", outside, true);
    onCleanup(() => document.removeEventListener("pointerdown", outside, true));
  });
  return <details ref={details} class={"application-actions " + (props.class ?? "")} onToggle={e => setOpen(e.currentTarget.open)}
    onKeyDown={e => { if (e.key === "Escape" && open()) { e.preventDefault(); close(true); } }}>
    <summary ref={trigger} aria-label={props.label}><Icon name={props.icon ?? "down"} /></summary>
    <div class="action-menu" onClick={e => { if (e.target instanceof Element && e.target.closest("button")) close(true); }}>{props.children}</div>
  </details>;
}
export function IconButton(props: JSX.ButtonHTMLAttributes<HTMLButtonElement> & { icon: IconName; label: string; ghost?: boolean }) {
  const [local, rest] = splitProps(props, ["icon", "label", "ghost", "class"]);
  return <button type="button" class={"button icon-button " + (local.ghost ? "ghost" : "neutral") + " " + (local.class ?? "")}
    aria-label={local.label} title={local.label} {...rest}><Icon name={local.icon} /></button>;
}
export function Status(props: { value: string; label?: string }) {
  return <span class={"status status-" + props.value}><span class="status-dot" />{props.label ?? statusLabel(props.value)}</span>;
}
export function ErrorNotice(props: { message: string; retry?: () => void }) {
  return <div class="error-notice" role="alert"><Icon name="warning" /><span>{props.message}</span>
    {props.retry && <Button variant="ghost" small onClick={props.retry}>Retry</Button>}</div>;
}

