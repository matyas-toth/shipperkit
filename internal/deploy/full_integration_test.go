package deploy

import (
	"bytes"
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

type appFixtureSource struct {
	railpack bool
	broken   bool
}

func (f *appFixtureSource) Clone(ctx context.Context, c store.Config, dir string, log func(string)) (string, error) {
	root := filepath.Join(dir, "repository")
	os.MkdirAll(root, 0700)
	server := `const http=require('node:http');const fs=require('node:fs');console.log('runtime-secret='+process.env.RUNTIME_TOKEN);http.createServer((req,res)=>{if(req.url==='/health'){res.end('healthy');return}if(req.url==='/token'){res.end(process.env.RUNTIME_TOKEN||'');return}let hits=0;if(fs.existsSync('/data')){try{hits=Number(fs.readFileSync('/data/hits','utf8'))}catch{}fs.writeFileSync('/data/hits',String(hits+1))}res.end('Shipper full application '+hits)}).listen(Number(process.env.PORT||3000),'0.0.0.0');`
	os.WriteFile(filepath.Join(root, "index.js"), []byte(server), 0644)
	os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"shipper-fixture","version":"1.0.0","scripts":{"start":"node index.js"},"engines":{"node":"24.x"}}`), 0644)
	if !f.railpack {
		dockerfile := "FROM node:24-alpine\nWORKDIR /app\nCOPY index.js .\nRUN --mount=type=secret,id=BUILD_TOKEN test -s /run/secrets/BUILD_TOKEN\nEXPOSE 3000\nCMD [\"node\",\"index.js\"]\n"
		if f.broken {
			dockerfile += "RUN exit 42\n"
		}
		os.WriteFile(filepath.Join(root, "Dockerfile"), []byte(dockerfile), 0644)
	}
	return strings.Repeat("c", 40), nil
}
func TestDockerFullApplications(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_FULL") != "1" {
		t.Skip("set SHIPPER_TEST_FULL=1 for BuildKit/Railpack integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	docker, e := engine.New()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { docker.Close() })
	instance := auth.Token()
	freePort := func() int {
		listener, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		return port
	}
	dir := t.TempDir()
	runtime := &DockerRuntime{Client: docker.Client(), Instance: instance, DataDir: dir, Config: ProxyConfig{BindIP: "127.0.0.1", HTTPPort: freePort(), HTTPSPort: freePort()}}
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	worker := NewWorker(db, runtime, events.New(), dir)
	source := &appFixtureSource{}
	worker.Source = source
	token := "runtime-fixture-" + auth.Token()[:12]
	buildToken := "build-fixture-" + auth.Token()[:12]
	app := store.Application{ID: auth.Token()[:24], Name: "Full application", Config: testConfig()}
	app.Config.Method = "dockerfile"
	app.Config.Dockerfile = "Dockerfile"
	app.Config.RootDirectory = "."
	app.Config.Domain = app.ID + ".localhost"
	app.Config.HealthPath = "/health"
	app.Config.Variables = []store.Variable{{Name: "RUNTIME_TOKEN", Value: token, Secret: true, Scope: "runtime"}, {Name: "BUILD_TOKEN", Value: buildToken, Secret: true, Scope: "build"}}
	app.Config.Volumes = []store.Volume{{Name: "data", Path: "/data"}}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 45*time.Second)
		defer stop()
		containers, _ := runtime.Client.ContainerList(cleanup, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, c := range containers.Items {
			runtime.removeOwned(cleanup, c.ID, "")
		}
		volumes, _ := runtime.Client.VolumeList(cleanup, client.VolumeListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, v := range volumes.Items {
			if owned(v.Labels, instance, "") {
				runtime.Client.VolumeRemove(cleanup, v.Name, client.VolumeRemoveOptions{})
			}
		}
		runtime.Client.NetworkRemove(cleanup, runtime.network(), client.NetworkRemoveOptions{})
		images, _ := runtime.Client.ImageList(cleanup, client.ImageListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, image := range images.Items {
			runtime.Client.ImageRemove(cleanup, image.ID, client.ImageRemoveOptions{})
		}
	})
	run := func(a store.Application, operation string, create bool) store.Deployment {
		t.Helper()
		d, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: operation}, "", create)
		if e != nil {
			t.Fatal(e)
		}
		worker.process(ctx, d)
		got, e := db.Deployment(ctx, d.ID)
		if e != nil {
			t.Fatal(e)
		}
		if got.State != "succeeded" {
			logs, _ := db.Logs(ctx, d.ID, 0)
			t.Fatalf("%s failed: %s\n%+v", operation, got.Error, logs)
		}
		return got
	}
	request := func(host, uri string) (int, string, string) {
		t.Helper()
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d%s", runtime.Config.HTTPPort, uri), nil)
		req.Host = host
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body), res.Header.Get("X-Shipper-Deployment")
	}
	t.Log("Building and starting a Dockerfile application")
	first := run(app, "deploy", true)
	if code, body, release := request(app.Config.Domain, "/"); code != 200 || !strings.Contains(body, "Shipper full application") || release != first.ID {
		t.Fatal("Dockerfile application did not serve", code, body, release)
	}
	if code, body, _ := request(app.Config.Domain, "/token"); code != 200 || body != token {
		t.Fatal("Runtime variable was not delivered")
	}
	current, _ := db.Application(ctx, app.ID)
	beforeContainer := first.ContainerID
	beforeImage := first.ImageID
	current.Config.Domains = []store.Domain{{ID: "primary", Hostname: app.Config.Domain, Primary: true}, {ID: "alias", Hostname: "alias-" + app.Config.Domain}}
	run(current, "domains", false)
	after, _ := db.Application(ctx, app.ID)
	if after.ActiveDeploymentID != first.ID {
		t.Fatal("Domain update replaced the release")
	}
	active, _ := db.Deployment(ctx, first.ID)
	if active.ContainerID != beforeContainer || active.ImageID != beforeImage {
		t.Fatal("Domain update rebuilt or recreated the app")
	}
	if code, _, release := request("alias-"+app.Config.Domain, "/"); code != 200 || release != first.ID {
		t.Fatal("Additional domain did not route")
	}
	t.Log("Testing a failed BuildKit build while the existing release serves")
	source.broken = true
	current, _ = db.Application(ctx, app.ID)
	d, e := db.Enqueue(ctx, current, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	worker.process(ctx, d)
	failed, _ := db.Deployment(ctx, d.ID)
	if failed.State != "failed" {
		t.Fatal("Broken Dockerfile was accepted")
	}
	if code, _, release := request(app.Config.Domain, "/"); code != 200 || release != first.ID {
		t.Fatal("Failed build disturbed active release")
	}
	logs, _ := db.Logs(ctx, d.ID, 0)
	for _, line := range logs {
		if strings.Contains(line.Line, token) || strings.Contains(line.Line, buildToken) {
			t.Fatal("A secret leaked into deployment logs")
		}
	}
	t.Log("Testing runtime log streaming and lifecycle")
	logCtx, stopLogs := context.WithTimeout(ctx, 5*time.Second)
	seen := false
	e = runtime.Logs(logCtx, app.ID, first.ContainerID, 100, func(line string) error {
		if strings.Contains(line, token) {
			seen = true
			stopLogs()
		}
		return nil
	})
	stopLogs()
	if !seen {
		t.Fatal("Runtime log stream did not include application output", e)
	}
	t.Log("Applying a runtime-only variable while source is deliberately unbuildable")
	current, _ = db.Application(ctx, app.ID)
	replacementToken := "replacement-" + auth.Token()[:16]
	for i := range current.Config.Variables {
		if current.Config.Variables[i].Name == "RUNTIME_TOKEN" {
			current.Config.Variables[i].Value = replacementToken
		}
	}
	recreated := run(current, "apply", false)
	if recreated.ImageID != first.ImageID || recreated.Commit != first.Commit {
		t.Fatal("runtime-only edit rebuilt the image")
	}
	if code, body, release := request(app.Config.Domain, "/token"); code != 200 || body != replacementToken || release != recreated.ID {
		t.Fatal("runtime-only configuration did not take effect", code, release)
	}
	current, _ = db.Application(ctx, app.ID)
	run(current, "stop", false)
	stopped, _ := db.Application(ctx, app.ID)
	if stopped.DesiredState != "stopped" {
		t.Fatal("Stop state was not persisted")
	}
	run(stopped, "start", false)
	current, _ = db.Application(ctx, app.ID)
	run(current, "restart", false)
	t.Log("Building and starting an automatically detected Railpack application")
	source.broken = false
	source.railpack = true
	current, _ = db.Application(ctx, app.ID)
	current.Config.Method = "auto"
	current.Config.Volumes = nil
	second := run(current, "deploy", false)
	if second.ResolvedMethod != "railpack" {
		t.Fatal("Railpack was not selected", second.ResolvedMethod)
	}
	if code, body, release := request(app.Config.Domain, "/"); code != 200 || !strings.Contains(body, "Shipper full application") || release != second.ID {
		t.Fatal("Railpack application did not serve", code, body, release)
	}
	t.Log("Process-only worker readiness waits for startup hooks without requiring HTTP")
	workerApp := store.Application{ID: auth.Token()[:24], Name: "Background worker", Config: store.Config{Method: "dockerfile", StartupPolicy: "process", StartCommand: "node -e 'setInterval(()=>{},1000)'", MemoryMB: 64, Hooks: []store.Hook{{Stage: "before_start", Command: store.Command{Argv: []string{"node", "-e", "setTimeout(()=>{},1000)"}, Timeout: 30, Mode: "job"}}}}}
	workerJob, e := db.Enqueue(ctx, workerApp, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	workerJob.ImageID = first.ImageID
	workerJob.ResolvedMethod = "dockerfile"
	workerJob.Commit = first.Commit
	if e = db.SaveDeployment(ctx, workerJob); e != nil {
		t.Fatal(e)
	}
	worker.process(ctx, workerJob)
	workerJob, _ = db.Deployment(ctx, workerJob.ID)
	if workerJob.State != "succeeded" {
		t.Fatal("worker incorrectly required HTTP", workerJob.Error)
	}
	raw, e := os.ReadFile(filepath.Join(dir, "shipper.db-wal"))
	if e == nil && (bytes.Contains(raw, []byte(token)) || bytes.Contains(raw, []byte(buildToken))) {
		t.Fatal("Variables were stored without encryption")
	}
	t.Log("Passed BuildKit Dockerfile builds, Railpack automatic builds, encrypted variables, secret mounts, failed-build preservation, multi-domain route-only updates, runtime logs, and lifecycle")
}
