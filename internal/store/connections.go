package store

import (
	"context"
	"errors"
	"time"
)

type Connection struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Kind      string           `json:"kind"`
	Host      string           `json:"host"`
	UpdatedAt int64            `json:"updatedAt"`
	Secret    ConnectionSecret `json:"-"`
}
type ConnectionSecret struct {
	Token          string `json:"token,omitempty"`
	AppID          int64  `json:"appId,omitempty"`
	InstallationID int64  `json:"installationId,omitempty"`
	PrivateKey     string `json:"privateKey,omitempty"`
	WebhookSecret  string `json:"webhookSecret,omitempty"`
	Slug           string `json:"slug,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	AllowPrivate   bool   `json:"allowPrivate,omitempty"`
}

const connectionColumns = "id,name,kind,host,secret,updated_at"

func (s *Store) scanConnection(row scanner) (Connection, error) {
	var c Connection
	var raw string
	e := row.Scan(&c.ID, &c.Name, &c.Kind, &c.Host, &raw, &c.UpdatedAt)
	if e == nil {
		e = s.unsealJSON(raw, "connection:"+c.ID, &c.Secret)
	}
	return c, e
}
func (s *Store) Connection(ctx context.Context, id string) (Connection, error) {
	return s.scanConnection(s.db.QueryRowContext(ctx, "SELECT "+connectionColumns+" FROM connections WHERE id=?", id))
}
func (s *Store) Connections(ctx context.Context) ([]Connection, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT "+connectionColumns+" FROM connections ORDER BY name,id")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		c, e := s.scanConnection(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) SaveConnection(ctx context.Context, c Connection) error {
	raw, e := s.sealJSON(c.Secret, "connection:"+c.ID)
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, "INSERT INTO connections("+connectionColumns+") VALUES (?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,kind=excluded.kind,host=excluded.host,secret=excluded.secret,updated_at=excluded.updated_at", c.ID, c.Name, c.Kind, c.Host, raw, time.Now().UnixNano())
	return e
}
func (s *Store) DeleteConnection(ctx context.Context, id string) error {
	var used int
	e := s.db.QueryRowContext(ctx, "SELECT count(*) FROM applications WHERE deleted_at=0 AND (json_extract(config_json,'$.sourceId')=? OR json_extract(config_json,'$.registryId')=?)", id, id).Scan(&used)
	if e != nil {
		return e
	}
	if used > 0 {
		return errors.New("This connection is used by an application")
	}
	_, e = s.db.ExecContext(ctx, "DELETE FROM connections WHERE id=?", id)
	return e
}

type ProviderFlow struct{ Stage, ConnectionID, Origin string }

func (s *Store) CreateProviderFlow(ctx context.Context, hash, id, origin string) error {
	_, e := s.db.ExecContext(ctx, "INSERT INTO provider_flows VALUES (?,'manifest',?,?,?)", hash, id, origin, time.Now().Add(15*time.Minute).Unix())
	return e
}
func (s *Store) AdvanceProviderFlow(ctx context.Context, hash, from, to string) (ProviderFlow, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return ProviderFlow{}, e
	}
	defer tx.Rollback()
	var f ProviderFlow
	e = tx.QueryRowContext(ctx, "SELECT stage,connection_id,origin FROM provider_flows WHERE hash=? AND stage=? AND expires_at>?", hash, from, time.Now().Unix()).Scan(&f.Stage, &f.ConnectionID, &f.Origin)
	if e != nil {
		return f, e
	}
	_, e = tx.ExecContext(ctx, "UPDATE provider_flows SET stage=? WHERE hash=?", to, hash)
	if e != nil {
		return f, e
	}
	return f, tx.Commit()
}
func (s *Store) RotateDeployToken(ctx context.Context, app, hash string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "DELETE FROM deploy_tokens WHERE application_id=?", app); e != nil {
		return e
	}
	if hash != "" {
		if _, e = tx.ExecContext(ctx, "INSERT INTO deploy_tokens VALUES (?,?,?)", hash, app, time.Now().Unix()); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) DeployToken(ctx context.Context, hash string) (string, error) {
	var app string
	e := s.db.QueryRowContext(ctx, "SELECT application_id FROM deploy_tokens WHERE hash=?", hash).Scan(&app)
	return app, e
}
func (s *Store) HasDeployToken(ctx context.Context, app string) bool {
	var n int
	_ = s.db.QueryRowContext(ctx, "SELECT count(*) FROM deploy_tokens WHERE application_id=?", app).Scan(&n)
	return n > 0
}

type DeployIntent struct {
	ApplicationID string
	Revision      string
	Actor         string
}

func (s *Store) AcceptWebhook(ctx context.Context, connection, delivery string, jobs []DeployIntent) (bool, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	result, e := tx.ExecContext(ctx, "INSERT INTO webhook_deliveries VALUES (?,?,?) ON CONFLICT DO NOTHING", connection, delivery, time.Now().Unix())
	if e != nil {
		return false, e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return false, nil
	}
	for _, job := range jobs {
		_, e = tx.ExecContext(ctx, "INSERT INTO deploy_intents VALUES (?,?,?,?) ON CONFLICT(application_id) DO UPDATE SET revision=excluded.revision,actor=excluded.actor,received_at=excluded.received_at", job.ApplicationID, job.Revision, job.Actor, time.Now().UnixNano())
		if e != nil {
			return false, e
		}
	}
	_, e = tx.ExecContext(ctx, "DELETE FROM webhook_deliveries WHERE created_at<?", time.Now().Add(-90*24*time.Hour).Unix())
	if e != nil {
		return false, e
	}
	return true, tx.Commit()
}
func (s *Store) DeployIntents(ctx context.Context) ([]DeployIntent, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT application_id,revision,actor FROM deploy_intents ORDER BY received_at LIMIT 32")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []DeployIntent{}
	for rows.Next() {
		var d DeployIntent
		if e = rows.Scan(&d.ApplicationID, &d.Revision, &d.Actor); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) ClearDeployIntent(ctx context.Context, app, revision string) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM deploy_intents WHERE application_id=? AND revision=?", app, revision)
	return e
}
func (s *Store) CancelDeployment(ctx context.Context, app, id string) error {
	result, e := s.db.ExecContext(ctx, "UPDATE deployments SET cancel_requested=1,state=CASE WHEN state='queued' THEN 'cancelled' ELSE state END,finished_at=CASE WHEN state='queued' THEN ? ELSE finished_at END WHERE id=? AND application_id=? AND operation IN ('deploy','apply') AND state IN ('queued','cloning','building','packaging','pulling')", time.Now().Unix(), id, app)
	if e != nil {
		return e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("This operation has passed the cancellable build stage")
	}
	return nil
}
func (s *Store) CancellationRequested(ctx context.Context, id string) bool {
	var requested bool
	_ = s.db.QueryRowContext(ctx, "SELECT cancel_requested FROM deployments WHERE id=?", id).Scan(&requested)
	return requested
}
