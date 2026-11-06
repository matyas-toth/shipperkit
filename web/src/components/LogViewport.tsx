import { For, Show, createEffect, createMemo, createSignal, on, onCleanup, onMount, type JSX } from "solid-js";
import { Dynamic } from "solid-js/web";
import { mediaQuery } from "../media";

const lineHeight = 22;
const windowRows = 64;
const overscan = 5;

export function logPresentation() {
  const compact = mediaQuery("(width < 48rem)");
  const [wrap, setWrap] = createSignal<boolean>();
  const [details, setDetails] = createSignal<boolean>();
  return {
    wrap: () => wrap() ?? compact(),
    details: () => details() ?? !compact(),
    toggleWrap: () => setWrap(!(wrap() ?? compact())),
    toggleDetails: () => setDetails(!(details() ?? !compact())),
  };
}

export default function LogViewport<T extends object>(props: {
  label: string;
  rows: readonly T[];
  lineNumber: (row: T) => number;
  message: (row: T) => string;
  renderMessage?: (row: T) => JSX.Element;
  tone?: (row: T) => string | undefined;
  activate?: (row: T) => void;
  reveal?: number;
  details?: (row: T) => JSX.Element;
  wrap: boolean;
  empty?: string;
  ref: (node: HTMLDivElement) => void;
  onFollow: (follow: boolean) => void;
}) {
  let viewport!: HTMLDivElement, measure!: HTMLSpanElement;
  const [scroll, setScroll] = createSignal(0), [columns, setColumns] = createSignal(0), [revision, revise] = createSignal(0);
  let measured = new WeakMap<T, number>(), estimates = new WeakMap<T, number>();
  let resizeFrame = 0;
  const elements = new WeakMap<Element, T>();
  function measureColumns() {
    const charWidth = measure.offsetWidth / 32;
    if (!charWidth) return;
    const stacked = props.details && viewport.clientWidth < 640;
    const next = Math.max(1, Math.floor((viewport.clientWidth - (props.details && !stacked ? 40 : 7) * charWidth - 22) / charWidth));
    if (next !== columns()) {
      measured = new WeakMap();
      estimates = new WeakMap();
      setColumns(next);
    }
  }
  const sizes = new ResizeObserver(entries => {
    let changed = false;
    for (const entry of entries) {
      if (entry.target === viewport || entry.target === measure) {
        measureColumns();
      } else {
        const row = elements.get(entry.target);
        const height = entry.borderBoxSize[0]?.blockSize ?? entry.contentRect.height;
        if (row && height && measured.get(row) !== height) { measured.set(row, height); changed = true; }
      }
    }
    if (changed && props.wrap && !resizeFrame) resizeFrame = requestAnimationFrame(() => {
      resizeFrame = 0;
      revise(value => value + 1);
    });
  });
  onMount(() => { sizes.observe(viewport); sizes.observe(measure); });
  onCleanup(() => { sizes.disconnect(); cancelAnimationFrame(resizeFrame); });
  createEffect(() => {
    if (!props.wrap) return;
    // Detail mode changes the width available to the message without resizing the pane.
    measureColumns();
  });
  const offsets = createMemo(() => {
    if (!props.wrap) return undefined;
    revision();
    const rows = props.rows, width = columns() || 36;
    const result = new Float64Array(rows.length + 1);
    for (let i = 0; i < rows.length; i++) {
      const row = rows[i];
      let height = measured.get(row) ?? estimates.get(row);
      if (height === undefined) {
        const message = props.message(row);
        height = message.split("\n").reduce((n, line) => n + Math.max(1, Math.ceil(line.length / width)), 0) * lineHeight;
        estimates.set(row, height);
      }
      result[i + 1] = result[i] + height;
    }
    return result;
  });
  const start = createMemo(() => {
    const offsetsByRow = offsets();
    let first = Math.floor(scroll() / lineHeight);
    if (offsetsByRow) {
      let low = 0, high = props.rows.length;
      while (low < high) {
        const mid = (low + high + 1) >>> 1;
        if (offsetsByRow[mid] <= scroll()) low = mid;
        else high = mid - 1;
      }
      first = low;
    }
    return Math.max(0, Math.min(first - overscan, props.rows.length - windowRows));
  });
  const end = () => Math.min(props.rows.length, start() + windowRows);
  const offset = (index: number) => offsets()?.[index] ?? index * lineHeight;
  createEffect(on(() => props.reveal, number => {
    if (number === undefined) return;
    const index = props.rows.findIndex(row => props.lineNumber(row) === number);
    if (index < 0) return;
    const first = Math.max(0, index - 6);
    viewport.scrollTop = first ? offset(first) + parseFloat(getComputedStyle(viewport).paddingTop) : 0;
    setScroll(viewport.scrollTop);
    viewport.focus({ preventScroll: true });
  }, { defer: true }));
  return <div ref={node => { viewport = node; props.ref(node); }} class="log-viewport mono" classList={{ "is-wrapped": props.wrap }} role="log" aria-label={props.label} aria-live="off" tabindex="0"
    onScroll={e => {
      const el = e.currentTarget;
      setScroll(el.scrollTop);
      props.onFollow(el.scrollHeight - el.clientHeight - el.scrollTop < lineHeight * 2);
    }}>
    <span ref={measure} class="log-measure" aria-hidden="true">00000000000000000000000000000000</span>
    <div aria-hidden="true" style={{ height: offset(start()) + "px" }} />
    <For each={props.rows.slice(start(), end())}>{row => {
      const [node, setNode] = createSignal<HTMLElement>();
      createEffect(() => {
        const element = node();
        if (!props.wrap || !element) return;
        elements.set(element, row);
        sizes.observe(element);
        onCleanup(() => sizes.unobserve(element));
      });
      return <Dynamic component={props.activate ? "button" : "div"} ref={(element: HTMLElement) => setNode(element)}
        type={props.activate ? "button" : undefined} aria-label={props.activate ? "Show line " + props.lineNumber(row) + " in context" : undefined}
        onClick={props.activate ? () => props.activate?.(row) : undefined}
        class="log-line" data-tone={props.tone?.(row)} classList={{ "line-revealed": props.reveal === props.lineNumber(row) }}>
        <span class="log-number">{props.lineNumber(row)}</span>{props.details?.(row)}<span class="log-message">{props.renderMessage ? props.renderMessage(row) : props.message(row)}</span>
      </Dynamic>;
    }}</For>
    <div aria-hidden="true" style={{ height: offset(props.rows.length) - offset(end()) + "px" }} />
    <Show when={props.empty}><div class="log-empty">{props.empty}</div></Show>
  </div>;
}
