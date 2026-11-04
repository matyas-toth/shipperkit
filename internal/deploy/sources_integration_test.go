package deploy

import (
	"context"
	"github.com/moby/moby/client"
	"net"
	"os"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/store"
	"strings"
	"testing"
	"time"
)

func TestDockerSourceWorkflows(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_SOURCES") != "1" {
		t.Skip("set SHIPPER_TEST_SOURCES=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	docker, e := engine.New()
	if e != nil {
		t.Fatal(e)
	}
	defer docker.Close()
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	instance := auth.Token()
	free := func() int {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		p := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return p
	}
	r := &DockerRuntime{Client: docker.Client(), Instance: instance, DataDir: dir, Config: ProxyConfig{BindIP: "127.0.0.1", HTTPPort: free(), HTTPSPort: free()}}
	w := NewWorker(db, r, events.New(), dir)
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		items, _ := r.Client.ContainerList(clean, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, c := range items.Items {
			r.removeOwned(clean, c.ID, "")
		}
		networks, _ := r.Client.NetworkList(clean, client.NetworkListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, n := range networks.Items {
			r.Client.NetworkRemove(clean, n.ID, client.NetworkRemoveOptions{})
		}
		volumes, _ := r.Client.VolumeList(clean, client.VolumeListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, v := range volumes.Items {
			r.Client.VolumeRemove(clean, v.Name, client.VolumeRemoveOptions{})
		}
		images, _ := r.Client.ImageList(clean, client.ImageListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, i := range images.Items {
			r.Client.ImageRemove(clean, i.ID, client.ImageRemoveOptions{})
		}
	})
	for _, method := range []string{"image", "inline"} {
		a := store.Application{ID: auth.Token()[:24], Name: "Source " + method, Config: store.Config{Method: method, Image: ApacheImage, InlineDockerfile: "FROM " + ApacheImage + "\nRUN echo 'Inline source works' > /usr/local/apache2/htdocs/index.html\n", Port: 80, MemoryMB: 128, HealthPath: "/"}}
		a.Config.Domain = a.ID + ".localhost"
		if e = Validate(&a.Config); e != nil {
			t.Fatal(e)
		}
		d, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", true)
		if e != nil {
			t.Fatal(e)
		}
		w.process(ctx, d)
		d, e = db.Deployment(ctx, d.ID)
		if e != nil || d.State != "succeeded" {
			logs, _ := db.Logs(ctx, d.ID, 0)
			t.Fatal(method, d.State, d.Error, logs, e)
		}
		if method == "image" && (d.Digest == "" || d.Commit != "") {
			t.Fatal("image identity missing", d)
		}
		if method == "inline" {
			out, e := r.probeHTTP(ctx, "site-"+d.ID, 80, "site-"+d.ID, "/")
			if e != nil || !strings.Contains(string(out), "Inline source works") {
				t.Fatal(string(out), e)
			}
		}
		if method == "inline" {
			first := d
			a, _ = db.Application(ctx, a.ID)
			a.Config.InlineDockerfile = strings.ReplaceAll(a.Config.InlineDockerfile, "Inline source works", "Second release works")
			a.Config.Runtime = store.RuntimePolicy{CPUs: 0.5, PIDs: 128, StopSeconds: 3, StartupSeconds: 30}
			next, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", false)
			if e != nil {
				t.Fatal(e)
			}
			w.process(ctx, next)
			next, _ = db.Deployment(ctx, next.ID)
			if next.State != "succeeded" {
				t.Fatal("second release", next.Error)
			}
			info, e := r.Client.ContainerInspect(ctx, next.ContainerID, client.ContainerInspectOptions{})
			if e != nil || info.Container.HostConfig.NanoCPUs != 500000000 || *info.Container.HostConfig.PidsLimit != 128 || *info.Container.Config.StopTimeout != 3 {
				t.Fatal("Runtime policy was not applied", e)
			}
			a, _ = db.Application(ctx, a.ID)
			a.Config.Domains = []store.Domain{{ID: "primary", Hostname: a.ID + ".localhost", Primary: true, PathPrefix: "/api", StripPrefix: true, Compress: true, ResponseHeaders: map[string]string{"X-Shipper-Test": "routed"}}}
			route, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "domains"}, "", false)
			if e != nil {
				t.Fatal(e)
			}
			w.process(ctx, route)
			route, _ = db.Deployment(ctx, route.ID)
			if route.State != "succeeded" {
				t.Fatal("route", route.Error)
			}
			out, e := r.probeHTTP(ctx, "127.0.0.1", 8080, a.ID+".localhost", "/api/")
			if e != nil || !strings.Contains(string(out), "Second release works") || !strings.Contains(string(out), "X-Shipper-Test: routed") {
				t.Fatal("path/header routing", string(out), e)
			}
			a, _ = db.Application(ctx, a.ID)
			a.Config.Domains = a.Domains
			a.Config.Domains[0].Maintenance = true
			maintenance, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "domains"}, "", false)
			if e != nil {
				t.Fatal(e)
			}
			w.process(ctx, maintenance)
			maintenance, _ = db.Deployment(ctx, maintenance.ID)
			if maintenance.State != "succeeded" {
				t.Fatal("maintenance", maintenance.Error)
			}
			out, e = r.probeHTTP(ctx, "127.0.0.1", 8080, a.ID+".localhost", "/api/")
			if e != nil || !strings.Contains(string(out), "503 Service Unavailable") {
				t.Fatal("maintenance response", string(out), e)
			}
			a, _ = db.Application(ctx, a.ID)
			a.Config.Domains = a.Domains
			a.Config.Domains[0].Maintenance = false
			restoreRoutes, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "domains"}, "", false)
			if e != nil {
				t.Fatal(e)
			}
			w.process(ctx, restoreRoutes)
			a, _ = db.Application(ctx, a.ID)
			a.Config = first.Config
			rollback, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "rollback", RollbackID: first.ID}, "", false)
			if e != nil {
				t.Fatal(e)
			}
			w.process(ctx, rollback)
			d, _ = db.Deployment(ctx, rollback.ID)
			if d.State != "succeeded" || d.ImageID != first.ImageID {
				t.Fatal("rollback", d.Error)
			}
			out, e = r.probeHTTP(ctx, "127.0.0.1", 8080, a.ID+".localhost", "/api/")
			if e != nil || !strings.Contains(string(out), "Inline source works") {
				t.Fatal("rollback did not retain domains", string(out), e)
			}
		}
		if e = r.Lifecycle(ctx, d, "restart"); e != nil {
			t.Fatal(e)
		}
		if e = r.Healthy(ctx, d); e != nil {
			t.Fatal(e)
		}
	}
}
