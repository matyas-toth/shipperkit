import { For, Show, createSignal, createUniqueId, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import { Icon } from "./Icon";
import { settleMenu } from "./menuMotion";

type Entry = {
  label: string;
  disabled: boolean;
};

type OpenMenu = {
  select: HTMLSelectElement;
  entries: Entry[];
  left: number;
  top: number;
  bottom: number;
  width: number;
  maxHeight: number;
  placement: "above" | "below";
  active: number;
};

const isFinePointer = () => window.matchMedia("(pointer: fine)").matches;
const usableSelect = (node: EventTarget | null): HTMLSelectElement | null =>
  node instanceof HTMLSelectElement && !node.multiple && !node.disabled && !node.classList.contains("app-section-select") ? node : null;

// The select stays the form control, label target and accessible value. The menu
// is its listbox popup, opened for pointer, keyboard and assistive technology.
export default function SelectMenu() {
  const [menu, setMenu] = createSignal<OpenMenu>();
  const listboxId = `select-menu-${createUniqueId()}`;
  const optionId = (index: number) => `${listboxId}-${index}`;
  let element: HTMLDivElement | undefined;
  let typeahead = "";
  let typeaheadTimer = 0;

  const clearAttributes = (select: HTMLSelectElement) => {
    select.setAttribute("aria-expanded", "false");
    select.removeAttribute("aria-controls");
    select.removeAttribute("aria-activedescendant");
  };

  const close = (focus = false) => {
    const current = menu();
    if (current) clearAttributes(current.select);
    setMenu(undefined);
    typeahead = "";
    if (focus && current) current.select.focus({ preventScroll: true });
  };

  const highlight = () =>
    requestAnimationFrame(() => element?.querySelector(".is-active")?.scrollIntoView({ block: "nearest" }));

  const commit = (current: OpenMenu, index: number) => {
    const option = current.select.options[index];
    if (!option || option.disabled) return;
    if (current.select.selectedIndex !== index) {
      current.select.selectedIndex = index;
      current.select.dispatchEvent(new Event("input", { bubbles: true }));
      current.select.dispatchEvent(new Event("change", { bubbles: true }));
    }
    current.select.setAttribute("aria-activedescendant", optionId(index));
    setMenu({ ...current, active: index });
    highlight();
  };

  const step = (current: OpenMenu, direction: number) => {
    let index = current.active;
    for (let count = 0; count < current.entries.length; count++) {
      index = (index + direction + current.entries.length) % current.entries.length;
      if (!current.entries[index].disabled) return index;
    }
    return current.active;
  };

  const open = (select: HTMLSelectElement, active?: number) => {
    if (!select.isConnected) return;
    const entries = Array.from(select.options).map(option => ({
      label: (option.label || option.text).trim(),
      disabled: option.disabled,
    }));
    if (!entries.length) return;
    const rect = select.getBoundingClientRect();
    const width = Math.min(Math.max(rect.width, 200), Math.min(360, window.innerWidth - 16));
    const estimated = Math.min(entries.length * 34 + 8, 320);
    const left = Math.min(Math.max(rect.left, 8), window.innerWidth - width - 8);
    const below = rect.bottom + 6;
    const above = window.innerHeight - rect.top + 6;
    const spaceBelow = window.innerHeight - rect.bottom - 8;
    const spaceAbove = rect.top - 8;
    const placeAbove = spaceBelow < estimated && spaceAbove > spaceBelow;
    const maxHeight = Math.min(320, Math.max(120, placeAbove ? spaceAbove : spaceBelow));
    let index = active ?? select.selectedIndex;
    if (index < 0 || index >= entries.length || entries[index].disabled) {
      index = entries.findIndex(entry => !entry.disabled);
    }
    if (index < 0) return;
    select.setAttribute("aria-haspopup", "listbox");
    select.setAttribute("aria-expanded", "true");
    select.setAttribute("aria-controls", listboxId);
    select.setAttribute("aria-activedescendant", optionId(index));
    setMenu({
      select,
      entries,
      left,
      top: below,
      bottom: above,
      width,
      maxHeight,
      placement: placeAbove ? "above" : "below",
      active: index,
    });
    requestAnimationFrame(() => {
      const node = element;
      const current = menu();
      if (!node || !current) return;
      settleMenu(node, current.maxHeight, highlight);
    });
  };

  onMount(() => {
    const pointer = (event: PointerEvent) => {
      if (event.button !== 0 || event.pointerType === "touch") return;
      const select = usableSelect(event.target);
      if (select) {
        if (!isFinePointer()) return;
        event.preventDefault();
        select.focus({ preventScroll: true });
        if (menu()?.select === select) close(true);
        else open(select);
        return;
      }
      const current = menu();
      if (!current) return;
      const target = event.target as Node | null;
      if (target && (element?.contains(target) || current.select.contains(target))) return;
      close();
    };

    const keyboard = (event: KeyboardEvent) => {
      const select = usableSelect(event.target);
      const current = menu();
      if (!current) {
        if (!select) return;
        const opens =
          event.key === "ArrowDown" ||
          event.key === "ArrowUp" ||
          event.key === " " ||
          event.key === "Enter" ||
          event.key === "F4" ||
          (event.altKey && event.key === "ArrowDown");
        if (!opens) return;
        event.preventDefault();
        open(select);
        return;
      }
      if (select !== current.select) return;
      switch (event.key) {
        case "ArrowDown":
        case "ArrowUp":
          event.preventDefault();
          commit(current, step(current, event.key === "ArrowDown" ? 1 : -1));
          break;
        case "Home":
        case "End": {
          event.preventDefault();
          const start = event.key === "Home" ? 0 : current.entries.length - 1;
          const index = current.entries[start].disabled ? step(current, event.key === "Home" ? 1 : -1) : start;
          commit(current, index);
          break;
        }
        case "PageDown":
        case "PageUp": {
          event.preventDefault();
          let index = current.active;
          for (let count = 0; count < 5; count++) index = step(current, event.key === "PageDown" ? 1 : -1);
          commit(current, index);
          break;
        }
        case "Enter":
        case " ":
          event.preventDefault();
          close(true);
          break;
        case "Escape":
          event.preventDefault();
          close(true);
          break;
        case "Tab":
          close();
          break;
        default:
          if (event.key.length === 1 && !event.metaKey && !event.ctrlKey && !event.altKey) {
            event.preventDefault();
            typeahead += event.key.toLowerCase();
            window.clearTimeout(typeaheadTimer);
            typeaheadTimer = window.setTimeout(() => { typeahead = ""; }, 600);
            const found = current.entries.findIndex(entry => !entry.disabled && entry.label.toLowerCase().startsWith(typeahead));
            if (found >= 0) commit(current, found);
          }
      }
    };

    const focusOut = (event: FocusEvent) => {
      const current = menu();
      if (!current) return;
      const next = event.relatedTarget as Node | null;
      if (next && (current.select.contains(next) || element?.contains(next))) return;
      close();
    };

    const scroll = (event: Event) => {
      if (!menu()) return;
      const target = event.target;
      if (target instanceof Node && element?.contains(target)) return;
      close();
    };

    const resize = () => close();
    const blur = () => close();

    document.addEventListener("pointerdown", pointer, true);
    document.addEventListener("keydown", keyboard, true);
    document.addEventListener("focusout", focusOut, true);
    document.addEventListener("scroll", scroll, true);
    window.addEventListener("resize", resize);
    window.addEventListener("blur", blur);
    onCleanup(() => {
      document.removeEventListener("pointerdown", pointer, true);
      document.removeEventListener("keydown", keyboard, true);
      document.removeEventListener("focusout", focusOut, true);
      document.removeEventListener("scroll", scroll, true);
      window.removeEventListener("resize", resize);
      window.removeEventListener("blur", blur);
      window.clearTimeout(typeaheadTimer);
    });
  });

  return (
    <Portal>
      <Show when={menu()}>
        {current => (
          <div
            ref={node => (element = node)}
            class="select-menu"
            id={listboxId}
            role="listbox"
            data-placement={current().placement}
            style={{
              left: `${Math.round(current().left)}px`,
              top: current().placement === "above" ? "auto" : `${Math.round(current().top)}px`,
              bottom: current().placement === "above" ? `${Math.round(current().bottom)}px` : "auto",
              width: `${Math.round(current().width)}px`,
              "max-height": `${current().maxHeight}px`,
            }}
          >
            <For each={current().entries}>
              {(entry, index) => (
                <div
                  role="option"
                  id={optionId(index())}
                  class="select-menu-option"
                  classList={{
                    "is-active": index() === current().active,
                    "is-selected": current().select.selectedIndex === index(),
                  }}
                  aria-selected={current().select.selectedIndex === index()}
                  aria-disabled={entry.disabled}
                  onPointerDown={event => event.preventDefault()}
                  onClick={() => {
                    if (entry.disabled) return;
                    commit(current(), index());
                    close(true);
                  }}
                >
                  <span>{entry.label}</span>
                  <Show when={current().select.selectedIndex === index()}>
                    <Icon name="check" />
                  </Show>
                </div>
              )}
            </For>
          </div>
        )}
      </Show>
    </Portal>
  );
}
