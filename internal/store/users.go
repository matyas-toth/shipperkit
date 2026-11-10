package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrLastAdmin = errors.New("At least one active administrator needs a working sign-in method")
var ErrIdentityConflict = errors.New("That sign-in identity is already linked to another account")
var ErrProviderLinked = errors.New("This account already has a different identity for that provider")
var ErrUserNotFound = errors.New("The user was not found")

var AuthProviders = []string{"google", "github", "discord"}

func ValidRole(role string) bool {
	return role == RoleAdmin || role == RoleDeveloper || role == RoleViewer
}

func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+userColumns+" FROM administrators ORDER BY CASE role WHEN 'admin' THEN 0 WHEN 'developer' THEN 1 ELSE 2 END, email")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, u)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Identities, err = s.identities(ctx, items[i].ID); err != nil {
			return nil, err
		}
	}
	factors, err := s.db.QueryContext(ctx, "SELECT user_id FROM account_factors")
	if err != nil {
		return nil, err
	}
	defer factors.Close()
	enabled := map[string]bool{}
	for factors.Next() {
		var id string
		if err = factors.Scan(&id); err != nil {
			return nil, err
		}
		enabled[id] = true
	}
	if err = factors.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].FactorEnabled = enabled[items[i].ID]
	}
	return items, nil
}

func (s *Store) User(ctx context.Context, id string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM administrators WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrUserNotFound
	}
	if err != nil {
		return u, err
	}
	items, err := s.identities(ctx, id)
	u.Identities = items
	if err == nil {
		err = s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM account_factors WHERE user_id=?)", id).Scan(&u.FactorEnabled)
	}
	return u, err
}

func (s *Store) identities(ctx context.Context, userID string) ([]Identity, error) {
	query := "SELECT provider,subject,email,created_at,last_login_at FROM user_identities"
	args := []any{}
	if userID != "" {
		query += " WHERE user_id=?"
		args = append(args, userID)
	}
	query += " ORDER BY provider"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Identity{}
	for rows.Next() {
		var item Identity
		if err = rows.Scan(&item.Provider, &item.Subject, &item.Email, &item.CreatedAt, &item.LastLoginAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, u User, actor string, now time.Time) error {
	if !ValidRole(u.Role) {
		return errors.New("Choose a valid role")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT INTO administrators(id,email,password_hash,role,disabled,password_change_required,created_at,updated_at,last_login_at) VALUES (?,?,?,?,0,?,?,?,0)",
		u.ID, u.Email, u.PasswordHash, u.Role, u.PasswordChangeRequired, now.Unix(), now.Unix())
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return errors.New("The user could not be created")
	}
	if err = auditAccountDetail(ctx, tx, u.ID, actor, "user_provisioned", u.Role+"/"+passwordState(u.PasswordHash), now); err != nil {
		return err
	}
	if err = ensureAdminRoute(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func passwordState(hash string) string {
	if hash == "" {
		return "provider_only"
	}
	return "temporary_password"
}

func (s *Store) UpdateUserRole(ctx context.Context, id, role, actor string, now time.Time) error {
	if !ValidRole(role) {
		return errors.New("Choose a valid role")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previous string
	if err = tx.QueryRowContext(ctx, "SELECT role FROM administrators WHERE id=?", id).Scan(&previous); err != nil {
		return ErrUserNotFound
	}
	if _, err = tx.ExecContext(ctx, "UPDATE administrators SET role=?,updated_at=? WHERE id=?", role, now.Unix(), id); err != nil {
		return err
	}
	if err = auditAccountDetail(ctx, tx, id, actor, "role_changed", previous+" to "+role, now); err != nil {
		return err
	}
	if err = ensureAdminRoute(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func revokeUserCredentials(ctx context.Context, tx *sql.Tx, id string) error {
	for _, query := range []string{
		"DELETE FROM sessions WHERE user_id=?",
		"DELETE FROM operation_tokens WHERE user_id=?",
		"DELETE FROM terminal_sessions WHERE user_id=?",
	} {
		if _, err := tx.ExecContext(ctx, query, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) SetUserDisabled(ctx context.Context, id string, disabled bool, actor string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE administrators SET disabled=?,updated_at=? WHERE id=?", disabled, now.Unix(), id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrUserNotFound
	}
	if disabled {
		if err = revokeUserCredentials(ctx, tx, id); err != nil {
			return err
		}
	}
	action := "user_enabled"
	if disabled {
		action = "user_disabled"
	}
	if err = auditAccountDetail(ctx, tx, id, actor, action, "", now); err != nil {
		return err
	}
	if err = ensureAdminRoute(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteUser(ctx context.Context, id, actor string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = revokeUserCredentials(ctx, tx, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM administrators WHERE id=?", id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrUserNotFound
	}
	if err = auditAccountDetail(ctx, tx, id, actor, "user_removed", "", now); err != nil {
		return err
	}
	if err = ensureAdminRoute(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ResetUserPassword(ctx context.Context, id, hash string, temporary bool, actor string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE administrators SET password_hash=?,password_change_required=?,updated_at=? WHERE id=?", hash, temporary, now.Unix(), id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrUserNotFound
	}
	if err = revokeUserCredentials(ctx, tx, id); err != nil {
		return err
	}
	if err = auditAccountDetail(ctx, tx, id, actor, "password_reset", passwordState(hash), now); err != nil {
		return err
	}
	if err = ensureAdminRoute(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RemoveUserPassword(ctx context.Context, id, actor string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE administrators SET password_hash='',password_change_required=0,updated_at=? WHERE id=?", now.Unix(), id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrUserNotFound
	}
	if err = auditAccountDetail(ctx, tx, id, actor, "password_removed", "", now); err != nil {
		return err
	}
	if err = ensureAdminRoute(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// ChangePassword is the restricted-session flow. It replaces the credential,
// clears the temporary-password gate and rotates the browser session while
// preserving the second-factor verification state and any pending provider link.
func (s *Store) ChangePassword(ctx context.Context, sessionHash, user, hash, next, csrf string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var temporary bool
	if err = tx.QueryRowContext(ctx, "SELECT password_change_required FROM administrators WHERE id=? AND disabled=0", user).Scan(&temporary); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE administrators SET password_hash=?,password_change_required=0,updated_at=? WHERE id=?", hash, now.Unix(), user); err != nil {
		return err
	}
	if err = rotateSession(ctx, tx, user, sessionHash, next, csrf, now, 7*24*time.Hour, nil, true); err != nil {
		return err
	}
	action := "password_changed"
	if temporary {
		action = "temporary_password_replaced"
	}
	if err = auditAccountDetail(ctx, tx, user, user, action, "", now); err != nil {
		return err
	}
	return tx.Commit()
}

// rotateSession issues a replacement browser session from an existing one. The
// new session inherits factor verification, the first-factor source and any
// pending provider identity so onboarding gates survive rotation.
func rotateSession(ctx context.Context, tx *sql.Tx, user, previous, next, csrf string, now time.Time, lifetime time.Duration, verified *bool, revokeOthers bool) error {
	var verifiedArg any
	if verified != nil {
		verifiedArg = *verified
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,factor_verified,first_factor,pending_provider,pending_subject,pending_email)
		SELECT ?,user_id,?,?,COALESCE(?,factor_verified),first_factor,pending_provider,pending_subject,pending_email
		FROM sessions WHERE token_hash=? AND user_id=? AND expires_at>?`, next, csrf, now.Add(lifetime).Unix(), verifiedArg, previous, user, now.Unix())
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return sql.ErrNoRows
	}
	if revokeOthers {
		_, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=? AND token_hash<>?", user, next)
	} else {
		_, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash=? AND user_id=?", previous, user)
	}
	return err
}

func (s *Store) ResetUserFactor(ctx context.Context, id, actor string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM account_factors WHERE user_id=?", id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM factor_recovery_codes WHERE user_id=?", id); err != nil {
		return err
	}
	if err = revokeUserCredentials(ctx, tx, id); err != nil {
		return err
	}
	if err = auditAccountDetail(ctx, tx, id, actor, "factor_reset", "", now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UnlinkIdentity(ctx context.Context, user, provider, actor string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "DELETE FROM user_identities WHERE user_id=? AND provider=?", user, provider)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return sql.ErrNoRows
	}
	if err = auditAccountDetail(ctx, tx, user, actor, "identity_unlinked", provider, now); err != nil {
		return err
	}
	if err = ensureAdminRoute(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) IdentityUser(ctx context.Context, provider, subject string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT a.id,a.email,a.password_hash,a.role,a.disabled,a.password_change_required,a.created_at,a.last_login_at
		FROM user_identities i JOIN administrators a ON a.id=i.user_id WHERE i.provider=? AND i.subject=?`, provider, subject))
}

func (s *Store) UserIdentity(ctx context.Context, user, provider string) (Identity, error) {
	var item Identity
	err := s.db.QueryRowContext(ctx, "SELECT provider,subject,email,created_at,last_login_at FROM user_identities WHERE user_id=? AND provider=?", user, provider).
		Scan(&item.Provider, &item.Subject, &item.Email, &item.CreatedAt, &item.LastLoginAt)
	return item, err
}

func (s *Store) UserEmailExists(ctx context.Context, email string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM administrators WHERE email=?", email).Scan(&count)
	return count > 0, err
}

// UsersByEmail resolves only active, admin-provisioned accounts. Email is
// normalized case-insensitively but never stripped or fuzzy matched.
func (s *Store) UsersByEmail(ctx context.Context, emails []string) ([]User, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(emails)), ",")
	args := make([]any, len(emails))
	for i, email := range emails {
		args[i] = email
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+userColumns+" FROM administrators WHERE disabled=0 AND email IN ("+placeholders+") ORDER BY created_at", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, u)
	}
	return items, rows.Err()
}

func (s *Store) TouchIdentity(ctx context.Context, provider, subject string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, "UPDATE user_identities SET last_login_at=? WHERE provider=? AND subject=?", now.Unix(), provider, subject)
	return err
}

// FinalizeSessionIdentity persists a provider binding only after every
// onboarding gate has passed, making the binding a deliberate completed action.
func (s *Store) FinalizeSessionIdentity(ctx context.Context, sessionHash string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var user, provider, subject, email string
	err = tx.QueryRowContext(ctx, "SELECT user_id,pending_provider,pending_subject,pending_email FROM sessions WHERE token_hash=? AND expires_at>?", sessionHash, now.Unix()).Scan(&user, &provider, &subject, &email)
	if errors.Is(err, sql.ErrNoRows) || provider == "" {
		return nil
	}
	if err != nil {
		return err
	}
	var other string
	err = tx.QueryRowContext(ctx, "SELECT user_id FROM user_identities WHERE provider=? AND subject=? AND user_id<>?", provider, subject, user).Scan(&other)
	if err == nil {
		return ErrIdentityConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	err = tx.QueryRowContext(ctx, "SELECT subject FROM user_identities WHERE provider=? AND user_id=?", provider, user).Scan(&other)
	if err == nil && other != subject {
		return ErrProviderLinked
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO user_identities(provider,subject,user_id,email,created_at,last_login_at) VALUES (?,?,?,?,?,?)
		ON CONFLICT(provider,subject) DO UPDATE SET email=excluded.email,last_login_at=excluded.last_login_at`, provider, subject, user, email, now.Unix(), now.Unix()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sessions SET pending_provider='',pending_subject='',pending_email='' WHERE user_id=?", user); err != nil {
		return err
	}
	if err = auditAccountDetail(ctx, tx, user, user, "identity_linked", provider, now); err != nil {
		return err
	}
	return tx.Commit()
}

type AuditEvent struct {
	Sequence  int64  `json:"sequence"`
	UserID    string `json:"userId"`
	ActorID   string `json:"actorId,omitempty"`
	Action    string `json:"action"`
	Detail    string `json:"detail,omitempty"`
	CreatedAt int64  `json:"createdAt"`
}

func (s *Store) UserAudit(ctx context.Context, user string) ([]AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT sequence,user_id,actor_id,action,detail,created_at FROM account_audit WHERE user_id=? ORDER BY sequence DESC LIMIT 50", user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AuditEvent{}
	for rows.Next() {
		var item AuditEvent
		if err = rows.Scan(&item.Sequence, &item.UserID, &item.ActorID, &item.Action, &item.Detail, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func auditAccountDetail(ctx context.Context, tx *sql.Tx, user, actor, action, detail string, now time.Time) error {
	if _, e := tx.ExecContext(ctx, "INSERT INTO account_audit(user_id,action,created_at,actor_id,detail) VALUES (?,?,?,?,?)", user, action, now.Unix(), actor, detail); e != nil {
		return e
	}
	_, e := tx.ExecContext(ctx, "DELETE FROM account_audit WHERE sequence <= (SELECT max(sequence)-2000 FROM account_audit)")
	return e
}

func (s *Store) RecordLogin(ctx context.Context, user, provider string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = auditAccountDetail(ctx, tx, user, user, "signed_in", provider, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RecordProviderTest(ctx context.Context, user, provider string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = auditAccountDetail(ctx, tx, user, user, "provider_tested", provider, now); err != nil {
		return err
	}
	return tx.Commit()
}

// AuthProvider describes an instance-level sign-in method. Secrets are never
// included; HasSecret only tells the settings UI that a write will be omitted.
type AuthProvider struct {
	Provider  string `json:"provider"`
	Enabled   bool   `json:"enabled"`
	ClientID  string `json:"clientId"`
	HasSecret bool   `json:"hasSecret"`
}

func authKey(provider, field string) string { return "auth_" + provider + "_" + field }

func (s *Store) PasswordLoginEnabled(ctx context.Context) (bool, error) {
	value, err := s.Setting(ctx, authKey("password", "enabled"), "true")
	return value == "true", err
}

func (s *Store) AuthProvider(ctx context.Context, provider string) (AuthProvider, error) {
	item := AuthProvider{Provider: provider}
	enabled, err := s.Setting(ctx, authKey(provider, "enabled"), "false")
	if err != nil {
		return item, err
	}
	item.Enabled = enabled == "true"
	if item.ClientID, err = s.Setting(ctx, authKey(provider, "client_id"), ""); err != nil {
		return item, err
	}
	secret, err := s.PrivateSetting(ctx, authKey(provider, "secret"), "")
	item.HasSecret = secret != ""
	return item, err
}

func (s *Store) AuthProviders(ctx context.Context) ([]AuthProvider, error) {
	items := make([]AuthProvider, 0, len(AuthProviders))
	for _, provider := range AuthProviders {
		item, err := s.AuthProvider(ctx, provider)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) AuthProviderSecret(ctx context.Context, provider string) (string, error) {
	return s.PrivateSetting(ctx, authKey(provider, "secret"), "")
}

func (s *Store) SaveAuthProvider(ctx context.Context, provider, clientID, secret string, enabled bool, actor string, now time.Time) error {
	if !supportedProvider(provider) {
		return errors.New("Choose a supported sign-in provider")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	sealed := ""
	if secret != "" {
		sealed, err = s.seal(secret, "setting:"+authKey(provider, "secret"))
		if err != nil {
			return err
		}
	}
	for key, value := range map[string]string{"enabled": fmt.Sprint(enabled), "client_id": clientID} {
		if _, err = tx.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", authKey(provider, key), value); err != nil {
			return err
		}
	}
	if sealed != "" {
		if _, err = tx.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", authKey(provider, "secret"), sealed); err != nil {
			return err
		}
	}
	action := "provider_updated"
	if enabled {
		action = "provider_enabled"
	}
	if err = auditAccountDetail(ctx, tx, actor, actor, action, provider, now); err != nil {
		return err
	}
	if err = ensureAdminRoute(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ClearAuthProvider(ctx context.Context, provider, actor string, now time.Time) error {
	if !supportedProvider(provider) {
		return errors.New("Choose a supported sign-in provider")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, field := range []string{"enabled", "client_id", "secret"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM settings WHERE key=?", authKey(provider, field)); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM user_identities WHERE provider=?", provider); err != nil {
		return err
	}
	if err = auditAccountDetail(ctx, tx, actor, actor, "provider_removed", provider, now); err != nil {
		return err
	}
	if err = ensureAdminRoute(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetPasswordLoginEnabled(ctx context.Context, enabled bool, actor string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", authKey("password", "enabled"), fmt.Sprint(enabled)); err != nil {
		return err
	}
	action := "password_login_disabled"
	if enabled {
		action = "password_login_enabled"
	}
	if err = auditAccountDetail(ctx, tx, actor, actor, action, "", now); err != nil {
		return err
	}
	if err = ensureAdminRoute(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func supportedProvider(provider string) bool {
	for _, value := range AuthProviders {
		if value == provider {
			return true
		}
	}
	return false
}

// ensureAdminRoute rejects a change that would leave no enabled administrator
// with a usable password or provider sign-in. It runs inside the mutation
// transaction so a rejected change never commits.
func ensureAdminRoute(tx *sql.Tx) error {
	ctx := context.Background()
	passwordEnabled := false
	if err := tx.QueryRowContext(ctx, "SELECT value='true' FROM settings WHERE key='auth_password_enabled'").Scan(&passwordEnabled); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	providers := map[string]bool{}
	rows, err := tx.QueryContext(ctx, "SELECT key FROM settings WHERE key LIKE 'auth_%_enabled' AND value='true'")
	if err != nil {
		return err
	}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return err
		}
		if strings.HasPrefix(key, "auth_password_") {
			continue
		}
		providers[strings.TrimSuffix(strings.TrimPrefix(key, "auth_"), "_enabled")] = true
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	admins, err := tx.QueryContext(ctx, "SELECT id,password_hash FROM administrators WHERE role='admin' AND disabled=0")
	if err != nil {
		return err
	}
	defer admins.Close()
	active := 0
	for admins.Next() {
		var id, hash string
		if err = admins.Scan(&id, &hash); err != nil {
			return err
		}
		if hash != "" && passwordEnabled {
			active++
			continue
		}
		var linked int
		for provider := range providers {
			if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_identities WHERE user_id=? AND provider=?", id, provider).Scan(&linked); err != nil {
				return err
			}
			if linked != 0 {
				active++
				break
			}
		}
	}
	if err = admins.Err(); err != nil {
		return err
	}
	if active == 0 {
		return ErrLastAdmin
	}
	return nil
}
