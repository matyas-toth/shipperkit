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
			json.NewEncoder(w).Encode(map[string]any{"Id": "network", "Labels": map[string]string{"io.shipper.instance": instance, "io.shipper.application.id": app}})
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
