ALTER TABLE deployments ADD COLUMN service TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN actor TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN task_id TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN task_command TEXT NOT NULL DEFAULT '';
CREATE TABLE tasks (
 id TEXT PRIMARY KEY, application_id TEXT NOT NULL REFERENCES applications(id), name TEXT NOT NULL,
 command TEXT NOT NULL, enabled INTEGER NOT NULL, schedule TEXT NOT NULL, timezone TEXT NOT NULL,
 overlap TEXT NOT NULL, missed TEXT NOT NULL, next_at INTEGER NOT NULL, waiting INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL
);
CREATE INDEX tasks_due ON tasks(next_at) WHERE enabled=1 AND schedule<>'';
CREATE TABLE task_runs (
 id TEXT PRIMARY KEY, deployment_id TEXT NOT NULL REFERENCES deployments(id), stage TEXT NOT NULL,
 state TEXT NOT NULL, command TEXT NOT NULL, container_id TEXT NOT NULL DEFAULT '', exec_id TEXT NOT NULL DEFAULT '',
 exit_code INTEGER NOT NULL DEFAULT -1, error TEXT NOT NULL DEFAULT '', started_at INTEGER NOT NULL,
 finished_at INTEGER NOT NULL DEFAULT 0, UNIQUE(deployment_id,stage)
);
