package deploy

import (
	"context"
	"errors"
	"shipper/internal/events"
	"shipper/internal/store"
	"strings"
	"testing"
)

type imageApplyRuntime struct {
	fakeRuntime
	pulls int
}

func (r *imageApplyRuntime) PullImage(_ context.Context, d *store.Deployment, _ *store.Connection, _ func(string)) error {
	r.pulls++
	d.ImageID = "sha256:image-" + strings.Repeat("a", r.pulls)
	d.Digest = "repo@" + d.ImageID
	d.ResolvedMethod = "image"
	return nil
}
func TestImageRuntimeApplyPreservesMutableTagIdentity(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	r := &imageApplyRuntime{}
	w := NewWorker(db, r, events.New(), dir)
	a := store.Application{ID: "image-apply", Name: "Image", Config: store.Config{Method: "image", Image: "httpd:latest", Port: 80, MemoryMB: 128}}
	var first store.Deployment
	for i, operation := range []string{"deploy", "apply", "deploy"} {
		if i > 0 {
			a, _ = db.Application(ctx, a.ID)
			a.Config.MemoryMB = 256
		}
		d, e := db.Enqueue(ctx, a, store.Deployment{ID: operation + strings.Repeat("x", i+1), Operation: operation}, "", i == 0)
		if e != nil {
			t.Fatal(e)
		}
		w.process(ctx, d)
		d, _ = db.Deployment(ctx, d.ID)
		if d.State != "succeeded" {
			t.Fatal(d.Error)
		}
		if i == 0 {
			first = d
		}
		if i == 1 && (r.pulls != 1 || d.ImageID != first.ImageID || d.Digest != first.Digest) {
			t.Fatal("Apply pulled a changed mutable tag", d)
		}
	}
	if r.pulls != 2 {
		t.Fatal("Redeploy did not resolve the tag", r.pulls)
	}
}

type recoveryRuntime struct {
	taskFixtureRuntime
	missing bool
}

func (r *recoveryRuntime) CheckRelease(context.Context, store.Deployment) error {
	if r.missing {
		return errors.New("Artifact unavailable")
	}
	return nil
}

func TestRecordedReleaseRecoveryAndFailureBoundaries(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	runtime := &recoveryRuntime{}
	w := NewWorker(db, runtime, events.New(), dir)
	w.Source = fixtureSource{commit: strings.Repeat("a", 40)}
	a := store.Application{ID: "restore-app", Name: "Restore", Config: testConfig()}
	first, e := db.Enqueue(ctx, a, store.Deployment{ID: "first", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	w.process(ctx, first)
	first, _ = db.Deployment(ctx, first.ID)
	a, _ = db.Application(ctx, a.ID)
	w.Source = fixtureSource{commit: strings.Repeat("b", 40)}
	second, e := db.Enqueue(ctx, a, store.Deployment{ID: "second", Operation: "deploy"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	w.process(ctx, second)
	for _, fail := range []bool{true, false} {
		a, _ = db.Application(ctx, a.ID)
		a.Config = first.Config
		// Restoring a release must not repeat a migration, even if that release had one.
		a.Config.Hooks = []store.Hook{{Stage: "before_deploy", Command: store.Command{Argv: []string{"migrate"}, Mode: "job", Timeout: 10}}}
		id := "restore"
		if fail {
			id = "restore-missing"
		}
		runtime.missing = fail
		job, e := db.Enqueue(ctx, a, store.Deployment{ID: id, Operation: "rollback", RollbackID: first.ID}, "", false)
		if e != nil {
			t.Fatal(e)
		}
		w.process(ctx, job)
		got, _ := db.Deployment(ctx, id)
		after, _ := db.Application(ctx, a.ID)
		if fail {
			if got.State != "failed" || after.ActiveDeploymentID != second.ID {
				t.Fatal("missing artifact displaced serving release", got, after)
			}
		} else if got.State != "succeeded" || got.Commit != first.Commit || got.ImageID != first.ImageID || after.ActiveDeploymentID != id {
			t.Fatal("recorded release was not restored", got, after)
		}
	}
	if len(runtime.calls) != 0 {
		t.Fatal("Rollback reran migration hooks", runtime.calls)
	}
}

func TestRouteAndRuntimePolicies(t *testing.T) {
	for _, d := range []store.Domain{{PathPrefix: "/api`) || Host(`evil"}, {PathPrefix: "//external"}, {PathPrefix: "/%2e%2e"}, {ResponseHeaders: map[string]string{"X-Shipper-Deployment": "fake"}}, {ResponseHeaders: map[string]string{"X-Test": "ok\r\nInjected: yes"}}} {
		if ValidateRoute(d) == nil {
			t.Fatal("unsafe route accepted", d)
		}
	}
	for _, p := range []store.RuntimePolicy{{CPUs: -1}, {PIDs: 1}, {StopSeconds: 1000}, {Restart: "always"}} {
		if ValidateRuntime(p) == nil {
			t.Fatal("invalid runtime accepted", p)
		}
	}
	a := store.Application{ID: "one", Domains: []store.Domain{{ID: "primary", Hostname: "app.example.com", Primary: true, PathPrefix: "/api", StripPrefix: true, Compress: true, ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}}}}
	routes := Routes([]store.Application{a}, map[string]store.Deployment{a.ID: {ID: "release", Config: store.Config{Port: 80}}})
	http := routes["http"].(map[string]any)
	router := http["routers"].(map[string]any)["app-one"].(map[string]any)
	if router["rule"] != "Host(`app.example.com`) && PathPrefix(`/api`)" || len(router["middlewares"].([]string)) != 4 {
		t.Fatal(router)
	}
	c := testConfig()
	next := c
	next.Runtime = store.RuntimePolicy{CPUs: 2}
	if PlanChanges(c, next).Operation != "recreate" {
		t.Fatal("Runtime edit rebuilds source")
	}
}

func TestDashboardRoutingAndValidation(t *testing.T) {
	c := ProxyConfig{BindIP: "0.0.0.0", HTTPPort: 80, HTTPSPort: 443, ACMEEmail: "owner@example.com", DashboardURL: "https://shipper.example.com", DashboardUpstream: "http://dashboard:8080"}
	if e := ValidateDashboard(c); e != nil {
		t.Fatal(e)
	}
	r := &DockerRuntime{Instance: strings.Repeat("a", 64), Config: c}
	config := Routes(nil, nil)
	r.dashboardRoute(config)
	if len(config["http"].(map[string]any)["routers"].(map[string]any)) != 2 {
		t.Fatal(config)
	}
	c.DashboardURL = "http://shipper.example.com"
	if ValidateDashboard(c) == nil {
		t.Fatal("insecure dashboard accepted")
	}
	// Adding optional dashboard routing must not invalidate an existing proxy's static hash.
	baseline := r.configHash()
	r.Config.DashboardURL = ""
	r.Config.DashboardUpstream = ""
	if r.configHash() != baseline {
		t.Fatal("dynamic routing changed proxy static hash")
	}
}
