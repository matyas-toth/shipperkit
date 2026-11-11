package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPendingRestoreReplacesTheInstance(t *testing.T) {
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "shipper.db"), []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "setup-token"), []byte("token"), 0600); e != nil {
		t.Fatal(e)
	}
	stage := filepath.Join(dir, ".restore-test")
	instance := filepath.Join(stage, "instance")
	if e := os.MkdirAll(instance, 0700); e != nil {
		t.Fatal(e)
	}
	for name, value := range map[string]string{"shipper.db": "restored", "encryption.key": "key", "sources.tar": "sources"} {
		if e := os.WriteFile(filepath.Join(instance, name), []byte(value), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := WritePendingRestore(dir, stage); e != nil {
		t.Fatal(e)
	}
	if e := ApplyPendingRestore(dir); e != nil {
		t.Fatal(e)
	}
	if raw, e := os.ReadFile(filepath.Join(dir, "shipper.db")); e != nil || string(raw) != "restored" {
		t.Fatalf("restored database = %q, %v", raw, e)
	}
	if raw, e := os.ReadFile(filepath.Join(dir, "encryption.key")); e != nil || string(raw) != "key" {
		t.Fatalf("restored key = %q, %v", raw, e)
	}
	if _, e := os.Stat(filepath.Join(dir, "setup-token")); !os.IsNotExist(e) {
		t.Fatal("the setup token survived the restore")
	}
	for _, name := range []string{"restore-pending.json", ".restore-test"} {
		if _, e := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(e) {
			t.Fatalf("%s survived the restore", name)
		}
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".pre-restore-") {
			t.Fatalf("backup %s survived the restore", entry.Name())
		}
	}
}

func TestApplyPendingRestoreWithoutRequestDoesNothing(t *testing.T) {
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "shipper.db"), []byte("live"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := ApplyPendingRestore(dir); e != nil {
		t.Fatal(e)
	}
	if raw, e := os.ReadFile(filepath.Join(dir, "shipper.db")); e != nil || string(raw) != "live" {
		t.Fatalf("database changed to %q, %v", raw, e)
	}
}
