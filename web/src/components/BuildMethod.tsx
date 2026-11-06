import type { StaticConfig } from "../types";
import { ChoiceCards, type Choice } from "./Choices";

type Build = "auto" | "static" | "dockerfile" | "railpack" | "compose";
const choices: readonly Choice<Build>[] = [
  { value: "auto", label: "Automatic", icon: "sliders" },
  { value: "dockerfile", label: "Dockerfile", icon: "code" },
  { value: "railpack", label: "Railpack", icon: "box" },
  { value: "static", label: "Static site", icon: "globe" },
  { value: "compose", label: "Docker Compose", icon: "layers" },
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
  const methods = choices.map(choice => ({
    ...choice,
    get recommended() { return choice.value === props.detected; },
    get detail() { return choice.value === "auto" && props.detected ? methodLabel(props.detected) : undefined; },
  }));
  return <ChoiceCards<Build> label="Build method" visibleLegend={props.visibleLegend} value={props.value as Build}
    choices={methods.filter(c => props.allowCompose || c.value !== "compose")}
    change={props.change} />;
}
