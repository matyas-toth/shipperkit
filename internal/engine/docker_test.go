package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

func TestOfficialSDKInventoryAndEvents(t *testing.T) {
	event := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/_ping") {
			w.Header().Set("API-Version", "1.56")
			w.Write([]byte("OK"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(path, "/info"):
			json.NewEncoder(w).Encode(map[string]any{"Name": "test-engine", "ServerVersion": "29.8.2", "NCPU": 4, "MemTotal": 1024, "OperatingSystem": "Linux", "Architecture": "x86_64"})
		case strings.HasSuffix(path, "/containers/json"):
			if r.URL.Query().Get("all") != "1" {
				t.Error("stopped containers were not requested")
			}
			w.Write([]byte(`[
{"Id":"container-a","Names":["/api"],"Image":"api:v1","State":"running","Status":"Up (unhealthy)","Created":1,"Ports":[{"IP":"127.0.0.1","PrivatePort":80,"PublicPort":8080,"Type":"tcp"}],"Labels":{"io.shipper.managed":"true","io.shipper.application.id":"app-a","sensitive.label":"never-return"}},
{"Id":"container-b","Names":["/foreign"],"Image":"db:v1","State":"exited","Created":2,"Labels":{}},
{"Id":"shipper","Names":["/shipper"],"Image":"shipper","State":"running","Labels":{"io.shipper.control-plane":"true"}}
]`))
		case strings.HasSuffix(path, "/events"):
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-event:
			}
			w.Write([]byte("{\"Type\":\"container\",\"Action\":\"start\",\"Actor\":{\"ID\":\"container-a\"}}\n"))
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			t.Errorf("unexpected SDK path %s", path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cli, err := client.New(client.WithHost(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	docker := NewWithClient(cli)
	defer docker.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	changes, errs := docker.Subscribe(ctx)
	snapshot, err := docker.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Containers) != 2 || snapshot.Containers[0].Health != "unhealthy" || snapshot.Containers[0].Ports[0].IP != "127.0.0.1" {
		t.Fatalf("unexpected projection: %+v", snapshot)
	}
	data, _ := json.Marshal(snapshot)
	if strings.Contains(string(data), "sensitive.label") {
		t.Fatal("Docker labels escaped the projection")
	}
	event <- struct{}{}
	select {
	case <-changes:
	case err := <-errs:
		t.Fatalf("stream error: %v", err)
	case <-ctx.Done():
		t.Fatal("SDK event was not received")
	}
}

func TestRejectsPlaintextRemoteDocker(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://127.0.0.1:2375")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	if _, err := New(); err == nil {
		t.Fatal("accepted unverified remote Docker")
	}
}
