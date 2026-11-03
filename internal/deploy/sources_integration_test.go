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
		if e = r.Lifecycle(ctx, d, "restart"); e != nil {
			t.Fatal(e)
		}
		if e = r.Healthy(ctx, d); e != nil {
			t.Fatal(e)
		}
	}
}
