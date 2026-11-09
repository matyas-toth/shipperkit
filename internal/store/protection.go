package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shipper/internal/auth"
)

type BackupSpec struct {
	NativeImport bool               `json:"nativeImport,omitempty"`
	Remote       *BackupDestination `json:"remote,omitempty"`
	Kind         string             `json:"kind"`
	Service      string             `json:"service"`
	Volume       string             `json:"volume"`
	Engine       string             `json:"engine"`
	Database     string             `json:"database"`
	User         string             `json:"user"`
	Password     string             `json:"password,omitempty"`
	Host         string             `json:"host"`
	Port         int                `json:"port"`
	TLS          string             `json:"tls"`
	Destination  string             `json:"destination"`
	Keep         int                `json:"keep"`
	Target       string             `json:"target,omitempty"`
	Identity     string             `json:"identity,omitempty"`
}
type BackupPlan struct {
	ID            string     `json:"id"`
	ApplicationID string     `json:"applicationId"`
	Name          string     `json:"name"`
	Config        BackupSpec `json:"config"`
	Enabled       bool       `json:"enabled"`
	Schedule      string     `json:"schedule"`
	Timezone      string     `json:"timezone"`
	NextAt        int64      `json:"nextAt"`
}
type BackupRun struct {
	Service       string     `json:"service"`
	Kind          string     `json:"kind"`
	ID            string     `json:"id"`
	PlanID        string     `json:"planId"`
	ApplicationID string     `json:"applicationId"`
	Operation     string     `json:"operation"`
	State         string     `json:"state"`
	Config        BackupSpec `json:"-"`
	CreatedAt     int64      `json:"createdAt"`
	StartedAt     int64      `json:"startedAt"`
	FinishedAt    int64      `json:"finishedAt"`
	Error         string     `json:"error"`
	Bytes         int64      `json:"bytes"`
	Checksum      string     `json:"checksum"`
	ObjectKey     string     `json:"objectKey"`
	Result        string     `json:"result"`
	SourceID      string     `json:"sourceId"`
	VerifiedAt    int64      `json:"verifiedAt"`
}
type BackupDestination struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	Private   bool   `json:"private"`
}

func (s *Store) PrivateSetting(ctx context.Context, key string, initial string) (string, error) {
	raw, e := s.seal(initial, "setting:"+key)
	if e != nil {
		return "", e
	}
	v, e := s.Setting(ctx, key, raw)
	if e != nil {
		return "", e
	}
	return s.unseal(v, "setting:"+key)
}
func (s *Store) Snapshot(ctx context.Context, path string) error {
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		return errors.New("Snapshot destination must not exist")
	}
	// A dedicated WAL reader keeps the main connection available for sessions,
	// inventory and the worker lease while a large snapshot is being written.
	reader, e := sql.Open("sqlite", s.file)
	if e != nil {
		return e
	}
	defer reader.Close()
	reader.SetMaxOpenConns(1)
	if _, e = reader.ExecContext(ctx, "PRAGMA busy_timeout=5000"); e != nil {
		return e
	}
	_, e = reader.ExecContext(ctx, "VACUUM INTO ?", path)
	if e == nil {
		e = os.Chmod(path, 0600)
	}
	return e
}
func (s *Store) BackupPlans(ctx context.Context, app string) ([]BackupPlan, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,application_id,name,config,enabled,schedule,timezone,next_at FROM backup_plans WHERE application_id=? ORDER BY rowid", app)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []BackupPlan{}
	for rows.Next() {
		var p BackupPlan
		var raw string
		if e = rows.Scan(&p.ID, &p.ApplicationID, &p.Name, &raw, &p.Enabled, &p.Schedule, &p.Timezone, &p.NextAt); e != nil {
			return nil, e
		}
		if e = s.unsealJSON(raw, "backup-plan", &p.Config); e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) DueBackupPlans(ctx context.Context, now int64) ([]BackupPlan, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,application_id,name,config,enabled,schedule,timezone,next_at FROM backup_plans WHERE enabled=1 AND schedule<>'' AND next_at>0 AND next_at<=? AND (application_id='' OR application_id IN (SELECT id FROM applications WHERE deleted_at=0)) ORDER BY next_at LIMIT 32", now)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []BackupPlan{}
	for rows.Next() {
		var p BackupPlan
		var raw string
		if e = rows.Scan(&p.ID, &p.ApplicationID, &p.Name, &raw, &p.Enabled, &p.Schedule, &p.Timezone, &p.NextAt); e != nil {
			return nil, e
		}
		if e = s.unsealJSON(raw, "backup-plan", &p.Config); e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) SaveBackupPlan(ctx context.Context, p BackupPlan) error {
	var count int
	if e := s.db.QueryRowContext(ctx, "SELECT count(*) FROM backup_plans WHERE application_id=? AND id<>?", p.ApplicationID, p.ID).Scan(&count); e != nil {
		return e
	}
	if count >= 32 {
		return errors.New("Use at most 32 backup plans per application")
	}
	raw, e := s.sealJSON(p.Config, "backup-plan")
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, `INSERT INTO backup_plans VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,config=excluded.config,enabled=excluded.enabled,schedule=excluded.schedule,timezone=excluded.timezone,next_at=excluded.next_at WHERE backup_plans.application_id=excluded.application_id`, p.ID, p.ApplicationID, p.Name, raw, p.Enabled, p.Schedule, p.Timezone, p.NextAt)
	return e
}
func (s *Store) DeleteBackupPlan(ctx context.Context, app, id string) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM backup_plans WHERE id=? AND application_id=?", id, app)
	return e
}

const backupColumns = "id,plan_id,application_id,operation,state,config,created_at,started_at,finished_at,error,bytes,checksum,object_key,result,source_id,verified_at"

func (s *Store) scanBackup(row scanner) (BackupRun, error) {
	var v BackupRun
	var raw string
	e := row.Scan(&v.ID, &v.PlanID, &v.ApplicationID, &v.Operation, &v.State, &raw, &v.CreatedAt, &v.StartedAt, &v.FinishedAt, &v.Error, &v.Bytes, &v.Checksum, &v.ObjectKey, &v.Result, &v.SourceID, &v.VerifiedAt)
	if e == nil {
		e = s.unsealJSON(raw, "backup-run", &v.Config)
	}
	v.Service = v.Config.Service
	v.Kind = v.Config.Kind
	return v, e
}
func (s *Store) BackupRun(ctx context.Context, id string) (BackupRun, error) {
	return s.scanBackup(s.db.QueryRowContext(ctx, "SELECT "+backupColumns+" FROM backup_runs WHERE id=?", id))
}
func (s *Store) BackupRuns(ctx context.Context, app string) ([]BackupRun, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT "+backupColumns+" FROM backup_runs WHERE application_id=? OR (?='' AND operation='delete-volume') ORDER BY created_at DESC,rowid DESC LIMIT 100", app, app)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []BackupRun{}
	for rows.Next() {
		v, e := s.scanBackup(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) QueueBackup(ctx context.Context, v BackupRun) (BackupRun, error) {
	v.Service = v.Config.Service
	v.Kind = v.Config.Kind
	if v.Operation == "backup" && v.Config.Destination != "" {
		items, e := s.BackupDestinations(ctx)
		if e != nil {
			return v, e
		}
		found := false
		for _, d := range items {
			if d.ID == v.Config.Destination {
				v.Config.Remote = &d
				found = true
				break
			}
		}
		if !found {
			return v, errors.New("Backup destination is no longer available")
		}
	}
	v.ID = auth.Token()[:24]
	v.State = "queued"
	if v.Operation == "import" {
		v.State = "running"
		v.StartedAt = time.Now().Unix()
	}
	v.CreatedAt = time.Now().Unix()
	raw, e := s.sealJSON(v.Config, "backup-run")
	if e != nil {
		return v, e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return v, e
	}
	defer tx.Rollback()
	var busy int
	if e = tx.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM backup_runs WHERE application_id=? AND state IN ('queued','running'))+(SELECT count(*) FROM deployments WHERE application_id=? AND state NOT IN ('succeeded','failed','interrupted','cancelled'))", v.ApplicationID, v.ApplicationID).Scan(&busy); e != nil {
		return v, e
	}
	if busy > 0 {
		return v, ErrBusy
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO backup_runs(id,plan_id,application_id,operation,state,config,created_at,source_id) VALUES(?,?,?,?,?,?,?,?)", v.ID, v.PlanID, v.ApplicationID, v.Operation, v.State, raw, v.CreatedAt, v.SourceID)
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func (s *Store) PendingBackup(ctx context.Context) (BackupRun, error) {
	return s.scanBackup(s.db.QueryRowContext(ctx, "SELECT "+backupColumns+" FROM backup_runs WHERE state='queued' ORDER BY created_at,rowid LIMIT 1"))
}
func (s *Store) SaveBackupRun(ctx context.Context, v BackupRun) error {
	_, e := s.db.ExecContext(ctx, "UPDATE backup_runs SET state=?,started_at=?,finished_at=?,error=?,bytes=?,checksum=?,object_key=?,result=?,verified_at=? WHERE id=?", v.State, v.StartedAt, v.FinishedAt, v.Error, v.Bytes, v.Checksum, v.ObjectKey, v.Result, v.VerifiedAt, v.ID)
	return e
}
func (s *Store) FinishBackupImport(ctx context.Context, v BackupRun) error {
	_, e := s.db.ExecContext(ctx, "UPDATE backup_runs SET operation='backup',state=?,finished_at=?,error=?,bytes=?,checksum=?,result=? WHERE id=? AND operation='import'", v.State, v.FinishedAt, v.Error, v.Bytes, v.Checksum, v.Result, v.ID)
	return e
}
func (s *Store) CheckVolumeUnreferenced(ctx context.Context, app, name string) error {
	var deleted int64
	if e := s.db.QueryRowContext(ctx, "SELECT deleted_at FROM applications WHERE id=?", app).Scan(&deleted); e != nil {
		return e
	}
	if deleted > 0 {
		return nil
	}
	rows, e := s.db.QueryContext(ctx, "SELECT config_json,compose_plan FROM deployments WHERE application_id=?", app)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var raw, plan string
		if e = rows.Scan(&raw, &plan); e != nil {
			return e
		}
		plan, e = s.unseal(plan, "compose-plan")
		if e != nil {
			return e
		}
		if strings.Contains(plan, `"name":"`+name+`"`) {
			return errors.New("This volume is referenced by a recorded Compose release")
		}
		var c Config
		e = s.unmarshalConfig(raw, &c)
		if e != nil {
			return e
		}
		for _, v := range c.Volumes {
			if strings.HasSuffix(name, "-"+app+"-"+v.Name) {
				return errors.New("This volume is referenced by a recorded application release")
			}
		}
		for _, v := range c.VolumeSources {
			if v == name {
				return errors.New("This volume is referenced by a recorded release; retain it for recovery")
			}
		}
	}
	return rows.Err()
}
func (s *Store) ProtectionApplicationName(ctx context.Context, app string) string {
	var name string
	_ = s.db.QueryRowContext(ctx, "SELECT name FROM applications WHERE id=?", app).Scan(&name)
	return name
}
func (s *Store) BackupRetention(ctx context.Context, plan string, keep int) ([]BackupRun, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT "+backupColumns+" FROM backup_runs WHERE plan_id=? AND operation='backup' AND state='succeeded' ORDER BY created_at DESC,rowid DESC LIMIT 100 OFFSET ?", plan, keep)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []BackupRun{}
	for rows.Next() {
		v, e := s.scanBackup(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) InterruptBackups(ctx context.Context) error {
	_, e := s.db.ExecContext(ctx, "UPDATE backup_runs SET state='interrupted',finished_at=?,error='The worker stopped; inspect restore effects before retrying' WHERE state='running'", time.Now().Unix())
	return e
}
func (s *Store) BackupDestinations(ctx context.Context) ([]BackupDestination, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT config FROM backup_destinations ORDER BY name")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []BackupDestination{}
	for rows.Next() {
		var raw string
		var d BackupDestination
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = s.unsealJSON(raw, "backup-destination", &d); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) SaveBackupDestination(ctx context.Context, d BackupDestination) error {
	var count int
	if e := s.db.QueryRowContext(ctx, "SELECT count(*) FROM backup_destinations WHERE id<>?", d.ID).Scan(&count); e != nil {
		return e
	}
	if count >= 16 {
		return errors.New("Use at most 16 backup destinations")
	}
	raw, e := s.sealJSON(d, "backup-destination")
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, "INSERT INTO backup_destinations VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,config=excluded.config", d.ID, d.Name, raw)
	return e
}

// PrepareRecovery operates on an extracted database, before the server starts.
// Captured work and login sessions cannot execute on a new host by accident.
func PrepareRecovery(dir string) error {
	db, e := sql.Open("sqlite", filepath.Join(dir, "shipper.db"))
	if e != nil {
		return e
	}
	defer db.Close()
	var result string
	if e = db.QueryRow("PRAGMA quick_check").Scan(&result); e != nil || result != "ok" {
		return errors.New("Recovered database failed its integrity check")
	}
	var newest string
	available, e := migrations.ReadDir("migrations")
	if e != nil {
		return e
	}
	if e = db.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&newest); e != nil || len(available) == 0 || newest != available[len(available)-1].Name() {
		return errors.New("Recovery schema does not match this Shipper version")
	}
	tx, e := db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, q := range []string{"UPDATE source_snapshots SET deleting=0", "DELETE FROM operation_tokens", "DELETE FROM notification_outbox", "UPDATE maintenance_runs SET state='interrupted',finished_at=unixepoch(),error='Interrupted by instance recovery' WHERE state IN ('queued','running')", "DELETE FROM sessions", "DELETE FROM provider_flows", "DELETE FROM deploy_tokens", "DELETE FROM deploy_intents", "UPDATE applications SET desired_state='stopped',config_json=json_set(config_json,'$.autoDeploy.enabled',json('false'))", "UPDATE tasks SET enabled=0", "UPDATE backup_plans SET enabled=0", "UPDATE deployments SET state='interrupted',finished_at=unixepoch(),error='Interrupted by instance recovery' WHERE state NOT IN ('succeeded','failed','interrupted','cancelled')", "UPDATE backup_runs SET state='interrupted',finished_at=unixepoch(),error='Interrupted by instance recovery' WHERE state IN ('queued','running')", "UPDATE task_runs SET state='interrupted',finished_at=unixepoch(),error='Interrupted by instance recovery' WHERE state NOT IN ('succeeded','failed','interrupted','uncertain','cancelled','skipped')", "DELETE FROM worker_lease"} {
		if _, e = tx.Exec(q); e != nil {
			return e
		}
	}
	return tx.Commit()
}
