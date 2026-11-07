package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"shipper/internal/auth"
	"shipper/internal/events"
	"shipper/internal/inventory"
	"shipper/internal/store"
	"strings"
	"testing"
	"time"
)

func TestDatabaseConnectionSecretBoundary(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cookie := auth.Token()
	if err = db.CreateAdmin(ctx, store.User{ID: "owner", Email: "owner@example.test"}, auth.Hash(cookie), "csrf", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	app := store.Application{ID: "db", Name: "Database", Config: store.Config{Method: "image", Image: "postgres:17", Variables: []store.Variable{{Name: "POSTGRES_PASSWORD", Value: "active-password", Secret: true, Scope: "runtime"}}}}
	d, err := db.Enqueue(ctx, app, store.Deployment{ID: "active-release", Operation: "deploy"}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	d.State = "succeeded"
	d.ContainerID = "container"
	d.ResolvedMethod = "image"
	if err = db.SaveDeployment(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err = db.SetActive(ctx, app.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	app.Config.Variables[0].Value = "unapplied-password"
	if _, err = db.Enqueue(ctx, app, store.Deployment{ID: "pending-release", Operation: "deploy"}, "", false); err != nil {
		t.Fatal(err)
	}
	hub := events.New()
	server := httptest.NewServer(New(db, inventory.New(testEngine{}, db, hub), hub, Config{Assets: http.NotFoundHandler()}))
	defer server.Close()
	request := func(method, path, token, csrf string, owner bool) (int, string, http.Header) {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		if owner {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("X-CSRF-Token", csrf)
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body), res.Header
	}
	path := "/api/applications/db/database/connection"
	if status, _, _ := request("POST", path, "", "", false); status != 401 {
		t.Fatal("Unauthenticated secret read", status)
	}
	if status, _, _ := request("POST", path, "", "", true); status != 403 {
		t.Fatal("CSRF bypass", status)
	}
	if status, body, headers := request("POST", path, "", "csrf", true); status != 200 || !strings.Contains(body, "active-password") || strings.Contains(body, "unapplied-password") || headers.Get("Cache-Control") != "no-store" {
		t.Fatal("Active release secret response incorrect", status)
	}
	if status, body, _ := request("GET", "/api/applications/db", "", "", true); status != 200 || !strings.Contains(body, `"database":{"engine":"postgresql"`) || strings.Contains(body, "active-password") || strings.Contains(body, "unapplied-password") {
		t.Fatal("Secret in ordinary application response", status)
	}
	for _, tc := range []struct {
		scope, app string
		status     int
	}{{"read", "db", 403}, {"secrets", "another", 403}, {"secrets", "db", 200}} {
		token := auth.Token()
		if err = db.SaveOperationToken(ctx, store.OperationToken{ID: auth.Token()[:24], Name: "test", UserID: "owner", Hash: auth.Hash(token), Scopes: []string{tc.scope}, Applications: []string{tc.app}, ExpiresAt: time.Now().Add(time.Hour).Unix()}); err != nil {
			t.Fatal(err)
		}
		if status, _, _ := request("POST", "/api/v1/applications/db/database/connection", token, "", false); status != tc.status {
			t.Fatal("Incorrect secrets capability enforcement", tc.scope, tc.app, status)
		}
	}
}
