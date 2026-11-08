package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTerminalLimitExpiryAndMetadataAudit(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err = db.CreateAdmin(ctx, User{ID: "owner", Email: "owner@test.invalid", PasswordHash: "unused"}, "hash", "csrf", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	record := func(id string) TerminalSession {
		return TerminalSession{ID: id, UserID: "owner", AuthHash: "hash", ApplicationID: "app", ContainerID: "owned", ExpiresAt: time.Now().Add(time.Minute).Unix()}
	}
	if err = db.CreateTerminal(ctx, record("first")); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateTerminal(ctx, record("second")); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateTerminal(ctx, record("third")); !errors.Is(err, ErrTerminalLimit) {
		t.Fatalf("unbounded session creation: %v", err)
	}
	if err = db.EndTerminal(ctx, "first", "revoke", "user_revoked"); err != nil {
		t.Fatal(err)
	}
	if err = db.EndTerminal(ctx, "first", "end", "disconnected"); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateTerminal(ctx, record("third")); err != nil {
		t.Fatal(err)
	}
	if _, err = db.db.ExecContext(ctx, `UPDATE terminal_sessions SET expires_at=? WHERE id='second'`, time.Now().Add(-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateTerminal(ctx, record("fourth")); err != nil {
		t.Fatal("expired session retained slot", err)
	}
	audit, err := db.TerminalAudit(ctx, "app")
	if err != nil {
		t.Fatal(err)
	}
	if len(audit) != 5 {
		t.Fatalf("expected starts and one idempotent revoke, got %#v", audit)
	}
	first, err := db.Terminal(ctx, "first")
	if err != nil || first.EndedAt == 0 {
		t.Fatal("revoked session remains active", err)
	}
}
