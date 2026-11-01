package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestQueueIsDurableExclusiveAndDomainScoped(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	db, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	a := Application{ID: "app-1", Name: "Site", Config: StaticConfig{Repository: "https://github.com/example/site.git", Domain: "site.localhost", PublishDirectory: ".", Routing: "files"}}
	first, e := db.Enqueue(ctx, a, Deployment{ID: "job-1", Operation: "deploy"}, "request-1", true)
	if e != nil {
		t.Fatal(e)
	}
	same, e := db.Enqueue(ctx, a, Deployment{ID: "other", Operation: "deploy"}, "request-1", true)
	if e != nil || same.ID != first.ID {
		t.Fatal("idempotency failed", same, e)
	}
	changed := a
	changed.Name = "Different name"
	if _, e = db.Enqueue(ctx, changed, Deployment{ID: "different", Operation: "deploy"}, "request-1", true); !errors.Is(e, ErrIdempotencyConflict) {
		t.Fatal("idempotency key accepted a different payload", e)
	}
	if _, e = db.Enqueue(ctx, a, Deployment{ID: "job-2", Operation: "deploy"}, "", false); !errors.Is(e, ErrBusy) {
		t.Fatal("concurrent operation accepted", e)
	}
	b := a
	b.ID = "app-2"
	if _, e = db.Enqueue(ctx, b, Deployment{ID: "job-domain", Operation: "deploy"}, "", true); !errors.Is(e, ErrDomainInUse) {
		t.Fatal("domain claimed twice", e)
	}
	if ok, e := db.Lease(ctx, "worker-a"); e != nil || !ok {
		t.Fatal(e)
	}
	if ok, _ := db.Lease(ctx, "worker-b"); ok {
		t.Fatal("two workers acquired lease")
	}
	db.ReleaseLease(ctx, "worker-a")
	db.Close()
	db, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	pending, e := db.Pending(ctx)
	if e != nil || len(pending) != 1 || pending[0].ID != first.ID {
		t.Fatal("queue did not survive restart", pending, e)
	}
	first.State = "succeeded"
	db.SaveDeployment(ctx, first)
	db.SetActive(ctx, a.ID, first.ID)
	current, _ := db.Application(ctx, a.ID)
	next, e := db.Enqueue(ctx, current, Deployment{ID: "job-3", Operation: "deploy"}, "", false)
	if e != nil || next.PreviousID != first.ID {
		t.Fatal("previous release not captured", next, e)
	}
	db.MarkDeleted(ctx, a.ID)
	db.ReplaceInventory(ctx, []Container{{ID: "late-event", Name: "old-site", Managed: true, ApplicationID: a.ID, Ports: []Port{}}})
	if _, e = db.Application(ctx, a.ID); e == nil {
		t.Fatal("Docker event resurrected deleted application")
	}
}
