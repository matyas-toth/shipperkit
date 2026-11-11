import { Show, createSignal } from "solid-js";
import { useAuth } from "../context";
import { post } from "../api";
import { Button, ErrorNotice } from "../components/Controls";
import { Icon } from "../components/Icon";

export default function CliAuthorize() {
  const auth = useAuth();
  const query = new URLSearchParams(window.location.search);
  const state = query.get("state") || "";
  const port = Number(query.get("port"));
  const name = (query.get("name") || "this computer").slice(0, 60);
  const valid = /^[a-f0-9]{16,128}$/i.test(state) && Number.isInteger(port) && port >= 1024 && port <= 65535;
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  const admin = () => auth.session()?.user?.role === "admin";

  function finish(values: Record<string, string>) {
    const callback = new URL(`http://127.0.0.1:${port}/callback`);
    callback.searchParams.set("state", state);
    for (const [key, value] of Object.entries(values)) {
      if (value) callback.searchParams.set(key, value);
    }
    window.location.assign(callback.toString());
  }

  async function authorize(event: SubmitEvent) {
    event.preventDefault();
    if (busy() || !valid || !admin()) return;
    setBusy(true);
    setError("");
    try {
      const result = await post<{ token: string; credential: { id: string; expiresAt: number } }>(
        "/operation-tokens",
        { name: `CLI · ${name}`, scopes: ["full"], applications: [], days: 30 },
        auth.session()?.csrfToken,
      );
      finish({
        token: result.token,
        id: result.credential.id,
        expiresAt: String(result.credential.expiresAt),
        email: auth.session()?.user?.email || "",
      });
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }

  return (
    <section class="cli-authorize">
      <form class="auth-form auth-card" onSubmit={authorize}>
        <header class="auth-card-head">
          <span class="auth-badge"><Icon name="terminal" /></span>
          <h1>Authorize the CLI.</h1>
          <p class="auth-lede">A terminal on {name} wants to control this instance.</p>
        </header>
        <Show when={!valid}>
          <ErrorNotice message="This authorization link is incomplete. Start the CLI login again." />
        </Show>
        <Show when={valid}>
          <dl class="cli-facts">
            <div><dt>Instance</dt><dd>{window.location.host}</dd></div>
            <div><dt>Access</dt><dd>Full app access</dd></div>
            <div><dt>Expires</dt><dd>30 days</dd></div>
          </dl>
          <div class="auth-warning">
            <Icon name="warning" />
            <span>Only authorize a CLI you started. It can read and change every application until the token expires.</span>
          </div>
          <Show when={error()}><ErrorNotice message={error()} /></Show>
          <Show
            when={admin()}
            fallback={
              <>
                <ErrorNotice message="Only an administrator can authorize the CLI." />
                <div class="cli-actions">
                  <Button variant="ghost" onClick={() => finish({ error: "forbidden" })}>Close</Button>
                </div>
              </>
            }
          >
            <div class="cli-actions">
              <Button variant="primary" type="submit" disabled={busy()}>
                {busy() ? "Authorizing" : "Authorize CLI"}
              </Button>
              <Button variant="ghost" onClick={() => finish({ error: "denied", email: auth.session()?.user?.email || "" })}>
                Cancel
              </Button>
            </div>
          </Show>
        </Show>
        <p class="auth-foot">Signed in as {auth.session()?.user?.email}</p>
      </form>
    </section>
  );
}
