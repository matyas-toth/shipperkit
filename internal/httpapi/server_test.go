package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/inventory"
	"shipper/internal/store"
)

type testEngine struct{}

type testLogRuntime struct{ deploy.Runtime }

func (testLogRuntime) Logs(ctx context.Context, appID, containerID string, tail int, emit func(string) error) error {
	return emit("runtime-api-private-value is masked")
}

func (testEngine) Snapshot(context.Context) (engine.Snapshot, error) {
	return engine.Snapshot{Containers: []store.Container{{ID: "abc", Name: "api", State: "running", Image: "test:v1", Ports: []store.Port{}, Managed: true, ApplicationID: "app-api"}, {ID: "external", Name: "unrelated", State: "running", Ports: []store.Port{}}}, Info: engine.Info{Name: "fixture"}}, nil
}
func (testEngine) Subscribe(context.Context) (<-chan struct{}, <-chan error) {
	return make(chan struct{}), make(chan error)
}

func TestAuthenticationCSRFAndReadOnlyAPI(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hub := events.New()
	monitor := inventory.New(testEngine{}, db, hub)
	if err = monitor.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	token := auth.Token()
	worker := deploy.NewWorker(db, nil, hub, t.TempDir())
	handler := New(db, monitor, hub, Config{SetupToken: token, Version: "test", Assets: http.NotFoundHandler(), Worker: worker})
	server := httptest.NewServer(handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	request := func(method, path string, body any, csrf, origin string) (*http.Response, map[string]any) {
		t.Helper()
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		r, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(data))
		if method != "GET" {
			r.Header.Set("Content-Type", "application/json")
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out := map[string]any{}
		json.NewDecoder(resp.Body).Decode(&out)
		return resp, out
	}
	health, healthBody := request("GET", "/api/health", nil, "", "")
	if health.StatusCode != 200 || healthBody["status"] != "ok" || healthBody["version"] != "test" {
		t.Fatalf("unexpected health response: %d %v", health.StatusCode, healthBody)
	}
	for _, path := range []string{"/api/containers", "/api/applications", "/api/system", "/api/events"} {
		resp, _ := request("GET", path, nil, "", "")
		if resp.StatusCode != 401 {
			t.Fatalf("%s allowed anonymous access", path)
		}
	}
	resp, body := request("GET", "/api/auth/session", nil, "", "")
	if body["setupRequired"] != true {
		t.Fatal(body)
	}
	input := map[string]string{"email": "owner@example.test", "password": "a-strong-test-password", "setupToken": token}
	resp, _ = request("POST", "/api/auth/setup", input, "", "https://evil.test")
	if resp.StatusCode != 403 {
		t.Fatal("cross-origin setup accepted")
	}
	wrong := map[string]string{"email": input["email"], "password": input["password"], "setupToken": "wrong"}
	resp, _ = request("POST", "/api/auth/setup", wrong, "", "")
	if resp.StatusCode != 403 {
		t.Fatal("setup token not required")
	}
	resp, body = request("POST", "/api/auth/setup", input, "", "")
	if resp.StatusCode != 201 {
		t.Fatalf("setup: %d %+v", resp.StatusCode, body)
	}
	csrf := body["csrfToken"].(string)
	cookie := resp.Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge <= 0 {
		t.Fatalf("insecure cookie: %+v", cookie)
	}
	if strings.Contains(string(mustJSON(body)), "password") {
		t.Fatal("password hash leaked")
	}
	resp, _ = request("POST", "/api/auth/setup", input, "", "")
	if resp.StatusCode != 409 {
		t.Fatal("setup was reusable")
	}
	resp, body = request("GET", "/api/containers", nil, "", "")
	if resp.StatusCode != 200 || len(body["items"].([]any)) != 1 {
		t.Fatal(body)
	}
	config := store.StaticConfig{Repository: "https://github.com/mdn/beginner-html-site.git", PublishDirectory: ".", Routing: "files", Variables: []store.Variable{{Name: "TOKEN", Value: "runtime-api-private-value", Secret: true, Scope: "runtime"}}}
	create := map[string]any{"name": "Website", "config": config}
	resp, _ = request("POST", "/api/applications", create, "", "")
	if resp.StatusCode != 403 {
		t.Fatal("deployment accepted without CSRF")
	}
	resp, body = request("POST", "/api/applications", create, csrf, "")
	if resp.StatusCode != 202 {
		t.Fatal("create", resp.StatusCode, body)
	}
	appID := body["applicationId"].(string)
	jobID := body["id"].(string)
	if strings.Contains(string(mustJSON(body)), "runtime-api-private-value") {
		t.Fatal("creation response exposed a secret")
	}
	resp, body = request("GET", "/api/applications/"+appID, nil, "", "")
	if resp.StatusCode != 200 || body["application"].(map[string]any)["name"] != "Website" {
		t.Fatal("application resource", body)
	}
	if strings.Contains(string(mustJSON(body)), "runtime-api-private-value") {
		t.Fatal("detail exposed a secret")
	}
	resp, body = request("POST", "/api/applications/"+appID+"/variables/reveal", map[string]string{"name": "TOKEN"}, csrf, "")
	if resp.StatusCode != 200 || body["value"] != "runtime-api-private-value" || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("explicit reveal failed", body)
	}
	worker.Runtime = testLogRuntime{}
	runtimeStream, err := client.Get(server.URL + "/api/applications/" + appID + "/logs?container=runtime-fixture")
	if err != nil {
		t.Fatal(err)
	}
	runtimeBody, err := io.ReadAll(runtimeStream.Body)
	runtimeStream.Body.Close()
	if err != nil || !strings.Contains(string(runtimeBody), "[REDACTED]") || strings.Contains(string(runtimeBody), "runtime-api-private-value") || !strings.Contains(string(runtimeBody), "event: end") {
		t.Fatal("runtime stream leaked or did not complete", string(runtimeBody), err)
	}
	resp, _ = request("POST", "/api/applications/"+appID+"/deployments", map[string]any{}, csrf, "")
	if resp.StatusCode != 409 {
		t.Fatal("duplicate operation accepted")
	}
	resp, _ = request("POST", "/api/applications/"+appID+"/delete", map[string]string{"confirmName": "wrong"}, csrf, "")
	if resp.StatusCode != 422 {
		t.Fatal("delete accepted without name confirmation")
	}
	job, _ := db.Deployment(context.Background(), jobID)
	job.State = "failed"
	db.SaveDeployment(context.Background(), job)
	resp, body = request("POST", "/api/applications/"+appID+"/delete", map[string]string{"confirmName": "Website"}, csrf, "")
	if resp.StatusCode != 202 || body["operation"] != "delete" {
		t.Fatal("delete did not enqueue", body)
	}
	resp, _ = request("POST", "/api/refresh", map[string]any{}, "", "")
	if resp.StatusCode != 403 {
		t.Fatal("refresh accepted without CSRF")
	}
	resp, _ = request("POST", "/api/auth/logout", map[string]any{}, "bad", "")
	if resp.StatusCode != 403 {
		t.Fatal("logout accepted invalid CSRF")
	}
	resp, _ = request("POST", "/api/auth/logout", map[string]any{}, csrf, "https://evil.test")
	if resp.StatusCode != 403 {
		t.Fatal("cross-origin logout accepted")
	}
	stream, err := client.Get(server.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 128)
	n, err := stream.Body.Read(buffer)
	stream.Body.Close()
	if err != nil || !strings.Contains(string(buffer[:n]), "event: inventory") {
		t.Fatalf("SSE did not deliver initial invalidation: %v", err)
	}
	resp, _ = request("POST", "/api/auth/logout", map[string]any{}, csrf, "")
	if resp.StatusCode != 200 {
		t.Fatal("logout failed")
	}
	resp, _ = request("GET", "/api/containers", nil, "", "")
	if resp.StatusCode != 401 {
		t.Fatal("revoked session still worked")
	}
	login := map[string]string{"email": input["email"], "password": input["password"]}
	resp, body = request("POST", "/api/auth/login", login, "", "")
	if resp.StatusCode != 200 {
		t.Fatalf("login failed: %+v", body)
	}
	user, _ := db.FindUser(context.Background(), input["email"])
	if !auth.Verify(user.PasswordHash, input["password"]) {
		t.Fatal("password hashing roundtrip failed")
	}
	if err = db.ResetPassword(context.Background(), input["email"], user.PasswordHash); err != nil {
		t.Fatal(err)
	}
	resp, _ = request("GET", "/api/containers", nil, "", "")
	if resp.StatusCode != 401 {
		t.Fatal("recovery did not revoke sessions")
	}
}

func mustJSON(value any) []byte { data, _ := json.Marshal(value); return data }
func TestOriginAndJSONRejection(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hub := events.New()
	monitor := inventory.New(testEngine{}, db, hub)
	handler := New(db, monitor, hub, Config{PublicURL: "https://shipper.example.test", SetupToken: auth.Token(), Assets: http.NotFoundHandler()})
	for _, tc := range []struct {
		contentType, origin string
		want                int
	}{
		{"text/plain", "", 415}, {"application/json", "https://other.example.test", 403},
		{"application/json", "https://shipper.example.test", 400},
	} {
		r := httptest.NewRequest("POST", "https://shipper.example.test/api/auth/login", strings.NewReader("invalid"))
		r.Header.Set("Content-Type", tc.contentType)
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("got %d want %d: %s", w.Code, tc.want, w.Body.String())
		}
	}
	for _, raw := range []string{"https://example.test/", "http://user@example.test", "ftp://example.test", "https://example.test/path", "https://example.test?query=1"} {
		if ValidatePublicURL(raw) == nil {
			t.Fatal("invalid origin accepted:", raw)
		}
	}
}

func TestRateLimitBoundsPasswordAttempts(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hub := events.New()
	handler := New(db, inventory.New(testEngine{}, db, hub), hub, Config{Assets: http.NotFoundHandler()})
	for i := 0; i < 11; i++ {
		r := httptest.NewRequest("POST", "http://localhost/api/auth/login", strings.NewReader("{}"))
		r.RemoteAddr = "127.0.0.1:9999"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if i == 10 && w.Code != 429 {
			t.Fatal("login attempts were not limited")
		}
	}
}
