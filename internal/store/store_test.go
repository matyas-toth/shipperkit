package store

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAdminSetupIsAtomicAndSessionsSurviveRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- db.CreateAdmin(ctx, User{ID: "owner", Email: "owner@example.test", PasswordHash: "test-only"}, "session-hash", "csrf", time.Now().Add(time.Hour).Unix())
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrAlreadyConfigured) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("setup results: %d successes, %d conflicts", success, conflict)
	}
	db.Close()
	db, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	session, err := db.Session(ctx, "session-hash")
	if err != nil || session.User.Email != "owner@example.test" {
		t.Fatalf("session lost after restart: %+v %v", session, err)
	}
	if err = db.ResetPassword(ctx, "owner@example.test", "new-test-only"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Session(ctx, "session-hash"); err == nil {
		t.Fatal("password recovery did not revoke sessions")
	}
}

func TestSnapshotReplacementPreservesApplicationIdentity(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	items := []Container{
		{ID: "a", Name: "api", Image: "api:v1", State: "running", Managed: true, ApplicationID: "app-1", Ports: []Port{}},
		{ID: "b", Name: "external", Image: "db:v1", State: "running", Ports: []Port{}},
	}
	if err = db.ReplaceInventory(ctx, items); err != nil {
		t.Fatal(err)
	}
	apps, err := db.Applications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].ID != "app-1" || len(apps[0].Containers) != 1 || apps[0].SourceKind != "" || apps[0].Method != "" {
		t.Fatalf("ownership/source model: %+v", apps)
	}
	if err = db.ReplaceInventory(ctx, nil); err != nil {
		t.Fatal(err)
	}
	containers, _ := db.Containers(ctx)
	apps, _ = db.Applications(ctx)
	if len(containers) != 0 || len(apps) != 1 || apps[0].State != "missing" {
		t.Fatalf("empty snapshot lost identity: %+v", apps)
	}
}

func TestAggregateDoesNotCallUnhealthyContainersHealthy(t *testing.T) {
	for _, tc := range []struct {
		items []Container
		want  string
	}{
		{[]Container{{State: "running", Health: "unhealthy"}}, "degraded"},
		{[]Container{{State: "running"}, {State: "exited"}}, "degraded"},
		{[]Container{{State: "running"}, {State: "running"}}, "running"},
		{[]Container{{State: "running", Health: "starting"}}, "starting"},
		{[]Container{{State: "restarting"}}, "degraded"},
		{[]Container{{State: "paused"}}, "paused"},
	} {
		if got := aggregate(tc.items); got != tc.want {
			t.Fatalf("got %s want %s", got, tc.want)
		}
	}
}
