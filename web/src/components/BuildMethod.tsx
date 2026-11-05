import { Show, createSignal } from "solid-js";
import type { StaticConfig } from "../types";
import { ChoiceCards, type Choice } from "./Choices";

type Build = "static" | "dockerfile" | "railpack";
const choices: readonly Choice<Build>[] = [
  { value: "dockerfile", label: "Dockerfile", icon: "code" },
  { value: "railpack", label: "Railpack", icon: "box" },
  { value: "static", label: "Static site", icon: "globe" },
];
export const methodLabel = (method: StaticConfig["method"]) => ({
  auto: "Automatic", dockerfile: "Dockerfile", railpack: "Railpack", static: "Static site",
  compose: "Docker Compose", image: "Container image", inline: "Inline Dockerfile",
})[method];

export default function BuildMethod(props: {
  value: StaticConfig["method"];
  detected?: Build;
  change: (method: StaticConfig["method"]) => void;
}) {
  const [choosing, setChoosing] = createSignal(props.value !== "auto");
  const override = () => choosing() || props.value !== "auto";
  return <div class="build-method">
    <div class="build-mode">
      <div class="segments" role="group" aria-label="Build selection">
        <button type="button" aria-pressed={!override()} onClick={() => { setChoosing(false); props.change("auto"); }}>Automatic</button>
        <button type="button" aria-pressed={override()} onClick={() => setChoosing(true)}>Choose method</button>
      </div>
      <Show when={!override() && props.detected}><span class="detected-method">{methodLabel(props.detected!)} <span>recommended</span></span></Show>
    </div>
    <Show when={override()}>
      <ChoiceCards<Build> label="Build method" value={props.value as Build}
        choices={choices.map(choice => ({ ...choice, recommended: choice.value === props.detected }))}
        change={props.change} />
    </Show>
  </div>;
}
