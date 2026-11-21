package deploy

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"shipper/internal/store"
)

func TestNetworkCutoverKeepsUnpublishedServingReleaseRunning(t *testing.T) {
	for _, test := range []struct {
		name      string
		published uint16
		database  bool
		stops     int
	}{
		{name: "image exposes a private HTTP port"},
		{name: "release publishes a host port", published: 19001, stops: 1},
		{name: "database must release its data volume", database: true, stops: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			instance, app := strings.Repeat("a", 64), strings.Repeat("b", 24)
			var stops atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/containers/json"):
					json.NewEncoder(w).Encode([]map[string]any{{"Id": "previous", "State": "running", "Labels": map[string]string{"io.shipper.instance": instance, "io.shipper.application.id": app}, "Ports": []map[string]any{{"PrivatePort": 8080, "PublicPort": test.published, "Type": "tcp"}}}})
				case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/containers/previous/stop"):
					stops.Add(1)
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
			d := store.Deployment{ApplicationID: app, PreviousID: "previous"}
			if test.database {
				d.Config.Database = &store.DatabaseResource{Engine: "redis"}
			}
			if err = runtime.PrepareNetworkCutover(context.Background(), d); err != nil {
				t.Fatal(err)
			}
			if int(stops.Load()) != test.stops {
				t.Fatalf("Serving release stopped %d times, expected %d", stops.Load(), test.stops)
			}
		})
	}
}

func TestPublishedPortsAreDeclaredToOlderDockerAPIs(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	instance := strings.Repeat("a", 64)
	app := strings.Repeat("b", 24)
	var created atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/containers/json"):
			w.Write([]byte(`[]`))
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/networks/"):
			json.NewEncoder(w).Encode(map[string]any{"Id": "network", "Labels": map[string]string{"io.shipper.instance": instance, "io.shipper.resource": "private-network"}})
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/containers/create"):
			var request container.CreateRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			bound := network.MustParsePort("9001/tcp")
			if _, ok := request.ExposedPorts[bound]; !ok {
				t.Error("A host binding was sent without an exposed container port")
				w.WriteHeader(400)
				return
			}
			if len(request.ExposedPorts) != 1 || len(request.HostConfig.PortBindings) != 1 {
				t.Error("Unexpected port was exposed or published")
			}
			created.Store(true)
			w.WriteHeader(201)
			w.Write([]byte(`{"Id":"created"}`))
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/start"):
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"message":"not found"}`))
		}
	}))
	defer server.Close()
	cli, err := client.New(client.WithHost(server.URL), client.WithAPIVersion("1.44"))
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	runtime := &DockerRuntime{Client: cli, Instance: instance}
	d := store.Deployment{ID: "release", ApplicationID: app, ImageID: "alpine:3.22", ResolvedMethod: "image", Config: store.Config{Method: "image", Image: "alpine:3.22", StartupPolicy: "process", Networking: store.NetworkAccess{Ports: []store.PublishedPort{{Protocol: "tcp", BindIP: "127.0.0.1", HostPort: port, ContainerPort: 9001}}}}}
	if _, err = runtime.Start(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if !created.Load() {
		t.Fatal("Runtime was not created")
	}
}
