package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type OAuthState struct {
	Provider  string
	Nonce     string
	Verifier  string
	Redirect  string
	Mode      string
	UserID    string
	ExpiresAt int64
}

func (s *Store) SaveOAuthState(ctx context.Context, hash string, state OAuthState, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO oauth_states(hash,provider,nonce,verifier,redirect,mode,user_id,created_at,expires_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(hash) DO UPDATE SET nonce=excluded.nonce,verifier=excluded.verifier,redirect=excluded.redirect,
		mode=excluded.mode,user_id=excluded.user_id,expires_at=excluded.expires_at`,
		hash, state.Provider, state.Nonce, state.Verifier, state.Redirect, state.Mode, state.UserID, now.Unix(), state.ExpiresAt)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM oauth_states WHERE expires_at<=?", now.Unix())
	return err
}

// ConsumeOAuthState makes the state value one use even when a provider retries
// a callback. The delete and the read share one transaction.
func (s *Store) ConsumeOAuthState(ctx context.Context, hash string, now time.Time) (OAuthState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OAuthState{}, err
	}
	defer tx.Rollback()
	var state OAuthState
	err = tx.QueryRowContext(ctx, `SELECT provider,nonce,verifier,redirect,mode,user_id,expires_at
		FROM oauth_states WHERE hash=? AND expires_at>?`, hash, now.Unix()).Scan(&state.Provider, &state.Nonce, &state.Verifier, &state.Redirect, &state.Mode, &state.UserID, &state.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return state, sql.ErrNoRows
	}
	if err != nil {
		return state, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM oauth_states WHERE hash=?", hash); err != nil {
		return state, err
	}
	return state, tx.Commit()
}
