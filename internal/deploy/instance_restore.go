package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const restoreMarker = "restore-pending.json"

// WritePendingRestore records a validated instance restore for the next start.
func WritePendingRestore(dataDir, stage string) error {
	raw, e := json.Marshal(map[string]any{"stage": filepath.Base(stage), "createdAt": time.Now().Unix()})
	if e != nil {
		return e
	}
	tmp := filepath.Join(dataDir, restoreMarker+".tmp")
	if e = os.WriteFile(tmp, raw, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(dataDir, restoreMarker))
}

// ApplyPendingRestore installs a validated recovery archive before the store opens.
// It only runs during first-run setup, so the current directory holds no workloads.
func ApplyPendingRestore(dataDir string) error {
	marker := filepath.Join(dataDir, restoreMarker)
	raw, e := os.ReadFile(marker)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	var request struct {
		Stage string `json:"stage"`
	}
	if e = json.Unmarshal(raw, &request); e != nil {
		_ = os.Remove(marker)
		return fmt.Errorf("restore request is invalid: %w", e)
	}
	stageName := filepath.Base(request.Stage)
	if stageName != request.Stage || !strings.HasPrefix(stageName, ".restore-") {
		_ = os.Remove(marker)
		return errors.New("restore request has an invalid stage")
	}
	stage := filepath.Join(dataDir, stageName)
	instance := filepath.Join(stage, "instance")
	for _, name := range []string{"shipper.db", "encryption.key"} {
		info, e := os.Lstat(filepath.Join(instance, name))
		if e != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			_ = os.RemoveAll(stage)
			_ = os.Remove(marker)
			return fmt.Errorf("restore is missing %s", name)
		}
	}
	backup := filepath.Join(dataDir, ".pre-restore-"+time.Now().UTC().Format("20060102T150405.000000000"))
	if e = os.Mkdir(backup, 0700); e != nil {
		return e
	}
	rollback := func(moved, installed []string) {
		for _, name := range installed {
			_ = os.Remove(filepath.Join(dataDir, name))
		}
		for _, name := range moved {
			_ = os.Rename(filepath.Join(backup, name), filepath.Join(dataDir, name))
		}
		_ = os.RemoveAll(backup)
	}
	entries, e := os.ReadDir(dataDir)
	if e != nil {
		rollback(nil, nil)
		return e
	}
	moved := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if name == stageName || name == restoreMarker || name == filepath.Base(backup) {
			continue
		}
		if e = os.Rename(filepath.Join(dataDir, name), filepath.Join(backup, name)); e != nil {
			rollback(moved, nil)
			return fmt.Errorf("prepare restore: %w", e)
		}
		moved = append(moved, name)
	}
	install, e := os.ReadDir(instance)
	if e != nil {
		rollback(moved, nil)
		_ = os.RemoveAll(stage)
		_ = os.Remove(marker)
		return e
	}
	installed := []string{}
	for _, entry := range install {
		name := entry.Name()
		if e = os.Rename(filepath.Join(instance, name), filepath.Join(dataDir, name)); e != nil {
			rollback(moved, installed)
			_ = os.RemoveAll(stage)
			_ = os.Remove(marker)
			return fmt.Errorf("install restore: %w", e)
		}
		installed = append(installed, name)
	}
	_ = os.Remove(marker)
	_ = os.RemoveAll(stage)
	_ = os.RemoveAll(backup)
	return nil
}
