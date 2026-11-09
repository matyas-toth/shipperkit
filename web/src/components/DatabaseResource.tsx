import { For, Show, createMemo, createSignal } from "solid-js";
import { post } from "../api";
import { useAuth } from "../context";
import type { StaticConfig } from "../types";
import type { IconName } from "./Icon";
import { Button, ErrorNotice } from "./Controls";
import { Icon } from "./Icon";
import "./DatabaseResource.css";

type Engine = { id: string; name: string; icon: IconName; versions: string[]; port: number };
const engines: Engine[] = [
  { id: "postgresql", name: "PostgreSQL", icon: "database", versions: ["18", "17", "16"], port: 5432 },
  { id: "mysql", name: "MySQL", icon: "server", versions: ["8.4"], port: 3306 },
  { id: "mariadb", name: "MariaDB", icon: "layers", versions: ["11.8", "11.4"], port: 3306 },
  { id: "mongodb", name: "MongoDB", icon: "database", versions: ["8", "7"], port: 27017 },
  { id: "redis", name: "Redis", icon: "bolt", versions: ["8", "7"], port: 6379 },
  { id: "clickhouse", name: "ClickHouse", icon: "layers", versions: ["25.3", "24.8"], port: 9000 },
  { id: "sqlite", name: "SQLite", icon: "code", versions: ["3.50"], port: 0 },
];

export default function DatabaseResource(props: { prepare: (config: StaticConfig, name: string) => void | Promise<void>; close: () => void }) {
  const auth = useAuth();
  const [engine, setEngine] = createSignal(engines[0]);
  const [version, setVersion] = createSignal(engines[0].versions[0]);
  const [name, setName] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  const selectedName = createMemo(() => name().trim() || engine().name);
  function choose(next: Engine) {
    setEngine(next);
    setVersion(next.versions[0]);
  }
  async function prepare(event: SubmitEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const result = await post<{ config: StaticConfig }>("/database-resources/prepare", { engine: engine().id, version: version() }, auth.session()?.csrfToken);
      await props.prepare(result.config, selectedName());
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return <form onSubmit={prepare} class="database-create-form">
    <div class="source-entry database-create">
    <Show when={error()}><ErrorNotice message={error()} /></Show>
    <fieldset disabled={busy()} class="database-engine-fieldset">
      <legend>Engine</legend>
      <div class="database-engine-grid">
        <For each={engines}>{entry =>
          <button type="button" class="database-engine-card" classList={{ selected: engine().id === entry.id }}
            aria-pressed={engine().id === entry.id} onClick={() => choose(entry)}>
            <span class="database-engine-title"><Icon name={entry.icon} /><span class="database-engine-name">{entry.name}</span></span>
            <Show when={engine().id === entry.id}><span class="database-engine-check"><Icon name="check" /></span></Show>
          </button>
        }</For>
      </div>
    </fieldset>
    <div class="database-create-fields">
      <div class="form-fields">
        <label for="database-version">Version</label>
        <select id="database-version" class="input" value={version()} onChange={e => setVersion(e.currentTarget.value)}>
          <For each={engine().versions}>{value => <option>{value}</option>}</For>
        </select>
      </div>
      <div class="form-fields">
        <label for="database-name">Name</label>
        <input id="database-name" class="input" maxlength="80" placeholder={engine().name} value={name()} onInput={e => setName(e.currentTarget.value)} />
      </div>
    </div>
    </div>
    <div class="form-actions creation-actions database-create-actions">
      <Button variant="ghost" disabled={busy()} onClick={props.close}>Cancel</Button>
      <Button type="submit" variant="primary" disabled={busy()}>{busy() ? "Preparing database" : "Review database"}</Button>
    </div>
  </form>;
}
