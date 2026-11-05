package store

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func sourceReferences(db *sql.DB) ([]string, error) {
	rows, e := db.Query(`SELECT DISTINCT json_extract(config_json,'$.localSource') FROM (SELECT config_json FROM applications UNION ALL SELECT config_json FROM deployments) WHERE COALESCE(json_extract(config_json,'$.localSource'),'')!=''`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		if !sourceID.MatchString(id) {
			return nil, errors.New("Invalid source reference")
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (s *Store) BundleSources(ctx context.Context, snapshot, destination string) error {
	db, e := sql.Open("sqlite", snapshot)
	if e != nil {
		return e
	}
	defer db.Close()
	ids, e := sourceReferences(db)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	w := tar.NewWriter(f)
	defer w.Close()
	total := int64(0)
	for _, id := range ids {
		if e = ctx.Err(); e != nil {
			return e
		}
		file, _ := s.SourcePath(id)
		input, e := os.Open(file)
		if e != nil {
			return errors.New("A referenced local source is missing")
		}
		info, e := input.Stat()
		if e != nil {
			input.Close()
			return e
		}
		total += info.Size()
		if total > 1<<30 || !info.Mode().IsRegular() {
			input.Close()
			return errors.New("Local sources exceed the recovery limit")
		}
		if e = w.WriteHeader(&tar.Header{Name: id + ".tar", Size: info.Size(), Mode: 0600}); e != nil {
			input.Close()
			return e
		}
		_, e = io.CopyN(w, input, info.Size())
		input.Close()
		if e != nil {
			return e
		}
	}
	return w.Close()
}
func RestoreSourceBundle(stage string) error {
	file := filepath.Join(stage, "sources.tar")
	input, e := os.Open(file)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	defer input.Close()
	root := filepath.Join(stage, "sources")
	if e = os.MkdirAll(root, 0700); e != nil {
		return e
	}
	reader := tar.NewReader(input)
	count := 0
	total := int64(0)
	for {
		h, e := reader.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		id := strings.TrimSuffix(h.Name, ".tar")
		count++
		total += h.Size
		if !sourceID.MatchString(id) || h.Name != id+".tar" || h.Typeflag != tar.TypeReg || count > 10000 || h.Size < 0 || h.Size > 70<<20 || total > 1<<30 {
			return errors.New("Invalid source recovery bundle")
		}
		out, e := os.OpenFile(filepath.Join(root, h.Name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = io.CopyN(out, reader, h.Size)
		out.Close()
		if e != nil {
			return e
		}
	}
	input.Close()
	return os.Remove(file)
}
func (s *Store) VerifyRecoveredSources(ctx context.Context) error {
	ids, e := sourceReferences(s.db)
	if e != nil {
		return e
	}
	for _, id := range ids {
		v, e := s.SourceSnapshot(ctx, id)
		if e != nil {
			return e
		}
		file, _ := s.SourcePath(id)
		f, e := os.Open(file)
		if e != nil {
			return errors.New("Recovery is missing a local source")
		}
		h := sha256.New()
		_, e = io.Copy(h, io.LimitReader(f, 70<<20))
		f.Close()
		if e != nil {
			return e
		}
		if hex.EncodeToString(h.Sum(nil)) != v.Digest {
			return errors.New("Recovered local source integrity check failed")
		}
	}
	// Notification credentials survive recovery, but old alerts must not be sent
	// from a newly restored host until the operator enables delivery deliberately.
	raw, e := s.PrivateSetting(ctx, "notifications", `{}`)
	if e != nil {
		return e
	}
	var config map[string]any
	if jsonErr := json.Unmarshal([]byte(raw), &config); jsonErr != nil {
		return jsonErr
	}
	if config == nil {
		config = map[string]any{}
	}
	if config["kind"] == nil {
		config["kind"] = "webhook"
	}
	if config["events"] == nil {
		config["events"] = []string{"deployment", "service", "backup", "storage", "certificate", "daemon"}
	}
	config["enabled"] = false
	value, _ := json.Marshal(config)
	if e = s.SavePrivateSetting(ctx, "notifications", string(value)); e != nil {
		return e
	}
	return s.SavePrivateSetting(ctx, "automatic_retention", "false")
}
