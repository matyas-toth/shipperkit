import { useSearchParams } from "@solidjs/router";
import "./DatabaseResource.css";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { Show, createMemo, createSignal } from "solid-js";
import { api, post } from "../api";
import { useAuth } from "../context";
import { databaseIdentity } from "../data";
import type { Application, Deployment, Inventory, StaticConfig } from "../types";
import { Button, ConfirmDialog } from "./Controls";

// Credentials and engine options of a managed database. The user and database
// are fixed at creation because the images only read them while initializing.
export default function DatabaseSettings(props: { application: Application; config: StaticConfig; change: (database: NonNullable<StaticConfig["database"]>) => void; busy: boolean }) {
  const auth = useAuth();
  const client = useQueryClient();
  const [, setSearch] = useSearchParams();
  const [rotating, setRotating] = createSignal(false);
  const engine = () => props.application.config.database?.engine;
  const identity = () => databaseIdentity(props.application.config);
  const inventory = createQuery(() => ({ queryKey: ["inventory"], queryFn: () => api<Inventory>("/applications"), staleTime: 60_000, reconcile: "id" }));
  const connected = createMemo(() => {
    const own = props.application.config.networking?.applications ?? [];
    return (inventory.data?.items ?? []).filter(app => app.id !== props.application.id && (own.includes(app.id) || (app.config.networking?.applications ?? []).includes(props.application.id))).map(app => app.name);
  });
  async function rotate() {
    const path = `/applications/${props.application.id}/database/password`;
    const review = await post<{ revision: string }>(path, { review: true }, auth.session()?.csrfToken);
    const d = await post<Deployment>(path, { revision: review.revision }, auth.session()?.csrfToken);
    await client.invalidateQueries({ queryKey: ["applications", props.application.id] });
    setSearch({ tab: "deployments", deployment: d.id, section: undefined });
  }
  const consequence = () => connected().length
    ? `${connected().join(", ")} ${connected().length === 1 ? "keeps" : "keep"} the old password until you paste the new connection string into ${connected().length === 1 ? "its" : "their"} environment.`
    : "Applications using the current connection string lose access until you paste the new one.";
  return <div class="database-settings">
    <dl class="settings-group">
      <Show when={identity()}>{value => <>
        <div class="setting-row"><dt>User</dt><dd class="mono">{value().user}</dd></div>
        <div class="setting-row"><dt>Database</dt><dd class="mono">{value().database}</dd></div>
      </>}</Show>
      <Show when={engine() !== "sqlite"}>
        <div class="setting-row"><dt>Password</dt><dd>
          <span class="mono database-password-mask" aria-label="Hidden">••••••••</span>
          <Button small variant="neutral" icon="refresh" disabled={props.busy || !props.application.activeDeploymentId} onClick={() => setRotating(true)}>Rotate</Button>
        </dd></div>
      </Show>
    </dl>
    <Show when={engine() === "redis" && props.config.database}>{database =>
      <label class="check-field database-cache-option"><input type="checkbox" checked={!!database().cache} onChange={e => props.change({ ...database(), cache: e.currentTarget.checked })} />Use as a cache and evict the least recently used keys when memory runs out</label>
    }</Show>
    <Show when={rotating()}>
      <ConfirmDialog title="Rotate the database password?" confirm="Rotate and redeploy" danger message={consequence()} close={() => setRotating(false)} run={rotate} />
    </Show>
  </div>;
}
