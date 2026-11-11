package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"shipper/internal/auth"
	"shipper/internal/store"
)

const automationKey contextKey = 1
const automationAppKey contextKey = 2

var operationScopes = []string{"read", "logs", "deploy", "execute", "secrets", "destructive", "host-admin", "full"}

func principal(r *http.Request) *store.OperationToken {
	v, _ := r.Context().Value(automationKey).(*store.OperationToken)
	return v
}
func publicDeployment(r *http.Request, d store.Deployment) store.Deployment {
	d = store.PublicDeployment(d)
	if p := principal(r); p != nil && !p.Allows("execute", d.ApplicationID) {
		d.Config.CommandsKeep = true
		d.TaskCommand = nil
		d.Config.Hooks = nil
		d.Config.InstallCommand = ""
		d.Config.BuildCommand = ""
		d.Config.StartCommand = ""
	}
	return d
}
func publicApplication(r *http.Request, a store.Application) store.Application {
	a = store.PublicApplication(a)
	if p := principal(r); p != nil && !p.Allows("execute", a.ID) {
		a.Config.CommandsKeep = true
		a.Config.Hooks = nil
		a.Config.InstallCommand = ""
		a.Config.BuildCommand = ""
		a.Config.StartCommand = ""
	}
	if a.LatestDeployment != nil {
		d := publicDeployment(r, *a.LatestDeployment)
		a.LatestDeployment = &d
	}
	return a
}
func (s *Server) automation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") || strings.HasSuffix(r.URL.Path, "/deploy") {
			next.ServeHTTP(w, r)
			return
		}
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || len(raw) != 64 {
			s.problem(w, 401, "unauthenticated", "Use a valid operations token")
			return
		}
		t, e := s.db.OperationToken(r.Context(), raw)
		if e != nil {
			s.problem(w, 401, "unauthenticated", "The operations token expired or was revoked")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		scope, app := "", ""
		if r.Method == "GET" {
			if path == "/capabilities" {
				scope = "read"
			}
			if path == "/applications" {
				scope = "read"
			}
			if (path == "/system/operations" || path == "/system/retention") && len(t.Applications) == 0 {
				scope = "read"
			}
			if len(parts) >= 2 && parts[0] == "applications" {
				app = parts[1]
				switch {
				case len(parts) == 2, len(parts) == 3 && (parts[2] == "diagnostics" || parts[2] == "observations" || parts[2] == "incidents"), len(parts) == 4 && parts[2] == "deployments":
					scope = "read"
				case len(parts) == 3 && parts[2] == "logs", len(parts) == 5 && parts[2] == "deployments" && parts[4] == "logs":
					scope = "logs"
				case len(parts) == 3 && parts[2] == "tasks":
					scope = "execute"
				}
			}
		}
		if r.Method == "POST" {
			if path == "/system/recheck" || path == "/system/retention" || path == "/system/repair" {
				scope = "host-admin"
			}
			if path == "/applications" || path == "/sources" || path == "/plan" || path == "/database-resources/prepare" {
				scope = "deploy"
			}
			if len(parts) >= 3 && parts[0] == "applications" {
				app = parts[1]
				switch {
				case len(parts) == 3 && (parts[2] == "changes" || parts[2] == "deployments" || parts[2] == "domains" || parts[2] == "domain-check"):
					scope = "deploy"
				case len(parts) == 5 && parts[2] == "deployments" && (parts[4] == "cancel" || parts[4] == "retry" || parts[4] == "rollback"):
					scope = "deploy"
				case len(parts) == 5 && parts[2] == "tasks" && parts[4] == "run":
					scope = "execute"
				case len(parts) == 4 && ((parts[2] == "variables" && parts[3] == "reveal") || (parts[2] == "database" && (parts[3] == "connection" || parts[3] == "ca" || parts[3] == "import"))):
					scope = "secrets"
				case len(parts) == 3 && parts[2] == "delete":
					scope = "destructive"
				}
			}
		}
		// Full app access mirrors the developer application surface. Administration,
		// terminals and instance recovery stay interactive.
		if len(parts) >= 2 && parts[0] == "applications" {
			app = parts[1]
		}
		if app != "" && t.Allows("full", app) && !strings.Contains(path, "/terminal") {
			scope = "full"
		}
		// App-scoped tokens can upload only for their chosen application. Creating a
		// new application, or reading global host evidence, requires instance scope.
		if path == "/sources" {
			app = r.URL.Query().Get("application")
			if t.Allows("full", app) {
				scope = "full"
			}
		}
		if t.Allows("full", "") {
			switch {
			case path == "/applications" && (r.Method == "GET" || r.Method == "POST"),
				path == "/plan",
				path == "/compose/preview",
				path == "/variables/parse",
				path == "/domain-options",
				path == "/containers",
				path == "/refresh",
				path == "/connections" && r.Method == "GET",
				path == "/proxy" && r.Method == "GET",
				path == "/database-resources",
				strings.HasPrefix(path, "/database-resources/"),
				strings.HasPrefix(path, "/repositories/"):
				scope = "full"
			}
		}
		allowed := t.Allows(scope, app)
		if (path == "/applications" || path == "/capabilities") && r.Method == "GET" {
			allowed = false
			for _, v := range t.Scopes {
				allowed = allowed || v == "read" || v == "full"
			}
		}
		if scope == "" || !allowed {
			s.problem(w, 403, "capability_denied", "This token cannot perform that operation")
			return
		}
		clone := r.Clone(context.WithValue(r.Context(), automationKey, &t))
		clone = clone.WithContext(context.WithValue(clone.Context(), automationAppKey, app))
		u := *r.URL
		u.Path = "/api" + path
		clone.URL = &u
		clone = clone.WithContext(context.WithValue(clone.Context(), sessionKey, store.Session{User: store.User{ID: t.UserID, Email: "token:" + t.ID + " user:" + t.UserID}, ExpiresAt: t.ExpiresAt}))
		next.ServeHTTP(w, clone)
	})
}
func (s *Server) capabilities(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{"apiVersion": "1", "version": s.cfg.Version, "instanceId": s.cfg.InstanceID, "scopes": operationScopes, "features": []string{"diagnostics", "operations", "local-source", "reviewed-changes", "incidents"}, "limits": map[string]int{"eventPage": 100, "logPage": 500, "sourceBytes": 64 << 20, "sourceFiles": 10000}})
}
func (s *Server) operationTokens(w http.ResponseWriter, r *http.Request) {
	items, e := s.db.OperationTokens(r.Context())
	if e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, map[string]any{"items": items})
}
func (s *Server) createOperationToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name         string   `json:"name"`
		Scopes       []string `json:"scopes"`
		Applications []string `json:"applications"`
		Days         int      `json:"days"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if !validName(input.Name) || len(input.Scopes) == 0 || len(input.Scopes) > len(operationScopes) || len(input.Applications) > 100 {
		s.problem(w, 422, "invalid_token", "Choose a name, permissions and applications")
		return
	}
	for _, v := range input.Scopes {
		ok := false
		for _, allowed := range operationScopes {
			ok = ok || v == allowed
		}
		if !ok {
			s.problem(w, 422, "invalid_scope", "Choose a supported capability")
			return
		}
	}
	for _, id := range input.Applications {
		if _, e := s.db.Application(r.Context(), id); e != nil {
			s.resourceError(w, e)
			return
		}
	}
	if input.Days == 0 {
		input.Days = 30
	}
	if input.Days < 1 || input.Days > 365 {
		s.problem(w, 422, "invalid_expiry", "Choose an expiry between 1 and 365 days")
		return
	}
	items, e := s.db.OperationTokens(r.Context())
	if e != nil {
		s.internal(w, e)
		return
	}
	if len(items) >= 100 {
		s.problem(w, 409, "token_limit", "Revoke an existing token first")
		return
	}
	raw := auth.Token()
	t := store.OperationToken{ID: auth.Token()[:24], Name: input.Name, Hash: auth.Hash(raw), UserID: r.Context().Value(sessionKey).(store.Session).User.ID, Scopes: input.Scopes, Applications: input.Applications, ExpiresAt: time.Now().Add(time.Duration(input.Days) * 24 * time.Hour).Unix()}
	if e = s.db.SaveOperationToken(r.Context(), t); e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 201, map[string]any{"token": raw, "credential": t})
}
func (s *Server) revokeOperationToken(w http.ResponseWriter, r *http.Request) {
	if e := s.db.RevokeOperationToken(r.Context(), chi.URLParam(r, "token")); e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, map[string]bool{"revoked": true})
}
