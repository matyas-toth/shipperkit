package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shipper/internal/auth"
	"shipper/internal/events"
	"shipper/internal/inventory"
	"shipper/internal/store"
)

func TestDeploymentContextIncludesAllRetainedLogsAndEnforcesLogScope(t *testing.T) {
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
	config := store.Config{Method: "railpack", Repository: "https://github.com/example/site.git", Branch: "main", Variables: []store.Variable{{Name: "TOKEN", Value: "private-context-secret", Secret: true}}}
	d, err := db.Enqueue(ctx, store.Application{ID: "site", Name: "Site", Config: config}, store.Deployment{ID: "failed-release", Operation: "deploy", Commit: strings.Repeat("a", 40)}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	d.State = "failed"
	d.ResolvedMethod = "railpack"
	d.Error = "build failed private-context-secret"
	d.FinishedAt = time.Now().Unix()
	if err = db.SaveDeployment(ctx, d); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 510; i++ {
		if err = db.AppendLog(ctx, d.ID, "building", fmt.Sprintf("output %d private-context-secret", i)); err != nil {
			t.Fatal(err)
		}
	}
	hub := events.New()
	monitor := inventory.New(testEngine{}, db, hub)
	server := httptest.NewServer(New(db, monitor, hub, Config{Assets: http.NotFoundHandler()}))
	defer server.Close()
	call := func(path, token string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", server.URL+path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
		}
		res, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}
	code, body := call("/api/applications/site/deployments/failed-release/context", "")
	if code != 200 || strings.Contains(body, "private-context-secret") {
		t.Fatal(code, body)
	}
	var result struct {
		Logs       []store.LogLine `json:"logs"`
		Deployment struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			Stage      string `json:"stage"`
			FinishedAt string `json:"finishedAt"`
		} `json:"deployment"`
	}
	if err = json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Logs) != 510 || result.Logs[509].Number != 510 || result.Deployment.ID != d.ID || result.Deployment.Status != "failed" || result.Deployment.Stage != "building" || result.Deployment.FinishedAt == "" {
		t.Fatal(result)
	}
	if code, _ = call("/api/applications/other/deployments/failed-release/context", ""); code != 404 {
		t.Fatal(code)
	}
	for _, scope := range []string{"read", "logs"} {
		raw := auth.Token()
		if err = db.SaveOperationToken(ctx, store.OperationToken{ID: scope, Name: scope, UserID: "owner", Hash: auth.Hash(raw), ExpiresAt: time.Now().Add(time.Hour).Unix(), Scopes: []string{scope}, Applications: []string{"site"}}); err != nil {
			t.Fatal(err)
		}
		code, _ = call("/api/v1/applications/site/deployments/failed-release/context", raw)
		if scope == "read" && code != 403 || scope == "logs" && code != 200 {
			t.Fatal(scope, code)
		}
	}
}
