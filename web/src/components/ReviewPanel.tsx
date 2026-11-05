import { Show, type JSX } from "solid-js";
import { Button } from "./Controls";

export function ReviewPanel(props: { title: string; edit?: () => void; children: JSX.Element }) {
  return <section class="review-panel">
    <div class="review-panel-heading"><h2>{props.title}</h2>
      <Show when={props.edit}><Button small variant="ghost" aria-label={"Edit " + props.title.toLowerCase()} onClick={props.edit}>Edit</Button></Show>
    </div>
    <div class="review-panel-body">{props.children}</div>
  </section>;
}

export function ReviewFact(props: { label: string; children: JSX.Element }) {
  return <div class="review-fact"><dt>{props.label}</dt><dd>{props.children}</dd></div>;
}
