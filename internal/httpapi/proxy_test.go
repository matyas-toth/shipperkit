package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/events"
	"shipper/internal/inventory"
	"shipper/internal/operations"
	"shipper/internal/store"
)

func TestDashboardDomainSetting(t *testing.T) {
	ctx := context.Background()
	db, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	hub := events.New()
	worker := deploy.NewWorker(db, nil, hub, t.TempDir())
	monitor := inventory.New(testEngine{}, db, hub)
	effective := func(ctx context.Context) (deploy.ProxyConfig, error) {
		domain, _ := db.Setting(ctx, "dashboard_domain", "")
		config := deploy.ProxyConfig{AutoHTTPS: true}
		if domain != "" {
			config.DashboardURL = "https://" + domain
			config.DashboardUpstream = "http://dashboard:8080"
		}
		return config, nil
	}
	setupToken := auth.Token()
	server := httptest.NewServer(New(db, monitor, hub, Config{SetupToken: setupToken, Assets: http.NotFoundHandler(), Worker: worker, Operations: &operations.Service{DB: db}, Proxy: deploy.ProxyConfig{AutoHTTPS: true}, EffectiveProxy: effective}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	request := func(method, path string, input any) (int, []byte) {
		raw, _ := json.Marshal(input)
		req, _ := http.NewRequest(method, server.URL+"/api"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var body json.RawMessage
		json.NewDecoder(res.Body).Decode(&body)
		return res.StatusCode, body
	}
	code, raw := request("POST", "/auth/setup", map[string]string{"email": "owner@example.test", "password": "strong-dashboard-password", "setupToken": setupToken})
	if code != 201 {
		t.Fatal(code, string(raw))
	}
	var session struct {
		CSRF string `json:"csrfToken"`
	}
	json.Unmarshal(raw, &session)
	csrf = session.CSRF

	cfg := store.Config{Method: "static", Repository: "https://github.com/example/app.git", Branch: "main", PublishDirectory: ".", Routing: "files", Domain: "app.example.test"}
	application := store.Application{ID: "app", Name: "App", Config: cfg}
	active, e := db.Enqueue(ctx, application, store.Deployment{ID: "active", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	active.State = "succeeded"
	active.ImageID = "sha256:fixture"
	if e = db.SaveDeployment(ctx, active); e != nil {
		t.Fatal(e)
	}
	db.SetActive(ctx, application.ID, active.ID)

	if code, raw = request("POST", "/proxy", map[string]string{"domain": "app.example.test"}); code != 409 {
		t.Fatal("application domain was accepted for the dashboard", code, string(raw))
	}
	if code, raw = request("POST", "/proxy", map[string]string{"domain": "shipper.example.test"}); code != 202 {
		t.Fatal(code, string(raw))
	}
	domain, e := db.Setting(ctx, "dashboard_domain", "")
	if e != nil || domain != "shipper.example.test" {
		t.Fatal(domain, e)
	}
	code, raw = request("GET", "/proxy", nil)
	if code != 200 || !strings.Contains(string(raw), `"dashboardURL":"https://shipper.example.test"`) {
		t.Fatal(code, string(raw))
	}
	if code, raw = request("POST", "/proxy", map[string]string{"domain": ""}); code != 409 {
		t.Fatal("maintenance conflict accepted a second change", code, string(raw))
	}
	domain, _ = db.Setting(ctx, "dashboard_domain", "")
	if domain != "shipper.example.test" {
		t.Fatal("maintenance conflict changed the dashboard domain", domain)
	}
	runs, e := db.MaintenanceRuns(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, run := range runs {
		if run.State == "queued" {
			run.State = "succeeded"
			if e = db.SaveMaintenance(ctx, run); e != nil {
				t.Fatal(e)
			}
		}
	}
	if code, raw = request("POST", "/proxy", map[string]string{"domain": ""}); code != 202 {
		t.Fatal(code, string(raw))
	}
	domain, _ = db.Setting(ctx, "dashboard_domain", "")
	if domain != "" {
		t.Fatal("dashboard domain was not cleared", domain)
	}
}
