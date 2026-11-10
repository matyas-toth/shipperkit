import { For, Show, createEffect, onCleanup, createMemo, createSignal, createUniqueId, untrack } from "solid-js";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import type { Application, Service, StaticConfig } from "../types";
import { bytes, formatDuration, operationTime } from "../data";
import { Button, ErrorNotice, IconButton, Status } from "./Controls";
import EditorActions from "./EditorActions";
import SchedulePicker from "./SchedulePicker";
import { ChoiceCards } from "./Choices";
import { Icon } from "./Icon";

interface Spec {
  kind: string;
  service: string;
  volume: string;
  engine: string;
  database: string;
  user: string;
  password: string;
  host: string;
  port: number;
  tls: string;
  destination: string;
  keep: number;
}
interface Plan {
  id: string;
  applicationId: string;
  name: string;
  config: Spec;
  enabled: boolean;
  schedule: string;
  timezone: string;
  nextAt: number;
}
interface Run {
  service?: string;
  kind?: string;
  id: string;
  planId: string;
  operation: string;
  state: string;
  createdAt: number;
  startedAt: number;
  finishedAt: number;
  error: string;
  bytes: number;
  checksum: string;
  objectKey: string;
  result: string;
  sourceId: string;
  verifiedAt: number;
}
interface Volume {
  applicationId: string;
  applicationName: string;
  name: string;
  logical: string;
  attached: boolean;
  running: boolean;
  restored: boolean;
  services: string[];
}
interface Destination {
  id: string;
  name: string;
  endpoint: string;
  region: string;
  bucket: string;
  prefix: string;
  accessKey?: string;
  secretKey?: string;
  private: boolean;
}
interface Protection {
  plans: Plan[];
  runs: Run[];
  destinations: Destination[];
  recipient: string;
  volumes: Volume[];
  volumeError: string;
  revision: string;
}
const when = (n: number) => n ? operationTime(n, true) : "";
const whenShort = (n: number) => n ? operationTime(n) : "";
const freshSpec = (): Spec => ({
  kind: "database",
  service: "",
  volume: "",
  engine: "postgres",
  database: "",
  user: "",
  password: "",
  host: "127.0.0.1",
  port: 0,
  tls: "prefer",
  destination: "",
  keep: 7,
});
export default function Backups(props: {
  application?: Application;
  services?: Service[];
  service?: string;
  save?: (config: StaticConfig) => Promise<boolean | void>;
  editorScope?: (name?: string) => void;
}) {
  const formId = createUniqueId(), auth = useAuth(),
    client = useQueryClient(),
    base = () =>
      props.application
        ? `/applications/${props.application.id}/protection`
        : "/protection";
  const query = createQuery(() => ({
    queryKey: props.application
      ? ["applications", props.application.id, "protection"]
      : ["protection"],
    queryFn: () => api<Protection>(base()),
    refetchInterval: (q) =>
      q.state.data?.runs.some(
        (r) => r.state === "queued" || r.state === "running",
      )
        ? 1500
        : 15000,
  }));
  const [editing, setEditing] = createSignal<Plan>(),
    [error, setError] = createSignal(""),
    [busy, setBusy] = createSignal(false),
    [downloading, setDownloading] = createSignal(""),
    [selected, setSelected] = createSignal<Run>(),
    [review, setReview] = createSignal<{
      revision: string;
      kind: string;
      database: string;
      volume: string;
      service: string;
      engine: string;
    }>(),
    [target, setTarget] = createSignal(""),
    [confirm, setConfirm] = createSignal(""),
    [identity, setIdentity] = createSignal(""),
    [password, setPassword] = createSignal(""),
    [restoreUser, setRestoreUser] = createSignal(""),
    [destination, setDestination] = createSignal<Destination>(),
    [importing, setImporting] = createSignal(false),
    [upload, setUpload] = createSignal<File>(),
    [attaching, setAttaching] = createSignal<Volume>(),
    [logical, setLogical] = createSignal(""),
    [deleting, setDeleting] = createSignal<Volume>();
  createEffect(() => props.editorScope?.(editing() ? editing()?.id ? "Edit schedule" : "New schedule" : importing() ? importLabel() : undefined));
  onCleanup(() => props.editorScope?.());
  const database = () => !!props.application?.config.database;
  const importLabel = () => database() ? "Create from dump" : "Import archive";
  const plans = () =>
    query.data?.plans.filter(
      (p) => (!props.service || p.config.service === props.service) && (!database() || p.config.kind === "database" || !!p.config.engine),
    ) ?? [];
  const volumes = () =>
    (query.data?.volumes ?? [])
      .filter((v) =>
        !props.application
          ? !v.attached
          : !props.service || v.services.includes(props.service) || v.restored,
      )
      .sort((a, b) => a.logical.localeCompare(b.logical) || a.name.localeCompare(b.name));
  const runs = createMemo(() => {
    const ids = new Set(plans().map((p) => p.id));
    const all = query.data?.runs ?? [];
    const scoped = props.service
      ? all.filter((r) => r.service === props.service || ids.has(r.planId))
      : all;
    return database() ? scoped.filter((r) => r.kind === "database" || ids.has(r.planId)) : scoped;
  });
  const update = (patch: Partial<Plan>) =>
      setEditing((p) => (p ? { ...p, ...patch } : p)),
    spec = (patch: Partial<Spec>) =>
      setEditing((p) => (p ? { ...p, config: { ...p.config, ...patch } } : p));
  async function refresh() {
    await query.refetch();
    await client.invalidateQueries({ queryKey: ["applications"] });
  }
  async function action(fn: () => Promise<void>) {
    if (busy()) return;
    setBusy(true);
    setError("");
    try {
      await fn();
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  function create() {
    const config = freshSpec();
    config.service = props.service ?? "";
    if (!props.application) config.kind = "instance";
    const database = props.application?.config.database;
    if (database) {
      config.engine = database.engine === "postgresql" ? "postgres" : database.engine;
      config.database = "app";
      if (["redis", "clickhouse", "sqlite"].includes(database.engine)) {
        config.kind = "volume";
        config.engine = database.engine;
        config.database = "";
        const volume = query.data?.volumes.find(row => row.logical === "data" && !row.restored);
        config.volume = volume?.name ?? "";
      }
    }
    if (props.application?.method !== "compose") config.service = "";
    setEditing({
      id: "",
      applicationId: props.application?.id ?? "",
      name: props.application?.config.database ? "Database backup" : props.application ? "Application data" : "Instance recovery",
      config,
      enabled: true,
      schedule: "",
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
      nextAt: 0,
    });
    setSelected(undefined);
  }
  async function save(e: SubmitEvent) {
    e.preventDefault();
    await action(async () => {
      await post(base() + "/plans", editing(), auth.session()?.csrfToken);
      setEditing(undefined);
    });
  }
  async function transfer(path: string, name: string, body?: unknown) {
    if (downloading()) return;
    setDownloading(name);
    setError("");
    try { await download(path, name, body); }
    catch (e) { setError((e as Error).message); }
    finally { setDownloading(""); }
  }
  async function download(path: string, name: string, body?: unknown) {
    if (body === undefined) {
      const response = await fetch("/api" + path, {
        method: "HEAD",
        credentials: "same-origin",
      });
      if (!response.ok)
        throw new Error(
          "The local archive is unavailable; retrieve its encrypted copy from the configured S3 destination",
        );
      const link = document.createElement("a");
      link.href = "/api" + path;
      link.download = name;
      document.body.append(link);
      link.click();
      link.remove();
      return;
    }
    const res = await fetch("/api" + path, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": auth.session()?.csrfToken ?? "",
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const value = await res.json();
      throw new Error(value.error?.message ?? "Download failed");
    }
    const url = URL.createObjectURL(await res.blob()),
      a = document.createElement("a");
    a.href = url;
    a.download = name;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  function choose(r: Run) {
    setSelected(r);
    setReview(undefined);
    setTarget("");
    setConfirm("");
    setIdentity("");
    setPassword("");
    setRestoreUser("");
    setEditing(undefined);
  }
  async function beginRestore(r: Run) {
    await action(async () => {
      const data = await post<NonNullable<ReturnType<typeof review>>>(
        base() + `/runs/${r.id}/restore`,
        { review: true },
        auth.session()?.csrfToken,
      );
      setReview(data);
      setConfirm("");
      setTarget(data.database ? data.database + "_recovered" : "");
    });
  }
  async function restore(e: SubmitEvent) {
    e.preventDefault();
    await action(async () => {
      await post(
        base() + `/runs/${selected()!.id}/restore`,
        {
          revision: review()!.revision,
          confirm: confirm(),
          target: target(),
          identity: identity(),
          password: password(),
          user: restoreUser(),
        },
        auth.session()?.csrfToken,
      );
      setReview(undefined);
      setSelected(undefined);
      setIdentity("");
      setPassword("");
    });
  }
  async function importFile(e: SubmitEvent) {
    e.preventDefault();
    await action(async () => {
      const form = new FormData();
      form.append("identity", identity());
      form.append("archive", upload()!);
      const response = await fetch("/api" + base() + "/import", {
        method: "POST",
        credentials: "same-origin",
        headers: { "X-CSRF-Token": auth.session()?.csrfToken ?? "" },
        body: form,
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body.error?.message ?? "Import failed");
      setImporting(false);
      setIdentity("");
      setUpload(undefined);
    });
  }
  const sourceVolumes = () => {
    const names = props.application?.method === "compose"
      ? (props.services ?? [])
          .flatMap((s) => s.volumes.map((v) => v.name))
          .filter((n) => n && !n.startsWith("."))
      : (props.application?.config.volumes.map((v) => v.name) ?? []);
    return [...new Set(names)].sort((a, b) => a.localeCompare(b));
  };
  const latestAttempt = createMemo(() => runs().find(run => run.operation === "backup"));
  const latestCopy = createMemo(() => runs().find(run => run.operation === "backup" && run.state === "succeeded"));
  const backupKinds: Record<string, string> = { instance: "Instance settings", database: "Database backup", volume: "Volume archive" };
  const copyKind = (run: Run) => backupKinds[run.kind || plans().find(plan => plan.id === run.planId)?.config.kind || ""] ?? "Encrypted archive";
  const attemptStatus = () => {
    const run = latestAttempt();
    if (!run) return { value: "unknown", label: "No backups" };
    if (run.state === "queued" || run.state === "running") return { value: "operating", label: "Backing up" };
    if (run.state !== "succeeded") return { value: "failed", label: "Last backup failed" };
    const plan = plans().find(item => item.id === run.planId);
    return plan?.enabled && plan.schedule && plan.nextAt > 0 && plan.nextAt < Date.now() / 1000
      ? { value: "warning", label: "Backup overdue" }
      : { value: "succeeded", label: "Backed up" };
  };
  return (
    <section class="backups-panel">
      <Show when={query.error || error()}>
        <ErrorNotice message={error() || query.error!.message} />
      </Show>
      <div class="section-toolbar toolbar-end">
        <Show when={!props.application && !editing() && !importing()}><h2>Recovery</h2></Show>
        <Show when={!editing() && !importing()}><div class="toolbar-actions">
          <Show when={props.application && !database()}>
            <Button
              variant="neutral"
              icon="upload"
              onClick={() => {
                setImporting(!importing());
                setSelected(undefined);
              }}
            >
              {importLabel()}
            </Button>
          </Show>
          <Button variant="primary" icon="plus" onClick={create} disabled={busy()}>
            New schedule
          </Button>
        </div></Show>
      </div>
      <Show when={!props.application && !editing() && query.data}>
        <div class="recovery-overview">
          <section class="recovery-copy" aria-label="Latest recovery copy">
            <h3>Latest recovery copy</h3>
            <Show when={latestCopy()} fallback={<Status value={attemptStatus().value} label={attemptStatus().label} />}>{copy =>
              <div class="recovery-copy-body">
                <strong>{copyKind(copy())}</strong>
                <time datetime={new Date((copy().finishedAt || copy().createdAt) * 1000).toISOString()}>{when(copy().finishedAt || copy().createdAt)}</time>
                <Status value={copy().objectKey ? "succeeded" : "warning"} label={copy().objectKey ? "Local and off-server" : "Local copy only"} />
                <Button small variant="ghost" onClick={() => choose(copy())}>Inspect latest<Icon name="chevron" /></Button>
                <Show when={attemptStatus().value !== "succeeded"}><Status value={attemptStatus().value} label={attemptStatus().label} /></Show>
              </div>
            }</Show>
          </section>
        <section class="backup-destinations">
          <div class="section-toolbar">
            <h3>Off-server storage</h3>
            <Button
              icon="plus"
              onClick={() =>
                setDestination({
                  id: "",
                  name: "",
                  endpoint: "",
                  region: "us-east-1",
                  bucket: "",
                  prefix: "",
                  private: false,
                })
              }
            >
              Add S3 destination
            </Button>
          </div>
          <Show when={query.data?.destinations.length}><div class="table-tray">
            <table class="inventory-table backups-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Bucket</th>
                </tr>
              </thead>
              <tbody>
                <For each={query.data?.destinations}>
                  {(d) => (
                    <tr>
                      <td>
                        <button
                          class="task-name"
                          onClick={() =>
                            setDestination({
                              ...d,
                              accessKey: "",
                              secretKey: "",
                            })
                          }
                        >
                          {d.name}
                        </button>
                      </td>
                      <td><span class="destination-bucket">{d.bucket}</span></td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div></Show>
          <Show when={destination()}>
            <form
              class="backup-editor"
              onSubmit={(e) => {
                e.preventDefault();
                void action(async () => {
                  await post(
                    "/protection/destinations",
                    destination(),
                    auth.session()?.csrfToken,
                  );
                  setDestination(undefined);
                });
              }}
            >
              <div class="form-fields">
                <For
                  each={
                    [
                      ["name", "Name"],
                      ["endpoint", "Endpoint"],
                      ["region", "Region"],
                      ["bucket", "Bucket"],
                      ["prefix", "Prefix"],
                      ["accessKey", "Access key"],
                      ["secretKey", "Secret key"],
                    ] as const
                  }
                >
                  {([key, label]) => (
                    <>
                      <label for={"s3-" + key}>{label}</label>
                      <input
                        id={"s3-" + key}
                        class="input"
                        required={
                          key !== "prefix" &&
                          ((!destination()?.id &&
                            (key === "accessKey" || key === "secretKey")) ||
                            (key !== "accessKey" && key !== "secretKey"))
                        }
                        type={
                          key === "secretKey" || key === "accessKey"
                            ? "password"
                            : "text"
                        }
                        autocomplete="off"
                        value={String(destination()![key] ?? "")}
                        onInput={(e) =>
                          setDestination((d) =>
                            d ? { ...d, [key]: e.currentTarget.value } : d,
                          )
                        }
                      />
                    </>
                  )}
                </For>
              </div>
              <details class="configuration">
                <summary>Private endpoint</summary>
                <label class="check-field">
                  <input
                    type="checkbox"
                    checked={destination()!.private}
                    onChange={(e) =>
                      setDestination((d) =>
                        d ? { ...d, private: e.currentTarget.checked } : d,
                      )
                    }
                  />
                  Allow private addresses and HTTP
                </label>
              </details>
              <div class="form-actions">
                <Button
                  variant="ghost"
                  onClick={() => setDestination(undefined)}
                >
                  Cancel
                </Button>
                <Button type="submit" disabled={busy()}>
                  Save destination
                </Button>
              </div>
            </form>
          </Show>
        </section>
          <div class="recovery-tools">
            <div class="recovery-tools-heading"><h3>Recovery tools</h3></div>
            <div class="recovery-tool-grid">
              <article class="recovery-tool">
                <div class="recovery-tool-row">
                  <span class="recovery-tool-icon"><Icon name="key" /></span>
                  <div class="recovery-tool-copy">
                    <h4>Recovery key</h4>
                    <p>Decrypts instance archives and encrypted backups.</p>
                  </div>
                  <Button
                    disabled={!!downloading()}
                    onClick={() => void transfer("/protection/key", "shipper-recovery-key.txt", {
                      confirm: "Download recovery key",
                    })}
                  >
                    {downloading() === "shipper-recovery-key.txt" ? "Downloading key" : "Download recovery key"}
                  </Button>
                </div>
              </article>
              <details class="recovery-tool recovery-restore">
                <summary class="recovery-tool-row">
                  <span class="recovery-tool-icon"><Icon name="terminal" /></span>
                  <div class="recovery-tool-copy">
                    <h4>Recover an installation</h4>
                    <p>Restore an archive onto a fresh host.</p>
                  </div>
                  <span class="recovery-tool-toggle">View command<Icon name="chevron" /></span>
                </summary>
                <div class="recovery-restore-body">
                  <pre class="recovery-command"><code>shipper restore --archive backup.age --identity shipper-recovery-key.txt --data-dir /new/shipper</code></pre>
                  <dl class="recovery-facts">
                    <div><dt>Target</dt><dd>Empty data directory</dd></div>
                    <div><dt>Applications</dt><dd>Stopped after recovery</dd></div>
                    <div><dt>Schedules</dt><dd>Disabled after recovery</dd></div>
                    <div><dt>Workload data</dt><dd>Separate database and volume backups</dd></div>
                  </dl>
                </div>
              </details>
            </div>
          </div>
        </div>
      </Show>
      <Show when={importing()}>
        <form class="backup-editor" onSubmit={importFile}>
          <div class="section-toolbar">
            <h3>Import encrypted backup</h3>
            <IconButton
              icon="close"
              label="Close import"
              ghost
              onClick={() => setImporting(false)}
            />
          </div>
          <div class="form-fields">
            <label for="backup-file">Archive</label>
            <input
              id="backup-file"
              class="input"
              type="file"
              accept=".age"
              required
              onChange={(e) => setUpload(e.currentTarget.files?.[0])}
            />
            <label for="import-key">Recovery key</label>
            <input
              id="import-key"
              class="input mono"
              type="password"
              autocomplete="off"
              placeholder="Current instance key"
              value={identity()}
              onInput={(e) => setIdentity(e.currentTarget.value)}
            />
          </div>
          <div class="form-actions">
            <Button type="submit" disabled={busy() || !upload()}>
              {importLabel()}
            </Button>
          </div>
        </form>
      </Show>
      <Show when={editing()}>
        {(p) => (
          <form id={formId} class="backup-editor" onSubmit={save}>
            <div class="section-toolbar">
              <h3>{p().id ? "Edit schedule" : "New schedule"}</h3>
              <div class="editor-heading-actions">
            <label class="check-field">
              <input
                type="checkbox"
                checked={editing()!.enabled}
                onChange={(e) => update({ enabled: e.currentTarget.checked })}
              />
              Enabled
            </label>
              <IconButton
                icon="close"
                label="Close backup"
                ghost
                onClick={() => setEditing(undefined)}
              />
              </div>
            </div>
            <div class="operational-editor-grid"><div class="backup-definition">
            <div class="form-fields field-pairs">
              <div class="field"><label for="backup-name">Name</label>
              <input
                id="backup-name"
                class="input"
                required
                maxlength="80"
                value={editing()!.name}
                onInput={(e) => update({ name: e.currentTarget.value })}
              /></div>
            </div>
            <Show when={props.application}>
              <ChoiceCards<"database" | "volume"> label="Backup type"
                value={editing()!.config.kind === "volume" ? "volume" : "database"}
                choices={[
                  { value: "database", label: "Database", icon: "database" },
                  { value: "volume", label: "Persistent volume", icon: "box" },
                ]}
                change={kind => spec({ kind })} />
              <div class="form-fields field-pairs">
                <Show when={editing()!.config.kind === "database"}>
                  <Show when={props.application?.method === "compose"}>
                    <div class="field"><label for="backup-service">Service</label>
                    <select
                      id="backup-service"
                      class="input"
                      required
                      value={editing()!.config.service}
                      onChange={(e) => spec({ service: e.currentTarget.value })}
                    >
                      <option value="">Choose a service</option>
                      <For each={props.services}>
                        {(s) => <option value={s.name}>{s.name}</option>}
                      </For>
                    </select></div>
                  </Show>
                  <div class="field"><label for="backup-engine">Engine</label>
                  <select
                    id="backup-engine"
                    class="input"
                    value={editing()!.config.engine}
                    onChange={(e) => spec({ engine: e.currentTarget.value })}
                  >
                    <option value="postgres">PostgreSQL</option>
                    <option value="mysql">MySQL</option>
                    <option value="mariadb">MariaDB</option>
                    <option value="mongodb">MongoDB</option>
                  </select></div>
                  <div class="field"><label for="backup-database">Database name</label>
                  <input
                    id="backup-database"
                    class="input"
                    required
                    value={editing()!.config.database}
                    onInput={(e) => spec({ database: e.currentTarget.value })}
                  /></div>
                </Show>
                <Show when={editing()!.config.kind === "volume"}>
                  <div class="field"><label for="backup-volume">Volume</label>
                  <select
                    id="backup-volume"
                    class="input"
                    required
                    value={editing()!.config.volume}
                    onChange={(e) => spec({ volume: e.currentTarget.value })}
                  >
                    <option value="">Choose a volume</option>
                    <For each={volumes()}>
                      {(v) => (
                        <option value={v.name}>
                          {v.logical}
                          {v.running ? " (stop writers first)" : ""}
                        </option>
                      )}
                    </For>
                  </select></div>
                </Show>
              </div>
            </Show>
            <details class="configuration">
              <summary><span>Storage and retention</span><span class="disclosure-value">{editing()!.config.destination ? "Local + " + (query.data?.destinations.find(d => d.id === editing()!.config.destination)?.name ?? "S3") : "Local disk"} · Keep {editing()!.config.keep}</span></summary>
              <div class="form-fields field-pairs configuration-fields">
                <div class="field"><label for="backup-destination">Destination</label>
                <select
                  id="backup-destination"
                  class="input"
                  value={editing()!.config.destination}
                  onChange={(e) => spec({ destination: e.currentTarget.value })}
                >
                  <option value="">Local disk</option>
                  <For each={query.data?.destinations}>
                    {(d) => <option value={d.id}>Local + {d.name}</option>}
                  </For>
                </select></div>
                <div class="field"><label for="backup-keep">Keep backups</label>
                <input
                  id="backup-keep"
                  class="input"
                  type="number"
                  min="1"
                  max="100"
                  value={editing()!.config.keep}
                  onInput={(e) => spec({ keep: Number(e.currentTarget.value) })}
                /></div>
              </div>
            </details>
            <Show when={editing()!.config.kind === "database"}>
              <details class="configuration">
                <summary><span>Connection</span><span class="disclosure-value">{editing()!.config.service || editing()!.config.user || "Container defaults"}</span></summary>
                <div class="form-fields field-pairs configuration-fields">
                  <div class="field"><label for="backup-user">User</label>
                  <input
                    id="backup-user"
                    class="input"
                    placeholder="From service"
                    value={editing()!.config.user}
                    onInput={(e) => spec({ user: e.currentTarget.value })}
                  /></div>
                  <div class="field"><label for="backup-password">Password</label>
                  <input
                    id="backup-password"
                    class="input"
                    type="password"
                    autocomplete="new-password"
                    placeholder="From service"
                    value={editing()!.config.password}
                    onInput={(e) => spec({ password: e.currentTarget.value })}
                  /></div>
                  <div class="field"><label for="backup-port">Port</label>
                  <input
                    id="backup-port"
                    class="input"
                    type="number"
                    min="0"
                    max="65535"
                    placeholder="Engine default"
                    value={editing()!.config.port || ""}
                    onInput={(e) =>
                      spec({ port: Number(e.currentTarget.value) })
                    }
                  /></div>
                  <div class="field"><label for="backup-tls">TLS</label>
                  <select
                    id="backup-tls"
                    class="input"
                    value={editing()!.config.tls}
                    onChange={(e) => spec({ tls: e.currentTarget.value })}
                  >
                    <option value="disable">Disabled</option>
                    <option value="prefer">Prefer TLS</option>
                    <option value="require">Require TLS</option>
                    <option value="verify-full">
                      Verify certificate and hostname
                    </option>
                  </select></div>
                </div>
              </details>
            </Show>
            </div><div class="backup-schedule"><details class="configuration" open={!!untrack(() => p().schedule)}>
              <summary>Schedule</summary>
              <SchedulePicker
                value={editing()!.schedule}
                timezone={editing()!.timezone}
                change={(schedule, timezone) => update({ schedule, timezone })}
              />
            </details></div></div>

            <EditorActions measure="form" inlineWhenFits>
              <Show when={p().id}>
                <Button
                  variant="ghost"
                  disabled={busy()}
                  onClick={() =>
                    void action(async () => {
                      await post(
                        base() + `/plans/${p().id}/delete`,
                        {},
                        auth.session()?.csrfToken,
                      );
                      setEditing(undefined);
                    })
                  }
                >
                  Delete plan
                </Button>
              </Show>
              <Button variant="ghost" onClick={() => setEditing(undefined)}>
                Cancel
              </Button>
              <Button type="submit" form={formId} variant="primary" disabled={busy()}>
                {busy() ? "Saving" : "Save schedule"}
              </Button>
            </EditorActions>
          </form>
        )}
      </Show>
      <Show when={!editing()}>
        <Show when={props.application && query.data}>
          <div class="backup-readiness" aria-label="Backup readiness">
            <dl>
              <div><dt>Backup status</dt><dd><Status value={attemptStatus().value} label={attemptStatus().label} /></dd></div>
              <Show when={latestCopy()}>{copy => <>
                <div><dt>Latest backup</dt><dd><strong>{whenShort(copy().finishedAt || copy().createdAt)}</strong><Button small variant="ghost" onClick={() => choose(copy())}>View backup</Button></dd></div>
                <div><dt>Destination</dt><dd>{copy().objectKey ? "Local and off-server" : "Local only"}</dd></div>
              </>}</Show>
            </dl>
          </div>
        </Show>
        <Show when={!props.application}><div class="backup-plans-heading"><h3>Scheduled backups</h3></div></Show>
        <div class="table-tray backup-plans" classList={{ "is-empty": !plans().length }}>
          <table class="inventory-table backups-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Next backup</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              <For each={plans()}>
                {(p) => (
                  <tr>
                    <td>
                      <button
                        class="task-name"
                        onClick={() => {
                          setSelected(undefined);
                          setEditing(JSON.parse(JSON.stringify(p)));
                        }}
                      >
                        {p.name}<Icon name="edit" />
                      </button>
                      <Show when={p.config.keep > 0}><span class="backup-plan-meta">{p.config.keep} day retention</span></Show>
                    </td>
                    <td>
                      {p.enabled && p.schedule
                        ? <><span class="backup-next-label">Next </span>{whenShort(p.nextAt)}</>
                        : p.enabled
                          ? "Manual"
                          : "Disabled"}
                    </td>
                    <td>
                      <Button
                        icon="play"
                        variant="ghost"
                        disabled={busy() || !p.enabled}
                        onClick={() =>
                          void action(async () => {
                            await post(
                              base() + `/plans/${p.id}/run`,
                              {},
                              auth.session()?.csrfToken,
                            );
                          })
                        }
                      >
                        Run now
                      </Button>
                    </td>
                  </tr>
                )}
              </For>
              <Show when={!plans().length}>
                <tr class="empty-row">
                  <td colspan="3"><div class="empty-backup-state"><span>No scheduled backups</span><Button small variant="ghost" icon="plus" onClick={() => {
                    create(); setEditing(plan => plan ? { ...plan, schedule: "0 3 * * *" } : plan);
                  }}>Add schedule</Button></div></td>
                </tr>
              </Show>
            </tbody>
          </table>
        </div>
      </Show>
      <Show when={!editing() && runs().length}>
        <section class="backup-history">
          <h3 class="backup-history-title">History</h3>
          <div class="table-tray">
            <table class="inventory-table backups-table">
              <thead>
                <tr>
                  <th>Backup</th>
                  <th>Status</th>
                  <th>Size</th>
                  <th aria-label="Details" />
                </tr>
              </thead>
              <tbody>
                <For each={runs()}>
                  {(r) => (
                    <tr
                      class="container-row"
                      classList={{ selected: selected()?.id === r.id }}
                      role="button"
                      tabindex="0"
                      aria-label={"Inspect backup " + r.id}
                      onClick={() => choose(r)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") { e.preventDefault(); choose(r); }
                      }}
                    >
                      <td>
                        <time class="backup-run-date" datetime={new Date((r.finishedAt || r.createdAt) * 1000).toISOString()}>{whenShort(r.finishedAt || r.createdAt)}</time>
                        <Show when={!database() || r.operation !== "backup"}><span class="backup-plan-name">{r.operation === "restore" ? "Restore copy" : r.operation === "delete-volume" ? "Delete volume" : (plans().find((p) => p.id === r.planId)?.name ?? "Imported backup")}</span></Show>
                      </td>
                      <td>
                        <Status
                          value={r.state}
                          label={
                            r.state === "succeeded"
                              ? r.operation === "delete-volume"
                                ? "Deleted"
                                : r.operation === "restore"
                                  ? r.verifiedAt
                                    ? r.kind === "volume"
                                      ? "Files checked"
                                      : "Database checked"
                                    : "Restored"
                                  : "Backed up"
                              : undefined
                          }
                        />
                        <Show when={r.state === "failed" && r.error}><span class="backup-error">{r.error}</span></Show>
                      </td>
                      <td class="mono">{r.bytes ? bytes(r.bytes) : ""}</td>
                      <td class="row-chevron"><Icon name="chevron" /></td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div>
        </section>
      </Show>
      <Show when={selected()} keyed>
        {(r) => (
          <section class="backup-detail">
            <div class="section-toolbar">
              <h3>
                {r.operation === "restore"
                  ? "Restore result"
                  : plans().find(plan => plan.id === r.planId)?.name ?? "Imported backup"}
                <time class="backup-detail-time">{when(r.createdAt)}</time>
              </h3>
              <IconButton
                icon="close"
                label="Close backup details"
                ghost
                onClick={() => setSelected(undefined)}
              />
            </div>
            <Show when={r.error}>
              <ErrorNotice message={r.error} />
            </Show>
            <dl class="settings-group">
              <div class="setting-row">
                <dt>Result</dt>
                <dd><Status value={r.state} label={r.state === "succeeded" ? "Backed up" : r.state === "failed" ? "Failed" : undefined} /></dd>
              </div>
              <div class="setting-row">
                <dt>Duration</dt>
                <dd>
                  {r.finishedAt && r.startedAt
                    ? formatDuration(r.finishedAt - r.startedAt)
                    : ""}
                </dd>
              </div>
              <Show when={r.checksum}>
                <div class="setting-row">
                  <dt>SHA-256</dt>
                  <dd class="mono backup-checksum">{r.checksum}</dd>
                </div>
                <div class="setting-row">
                  <dt>Off-server copy</dt>
                  <dd>{r.objectKey ? "Uploaded" : "Local only"}</dd>
                </div>
                <div class="setting-row">
                  <dt>Restore verification</dt>
                  <dd>{r.verifiedAt ? when(r.verifiedAt) : "Not tested"}</dd>
                </div>
              </Show>
            </dl>
            <Show when={r.operation === "backup" && r.state === "succeeded"}>
              <div class="toolbar-actions">
                <Button
                  disabled={!!downloading()}
                  onClick={() => void transfer(base() + `/runs/${r.id}/download`, `shipper-${r.id}.age`)}
                >
                  {downloading() === `shipper-${r.id}.age` ? "Downloading archive" : "Download archive"}
                </Button>
                <Show when={props.application}>
                  <Button
                    disabled={busy()}
                    onClick={() => void beginRestore(r)}
                  >
                    Restore a copy
                  </Button>
                </Show>
              </div>
            </Show>
            <Show when={review()}>
              <form class="restore-review" onSubmit={restore}>
                <h3>Restore a copy</h3>
                <strong class="review-application">{props.application?.name}</strong>
                <dl class="settings-group">
                  <div class="setting-row">
                    <dt>Source</dt>
                    <dd>{review()!.database || review()!.volume}</dd>
                  </div>
                  <div class="setting-row">
                    <dt>Backup</dt>
                    <dd><time datetime={new Date(r.createdAt * 1000).toISOString()}>{operationTime(r.createdAt, true)}</time> <code class="mono" title={r.id}>{r.id.slice(-8)}</code></dd>
                  </div>
                  <div class="setting-row">
                    <dt>Existing data</dt>
                    <dd>Preserved</dd>
                  </div>
                  <div class="setting-row">
                    <dt>Application switch</dt>
                    <dd>Requires a separate change</dd>
                  </div>
                </dl>
                <div class="form-fields field-pairs restore-fields">
                  <Show when={review()!.kind === "database"}>
                    <div class="field"><label for="restore-target">New database</label>
                    <input
                      id="restore-target"
                      class="input"
                      required
                      pattern="[a-zA-Z_][a-zA-Z0-9_]{0,62}"
                      value={target()}
                      onInput={(e) => setTarget(e.currentTarget.value)}
                    /></div>
                  </Show>
                  <div class="field"><label for="restore-confirm">Confirm application name</label>
                  <input
                    id="restore-confirm"
                    class="input"
                    required
                    value={confirm()}
                    onInput={(e) => setConfirm(e.currentTarget.value)}
                  /></div>
                </div>
                <details class="configuration">
                  <summary>Recovery credentials</summary>
                  <div class="form-fields configuration-fields">
                    <label for="restore-key">Recovery key</label>
                    <input
                      id="restore-key"
                      class="input mono"
                      type="password"
                      autocomplete="off"
                      placeholder="Current instance key"
                      value={identity()}
                      onInput={(e) => setIdentity(e.currentTarget.value)}
                    />
                    <Show when={review()!.kind === "database"}>
                      <label for="restore-user">User</label>
                      <input
                        id="restore-user"
                        class="input"
                        placeholder="Saved backup user"
                        value={restoreUser()}
                        onInput={(e) => setRestoreUser(e.currentTarget.value)}
                      />
                      <label for="restore-password">Password</label>
                      <input
                        id="restore-password"
                        class="input"
                        type="password"
                        autocomplete="new-password"
                        placeholder="Saved backup password"
                        value={password()}
                        onInput={(e) => setPassword(e.currentTarget.value)}
                      />
                    </Show>
                  </div>
                </details>
                <div class="form-actions">
                  <Button variant="ghost" onClick={() => setReview(undefined)}>
                    Cancel
                  </Button>
                  <Button
                    type="submit"
                    variant="primary"
                    disabled={busy() || confirm() !== props.application?.name}
                  >
                    {busy() ? "Restoring copy" : "Restore copy"}
                  </Button>
                </div>
              </form>
            </Show>
          </section>
        )}
      </Show>
      <Show when={!database()}><section class="retained-volumes">
        <h3>{props.application ? "Persistent volumes" : "Retained volumes"}</h3>
        <Show when={query.data?.volumeError}>
          <ErrorNotice message={query.data!.volumeError} />
        </Show>
        <div class="table-tray">
          <table class="inventory-table backups-table">
            <thead>
              <tr>
                <th>Volume</th>
                <th>State</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              <For each={volumes()}>
                {(v) => (
                  <tr>
                    <td class="mono">
                      {v.logical}
                      <Show when={!props.application}>
                        <span class="backup-time">{v.applicationName}</span>
                      </Show>
                      <Show when={v.name !== v.logical}><span class="backup-time">{v.name}</span></Show>
                    </td>
                    <td>
                      {v.running
                        ? "In use"
                        : v.attached
                          ? "Stopped"
                          : "Retained"}
                    </td>
                    <td>
                      <Show when={!v.attached}>
                        <Show when={props.application}>
                          <Button
                            variant="ghost"
                            onClick={() => {
                              setAttaching(v);
                              setLogical(sourceVolumes()[0] ?? "");
                            }}
                          >
                            Attach
                          </Button>
                        </Show>
                        <IconButton
                          icon="trash"
                          label={"Delete retained volume " + v.logical}
                          ghost
                          onClick={() => {
                            setDeleting(v);
                            setConfirm("");
                          }}
                        />
                      </Show>
                    </td>
                  </tr>
                )}
              </For>
              <Show when={!volumes().length}>
                <tr class="empty-row">
                  <td colspan="3">No persistent volumes</td>
                </tr>
              </Show>
            </tbody>
          </table>
        </div>
      </section></Show>
      <Show when={attaching()}>
        <form
          class="backup-editor"
          onSubmit={(e) => {
            e.preventDefault();
            void action(async () => {
              const config = JSON.parse(
                JSON.stringify(props.application!.config),
              );
              config.volumeSources = {
                ...config.volumeSources,
                [logical()]: attaching()!.name,
              };
              await props.save?.(config);
              setAttaching(undefined);
            });
          }}
        >
          <h3>Attach retained volume</h3>
          <div class="form-fields">
            <label for="attach-logical">Application volume</label>
            <select
              id="attach-logical"
              class="input"
              required
              value={logical()}
              onChange={(e) => setLogical(e.currentTarget.value)}
            >
              <For each={sourceVolumes()}>
                {(v) => <option value={v}>{v}</option>}
              </For>
            </select>
          </div>
          <div class="form-actions">
            <Button variant="ghost" onClick={() => setAttaching(undefined)}>
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={busy() || !props.save || !logical()}
            >
              Review attachment
            </Button>
          </div>
        </form>
      </Show>
      <Show when={deleting()}>
        <form
          class="backup-editor"
          onSubmit={(e) => {
            e.preventDefault();
            void action(async () => {
              await post(
                base() +
                  `/volumes/${encodeURIComponent(deleting()!.name)}/delete`,
                { confirm: confirm() },
                auth.session()?.csrfToken,
              );
              setDeleting(undefined);
            });
          }}
        >
          <h3>Delete retained data</h3>
          <div class="form-fields">
            <label for="delete-volume-confirm">Volume name</label>
            <input
              id="delete-volume-confirm"
              class="input mono"
              required
              value={confirm()}
              onInput={(e) => setConfirm(e.currentTarget.value)}
            />
            <span>Delete permanently</span>
            <output class="mono">{deleting()!.name}</output>
          </div>
          <div class="form-actions">
            <Button variant="ghost" onClick={() => setDeleting(undefined)}>
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={busy() || confirm() !== deleting()!.name}
            >
              Delete data
            </Button>
          </div>
        </form>
      </Show>
    </section>
  );
}
