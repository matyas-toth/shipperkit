package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/events"
	"shipper/internal/inventory"
	"shipper/internal/store"
	"strings"
	"testing"
)

func TestTaskAPIAndReviewedConfiguration(t *testing.T) {
	ctx := context.Background()
	db, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	hub := events.New()
	worker := deploy.NewWorker(db, nil, hub, t.TempDir())
	monitor := inventory.New(testEngine{}, db, hub)
	token := auth.Token()
	server := httptest.NewServer(New(db, monitor, hub, Config{SetupToken: token, Assets: http.NotFoundHandler(), Worker: worker}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	request := func(path string, input any) (int, []byte) {
		t.Helper()
		raw, _ := json.Marshal(input)
		req, _ := http.NewRequest("POST", server.URL+"/api"+path, bytes.NewReader(raw))
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
	code, raw := request("/auth/setup", map[string]string{"email": "owner@example.test", "password": "strong-task-api-password", "setupToken": token})
	if code != 201 {
		t.Fatal(code, string(raw))
	}
	var session struct {
		CSRF string `json:"csrfToken"`
	}
	json.Unmarshal(raw, &session)
	csrf = session.CSRF
	c := store.Config{Method: "dockerfile", Repository: "https://github.com/example/app.git", Domain: "app.localhost", Variables: []store.Variable{{Name: "TOKEN", Value: "private-api-value", Secret: true, Scope: "runtime"}}}
	if e = deploy.Validate(&c); e != nil {
		t.Fatal(e)
	}
	a := store.Application{ID: "app", Name: "App", Config: c}
	release, e := db.Enqueue(ctx, a, store.Deployment{ID: "release", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	release.State = "succeeded"
	release.ImageID = "sha256:fixture"
	db.SaveDeployment(ctx, release)
	db.SetActive(ctx, a.ID, release.ID)
	next := store.PublicConfig(c)
	next.Variables[0].Value = "new-private-api-value"
	next.Variables[0].Keep = false
	code, raw = request("/applications/app/changes", map[string]any{"config": next})
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	if strings.Contains(string(raw), "private-api-value") {
		t.Fatal("diff leaked secret")
	}
	var plan deploy.ChangePlan
	json.Unmarshal(raw, &plan)
	if plan.Operation != "recreate" || plan.Revision == "" {
		t.Fatal(plan)
	}
	changed := c
	changed.MemoryMB = 1024
	db.SaveConfig(ctx, a.ID, changed)
	code, raw = request("/applications/app/deployments", map[string]any{"config": next, "revision": plan.Revision})
	if code != 409 {
		t.Fatal("stale review was accepted", code, string(raw))
	}
	code, raw = request("/applications/app/changes", map[string]any{"config": next})
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	json.Unmarshal(raw, &plan)
	code, raw = request("/applications/app/deployments", map[string]any{"config": next, "revision": plan.Revision})
	if code != 202 {
		t.Fatal(code, string(raw))
	}
	var applied store.Deployment
	json.Unmarshal(raw, &applied)
	if applied.Operation != "apply" || applied.Actor != "owner@example.test" {
		t.Fatal(applied)
	}
	applied, _ = db.Deployment(ctx, applied.ID)
	applied.State = "failed"
	db.SaveDeployment(ctx, applied)
	task := store.Task{Name: "Cleanup", Enabled: true, Command: store.Command{Argv: []string{"/cleanup"}}, Schedule: "0 3 * * *", Timezone: "Europe/Budapest", Overlap: "queue", Missed: "once"}
	code, raw = request("/applications/app/tasks", task)
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	json.Unmarshal(raw, &task)
	if task.NextAt == 0 || task.Command.Timeout != 300 || task.ID == "" {
		t.Fatal(task)
	}
	code, _ = request("/applications/another/tasks/"+task.ID+"/run", map[string]any{})
	if code != 404 {
		t.Fatal("task escaped application", code)
	}
	code, raw = request("/applications/app/tasks/"+task.ID+"/run", map[string]any{})
	if code != 202 {
		t.Fatal(code, string(raw))
	}
	var operation store.Deployment
	json.Unmarshal(raw, &operation)
	if operation.TaskID != task.ID || operation.Actor != "owner@example.test" {
		t.Fatal(operation)
	}
	if strings.Contains(string(raw), "private-api-value") {
		t.Fatal("run response leaked configuration")
	}
	response, e := client.Get(server.URL + "/api/applications/app/deployments/" + operation.ID)
	if e != nil {
		t.Fatal(e)
	}
	var historical store.Deployment
	json.NewDecoder(response.Body).Decode(&historical)
	response.Body.Close()
	if response.StatusCode != 200 || historical.ID != operation.ID || historical.Actor != "owner@example.test" {
		t.Fatal("individual operation lookup failed", historical)
	}
	response, e = client.Get(server.URL + "/api/applications/another/deployments/" + operation.ID)
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal("deployment lookup escaped application")
	}
	csrf = "invalid"
	code, _ = request("/applications/app/tasks", task)
	if code != 403 {
		t.Fatal("task mutation did not require CSRF", code)
	}
}
