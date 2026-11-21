package deploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"

	"github.com/moby/moby/client"
	"shipper/internal/events"
	"shipper/internal/store"
	"testing"
)

// Applications used to get a network each. Every release now joins the shared
// private network first, so an old one is emptied and removed, never another
// instance's.
func TestRetiredApplicationNetworkIsEmptiedAndRemoved(t *testing.T) {
	for _, test := range []struct {
		name, endpoint string
		foreign        bool
		remove         bool
		disconnect     bool
	}{
		{name: "empty", remove: true},
		{name: "proxy only", endpoint: "proxy", remove: true, disconnect: true},
		{name: "connected application", endpoint: "consumer", remove: true, disconnect: true},
		{name: "foreign network", foreign: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			instance, app := strings.Repeat("a", 64), strings.Repeat("b", 24)
			var removed, disconnected atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == "GET" && strings.Contains(r.URL.Path, "/networks/"):
					owner := instance
					if test.foreign {
						owner = strings.Repeat("c", 64)
					}
					endpoints := map[string]any{}
					if test.endpoint != "" {
						endpoints[test.endpoint] = map[string]string{"Name": test.endpoint}
					}
					json.NewEncoder(w).Encode(map[string]any{"Id": "network", "Labels": map[string]string{"io.shipper.instance": owner, "io.shipper.application.id": app}, "Containers": endpoints})
				case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/json"):
					json.NewEncoder(w).Encode(map[string]any{"Id": "proxy", "Config": map[string]any{"Labels": map[string]string{"io.shipper.instance": instance, "io.shipper.infrastructure": "proxy"}}})
				case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/disconnect"):
					disconnected.Store(true)
					w.WriteHeader(200)
				case r.Method == "DELETE" && strings.Contains(r.URL.Path, "/networks/"):
					removed.Store(true)
					w.WriteHeader(204)
				default:
					t.Errorf("Unexpected Docker request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			cli, err := client.New(client.WithHost(server.URL), client.WithAPIVersion("1.48"))
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()
			runtime := &DockerRuntime{Client: cli, Instance: instance}
			err = runtime.removeApplicationNetwork(context.Background(), app)
			if (err != nil) != test.foreign || removed.Load() != test.remove || disconnected.Load() != test.disconnect {
				t.Fatalf("Network cleanup violated ownership or live-connection bounds: removed=%v disconnected=%v error=%v", removed.Load(), disconnected.Load(), err)
			}
		})
	}
}

func TestNetworkAccessValidation(t *testing.T) {
	app := "111111111111111111111111"
	valid := store.NetworkAccess{Applications: []string{"222222222222222222222222"}, Ports: []store.PublishedPort{{Protocol: "TCP", HostPort: 15432, ContainerPort: 5432}}}
	if err := ValidateNetworkAccess(app, &valid); err != nil {
		t.Fatal(err)
	}
	if valid.Ports[0].Protocol != "tcp" || valid.Ports[0].BindIP != "0.0.0.0" || valid.Applications != nil {
		t.Fatal(valid)
	}
	for _, bad := range []store.NetworkAccess{
		{Ports: []store.PublishedPort{{Protocol: "sctp", HostPort: 10000, ContainerPort: 1}}},
		{Ports: []store.PublishedPort{{Protocol: "tcp", BindIP: "host", HostPort: 10000, ContainerPort: 1}}},
		{Ports: []store.PublishedPort{{Protocol: "tcp", HostPort: 80, ContainerPort: 1}}},
		{Ports: []store.PublishedPort{{Protocol: "tcp", HostPort: 10000, ContainerPort: 0}}},
		{Ports: []store.PublishedPort{{Protocol: "tcp", HostPort: 10000, ContainerPort: 1}, {Protocol: "tcp", HostPort: 10000, ContainerPort: 2}}},
	} {
		if err := ValidateNetworkAccess(app, &bad); err == nil {
			t.Fatalf("accepted unsafe networking: %+v", bad)
		}
	}
}
func TestNetworkChangeReusesImage(t *testing.T) {
	old := testConfig()
	next := old
	next.Networking = store.NetworkAccess{Applications: []string{"222222222222222222222222"}, Ports: []store.PublishedPort{{Protocol: "tcp", BindIP: "127.0.0.1", HostPort: 12345, ContainerPort: 8080}}}
	plan := PlanChanges(old, next)
	if plan.Operation != "recreate" {
		t.Fatalf("%+v", plan)
	}
}

func TestDomainlessApplicationsKeepPrivateNames(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	a := store.Application{ID: "database", Name: "Database", Config: store.Config{Method: "image", Image: "mongo:8", Database: &store.DatabaseResource{Engine: "mongodb", Version: "8"}}}
	d, err := db.Enqueue(ctx, a, store.Deployment{ID: "release", Operation: "deploy"}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	d.State = "succeeded"
	if err = db.SaveDeployment(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err = db.SetActive(ctx, a.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	runtime := &fakeRuntime{}
	w := NewWorker(db, runtime, events.New(), dir)
	if err = w.routes(ctx, "", nil); err != nil {
		t.Fatal(err)
	}
	// A database has no public domain, yet the runtime must still give it db-<id>.
	if runtime.active[a.ID].ID != d.ID {
		t.Fatalf("domainless application missing from alias reconciliation: %+v", runtime.active)
	}
	if routes := Routes([]store.Application{{ID: a.ID, Domains: []store.Domain{}}}, runtime.active)["http"].(map[string]any); routes["routers"] != nil {
		t.Fatalf("domainless application received a public router: %+v", routes)
	}
}
