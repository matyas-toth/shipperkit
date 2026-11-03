package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Task struct {
	ID            string  `json:"id"`
	ApplicationID string  `json:"applicationId"`
	Name          string  `json:"name"`
	Command       Command `json:"command"`
	Enabled       bool    `json:"enabled"`
	Schedule      string  `json:"schedule"`
	Timezone      string  `json:"timezone"`
	Overlap       string  `json:"overlap"`
	Missed        string  `json:"missed"`
	Waiting       bool    `json:"-"`
	NextAt        int64   `json:"nextAt"`
}
type TaskRun struct {
	ID           string  `json:"id"`
	DeploymentID string  `json:"deploymentId"`
	Stage        string  `json:"stage"`
	State        string  `json:"state"`
	Command      Command `json:"command"`
	ContainerID  string  `json:"containerId"`
	ExecID       string  `json:"execId"`
	ExitCode     int     `json:"exitCode"`
	Error        string  `json:"error"`
	StartedAt    int64   `json:"startedAt"`
	FinishedAt   int64   `json:"finishedAt"`
}

func (s *Store) sealJSON(v any, purpose string) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	return s.seal(string(b), purpose)
}
func (s *Store) unsealJSON(raw, purpose string, v any) error {
	b, e := s.unseal(raw, purpose)
	if e != nil {
		return e
	}
	return json.Unmarshal([]byte(b), v)
}

const taskColumns = "id,application_id,name,command,enabled,schedule,timezone,overlap,missed,next_at,waiting"

func (s *Store) scanTask(row scanner) (Task, error) {
	var t Task
	var raw string
	e := row.Scan(&t.ID, &t.ApplicationID, &t.Name, &raw, &t.Enabled, &t.Schedule, &t.Timezone, &t.Overlap, &t.Missed, &t.NextAt, &t.Waiting)
	if e == nil {
		e = s.unsealJSON(raw, "task", &t.Command)
	}
	return t, e
}
func (s *Store) Task(ctx context.Context, app, id string) (Task, error) {
	return s.scanTask(s.db.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE application_id=? AND id=?", app, id))
}
func (s *Store) Tasks(ctx context.Context, app string) ([]Task, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE application_id=? ORDER BY rowid", app)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, e := s.scanTask(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Store) DueTasks(ctx context.Context, now int64) ([]Task, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE enabled=1 AND schedule<>'' AND next_at>0 AND next_at<=? AND application_id IN (SELECT id FROM applications WHERE deleted_at=0) ORDER BY next_at LIMIT 32", now)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, e := s.scanTask(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Store) SaveTask(ctx context.Context, t Task) error {
	raw, e := s.sealJSON(t.Command, "task")
	if e != nil {
		return e
	}
	result, e := s.db.ExecContext(ctx, `INSERT INTO tasks (`+taskColumns+`,updated_at) SELECT ?,?,?,?,?,?,?,?,?,?,?,? WHERE EXISTS(SELECT 1 FROM applications WHERE id=? AND deleted_at=0) AND ((SELECT count(*) FROM tasks WHERE application_id=?)<64 OR EXISTS(SELECT 1 FROM tasks WHERE id=? AND application_id=?))
 ON CONFLICT(id) DO UPDATE SET name=excluded.name,command=excluded.command,enabled=excluded.enabled,schedule=excluded.schedule,timezone=excluded.timezone,overlap=excluded.overlap,missed=excluded.missed,next_at=excluded.next_at,waiting=0,updated_at=excluded.updated_at WHERE tasks.application_id=excluded.application_id`, t.ID, t.ApplicationID, t.Name, raw, t.Enabled, t.Schedule, t.Timezone, t.Overlap, t.Missed, t.NextAt, false, time.Now().Unix(), t.ApplicationID, t.ApplicationID, t.ID, t.ApplicationID)
	if e != nil {
		return e
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return errors.New("Applications support up to 64 saved tasks")
	}
	return nil
}
func (s *Store) AdvanceTask(ctx context.Context, id string, old, next int64) error {
	_, e := s.db.ExecContext(ctx, "UPDATE tasks SET next_at=?,waiting=0 WHERE id=? AND next_at=?", next, id, old)
	return e
}
func (s *Store) DeleteTask(ctx context.Context, app, id string) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE application_id=? AND id=?", app, id)
	return e
}

const runColumns = "id,deployment_id,stage,state,command,container_id,exec_id,exit_code,error,started_at,finished_at"

func (s *Store) scanRun(row scanner) (TaskRun, error) {
	var r TaskRun
	var raw string
	e := row.Scan(&r.ID, &r.DeploymentID, &r.Stage, &r.State, &raw, &r.ContainerID, &r.ExecID, &r.ExitCode, &r.Error, &r.StartedAt, &r.FinishedAt)
	if e == nil {
		e = s.unsealJSON(raw, "task-run", &r.Command)
	}
	return r, e
}
func (s *Store) EnsureTaskRun(ctx context.Context, r TaskRun) (TaskRun, error) {
	raw, e := s.sealJSON(r.Command, "task-run")
	if e != nil {
		return r, e
	}
	_, e = s.db.ExecContext(ctx, "INSERT INTO task_runs ("+runColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(deployment_id,stage) DO NOTHING", r.ID, r.DeploymentID, r.Stage, "prepared", raw, "", "", -1, "", time.Now().Unix(), 0)
	if e != nil {
		return r, e
	}
	return s.scanRun(s.db.QueryRowContext(ctx, "SELECT "+runColumns+" FROM task_runs WHERE deployment_id=? AND stage=?", r.DeploymentID, r.Stage))
}
func (s *Store) SaveTaskRun(ctx context.Context, r TaskRun) error {
	_, e := s.db.ExecContext(ctx, "UPDATE task_runs SET state=?,container_id=?,exec_id=?,exit_code=?,error=?,finished_at=? WHERE id=?", r.State, r.ContainerID, r.ExecID, r.ExitCode, r.Error, r.FinishedAt, r.ID)
	return e
}
func (s *Store) TaskRuns(ctx context.Context, app string) ([]TaskRun, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT "+prefixColumns("r.", runColumns)+" FROM task_runs r JOIN deployments d ON d.id=r.deployment_id WHERE d.application_id=? ORDER BY r.started_at DESC,r.rowid DESC LIMIT 100", app)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []TaskRun{}
	for rows.Next() {
		r, e := s.scanRun(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) TaskRun(ctx context.Context, app, id string) (TaskRun, error) {
	return s.scanRun(s.db.QueryRowContext(ctx, "SELECT "+prefixColumns("r.", runColumns)+" FROM task_runs r JOIN deployments d ON d.id=r.deployment_id WHERE d.application_id=? AND r.id=?", app, id))
}
func prefixColumns(prefix, columns string) string {
	out := ""
	for _, c := range strings.Split(columns, ",") {
		if out != "" {
			out += ","
		}
		out += prefix + c
	}
	return out
}
func (s *Store) SaveConfig(ctx context.Context, app string, c Config) error {
	raw, e := s.marshalConfig(c)
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, "UPDATE applications SET config_json=?,updated_at=? WHERE id=? AND deleted_at=0", raw, time.Now().Unix(), app)
	return e
}

func (s *Store) WaitTask(ctx context.Context, id string, at int64) error {
	_, e := s.db.ExecContext(ctx, "UPDATE tasks SET waiting=1 WHERE id=? AND next_at=?", id, at)
	return e
}
