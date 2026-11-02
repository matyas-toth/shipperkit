ALTER TABLE deployments ADD COLUMN compose_plan TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN services_json TEXT NOT NULL DEFAULT '[]';
