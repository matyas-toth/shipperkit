package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
	"shipper/internal/updates"
)

func TestUpdateAPI(t *testing.T) {
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
	feed := &updateFeed{main: "1.0.0-rc.9", nightly: "1.0.0-rc.9.nightly.4"}
	serverFeed := httptest.NewServer(feed)
	defer serverFeed.Close()
	dataDir := t.TempDir()
	service := updates.New(updates.Config{Store: db, DataDir: dataDir, CurrentVersion: "1.0.0-rc.8",
		MainFeed: serverFeed.URL + "/main", NightlyFeed: serverFeed.URL + "/nightly",
		Enabled: true, Ready: func() bool { return true }})
	hub := events.New()
	server := httptest.NewServer(New(db, inventory.New(testEngine{}, db, hub), hub, Config{Assets: http.NotFoundHandler(), Version: "1.0.0-rc.8", Updates: service}))
	defer server.Close()
	request := func(method, path, csrf string, body any, session bool) (int, string) {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, _ = json.Marshal(body)
		}
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		if session {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
		}
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(out)
	}
	if code, _ := request("GET", "/api/updates", "", nil, false); code != 401 {
		t.Fatal("updates leaked without a session", code)
	}
	code, body := request("GET", "/api/updates", "", nil, true)
	if code != 200 || !strings.Contains(body, `"currentVersion":"1.0.0-rc.8"`) || !strings.Contains(body, `"enabled":true`) {
		t.Fatal(code, body)
	}
	if code, _ = request("POST", "/api/updates/check", "", nil, true); code != 403 {
		t.Fatal("update check bypassed CSRF", code)
	}
	code, body = request("POST", "/api/updates/check", "csrf", nil, true)
	if code != 200 || !strings.Contains(body, `"version":"1.0.0-rc.9"`) {
		t.Fatal(code, body)
	}
	code, body = request("POST", "/api/updates/settings", "csrf", map[string]any{"channel": "nightly", "auto": false}, true)
	if code != 200 || !strings.Contains(body, `"channel":"nightly"`) || !strings.Contains(body, `"version":"1.0.0-rc.9.nightly.4"`) {
		t.Fatal(code, body)
	}
	code, body = request("POST", "/api/updates/apply", "csrf", nil, true)
	if code != 202 || !strings.Contains(body, `"state":"queued"`) {
		t.Fatal(code, body)
	}
	var applied updates.Status
	if err = json.Unmarshal([]byte(body), &applied); err != nil || applied.RequestID == "" {
		t.Fatal(err, body)
	}
	if _, err = os.Stat(filepath.Join(dataDir, "update-request.json")); err != nil {
		t.Fatal("apply did not write a request", err)
	}
	if code, body = request("GET", "/api/updates/status", "", nil, true); code != 200 || !strings.Contains(body, `"state":"queued"`) {
		t.Fatal(code, body)
	}
	raw, _ := json.Marshal(updates.Status{State: "updating", Message: "Installing", ToVersion: "1.0.0-rc.9.nightly.4", RequestID: applied.RequestID})
	if err = os.WriteFile(filepath.Join(dataDir, "update-status.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if code, body = request("GET", "/api/updates/status", "", nil, true); code != 200 || !strings.Contains(body, `"state":"updating"`) {
		t.Fatal(code, body)
	}

	disabled := httptest.NewServer(New(db, inventory.New(testEngine{}, db, hub), hub, Config{Assets: http.NotFoundHandler(), Version: "1.0.0-rc.8"}))
	defer disabled.Close()
	req, _ := http.NewRequest("GET", disabled.URL+"/api/updates", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
	res, err := disabled.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(out), `"enabled":false`) {
		t.Fatal(res.StatusCode, string(out))
	}
}

type updateFeed struct {
	main, nightly string
}

func (f *updateFeed) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	channel := ""
	version := ""
	switch {
	case strings.HasPrefix(r.URL.Path, "/main/"):
		channel, version = "main", f.main
	case strings.HasPrefix(r.URL.Path, "/nightly/"):
		channel, version = "nightly", f.nightly
	default:
		http.NotFound(w, r)
		return
	}
	switch strings.TrimPrefix(r.URL.Path, "/"+channel+"/") {
	case "version.txt":
		_, _ = w.Write([]byte(version))
	case "image.txt":
		_, _ = w.Write([]byte("ghcr.io/matyas-toth/shipperkit@sha256:1111111111111111111111111111111111111111111111111111111111111111"))
	case "release-url.txt":
		_, _ = w.Write([]byte("https://github.com/matyas-toth/shipperkit/commit/abc123"))
	default:
		http.NotFound(w, r)
	}
}
