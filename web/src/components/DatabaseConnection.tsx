import { Show, createSignal, onCleanup } from "solid-js";
import { post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice, IconButton } from "./Controls";

export default function DatabaseConnection(props: { applicationId: string; deploymentId: string }) {
  const auth = useAuth();
  const [uri, setURI] = createSignal(""), [busy, setBusy] = createSignal(false), [copied, setCopied] = createSignal(false), [error, setError] = createSignal("");
  let mounted = true, timer: ReturnType<typeof setTimeout> | undefined;
  onCleanup(() => { mounted = false; clearTimeout(timer); });
  async function load(copy: boolean) {
    setBusy(true); setError("");
    try {
      const result = await post<{ uri: string; deploymentId: string }>(`/applications/${props.applicationId}/database/connection`, {}, auth.session()?.csrfToken);
      if (!mounted) return;
      if (result.deploymentId !== props.deploymentId) throw new Error("The database release changed; refresh the application");
      if (copy) {
        if (navigator.clipboard) await navigator.clipboard.writeText(result.uri);
        else {
          const field = document.createElement("textarea"), focused = document.activeElement as HTMLElement | null;
          field.value = result.uri; field.style.cssText = "position:fixed;opacity:0;pointer-events:none";
          document.body.append(field); field.select();
          try { if (!document.execCommand("copy")) throw new Error("Reveal the connection string to copy it manually"); }
          finally { field.remove(); focused?.focus({ preventScroll: true }); }
        }
        if (!mounted) return;
        setCopied(true); clearTimeout(timer); timer = setTimeout(() => setCopied(false), 2000);
      } else setURI(result.uri);
    } catch (e) { if (mounted) setError((e as Error).message); }
    finally { if (mounted) setBusy(false); }
  }
  return <section class="database-connection" aria-label="Database connection">
    <label for="database-connection-string">Connection string</label>
    <div class="database-connection-controls">
      <input id="database-connection-string" class="input mono" readonly type={uri() ? "text" : "password"} value={uri() || "connection-string"} autocomplete="off" spellcheck={false} />
      <IconButton icon="eye" label={uri() ? "Hide connection string" : "Reveal connection string"} disabled={busy()} onClick={() => uri() ? setURI("") : void load(false)} />
      <Button icon={copied() ? "check" : "copy"} disabled={busy()} aria-label="Copy connection string" onClick={() => void load(true)}>{copied() ? "Copied" : "Copy"}</Button>
    </div>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
  </section>;
}
