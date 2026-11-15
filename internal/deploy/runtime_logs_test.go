package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/moby/moby/client"
)

func TestRuntimeLogSnapshotDoesNotFollowRunningContainers(t *testing.T) {
	for _, follow := range []bool{false, true} {
		t.Run(map[bool]string{false: "snapshot", true: "follow"}[follow], func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/json") {
					json.NewEncoder(w).Encode(map[string]any{"Id": "owned", "Config": map[string]any{"Tty": true, "Labels": map[string]string{"io.shipper.instance": "instance", "io.shipper.application.id": "app", "io.shipper.managed": "true"}}, "State": map[string]any{"Running": true}})
					return
				}
				if strings.HasSuffix(r.URL.Path, "/logs") {
					requests.Add(1)
					if r.URL.Query().Get("follow") != map[bool]string{false: "", true: "1"}[follow] || r.URL.Query().Get("tail") != "2" {
						t.Errorf("wrong log request: %s", r.URL.RawQuery)
					}
					w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
					w.Write([]byte("latest one\nlatest two\n"))
					return
				}
				w.WriteHeader(404)
			}))
			defer server.Close()
			cli, err := client.New(client.WithHost(server.URL), client.WithAPIVersion("1.54"))
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()
			runtime := &DockerRuntime{Client: cli, Instance: "instance"}
			var lines []string
			read := runtime.LogSnapshot
			if follow {
				read = runtime.Logs
			}
			err = read(context.Background(), "app", "owned", 2, func(line string) error { lines = append(lines, line); return nil })
			if err != nil && !errors.Is(err, io.EOF) {
				t.Fatal(err)
			}
			if requests.Load() != 1 || strings.Join(lines, "|") != "latest one|latest two" {
				t.Fatalf("wrong snapshot: %v", lines)
			}
		})
	}
}
