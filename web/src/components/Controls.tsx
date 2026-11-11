import { createEffect, createSignal, createUniqueId, on, onCleanup, Show, splitProps, type JSX } from "solid-js";
import { Portal } from "solid-js/web";
import { Icon, type IconName } from "./Icon";
import { settleMenu } from "./menuMotion";
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

type ActionMenuPosition = {
  left: number;
  top: number;
  bottom: number;
  width: number;
  maxHeight: number;
  placement: "above" | "below";
};

export function ActionMenu(props: { label: string; icon?: IconName; scope?: string; class?: string; children: JSX.Element }) {
  const [menu, setMenu] = createSignal<ActionMenuPosition>();
  const menuId = `action-menu-${createUniqueId()}`;
  let trigger!: HTMLButtonElement;
  let element: HTMLDivElement | undefined;

  const items = () => Array.from(element?.querySelectorAll<HTMLButtonElement>(":scope > button:not(:disabled)") ?? []);

  const close = (restoreFocus = false) => {
    setMenu(undefined);
    if (restoreFocus) trigger.focus({ preventScroll: true });
  };

  const focusItem = (index: number) => {
    const list = items();
    if (!list.length) return;
    list[(index + list.length) % list.length].focus();
  };

  const show = (last = false) => {
    if (menu()) return;
    const rect = trigger.getBoundingClientRect();
    const width = Math.max(204, rect.width);
    const spaceBelow = window.innerHeight - rect.bottom - 8;
    const spaceAbove = rect.top - 8;
    const above = spaceBelow < 220 && spaceAbove > spaceBelow;
    const maxHeight = Math.min(320, Math.max(120, above ? spaceAbove : spaceBelow));
    setMenu({
      left: Math.min(Math.max(rect.right - width, 8), window.innerWidth - width - 8),
      top: rect.bottom + 6,
      bottom: window.innerHeight - rect.top + 6,
      width,
      maxHeight,
      placement: above ? "above" : "below",
    });
    requestAnimationFrame(() => {
      const node = element;
      const current = menu();
      if (!node || !current) return;
      const buttons = Array.from(node.querySelectorAll<HTMLButtonElement>(":scope > button"));
      for (const button of buttons) {
        button.setAttribute("role", "menuitem");
        button.tabIndex = -1;
      }
      const enabled = buttons.filter(button => !button.disabled);
      const target = last ? enabled[enabled.length - 1] : enabled[0];
      target?.focus({ preventScroll: true });
      settleMenu(node, current.maxHeight, () => target?.scrollIntoView({ block: "nearest" }));
    });
  };

  createEffect(on(() => props.scope, () => close(), { defer: true }));
  createEffect(() => {
    if (!menu()) return;
    const outside = (event: PointerEvent) => {
      if (event.target instanceof Node && (element?.contains(event.target) || trigger.contains(event.target))) return;
      close();
    };
    const focusOut = (event: FocusEvent) => {
      const next = event.relatedTarget as Node | null;
      if (next && (element?.contains(next) || trigger.contains(next))) return;
      close();
    };
    const scroll = (event: Event) => {
      if (event.target instanceof Node && element?.contains(event.target)) return;
      close();
    };
    const resize = () => close();
    const blur = () => close();
    document.addEventListener("pointerdown", outside, true);
    document.addEventListener("focusout", focusOut, true);
    document.addEventListener("scroll", scroll, true);
    window.addEventListener("resize", resize);
    window.addEventListener("blur", blur);
    onCleanup(() => {
      document.removeEventListener("pointerdown", outside, true);
      document.removeEventListener("focusout", focusOut, true);
      document.removeEventListener("scroll", scroll, true);
      window.removeEventListener("resize", resize);
      window.removeEventListener("blur", blur);
    });
  });

  return <span class={"application-actions " + (props.class ?? "")}>
    <button
      ref={trigger}
      type="button"
      class="action-trigger"
      aria-label={props.label}
      title={props.label}
      aria-haspopup="menu"
      aria-expanded={!!menu()}
      aria-controls={menu() ? menuId : undefined}
      onClick={() => (menu() ? close(true) : show())}
      onKeyDown={event => {
        if (event.key === "ArrowDown") {
          if (menu()) return;
          event.preventDefault();
          show();
        } else if (event.key === "ArrowUp") {
          if (menu()) return;
          event.preventDefault();
          show(true);
        } else if (event.key === "Escape" && menu()) {
          event.preventDefault();
          close(true);
        }
      }}
    >
      <Icon name={props.icon ?? "down"} />
    </button>
    <Show when={menu()}>{current => (
      <Portal>
        <div
          ref={node => (element = node)}
          id={menuId}
          class="action-menu"
          role="menu"
          aria-label={props.label}
          data-placement={current().placement}
          style={{
            left: `${Math.round(current().left)}px`,
            top: current().placement === "above" ? "auto" : `${Math.round(current().top)}px`,
            bottom: current().placement === "above" ? `${Math.round(current().bottom)}px` : "auto",
            width: `${Math.round(current().width)}px`,
            "max-height": `${current().maxHeight}px`,
          }}
          onKeyDown={event => {
            const list = items();
            if (!list.length) return;
            const index = list.indexOf(document.activeElement as HTMLButtonElement);
            if (event.key === "ArrowDown" || event.key === "ArrowUp") {
              event.preventDefault();
              focusItem(index + (event.key === "ArrowDown" ? 1 : -1));
            } else if (event.key === "Home" || event.key === "End") {
              event.preventDefault();
              focusItem(event.key === "Home" ? 0 : list.length - 1);
            } else if (event.key === "Escape") {
              event.preventDefault();
              close(true);
            } else if (event.key === "Tab") {
              event.preventDefault();
              close(true);
            }
          }}
          onClick={event => {
            if (event.target instanceof Element && event.target.closest("button")) close(true);
          }}
        >
          {props.children}
        </div>
      </Portal>
    )}</Show>
  </span>;
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

