package deploy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/moby/moby/client"
	"net/http"
	"net/http/httptest"
	"shipper/internal/store"
	"strings"
	"testing"
)

func TestRegistryCredentialsAndDigestStayBoundToRequestedRepository(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/images/create") {
			raw, e := base64.URLEncoding.DecodeString(r.Header.Get("X-Registry-Auth"))
			if e != nil {
				t.Error(e)
			}
			var auth map[string]string
			json.Unmarshal(raw, &auth)
			if auth["serveraddress"] != "ghcr.io" || auth["username"] != "owner" || auth["password"] != "registry-secret" {
				t.Error("Incorrect registry credential scope")
			}
			w.Write([]byte(`{"status":"Download complete"}`))
			return
		}
		if strings.Contains(r.URL.Path, "/images/") && strings.HasSuffix(r.URL.Path, "/json") {
			json.NewEncoder(w).Encode(map[string]any{"Id": "sha256:fixture", "RepoDigests": []string{"other.example.com/foreign@" + digest, "ghcr.io/owner/app@" + digest}, "Config": map[string]any{}})
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	cli, e := client.New(client.WithHost(server.URL), client.WithAPIVersion("1.54"))
	if e != nil {
		t.Fatal(e)
	}
	defer cli.Close()
	r := &DockerRuntime{Client: cli}
	d := store.Deployment{Config: store.Config{Method: "image", Image: "ghcr.io/owner/app:latest"}}
	c := store.Connection{Kind: "registry", Host: "ghcr.io", Secret: store.ConnectionSecret{Username: "owner", Password: "registry-secret"}}
	if e = r.PullImage(context.Background(), &d, &c, func(line string) {
		if strings.Contains(line, "registry-secret") {
			t.Error("Credential in pull log")
		}
	}); e != nil {
		t.Fatal(e)
	}
	if d.Digest != "ghcr.io/owner/app@"+digest || d.ImageID != "sha256:fixture" {
		t.Fatal("Wrong repository identity recorded", d.Digest)
	}
	before := calls
	c.Host = "other.example.com"
	if e = r.PullImage(context.Background(), &d, &c, func(string) {}); e == nil || calls != before {
		t.Fatal("Credentials reached a different registry")
	}
}
