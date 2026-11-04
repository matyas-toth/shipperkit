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
	"shipper/internal/events"
	"shipper/internal/inventory"
	"shipper/internal/store"
)

func TestProtectionAuthenticationSecretsAndReview(t *testing.T) {
	db, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	hub := events.New()
	worker := deploy.NewWorker(db, nil, hub, t.TempDir())
	server := httptest.NewServer(New(db, inventory.New(testEngine{}, db, hub), hub, Config{SetupToken: "test-setup", Worker: worker, Assets: http.NotFoundHandler()}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	call := func(method, path string, body any) (int, string) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+"/api"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if code, _ := call("GET", "/protection", nil); code != 401 {
		t.Fatal("anonymous recovery access", code)
	}
	code, body := call("POST", "/auth/setup", map[string]string{"email": "owner@example.test", "password": "protection-test-password", "setupToken": "test-setup"})
	if code != 201 {
		t.Fatal(code, body)
	}
	var session struct {
		CSRF string `json:"csrfToken"`
	}
	json.Unmarshal([]byte(body), &session)
	csrf = session.CSRF
	d := store.BackupDestination{Name: "Off server", Endpoint: "https://s3.example.test", Region: "us-east-1", Bucket: "bucket", AccessKey: "private-access-value", SecretKey: "private-secret-value"}
	if code, body = call("POST", "/protection/destinations", d); code != 200 || strings.Contains(body, d.SecretKey) || strings.Contains(body, d.AccessKey) {
		t.Fatal("destination secrets escaped", code, body)
	}
	if code, body = call("GET", "/protection", nil); code != 200 || strings.Contains(body, d.SecretKey) || strings.Contains(body, "AGE-SECRET-KEY") {
		t.Fatal("private key or credentials in ordinary response", code, body)
	}
	if code, _ = call("GET", "/protection/key", nil); code != 405 {
		t.Fatal("private recovery identity has a GET endpoint", code)
	}
	savedCSRF := csrf
	csrf = ""
	if code, _ = call("POST", "/protection/key", map[string]string{"confirm": "Download recovery key"}); code != 403 {
		t.Fatal("key download allowed without CSRF", code)
	}
	csrf = savedCSRF
	if code, body = call("POST", "/protection/key", map[string]string{"confirm": "Download recovery key"}); code != 200 || !strings.Contains(body, "AGE-SECRET-KEY") {
		t.Fatal("explicit recovery download failed", code, body)
	}
	ctx := context.Background()
	a := store.Application{ID: "protected-app", Name: "Protected", Config: store.Config{Method: "image", Image: "postgres:17"}}
	dep, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	dep.State = "succeeded"
	db.SaveDeployment(ctx, dep)
	v, e := db.QueueBackup(ctx, store.BackupRun{ApplicationID: a.ID, Operation: "backup", Config: store.BackupSpec{Kind: "database", Engine: "postgres", Database: "original", User: "postgres", Password: "saved-password"}})
	if e != nil {
		t.Fatal(e)
	}
	v.State = "succeeded"
	db.SaveBackupRun(ctx, v)
	path := "/applications/" + a.ID + "/protection/runs/" + v.ID + "/restore"
	if code, body = call("POST", path, map[string]any{"review": true}); code != 200 || strings.Contains(body, "saved-password") {
		t.Fatal("restore review leaked credentials", code, body)
	}
	var review struct {
		Revision string `json:"revision"`
	}
	json.Unmarshal([]byte(body), &review)
	if code, _ = call("POST", path, map[string]string{"target": "copy", "confirm": a.Name, "revision": "stale"}); code != 409 {
		t.Fatal("stale restore accepted", code)
	}
	if code, _ = call("POST", path, map[string]string{"target": "original", "confirm": a.Name, "revision": review.Revision}); code != 422 {
		t.Fatal("in-place restore accepted", code)
	}
	if code, _ = call("POST", path, map[string]string{"target": "copy", "confirm": a.Name, "revision": review.Revision}); code != 202 {
		t.Fatal("reviewed restore was not queued", code)
	}
}
