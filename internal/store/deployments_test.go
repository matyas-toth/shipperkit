package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
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

func TestFrequentTaskRunsNeverHideReleases(t *testing.T) {
	db, e := Open(filepath.Join(t.TempDir(), "data"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	a := Application{ID: "app-1", Name: "Site", Config: StaticConfig{Repository: "https://github.com/example/site.git", PublishDirectory: ".", Routing: "files"}}
	for i := 0; i < 3; i++ {
		d, e := db.Enqueue(ctx, a, Deployment{ID: fmt.Sprintf("release-%d", i), Operation: "deploy"}, "", i == 0)
		if e != nil {
			t.Fatal(e)
		}
		d.State = "succeeded"
		if e = db.SaveDeployment(ctx, d); e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 150; i++ {
		if _, e = db.db.ExecContext(ctx, "INSERT INTO deployments(id,application_id,operation,state,config_json,created_at,application_name) VALUES (?,?,?,?,?,?,?)", fmt.Sprintf("run-%03d", i), a.ID, "task", "succeeded", "{}", time.Now().Unix()+int64(i)+1, a.Name); e != nil {
			t.Fatal(e)
		}
	}
	items, e := db.Deployments(ctx, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	releases, runs := 0, 0
	for _, d := range items {
		if d.Operation == "task" {
			runs++
		} else {
			releases++
		}
	}
	if releases != 3 || runs != 10 || items[0].ID != "run-149" || items[len(items)-1].Operation == "task" {
		t.Fatalf("history kept %d releases and %d runs, first %s", releases, runs, items[0].ID)
	}
}
