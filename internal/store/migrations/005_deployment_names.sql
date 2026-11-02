ALTER TABLE deployments ADD COLUMN application_name TEXT NOT NULL DEFAULT '';
UPDATE deployments SET application_name=COALESCE((SELECT name FROM applications WHERE applications.id=deployments.application_id),'');
