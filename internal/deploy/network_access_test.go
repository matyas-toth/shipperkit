package deploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"

	"github.com/moby/moby/client"
	"shipper/internal/store"
	"testing"
)

func TestDeletedApplicationReleasesOnlyItsUnusedNetwork(t *testing.T) {
	for _, test := range []struct {
		name, endpoint string
		foreign        bool
		remove         bool
		disconnect     bool
	}{
		{name: "empty", remove: true},
		{name: "proxy only", endpoint: "proxy", remove: true, disconnect: true},
		{name: "connected application", endpoint: "consumer"},
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
	if valid.Ports[0].Protocol != "tcp" || valid.Ports[0].BindIP != "0.0.0.0" {
		t.Fatal(valid)
	}
	for _, bad := range []store.NetworkAccess{
		{Applications: []string{app}}, {Applications: []string{"host"}},
		{Applications: []string{"222222222222222222222222", "222222222222222222222222"}},
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
