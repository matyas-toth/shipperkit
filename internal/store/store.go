// Package store owns Shipper's metadata and observed Docker inventory.
package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrAlreadyConfigured = errors.New("administrator already configured")

type Store struct{ db *sql.DB }

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	file := filepath.Join(dir, "shipper.db")
	handle, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := handle.Close(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db}
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000", "PRAGMA synchronous=NORMAL"} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)"); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var count int
		if err = tx.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version=?", entry.Name()).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			continue
		}
		body, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if _, err = tx.Exec(string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if _, err = tx.Exec("INSERT INTO schema_migrations VALUES (?,?)", entry.Name(), time.Now().Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Configured(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM administrators").Scan(&count)
	return count > 0, err
}

func (s *Store) CreateAdmin(ctx context.Context, user User, hash, csrf string, expiry int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM administrators").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrAlreadyConfigured
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO administrators(id,email,password_hash,created_at) VALUES (?,?,?,?)", user.ID, user.Email, user.PasswordHash, time.Now().Unix())
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO sessions VALUES (?,?,?,?)", hash, user.ID, csrf, expiry); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) FindUser(ctx context.Context, email string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, "SELECT id,email,password_hash FROM administrators WHERE email=?", email).Scan(&u.ID, &u.Email, &u.PasswordHash)
	return u, err
}

func (s *Store) CreateSession(ctx context.Context, hash, userID, csrf string, expiry int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", time.Now().Unix()); err != nil {
		return err
	}
	// Bound stored sessions even when the owner repeatedly signs in.
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash IN (SELECT token_hash FROM sessions WHERE user_id=? ORDER BY expires_at DESC LIMIT -1 OFFSET 19)", userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO sessions VALUES (?,?,?,?)", hash, userID, csrf, expiry); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Session(ctx context.Context, hash string) (Session, error) {
	var v Session
	err := s.db.QueryRowContext(ctx, `SELECT a.id,a.email,s.csrf_token,s.expires_at FROM sessions s JOIN administrators a ON a.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?`, hash, time.Now().Unix()).Scan(&v.User.ID, &v.User.Email, &v.CSRF, &v.ExpiresAt)
	return v, err
}

func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash=?", hash)
	return err
}

func (s *Store) ResetPassword(ctx context.Context, email, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE administrators SET password_hash=? WHERE email=?", passwordHash, email)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions"); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceInventory is all-or-nothing; a failed snapshot never replaces good data.
func (s *Store) ReplaceInventory(ctx context.Context, items []Container) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM containers"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE applications SET state='missing'"); err != nil {
		return err
	}
	now := time.Now().Unix()
	owned := map[string][]Container{}
	for _, c := range items {
		ports, err := json.Marshal(c.Ports)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO containers VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, c.ID, c.Name, c.Image, c.State, c.Status, c.Health, c.CreatedAt, string(ports), c.Managed, c.ApplicationID, c.Project, c.Service); err != nil {
			return err
		}
		if c.Managed && c.ApplicationID != "" {
			owned[c.ApplicationID] = append(owned[c.ApplicationID], c)
		}
	}
	for id, containers := range owned {
		sort.Slice(containers, func(i, j int) bool { return containers[i].Name < containers[j].Name })
		name := containers[0].Name
		if containers[0].Project != "" {
			name = containers[0].Project
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO applications(id,name,state,created_at,updated_at) VALUES (?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET state=excluded.state,updated_at=excluded.updated_at`, id, name, aggregate(containers), now, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func aggregate(items []Container) string {
	running := 0
	starting := false
	for _, c := range items {
		if c.Health == "unhealthy" || c.State == "dead" || c.State == "restarting" {
			return "degraded"
		}
		if c.State == "running" {
			running++
		}
		if c.Health == "starting" {
			starting = true
		}
	}
	if running == len(items) {
		if starting {
			return "starting"
		}
		return "running"
	}
	if running > 0 {
		return "degraded"
	}
	for _, c := range items {
		if c.State == "paused" {
			return "paused"
		}
		if c.State == "created" {
			return "created"
		}
	}
	return "exited"
}

func (s *Store) Containers(ctx context.Context) ([]Container, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,image,state,status,health,created_at,ports_json,managed,application_id,project,service FROM containers ORDER BY name COLLATE NOCASE,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Container{}
	for rows.Next() {
		var c Container
		var ports string
		if err = rows.Scan(&c.ID, &c.Name, &c.Image, &c.State, &c.Status, &c.Health, &c.CreatedAt, &ports, &c.Managed, &c.ApplicationID, &c.Project, &c.Service); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(ports), &c.Ports); err != nil {
			return nil, err
		}
		if c.Ports == nil {
			c.Ports = []Port{}
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) Applications(ctx context.Context) ([]Application, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,source_kind,source_ref,method,state,created_at,updated_at FROM applications ORDER BY name COLLATE NOCASE,id")
	if err != nil {
		return nil, err
	}
	apps := []Application{}
	for rows.Next() {
		var a Application
		a.Containers = []Container{}
		if err = rows.Scan(&a.ID, &a.Name, &a.SourceKind, &a.SourceRef, &a.Method, &a.State, &a.CreatedAt, &a.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		apps = append(apps, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	containers, err := s.Containers(ctx)
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	for i, a := range apps {
		index[a.ID] = i
	}
	for _, c := range containers {
		if i, ok := index[c.ApplicationID]; ok && c.Managed {
			apps[i].Containers = append(apps[i].Containers, c)
		}
	}
	return apps, nil
}

func (s *Store) Counts(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	for _, table := range []string{"applications", "containers", "sessions"} {
		var n int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
			return nil, err
		}
		out[table] = n
	}
	return out, nil
}
