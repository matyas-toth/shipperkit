ALTER TABLE deployments ADD COLUMN digest TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN cancel_requested INTEGER NOT NULL DEFAULT 0;
DROP INDEX deployments_active_job;
CREATE UNIQUE INDEX deployments_active_job ON deployments(application_id) WHERE state NOT IN ('succeeded','failed','interrupted','cancelled');
CREATE TABLE connections(id TEXT PRIMARY KEY,name TEXT NOT NULL,kind TEXT NOT NULL,host TEXT NOT NULL,secret TEXT NOT NULL,updated_at INTEGER NOT NULL);
CREATE TABLE provider_flows(hash TEXT PRIMARY KEY,stage TEXT NOT NULL,connection_id TEXT NOT NULL,origin TEXT NOT NULL,expires_at INTEGER NOT NULL);
CREATE TABLE deploy_tokens(hash TEXT PRIMARY KEY,application_id TEXT NOT NULL REFERENCES applications(id),created_at INTEGER NOT NULL);
CREATE TABLE deploy_intents(application_id TEXT PRIMARY KEY REFERENCES applications(id),revision TEXT NOT NULL,actor TEXT NOT NULL,received_at INTEGER NOT NULL);
CREATE TABLE webhook_deliveries(connection_id TEXT NOT NULL,delivery TEXT NOT NULL,created_at INTEGER NOT NULL,PRIMARY KEY(connection_id,delivery));
