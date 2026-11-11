import { For, Show, createSignal, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import { Icon } from "./Icon";

type Entry = {
  label: string;
  disabled: boolean;
  selected: boolean;
};

type OpenMenu = {
  select: HTMLSelectElement;
  entries: Entry[];
  left: number;
  top: number;
  width: number;
};

const isFinePointer = () => window.matchMedia("(pointer: fine)").matches;
const usableSelect = (node: EventTarget | null): HTMLSelectElement | null =>
  node instanceof HTMLSelectElement && !node.multiple && !node.disabled && !node.classList.contains("app-section-select") ? node : null;

// Pointer users get the designed menu. Keyboard and assistive technology keep the
// native select and its platform picker, which remains the accessible control.
export default function SelectMenu() {
  const [menu, setMenu] = createSignal<OpenMenu>();
  let element: HTMLDivElement | undefined;

  const close = (focus = false) => {
    const current = menu();
    setMenu(undefined);
    if (focus && current) current.select.focus({ preventScroll: true });
  };

  const open = (select: HTMLSelectElement) => {
    if (!select.isConnected) return;
    const entries = Array.from(select.options).map(option => ({
      label: (option.label || option.text).trim(),
      disabled: option.disabled,
      selected: option.selected,
    }));
    if (!entries.length) return;
    const rect = select.getBoundingClientRect();
    const width = Math.min(Math.max(rect.width, 200), Math.min(360, window.innerWidth - 16));
    const estimated = Math.min(entries.length * 36 + 10, 320);
    const left = Math.min(Math.max(rect.left, 8), window.innerWidth - width - 8);
    let top = rect.bottom + 6;
    if (top + estimated > window.innerHeight - 8 && rect.top - estimated - 6 > 8) {
      top = rect.top - estimated - 6;
    }
    setMenu({ select, entries, left, top, width });
    requestAnimationFrame(() => {
      const node = element;
      if (!node || !menu()) return;
      const bounds = select.getBoundingClientRect();
      let settled = bounds.bottom + 6;
      if (settled + node.offsetHeight > window.innerHeight - 8 && bounds.top - node.offsetHeight - 6 > 8) {
        settled = bounds.top - node.offsetHeight - 6;
      }
      node.style.top = `${Math.round(settled)}px`;
      node.querySelector(".is-selected")?.scrollIntoView({ block: "nearest" });
    });
  };

  const choose = (current: OpenMenu, index: number) => {
    const option = current.select.options[index];
    if (!option || option.disabled) return;
    if (current.select.selectedIndex !== index) {
      current.select.selectedIndex = index;
      current.select.dispatchEvent(new Event("input", { bubbles: true }));
      current.select.dispatchEvent(new Event("change", { bubbles: true }));
    }
    close(true);
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
      if (!menu()) return;
      if (event.key === "Shift" || event.key === "Control" || event.key === "Alt" || event.key === "Meta") return;
      close();
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
    });
  });

  return (
    <Portal>
      <Show when={menu()}>
        {current => (
          <div
            ref={node => (element = node)}
            class="select-menu"
            role="presentation"
            aria-hidden="true"
            style={{
              left: `${Math.round(current().left)}px`,
              top: `${Math.round(current().top)}px`,
              width: `${Math.round(current().width)}px`,
            }}
          >
            <For each={current().entries}>
              {(entry, index) => (
                <button
                  type="button"
                  tabindex="-1"
                  class="select-menu-option"
                  classList={{ "is-selected": entry.selected }}
                  disabled={entry.disabled}
                  onPointerDown={event => event.preventDefault()}
                  onClick={() => choose(current(), index())}
                >
                  <span>{entry.label}</span>
                  <Show when={entry.selected}>
                    <Icon name="check" />
                  </Show>
                </button>
              )}
            </For>
          </div>
        )}
      </Show>
    </Portal>
  );
}
