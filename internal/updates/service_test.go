package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"shipper/internal/store"
)

const digest = "ghcr.io/matyas-toth/shipperkit@sha256:1111111111111111111111111111111111111111111111111111111111111111"

type feedFixture struct {
	mu      sync.Mutex
	version string
	image   string
	notes   string
}

func (f *feedFixture) set(version string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.version = version
}

func (f *feedFixture) handler(channel string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var value string
		switch strings.TrimPrefix(r.URL.Path, "/"+channel+"/") {
		case "version.txt":
			value = f.version
		case "image.txt":
			value = f.image
		case "release-url.txt":
			value = f.notes
		default:
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(value))
	})
}

func newService(t *testing.T, main, nightly *feedFixture, current string, enabled bool) (*Service, *store.Store) {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/main/"):
			main.handler("main").ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/nightly/"):
			nightly.handler("nightly").ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	service := New(Config{
		Store: db, DataDir: t.TempDir(), CurrentVersion: current,
		MainFeed: server.URL + "/main", NightlyFeed: server.URL + "/nightly",
		APIBase: server.URL + "/api", RawBase: server.URL + "/raw",
		Enabled: enabled, Ready: func() bool { return true },
		Now: func() time.Time { return time.Unix(1_800_000_000, 0) },
	})
	return service, db
}

func fixture() (*feedFixture, *feedFixture) {
	return &feedFixture{version: "1.0.0-rc.9", image: digest, notes: "https://github.com/matyas-toth/shipperkit/releases/tag/v1.0.0-rc.9"},
		&feedFixture{version: "1.0.0-rc.9.nightly.4", image: digest, notes: "https://github.com/matyas-toth/shipperkit/commit/abc123"}
}

func TestCheckApplyAndStatus(t *testing.T) {
	main, nightly := fixture()
	service, _ := newService(t, main, nightly, "1.0.0-rc.8", true)
	state, err := service.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Available == nil || state.Available.Version != "1.0.0-rc.9" || state.Available.Image != digest {
		t.Fatalf("unexpected available state: %+v", state)
	}
	status, err := service.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "queued" || status.ToVersion != "1.0.0-rc.9" || status.RequestID == "" {
		t.Fatalf("unexpected queued status: %+v", status)
	}
	raw, err := os.ReadFile(filepath.Join(service.dataDir, "update-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	var request struct {
		RequestID string `json:"requestId"`
		Version   string `json:"version"`
		Image     string `json:"image"`
		Channel   string `json:"channel"`
	}
	if err = json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	if request.RequestID != status.RequestID || request.Version != "1.0.0-rc.9" || request.Image != digest || request.Channel != "main" {
		t.Fatalf("unexpected request: %+v", request)
	}

	writeStatus(t, service.dataDir, status.RequestID, "updating", "")
	if _, err = service.Apply(context.Background()); err == nil || !strings.Contains(err.Error(), "already") {
		t.Fatal("Apply did not reject an active update", err)
	}
	if service.Status().State != "updating" {
		t.Fatal("active status was not read")
	}

	writeStatus(t, service.dataDir, status.RequestID, "succeeded", "")
	if service.Status().State != "succeeded" {
		t.Fatal("terminal status was not read")
	}
	state, err = service.Check(context.Background())
	if err != nil || state.Available == nil {
		t.Fatal(err, state)
	}
	next, err := service.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if next.RequestID == status.RequestID {
		t.Fatal("new request reused the old identity")
	}
	if service.Status().State != "queued" {
		t.Fatal("a new request did not supersede the old terminal status")
	}
}

func TestChannelsAndAutomaticApply(t *testing.T) {
	main, nightly := fixture()
	service, _ := newService(t, main, nightly, "1.0.0-rc.9", true)
	state, err := service.Check(context.Background())
	if err != nil || state.Available != nil {
		t.Fatal("equal main release was offered", err, state)
	}
	if _, err = service.SetSettings(context.Background(), Nightly, false); err != nil {
		t.Fatal(err)
	}
	state, err = service.Check(context.Background())
	if err != nil || state.Available == nil || state.Available.Version != "1.0.0-rc.9.nightly.4" {
		t.Fatal("nightly release was not offered", err, state)
	}
	if _, err = service.SetSettings(context.Background(), Main, true); err != nil {
		t.Fatal(err)
	}
	main.set("1.0.0-rc.10")
	state, err = service.Check(context.Background())
	if err != nil || state.Available == nil {
		t.Fatal(err, state)
	}
	if err = service.MaybeAutoApply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(service.dataDir, "update-request.json")); err != nil {
		t.Fatal("automatic update did not write a request", err)
	}
	_ = os.Remove(filepath.Join(service.dataDir, "update-request.json"))
	if err = service.MaybeAutoApply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(service.dataDir, "update-request.json")); !os.IsNotExist(err) {
		t.Fatal("failed automatic update retried the same version")
	}
	main.set("1.0.0-rc.11")
	if _, err = service.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = service.MaybeAutoApply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(service.dataDir, "update-request.json")); err != nil {
		t.Fatal("automatic update did not retry a newer version", err)
	}
}

func TestCheckUsesReleaseCommit(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sha := strings.Repeat("a", 40)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/git/ref/heads/latest" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"object": map[string]string{"sha": sha}})
	}))
	defer api.Close()
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + sha + "/version.txt":
			_, _ = w.Write([]byte("1.0.0-rc.9"))
		case "/" + sha + "/image.txt":
			_, _ = w.Write([]byte(digest))
		case "/" + sha + "/release-url.txt":
			_, _ = w.Write([]byte("https://github.com/matyas-toth/shipperkit/releases/tag/v1.0.0-rc.9"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer raw.Close()
	service := New(Config{Store: db, DataDir: t.TempDir(), CurrentVersion: "1.0.0-rc.8",
		MainFeed: "http://127.0.0.1:1/main", NightlyFeed: "http://127.0.0.1:1/nightly",
		APIBase: api.URL, RawBase: raw.URL, Enabled: true, Ready: func() bool { return true }})
	state, err := service.Check(context.Background())
	if err != nil || state.Available == nil || state.Available.Version != "1.0.0-rc.9" {
		t.Fatal(err, state)
	}
}

func TestInvalidFeedAndDisabled(t *testing.T) {
	main, nightly := fixture()
	main.set("not-a-version")
	service, _ := newService(t, main, nightly, "1.0.0-rc.8", true)
	state, err := service.Check(context.Background())
	if err == nil || state.Available != nil || state.Error == "" {
		t.Fatal("invalid feed was accepted", state, err)
	}
	disabled, _ := newService(t, main, nightly, "1.0.0-rc.8", false)
	state, err = disabled.Check(context.Background())
	if err == nil || state.Enabled {
		t.Fatal("disabled service accepted a check", state, err)
	}
	if _, err = disabled.Apply(context.Background()); err == nil {
		t.Fatal("disabled service accepted an update")
	}
}

func TestStatusPrefersNewRequest(t *testing.T) {
	main, nightly := fixture()
	service, _ := newService(t, main, nightly, "1.0.0-rc.8", true)
	writeStatus(t, service.dataDir, "old", "succeeded", "")
	raw, _ := json.Marshal(map[string]string{"requestId": "new", "version": "1.0.0-rc.9", "fromVersion": "1.0.0-rc.8"})
	if err := os.WriteFile(filepath.Join(service.dataDir, "update-request.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	status := service.Status()
	if status.State != "queued" || status.RequestID != "new" || status.ToVersion != "1.0.0-rc.9" {
		t.Fatalf("new request did not supersede old status: %+v", status)
	}
}

func writeStatus(t *testing.T, dir, requestID, state, message string) {
	t.Helper()
	raw, _ := json.Marshal(Status{State: state, Message: message, FromVersion: "1.0.0-rc.8", ToVersion: "1.0.0-rc.9", RequestID: requestID})
	if err := os.WriteFile(filepath.Join(dir, "update-status.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
