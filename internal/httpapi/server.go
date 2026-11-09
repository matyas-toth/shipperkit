// Package httpapi exposes the same authenticated resources to the SPA and automation.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/events"
	"shipper/internal/gitprovider"
	"shipper/internal/inventory"
	"shipper/internal/operations"
	"shipper/internal/store"
	"shipper/internal/updates"
)

const cookieName = "shipper_session"

type Config struct {
	GitHub         *gitprovider.GitHub
	PublicURL      string
	SetupToken     string
	Assets         http.Handler
	Version        string
	InstanceID     string
	Worker         *deploy.Worker
	Proxy          deploy.ProxyConfig
	EffectiveProxy func(context.Context) (deploy.ProxyConfig, error)
	SetupClaim     func() (string, error)
	Operations     *operations.Service
	Updates        *updates.Service
	OAuth          *OAuthEndpoints
}
type Server struct {
	terminalMu sync.Mutex
	terminals map[string]*terminalSession
	github          *gitprovider.GitHub
	db              *store.Store
	monitor         *inventory.Monitor
	hub             *events.Hub
	cfg             Config
	limiter         *limiter
	limitMax        int
	dummyHash       string
	authSlots       chan struct{}
	logSlots        chan struct{}
	protectionSlots chan struct{}
	authMu          sync.Mutex
	authChanged     chan struct{}
	urlMu           sync.RWMutex
	publicURLValue  string
	dashboardValue  string
	oauth           *OAuthEndpoints
	jwksMu          sync.Mutex
	jwks            map[string]jwksKey
	jwksFetched     time.Time
}
type contextKey int

const sessionKey contextKey = 0

func New(db *store.Store, monitor *inventory.Monitor, hub *events.Hub, cfg Config) http.Handler {
	if cfg.GitHub == nil {
		cfg.GitHub = gitprovider.New()
	}
	dummy, _ := auth.Password(auth.Token()[:32])
	dashboard, _ := db.Setting(context.Background(), "dashboard_domain", "")
	if dashboard == "" {
		if u, e := url.Parse(cfg.Proxy.DashboardURL); e == nil && u.Scheme == "https" {
			dashboard = u.Hostname()
		}
	}
	publicURL := cfg.PublicURL
	if dashboard != "" {
		publicURL = "https://" + dashboard
	}
	limitMax := 10
	if raw := os.Getenv("SHIPPER_RATE_LIMIT"); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
			limitMax = parsed
		}
	}
	s := &Server{github: cfg.GitHub, db: db, monitor: monitor, hub: hub, cfg: cfg, limiter: &limiter{entries: map[string]attempt{}}, limitMax: limitMax, dummyHash: dummy, authSlots: make(chan struct{}, 2), logSlots: make(chan struct{}, 8), protectionSlots: make(chan struct{}, 1), publicURLValue: publicURL, dashboardValue: dashboard}
	s.authChanged = make(chan struct{})
	s.oauth = cfg.OAuth
	if s.oauth == nil {
		s.oauth = DefaultOAuthEndpoints()
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, s.headers)
	r.Get("/connect/github/manifest", s.githubManifest)
	r.Get("/connect/github/installed", s.githubInstalled)
	r.Post("/hooks/github/{connection}", s.githubWebhook)
	r.Post("/api/v1/applications/{id}/deploy", s.tokenDeploy)
	r.With(s.limit).Get("/auth/oauth/{provider}/start", s.oauthStart)
	r.Get("/auth/oauth/{provider}/callback", s.oauthCallback)
	r.Route("/api", func(r chi.Router) {
		r.Use(s.origin)
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			s.json(w, 200, map[string]string{"status": "ok", "version": s.cfg.Version})
		})
		r.Get("/auth/session", s.currentSession)
		r.Get("/auth/methods", s.authMethods)
		r.With(s.limit, s.hashCapacity).Post("/auth/setup", s.setup)
		r.With(s.limit, s.hashCapacity).Post("/auth/login", s.login)
		s.factorRoutes(r)
		r.Group(func(r chi.Router) {
			r.Use(s.requireSession)
			s.operationsRoutes(r)
			s.networkingRoutes(r)
			r.Get("/containers", s.containers)
			r.Get("/applications", s.applications)
			r.Get("/applications/{id}", s.application)
			r.Get("/applications/{id}/deployments/{deployment}", s.deployment)
			r.Get("/applications/{id}/deployments/{deployment}/logs", s.deploymentLogs)
			r.Get("/applications/{id}/tasks", s.tasks)
			r.Get("/proxy", s.proxyInfo)
			r.Get("/domain-options", s.domainOptions)
			r.Get("/updates", s.updateState)
			r.Get("/updates/status", s.updateStatus)
			r.Get("/events", s.stream)
			r.Group(func(r chi.Router) {
				r.Use(s.requireRole(store.RoleDeveloper))
				s.databaseResourceRoutes(r)
				s.terminalRoutes(r)
				s.protectionRoutes(r)
				r.Get("/connections", s.connections)
				r.Get("/connections/{connection}/repositories", s.repositories)
				r.Get("/applications/{id}/logs", s.runtimeLogs)
				r.With(s.csrf, s.limit).Post("/applications/{id}/tasks", s.saveTask)
				r.With(s.csrf, s.limit).Post("/applications/{id}/tasks/{task}/run", s.runTask)
				r.With(s.csrf, s.limit).Post("/applications/{id}/tasks/{task}/delete", s.deleteTask)
				r.With(s.csrf, s.limit).Post("/applications/{id}/schedule-preview", s.schedulePreview)
				r.With(s.csrf, s.limit).Post("/applications/{id}/changes", s.redeploy)
				r.With(s.csrf, s.limit).Post("/applications/{id}/inline/source", s.inlineSource)
				r.With(s.csrf, s.limit).Post("/applications/{id}/deploy-token", s.deployToken)
				r.With(s.csrf, s.limit).Post("/applications/{id}/deployments/{deployment}/cancel", s.cancelDeployment)
				r.With(s.csrf, s.limit).Post("/applications/{id}/deployments/{deployment}/retry", s.retryDeployment)
				r.With(s.csrf, s.limit).Post("/applications/{id}/deployments/{deployment}/rollback", s.rollbackRelease)
				r.With(s.csrf, s.limit).Post("/compose/preview", s.previewCompose)
				r.With(s.csrf, s.limit).Post("/applications/{id}/compose/source", s.composeSource)
				r.With(s.csrf, s.limit).Post("/repositories/branches", s.sourceBranches)
				r.With(s.csrf, s.limit).Post("/repositories/inspect", s.inspectSource)
				r.With(s.csrf, s.limit).Post("/variables/parse", s.parseVariables)
				r.With(s.csrf, s.limit).Post("/applications/{id}/name", s.renameApplication)
				r.With(s.csrf, s.limit).Post("/applications/{id}/domains", s.updateDomains)
				r.With(s.csrf, s.limit).Post("/applications/{id}/lifecycle", s.lifecycle)
				r.With(s.csrf, s.limit).Post("/applications/{id}/variables/reveal", s.revealVariable)
				r.With(s.csrf, s.limit).Post("/applications/{id}/database/connection", s.databaseConnection)
				r.With(s.csrf, s.limit).Post("/applications", s.createApplication)
				r.With(s.csrf, s.limit).Post("/applications/{id}/deployments", s.redeploy)
				r.With(s.csrf, s.limit).Post("/applications/{id}/delete", s.deleteApplication)
				r.With(s.csrf, s.limit).Post("/applications/{id}/domain-check", s.checkDomain)
				r.With(s.csrf, s.limit).Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
					s.monitor.Trigger()
					s.json(w, 202, map[string]string{"status": "queued"})
				})
			})
			r.Group(func(r chi.Router) {
				r.Use(s.requireRole(store.RoleAdmin))
				s.userRoutes(r)
				s.authProviderRoutes(r)
				r.With(s.csrf, s.limit).Post("/connections", s.saveConnection)
				r.With(s.csrf, s.limit).Post("/connections/{connection}/delete", s.deleteConnection)
				r.With(s.csrf, s.limit).Post("/connections/{connection}/webhook", s.connectionWebhook)
				r.With(s.csrf, s.limit).Post("/connections/github", s.beginGitHub)
				r.With(s.csrf, s.limit).Post("/proxy/check", s.checkProxy)
				r.With(s.csrf, s.limit).Post("/proxy", s.saveProxy)
				r.Get("/system", s.system)
				r.With(s.csrf, s.limit).Post("/updates/check", s.checkUpdates)
				r.With(s.csrf, s.limit).Post("/updates/settings", s.saveUpdateSettings)
				r.With(s.csrf, s.limit).Post("/updates/apply", s.applyUpdate)
			})
		})
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			s.problem(w, 404, "not_found", "The resource was not found")
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			s.problem(w, 405, "method_not_allowed", "This method is not supported")
		})
	})
	r.Handle("/*", cfg.Assets)
	return s.automation(r)
}
func (s *Server) publicURL() string {
	s.urlMu.RLock()
	defer s.urlMu.RUnlock()
	return s.publicURLValue
}
func (s *Server) dashboardName() string {
	s.urlMu.RLock()
	defer s.urlMu.RUnlock()
	return s.dashboardValue
}
func (s *Server) setDashboard(domain string) {
	s.urlMu.Lock()
	s.dashboardValue = domain
	if domain != "" {
		s.publicURLValue = "https://" + domain
	} else {
		s.publicURLValue = s.cfg.PublicURL
	}
	s.urlMu.Unlock()
}
func (s *Server) secureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
func (s *Server) headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self' https://github.com")
		if s.secureRequest(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) origin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Same-origin checks protect writes and authenticated streams without trusting proxy headers.
		if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
			s.problem(w, 403, "origin_denied", "The request origin is not allowed")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			allowed := s.publicURL()
			current := requestOrigin(r)
			if origin != allowed && origin != current {
				s.problem(w, 403, "origin_denied", "The request origin is not allowed")
				return
			}
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			kind, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if kind != "application/json" && !(kind == "application/x-tar" && r.URL.Path == "/api/sources") && !(kind == "multipart/form-data" && (strings.HasSuffix(r.URL.Path, "/protection/import") || strings.HasSuffix(r.URL.Path, "/database/import"))) {
				s.problem(w, 415, "json_required", "Send the request as JSON")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if principal(r) != nil {
			next.ServeHTTP(w, r)
			return
		}
		session, err := s.session(r)
		if err != nil {
			if errors.Is(err, errFactorGate) {
				code := "factor_required"
				if session.User.PasswordChangeRequired {
					code = "password_change_required"
				}
				s.problem(w, 403, code, "Complete account setup to continue")
				return
			}
			s.problem(w, 401, "unauthenticated", "Sign in to continue")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, session)))
	})
}

func roleRank(role string) int {
	switch role {
	case store.RoleAdmin:
		return 3
	case store.RoleDeveloper:
		return 2
	case store.RoleViewer:
		return 1
	}
	return 0
}

// requireRole is applied after requireSession so the browser session is
// authoritative. Personal API credentials never gain administration rights.
func (s *Server) requireRole(minimum string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if principal(r) != nil {
				// Operations tokens already passed the path-and-scope allowlist in
				// the automation middleware. They can never reach administration
				// routes, which are marked admin-only.
				if minimum == store.RoleAdmin {
					s.problem(w, 403, "forbidden", "This action needs an interactive account")
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			session, ok := r.Context().Value(sessionKey).(store.Session)
			if !ok {
				s.problem(w, 401, "unauthenticated", "Sign in to continue")
				return
			}
			if roleRank(session.User.Role) < roleRank(minimum) {
				s.problem(w, 403, "forbidden", "Your role does not allow this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if principal(r) != nil {
			next.ServeHTTP(w, r)
			return
		}
		session := r.Context().Value(sessionKey).(store.Session)
		if !auth.Equal(r.Header.Get("X-CSRF-Token"), session.CSRF) {
			s.problem(w, 403, "csrf_invalid", "Refresh the page and try again")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) session(r *http.Request) (store.Session, error) {
	if principal(r) != nil {
		t, e := s.db.OperationToken(r.Context(), strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		return store.Session{User: store.User{ID: t.UserID}, ExpiresAt: t.ExpiresAt}, e
	}
	session, err := s.browserSession(r)
	if err == nil && session.AuthStep() != "" {
		err = errFactorGate
	}
	return session, err
}

func (s *Server) browserSession(r *http.Request) (store.Session, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return store.Session{}, err
	}
	if len(cookie.Value) != 64 {
		return store.Session{}, errors.New("invalid session")
	}
	return s.db.Session(r.Context(), auth.Hash(cookie.Value))
}
func (s *Server) currentSession(w http.ResponseWriter, r *http.Request) {
	configured, err := s.db.Configured(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	session, err := s.browserSession(r)
	if err != nil {
		s.json(w, 200, map[string]any{"setupRequired": !configured, "user": nil})
		return
	}
	if session.AuthStep() == "" {
		if err = s.db.FinalizeSessionIdentity(r.Context(), sessionHash(r), time.Now()); err != nil {
			_ = s.db.DeleteSession(r.Context(), sessionHash(r))
			s.json(w, 200, map[string]any{"setupRequired": false, "user": nil})
			return
		}
	}
	s.writeSession(w, 200, session, nil)
}

type credentials struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	SetupToken string `json:"setupToken,omitempty"`
}

func (s *Server) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		s.problem(w, 400, "invalid_json", "The request body is invalid")
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		s.problem(w, 400, "invalid_json", "Send a single JSON object")
		return false
	}
	return true
}
func validEmail(value string) bool {
	p, err := mail.ParseAddress(value)
	return err == nil && p.Address == value && len(value) <= 254
}
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !s.decode(w, r, &input) {
		return
	}
	configured, err := s.db.Configured(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	if configured {
		s.problem(w, 409, "setup_complete", "Shipper already has an administrator")
		return
	}
	claim := s.cfg.SetupToken
	if s.cfg.SetupClaim != nil {
		claim, err = s.cfg.SetupClaim()
		if err != nil {
			s.problem(w, 403, "setup_token_expired", "Get a fresh setup token on the server")
			return
		}
	}
	if claim == "" || !auth.Equal(input.SetupToken, claim) {
		s.problem(w, 403, "setup_token_invalid", "The setup token is invalid")
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(input.Email) {
		s.problem(w, 422, "invalid_email", "Enter a valid email address")
		return
	}
	password, err := auth.Password(input.Password)
	if err != nil {
		s.problem(w, 422, "invalid_password", err.Error())
		return
	}
	token, csrf := auth.Token(), auth.Token()
	expiry := time.Now().Add(7 * 24 * time.Hour).Unix()
	user := store.User{ID: auth.Token()[:24], Email: input.Email, PasswordHash: password}
	if err = s.db.CreateAdmin(r.Context(), user, auth.Hash(token), csrf, expiry); err != nil {
		if errors.Is(err, store.ErrAlreadyConfigured) {
			s.problem(w, 409, "setup_complete", "Shipper already has an administrator")
		} else {
			s.internal(w, err)
		}
		return
	}
	s.cookie(w, r, token, expiry)
	if s.cfg.Worker != nil {
		s.cfg.Worker.Trigger()
	}
	user.Role = store.RoleAdmin
	user.HasPassword = true
	user.CreatedAt = time.Now().Unix()
	s.json(w, 201, map[string]any{"setupRequired": false, "user": user, "csrfToken": csrf})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !s.decode(w, r, &input) {
		return
	}
	enabled, err := s.db.PasswordLoginEnabled(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	if !enabled {
		s.problem(w, 403, "password_login_disabled", "Password sign-in is disabled. Use a configured provider.")
		return
	}
	user, err := s.db.FindUser(r.Context(), strings.ToLower(strings.TrimSpace(input.Email)))
	hash := s.dummyHash
	if err == nil && user.HasPassword {
		hash = user.PasswordHash
	}
	valid := auth.Verify(hash, input.Password)
	if err != nil || !user.HasPassword || !valid {
		s.problem(w, 401, "invalid_credentials", "The email or password is incorrect")
		return
	}
	token, csrf := auth.Token(), auth.Token()
	expiry := time.Now().Add(7 * 24 * time.Hour).Unix()
	factor, err := s.db.FactorState(r.Context(), user.ID)
	if err != nil {
		s.internal(w, err)
		return
	}
	if user.PasswordChangeRequired || factor.Enrolled || factor.Required {
		expiry = time.Now().Add(10 * time.Minute).Unix()
	}
	if err = s.db.CreateSession(r.Context(), auth.Hash(token), user.ID, csrf, expiry, "password", nil); err != nil {
		s.internal(w, err)
		return
	}
	if err = s.db.RecordLogin(r.Context(), user.ID, "password", time.Now()); err != nil {
		s.internal(w, err)
		return
	}
	s.cookie(w, r, token, expiry)
	s.writeSession(w, 200, store.Session{User: user, CSRF: csrf, ExpiresAt: expiry, Factor: factor, FirstFactor: "password"}, nil)
}
func (s *Server) cookie(w http.ResponseWriter, r *http.Request, token string, expiry int64) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, Secure: s.secureRequest(r), SameSite: http.SameSiteStrictMode, Expires: time.Unix(expiry, 0), MaxAge: max(1, int(expiry-time.Now().Unix()))})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(cookieName)
	if err := s.db.DeleteSession(r.Context(), auth.Hash(cookie.Value)); err != nil {
		s.internal(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.secureRequest(r), SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	s.json(w, 200, map[string]bool{"ok": true})
}
func (s *Server) containers(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.Containers(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	managed := []store.Container{}
	for _, c := range items {
		if c.Managed {
			managed = append(managed, c)
		}
	}
	s.json(w, 200, map[string]any{"items": managed, "docker": s.monitor.Health()})
}
func (s *Server) applications(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.Applications(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	for i := range items {
		items[i] = publicApplication(r, items[i])
	}
	if p := principal(r); p != nil {
		filtered := []store.Application{}
		for _, a := range items {
			if p.Allows("read", a.ID) {
				filtered = append(filtered, a)
			}
		}
		items = filtered
		if len(p.Applications) > 0 {
			h := s.monitor.Health()
			s.json(w, 200, map[string]any{"items": items, "docker": map[string]any{"connected": h.Connected, "checkedAt": h.CheckedAt, "observedAt": h.ObservedAt}})
			return
		}
	}
	s.json(w, 200, map[string]any{"items": items, "docker": s.monitor.Health()})
}
func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	counts, err := s.db.Counts(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	s.json(w, 200, map[string]any{"version": s.cfg.Version, "instanceId": s.cfg.InstanceID, "docker": s.monitor.Health(), "database": map[string]any{"engine": "SQLite", "journal": "WAL", "counts": counts}, "runtime": map[string]any{"go": runtime.Version(), "heapBytes": memory.HeapAlloc, "goroutines": runtime.NumGoroutine()}})
}
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	changes := s.authChanges()
	if _, err := s.session(r); err != nil {
		s.problem(w, 403, "factor_required", "Sign in again to follow events")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.problem(w, 500, "stream_unavailable", "Streaming is unavailable")
		return
	}
	messages, unsubscribe, ok := s.hub.Subscribe()
	if !ok {
		s.problem(w, 503, "stream_limit", "Too many active event streams")
		return
	}
	defer unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-changes:
			return
		case event, ok := <-messages:
			if !ok {
				return
			}
			controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
			data, _ := json.Marshal(event)
			if _, err := fmt.Fprintf(w, "id: %d\nevent: inventory\ndata: %s\n\n", event.Revision, data); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := s.session(r); err != nil {
				return
			}
			controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := io.WriteString(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
func (s *Server) json(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func (s *Server) problem(w http.ResponseWriter, status int, code, message string) {
	s.json(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func (s *Server) internal(w http.ResponseWriter, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	slog.Error("API storage failure", "error", err)
	s.problem(w, 500, "internal_error", "The request could not be completed")
}

type attempt struct {
	count   int
	expires time.Time
}

// Bound expensive bcrypt work globally as well as limiting each direct peer.
func (s *Server) hashCapacity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case s.authSlots <- struct{}{}:
			defer func() { <-s.authSlots }()
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			s.problem(w, 503, "authentication_busy", "Authentication is busy, try again shortly")
		}
	})
}

type limiter struct {
	mu      sync.Mutex
	entries map[string]attempt
}

func (s *Server) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		key := host + r.URL.Path
		now := time.Now()
		s.limiter.mu.Lock()
		for k, v := range s.limiter.entries {
			if now.After(v.expires) {
				delete(s.limiter.entries, k)
			}
		}
		entry := s.limiter.entries[key]
		if now.After(entry.expires) {
			entry = attempt{expires: now.Add(time.Minute)}
		}
		entry.count++
		s.limiter.entries[key] = entry
		allowed := entry.count <= s.limitMax
		s.limiter.mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "60")
			s.problem(w, 429, "rate_limited", "Too many attempts, try again in a minute")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ValidatePublicURL keeps cookie/origin policy independent from forwarded headers.
func ValidatePublicURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return errors.New("SHIPPER_PUBLIC_URL must be an http(s) origin without a path or trailing slash")
	}
	return nil
}
