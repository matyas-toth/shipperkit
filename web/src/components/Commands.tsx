import { For, Show, createSignal, createUniqueId } from "solid-js";
import type { Command, Hook, StaticConfig } from "../types";
import { Icon } from "./Icon";
import { SegmentedChoice } from "./Choices";

const stages = [
  { stage: "before_deploy", label: "Before deploy" },
  { stage: "before_start", label: "Before start" },
  { stage: "after_deploy", label: "After deploy" },
  { stage: "before_stop", label: "Before stop" },
] as const;

export function commandStageLabel(stage: string) {
  return stages.find(item => item.stage === stage)?.label ?? (stage.charAt(0).toUpperCase() + stage.slice(1)).replaceAll("_", " ");
}

export const emptyCommand = (): Command => ({ shell: "", service: "", directory: "", user: "", timeout: 300, mode: "job" });
export const commandText = (command: Command) => command.argv ? JSON.stringify(command.argv) : command.shell;
const hasCommand = (command: Command) => !!(command.shell.trim() || command.argv?.length);

export function CommandFields(props: { value: Command; change: (command: Command) => void; label?: string; services?: string[]; execution?: boolean }) {
  const id = createUniqueId();
  const update = (next: Partial<Command>) => props.change({ ...props.value, ...next });
  function updateArguments(input: HTMLTextAreaElement) {
    try {
      const argv: unknown = JSON.parse(input.value);
      if (!Array.isArray(argv) || !argv.every(value => typeof value === "string")) throw new Error();
      update({ argv });
    } catch {
      input.setCustomValidity("Enter a JSON array of arguments");
    }
  }
  return <div class="command-fields form-fields" classList={{ "stage-editor": !!props.label }}>
    <Show when={props.services?.length}>
      <label for={id + "service"}>Service</label>
      <select id={id + "service"} class="input" value={props.value.service} onChange={e => update({ service: e.currentTarget.value })}>
        <option value="" disabled>Choose service</option>
        <For each={props.services}>{service => <option value={service}>{service}</option>}</For>
      </select>
    </Show>
    <label for={id + "command"} classList={{ "sr-only": !!props.label }}>{props.label ?? "Command"}</label>
    <Show when={!props.value.argv} fallback={
      <textarea id={id + "command"} class="input mono command-input" value={JSON.stringify(props.value.argv)}
        onChange={e => updateArguments(e.currentTarget)} onInput={e => e.currentTarget.setCustomValidity("")} />
    }>
      <textarea id={id + "command"} class="input mono command-input" value={props.value.shell}
        onInput={e => update({ shell: e.currentTarget.value })} />
    </Show>
    <details class="configuration command-options">
      <summary>
        <span>Execution</span>
        <span class="disclosure-value">{props.value.argv ? "Argument list" : "Shell"} · {props.value.timeout}s
          <Show when={props.execution}> · {props.value.mode === "exec" ? "Running service" : "Job container"}</Show>
        </span>
      </summary>
      <div class="command-options-body">
        <div class="command-modes"><SegmentedChoice label="Command format" value={props.value.argv ? "argv" : "shell"}
          choices={[{ value: "shell", label: "Shell" }, { value: "argv", label: "Argument list" }]}
          change={format => update({ shell: "", argv: format === "argv" ? [] : undefined })} />
        <Show when={props.execution}>
          <SegmentedChoice<Command["mode"]> label="Run in" value={props.value.mode}
            choices={[{ value: "job", label: "Job container" }, { value: "exec", label: "Running service" }]}
            change={mode => update({ mode })} />
        </Show></div>
        <div class="form-fields field-pairs command-execution-fields">
        <div class="field">
          <label for={id + "cwd"}>Working directory</label>
          <input id={id + "cwd"} class="input mono" placeholder="Image default" value={props.value.directory} onInput={e => update({ directory: e.currentTarget.value })} />
        </div>
        <div class="field">
          <label for={id + "user"}>User</label>
          <input id={id + "user"} class="input" placeholder="Image default" value={props.value.user} onInput={e => update({ user: e.currentTarget.value })} />
        </div>
        <div class="field">
          <label for={id + "timeout"}>Timeout (seconds)</label>
          <input id={id + "timeout"} class="input" type="number" min="1" max="3600" value={props.value.timeout} onInput={e => update({ timeout: Number(e.currentTarget.value) })} />
        </div></div>
      </div>
    </details>
  </div>;
}

export function LifecycleCommands(props: { config: StaticConfig; change: (hooks: Hook[]) => void; services?: string[]; compact?: boolean }) {
  const id = createUniqueId();
  const [selected, setSelected] = createSignal<(typeof stages)[number]>();
  const [service, setService] = createSignal(props.services?.[0] ?? props.config.hooks?.[0]?.command.service ?? "");
  const [emptyStageDrafts, setEmptyStageDrafts] = createSignal<Record<string, Command>>({});
  let editor: HTMLElement | undefined;
  const key = (stage: Hook["stage"]) => stage + ":" + service();
  const commandFor = (stage: Hook["stage"]) =>
    props.config.hooks?.find(hook => hook.stage === stage && hook.command.service === service())?.command ??
    emptyStageDrafts()[key(stage)] ?? { ...emptyCommand(), service: service(), mode: stage === "before_stop" ? "exec" : "job" };

  function update(stage: Hook["stage"], command: Command) {
    const hooks = (props.config.hooks ?? []).filter(hook => hook.stage !== stage || hook.command.service !== service());
    if (hasCommand(command)) hooks.push({ stage, command: { ...command, service: service() } });
    else setEmptyStageDrafts(drafts => ({ ...drafts, [key(stage)]: command }));
    props.change(hooks);
  }
  function select(item: (typeof stages)[number]) {
    const invalid = editor?.querySelector<HTMLTextAreaElement | HTMLInputElement>(":invalid");
    if (invalid) {
      const details = invalid.closest("details");
      if (details) details.open = true;
      invalid.reportValidity();
      return;
    }
    setSelected(selected()?.stage === item.stage ? undefined : item);
  }

  return <section class="lifecycle-settings">
    <Show when={!props.compact}><h2>Deployment commands</h2></Show>
    <Show when={props.config.method === "compose"}>
      <div class="field lifecycle-service">
        <label for={id + "service"}>Service</label>
        <input id={id + "service"} class="input" list={id + "services"} value={service()} onInput={e => setService(e.currentTarget.value)} />
        <datalist id={id + "services"}><For each={props.services}>{name => <option value={name} />}</For></datalist>
      </div>
    </Show>
    <div class="lifecycle-workspace" classList={{ "has-stage": !!selected() }}>
    <div class="lifecycle-stages" role="group" aria-label="Deployment stages">
      <For each={stages}>{item => {
        const command = () => commandFor(item.stage);
        return <button type="button" class="lifecycle-stage" aria-label={"Edit " + item.label.toLowerCase() + " command"}
          aria-pressed={selected()?.stage === item.stage} onClick={() => select(item)}>
            <strong>{item.label}</strong>
            <Show when={hasCommand(command())} fallback={<span class="stage-command stage-empty">No command</span>}>
              <code class="stage-command" title={commandText(command())}>{commandText(command())}</code>
            </Show>
            <Icon name="chevron" />
        </button>;
      }}</For>
    </div>
    <Show when={selected()} keyed>{item => <section ref={editor} class="lifecycle-editor" aria-label={item.label + " command"}>
      <h3>{item.label}</h3>
      <CommandFields value={commandFor(item.stage)} label={item.label} execution={item.stage === "after_deploy" || item.stage === "before_stop"}
        change={value => update(item.stage, value)} />
    </section>}</Show>
    </div>
  </section>;
}
