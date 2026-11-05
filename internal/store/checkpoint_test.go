package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestOfflineUpdateCheckpointRestoresOriginalMetadata(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Setting(ctx, "original", "kept"); e != nil {
		t.Fatal(e)
	}
	if ok, err := db.Lease(ctx, "worker"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if e = db.WriteCheckpoint(ctx, "before-update"); e == nil {
		t.Fatal("Checkpoint accepted while worker was active")
	}
	db.ReleaseLease(ctx, "worker")
	if e = db.WriteCheckpoint(ctx, "before-update"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Setting(ctx, "candidate", "not committed"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	if e = RestoreCheckpoint(ctx, dir, "before-update"); e != nil {
		t.Fatal(e)
	}
	db, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var value string
	if e = db.db.QueryRow("SELECT value FROM settings WHERE key='candidate'").Scan(&value); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("Candidate state survived recovery", e)
	}
	if e = db.db.QueryRow("SELECT value FROM settings WHERE key='original'").Scan(&value); e != nil || value != "kept" {
		t.Fatal("Original state was not restored", e)
	}
}
