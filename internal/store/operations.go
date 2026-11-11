package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"shipper/internal/auth"
)

type OperationToken struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	UserID       string   `json:"-"`
	Hash         string   `json:"-"`
	Scopes       []string `json:"scopes"`
	Applications []string `json:"applications"`
	ExpiresAt    int64    `json:"expiresAt"`
	CreatedAt    int64    `json:"createdAt"`
}

var applicationScopes = map[string]struct{}{
	"read": {}, "logs": {}, "deploy": {}, "execute": {}, "secrets": {}, "destructive": {},
}

func (t OperationToken) Allows(scope, app string) bool {
	found := false
	for _, s := range t.Scopes {
		if s == scope {
			found = true
		}
		if s == "full" && scope != "host-admin" {
			if _, ok := applicationScopes[scope]; ok {
				found = true
			}
		}
	}
	if !found {
		return false
	}
	if app == "" {
		return len(t.Applications) == 0
	}
	if len(t.Applications) == 0 {
		return true
	}
	for _, id := range t.Applications {
		if id == app {
			return true
		}
	}
	return false
}
func (s *Store) SaveOperationToken(ctx context.Context, t OperationToken) error {
	scopes, _ := json.Marshal(t.Scopes)
	apps, _ := json.Marshal(t.Applications)
	_, e := s.db.ExecContext(ctx, "INSERT INTO operation_tokens VALUES (?,?,?,?,?,?,?,?)", t.ID, t.Name, t.Hash, t.UserID, string(scopes), string(apps), t.ExpiresAt, time.Now().Unix())
	return e
}
func scanOperationToken(row interface{ Scan(...any) error }) (t OperationToken, e error) {
	var scopes, apps string
	e = row.Scan(&t.ID, &t.Name, &t.Hash, &t.UserID, &scopes, &apps, &t.ExpiresAt, &t.CreatedAt)
	if e == nil {
		e = json.Unmarshal([]byte(scopes), &t.Scopes)
	}
	if e == nil {
		e = json.Unmarshal([]byte(apps), &t.Applications)
	}
	return
}
func (s *Store) OperationToken(ctx context.Context, raw string) (OperationToken, error) {
	return scanOperationToken(s.db.QueryRowContext(ctx, `SELECT t.id,t.name,t.token_hash,t.user_id,t.scopes,t.applications,t.expires_at,t.created_at
		FROM operation_tokens t WHERE t.token_hash=? AND t.expires_at>?
		AND EXISTS(SELECT 1 FROM administrators a WHERE a.id=t.user_id AND a.disabled=0 AND a.password_change_required=0)
		AND (NOT EXISTS(SELECT 1 FROM settings WHERE key='require_factor' AND value='true')
			OR EXISTS(SELECT 1 FROM account_factors f WHERE f.user_id=t.user_id))`, auth.Hash(raw), time.Now().Unix()))
}
func (s *Store) OperationTokens(ctx context.Context) ([]OperationToken, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,name,token_hash,user_id,scopes,applications,expires_at,created_at FROM operation_tokens ORDER BY created_at DESC LIMIT 100")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []OperationToken{}
	for rows.Next() {
		t, e := scanOperationToken(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, t)
	}
	return items, rows.Err()
}
func (s *Store) RevokeOperationToken(ctx context.Context, id string) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM operation_tokens WHERE id=?", id)
	return e
}

type Observation struct {
	Stale         bool            `json:"stale"`
	AgeSeconds    int64           `json:"ageSeconds"`
	ID            string          `json:"id"`
	ApplicationID string          `json:"applicationId"`
	Kind          string          `json:"kind"`
	State         string          `json:"state"`
	Summary       string          `json:"summary"`
	Reference     string          `json:"reference"`
	ObservedAt    int64           `json:"observedAt"`
	Details       json.RawMessage `json:"details"`
}
type OperationEvent struct {
	Sequence      int64  `json:"sequence"`
	ApplicationID string `json:"applicationId"`
	Kind          string `json:"kind"`
	State         string `json:"state"`
	Summary       string `json:"summary"`
	Reference     string `json:"reference"`
	CreatedAt     int64  `json:"createdAt"`
}

// Observe atomically persists the fact and its transition/outbox. Repeated
// unchanged failures update freshness without creating notification storms.
func (s *Store) Observe(ctx context.Context, o Observation) error {
	if o.Details == nil {
		o.Details = json.RawMessage(`{}`)
	}
	if o.ObservedAt == 0 {
		o.ObservedAt = time.Now().Unix()
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var oldState, oldRef string
	e = tx.QueryRowContext(ctx, "SELECT state,reference FROM observations WHERE id=?", o.ID).Scan(&oldState, &oldRef)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO observations VALUES (?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET kind=excluded.kind,state=excluded.state,summary=excluded.summary,reference=excluded.reference,observed_at=excluded.observed_at,details=excluded.details`, o.ID, o.ApplicationID, o.Kind, o.State, o.Summary, o.Reference, o.ObservedAt, string(o.Details))
	if e != nil {
		return e
	}
	if oldState != o.State || oldRef != o.Reference {
		result, e := tx.ExecContext(ctx, "INSERT INTO operation_events(application_id,kind,state,summary,reference,created_at) VALUES (?,?,?,?,?,?)", o.ApplicationID, o.Kind, o.State, o.Summary, o.Reference, o.ObservedAt)
		if e != nil {
			return e
		}
		if o.State == "failed" || o.State == "warning" || (oldState == "failed" || oldState == "warning") && o.State == "healthy" {
			id, _ := result.LastInsertId()
			if _, e = tx.ExecContext(ctx, "INSERT INTO notification_outbox(sequence,next_at) VALUES (?,?)", id, o.ObservedAt); e != nil {
				return e
			}
		}
	}
	return tx.Commit()
}
func (s *Store) Observations(ctx context.Context, app string) ([]Observation, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,application_id,kind,state,summary,reference,observed_at,details FROM observations WHERE application_id=? ORDER BY kind,id LIMIT 256", app)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []Observation{}
	for rows.Next() {
		var o Observation
		var details string
		if e = rows.Scan(&o.ID, &o.ApplicationID, &o.Kind, &o.State, &o.Summary, &o.Reference, &o.ObservedAt, &details); e != nil {
			return nil, e
		}
		o.Details = json.RawMessage(details)
		o.AgeSeconds = max(0, time.Now().Unix()-o.ObservedAt)
		staleAfter := int64(60)
		if o.Kind == "certificate" {
			staleAfter = 15 * 60
		}
		o.Stale = o.ObservedAt == 0 || o.AgeSeconds > staleAfter
		items = append(items, o)
	}
	return items, rows.Err()
}
func (s *Store) OperationEvents(ctx context.Context, app string, after int64) ([]OperationEvent, error) {
	order := "ASC"
	if after < 0 {
		order = "DESC"
	}
	rows, e := s.db.QueryContext(ctx, "SELECT sequence,application_id,kind,state,summary,reference,created_at FROM operation_events WHERE (?='' OR application_id=?) AND sequence>? ORDER BY sequence "+order+" LIMIT 100", app, app, after)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []OperationEvent{}
	for rows.Next() {
		var v OperationEvent
		if e = rows.Scan(&v.Sequence, &v.ApplicationID, &v.Kind, &v.State, &v.Summary, &v.Reference, &v.CreatedAt); e != nil {
			return nil, e
		}
		items = append(items, v)
	}
	if after < 0 {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}
	return items, rows.Err()
}
func (s *Store) NextNotification(ctx context.Context) (OperationEvent, int, error) {
	var v OperationEvent
	var attempts int
	e := s.db.QueryRowContext(ctx, `SELECT e.sequence,e.application_id,e.kind,e.state,e.summary,e.reference,e.created_at,o.attempts FROM notification_outbox o JOIN operation_events e USING(sequence) WHERE delivered_at=0 AND attempts<5 AND next_at<=? ORDER BY sequence LIMIT 1`, time.Now().Unix()).Scan(&v.Sequence, &v.ApplicationID, &v.Kind, &v.State, &v.Summary, &v.Reference, &v.CreatedAt, &attempts)
	return v, attempts, e
}
func (s *Store) FinishNotification(ctx context.Context, id int64, attempt int, success bool, failure ...string) error {
	delivered := int64(0)
	message := "Delivery failed"
	if len(failure) > 0 && failure[0] != "" {
		message = failure[0]
	}
	if success {
		delivered = time.Now().Unix()
		message = ""
	}
	_, e := s.db.ExecContext(ctx, "UPDATE notification_outbox SET attempts=attempts+1,next_at=?,delivered_at=?,error=? WHERE sequence=?", time.Now().Add(time.Duration(1<<min(attempt, 4))*time.Minute).Unix(), delivered, message, id)
	return e
}
func (s *Store) NotificationStatus(ctx context.Context) (map[string]int, error) {
	var pending, failed int
	e := s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(delivered_at=0 AND attempts<5),0),COALESCE(SUM(delivered_at=0 AND attempts>=5),0) FROM notification_outbox").Scan(&pending, &failed)
	return map[string]int{"pending": pending, "failed": failed}, e
}

type FailedNotification struct {
	Sequence  int64  `json:"sequence"`
	Kind      string `json:"kind"`
	Summary   string `json:"summary"`
	CreatedAt int64  `json:"createdAt"`
	Attempts  int    `json:"attempts"`
	Error     string `json:"error"`
}

func (s *Store) FailedNotifications(ctx context.Context) ([]FailedNotification, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT e.sequence,e.kind,e.summary,e.created_at,o.attempts,o.error FROM notification_outbox o JOIN operation_events e USING(sequence) WHERE o.delivered_at=0 AND o.attempts>=5 ORDER BY e.sequence DESC LIMIT 50`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []FailedNotification{}
	for rows.Next() {
		var item FailedNotification
		if e = rows.Scan(&item.Sequence, &item.Kind, &item.Summary, &item.CreatedAt, &item.Attempts, &item.Error); e != nil {
			return nil, e
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RetryNotification(ctx context.Context, sequence int64) error {
	result, e := s.db.ExecContext(ctx, `UPDATE notification_outbox SET attempts=0,next_at=unixepoch(),error='' WHERE sequence=? AND delivered_at=0 AND attempts>=5`, sequence)
	if e != nil {
		return e
	}
	count, e := result.RowsAffected()
	if e == nil && count == 0 {
		return sql.ErrNoRows
	}
	return e
}
func (s *Store) SavePrivateSetting(ctx context.Context, key, value string) error {
	sealed, e := s.seal(value, "setting:"+key)
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, sealed)
	return e
}

// Bound incident history without deleting release records or application data.
func (s *Store) RetainObservations(ctx context.Context) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM operation_events WHERE sequence NOT IN (SELECT sequence FROM operation_events ORDER BY sequence DESC LIMIT 10000) OR created_at<?", time.Now().Add(-30*24*time.Hour).Unix())
	return e
}

type SourceSnapshot struct {
	ID            string `json:"id"`
	OwnerID       string `json:"-"`
	ApplicationID string `json:"applicationId"`
	Digest        string `json:"digest"`
	Files         int    `json:"files"`
	Bytes         int64  `json:"bytes"`
	CreatedAt     int64  `json:"createdAt"`
	Provenance    string `json:"provenance"`
}

func (s *Store) SaveSourceSnapshot(ctx context.Context, v SourceSnapshot) error {
	_, e := s.db.ExecContext(ctx, "INSERT INTO source_snapshots(id,owner_id,application_id,digest,files,bytes,created_at,provenance) VALUES (?,?,?,?,?,?,?,?)", v.ID, v.OwnerID, v.ApplicationID, v.Digest, v.Files, v.Bytes, time.Now().Unix(), v.Provenance)
	return e
}
func (s *Store) SourceSnapshot(ctx context.Context, id string) (v SourceSnapshot, e error) {
	e = s.db.QueryRowContext(ctx, "SELECT id,owner_id,application_id,digest,files,bytes,created_at,provenance FROM source_snapshots WHERE id=? AND deleting=0", id).Scan(&v.ID, &v.OwnerID, &v.ApplicationID, &v.Digest, &v.Files, &v.Bytes, &v.CreatedAt, &v.Provenance)
	return
}
func (s *Store) SourceReferenced(ctx context.Context, id string) (bool, error) {
	var n int
	e := s.db.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM applications WHERE json_extract(config_json,'$.localSource')=?)+(SELECT COUNT(*) FROM deployments WHERE json_extract(config_json,'$.localSource')=?)", id, id).Scan(&n)
	return n > 0, e
}
func (s *Store) AbandonedSources(ctx context.Context) ([]SourceSnapshot, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id FROM source_snapshots s WHERE deleting=0 AND created_at<? AND NOT EXISTS(SELECT 1 FROM applications WHERE json_extract(config_json,'$.localSource')=s.id) AND NOT EXISTS(SELECT 1 FROM deployments WHERE json_extract(config_json,'$.localSource')=s.id) LIMIT 100", time.Now().Add(-24*time.Hour).Unix())
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	out := []SourceSnapshot{}
	for _, id := range ids {
		used, e := s.SourceReferenced(ctx, id)
		if e != nil {
			return nil, e
		}
		if !used {
			v, e := s.SourceSnapshot(ctx, id)
			if e == nil {
				out = append(out, v)
			}
		}
	}
	return out, nil
}
func (s *Store) DeleteSourceSnapshot(ctx context.Context, id string) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM source_snapshots WHERE id=?", id)
	return e
}
func SafeEventText(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	if len(s) > 512 {
		s = s[:512]
	}
	return s
}
