package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"shipper/internal/auth"
	"shipper/internal/events"
	"shipper/internal/inventory"
	"shipper/internal/store"
)

func TestSetupRestoreGuards(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	hub := events.New()
	monitor := inventory.New(testEngine{}, db, hub)
	server := httptest.NewServer(New(db, monitor, hub, Config{SetupToken: "setup-token", DataDir: dir, Assets: http.NotFoundHandler()}))
	defer server.Close()

	call := func(token, kind, key, body string) (int, string) {
		t.Helper()
		request, _ := http.NewRequest("POST", server.URL+"/api/auth/restore", bytes.NewReader([]byte(body)))
		request.Header.Set("Content-Type", kind)
		if token != "" {
			request.Header.Set("X-Setup-Token", token)
		}
		if key != "" {
			request.Header.Set("X-Recovery-Key", key)
		}
		response, e := server.Client().Do(request)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		var result bytes.Buffer
		_, _ = result.ReadFrom(response.Body)
		return response.StatusCode, result.String()
	}
	if code, body := call("wrong", "application/octet-stream", "key", "archive"); code != 403 {
		t.Fatal(code, body)
	}
	if code, body := call("setup-token", "application/json", "key", "{}"); code != 415 {
		t.Fatal(code, body)
	}
	if code, body := call("setup-token", "application/octet-stream", "", "archive"); code != 422 || !strings.Contains(body, "recovery key") {
		t.Fatal(code, body)
	}
	if code, body := call("setup-token", "application/octet-stream", "key", "not an archive"); code != 422 || !strings.Contains(body, "invalid_archive") {
		t.Fatal(code, body)
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".restore-") {
			t.Fatal("failed validation left a staging directory behind")
		}
	}

	cookie := auth.Token()
	if e = db.CreateAdmin(ctx, store.User{ID: "owner", Email: "owner@example.test"}, auth.Hash(cookie), "csrf", time.Now().Add(time.Hour).Unix()); e != nil {
		t.Fatal(e)
	}
	if code, body := call("setup-token", "application/octet-stream", "key", "archive"); code != 409 {
		t.Fatal(code, body)
	}
	if _, e = os.Stat(filepath.Join(dir, "restore-pending.json")); !os.IsNotExist(e) {
		t.Fatal("configured instance queued a restore")
	}
}
