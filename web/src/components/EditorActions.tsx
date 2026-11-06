import { createContext, createSignal, onCleanup, onMount, Show, useContext, type Accessor, type JSX } from "solid-js";
import { Portal } from "solid-js/web";

export const EditorActionsContext = createContext<Accessor<HTMLDivElement | undefined>>();

export default function EditorActions(props: { docked?: boolean; measure?: "form"; inlineWhenFits?: boolean; children: JSX.Element }) {
  const target = useContext(EditorActionsContext);
  const [fits, setFits] = createSignal(false);
  let anchor: HTMLDivElement | undefined;
  onMount(() => {
    if (!props.inlineWhenFits || !anchor) return;
    const scroller = anchor.closest<HTMLElement>(".page-scroll");
    if (!scroller) return;
    function measure() {
      if (!anchor || !scroller) return;
      const dockHeight = target?.()?.getBoundingClientRect().height ?? 0;
      const top = anchor.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop;
      // Evaluate against the entire work area even when the dock is mounted.
      setFits(top + 78 <= scroller.clientHeight + dockHeight);
    }
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(scroller);
    if (anchor.parentElement) observer.observe(anchor.parentElement);
    if (scroller.firstElementChild) observer.observe(scroller.firstElementChild);
    onCleanup(() => observer.disconnect());
  });
  return <><Show when={props.inlineWhenFits}><div ref={anchor} class="editor-actions-anchor" /></Show>
  <Show when={props.docked !== false && (!props.inlineWhenFits || !fits()) && target?.()} fallback={<div class="form-actions" classList={{ "editor-actions-inline": !!props.inlineWhenFits }}>{props.children}</div>}>
    {mount => <Portal mount={mount()}><div class="editor-action-row" classList={{ "editor-action-form": props.measure === "form" }}>{props.children}</div></Portal>}
  </Show></>;
}
