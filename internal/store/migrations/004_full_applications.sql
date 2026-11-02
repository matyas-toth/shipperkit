ALTER TABLE applications ADD COLUMN domains_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE applications ADD COLUMN desired_state TEXT NOT NULL DEFAULT 'running';
UPDATE applications SET domains_json=json_array(json_object('id','primary','hostname',
 COALESCE(NULLIF((SELECT json_extract(d.config_json,'$.domain') FROM deployments d WHERE d.id=applications.active_deployment_id),''),json_extract(config_json,'$.domain')),
 'https',json(CASE WHEN COALESCE((SELECT json_extract(d.config_json,'$.https') FROM deployments d WHERE d.id=applications.active_deployment_id),json_extract(config_json,'$.https'),0)=1 THEN 'true' ELSE 'false' END),'primary',json('true'),'redirect',json('false')))
 WHERE COALESCE(json_extract(config_json,'$.domain'),'')<>'';
ALTER TABLE deployments ADD COLUMN previous_domains_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE deployments ADD COLUMN no_cache INTEGER NOT NULL DEFAULT 0;
ALTER TABLE deployments ADD COLUMN resolved_method TEXT NOT NULL DEFAULT '';
CREATE TABLE domain_claims(hostname TEXT PRIMARY KEY,application_id TEXT NOT NULL REFERENCES applications(id));
INSERT OR IGNORE INTO domain_claims SELECT json_extract(value,'$.hostname'),a.id FROM applications a,json_each(a.domains_json) WHERE a.deleted_at=0 AND json_extract(value,'$.hostname') IS NOT NULL;
CREATE INDEX deployments_queue ON deployments(created_at) WHERE state NOT IN ('succeeded','failed','interrupted');

ALTER TABLE deployments ADD COLUMN runtime_port INTEGER NOT NULL DEFAULT 0;
