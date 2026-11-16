import { Show, createEffect, createSignal } from "solid-js";
import { Icon } from "./Icon";

// ServiceLogo keeps the monogram as a fallback when the catalog has no asset or
// when a downloaded asset fails to decode.
export default function ServiceLogo(props: { logo?: string; mark: string; class?: string }) {
  const [failed, setFailed] = createSignal(false);
  createEffect(() => {
    props.logo;
    setFailed(false);
  });
  return <span class={"service-logo " + (props.class ?? "")} aria-hidden="true">
    <Show when={props.logo && !failed()} fallback={<Show when={props.mark} fallback={<Icon name="box" />}><span class="service-logo-mark">{props.mark}</span></Show>}>
      <img src={props.logo} alt="" loading="lazy" decoding="async" onError={() => setFailed(true)} />
    </Show>
  </span>;
}
