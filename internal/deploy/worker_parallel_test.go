package deploy

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"shipper/internal/events"
	"shipper/internal/store"
)

// parallelRuntime holds one application in packaging until released.
type parallelRuntime struct {
	mu      sync.Mutex
	slowApp string
	release chan struct{}
	routing int
	overlap bool
}

func (p *parallelRuntime) Ensure(context.Context) error { return nil }
func (p *parallelRuntime) Package(ctx context.Context, d store.Deployment, _ []byte, _ func(string)) (string, error) {
	if d.ApplicationID == p.slowApp {
		select {
		case <-p.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "sha256:fixture", nil
}
func (p *parallelRuntime) Start(_ context.Context, d store.Deployment) (string, error) {
	return "container-" + d.ID, nil
}
func (p *parallelRuntime) Healthy(context.Context, store.Deployment) error { return nil }
func (p *parallelRuntime) Routes(context.Context, []store.Application, map[string]store.Deployment) error {
	p.mu.Lock()
	p.routing++
	p.overlap = p.overlap || p.routing > 1
	p.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	p.mu.Lock()
	p.routing--
	p.mu.Unlock()
	return nil
}
func (p *parallelRuntime) RemoveDeployment(context.Context, store.Deployment) error { return nil }
func (p *parallelRuntime) RemoveApplication(context.Context, string) error          { return nil }
func (p *parallelRuntime) ProbeLocal(context.Context, store.Deployment) (bool, error) {
	return true, nil
}

func TestSlowDeploymentDoesNotBlockOtherApplications(t *testing.T) {
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	runtime := &parallelRuntime{slowApp: "slow", release: make(chan struct{})}
	w := NewWorker(db, runtime, events.New(), dir)
	w.Source = fixtureSource{commit: strings.Repeat("a", 40)}

	enqueue := func(app, id string) {
		t.Helper()
		c := testConfig()
		c.Domain = app + ".localhost"
		if _, e := db.Enqueue(ctx, store.Application{ID: app, Name: app, Config: c}, store.Deployment{ID: id, Operation: "deploy"}, "", true); e != nil {
			t.Fatal(e)
		}
	}
	state := func(id string) string {
		d, e := db.Deployment(context.Background(), id)
		if e != nil {
			t.Fatal(e)
		}
		return d.State
	}
	waitFor := func(id, want string) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for state(id) != want {
			if time.Now().After(deadline) {
				t.Fatalf("%s stayed %s, want %s", id, state(id), want)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}

	enqueue("slow", "slow-1")
	enqueue("fast", "fast-1")
	enqueue("other", "other-1")
	stopped := make(chan struct{})
	go func() { defer close(stopped); w.Run(ctx) }()

	waitFor("fast-1", "succeeded")
	waitFor("other-1", "succeeded")
	if s := state("slow-1"); s == "succeeded" || s == "failed" {
		t.Fatalf("slow deployment finished early: %s", s)
	}
	close(runtime.release)
	waitFor("slow-1", "succeeded")
	cancel()
	<-stopped
	if runtime.overlap {
		t.Fatal("proxy routes were written concurrently")
	}
}
