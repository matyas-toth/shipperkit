import { For, Show, createUniqueId } from "solid-js";
import { Icon, type IconName } from "./Icon";

export interface Choice<T extends string> {
  value: T;
  label: string;
  icon: IconName;
  detail?: string;
  recommended?: boolean;
}

export function ChoiceCards<T extends string>(props: {
  label: string;
  value: T;
  choices: readonly Choice<T>[];
  change: (value: T) => void;
}) {
  const name = createUniqueId();
  return <fieldset class="choice-grid" aria-label={props.label}>
    <For each={props.choices}>{choice =>
      <label class="choice-card" classList={{ selected: props.value === choice.value }}>
        <input type="radio" name={name} value={choice.value} checked={props.value === choice.value}
          aria-label={choice.label} aria-describedby={choice.recommended ? name + "-recommended" : undefined}
          onChange={() => props.change(choice.value)} />
        <Icon name={choice.icon} class="choice-icon" />
        <Icon name="check" class="choice-check" />
        <span class="choice-label">{choice.label}</span>
        <Show when={choice.detail}><span class="choice-detail">{choice.detail}</span></Show>
        <Show when={choice.recommended}><span class="choice-recommendation" id={name + "-recommended"}>Detected</span></Show>
      </label>
    }</For>
  </fieldset>;
}

export function SegmentedChoice<T extends string>(props: {
  label: string;
  value: T;
  choices: readonly { value: T; label: string }[];
  change: (value: T) => void;
}) {
  const name = createUniqueId();
  return <fieldset class="segmented-choice" aria-label={props.label}>
    <legend>{props.label}</legend>
    <For each={props.choices}>{choice => <label classList={{ selected: choice.value === props.value }}>
      <input type="radio" name={name} value={choice.value} checked={choice.value === props.value}
        onChange={() => props.change(choice.value)} />
      <span>{choice.label}</span>
    </label>}</For>
  </fieldset>;
}
