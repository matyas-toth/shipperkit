import { For, Show, createSignal } from "solid-js";
import { useQueryClient } from "@tanstack/solid-query";
import { post } from "../api";
import { useAuth } from "../context";
import { Button, ErrorNotice, IconButton } from "./Controls";
import { ChoiceCards } from "./Choices";
import type { Connection } from "../types";
import { useConnections } from "./SourceFields";
export default function Connections() {
  const auth = useAuth(),
    client = useQueryClient(),
    query = useConnections(),
    [error, setError] = createSignal(""),
    [busy, setBusy] = createSignal(false),
    [connecting, setConnecting] = createSignal(false),
    [account, setAccount] = createSignal("personal"),
    [organization, setOrganization] = createSignal(""),
    [editing, setEditing] = createSignal(false),
    [editingId, setEditingId] = createSignal(""),
    [disconnect, setDisconnect] = createSignal<Connection>(),
    [webhook, setWebhook] = createSignal<{ secret: string; path: string }>(),
    [kind, setKind] = createSignal("github_token"),
    [name, setName] = createSignal(""),
    [host, setHost] = createSignal("docker.io"),
    [token, setToken] = createSignal(""),
    [username, setUsername] = createSignal(""),
    [password, setPassword] = createSignal(""),
    [appID, setAppID] = createSignal(""),
    [installation, setInstallation] = createSignal(""),
    [key, setKey] = createSignal(""),
    [allowPrivate, setAllowPrivate] = createSignal(false);
  let connectButton: HTMLButtonElement | undefined;
  function closeConnect() {
    setConnecting(false);
    setError("");
    queueMicrotask(() => connectButton?.focus());
  }
  async function connect(event: SubmitEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const result = await post<{ action: string; manifest: unknown }>(
        "/connections/github",
        { organization: account() === "organization" ? organization().trim() : "" },
        auth.session()?.csrfToken,
      );
      const form = document.createElement("form");
      form.method = "POST";
      form.action = result.action;
      const input = document.createElement("input");
      input.type = "hidden";
      input.name = "manifest";
      input.value = JSON.stringify(result.manifest);
      form.append(input);
      document.body.append(form);
      form.submit();
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }
  function edit(c: Connection) {
    setConnecting(false);
    setEditingId(c.id);
    setName(c.name);
    setKind(c.kind);
    setHost(c.host);setUsername(c.username??"");setAllowPrivate(c.allowPrivate??false);
    setToken("");
    setPassword("");
    setKey("");
    setAppID("");
    setInstallation("");
    setEditing(true);
  }
  async function hook(id: string) {
    try {
      setWebhook(
        await post<{ secret: string; path: string }>(
          `/connections/${id}/webhook`,
          {},
          auth.session()?.csrfToken,
        ),
      );
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function save(e: SubmitEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await post(
        "/connections",
        {
          id: editingId() || undefined,
          name: name(),
          kind: kind(),
          host: host(),
          secret: {
            token: token(),
            username: username(),
            password: password(),
            appId: Number(appID()),
            installationId: Number(installation()),
            privateKey: key(),
            allowPrivate: allowPrivate(),
          },
        },
        auth.session()?.csrfToken,
      );
      setToken("");
      setPassword("");
      setKey("");
      setEditing(false);
      await client.invalidateQueries({ queryKey: ["connections"] });
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function remove(id: string) {
    setError("");
    try {
      await post(`/connections/${id}/delete`, {}, auth.session()?.csrfToken);
      await client.invalidateQueries({ queryKey: ["connections"] });
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <section class="settings-section" id="sources">
      <div class="section-toolbar connections-toolbar">
        <h2>Connections</h2>
        <Show when={!connecting()}><Button ref={connectButton} icon="git" variant="primary" disabled={busy()} onClick={() => {
          setEditing(false); setWebhook(undefined); setError(""); setConnecting(true);
        }}>
          Connect GitHub
        </Button></Show>
      </div>
      <Show when={error() || query.error}>
        <ErrorNotice message={error() || query.error!.message} />
      </Show>
      <Show when={connecting()}>
        <form class="connection-form github-connect-form" aria-label="Connect GitHub" onSubmit={connect}
          ref={form => queueMicrotask(() => form.querySelector<HTMLInputElement>('input[type="radio"]:checked')?.focus())}>
          <div class="form-fields">
            <ChoiceCards label="GitHub account" value={account()} change={setAccount} choices={[
              { value: "personal", label: "Personal account", icon: "user" },
              { value: "organization", label: "Organization", icon: "layers" },
            ]} />
            <Show when={account() === "organization"}>
              <div class="field">
              <label for="github-organization">Organization name</label>
              <input id="github-organization" class="input" required maxlength="39" pattern={"(?!.*--)[A-Za-z0-9]([A-Za-z0-9\\-]{0,37}[A-Za-z0-9])?"} autocomplete="off"
                value={organization()} onInput={event => setOrganization(event.currentTarget.value)} />
              </div>
            </Show>
          </div>
          <div class="form-actions">
            <Button variant="ghost" disabled={busy()} onClick={closeConnect}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={busy()}>{busy() ? "Connecting to GitHub" : "Continue to GitHub"}</Button>
          </div>
        </form>
      </Show>
      <div class="table-tray">
        <table class="inventory-table connection-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Provider</th>
              <th aria-label="Actions" />
            </tr>
          </thead>
          <tbody>
            <For each={query.data?.items}>
              {(c) => (
                <tr>
                  <td class="connection-name">{c.name}</td>
                  <td class="connection-provider">{c.kind === "registry" ? c.host : "GitHub"}</td>
                  <td class="connection-actions-cell"><div class="connection-actions">
                    <Button
                      small variant="ghost"
                      aria-label={"Edit " + c.name}
                      onClick={() => edit(c)}
                    >Edit</Button>
                    <Show when={c.kind !== "registry"}>
                      <Button
                        small variant="ghost"
                        aria-label={"Webhook for " + c.name}
                        onClick={() => void hook(c.id)}
                      >Webhook</Button>
                    </Show>
                    <IconButton
                      icon="trash"
                      label={"Disconnect " + c.name}
                      onClick={() => setDisconnect(c)}
                    />
                  </div></td>
                </tr>
              )}
            </For>
            <Show when={!query.data?.items.length}>
              <tr class="empty-row">
                <td colspan="3">No connections</td>
              </tr>
            </Show>
          </tbody>
        </table>
      </div>
      <Show when={disconnect()}>{item => <div class="inline-confirm" role="group" aria-label="Disconnect connection">
        <strong>Disconnect {item().name}?</strong>
        <Button variant="ghost" onClick={() => setDisconnect(undefined)}>Cancel</Button>
        <Button class="danger-button" onClick={async () => { const id = item().id; setDisconnect(undefined); await remove(id); }}>Disconnect</Button>
      </div>}</Show>
      <Show when={webhook()}>
        {(value) => (
          <div class="form-fields connection-form">
            <label for="source-webhook-url">Webhook URL</label>
            <input
              id="source-webhook-url"
              class="input mono"
              readonly
              value={location.origin + value().path}
            />
            <label for="source-webhook-secret">Webhook secret</label>
            <input
              id="source-webhook-secret"
              class="input mono"
              readonly
              value={value().secret}
            />
            <Button variant="ghost" onClick={() => setWebhook(undefined)}>
              Hide webhook
            </Button>
          </div>
        )}
      </Show>
      <Show
        when={editing()}
        fallback={
          <Button
            icon="plus"
            onClick={() => {
              setConnecting(false);
              setEditingId("");
              setKind("github_token");
 setUsername("");setAllowPrivate(false);
              setName("");
              setToken("");
              setPassword("");
              setKey("");
              setEditing(true);
            }}
          >
            Add custom connection
          </Button>
        }
      >
        <form onSubmit={save} class="connection-form">
          <div class="form-fields">
            <Show when={!editingId()}>
              <ChoiceCards label="Provider" value={kind()} change={setKind} choices={[
                {value:"github_token",label:"GitHub token",icon:"git"},
                {value:"github_app",label:"Existing GitHub App",icon:"layers"},
                {value:"registry",label:"Container registry",icon:"box"},
              ]} />
            </Show>
            <label for="connection-name">Name</label>
            <input
              id="connection-name"
              class="input"
              required
              maxlength="80"
              value={name()}
              onInput={(e) => setName(e.currentTarget.value)}
            />
            <Show when={kind() === "github_token"}>
              <label for="connection-token">Token</label>
              <input
                id="connection-token"
                class="input"
                type="password"
                required={!editingId()}
                placeholder={editingId() ? "Unchanged" : ""}
                value={token()}
                onInput={(e) => setToken(e.currentTarget.value)}
                autocomplete="off"
              />
            </Show>
            <Show when={kind() === "github_app"}>
              <label for="github-app-id">App ID</label>
              <input
                id="github-app-id"
                class="input"
                type="number"
                required={!editingId()}
                value={appID()}
                onInput={(e) => setAppID(e.currentTarget.value)}
              />
              <label for="github-installation-id">Installation ID</label>
              <input
                id="github-installation-id"
                class="input"
                type="number"
                required={!editingId()}
                value={installation()}
                onInput={(e) => setInstallation(e.currentTarget.value)}
              />
              <label for="github-private-key">Private key</label>
              <textarea
                id="github-private-key"
                class="input mono command-input"
                required={!editingId()}
                placeholder={editingId() ? "Unchanged" : ""}
                value={key()}
                onInput={(e) => setKey(e.currentTarget.value)}
              />
            </Show>
            <Show when={kind() === "registry"}>
              <label for="registry-host">Registry host</label>
              <input
                id="registry-host"
                class="input"
                required
                value={host()}
                onInput={(e) => setHost(e.currentTarget.value)}
              />
              <label for="registry-user">Username</label>
              <input
                id="registry-user"
                class="input"
                value={username()}
                onInput={(e) => setUsername(e.currentTarget.value)}
              />
              <label for="registry-password">Password or token</label>
              <input
                id="registry-password"
                class="input"
                type="password"
                value={password()}
                onInput={(e) => setPassword(e.currentTarget.value)}
                autocomplete="off"
              />
              <label class="check-field">
                <input
                  type="checkbox"
                  checked={allowPrivate()}
                  onChange={(e) => setAllowPrivate(e.currentTarget.checked)}
                />
                Allow private registry access
              </label>
            </Show>
          </div>
          <div class="form-actions">
            <Button variant="ghost" onClick={() => setEditing(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy()}>
              Save connection
            </Button>
          </div>
        </form>
      </Show>
    </section>
  );
}
