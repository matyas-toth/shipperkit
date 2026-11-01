package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

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

// Enqueue commits the desired configuration and operation together. A unique partial
// index prevents concurrent operations on the same application, including deletion.
func (s *Store) Enqueue(ctx context.Context, a Application, d Deployment, requestKey string, create bool) (Deployment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return d, err
	}
	defer tx.Rollback()
	canonical := a.Config
	scope := a.ID
	if create {
		scope = ""
		if canonical.Domain == a.ID+".localhost" {
			canonical.Domain = ""
		}
	}
	payload, _ := json.Marshal(struct {
		Name, ApplicationID, Operation string
		Create                         bool
		Config                         StaticConfig
	}{a.Name, scope, d.Operation, create, canonical})
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(payload))
	if requestKey != "" {
		old, e := scanDeployment(tx.QueryRowContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE request_key=?", requestKey))
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
	cfg, _ := json.Marshal(a.Config)
	if a.Config.Domain != "" && d.Operation != "delete" {
		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM applications a LEFT JOIN deployments d ON d.id=a.active_deployment_id WHERE a.deleted_at=0 AND a.id<>? AND (json_extract(a.config_json,'$.domain')=? OR json_extract(d.config_json,'$.domain')=?)", a.ID, a.Config.Domain, a.Config.Domain).Scan(&count)
		if err != nil {
			return d, err
		}
		if count > 0 {
			return d, ErrDomainInUse
		}
	}
	if create {
		_, err = tx.ExecContext(ctx, "INSERT INTO applications(id,name,source_kind,source_ref,method,state,created_at,updated_at,config_json) VALUES (?,?, 'git',?,'static','missing',?,?,?)", a.ID, a.Name, a.Config.Repository, now, now, string(cfg))
	} else {
		var busy int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM deployments WHERE application_id=? AND state NOT IN ('succeeded','failed','interrupted')", a.ID).Scan(&busy)
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
		result, err = tx.ExecContext(ctx, "UPDATE applications SET name=?,source_ref=?,config_json=?,updated_at=? WHERE id=? AND deleted_at=0", a.Name, a.Config.Repository, string(cfg), now, a.ID)
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
	var key any
	if requestKey != "" {
		key = requestKey
	}
	d.ApplicationID = a.ID
	d.PreviousID = a.ActiveDeploymentID
	d.State = "queued"
	d.Config = a.Config
	d.CreatedAt = now
	_, err = tx.ExecContext(ctx, "INSERT INTO deployments(id,application_id,operation,config_json,previous_id,created_at,request_key,request_fingerprint) VALUES (?,?,?,?,?,?,?,?)", d.ID, a.ID, d.Operation, string(cfg), a.ActiveDeploymentID, now, key, fingerprint)
	if err != nil {
		return d, err
	}
	return d, tx.Commit()
}

const deploymentColumns = "id,application_id,operation,state,config_json,commit_sha,image_id,container_id,previous_id,error,created_at,started_at,finished_at"

type scanner interface{ Scan(...any) error }

func scanDeployment(row scanner) (Deployment, error) {
	var d Deployment
	var cfg string
	err := row.Scan(&d.ID, &d.ApplicationID, &d.Operation, &d.State, &cfg, &d.Commit, &d.ImageID, &d.ContainerID, &d.PreviousID, &d.Error, &d.CreatedAt, &d.StartedAt, &d.FinishedAt)
	if err == nil {
		err = json.Unmarshal([]byte(cfg), &d.Config)
	}
	return d, err
}
func (s *Store) Deployment(ctx context.Context, id string) (Deployment, error) {
	return scanDeployment(s.db.QueryRowContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE id=?", id))
}
func (s *Store) Deployments(ctx context.Context, appID string) ([]Deployment, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE application_id=? ORDER BY created_at DESC,rowid DESC LIMIT 100", appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Deployment{}
	for rows.Next() {
		d, e := scanDeployment(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) Pending(ctx context.Context) ([]Deployment, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE state NOT IN ('succeeded','failed','interrupted') ORDER BY created_at,rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Deployment{}
	for rows.Next() {
		d, e := scanDeployment(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) SaveDeployment(ctx context.Context, d Deployment) error {
	_, err := s.db.ExecContext(ctx, "UPDATE deployments SET state=?,commit_sha=?,image_id=?,container_id=?,previous_id=?,error=?,started_at=?,finished_at=? WHERE id=?", d.State, d.Commit, d.ImageID, d.ContainerID, d.PreviousID, d.Error, d.StartedAt, d.FinishedAt, d.ID)
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
	rows, err := s.db.QueryContext(ctx, "SELECT seq,step,line,created_at FROM deployment_logs WHERE deployment_id=? AND seq>? ORDER BY seq LIMIT 500", id, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogLine{}
	for rows.Next() {
		var l LogLine
		if err = rows.Scan(&l.Sequence, &l.Step, &l.Line, &l.CreatedAt); err != nil {
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
