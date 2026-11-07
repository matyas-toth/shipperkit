package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/events"
	"shipper/internal/gitprovider"
	"shipper/internal/inventory"
	"shipper/internal/store"
	"strings"
	"testing"
)

func TestGitHubManifestAccountTarget(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Server{db: db, publicURLValue: "https://shipper.example.test"}
	for _, tc := range []struct {
		organization, path string
		status             int
	}{
		{"", "/settings/apps/new", 200},
		{" Northstar-Team ", "/organizations/Northstar-Team/settings/apps/new", 200},
		{"https://github.com/northstar", "", 422},
		{"../settings", "", 422},
		{"-northstar", "", 422},
		{"north--star", "", 422},
		{strings.Repeat("a", 40), "", 422},
	} {
		t.Run(tc.organization, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"organization": tc.organization})
			req := httptest.NewRequest("POST", "/api/connections/github", bytes.NewReader(body))
			res := httptest.NewRecorder()
			s.beginGitHub(res, req)
			if res.Code != tc.status {
				t.Fatalf("%d: %s", res.Code, res.Body)
			}
			if tc.status != 200 {
				return
			}
			var result struct {
				Action   string
				Manifest struct {
					Public      bool
					RedirectURL string            `json:"redirect_url"`
					SetupURL    string            `json:"setup_url"`
					Permissions map[string]string `json:"default_permissions"`
				}
			}
			if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(result.Action)
			if err != nil || u.Host != "github.com" || u.Path != tc.path {
				t.Fatal(result.Action, err)
			}
			state := u.Query().Get("state")
			if len(state) != 64 || result.Manifest.Public || result.Manifest.Permissions["contents"] != "read" || result.Manifest.Permissions["metadata"] != "read" || len(result.Manifest.Permissions) != 2 || result.Manifest.RedirectURL != "https://shipper.example.test/connect/github/manifest" || result.Manifest.SetupURL != "https://shipper.example.test/connect/github/installed?state="+state {
				t.Fatalf("Unsafe manifest: %+v", result)
			}
			if _, err := db.AdvanceProviderFlow(context.Background(), auth.Hash(state), "manifest", "exchanging"); err != nil {
				t.Fatal(err)
			}
			if _, err := db.AdvanceProviderFlow(context.Background(), auth.Hash(state), "manifest", "exchanging"); err == nil {
				t.Fatal("Setup state reused")
			}
		})
	}
}

func TestPrivateSourcesPushFiltersTokensAndIsolation(t *testing.T) {
	ctx := context.Background()
	secret := "fixture-private-github-token"
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/user/repos":
			w.Write([]byte(`[{"id":55,"full_name":"owner/private","clone_url":"https://github.com/owner/private.git","default_branch":"main","private":true}]`))
		case "/repos/owner/private":
			w.Write([]byte(`{"id":55,"full_name":"owner/private","clone_url":"https://github.com/owner/private.git","default_branch":"main","private":true}`))
		case "/repos/owner/private/branches":
			w.Write([]byte(`[{"name":"main","commit":{"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}]`))
		case "/repos/owner/private/tags":
			w.Write([]byte(`[]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer github.Close()
	g := gitprovider.New()
	g.BaseURL = github.URL
	db, _ := store.Open(t.TempDir())
	defer db.Close()
	hub := events.New()
	worker := deploy.NewWorker(db, nil, hub, t.TempDir())
	setup := auth.Token()
	server := httptest.NewServer(New(db, inventory.New(testEngine{}, db, hub), hub, Config{GitHub: g, SetupToken: setup, Worker: worker, Assets: http.NotFoundHandler()}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	call := func(method, path string, input any, headers map[string]string) (int, []byte) {
		t.Helper()
		body, _ := json.Marshal(input)
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var raw json.RawMessage
		json.NewDecoder(res.Body).Decode(&raw)
		return res.StatusCode, raw
	}
	code, raw := call("POST", "/api/auth/setup", map[string]string{"email": "owner@example.test", "password": "private-source-test-password", "setupToken": setup}, nil)
	if code != 201 {
		t.Fatal(code, string(raw))
	}
	var session struct {
		CSRF string `json:"csrfToken"`
	}
	json.Unmarshal(raw, &session)
	csrf = session.CSRF
	code, raw = call("POST", "/api/connections", map[string]any{"name": "Owner GitHub", "kind": "github_token", "secret": map[string]string{"token": secret}}, nil)
	if code != 200 || strings.Contains(string(raw), secret) {
		t.Fatal(code, string(raw))
	}
	var c store.Connection
	json.Unmarshal(raw, &c)
	code, raw = call("GET", "/api/connections/"+c.ID+"/repositories", nil, nil)
	if code != 200 || !strings.Contains(string(raw), "owner/private") {
		t.Fatal(code, string(raw))
	}
	code, raw = call("POST", "/api/repositories/branches", map[string]string{"sourceId": c.ID, "repository": "https://github.com/owner/private.git"}, nil)
	if code != 200 || !strings.Contains(string(raw), "main") {
		t.Fatal(code, string(raw))
	}
	cfg := store.Config{Method: "static", Repository: "https://github.com/owner/private.git", SourceID: c.ID, RepositoryID: 55, AutoDeploy: store.AutoDeploy{Enabled: true, Paths: []string{"apps/web/**"}, Supersede: true}}
	code, raw = call("POST", "/api/applications", map[string]any{"name": "Private app", "config": cfg}, nil)
	if code != 202 {
		t.Fatal(code, string(raw))
	}
	var d store.Deployment
	json.Unmarshal(raw, &d)
	d, _ = db.Deployment(ctx, d.ID)
	d.State = "succeeded"
	db.SaveDeployment(ctx, d)
	db.SetActive(ctx, d.ApplicationID, d.ID)
	source, _ := db.Connection(ctx, c.ID)
	push := func(delivery, file string, valid bool) int {
		event := map[string]any{"ref": "refs/heads/main", "after": strings.Repeat("b", 40), "size": 1, "repository": map[string]any{"id": 55, "default_branch": "main"}, "commits": []any{map[string]any{"modified": []string{file}}}}
		raw, _ := json.Marshal(event)
		mac := hmac.New(sha256.New, []byte(source.Secret.WebhookSecret))
		mac.Write(raw)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !valid {
			signature = "sha256=00"
		}
		code, _ := call("POST", "/hooks/github/"+c.ID, event, map[string]string{"X-GitHub-Event": "push", "X-GitHub-Delivery": delivery, "X-Hub-Signature-256": signature})
		return code
	}
	if push("bad", "apps/web/a.ts", false) != 403 {
		t.Fatal("invalid signature accepted")
	}
	if push("unrelated", "apps/api/a.ts", true) != 202 {
		t.Fatal("event rejected")
	}
	jobs, _ := db.DeployIntents(ctx)
	if len(jobs) != 0 {
		t.Fatal("unrelated path triggered build")
	}
	push("changed", "apps/web/a.ts", true)
	push("changed", "apps/web/a.ts", true)
	jobs, _ = db.DeployIntents(ctx)
	if len(jobs) != 1 {
		t.Fatal("push was duplicated", jobs)
	}
	code, raw = call("POST", "/api/applications/"+d.ApplicationID+"/deploy-token", map[string]any{}, nil)
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	var result struct{ Token string }
	json.Unmarshal(raw, &result)
	code, raw = call("POST", "/api/v1/applications/"+d.ApplicationID+"/deploy", map[string]string{"revision": strings.Repeat("c", 40)}, map[string]string{"Authorization": "Bearer " + result.Token})
	if code != 202 {
		t.Fatal(code, string(raw))
	}
	var pinned store.Deployment
	json.Unmarshal(raw, &pinned)
	if pinned.Commit != strings.Repeat("c", 40) {
		t.Fatal("token revision not pinned")
	}
	code, _ = call("POST", "/api/v1/applications/another/deploy", map[string]any{}, map[string]string{"Authorization": "Bearer " + result.Token})
	if code != 403 {
		t.Fatal("token escaped its application")
	}
	call("POST", "/api/applications/"+d.ApplicationID+"/deploy-token", map[string]bool{"revoke": true}, nil)
	code, _ = call("POST", "/api/v1/applications/"+d.ApplicationID+"/deploy", map[string]any{}, map[string]string{"Authorization": "Bearer " + result.Token})
	if code != 403 {
		t.Fatal("revoked token remained valid")
	}
}
