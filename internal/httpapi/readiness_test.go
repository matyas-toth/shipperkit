package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/events"
	"shipper/internal/store"
)

func TestRoleDowngradeRevokesAutomationAndTerminals(t *testing.T) {
	for _, transition := range [][2]string{{store.RoleAdmin, store.RoleDeveloper}, {store.RoleAdmin, store.RoleViewer}, {store.RoleDeveloper, store.RoleViewer}} {
		t.Run(transition[0]+"_to_"+transition[1], func(t *testing.T) {
			fx := newAuthFixture(t)
			ctx := context.Background()
			owner, err := fx.db.User(ctx, "owner")
			if err != nil {
				t.Fatal(err)
			}
			if err = fx.db.CreateUser(ctx, store.User{ID: "second", Email: "second@example.test", Role: store.RoleAdmin, PasswordHash: owner.PasswordHash}, "owner", time.Now()); err != nil {
				t.Fatal(err)
			}
			if transition[0] != store.RoleAdmin {
				if err = fx.db.UpdateUserRole(ctx, "owner", transition[0], "second", time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			browser := newTestClient(t)
			login := browser.login(t, fx, "owner@example.test", "administrator-password-123", 200)
			app := store.Application{ID: "audit-app", Name: "Audit app", SourceKind: "image", Method: "image", State: "running", Config: store.Config{Image: "nginx:alpine", Port: 80}}
			if _, err = fx.db.Enqueue(ctx, app, store.Deployment{ID: "audit-deploy", ApplicationID: app.ID, Operation: "deploy", State: "succeeded", Config: app.Config}, "", true); err != nil {
				t.Fatal(err)
			}
			if err = fx.db.ReplaceInventory(ctx, []store.Container{{ID: "audit-container", ApplicationID: app.ID, Managed: true, State: "running"}}); err != nil {
				t.Fatal(err)
			}
			raw := auth.Token()
			if err = fx.db.SaveOperationToken(ctx, store.OperationToken{ID: "audit-token", Name: "Audit", UserID: "owner", Hash: auth.Hash(raw), Scopes: []string{"full"}, ExpiresAt: time.Now().Add(time.Hour).Unix()}); err != nil {
				t.Fatal(err)
			}
			u, _ := url.Parse(fx.server.URL)
			var cookie string
			for _, c := range browser.client.Jar.Cookies(u) {
				if c.Name == cookieName {
					cookie = c.Value
				}
			}
			terminal := store.TerminalSession{ID: "audit-terminal", UserID: "owner", AuthHash: auth.Hash(cookie), ApplicationID: app.ID, ContainerID: "audit-container", ExpiresAt: time.Now().Add(time.Hour).Unix()}
			if err = fx.db.CreateTerminal(ctx, terminal); err != nil {
				t.Fatal(err)
			}
			runtime := &terminalRuntime{ownership: true}
			server := &Server{db: fx.db, cfg: Config{Worker: deploy.NewWorker(fx.db, runtime, events.New(), t.TempDir())}}
			if err = server.terminalOwned(ctx, terminal); err != nil {
				t.Fatal(err)
			}
			if err = fx.db.UpdateUserRole(ctx, "owner", transition[1], "second", time.Now()); err != nil {
				t.Fatal(err)
			}
			if code, _, _ := browser.call(t, fx, "POST", "/api/applications/audit-app/name", map[string]string{"name": "Browser mutation"}, login["csrfToken"].(string)); code != 401 {
				t.Fatalf("old browser session accepted: %d", code)
			}
			request, _ := http.NewRequest("POST", fx.server.URL+"/api/v1/applications/audit-app/name", bytes.NewBufferString(`{"name":"Token mutation"}`))
			request.Header.Set("Authorization", "Bearer "+raw)
			request.Header.Set("Content-Type", "application/json")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 401 {
				t.Fatalf("old token accepted: %d", response.StatusCode)
			}
			if err = server.terminalOwned(ctx, terminal); err == nil {
				t.Fatal("existing terminal remained authorized")
			}
			fresh := newTestClient(t)
			session := fresh.login(t, fx, "owner@example.test", "administrator-password-123", 200)
			code, _, _ := fresh.call(t, fx, "POST", "/api/applications/audit-app/name", map[string]string{"name": "Fresh session"}, session["csrfToken"].(string))
			want := 403
			if transition[1] == store.RoleDeveloper {
				want = 200
			}
			if code != want {
				t.Fatalf("new role got %d, want %d", code, want)
			}
		})
	}
}

func TestHostMaintenanceScopeIsNarrowAndUsesCurrentRole(t *testing.T) {
	fx := newAuthFixture(t)
	for _, tc := range []struct {
		name         string
		scopes, apps []string
		path         string
		want         int
	}{
		{"maintenance", []string{"host-admin"}, nil, "/system/recheck", 202},
		{"ordinary full", []string{"full"}, nil, "/system/recheck", 403},
		{"read", []string{"read"}, nil, "/system/recheck", 403},
		{"app bound", []string{"host-admin"}, []string{"one"}, "/system/recheck", 403},
		{"users remain interactive", []string{"host-admin"}, nil, "/users", 403},
		{"updates remain interactive", []string{"host-admin"}, nil, "/updates/apply", 403},
		{"factor policy remains interactive", []string{"host-admin"}, nil, "/auth/factor/policy", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := auth.Token()
			if err := fx.db.SaveOperationToken(context.Background(), store.OperationToken{ID: raw[:24], Name: tc.name, UserID: "owner", Hash: auth.Hash(raw), Scopes: tc.scopes, Applications: tc.apps, ExpiresAt: time.Now().Add(time.Hour).Unix()}); err != nil {
				t.Fatal(err)
			}
			req, _ := http.NewRequest("POST", fx.server.URL+"/api/v1"+tc.path, strings.NewReader(`{}`))
			req.Header.Set("Authorization", "Bearer "+raw)
			req.Header.Set("Content-Type", "application/json")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, _ := io.ReadAll(res.Body)
			if res.StatusCode != tc.want {
				t.Fatalf("got %d %s, want %d", res.StatusCode, body, tc.want)
			}
		})
	}
	// Old database records must not bypass a current Viewer role, even if a
	// previous version failed to revoke a powerful token during demotion.
	owner, _ := fx.db.User(context.Background(), "owner")
	for _, tc := range []struct {
		role, scope string
		want        int
	}{{store.RoleViewer, "full", 401}, {store.RoleDeveloper, "host-admin", 401}, {store.RoleDeveloper, "read", 200}} {
		u := store.User{ID: auth.Token()[:24], Email: auth.Token()[:8] + "@example.test", Role: tc.role, PasswordHash: owner.PasswordHash}
		if err := fx.db.CreateUser(context.Background(), u, "owner", time.Now()); err != nil {
			t.Fatal(err)
		}
		raw := auth.Token()
		if err := fx.db.SaveOperationToken(context.Background(), store.OperationToken{ID: raw[:24], Name: "legacy", UserID: u.ID, Hash: auth.Hash(raw), Scopes: []string{tc.scope}, ExpiresAt: time.Now().Add(time.Hour).Unix()}); err != nil {
			t.Fatal(err)
		}
		if code := newTestClient(t).bearer(t, fx, "/api/v1/capabilities", raw); code != tc.want {
			t.Fatalf("%s/%s token: %d", tc.role, tc.scope, code)
		}
	}
}
