package deploy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/store"
)

type composeFixtureSource struct {
	broken  bool
	version string
}

func (f *composeFixtureSource) Clone(ctx context.Context, c store.Config, dir string, log func(string)) (string, error) {
	root := filepath.Join(dir, "repository")
	os.MkdirAll(filepath.Join(root, "web"), 0700)
	dockerfile := "FROM " + ApacheImage + "\nCOPY index.html /usr/local/apache2/htdocs/index.html\n"
	if f.broken {
		dockerfile += "RUN exit 42\n"
	}
	os.WriteFile(filepath.Join(root, "web", "Dockerfile"), []byte(dockerfile), 0644)
	os.WriteFile(filepath.Join(root, "web", "index.html"), []byte("Compose "+f.version), 0644)
	os.WriteFile(filepath.Join(root, "message.txt"), []byte("file mount works"), 0644)
	os.WriteFile(filepath.Join(root, "service.env"), []byte("FROM_FILE=from-file\n"), 0600)
	yaml := `services:
  web:
    build: ./web
    ports: ['8080:80']
    env_file: service.env
    volumes: ['./message.txt:/usr/local/apache2/htdocs/message.txt:ro']
    depends_on:
      db: {condition: service_healthy}
  db:
    image: postgres:17-alpine
    environment:
      POSTGRES_PASSWORD: ${DATABASE_PASSWORD:?required}
    volumes: ['database:/var/lib/postgresql/data']
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U postgres']
      interval: 2s
      timeout: 2s
      retries: 20
volumes:
  database:
`
	os.WriteFile(filepath.Join(root, "compose.yaml"), []byte(yaml), 0600)
	return strings.Repeat("d", 40), nil
}
func TestDockerComposeApplications(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_COMPOSE") != "1" {
		t.Skip("set SHIPPER_TEST_COMPOSE=1 for real Compose integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	docker, e := engine.New()
	if e != nil {
		t.Fatal(e)
	}
	defer docker.Close()
	instance := auth.Token()
	dir := t.TempDir()
	free := func() int {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		p := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return p
	}
	runtime := &DockerRuntime{Client: docker.Client(), Instance: instance, DataDir: dir, Config: ProxyConfig{BindIP: "127.0.0.1", HTTPPort: free(), HTTPSPort: free()}}
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	w := NewWorker(db, runtime, events.New(), dir)
	source := &composeFixtureSource{version: "one"}
	w.Source = source
	a := store.Application{ID: auth.Token()[:24], Name: "Compose fixture", Config: store.Config{Method: "compose", ComposeSource: "git", ComposeFile: "compose.yaml", RootDirectory: ".", Repository: "https://github.com/example/app.git", Variables: []store.Variable{{Name: "DATABASE_PASSWORD", Value: "fixture-password-" + auth.Token()[:12], Secret: true}}}}
	a.Config.Domains = []store.Domain{{ID: "web", Service: "web", Hostname: a.ID + ".localhost", Primary: true}}
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		items, _ := runtime.Client.ContainerList(clean, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, c := range items.Items {
			runtime.removeOwned(clean, c.ID, "")
		}
		networks, _ := runtime.Client.NetworkList(clean, client.NetworkListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, n := range networks.Items {
			runtime.Client.NetworkRemove(clean, n.ID, client.NetworkRemoveOptions{})
		}
		volumes, _ := runtime.Client.VolumeList(clean, client.VolumeListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, v := range volumes.Items {
			runtime.Client.VolumeRemove(clean, v.Name, client.VolumeRemoveOptions{})
		}
	})
	run := func(operation string, create bool, want string) store.Deployment {
		t.Helper()
		d, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: operation}, "", create)
		if e != nil {
			t.Fatal(e)
		}
		w.process(ctx, d)
		got, e := db.Deployment(ctx, d.ID)
		if e != nil {
			t.Fatal(e)
		}
		if got.State != want {
			logs, _ := db.Logs(ctx, d.ID, 0)
			t.Fatalf("%s %s: %s\n%+v", operation, got.State, got.Error, logs)
		}
		return got
	}
	request := func(path string, hosts ...string) string {
		t.Helper()
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d%s", runtime.Config.HTTPPort, path), nil)
		req.Host = a.ID + ".localhost"
		if len(hosts) > 0 {
			req.Host = hosts[0]
		}
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("HTTP %d: %s", res.StatusCode, body)
		}
		return string(body)
	}
	t.Log("Deploying a repository Compose application with PostgreSQL and a relative file mount")
	first := run("deploy", true, "succeeded")
	if !strings.Contains(request("/"), "Compose one") || !strings.Contains(request("/message.txt"), "file mount works") {
		t.Fatal("Repository build or file mount did not serve")
	}
	items, e := runtime.composeContainers(ctx, a.ID)
	if e != nil || len(items) != 2 {
		t.Fatal(items, e)
	}
	dbID := ""
	for _, c := range items {
		if c.Labels["com.docker.compose.service"] == "db" {
			dbID = c.ID
		}
		if len(c.Ports) > 0 {
			for _, p := range c.Ports {
				if p.PublicPort > 0 {
					t.Fatal("Compose published a host port")
				}
			}
		}
	}
	if _, e = runtime.exec(ctx, dbID, []string{"psql", "-U", "postgres", "-c", "CREATE TABLE shipper_proof(value text); INSERT INTO shipper_proof VALUES ('survived');"}); e != nil {
		t.Fatal(e)
	}
	a, _ = db.Application(ctx, a.ID)
	source.version = "two"
	second := run("deploy", false, "succeeded")
	if second.ID == first.ID || !strings.Contains(request("/"), "Compose two") {
		t.Fatal("Compose did not update")
	}
	items, _ = runtime.composeContainers(ctx, a.ID)
	for _, c := range items {
		if c.Labels["com.docker.compose.service"] == "db" {
			if c.ID != dbID {
				t.Fatal("Unchanged database container was recreated")
			}
		}
	}
	if out, e := runtime.exec(ctx, dbID, []string{"psql", "-U", "postgres", "-Atc", "SELECT value FROM shipper_proof"}); e != nil || !strings.Contains(string(out), "survived") {
		t.Fatal("Database data lost", string(out), e)
	}
	t.Log("Editing inherited service variables and recovering a failed HTTP startup")
	a, _ = db.Application(ctx, a.ID)
	a.Config.Services = []store.ServiceConfig{{Name: "web", Port: ptr(80), HealthPath: "/", Variables: []store.Variable{{Name: "RENAMED_FROM_FILE", Keep: true, KeepFrom: "FROM_FILE", Secret: true, Scope: "runtime"}}, UnsetVariables: []string{"FROM_FILE"}}}
	if e = store.MergeReleaseVariables(&a.Config, a.Config, second); e != nil {
		t.Fatal(e)
	}
	second = run("deploy", false, "succeeded")
	items, _ = runtime.composeContainers(ctx, a.ID)
	for _, c := range items {
		if c.Labels["com.docker.compose.service"] == "web" {
			info, e := runtime.Client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
			if e != nil {
				t.Fatal(e)
			}
			env := strings.Join(info.Container.Config.Env, "\n")
			if !strings.Contains(env, "RENAMED_FROM_FILE=from-file") || strings.Contains("\n"+env, "\nFROM_FILE=") {
				t.Fatal("Service environment override/removal failed")
			}
		}
	}
	a, _ = db.Application(ctx, a.ID)
	a.Config.Services[0].HealthPath = "/not-found"
	source.version = "three"
	run("deploy", false, "failed")
	if !strings.Contains(request("/"), "Compose two") {
		t.Fatal("Failed HTTP startup did not restore the previous model")
	}
	a, _ = db.Application(ctx, a.ID)
	if a.ActiveDeploymentID != second.ID {
		t.Fatal("Failed startup changed active identity")
	}
	a.Config.Services[0].HealthPath = "/"
	source.version = "two"

	t.Log("Checking failed builds, lifecycle, routes, inventory and volume retention")
	a, _ = db.Application(ctx, a.ID)
	source.broken = true
	run("deploy", false, "failed")
	if !strings.Contains(request("/"), "Compose two") {
		t.Fatal("Failed build displaced serving release")
	}
	source.broken = false
	a, _ = db.Application(ctx, a.ID)
	run("restart", false, "succeeded")
	if !strings.Contains(request("/"), "Compose two") {
		t.Fatal("Restart lost routing")
	}
	a, _ = db.Application(ctx, a.ID)
	run("stop", false, "succeeded")
	a, _ = db.Application(ctx, a.ID)
	run("start", false, "succeeded")
	snapshot, e := docker.Snapshot(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.ReplaceInventory(ctx, snapshot.Containers); e != nil {
		t.Fatal(e)
	}
	a, _ = db.Application(ctx, a.ID)
	if len(a.Containers) != 2 || a.State != "running" {
		t.Fatal("Compose inventory not grouped", a.State, len(a.Containers))
	}
	volume := runtime.composeProject(a.ID) + "-database"
	run("delete", false, "succeeded")
	if _, e = runtime.Client.VolumeInspect(ctx, volume, client.VolumeInspectOptions{}); e != nil {
		t.Fatal("Delete removed persistent volume", e)
	}
	t.Log("Deploying pasted Compose without a repository")
	a = store.Application{ID: auth.Token()[:24], Name: "Pasted fixture", Config: store.Config{Method: "compose", ComposeSource: "paste", RootDirectory: ".", ComposeYAML: "services:\n  web:\n    image: " + ApacheImage + "\n    expose: ['80']\n    configs: [{source: web-page, target: /usr/local/apache2/htdocs/index.html}]\n  api:\n    image: " + ApacheImage + "\n    expose: ['80']\n    configs: [{source: api-page, target: /usr/local/apache2/htdocs/index.html}]\nconfigs:\n  web-page: {content: Web service}\n  api-page: {content: API service}\n"}}
	a.Config.Domains = []store.Domain{{ID: "web", Service: "web", Hostname: a.ID + ".localhost", Primary: true}, {ID: "api", Service: "api", Hostname: "api-" + a.ID + ".localhost", Primary: true}}
	run("deploy", true, "succeeded")
	if !strings.Contains(request("/"), "Web service") {
		t.Fatal("Pasted Compose did not serve")
	}
	if !strings.Contains(request("/", "api-"+a.ID+".localhost"), "API service") {
		t.Fatal("Per-service domain reached the wrong backend")
	}

}
