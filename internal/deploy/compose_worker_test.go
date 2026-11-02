package deploy

import (
	"context"
	"errors"
	"testing"

	"shipper/internal/events"
	"shipper/internal/store"
)

type composeFailureRuntime struct {
	fakeRuntime
	failure string
	up      []string
	stopped bool
	deleted bool
}

func (f *composeFailureRuntime) PrepareCompose(_ context.Context, d *store.Deployment, _, _ string, _ func(string)) error {
	d.ComposePlan = `{"project":{"services":{}}}`
	d.ResolvedMethod = "compose"
	d.Services = []store.Service{{Name: "web", Port: 80}}
	return nil
}
func (f *composeFailureRuntime) BuildCompose(context.Context, *store.Deployment, func(string)) error {
	return nil
}
func (f *composeFailureRuntime) UpCompose(_ context.Context, d store.Deployment, _ func(string)) error {
	f.up = append(f.up, d.ID)
	if d.ID == "candidate" && f.failure == "start" {
		return errors.New("Candidate failed to start")
	}
	return nil
}
func (f *composeFailureRuntime) Lifecycle(_ context.Context, _ store.Deployment, action string) error {
	f.stopped = action == "stop"
	return nil
}
func (f *composeFailureRuntime) RemoveApplication(context.Context, string) error {
	f.deleted = true
	return nil
}
func (f *composeFailureRuntime) Routes(ctx context.Context, apps []store.Application, active map[string]store.Deployment) error {
	if f.failure == "withdraw" && len(active) == 0 {
		f.failure = ""
		return errors.New("Proxy withdrawal failed after its write")
	}
	if f.failure == "promote" && active["app"].ID == "candidate" {
		f.failure = ""
		return errors.New("Proxy promotion failed after its write")
	}
	return f.fakeRuntime.Routes(ctx, apps, active)
}

func TestComposeRecoveryPreservesPreviousRunningIntent(t *testing.T) {
	for _, state := range []string{"running", "stopped"} {
		for _, failure := range []string{"withdraw", "start", "promote"} {
			t.Run(state+"/"+failure, func(t *testing.T) {
				dir := t.TempDir()
				db, e := store.Open(dir)
				if e != nil {
					t.Fatal(e)
				}
				defer db.Close()
				ctx := context.Background()
				a := store.Application{ID: "app", Name: "App", Config: store.Config{Method: "compose", ComposeSource: "paste", ComposeYAML: "services: {}", RootDirectory: ".", Domains: []store.Domain{{ID: "web", Service: "web", Hostname: "app.localhost", Primary: true}}}}
				old, e := db.Enqueue(ctx, a, store.Deployment{ID: "previous", Operation: "deploy"}, "", true)
				if e != nil {
					t.Fatal(e)
				}
				old.State = "succeeded"
				old.ImageID = "compose"
				old.ComposePlan = `{"project":{"services":{}}}`
				old.Services = []store.Service{{Name: "web", Port: 80}}
				if e = db.SaveDeployment(ctx, old); e != nil {
					t.Fatal(e)
				}
				db.SetActive(ctx, a.ID, old.ID)
				db.SetDesiredState(ctx, a.ID, state)
				a, _ = db.Application(ctx, a.ID)
				d, e := db.Enqueue(ctx, a, store.Deployment{ID: "candidate", Operation: "deploy"}, "", false)
				if e != nil {
					t.Fatal(e)
				}
				runtime := &composeFailureRuntime{failure: failure}
				w := NewWorker(db, runtime, events.New(), dir)
				w.process(ctx, d)
				failed, _ := db.Deployment(ctx, d.ID)
				after, _ := db.Application(ctx, a.ID)
				if failed.State != "failed" || after.ActiveDeploymentID != old.ID || after.DesiredState != state {
					t.Fatal("Recovery changed the previous active release or running intent", failed.State, after.ActiveDeploymentID, after.DesiredState)
				}
				if len(runtime.up) == 0 || runtime.up[len(runtime.up)-1] != old.ID {
					t.Fatal("Recovery did not restore the previous runtime model")
				}
				if state == "stopped" {
					if !runtime.stopped || len(runtime.active) != 0 {
						t.Fatal("Recovery published or left a previously stopped application running")
					}
				} else if runtime.active[a.ID].ID != old.ID {
					t.Fatal("Recovery did not restore the previous route")
				}
			})
		}
	}
}

func TestComposeFirstPromotionFailureClearsActiveRelease(t *testing.T) {
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	a := store.Application{ID: "app", Name: "App", Config: store.Config{Method: "compose", ComposeSource: "paste", RootDirectory: ".", ComposeYAML: "services: {}"}}
	d, e := db.Enqueue(ctx, a, store.Deployment{ID: "candidate", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	runtime := &composeFailureRuntime{failure: "promote"}
	w := NewWorker(db, runtime, events.New(), dir)
	w.process(ctx, d)
	a, e = db.Application(ctx, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	if a.ActiveDeploymentID != "" || !runtime.deleted || len(runtime.active) != 0 {
		t.Fatal("First failed promotion retained an active identity, routes or containers")
	}
}
