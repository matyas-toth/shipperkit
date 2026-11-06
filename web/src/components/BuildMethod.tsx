import type { StaticConfig } from "../types";
import { For, createUniqueId } from "solid-js";

type Build = "auto" | "static" | "dockerfile" | "railpack" | "compose";
const choices: readonly { value: Build; label: string }[] = [
  { value: "auto", label: "Automatic" },
  { value: "dockerfile", label: "Dockerfile" },
  { value: "railpack", label: "Railpack" },
  { value: "static", label: "Static site" },
  { value: "compose", label: "Docker Compose" },
];
export const methodLabel = (method: StaticConfig["method"]) => ({
  auto: "Automatic", dockerfile: "Dockerfile", railpack: "Railpack", static: "Static site",
  compose: "Docker Compose", image: "Container image", inline: "Inline Dockerfile",
})[method];

export default function BuildMethod(props: {
  value: StaticConfig["method"];
  detected?: Exclude<Build, "auto">;
  allowCompose?: boolean;
  visibleLegend?: boolean;
  change: (method: StaticConfig["method"]) => void;
}) {
  const id = createUniqueId();
  return <div class="field build-method-field">
    <label for={id} classList={{ "sr-only": !props.visibleLegend }}>Build method</label>
    <select id={id} class="input" value={props.value} onChange={e => props.change(e.currentTarget.value as Build)}>
      <For each={choices.filter(c => props.allowCompose || c.value !== "compose")}>{choice =>
        <option value={choice.value}>{choice.value === "auto" && props.detected ? "Automatic (" + methodLabel(props.detected) + ")" : choice.label}</option>
      }</For>
    </select>
  </div>;
}
