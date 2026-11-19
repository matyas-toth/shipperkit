import { For, createMemo } from "solid-js";
import { retainedValue } from "../env";
import "./EnvEditor.css";

// A plain textarea drawn over a highlighted copy of the same text, so typing, selection,
// undo and paste stay native while names, values and comments read like a code editor.
export default function EnvEditor(props: { value: string; label: string; invalidLine?: number; change: (value: string) => void }) {
  const lines = createMemo(() => props.value.split("\n"));
  const width = createMemo(() => Math.max(...lines().map(line => line.length)) + 2);
  return <div class="env-editor">
    <div class="env-editor-gutter" aria-hidden="true">
      <For each={lines()}>{(_, index) => <span classList={{ "is-invalid": props.invalidLine === index() + 1 }}>{index() + 1}</span>}</For>
    </div>
    <div class="env-editor-code">
      <pre class="env-editor-highlight" aria-hidden="true"><For each={lines()}>{line => <span class="env-line">{highlight(line)}{"\n"}</span>}</For></pre>
      <textarea class="env-editor-input" aria-label={props.label} spellcheck={false} autocomplete="off" autocapitalize="off" wrap="off"
        rows={lines().length} style={{ width: `max(100%, ${width()}ch)` }} value={props.value}
        onInput={event => props.change(event.currentTarget.value)}
        onKeyDown={event => {
          if (event.key !== "Tab" || event.shiftKey) return;
          event.preventDefault();
          const input = event.currentTarget, start = input.selectionStart;
          input.setRangeText("  ", start, input.selectionEnd, "end");
          props.change(input.value);
        }} />
    </div>
  </div>;
}

function highlight(line: string) {
  if (line.trimStart().startsWith("#")) return <span class="env-token-comment">{line}</span>;
  const match = /^(\s*)(export\s+)?([A-Za-z_][A-Za-z0-9_]*)(\s*=)(.*)$/.exec(line);
  if (!match) return line;
  const [, indent, keyword, name, equals, value] = match;
  return <>{indent}{keyword && <span class="env-token-keyword">{keyword}</span>}<span class="env-token-name">{name}</span><span class="env-token-operator">{equals}</span>
    <span classList={{ "env-token-value": true, "env-token-retained": value.trim() === retainedValue }}>{value}</span></>;
}
