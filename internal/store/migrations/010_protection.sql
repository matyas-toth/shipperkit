CREATE TABLE backup_plans (
 id TEXT PRIMARY KEY, application_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL,
 config TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1,
 schedule TEXT NOT NULL DEFAULT '', timezone TEXT NOT NULL DEFAULT 'UTC', next_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE backup_runs (
 id TEXT PRIMARY KEY, plan_id TEXT NOT NULL DEFAULT '', application_id TEXT NOT NULL DEFAULT '',
 operation TEXT NOT NULL, state TEXT NOT NULL, config TEXT NOT NULL,
 created_at INTEGER NOT NULL, started_at INTEGER NOT NULL DEFAULT 0, finished_at INTEGER NOT NULL DEFAULT 0,
 error TEXT NOT NULL DEFAULT '', bytes INTEGER NOT NULL DEFAULT 0, checksum TEXT NOT NULL DEFAULT '',
 object_key TEXT NOT NULL DEFAULT '', result TEXT NOT NULL DEFAULT '', source_id TEXT NOT NULL DEFAULT '',
 verified_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX backup_runs_queue ON backup_runs(state,created_at);
CREATE TABLE backup_destinations(id TEXT PRIMARY KEY,name TEXT NOT NULL,config TEXT NOT NULL);
