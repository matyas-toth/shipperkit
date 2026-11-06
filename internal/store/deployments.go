package store

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrCancelled = errors.New("Deployment cancelled before updating the application")
var ErrBusy = errors.New("An operation is already running for this application")
var ErrDomainInUse = errors.New("This domain is already assigned to an application")
var ErrIdempotencyConflict = errors.New("This idempotency key was used for a different request")

func (s *Store) Setting(ctx context.Context, key, initial string) (string, error) {
	if _, err := s.db.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES (?,?) ON CONFLICT(key) DO NOTHING", key, initial); err != nil {
		return "", err
	}
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key=?", key).Scan(&value)
	return value, err
}

func (s *Store) SaveSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}

// Enqueue commits the desired configuration and operation together. A unique partial
// index prevents concurrent operations on the same application, including deletion.
func (s *Store) Enqueue(ctx context.Context, a Application, d Deployment, requestKey string, create bool) (Deployment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return d, err
	}
	defer tx.Rollback()
	if a.Config.LocalSource != "" {
		var n int
		if e := tx.QueryRowContext(ctx, "SELECT count(*) FROM source_snapshots WHERE id=? AND deleting=0", a.Config.LocalSource).Scan(&n); e != nil {
			return d, e
		}
		if n != 1 {
			return d, errors.New("Local source is unavailable; upload it again")
		}
	}
	fingerprint := s.requestFingerprint(a, d, create)
	if requestKey != "" {
		old, e := s.scanDeployment(tx.QueryRowContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE request_key=?", requestKey))
		if e == nil {
			var original string
			if err = tx.QueryRowContext(ctx, "SELECT request_fingerprint FROM deployments WHERE id=?", old.ID).Scan(&original); err != nil {
				return d, err
			}
			if original != fingerprint {
				return d, ErrIdempotencyConflict
			}
			return old, nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return d, e
		}
	}
	now := time.Now().Unix()
	if a.Config.LocalSource != "" {
		result, e := tx.ExecContext(ctx, "UPDATE source_snapshots SET application_id=? WHERE id=? AND deleting=0 AND (application_id='' OR application_id=?)", a.ID, a.Config.LocalSource, a.ID)
		if e != nil {
			return d, e
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return d, errors.New("Local source belongs to another application")
		}
	}
	cfg, err := s.marshalConfig(a.Config)
	if err != nil {
		return d, err
	}
	requested := a.Domains
	if create && len(requested) == 0 {
		requested = a.Config.Domains
		if len(requested) == 0 && a.Config.Domain != "" {
			requested = []Domain{{ID: "primary", Hostname: a.Config.Domain, HTTPS: a.Config.HTTPS, Primary: true}}
		}
	}
	if d.Operation == "domains" {
		requested = a.Config.Domains
	}
	if d.Operation != "delete" {
		for _, domain := range requested {
			var owner string
			e := tx.QueryRowContext(ctx, "SELECT application_id FROM domain_claims WHERE hostname=?", domain.Hostname).Scan(&owner)
			if e == nil && owner != a.ID {
				return d, ErrDomainInUse
			}
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return d, e
			}

		}
	}

	if create {
		domains, _ := json.Marshal(requested)
		method := a.Config.Method
		if method == "" {
			method = "static"
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO applications(id,name,source_kind,source_ref,method,state,created_at,updated_at,config_json,domains_json) VALUES (?,?,?,?,?,'missing',?,?,?,?)", a.ID, a.Name, func() string {
			return sourceKind(a.Config)
		}(), sourceRef(a.Config), method, now, now, cfg, string(domains))
	} else {
		var busy int
		err = tx.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM deployments WHERE application_id=? AND state NOT IN ('succeeded','failed','interrupted','cancelled'))+(SELECT count(*) FROM backup_runs WHERE application_id=? AND state IN ('queued','running'))", a.ID, a.ID).Scan(&busy)
		if err != nil {
			return d, err
		}
		if busy > 0 {
			return d, ErrBusy
		}
		if err = tx.QueryRowContext(ctx, "SELECT active_deployment_id FROM applications WHERE id=? AND deleted_at=0", a.ID).Scan(&a.ActiveDeploymentID); err != nil {
			return d, err
		}
		var result sql.Result
		if d.Operation == "task" {
			result, err = tx.ExecContext(ctx, "UPDATE applications SET updated_at=updated_at WHERE id=? AND deleted_at=0", a.ID)
		} else {
			result, err = tx.ExecContext(ctx, "UPDATE applications SET name=?,source_kind=?,source_ref=?,method=?,config_json=?,updated_at=? WHERE id=? AND deleted_at=0", a.Name, sourceKind(a.Config), sourceRef(a.Config), func() string {
				if a.Config.Method == "" {
					return "static"
				}
				return a.Config.Method
			}(), cfg, now, a.ID)
		}
		if err == nil {
			n, _ := result.RowsAffected()
			if n != 1 {
				return d, sql.ErrNoRows
			}
		}
	}
	if err != nil {
		return d, err
	}
	if d.Operation != "delete" {
		for _, domain := range requested {
			if _, err = tx.ExecContext(ctx, "INSERT INTO domain_claims(hostname,application_id) VALUES (?,?) ON CONFLICT(hostname) DO NOTHING", domain.Hostname, a.ID); err != nil {
				return d, err
			}
		}
	}
	var key any
	if requestKey != "" {
		key = requestKey
	}
	d.ApplicationID = a.ID
	d.ApplicationName = a.Name
	d.PreviousID = a.ActiveDeploymentID
	d.State = "queued"
	d.Config = a.Config
	d.CreatedAt = now
	d.PreviousDomains = append([]Domain(nil), a.Domains...)
	taskCommand, err := s.sealJSON(d.TaskCommand, "task-command")
	if err != nil {
		return d, err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO deployments(id,application_id,operation,config_json,previous_id,created_at,request_key,request_fingerprint,previous_domains_json,no_cache,application_name,task_id,task_command,actor,service,commit_sha,rollback_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", d.ID, a.ID, d.Operation, string(cfg), a.ActiveDeploymentID, now, key, fingerprint, func() string { b, _ := json.Marshal(a.Domains); return string(b) }(), d.NoCache, a.Name, d.TaskID, taskCommand, d.Actor, d.Service, d.Commit, d.RollbackID)
	if err != nil {
		return d, err
	}
	return d, tx.Commit()
}

const deploymentColumns = "id,application_id,operation,state,config_json,commit_sha,image_id,container_id,previous_id,error,created_at,started_at,finished_at,previous_domains_json,no_cache,resolved_method,runtime_port,application_name,compose_plan,services_json,task_id,task_command,actor,service,digest,cancel_requested,rollback_id"

type scanner interface{ Scan(...any) error }

func (s *Store) scanDeployment(row scanner) (Deployment, error) {
	var d Deployment
	var cfg, previous, services, taskCommand string
	err := row.Scan(&d.ID, &d.ApplicationID, &d.Operation, &d.State, &cfg, &d.Commit, &d.ImageID, &d.ContainerID, &d.PreviousID, &d.Error, &d.CreatedAt, &d.StartedAt, &d.FinishedAt, &previous, &d.NoCache, &d.ResolvedMethod, &d.RuntimePort, &d.ApplicationName, &d.ComposePlan, &services, &d.TaskID, &taskCommand, &d.Actor, &d.Service, &d.Digest, &d.CancelRequested, &d.RollbackID)
	if err == nil {
		if taskCommand != "" {
			if err = s.unsealJSON(taskCommand, "task-command", &d.TaskCommand); err != nil {
				return d, err
			}
		}
		d.ComposePlan, err = s.unseal(d.ComposePlan, "compose-plan")
		if err != nil {
			return d, err
		}
		err = json.Unmarshal([]byte(services), &d.Services)
		if err != nil {
			return d, err
		}
		err = s.unmarshalConfig(cfg, &d.Config)
		if err == nil {
			err = json.Unmarshal([]byte(previous), &d.PreviousDomains)
		}
	}
	return d, err
}
func (s *Store) Deployment(ctx context.Context, id string) (Deployment, error) {
	return s.scanDeployment(s.db.QueryRowContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE id=?", id))
}
func (s *Store) Deployments(ctx context.Context, appID string) ([]Deployment, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE application_id=? ORDER BY created_at DESC,rowid DESC LIMIT 100", appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Deployment{}
	for rows.Next() {
		d, e := s.scanDeployment(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) Pending(ctx context.Context) ([]Deployment, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE state NOT IN ('succeeded','failed','interrupted','cancelled') ORDER BY created_at,rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Deployment{}
	for rows.Next() {
		d, e := s.scanDeployment(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) SaveDeployment(ctx context.Context, d Deployment) error {
	plan, e := s.seal(d.ComposePlan, "compose-plan")
	if e != nil {
		return e
	}
	services, e := json.Marshal(d.Services)
	if e != nil {
		return e
	}
	result, err := s.db.ExecContext(ctx, "UPDATE deployments SET state=?,commit_sha=?,image_id=?,container_id=?,previous_id=?,error=?,started_at=?,finished_at=?,resolved_method=?,runtime_port=?,compose_plan=?,services_json=?,digest=? WHERE id=? AND (cancel_requested=0 OR ? IN ('succeeded','failed','cancelled'))", d.State, d.Commit, d.ImageID, d.ContainerID, d.PreviousID, d.Error, d.StartedAt, d.FinishedAt, d.ResolvedMethod, d.RuntimePort, plan, string(services), d.Digest, d.ID, d.State)
	if err == nil {
		n, _ := result.RowsAffected()
		if n == 0 {
			return ErrCancelled
		}
		state := d.State
		if state == "succeeded" {
			state = "healthy"
		}
		if state == "interrupted" {
			state = "failed"
		}
		err = s.Observe(ctx, Observation{ID: "operation/" + d.ApplicationID, ApplicationID: d.ApplicationID, Kind: func() string {
			if d.Operation == "task" {
				return "task"
			}
			return "deployment"
		}(), State: state, Summary: "Latest " + d.Operation + " is " + d.State, Reference: d.ID})
	}
	return err
}
func (s *Store) SetActive(ctx context.Context, appID, deploymentID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE applications SET active_deployment_id=?,updated_at=? WHERE id=? AND deleted_at=0", deploymentID, time.Now().Unix(), appID)
	return err
}
func (s *Store) MarkDeleted(ctx context.Context, appID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE applications SET deleted_at=?,active_deployment_id='' WHERE id=?", time.Now().Unix(), appID)
	return err
}
func (s *Store) AppendLog(ctx context.Context, id, step, line string) error {
	if len(line) > 2048 {
		line = line[:2048] + " [truncated]"
	}
	// Persist a bounded history even when a tool produces unbounded output.
	_, err := s.db.ExecContext(ctx, `INSERT INTO deployment_logs(deployment_id,step,line,created_at)
 SELECT ?,?,?,? WHERE (SELECT COUNT(*) FROM deployment_logs WHERE deployment_id=?)<10000`, id, step, line, time.Now().Unix(), id)
	return err
}
func (s *Store) Logs(ctx context.Context, id string, after int64) ([]LogLine, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT seq,step,line,created_at,number FROM (
 SELECT seq,step,line,created_at,ROW_NUMBER() OVER (ORDER BY seq) AS number
 FROM deployment_logs WHERE deployment_id=?) WHERE seq>? ORDER BY seq LIMIT 500`, id, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogLine{}
	for rows.Next() {
		var l LogLine
		if err = rows.Scan(&l.Sequence, &l.Step, &l.Line, &l.CreatedAt, &l.Number); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// A renewable lease fences two control-plane processes using the same database.
func (s *Store) Lease(ctx context.Context, owner string) (bool, error) {
	now := time.Now().Unix()
	r, err := s.db.ExecContext(ctx, `INSERT INTO worker_lease VALUES (1,?,?) ON CONFLICT(id) DO UPDATE SET owner=excluded.owner,expires_at=excluded.expires_at WHERE worker_lease.owner=? OR worker_lease.expires_at<?`, owner, now+30, owner, now)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}
func (s *Store) ReleaseLease(ctx context.Context, owner string) {
	s.db.ExecContext(ctx, "DELETE FROM worker_lease WHERE owner=?", owner)
}

func sourceKind(c Config) string {
	if c.LocalSource != "" {
		return "local"
	}
	if c.Method == "image" {
		return "image"
	}
	if c.Method == "inline" {
		return "inline"
	}
	if c.Method == "compose" && c.ComposeSource == "paste" {
		return "compose"
	}
	return "git"
}

func (s *Store) ConfigRevision(a Application) string {
	data, _ := json.Marshal(struct {
		Config Config
		Active string
	}{a.Config, a.ActiveDeploymentID})
	mac := hmac.New(sha256.New, s.signingKey)
	mac.Write(data)
	return fmt.Sprintf("%x", mac.Sum(nil))
}

// ChangeRevision binds the proposed inputs as well as the current serving state.
func (s *Store) ChangeRevision(current, next Application, commit ...string) string {
	data, _ := json.Marshal(struct {
		Current string
		Next    Application
	}{s.ConfigRevision(current), Application{ID: next.ID, Name: next.Name, Config: next.Config}})
	mac := hmac.New(sha256.New, s.signingKey)
	mac.Write(data)
	if len(commit) > 0 && commit[0] != "" {
		mac.Write([]byte("\x00" + commit[0]))
	}
	return fmt.Sprintf("%x", mac.Sum(nil))
}

func (s *Store) TailLogs(ctx context.Context, id string, limit int) ([]LogLine, error) {
	limit = min(max(limit, 1), 100)
	rows, e := s.db.QueryContext(ctx, `SELECT seq,step,line,created_at,number FROM (SELECT seq,step,line,created_at,ROW_NUMBER() OVER (ORDER BY seq) AS number FROM deployment_logs WHERE deployment_id=?) ORDER BY seq DESC LIMIT ?`, id, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []LogLine{}
	for rows.Next() {
		var l LogLine
		if e = rows.Scan(&l.Sequence, &l.Step, &l.Line, &l.CreatedAt, &l.Number); e != nil {
			return nil, e
		}
		out = append(out, l)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func sourceRef(c Config) string {
	if c.LocalSource != "" {
		return c.LocalSource
	}
	if c.Method == "image" {
		return c.Image
	}
	return c.Repository
}

func (s *Store) requestFingerprint(a Application, d Deployment, create bool) string {
	canonical := a.Config
	scope := a.ID
	if create {
		scope = ""
		if canonical.Domain == a.ID+".localhost" {
			canonical.Domain = ""
		}
	}
	payload, _ := json.Marshal(struct {
		Name, ApplicationID, Operation, Service, Revision, Rollback string
		Create, NoCache                                             bool
		TaskID                                                      string
		TaskCommand                                                 *Command
		Config                                                      StaticConfig
	}{a.Name, scope, d.Operation, d.Service, d.Commit, d.RollbackID, create, d.NoCache, d.TaskID, d.TaskCommand, canonical})
	mac := hmac.New(sha256.New, s.signingKey)
	mac.Write(payload)
	return fmt.Sprintf("%x", mac.Sum(nil))
}

func (s *Store) ReplayRequest(ctx context.Context, key string, a Application, d Deployment, create ...bool) (Deployment, bool, error) {
	if key == "" {
		return Deployment{}, false, nil
	}
	old, e := s.scanDeployment(s.db.QueryRowContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE request_key=?", key))
	if errors.Is(e, sql.ErrNoRows) {
		return Deployment{}, false, nil
	}
	if e != nil {
		return Deployment{}, false, e
	}
	var original string
	e = s.db.QueryRowContext(ctx, "SELECT request_fingerprint FROM deployments WHERE id=?", old.ID).Scan(&original)
	if e != nil {
		return Deployment{}, false, e
	}
	d.Operation = old.Operation
	if original != s.requestFingerprint(a, d, len(create) > 0 && create[0]) {
		return Deployment{}, false, ErrIdempotencyConflict
	}
	return old, true, nil
}
