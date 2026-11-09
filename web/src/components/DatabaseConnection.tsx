import { Show, createSignal, onCleanup } from "solid-js";
import { post } from "../api";
import { useAuth } from "../context";
import { mediaQuery } from "../media";
import { Button, ErrorNotice, IconButton } from "./Controls";

export default function DatabaseConnection(props: { applicationId: string; deploymentId: string; preview?: string }) {
  const auth = useAuth();
  const compact = mediaQuery("(width < 640px)");
  const [uri, setURI] = createSignal(""), [busy, setBusy] = createSignal(false), [copied, setCopied] = createSignal(false), [error, setError] = createSignal("");
  const parts = () => {
    try {
      const value = new URL(props.preview ?? "");
      return { host: value.host, database: value.pathname.replace(/^\//, "") || "default", tls: value.searchParams.get("sslmode") ?? (value.protocol === "rediss:" ? "Required" : "") };
    } catch { return undefined; }
  };
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
    <div class="database-connection-head"><h2>Connection string</h2><span>Private network</span></div>
    <div class="database-connection-controls">
      <Show when={props.preview} fallback={<><input id="database-connection-string" class="input mono" aria-label="Connection string" readonly type={uri() ? "text" : "password"} value={uri() || "connection-string"} autocomplete="off" spellcheck={false} /><IconButton icon="eye" label={uri() ? "Hide connection string" : "Reveal connection string"} disabled={busy()} onClick={() => uri() ? setURI("") : void load(false)} /></>}>
        <Show when={compact()} fallback={<><div class="database-connection-field">
          <code class="database-connection-preview mono" aria-label="Connection string">{uri() || props.preview}</code>
        </div>
        <IconButton icon="eye" label={uri() ? "Hide connection string" : "Reveal connection string"} disabled={busy()} onClick={() => uri() ? setURI("") : void load(false)} /></>}>
          <dl class="database-connection-parts" aria-label="Connection string">
            <div><dt>Host</dt><dd class="mono">{parts()?.host ?? props.preview}</dd></div>
            <div><dt>Database</dt><dd class="mono">{parts()?.database ?? "default"}</dd></div>
            <Show when={parts()?.tls}><div><dt>TLS</dt><dd>{parts()!.tls === "require" ? "Required" : parts()!.tls}</dd></div></Show>
          </dl>
        </Show>
      </Show>
      <Button variant="primary" icon={copied() ? "check" : "copy"} disabled={busy()} aria-label="Copy full connection string" title="Copies the full connection string" onClick={() => void load(true)}>{copied() ? "Copied" : "Copy"}</Button>
    </div>
    <Show when={error()}><ErrorNotice message={error()} /></Show>
  </section>;
}
