package deploy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/store"
)

// The defaults a first deploy relies on: an API without a root route, a port
// nobody configured, data in an image's VOLUME, and a database's public domain
// changing without a restart.
func TestDockerFirstDeployDefaults(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_DOCKER") != "1" {
		t.Skip("set SHIPPER_TEST_DOCKER=1 for Docker first deploy defaults")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	docker, e := engine.New()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { docker.Close() })
	free := func() int {
		listener, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		defer listener.Close()
		return listener.Addr().(*net.TCPAddr).Port
	}
	dir := t.TempDir()
	instance := auth.Token()
	r := &DockerRuntime{Client: docker.Client(), Instance: instance, DataDir: dir, Config: ProxyConfig{BindIP: "127.0.0.1", HTTPPort: free(), HTTPSPort: free()}}
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	w := NewWorker(db, r, events.New(), dir)
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		cs, _ := r.Client.ContainerList(clean, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, c := range cs.Items {
			if owned(c.Labels, instance, "") {
				_ = r.removeOwned(clean, c.ID, "")
			}
		}
		vs, _ := r.Client.VolumeList(clean, client.VolumeListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, v := range vs.Items {
			if owned(v.Labels, instance, "") {
				_, _ = r.Client.VolumeRemove(clean, v.Name, client.VolumeRemoveOptions{})
			}
		}
		ns, _ := r.Client.NetworkList(clean, client.NetworkListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, n := range ns.Items {
			if owned(n.Labels, instance, "") {
				_, _ = r.Client.NetworkRemove(clean, n.ID, client.NetworkRemoveOptions{})
			}
		}
	})
	run := func(t *testing.T, a store.Application, operation string, create bool) store.Deployment {
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
		if os.Getenv("SHIPPER_TEST_TIMINGS") == "1" {
			logs, _ := db.Logs(ctx, d.ID, 0)
			for _, l := range logs {
				t.Logf("%d %s %s", l.CreatedAt-got.CreatedAt, l.Step, l.Line)
			}
		}
		if got.State != "succeeded" {
			logs, _ := db.Logs(ctx, d.ID, 0)
			t.Fatalf("%s failed: %s\n%+v", operation, got.Error, logs)
		}
		return got
	}
	logged := func(d store.Deployment, text string) bool {
		logs, _ := db.Logs(ctx, d.ID, 0)
		for _, line := range logs {
			if strings.Contains(line.Line, text) {
				return true
			}
		}
		return false
	}

	t.Run("api on an unconfigured port", func(t *testing.T) {
		// Answers 404 everywhere on port 8000, with no EXPOSE and no PORT variable.
		script := "from http.server import BaseHTTPRequestHandler,HTTPServer\n" +
			"class H(BaseHTTPRequestHandler):\n" +
			" def do_GET(self):\n" +
			"  self.send_response(404);self.end_headers();self.wfile.write(b'not found')\n" +
			"HTTPServer(('0.0.0.0',8000),H).serve_forever()"
		app := store.Application{ID: auth.Token()[:24], Name: "API", Config: store.Config{Method: "image", Image: "python:3.13-alpine", StartCommand: `python -u -c "$SCRIPT"`, Variables: []store.Variable{{Name: "SCRIPT", Value: script, Scope: "runtime"}}}}
		app.Config.Domain = app.ID + ".localhost"
		if e := ValidateBuild(&app.Config); e != nil {
			t.Fatal(e)
		}
		first := run(t, app, "deploy", true)
		if first.RuntimePort != 8000 || !logged(first, "listens on port 8000") {
			t.Fatalf("the listening port was not adopted: %d", first.RuntimePort)
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/", r.Config.HTTPPort), nil)
		req.Host = app.Config.Domain
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 404 || string(body) != "not found" || res.Header.Get("X-Shipper-Deployment") != first.ID {
			t.Fatalf("the proxy did not route to the adopted port: %d %q", res.StatusCode, body)
		}
		current, _ := db.Application(ctx, app.ID)
		second := run(t, current, "deploy", false)
		if second.RuntimePort != 8000 || logged(second, "listens on port 8000") {
			t.Fatal("a redeploy should start from the port the serving release answered on")
		}
		t.Log("An API answering 404 on an unconfigured port deployed and redeployed")
	})

	t.Run("image volume", func(t *testing.T) {
		// redis 7 declares VOLUME /data.
		image := "redis:7-alpine"
		start := "test -f /data/marker || cat /proc/sys/kernel/random/uuid > /data/marker; exec sleep 3600"
		if e := r.pull(ctx, image, func(string) {}); e != nil {
			t.Fatal(e)
		}
		read := func(id string) string {
			t.Helper()
			out, e := r.execPipe(ctx, id, []string{"cat", "/data/marker"}, nil, nil, false)
			if e != nil {
				t.Fatal(e)
			}
			return strings.TrimSpace(string(out))
		}
		// A release from before this behaviour wrote into an anonymous volume.
		appID := auth.Token()[:24]
		legacy := store.Deployment{ID: auth.Token()[:24], ApplicationID: appID, ApplicationName: "Legacy", ImageID: image, ResolvedMethod: "image", Config: store.Config{Method: "image", Image: image, StartupPolicy: "process", StartCommand: start}}
		if legacy.ContainerID, e = r.Start(ctx, legacy); e != nil {
			t.Fatal(e)
		}
		if e = r.Healthy(ctx, legacy); e != nil {
			t.Fatal(e)
		}
		marker := read(legacy.ContainerID)
		next := legacy
		next.ID, next.PreviousID, next.ContainerID = auth.Token()[:24], legacy.ID, ""
		copied := false
		volumes, e := r.ImageVolumes(ctx, next, func(line string) { copied = copied || strings.Contains(line, "Copied") })
		if e != nil {
			t.Fatal(e)
		}
		if len(volumes) != 1 || volumes[0] != (store.Volume{Name: "data", Path: "/data"}) || !copied {
			t.Fatalf("expected the declared /data volume with the current data: %v %v", volumes, copied)
		}
		next.Config.Volumes = volumes
		if next.ContainerID, e = r.Start(ctx, next); e != nil {
			t.Fatal(e)
		}
		if got := read(next.ContainerID); got != marker {
			t.Fatalf("the upgrade lost the data in /data: %q != %q", got, marker)
		}

		// Through the worker the volume is recorded and survives every redeploy.
		app := store.Application{ID: auth.Token()[:24], Name: "Stateful", Config: store.Config{Method: "image", Image: image, StartupPolicy: "process", StartCommand: start}}
		if e := ValidateBuild(&app.Config); e != nil {
			t.Fatal(e)
		}
		first := run(t, app, "deploy", true)
		saved, _ := db.Application(ctx, app.ID)
		if len(saved.Config.Volumes) != 1 || saved.Config.Volumes[0].Path != "/data" || len(first.Config.Volumes) != 1 {
			t.Fatalf("the declared volume was not recorded: %v %v", saved.Config.Volumes, first.Config.Volumes)
		}
		if plan := PlanChanges(first.Config, saved.Config); plan.Operation != "none" {
			t.Fatalf("recording the volume left a pending change: %#v", plan)
		}
		marker = read(first.ContainerID)
		second := run(t, saved, "deploy", false)
		if got := read(second.ContainerID); got != marker {
			t.Fatalf("a redeploy lost the data in /data: %q != %q", got, marker)
		}
		t.Log("Data in an image's declared volume survived the upgrade and a redeploy")
	})

	t.Run("database public domain", func(t *testing.T) {
		port := store.DatabaseDomainPorts["redis"]
		store.DatabaseDomainPorts["redis"] = free()
		t.Cleanup(func() { store.DatabaseDomainPorts["redis"] = port })
		c, e := DatabaseResourceConfig("redis", "8")
		if e != nil {
			t.Fatal(e)
		}
		if e = ValidateBuild(&c); e != nil {
			t.Fatal(e)
		}
		app := store.Application{ID: auth.Token()[:24], Name: "Cache", Config: c}
		release := run(t, app, "deploy", true)
		current, _ := db.Application(ctx, app.ID)
		current.Config.Database.Domain = "cache.shipper.test"
		if plan := PlanChanges(release.Config, current.Config); plan.Operation != "route" {
			t.Fatalf("a domain change should only route: %#v", plan)
		}
		run(t, current, "route", false)
		after, _ := db.Application(ctx, app.ID)
		serving, _ := db.Deployment(ctx, after.ActiveDeploymentID)
		if after.ActiveDeploymentID != release.ID || serving.ContainerID != release.ContainerID || serving.Config.Database.Domain != "cache.shipper.test" {
			t.Fatal("the domain change replaced the database or was not recorded")
		}
		if plan := PlanChanges(serving.Config, after.Config); plan.Operation != "none" {
			t.Fatalf("the domain change left a pending change: %#v", plan)
		}
		proxy, e := r.Client.ContainerInspect(ctx, r.proxy(), client.ContainerInspectOptions{})
		if e != nil || proxy.Container.Config.Labels[proxyDatabasesLabel] != "redis" {
			t.Fatal("the proxy did not open the redis entrypoint", e)
		}
		after.Config.Database.Domain = ""
		run(t, after, "route", false)
		proxy, e = r.Client.ContainerInspect(ctx, r.proxy(), client.ContainerInspectOptions{})
		if e != nil || proxy.Container.Config.Labels[proxyDatabasesLabel] != "" {
			t.Fatal("the proxy kept the redis entrypoint", e)
		}
		info, e := r.Client.ContainerInspect(ctx, release.ContainerID, client.ContainerInspectOptions{})
		if e != nil || !info.Container.State.Running || info.Container.RestartCount != 0 {
			t.Fatal("the database restarted", e)
		}
		t.Log("Adding and removing a public domain kept the database running")
	})
}
