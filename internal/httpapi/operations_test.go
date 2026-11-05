package httpapi

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/events"
	"shipper/internal/inventory"
	"shipper/internal/store"
)

func TestOperationsCapabilitiesSourcesAndBoundReviews(t *testing.T) {
	ctx := context.Background()
	db, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	cookie := auth.Token()
	if e = db.CreateAdmin(ctx, store.User{ID: "owner", Email: "owner@example.test"}, auth.Hash(cookie), "csrf", time.Now().Add(time.Hour).Unix()); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"one", "two"} {
		d, e := db.Enqueue(ctx, store.Application{ID: id, Name: id, Config: store.Config{Hooks: []store.Hook{{Stage: "after_deploy", Command: store.Command{Shell: "echo private-command-body"}}}, Method: "static", Repository: "https://github.com/mdn/beginner-html-site.git", PublishDirectory: ".", Domain: id + ".localhost"}}, store.Deployment{ID: "release-" + id, Operation: "deploy"}, "", true)
		if e != nil {
			t.Fatal(e)
		}
		d.State = "succeeded"
		db.SaveDeployment(ctx, d)
		db.SetActive(ctx, id, d.ID)
	}
	hub := events.New()
	monitor := inventory.New(testEngine{}, db, hub)
	worker := deploy.NewWorker(db, nil, hub, t.TempDir())
	server := httptest.NewServer(New(db, monitor, hub, Config{Worker: worker, Assets: http.NotFoundHandler()}))
	defer server.Close()
	call := func(method, path, token string, body any) (int, string) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
			req.Header.Set("X-CSRF-Token", "csrf")
		}
		res, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(data)
	}
	if code, body := call("GET", "/api/applications/one", "", nil); code != 200 || !strings.Contains(body, "private-command-body") {
		t.Fatal("Command projection fixture is missing", code, body)
	}
	code, body := call("POST", "/api/operation-tokens", "", map[string]any{"name": "Reader", "scopes": []string{"read"}, "applications": []string{"one"}, "days": 1})
	if code != 201 {
		t.Fatal(code, body)
	}
	var created struct {
		Token      string               `json:"token"`
		Credential store.OperationToken `json:"credential"`
	}
	json.Unmarshal([]byte(body), &created)
	code, body = call("GET", "/api/v1/applications", created.Token, nil)
	if code != 200 || strings.Contains(body, `"id":"two"`) || !strings.Contains(body, `"id":"one"`) {
		t.Fatal(code, body)
	}
	for _, path := range []string{"/api/v1/applications/two", "/api/v1/applications/one/logs", "/api/v1/applications/one/diagnostics?include=logs", "/api/v1/operation-tokens", "/api/v1/system/operations", "/api/v1/events"} {
		if code, body = call("GET", path, created.Token, nil); code != 403 {
			t.Fatal(path, code, body)
		}
	}
	for _, path := range []string{"/api/v1/applications", "/api/v1/applications/one/deployments", "/api/v1/applications/one/variables/reveal", "/api/v1/applications/one/delete", "/api/v1/system/retention", "/api/v1/sources"} {
		if code, body = call("POST", path, created.Token, map[string]any{}); code != 403 {
			t.Fatal(path, code, body)
		}
	}
	code, body = call("GET", "/api/v1/applications/one/diagnostics", created.Token, nil)
	if code != 200 || !strings.Contains(body, "servingRelease") {
		t.Fatal(code, body)
	}
	if strings.Contains(body, "private-command-body") {
		t.Fatal("Read token exposed command bodies")
	}
	if code, body = call("POST", "/api/operation-tokens/"+created.Credential.ID+"/revoke", "", map[string]any{}); code != 200 {
		t.Fatal(code, body)
	}
	if code, _ = call("GET", "/api/v1/capabilities", created.Token, nil); code != 401 {
		t.Fatal("Revoked token accepted", code)
	}
	var tarBody bytes.Buffer
	tw := tar.NewWriter(&tarBody)
	tw.WriteHeader(&tar.Header{Name: "index.html", Mode: 0644, Size: 5})
	tw.Write([]byte("hello"))
	tw.Close()
	req, _ := http.NewRequest("POST", server.URL+"/api/sources", &tarBody)
	req.Header.Set("Content-Type", "application/x-tar")
	req.Header.Set("X-CSRF-Token", "csrf")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
	res, e := server.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	var source store.SourceSnapshot
	json.NewDecoder(res.Body).Decode(&source)
	res.Body.Close()
	if res.StatusCode != 201 || source.ID == "" {
		t.Fatal(res.StatusCode, source)
	}
	config := store.Config{Method: "static", LocalSource: source.ID, PublishDirectory: ".", Routing: "files"}
	code, body = call("POST", "/api/plan", "", map[string]any{"name": "Local project", "config": config})
	if code != 200 {
		t.Fatal(code, body)
	}
	var plan struct {
		ApplicationID string `json:"applicationId"`
		Revision      string `json:"revision"`
	}
	json.Unmarshal([]byte(body), &plan)
	config.BuildCommand = "echo changed"
	code, body = call("POST", "/api/applications", "", map[string]any{"id": plan.ApplicationID, "name": "Local project", "revision": plan.Revision, "config": config})
	if code != 409 {
		t.Fatal("Changed source inputs were not rejected as stale", code, body)
	}
	config.BuildCommand = ""
	code, body = call("POST", "/api/applications", "", map[string]any{"id": plan.ApplicationID, "name": "Local project", "revision": plan.Revision, "config": config})
	if code != 202 {
		t.Fatal(code, body)
	}
	bound, e := db.SourceSnapshot(ctx, source.ID)
	if e != nil || bound.ApplicationID != plan.ApplicationID {
		t.Fatal("Snapshot was not bound to its application", bound, e)
	}
	code, body = call("POST", "/api/operation-tokens", "", map[string]any{"name": "App deployer", "scopes": []string{"read", "deploy"}, "applications": []string{plan.ApplicationID}, "days": 1})
	if code != 201 {
		t.Fatal(code, body)
	}
	var deployer struct {
		Token string `json:"token"`
	}
	json.Unmarshal([]byte(body), &deployer)
	current, e := db.Application(ctx, plan.ApplicationID)
	if e != nil {
		t.Fatal(e)
	}
	code, body = call("POST", "/api/v1/applications/"+plan.ApplicationID+"/changes", deployer.Token, map[string]any{"config": current.Config})
	if code != 200 {
		t.Fatal("Scoped token could not review its bound source", code, body)
	}
	current.Config.SourceID = "another-private-connection"
	code, body = call("POST", "/api/v1/applications/"+plan.ApplicationID+"/changes", deployer.Token, map[string]any{"config": current.Config})
	if code != 422 {
		t.Fatal("Scoped token changed private connection", code, body)
	}
}
