CREATE TABLE operation_tokens (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, token_hash TEXT UNIQUE NOT NULL,
 user_id TEXT NOT NULL REFERENCES administrators(id), scopes TEXT NOT NULL,
 applications TEXT NOT NULL, expires_at INTEGER NOT NULL, created_at INTEGER NOT NULL
);
CREATE TABLE observations (
 id TEXT PRIMARY KEY, application_id TEXT NOT NULL, kind TEXT NOT NULL,
 state TEXT NOT NULL, summary TEXT NOT NULL, reference TEXT NOT NULL,
 observed_at INTEGER NOT NULL, details TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE operation_events (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT, application_id TEXT NOT NULL,
 kind TEXT NOT NULL, state TEXT NOT NULL, summary TEXT NOT NULL,
 reference TEXT NOT NULL, created_at INTEGER NOT NULL
);
CREATE INDEX operation_events_app ON operation_events(application_id, sequence);
CREATE TABLE notification_outbox (
 sequence INTEGER PRIMARY KEY REFERENCES operation_events(sequence) ON DELETE CASCADE,
 attempts INTEGER NOT NULL DEFAULT 0, next_at INTEGER NOT NULL,
 delivered_at INTEGER NOT NULL DEFAULT 0, error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE source_snapshots (
 id TEXT PRIMARY KEY, owner_id TEXT NOT NULL, application_id TEXT NOT NULL,
 digest TEXT NOT NULL, files INTEGER NOT NULL, bytes INTEGER NOT NULL,
 created_at INTEGER NOT NULL, provenance TEXT NOT NULL
);
CREATE TABLE maintenance_runs (
 id TEXT PRIMARY KEY, state TEXT NOT NULL, revision TEXT NOT NULL,
 created_at INTEGER NOT NULL, finished_at INTEGER NOT NULL DEFAULT 0,
 error TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX maintenance_one_pending ON maintenance_runs((1)) WHERE state IN ('queued','running');
ALTER TABLE maintenance_runs ADD COLUMN kind TEXT NOT NULL DEFAULT 'cleanup';
ALTER TABLE maintenance_runs ADD COLUMN actor TEXT NOT NULL DEFAULT '';
ALTER TABLE source_snapshots ADD COLUMN deleting INTEGER NOT NULL DEFAULT 0;
