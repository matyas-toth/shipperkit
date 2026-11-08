package store

import (
	"context"
	"errors"
	"time"
)

var ErrTerminalLimit = errors.New("terminal session limit reached")

// TerminalSession deliberately contains no terminal input or output.
type TerminalSession struct {
	ID            string `json:"id"`
	UserID        string `json:"-"`
	AuthHash      string `json:"-"`
	ApplicationID string `json:"applicationId"`
	ContainerID   string `json:"containerId"`
	ExpiresAt     int64  `json:"expiresAt"`
	EndedAt       int64  `json:"endedAt"`
}

func (s *Store) CreateTerminal(ctx context.Context, v TerminalSession) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var total, owner int
	if err = tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(sum(user_id=?),0) FROM terminal_sessions WHERE ended_at=0 AND expires_at>?`, v.UserID, time.Now().Unix()).Scan(&total, &owner); err != nil {
		return err
	}
	if total >= 8 || owner >= 2 {
		return ErrTerminalLimit
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO terminal_sessions(id,user_id,auth_hash,application_id,container_id,expires_at) VALUES(?,?,?,?,?,?)`, v.ID, v.UserID, v.AuthHash, v.ApplicationID, v.ContainerID, v.ExpiresAt); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO terminal_audit(session_id,user_id,application_id,container_id,event,created_at) VALUES(?,?,?,?,?,?)`, v.ID, v.UserID, v.ApplicationID, v.ContainerID, "start", time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Terminal(ctx context.Context, id string) (v TerminalSession, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT id,user_id,auth_hash,application_id,container_id,expires_at,ended_at FROM terminal_sessions WHERE id=?`, id).Scan(&v.ID, &v.UserID, &v.AuthHash, &v.ApplicationID, &v.ContainerID, &v.ExpiresAt, &v.EndedAt)
	return
}
func (s *Store) EndTerminal(ctx context.Context, id, event, reason string) error {
	if event != "end" && event != "revoke" {
		return errors.New("invalid terminal event")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE terminal_sessions SET ended_at=? WHERE id=? AND ended_at=0`, time.Now().Unix(), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO terminal_audit(session_id,user_id,application_id,container_id,event,reason,created_at) SELECT id,user_id,application_id,container_id,?,?,? FROM terminal_sessions WHERE id=?`, event, reason, time.Now().Unix(), id); err != nil {
		return err
	}
	// A bounded metadata trail never records shell commands, output or credentials.
	if _, err = tx.ExecContext(ctx, `DELETE FROM terminal_audit WHERE sequence NOT IN (SELECT sequence FROM terminal_audit ORDER BY sequence DESC LIMIT 10000)`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM terminal_sessions WHERE ended_at>0 AND id NOT IN (SELECT id FROM terminal_sessions ORDER BY expires_at DESC LIMIT 1000)`); err != nil {
		return err
	}
	return tx.Commit()
}

type TerminalAudit struct {
	Sequence      int64  `json:"sequence"`
	SessionID     string `json:"sessionId"`
	UserID        string `json:"userId"`
	ApplicationID string `json:"applicationId"`
	ContainerID   string `json:"containerId"`
	Event         string `json:"event"`
	Reason        string `json:"reason"`
	CreatedAt     int64  `json:"createdAt"`
}

func (s *Store) TerminalAudit(ctx context.Context, app string) ([]TerminalAudit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sequence,session_id,user_id,application_id,container_id,event,reason,created_at FROM terminal_audit WHERE application_id=? ORDER BY sequence DESC LIMIT 100`, app)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TerminalAudit{}
	for rows.Next() {
		var v TerminalAudit
		if err = rows.Scan(&v.Sequence, &v.SessionID, &v.UserID, &v.ApplicationID, &v.ContainerID, &v.Event, &v.Reason, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
