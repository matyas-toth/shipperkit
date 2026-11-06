import { For, Show, createMemo, createSignal, onCleanup, onMount } from "solid-js";
import { Portal } from "solid-js/web";
import type { ChangePlan, Command, Deployment, Hook, Variable } from "../types";
import { operationTime, deploymentRef } from "../data";
import { Button, ErrorNotice, IconButton } from "./Controls";
import { Icon } from "./Icon";
import { commandStageLabel, commandText } from "./Commands";
import { variableScopeLabel, variableSignature } from "../env";

const operationNames: Record<ChangePlan["operation"], string> = {
  rollback: "Restore release", metadata: "Update settings", rebuild: "Rebuild and deploy",
  recreate: "Recreate using the current image", none: "No changes",
};

const confirmNames: Record<ChangePlan["operation"], string> = {
  rollback: "Restore release", metadata: "Save settings", rebuild: "Rebuild and deploy",
  recreate: "Recreate application", none: "Apply changes",
};

const executionFacts = (command: Command): [string, string][] => [
  ["Mode", command.mode === "exec" ? "Running service" : "Job container"],
  ["Working directory", command.directory || "Image default"],
  ["User", command.user || "Image default"],
  ["Timeout", command.timeout + "s"],
];
const caution = (effect: string) => effect.includes("not reversed by release rollback") || effect.startsWith("Possible");

function CommandChange(props: { before?: Command; after?: Command }) {
  const details = () => {
    if (!props.after) return [];
    const before = props.before ? new Map(executionFacts(props.before)) : undefined;
    return executionFacts(props.after).flatMap(([label, value]) => {
      const old = before?.get(label);
      if (old === value || !before && value === "Image default") return [];
      return [{ label, before: old, after: value }];
    });
  };
  return <>
    <Show when={props.before && props.after && commandText(props.before) !== commandText(props.after)}>
      <div class="command-diff command-before"><span>Before</span><code>{commandText(props.before!)}</code></div>
    </Show>
    <Show when={props.after} fallback={<Show when={props.before}>
      <div class="command-diff command-before"><code>{commandText(props.before!)}</code></div>
    </Show>}>{command =>
      <div class="command-diff">
        <Show when={props.before && commandText(props.before) !== commandText(command())}><span>After</span></Show>
        <code>{commandText(command())}</code>
      </div>
    }</Show>
    <Show when={details().length}><dl class="command-review-facts"><For each={details()}>{field =>
      <div><dt>{field.label}</dt><dd><Show when={field.before}>{field.before} → </Show>{field.after}</dd></div>
    }</For></dl></Show>
  </>;
}

function ReleaseReference(props: { deployment?: Deployment }) {
  return <Show when={props.deployment} fallback="None">{release => <>
    <code title={release().commit || release().id}>{deploymentRef(release())}</code>
    <time datetime={new Date(release().createdAt * 1000).toISOString()}>{operationTime(release().createdAt, true)}</time>
  </>}</Show>;
}

export default function ChangeReview(props: {
  title?: string;
  confirm?: string;
  application?: string;
  release?: { serving?: Deployment; target: Deployment };
  hooks?: { before?: Hook[]; after?: Hook[] };
  variables?: { before?: Variable[]; after?: Variable[] };
  plan: ChangePlan;
  apply: () => Promise<void>;
  close: () => void;
}) {
  const [busy, setBusy] = createSignal(false), [error, setError] = createSignal("");
  const hookChanges = createMemo(() => {
    if (!props.hooks || !props.plan.changes.includes("Lifecycle commands")) return [];
    const key = (hook: Hook) => hook.stage + ":" + hook.command.service;
    const before = new Map(props.hooks.before?.map(hook => [key(hook), hook]));
    const after = new Map(props.hooks.after?.map(hook => [key(hook), hook]));
    return [...new Set([...before.keys(), ...after.keys()])].flatMap(id => {
      const current = before.get(id), next = after.get(id);
      if (JSON.stringify(current?.command) === JSON.stringify(next?.command)) return [];
      const hook = next ?? current!;
      return [{
        stage: commandStageLabel(hook.stage), service: hook.command.service,
        change: current ? next ? "Updated" : "Removed" : "Added", before: current?.command, after: next?.command,
      }];
    });
  });
  const variableChanges = createMemo(() => {
    if (!props.variables || !props.plan.changes.includes("Environment variables")) return [];
    const before = new Map(props.variables.before?.map(variable => [variable.name, variable]));
    const after = new Map(props.variables.after?.map(variable => [variable.name, variable]));
    return [...new Set([...before.keys(), ...after.keys()])].flatMap(name => {
      const current = before.get(name), next = after.get(name);
      if (current && next && variableSignature([current]) === variableSignature([next])) return [];
      return [{
        name, scope: variableScopeLabel((next ?? current)?.scope), change: current ? next ? "Updated" : "Removed" : "Added",
        before: current && !current.secret ? current.value : undefined,
        after: next && !next.secret ? next.value : undefined,
      }];
    });
  });
  let panel!: HTMLElement;
  const previous = document.activeElement as HTMLElement | null;
  onMount(() => {
    panel.querySelector<HTMLButtonElement>("button")?.focus();
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busy()) props.close();
      if (event.key !== "Tab") return;
      const nodes = [...panel.querySelectorAll<HTMLButtonElement>("button:not(:disabled)")];
      const first = nodes[0], last = nodes[nodes.length - 1];
      if (!first || !last) { event.preventDefault(); return; }
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener("keydown", key);
    onCleanup(() => { document.removeEventListener("keydown", key); previous?.focus(); });
  });
  async function apply() {
    if (busy()) return;
    setBusy(true);
    setError("");
    try { await props.apply(); }
    catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  return <Portal>
    <div class="drawer-scrim" onClick={() => !busy() && props.close()} />
    <section ref={panel} class="confirm-dialog change-review" role="dialog" aria-modal="true" aria-labelledby="review-title">
      <header><h2 id="review-title">{props.title ?? "Review changes"}</h2><IconButton icon="close" label="Close review" ghost disabled={busy()} onClick={props.close} /></header>
      <div class="change-review-content">
        <Show when={error()}><ErrorNotice message={error()} /></Show>
        <Show when={props.application}><strong class="review-application"><Icon name="box" />{props.application}</strong></Show>
        <Show when={props.release} fallback={<p class="change-operation">{operationNames[props.plan.operation]}</p>}>{release =>
          <dl class="release-comparison">
            <div><dt>Current release</dt><dd><ReleaseReference deployment={release().serving} /></dd></div>
            <div><dt>Restore to</dt><dd><ReleaseReference deployment={release().target} /></dd></div>
          </dl>
        }</Show>
        <Show when={props.plan.changes.length}><ul class="change-list">
          <For each={props.plan.changes}>{change => <li>
            {change === "Lifecycle commands" ? "Deployment commands" : change}
            <Show when={change === "Lifecycle commands" && hookChanges().length}>
              <For each={hookChanges()}>{hook => <div class="hook-review">
                <div class="hook-review-heading"><strong>{hook.stage}<Show when={hook.service}> · {hook.service}</Show></strong><span>{hook.change}</span></div>
                <CommandChange before={hook.before} after={hook.after} />
              </div>}</For>
            </Show>
            <Show when={change === "Environment variables" && variableChanges().length}>
              <dl class="hook-comparison"><For each={variableChanges()}>{variable => <div>
                <dt class="mono">{variable.name}</dt><dd>{variable.change} · {variable.scope}
                  <Show when={variable.change === "Updated" && (variable.before !== undefined || variable.after !== undefined)}>
                    <div class="variable-diff">
                      <div><span>Before</span><code>{variable.before || "None"}</code></div>
                      <div><span>After</span><code>{variable.after || "None"}</code></div>
                    </div>
                  </Show>
                </dd>
              </div>}</For></dl>
            </Show>
          </li>}</For>
        </ul></Show>
        <Show when={props.plan.effects?.length || props.plan.downtime}>
          <ul class="review-effects">
            <For each={props.plan.effects}>{effect => <li classList={{ caution: caution(effect) }}><Icon name={caution(effect) ? "warning" : "check"} />{effect}</li>}</For>
            <Show when={props.plan.downtime}><li classList={{ caution: caution(props.plan.downtime!) }}><Icon name={caution(props.plan.downtime!) ? "warning" : "clock"} />{props.plan.downtime}</li></Show>
          </ul>
        </Show>
      </div>
      <div class="form-actions">
        <Button variant="ghost" disabled={busy()} onClick={props.close}>{props.plan.operation === "rollback" ? "Cancel" : "Keep editing"}</Button>
        <Button variant="primary" icon="deploy" disabled={busy() || props.plan.operation === "none"} onClick={() => void apply()}>{busy() ? "Applying changes" : props.confirm ?? confirmNames[props.plan.operation]}</Button>
      </div>
    </section>
  </Portal>;
}
