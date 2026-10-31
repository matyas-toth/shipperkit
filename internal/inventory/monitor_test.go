package inventory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/store"
)

type fakeSource struct {
	mu      sync.Mutex
	items   []store.Container
	err     error
	changes chan struct{}
}

func (f *fakeSource) Snapshot(context.Context) (engine.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return engine.Snapshot{Containers: append([]store.Container{}, f.items...)}, f.err
}
func (f *fakeSource) Subscribe(ctx context.Context) (<-chan struct{}, <-chan error) {
	return f.changes, make(chan error)
}
func TestFailedReconciliationRetainsLastCompleteSnapshot(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	source := &fakeSource{items: []store.Container{{ID: "a", Name: "api", State: "running", Ports: []store.Port{}}}}
	monitor := New(source, db, events.New())
	ctx := context.Background()
	if err = monitor.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	observed := monitor.Health().ObservedAt
	source.err = errors.New("socket unavailable")
	if err = monitor.Reconcile(ctx); err == nil {
		t.Fatal("expected reconciliation failure")
	}
	items, _ := db.Containers(ctx)
	if len(items) != 1 || monitor.Health().Connected || monitor.Health().ObservedAt != observed {
		t.Fatal("failure replaced the cached snapshot or hid stale state")
	}
	source.err = nil
	source.items = nil
	if err = monitor.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	items, _ = db.Containers(ctx)
	if len(items) != 0 || !monitor.Health().Connected {
		t.Fatal("successful empty snapshot was not applied")
	}
}
func TestDockerEventsTriggerReconciliation(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	source := &fakeSource{items: []store.Container{{ID: "a", Name: "api", State: "running", Ports: []store.Port{}}}, changes: make(chan struct{}, 1)}
	monitor := New(source, db, events.New())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); monitor.Run(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(3 * time.Second)
	for !monitor.Health().Connected && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	source.mu.Lock()
	source.items[0].State = "exited"
	source.mu.Unlock()
	source.changes <- struct{}{}
	for time.Now().Before(deadline) {
		items, _ := db.Containers(context.Background())
		if len(items) == 1 && items[0].State == "exited" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("event-driven state did not reach SQLite")
}
