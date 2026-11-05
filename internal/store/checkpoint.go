package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var checkpointID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,63}$`)

func (s *Store) UpdateReady(ctx context.Context) error {
	var busy int
	e := s.db.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM deployments WHERE state NOT IN ('succeeded','failed','cancelled','interrupted'))+
		(SELECT count(*) FROM backup_runs WHERE state IN ('queued','running'))+
		(SELECT count(*) FROM maintenance_runs WHERE state IN ('queued','running'))`).Scan(&busy)
	if e == nil && busy != 0 {
		return errors.New("Wait for queued operations before updating")
	}
	return e
}

func (s *Store) WriteCheckpoint(ctx context.Context, id string) error {
	if !checkpointID.MatchString(id) {
		return errors.New("Invalid checkpoint identity")
	}
	var workers int
	if e := s.db.QueryRowContext(ctx, "SELECT count(*) FROM worker_lease WHERE expires_at>?", time.Now().Unix()).Scan(&workers); e != nil {
		return e
	}
	if workers != 0 {
		return errors.New("Stop the control plane before taking an update checkpoint")
	}
	if e := s.UpdateReady(ctx); e != nil {
		return e
	}
	dir := filepath.Join(filepath.Dir(s.file), "checkpoints", id)
	if e := os.MkdirAll(filepath.Dir(dir), 0700); e != nil {
		return e
	}
	if e := os.Mkdir(dir, 0700); e != nil {
		return e
	}
	snapshot := filepath.Join(dir, "shipper.db")
	if e := s.Snapshot(ctx, snapshot); e != nil {
		return e
	}
	if e := verifyCheckpoint(ctx, snapshot); e != nil {
		return e
	}
	digest, e := fileHash(snapshot)
	if e != nil {
		return e
	}
	key, e := fileHash(filepath.Join(filepath.Dir(s.file), "encryption.key"))
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "verified"), []byte(digest+"\n"+key+"\n"), 0600)
}

func verifyCheckpoint(ctx context.Context, path string) error {
	file := filepath.ToSlash(path)
	if !strings.HasPrefix(file, "/") {
		file = "/" + file
	}
	db, e := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: file, RawQuery: "mode=ro&immutable=1"}).String())
	if e != nil {
		return e
	}
	defer db.Close()
	var result string
	if e = db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); e != nil {
		return e
	}
	if result != "ok" {
		return errors.New("SQLite checkpoint did not pass integrity verification")
	}
	return nil
}

func fileHash(path string) (string, error) {
	file, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer file.Close()
	h := sha256.New()
	if _, e = io.Copy(h, file); e != nil {
		return "", e
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// RestoreCheckpoint replaces only the SQLite metadata after an offline update.
// Keep the candidate database and sidecars together for recovery/inspection.
func RestoreCheckpoint(ctx context.Context, root, id string) error {
	if !checkpointID.MatchString(id) {
		return errors.New("Invalid checkpoint identity")
	}
	dir := filepath.Join(root, "checkpoints", id)
	source := filepath.Join(dir, "shipper.db")
	signed, e := os.ReadFile(filepath.Join(dir, "verified"))
	if e != nil {
		return e
	}
	fields := strings.Fields(string(signed))
	digest, e := fileHash(source)
	if e != nil || len(fields) != 2 || digest != fields[0] {
		return errors.New("Checkpoint checksum failed")
	}
	key, e := fileHash(filepath.Join(root, "encryption.key"))
	if e != nil || key != fields[1] {
		return errors.New("Checkpoint belongs to a different encryption key")
	}
	if e = verifyCheckpoint(ctx, source); e != nil {
		return e
	}
	candidate, e := os.MkdirTemp(dir, "candidate-")
	if e != nil {
		return e
	}
	next, e := os.CreateTemp(root, "restore-")
	if e != nil {
		return e
	}
	defer os.Remove(next.Name())
	input, e := os.Open(source)
	if e != nil {
		next.Close()
		return e
	}
	_, copyErr := io.Copy(next, input)
	input.Close()
	e = errors.Join(copyErr, next.Sync(), next.Close())
	if e != nil {
		return e
	}
	moved := []string{}
	rollback := func() {
		for i := len(moved) - 1; i >= 0; i-- {
			_ = os.Rename(filepath.Join(candidate, moved[i]), filepath.Join(root, moved[i]))
		}
	}
	for _, file := range []string{"shipper.db", "shipper.db-wal", "shipper.db-shm"} {
		e = os.Rename(filepath.Join(root, file), filepath.Join(candidate, file))
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			rollback()
			return e
		}
		moved = append(moved, file)
	}
	if e = os.Rename(next.Name(), filepath.Join(root, "shipper.db")); e != nil {
		rollback()
	}
	return e
}
