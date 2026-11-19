// Native selects lose their selection when their <option> nodes are re-created, which
// happens whenever a background refetch hands a <For> fresh option objects. The browser
// then shows the first option while the component state still holds the chosen value,
// so the screen and the saved value disagree. This keeps every select on the value the
// app or the person last chose, whenever its options change.

const chosen = new WeakMap<HTMLSelectElement, string>();
let installed = false;

export function keepSelectValues() {
  if (installed || typeof window === "undefined") return;
  installed = true;
  const descriptor = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value");
  if (!descriptor?.set || !descriptor.get) return;
  const read = descriptor.get, write = descriptor.set;
  Object.defineProperty(HTMLSelectElement.prototype, "value", {
    configurable: true,
    enumerable: descriptor.enumerable,
    get() { return read.call(this); },
    set(value: string) { chosen.set(this, String(value)); write.call(this, value); },
  });
  document.addEventListener("change", event => {
    if (event.target instanceof HTMLSelectElement) chosen.set(event.target, read.call(event.target));
  }, true);
  const restore = (select: HTMLSelectElement) => {
    const value = chosen.get(select);
    if (value === undefined || read.call(select) === value) return;
    if (Array.from(select.options).some(option => option.value === value)) write.call(select, value);
  };
  new MutationObserver(records => {
    let selects: Set<HTMLSelectElement> | undefined;
    for (const record of records) {
      const target = record.target;
      const select = target instanceof HTMLSelectElement ? target
        : target instanceof HTMLOptGroupElement || target instanceof HTMLOptionElement ? target.closest("select") : null;
      if (select) (selects ??= new Set()).add(select);
    }
    selects?.forEach(restore);
  }).observe(document.documentElement, { childList: true, subtree: true });
}
