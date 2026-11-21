package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shipper/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestReportIsAnonymousAndAggregated(t *testing.T) {
	db := testStore(t)
	ctx := context.Background()
	app := store.Application{ID: "app-1", Name: "secret-customer-portal", SourceKind: "git", SourceRef: "https://github.com/acme/private.git", Method: "dockerfile",
		Config: store.Config{Method: "dockerfile", Repository: "https://github.com/acme/private.git", Domain: "portal.acme.example", Database: &store.DatabaseResource{Engine: "postgres", Version: "17"},
			Variables: []store.Variable{{Name: "API_KEY", Value: "do-not-send", Secret: true, Scope: "runtime"}}}}
	deployment, err := db.Enqueue(ctx, app, store.Deployment{ID: "release-1", Operation: "deploy"}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	deployment.State, deployment.StartedAt, deployment.FinishedAt = "succeeded", time.Now().Unix()-42, time.Now().Unix()
	if err = db.SaveDeployment(ctx, deployment); err != nil {
		t.Fatal(err)
	}
	if err = db.SetDomains(ctx, app.ID, []store.Domain{{ID: "d1", Hostname: "portal.acme.example", HTTPS: true, Primary: true}}); err != nil {
		t.Fatal(err)
	}

	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	service := New(Config{Store: db, Version: "1.2.3", Endpoint: server.URL, Allowed: true, Managed: true,
		Platform: func() Platform { return Platform{HostOS: "Ubuntu 24.04 LTS", Docker: "29.0.0", CPUs: 4, MemoryMB: 8192} }})
	if err = service.send(ctx); err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{"secret-customer-portal", "acme", "do-not-send", "API_KEY", "app-1", "release-1"} {
		if strings.Contains(string(received), leaked) {
			t.Fatalf("report leaked %q: %s", leaked, received)
		}
	}
	var report Report
	if err = json.Unmarshal(received, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.ID) != 32 || report.Version != "1.2.3" || report.Install != "managed" || report.Platform.HostOS != "Ubuntu 24.04 LTS" {
		t.Fatalf("unexpected report identity: %+v", report)
	}
	if report.Usage.Applications != 1 || report.Usage.Domains != 1 || report.Usage.Deployments != 1 || report.Usage.DeployMedianSeconds != 42 {
		t.Fatalf("unexpected usage: %+v", report.Usage)
	}
	if report.Methods["dockerfile"] != 1 || report.Databases["postgres"] != 1 || report.Sources["git"] != 1 {
		t.Fatalf("unexpected breakdowns: %+v %+v %+v", report.Methods, report.Databases, report.Sources)
	}
	again, err := service.Collect(ctx)
	if err != nil || again.ID != report.ID {
		t.Fatalf("identifier was not stable: %q %q %v", again.ID, report.ID, err)
	}
	if state, _ := service.State(ctx); state.LastSent == 0 {
		t.Fatal("successful delivery was not recorded")
	}
}

func TestOptOut(t *testing.T) {
	db := testStore(t)
	ctx := context.Background()
	locked := New(Config{Store: db, Allowed: false})
	if locked.Enabled(ctx) || locked.SetEnabled(ctx, true) == nil {
		t.Fatal("environment opt-out could be overridden")
	}
	service := New(Config{Store: db, Allowed: true})
	if !service.Enabled(ctx) {
		t.Fatal("usage data should default on when the environment allows it")
	}
	if err := service.SetEnabled(ctx, false); err != nil || service.Enabled(ctx) {
		t.Fatalf("setting did not turn usage data off: %v", err)
	}
}

func TestNormalizeFoldsUnexpectedValues(t *testing.T) {
	if normalize("Dockerfile") != "dockerfile" || normalize("") != "none" || normalize("https://example.com/x") != "other" {
		t.Fatal("keys were not normalized")
	}
	values := map[string]int{}
	for i := 0; i < 40; i++ {
		values[strings.Repeat("a", i+1)] = i + 1
	}
	trim(values)
	if len(values) != maxKeys || values["other"] == 0 {
		t.Fatalf("long tail was not folded: %d keys", len(values))
	}
}
