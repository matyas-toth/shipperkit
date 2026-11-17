CREATE TABLE deployment_diagnoses (
  deployment_id TEXT PRIMARY KEY,
  application_id TEXT NOT NULL,
  captured_at INTEGER NOT NULL,
  snapshot_json TEXT NOT NULL
);
CREATE INDEX deployment_diagnoses_application ON deployment_diagnoses(application_id, captured_at DESC);
