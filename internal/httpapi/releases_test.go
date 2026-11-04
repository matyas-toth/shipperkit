package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type releaseAPIRuntime struct {
	deploy.Runtime
	missing bool
}

func (r *releaseAPIRuntime) CheckRelease(context.Context, store.Deployment) error {
	if r.missing {
		return errors.New("A recorded image is unavailable")
	}
	return nil
}

func TestRollbackReviewOwnershipAndImmutableEnqueue(t *testing.T) {
	ctx := context.Background()
	db, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	hub := events.New()
	runtime := &releaseAPIRuntime{}
	worker := deploy.NewWorker(db, runtime, hub, t.TempDir())
	setup := auth.Token()
	server := httptest.NewServer(New(db, inventory.New(testEngine{}, db, hub), hub, Config{SetupToken: setup, Worker: worker, Assets: http.NotFoundHandler()}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	call := func(path string, input any) (int, []byte) {
		t.Helper()
		body, _ := json.Marshal(input)
		req, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var data json.RawMessage
		json.NewDecoder(res.Body).Decode(&data)
		return res.StatusCode, data
	}
	code, raw := call("/api/auth/setup", map[string]string{"email": "owner@example.test", "password": "release-review-test-password", "setupToken": setup})
	if code != 201 {
		t.Fatal(code, string(raw))
	}
	var session struct {
		CSRF string `json:"csrfToken"`
	}
	json.Unmarshal(raw, &session)
	csrf = session.CSRF
	a := store.Application{ID: "app", Name: "App", Config: store.Config{Method: "image", Image: "httpd:alpine", Port: 80, Domain: "app.localhost", Variables: []store.Variable{{Name: "SECRET", Value: "historical-secret", Secret: true}}}}
	old, e := db.Enqueue(ctx, a, store.Deployment{ID: "old", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	old.State = "succeeded"
	old.ImageID = "sha256:old"
	old.ResolvedMethod = "image"
	db.SaveDeployment(ctx, old)
	db.SetActive(ctx, a.ID, old.ID)
	a, _ = db.Application(ctx, a.ID)
	a.Config.Image = "httpd:new"
	current, e := db.Enqueue(ctx, a, store.Deployment{ID: "current", Operation: "deploy"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	current.State = "succeeded"
	current.ImageID = "sha256:new"
	db.SaveDeployment(ctx, current)
	db.SetActive(ctx, a.ID, current.ID)
	path := "/api/applications/app/deployments/old/rollback"
	code, raw = call(path, map[string]any{"review": true})
	if code != 200 || strings.Contains(string(raw), "historical-secret") {
		t.Fatal(code, string(raw))
	}
	var plan deploy.ChangePlan
	json.Unmarshal(raw, &plan)
	code, _ = call("/api/applications/other/deployments/old/rollback", map[string]any{"review": true})
	if code != 404 {
		t.Fatal("cross application access", code)
	}
	runtime.missing = true
	code, _ = call(path, map[string]any{"review": true})
	if code != 409 {
		t.Fatal("missing artifact", code)
	}
	runtime.missing = false
	code, _ = call(path, map[string]any{"revision": "stale"})
	if code != 409 {
		t.Fatal("stale review", code)
	}
	code, raw = call(path, map[string]any{"revision": plan.Revision})
	if code != 202 {
		t.Fatal(code, string(raw))
	}
	var queued store.Deployment
	json.Unmarshal(raw, &queued)
	persisted, e := db.Deployment(ctx, queued.ID)
	if e != nil || persisted.RollbackID != old.ID || persisted.Config.Image != old.Config.Image || persisted.PreviousID != current.ID {
		t.Fatal(persisted, e)
	}
	if strings.Contains(string(raw), "historical-secret") {
		t.Fatal("secret in deployment response")
	}
}
