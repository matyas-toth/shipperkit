ALTER TABLE applications ADD COLUMN config_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE applications ADD COLUMN active_deployment_id TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN deleted_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE containers ADD COLUMN deployment_id TEXT NOT NULL DEFAULT '';
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE deployments (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id),
    operation TEXT NOT NULL DEFAULT 'deploy',
    state TEXT NOT NULL DEFAULT 'queued',
    config_json TEXT NOT NULL,
    commit_sha TEXT NOT NULL DEFAULT '',
    image_id TEXT NOT NULL DEFAULT '',
    container_id TEXT NOT NULL DEFAULT '',
    previous_id TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    started_at INTEGER NOT NULL DEFAULT 0,
    finished_at INTEGER NOT NULL DEFAULT 0,
    request_key TEXT UNIQUE
);
CREATE UNIQUE INDEX deployments_active_job ON deployments(application_id)
    WHERE state NOT IN ('succeeded','failed','interrupted');
CREATE INDEX deployments_history ON deployments(application_id,created_at DESC);
CREATE TABLE deployment_logs (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id TEXT NOT NULL REFERENCES deployments(id),
    step TEXT NOT NULL,
    line TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX deployment_logs_cursor ON deployment_logs(deployment_id,seq);
CREATE TABLE worker_lease (id INTEGER PRIMARY KEY CHECK(id=1), owner TEXT NOT NULL, expires_at INTEGER NOT NULL);
