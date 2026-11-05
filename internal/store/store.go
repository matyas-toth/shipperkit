// Package store owns Shipper's metadata and observed Docker inventory.
package store

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrAlreadyConfigured = errors.New("administrator already configured")

type Store struct {
	db         *sql.DB
	file       string
	key        cipher.AEAD
	signingKey []byte
}

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
	s := &Store{db: db, file: file}
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
	keyFile := filepath.Join(dir, "encryption.key")
	_, keyError := os.Stat(keyFile)
	if os.IsNotExist(keyError) {
		var encrypted int
		_ = db.QueryRow("SELECT (SELECT COUNT(*) FROM applications WHERE instr(config_json,'sealed:v1:')>0)+(SELECT COUNT(*) FROM deployments WHERE instr(config_json,'sealed:v1:')>0 OR instr(compose_plan,'sealed:v1:')>0)+(SELECT count(*) FROM tasks)+(SELECT count(*) FROM task_runs)+(SELECT count(*) FROM connections)+(SELECT count(*) FROM backup_plans)+(SELECT count(*) FROM backup_runs)+(SELECT count(*) FROM backup_destinations)+(SELECT count(*) FROM settings WHERE instr(value,'sealed:v1:')>0)+(SELECT count(*) FROM account_factors)+(SELECT count(*) FROM factor_enrollments)").Scan(&encrypted)
		if encrypted > 0 {
			db.Close()
			return nil, errors.New("Restore encryption.key before opening encrypted application configuration")
		}
	}
	s.key, err = configKey(dir)
	if err != nil {
		db.Close()
		return nil, err
	}
	s.signingKey, err = os.ReadFile(keyFile)
	if err != nil {
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
	if _, err = tx.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES ('acme_contact',?) ON CONFLICT(key) DO NOTHING", user.Email); err != nil {
		return err
	}
	if hash != "" {
		if _, err = tx.ExecContext(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES (?,?,?,?)", hash, user.ID, csrf, expiry); err != nil {
			return err
		}
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
	if _, err = tx.ExecContext(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES (?,?,?,?)", hash, userID, csrf, expiry); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Session(ctx context.Context, hash string) (Session, error) {
	var v Session
	err := s.db.QueryRowContext(ctx, `SELECT a.id,a.email,s.csrf_token,s.expires_at,s.factor_verified,
		EXISTS(SELECT 1 FROM account_factors f WHERE f.user_id=a.id),
		EXISTS(SELECT 1 FROM settings WHERE key='require_factor' AND value='true')
		FROM sessions s JOIN administrators a ON a.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?`, hash, time.Now().Unix()).Scan(&v.User.ID, &v.User.Email, &v.CSRF, &v.ExpiresAt, &v.Verified, &v.Factor.Enrolled, &v.Factor.Required)
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
		if _, err = tx.ExecContext(ctx, `INSERT INTO containers VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.ID, c.Name, c.Image, c.State, c.Status, c.Health, c.CreatedAt, string(ports), c.Managed, c.ApplicationID, c.Project, c.Service, c.DeploymentID); err != nil {
			return err
		}
		if c.Managed && c.ApplicationID != "" {
			owned[c.ApplicationID] = append(owned[c.ApplicationID], c)
		}
	}
	for id, containers := range owned {
		var active string
		var deleted int64
		var serviceJSON string
		var expected []Service
		if e := tx.QueryRowContext(ctx, "SELECT active_deployment_id,deleted_at,COALESCE((SELECT services_json FROM deployments WHERE id=applications.active_deployment_id),'[]') FROM applications WHERE id=?", id).Scan(&active, &deleted, &serviceJSON); e == nil {
			if e = json.Unmarshal([]byte(serviceJSON), &expected); e != nil {
				return e
			}
			if deleted != 0 {
				continue
			}
			if active != "" {
				filtered := []Container{}
				for _, c := range containers {
					if c.DeploymentID == active || (c.DeploymentID == "" && c.Project != "") {
						filtered = append(filtered, c)
					}
				}
				containers = filtered
			}
		}
		if len(containers) == 0 {
			continue
		}
		sort.Slice(containers, func(i, j int) bool { return containers[i].Name < containers[j].Name })
		name := containers[0].Name
		if containers[0].Project != "" {
			name = containers[0].Project
		}
		state := aggregate(containers)
		if len(expected) > 0 {
			runtime := []Container{}
			missing := false
			for _, svc := range expected {
				found := false
				for _, c := range containers {
					if c.Service == svc.Name {
						found = true
						if svc.OneShot && c.State == "exited" && strings.HasPrefix(c.Status, "Exited (0)") {
							continue
						}
						runtime = append(runtime, c)
					}
				}
				if !found {
					missing = true
				}
			}
			if len(runtime) > 0 {
				state = aggregate(runtime)
			}
			if missing {
				state = "degraded"
			}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO applications(id,name,state,created_at,updated_at) VALUES (?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET state=excluded.state,updated_at=excluded.updated_at`, id, name, state, now, now)
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
	return s.containers(ctx, "")
}
func (s *Store) containers(ctx context.Context, appID string) ([]Container, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,image,state,status,health,created_at,ports_json,managed,application_id,project,service,deployment_id FROM containers WHERE (?='' OR application_id=?) ORDER BY name COLLATE NOCASE,id", appID, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Container{}
	for rows.Next() {
		var c Container
		var ports string
		if err = rows.Scan(&c.ID, &c.Name, &c.Image, &c.State, &c.Status, &c.Health, &c.CreatedAt, &ports, &c.Managed, &c.ApplicationID, &c.Project, &c.Service, &c.DeploymentID); err != nil {
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
	return s.applications(ctx, "")
}
func (s *Store) applications(ctx context.Context, appID string) ([]Application, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,source_kind,source_ref,method,state,created_at,updated_at,config_json,active_deployment_id,domains_json,desired_state,COALESCE((SELECT COALESCE(NULLIF(d.resolved_method,''),NULLIF(json_extract(d.config_json,'$.method'),''),'static') FROM deployments d WHERE d.id=applications.active_deployment_id),'') FROM applications WHERE deleted_at=0 AND (?='' OR id=?) ORDER BY name COLLATE NOCASE,id", appID, appID)
	if err != nil {
		return nil, err
	}
	apps := []Application{}
	for rows.Next() {
		var a Application
		var config, domains string
		a.Containers = []Container{}
		if err = rows.Scan(&a.ID, &a.Name, &a.SourceKind, &a.SourceRef, &a.Method, &a.State, &a.CreatedAt, &a.UpdatedAt, &config, &a.ActiveDeploymentID, &domains, &a.DesiredState, &a.ActiveMethod); err != nil {
			rows.Close()
			return nil, err
		}
		if err = s.unmarshalConfig(config, &a.Config); err != nil {
			rows.Close()
			return nil, err
		}
		if err = json.Unmarshal([]byte(domains), &a.Domains); err != nil {
			rows.Close()
			return nil, err
		}
		if a.Domains == nil {
			a.Domains = []Domain{}
		}
		apps = append(apps, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	containers, err := s.containers(ctx, appID)
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
	latest, err := s.db.QueryContext(ctx, "SELECT d."+strings.ReplaceAll(deploymentColumns, ",", ",d.")+" FROM deployments d WHERE d.rowid IN (SELECT MAX(rowid) FROM deployments WHERE (?='' OR application_id=?) GROUP BY application_id)", appID, appID)
	if err != nil {
		return nil, err
	}
	defer latest.Close()
	for latest.Next() {
		d, e := s.scanDeployment(latest)
		if e != nil {
			return nil, e
		}
		if i, ok := index[d.ApplicationID]; ok {
			apps[i].LatestDeployment = &d
		}
	}
	if err = latest.Err(); err != nil {
		return nil, err
	}
	return apps, nil
}

func (s *Store) Application(ctx context.Context, id string) (Application, error) {
	apps, err := s.applications(ctx, id)
	if err != nil {
		return Application{}, err
	}
	for _, a := range apps {
		if a.ID == id {
			return a, nil
		}
	}
	return Application{}, sql.ErrNoRows
}

func (s *Store) Counts(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	for _, table := range []string{"applications", "containers", "sessions"} {
		var n int
		query := "SELECT COUNT(*) FROM " + table
		if table == "applications" {
			query += " WHERE deleted_at=0"
		}
		if table == "containers" {
			query += " WHERE managed=1"
		}
		if err := s.db.QueryRowContext(ctx, query).Scan(&n); err != nil {
			return nil, err
		}
		out[table] = n
	}
	return out, nil
}
