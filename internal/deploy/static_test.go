package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"shipper/internal/events"
	"shipper/internal/store"
)

func testConfig() store.StaticConfig {
	return store.StaticConfig{Repository: "https://github.com/example/site", PublishDirectory: ".", Routing: "files", Domain: "test.localhost"}
}
func TestStaticValidationAndPublicAddresses(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "https://owner:secret@github.com/site", "https://127.0.0.1/site", "https://github.com/site?token=secret", "http://github.com/site", "https://localhost/site"} {
		c := testConfig()
		c.Repository = raw
		if Validate(&c) == nil {
			t.Errorf("accepted repository %s", raw)
		}
	}
	for _, raw := range []string{"../escape", "C:/secret", "/etc", "..\\escape"} {
		c := testConfig()
		c.PublishDirectory = raw
		if Validate(&c) == nil {
			t.Errorf("accepted publish directory %s", raw)
		}
	}
	for _, raw := range []string{"https://example.com", "example.com:8080", "example.com`)", "*.example.com", "-bad.example"} {
		c := testConfig()
		c.Domain = raw
		if Validate(&c) == nil {
			t.Errorf("accepted domain %s", raw)
		}
	}
	for _, raw := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "198.18.0.1", "192.0.2.4"} {
		if publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("accepted private/reserved IP %s", raw)
		}
	}
	c := testConfig()
	if e := Validate(&c); e != nil {
		t.Fatal(e)
	}
}
func TestArchiveIsolationAndApacheModes(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{"index.html": "<h1>Site</h1>", ".env": "SECRET=hidden", ".htaccess": "RewriteEngine On", "about.html": "About", ".git/config": "credentials"} {
		file := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(file), 0700)
		os.WriteFile(file, []byte(data), 0600)
	}
	c := testConfig()
	c.HTAccess = true
	archive, e := StaticArchive(dir, c, "release-1")
	if e != nil {
		t.Fatal(e)
	}
	names := map[string]string{}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		body, _ := io.ReadAll(tr)
		names[h.Name] = string(body)
	}
	if !strings.Contains(names["usr/local/apache2/conf/httpd.conf"], "AllowOverrideList RewriteEngine") || !strings.Contains(names["usr/local/apache2/conf/httpd.conf"], "X-Shipper-Deployment \"release-1\"") {
		t.Fatal("Apache policy/provenance missing")
	}
	if names["usr/local/apache2/htdocs/.htaccess"] == "" {
		t.Fatal(".htaccess missing")
	}
	for name := range names {
		if strings.Contains(name, ".env") || strings.Contains(name, ".git/") {
			t.Fatal("secret in archive", name)
		}
	}
	c.Routing = "spa"
	if !strings.Contains(ApacheConfig(c), "FallbackResource /index.html") {
		t.Fatal("SPA fallback missing")
	}
	c.Routing = "strict"
	if strings.Contains(ApacheConfig(c), "FallbackResource") || strings.Contains(ApacheConfig(c), "\nRewriteRule") {
		t.Fatal("strict routing is not strict")
	}
	// Symlinks may be unavailable on Windows without developer mode.
	outside := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(outside, []byte("private"), 0600)
	if os.Symlink(outside, filepath.Join(dir, "escape.txt")) == nil {
		if _, e = StaticArchive(dir, c, "release-1"); e == nil {
			t.Fatal("archive followed a symlink")
		}
	}
}

type fixtureSource struct {
	broken bool
	commit string
}

func (f fixtureSource) Clone(ctx context.Context, c store.StaticConfig, dir string, log func(string)) (string, error) {
	repo := filepath.Join(dir, "repository")
	os.MkdirAll(repo, 0700)
	os.WriteFile(filepath.Join(repo, "index.html"), []byte("<h1>Shipper static fixture</h1>"), 0600)
	os.WriteFile(filepath.Join(repo, "about.html"), []byte("About Shipper"), 0600)
	if c.HTAccess {
		rules := "<IfModule mod_rewrite.c>\nRewriteEngine On\nRewriteRule ^hello$ about.html [L]\n</IfModule>\n"
		if f.broken {
			rules = "InvalidApacheDirective yes\n"
		}
		os.WriteFile(filepath.Join(repo, ".htaccess"), []byte(rules), 0600)
	}
	log("Checked out fixture")
	return f.commit, nil
}

type fakeRuntime struct {
	failHealth bool
	failRoute  bool
	removed    []string
	active     map[string]store.Deployment
}

func (f *fakeRuntime) Ensure(context.Context) error { return nil }
func (f *fakeRuntime) Package(context.Context, store.Deployment, []byte, func(string)) (string, error) {
	return "sha256:fixture", nil
}
func (f *fakeRuntime) Start(ctx context.Context, d store.Deployment) (string, error) {
	return "container-" + d.ID, nil
}
func (f *fakeRuntime) Healthy(context.Context, store.Deployment) error {
	if f.failHealth {
		return errors.New("Health check failed")
	}
	return nil
}
func (f *fakeRuntime) Routes(ctx context.Context, apps []store.Application, a map[string]store.Deployment) error {
	if f.failRoute {
		f.failRoute = false
		return errors.New("Proxy rejected candidate")
	}
	f.active = a
	return nil
}
func (f *fakeRuntime) RemoveDeployment(ctx context.Context, d store.Deployment) error {
	f.removed = append(f.removed, d.ID)
	return nil
}
func (f *fakeRuntime) RemoveApplication(context.Context, string) error            { return nil }
func (f *fakeRuntime) ProbeLocal(context.Context, store.Deployment) (bool, error) { return true, nil }
func TestFailedRedeployPreservesReleaseAndRecovery(t *testing.T) {
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	runtime := &fakeRuntime{}
	w := NewWorker(db, runtime, events.New(), dir)
	w.Source = fixtureSource{commit: strings.Repeat("a", 40)}
	app := store.Application{ID: "application-1", Name: "Website", Config: testConfig()}
	first, e := db.Enqueue(ctx, app, store.Deployment{ID: "release-1", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	w.process(ctx, first)
	current, _ := db.Application(ctx, app.ID)
	if current.ActiveDeploymentID != first.ID {
		t.Fatal("initial deployment failed", current)
	}
	for _, routeFailure := range []bool{false, true} {
		id := "release-health"
		runtime.failHealth = !routeFailure
		runtime.failRoute = routeFailure
		if routeFailure {
			id = "release-route"
		}
		next, e := db.Enqueue(ctx, current, store.Deployment{ID: id, Operation: "deploy"}, "", false)
		if e != nil {
			t.Fatal(e)
		}
		w.process(ctx, next)
		got, _ := db.Deployment(ctx, id)
		after, _ := db.Application(ctx, app.ID)
		if got.State != "failed" || after.ActiveDeploymentID != first.ID {
			t.Fatalf("failed release replaced active: %+v %+v", got, after)
		}
	}
	runtime.failHealth = false
	next, e := db.Enqueue(ctx, current, store.Deployment{ID: "release-recovered", Operation: "deploy"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	next.State = "promoting"
	next.ImageID = "sha256:fixture"
	next.ContainerID = "container-release-recovered"
	next.Commit = strings.Repeat("a", 40)
	db.SaveDeployment(ctx, next)
	db.SetActive(ctx, app.ID, next.ID)
	pending, e := db.Pending(ctx)
	if e != nil || len(pending) != 1 {
		t.Fatal(pending, e)
	}
	w.process(ctx, pending[0])
	done, _ := db.Deployment(ctx, next.ID)
	if done.State != "succeeded" {
		t.Fatal("promotion was not recovered", done)
	}
}
