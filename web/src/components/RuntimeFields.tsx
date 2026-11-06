import { For, Show, type JSX } from "solid-js";
import type { RuntimePolicy } from "../types";
import { Icon } from "./Icon";

export default function RuntimeFields(props: {
  value?: RuntimePolicy;
  change: (value: RuntimePolicy) => void;
  service?: boolean;
  connectivity?: JSX.Element;
  expanded?: boolean;
  memory?: JSX.Element;
}) {
  const change = (key: keyof RuntimePolicy, value: number | string) =>
    props.change({ ...props.value, [key]: value });
  const fields = () => [
    { key: "cpus", label: "CPU limit", max: 64, min: 0, step: 0.1, default: 1 },
    {
      key: "pids",
      label: "PID limit",
      max: 32768,
      min: 16,
      step: 1,
      default: 256,
    },
    {
      key: "startupSeconds",
      label: "Startup timeout (seconds)",
      max: 600,
      min: 1,
      step: 1,
      default: 45,
    },
    {
      key: "stopSeconds",
      label: "Stop grace (seconds)",
      max: 120,
      min: 1,
      step: 1,
      default: 10,
    },
    ...(!props.service
      ? [
          {
            key: "drainSeconds",
            label: "Traffic drain (seconds)",
            max: 60,
            min: 0,
            step: 1,
            default: 0,
          },
        ]
      : []),
  ];
  function limits(keys?: string[]) {
    return <For each={keys ? fields().filter(field => keys.includes(field.key)) : fields()}>{field =>
      <div class="field">
        <label for={"runtime-" + field.key}>{field.label}</label>
        <input id={"runtime-" + field.key} class="input" type="number"
          min={field.min} max={field.max} step={field.step}
          placeholder={props.service && ["cpus", "pids", "stopSeconds"].includes(field.key) ? "Compose configuration" : String(field.default)}
          value={props.value?.[field.key as keyof RuntimePolicy] || ""}
          onInput={e => change(field.key as keyof RuntimePolicy, Number(e.currentTarget.value) || 0)} />
      </div>
    }</For>;
  }
  const restart = () => <div class="field">
    <label for="runtime-restart">Restart policy</label>
    <select id="runtime-restart" class="input" value={props.value?.restart ?? ""} onChange={e => change("restart", e.currentTarget.value)}>
      <option value="">{props.service ? "Compose configuration" : "Unless stopped"}</option>
      <option value="unless-stopped">Unless stopped</option>
      <option value="on-failure">On failure</option>
      <option value="no">Never</option>
    </select>
  </div>;
  return <Show when={props.expanded} fallback={<details class="configuration runtime-configuration">
    <summary>Runtime limits<Icon name="down" /></summary>
    <div class="form-fields field-pairs configuration-fields">{props.connectivity}{limits()}{restart()}</div>
  </details>}>
    <div class="runtime-groups">
      <section class="runtime-group"><h4>Network and health</h4><div class="form-fields field-pairs">{props.connectivity}{limits(["startupSeconds"])}</div></section>
      <section class="runtime-group"><h4>Resources</h4><div class="form-fields field-pairs">{props.memory}{limits(["cpus"])}</div></section>
      <details class="configuration runtime-configuration"><summary>Advanced runtime<Icon name="down" /></summary><div class="form-fields field-pairs configuration-fields">{limits(["pids", "stopSeconds", "drainSeconds"])}{restart()}</div></details>
    </div>
  </Show>;
}
