import { For, Show, createUniqueId } from "solid-js";
import { Icon, type IconName } from "./Icon";

export interface Choice<T extends string> {
  value: T;
  label: string;
  icon: IconName;
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
        <Show when={choice.recommended}><span class="choice-recommendation" id={name + "-recommended"}>Recommended</span></Show>
      </label>
    }</For>
  </fieldset>;
}
