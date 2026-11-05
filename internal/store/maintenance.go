package store

import (
	"context"
	"time"
)

const eligibleLogs = `deployment_id IN (SELECT id FROM deployments WHERE finished_at>0 AND finished_at<? AND state IN ('succeeded','failed','cancelled','interrupted') AND id NOT IN (SELECT active_deployment_id FROM applications) AND id NOT IN (SELECT deployment_id FROM task_runs WHERE state IN ('uncertain','interrupted')) AND id NOT IN (SELECT id FROM (SELECT id,ROW_NUMBER() OVER(PARTITION BY application_id ORDER BY created_at DESC,rowid DESC) AS rank FROM deployments) WHERE rank<=10))`

func (s *Store) LogRetention(ctx context.Context, cutoff int64) (count, maxSequence int64, e error) {
	e = s.db.QueryRowContext(ctx, "SELECT count(*),COALESCE(max(seq),0) FROM deployment_logs WHERE "+eligibleLogs, cutoff).Scan(&count, &maxSequence)
	return
}
func (s *Store) RetainLogs(ctx context.Context, cutoff int64) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM deployment_logs WHERE "+eligibleLogs, cutoff)
	return e
}

type MaintenanceRun struct {
	Actor      string `json:"actor"`
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	State      string `json:"state"`
	Revision   string `json:"revision"`
	CreatedAt  int64  `json:"createdAt"`
	FinishedAt int64  `json:"finishedAt"`
	Error      string `json:"error"`
}

func (s *Store) EnqueueMaintenance(ctx context.Context, v MaintenanceRun) error {
	if v.Kind == "" {
		v.Kind = "cleanup"
	}
	_, e := s.db.ExecContext(ctx, "INSERT INTO maintenance_runs(id,state,revision,created_at,kind,actor) VALUES (?,'queued',?,?,?,?)", v.ID, v.Revision, time.Now().Unix(), v.Kind, v.Actor)
	return e
}
func (s *Store) MaintenanceRuns(ctx context.Context) ([]MaintenanceRun, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,state,revision,created_at,finished_at,error,kind,actor FROM maintenance_runs ORDER BY created_at DESC LIMIT 20")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []MaintenanceRun{}
	for rows.Next() {
		var v MaintenanceRun
		if e = rows.Scan(&v.ID, &v.State, &v.Revision, &v.CreatedAt, &v.FinishedAt, &v.Error, &v.Kind, &v.Actor); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) SaveMaintenance(ctx context.Context, v MaintenanceRun) error {
	_, e := s.db.ExecContext(ctx, "UPDATE maintenance_runs SET state=?,finished_at=?,error=? WHERE id=?", v.State, v.FinishedAt, v.Error, v.ID)
	return e
}
func (s *Store) ImageReferences(ctx context.Context) (map[string]bool, bool, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT DISTINCT image_id FROM deployments WHERE image_id!='' UNION SELECT json_extract(config_json,'$.image') FROM applications WHERE COALESCE(json_extract(config_json,'$.image'),'')!='' UNION SELECT json_extract(config_json,'$.image') FROM deployments WHERE COALESCE(json_extract(config_json,'$.image'),'')!=''")
	if e != nil {
		return nil, false, e
	}
	defer rows.Close()
	out := map[string]bool{}
	compose := false
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, false, e
		}
		if id == "compose" {
			compose = true
		}
		out[id] = true
	}
	return out, compose, rows.Err()
}
func (s *Store) RemoveAbandonedSource(ctx context.Context, id string) (bool, error) {
	result, e := s.db.ExecContext(ctx, `UPDATE source_snapshots SET deleting=1 WHERE id=? AND deleting=0 AND NOT EXISTS(SELECT 1 FROM applications WHERE json_extract(config_json,'$.localSource')=?) AND NOT EXISTS(SELECT 1 FROM deployments WHERE json_extract(config_json,'$.localSource')=?)`, id, id, id)
	if e != nil {
		return false, e
	}
	n, _ := result.RowsAffected()
	return n == 1, nil
}
func (s *Store) ResetSourceClaims(ctx context.Context) error {
	_, e := s.db.ExecContext(ctx, "UPDATE source_snapshots SET deleting=0 WHERE deleting=1")
	return e
}
func (s *Store) ReleaseSourceClaim(ctx context.Context, id string) error {
	_, e := s.db.ExecContext(ctx, "UPDATE source_snapshots SET deleting=0 WHERE id=?", id)
	return e
}
