package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"shipper/internal/auth"
)

type FactorState struct {
	Enrolled bool `json:"enrolled"`
	Required bool `json:"required"`
}

var ErrFactorCode = errors.New("The code is invalid or has already been used")
var ErrEnrollment = errors.New("Start setup again to get a fresh key")
var ErrFactorRequired = errors.New("This instance requires two-factor authentication")

func (s *Store) FactorState(ctx context.Context, user string) (FactorState, error) {
	var v FactorState
	e := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM account_factors WHERE user_id=?),
		EXISTS(SELECT 1 FROM settings WHERE key='require_factor' AND value='true')`, user).Scan(&v.Enrolled, &v.Required)
	return v, e
}

func (s *Store) StartFactor(ctx context.Context, sessionHash, user, secret string, now time.Time) error {
	sealed, e := s.seal(secret, "account-factor:"+user)
	if e != nil {
		return e
	}
	result, e := s.db.ExecContext(ctx, `INSERT INTO factor_enrollments(session_hash,secret,expires_at)
		SELECT token_hash,?,? FROM sessions WHERE token_hash=? AND user_id=? AND expires_at>?
		AND NOT EXISTS(SELECT 1 FROM account_factors WHERE user_id=?)
		ON CONFLICT(session_hash) DO UPDATE SET secret=excluded.secret, expires_at=excluded.expires_at`,
		sealed, now.Add(10*time.Minute).Unix(), sessionHash, user, now.Unix(), user)
	if e != nil {
		return e
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrEnrollment
	}
	return nil
}

func (s *Store) ConfirmFactor(ctx context.Context, sessionHash, user, code, nextHash, csrf string, now time.Time) ([]string, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	var sealed string
	e = tx.QueryRowContext(ctx, `SELECT f.secret FROM factor_enrollments f JOIN sessions s ON s.token_hash=f.session_hash
		WHERE f.session_hash=? AND s.user_id=? AND s.expires_at>? AND f.expires_at>?`, sessionHash, user, now.Unix(), now.Unix()).Scan(&sealed)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, ErrEnrollment
	}
	if e != nil {
		return nil, e
	}
	secret, e := s.unseal(sealed, "account-factor:"+user)
	if e != nil {
		return nil, e
	}
	step, valid := auth.MatchFactor(secret, strings.TrimSpace(code), now)
	if !valid {
		return nil, ErrFactorCode
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO account_factors(user_id,secret,last_step) VALUES (?,?,?)", user, sealed, step); e != nil {
		return nil, e
	}
	codes, e := replaceRecoveryCodes(ctx, tx, user)
	if e != nil {
		return nil, e
	}
	if e = rotateFactorSession(ctx, tx, user, sessionHash, nextHash, csrf, now, true, true); e != nil {
		return nil, e
	}
	if e = auditAccount(ctx, tx, user, "factor_enabled", now); e != nil {
		return nil, e
	}
	return codes, tx.Commit()
}

func replaceRecoveryCodes(ctx context.Context, tx *sql.Tx, user string) ([]string, error) {
	if _, e := tx.ExecContext(ctx, "DELETE FROM factor_recovery_codes WHERE user_id=?", user); e != nil {
		return nil, e
	}
	codes := make([]string, 8)
	for i := range codes {
		raw := auth.Token()[:24]
		if _, e := tx.ExecContext(ctx, "INSERT INTO factor_recovery_codes(user_id,code_hash) VALUES (?,?)", user, auth.Hash(raw)); e != nil {
			return nil, e
		}
		codes[i] = raw[:8] + "-" + raw[8:16] + "-" + raw[16:]
	}
	return codes, nil
}

func (s *Store) consumeFactor(ctx context.Context, tx *sql.Tx, user, code string, now time.Time, recovery bool) error {
	code = strings.TrimSpace(code)
	if recovery && len(code) > 6 {
		raw := strings.ToLower(strings.ReplaceAll(code, "-", ""))
		if len(raw) != 24 {
			return ErrFactorCode
		}
		result, e := tx.ExecContext(ctx, "DELETE FROM factor_recovery_codes WHERE user_id=? AND code_hash=?", user, auth.Hash(raw))
		if e != nil {
			return e
		}
		if n, _ := result.RowsAffected(); n == 1 {
			return auditAccount(ctx, tx, user, "recovery_code_used", now)
		}
		return ErrFactorCode
	}
	var sealed string
	e := tx.QueryRowContext(ctx, "SELECT secret FROM account_factors WHERE user_id=?", user).Scan(&sealed)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrFactorCode
	}
	if e != nil {
		return e
	}
	secret, e := s.unseal(sealed, "account-factor:"+user)
	if e != nil {
		return e
	}
	step, valid := auth.MatchFactor(secret, code, now)
	if !valid {
		return ErrFactorCode
	}
	result, e := tx.ExecContext(ctx, "UPDATE account_factors SET last_step=? WHERE user_id=? AND last_step<?", step, user, step)
	if e != nil {
		return e
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrFactorCode
	}
	return nil
}

func rotateFactorSession(ctx context.Context, tx *sql.Tx, user, previous, next, csrf string, now time.Time, verified, all bool) error {
	return rotateSession(ctx, tx, user, previous, next, csrf, now, 7*24*time.Hour, &verified, all)
}

func (s *Store) VerifyFactor(ctx context.Context, sessionHash, user, code, next, csrf string, now time.Time) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.consumeFactor(ctx, tx, user, code, now, true); e != nil {
		return e
	}
	if e = rotateFactorSession(ctx, tx, user, sessionHash, next, csrf, now, true, false); e != nil {
		return e
	}
	return tx.Commit()
}

func auditAccount(ctx context.Context, tx *sql.Tx, user, action string, now time.Time) error {
	return auditAccountDetail(ctx, tx, user, "", action, "", now)
}

func (s *Store) SetFactorPolicy(ctx context.Context, sessionHash, user, code string, required bool, now time.Time) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var enrolled, active bool
	if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM account_factors WHERE user_id=?),
		EXISTS(SELECT 1 FROM sessions WHERE token_hash=? AND user_id=? AND expires_at>?
		AND (factor_verified=1 OR NOT EXISTS(SELECT 1 FROM account_factors WHERE user_id=?)))`,
		user, sessionHash, user, now.Unix(), user).Scan(&enrolled, &active); e != nil {
		return e
	}
	if !active {
		return sql.ErrNoRows
	}
	if enrolled {
		if e = s.consumeFactor(ctx, tx, user, code, now, false); e != nil {
			return e
		}
	}
	value, event := "false", "factor_policy_optional"
	if required {
		value, event = "true", "factor_policy_required"
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES ('require_factor',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", value); e != nil {
		return e
	}
	if e = auditAccount(ctx, tx, user, event, now); e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Store) ChangeFactor(ctx context.Context, sessionHash, user, code, nextHash, csrf string, disable bool, now time.Time) ([]string, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if disable {
		var required bool
		if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM settings WHERE key='require_factor' AND value='true')").Scan(&required); e != nil {
			return nil, e
		}
		if required {
			return nil, ErrFactorRequired
		}
	}
	if e = s.consumeFactor(ctx, tx, user, code, now, false); e != nil {
		return nil, e
	}
	var codes []string
	action := "factor_disabled"
	if disable {
		if _, e = tx.ExecContext(ctx, "DELETE FROM account_factors WHERE user_id=?", user); e != nil {
			return nil, e
		}
		if _, e = tx.ExecContext(ctx, "DELETE FROM factor_recovery_codes WHERE user_id=?", user); e != nil {
			return nil, e
		}
	} else {
		codes, e = replaceRecoveryCodes(ctx, tx, user)
		action = "recovery_codes_replaced"
		if e != nil {
			return nil, e
		}
	}
	if e = rotateFactorSession(ctx, tx, user, sessionHash, nextHash, csrf, now, !disable, true); e != nil {
		return nil, e
	}
	if e = auditAccount(ctx, tx, user, action, now); e != nil {
		return nil, e
	}
	return codes, tx.Commit()
}

func (s *Store) RecoverFactor(ctx context.Context, email string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var user string
	if e = tx.QueryRowContext(ctx, "SELECT id FROM administrators WHERE email=?", email).Scan(&user); e != nil {
		return e
	}
	for _, query := range []string{
		"DELETE FROM sessions WHERE user_id=?",
		"DELETE FROM account_factors WHERE user_id=?",
		"DELETE FROM factor_recovery_codes WHERE user_id=?",
		"DELETE FROM operation_tokens WHERE user_id=?",
	} {
		if _, e = tx.ExecContext(ctx, query, user); e != nil {
			return e
		}
	}
	if e = auditAccount(ctx, tx, user, "offline_factor_recovery", time.Now()); e != nil {
		return e
	}
	return tx.Commit()
}
