package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"shipper/internal/store"
)

func (s *Server) authProviderRoutes(r chi.Router) {
	r.Get("/auth/providers", s.authProviderList)
	r.With(s.csrf, s.limit).Post("/auth/providers/{provider}", s.saveAuthProvider)
	r.With(s.csrf, s.limit).Post("/auth/providers/{provider}/remove", s.removeAuthProvider)
	r.With(s.csrf, s.limit).Post("/auth/providers/{provider}/test", s.testAuthProvider)
	r.With(s.csrf, s.limit).Post("/auth/password-policy", s.savePasswordPolicy)
}

type providerView struct {
	Provider   string `json:"provider"`
	Enabled    bool   `json:"enabled"`
	Configured bool   `json:"configured"`
	ClientID   string `json:"clientId"`
	HasSecret  bool   `json:"hasSecret"`
	Callback   string `json:"callback"`
}

func (s *Server) authProviderList(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.AuthProviders(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	password, err := s.db.PasswordLoginEnabled(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	views := make([]providerView, 0, len(items))
	for _, item := range items {
		views = append(views, providerView{
			Provider: item.Provider, Enabled: item.Enabled, Configured: item.ClientID != "" && item.HasSecret,
			ClientID: item.ClientID, HasSecret: item.HasSecret, Callback: s.oauthCallbackURL(item.Provider),
		})
	}
	s.json(w, 200, map[string]any{"passwordEnabled": password, "origin": s.publicURL(), "providers": views})
}

func (s *Server) saveAuthProvider(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	var input struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
		Enabled      bool   `json:"enabled"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	clientID := strings.TrimSpace(input.ClientID)
	secret := strings.TrimSpace(input.ClientSecret)
	if len(clientID) > 256 || len(secret) > 512 || strings.ContainsAny(clientID+secret, "\r\n") {
		s.problem(w, 422, "invalid_provider", "Check the provider credentials")
		return
	}
	current, err := s.db.AuthProvider(r.Context(), provider)
	if err != nil {
		s.internal(w, err)
		return
	}
	hasSecret := current.HasSecret || secret != ""
	if input.Enabled && (clientID == "" || !hasSecret) {
		s.problem(w, 422, "provider_incomplete", "Add the client ID and secret before enabling this provider")
		return
	}
	if input.Enabled && s.cfg.OAuth == nil && !strings.HasPrefix(s.publicURL(), "https://") {
		s.problem(w, 409, "dashboard_required", "Set the dashboard domain before enabling provider sign-in")
		return
	}
	actor := r.Context().Value(sessionKey).(store.Session).User.ID
	if err = s.db.SaveAuthProvider(r.Context(), provider, clientID, secret, input.Enabled, actor, time.Now()); err != nil {
		s.userError(w, err)
		return
	}
	s.revokeStreams()
	s.authProviderList(w, r)
}

func (s *Server) removeAuthProvider(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	actor := r.Context().Value(sessionKey).(store.Session).User.ID
	if err := s.db.ClearAuthProvider(r.Context(), provider, actor, time.Now()); err != nil {
		s.userError(w, err)
		return
	}
	s.revokeStreams()
	s.json(w, 200, map[string]bool{"ok": true})
}

func (s *Server) testAuthProvider(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	info, err := s.db.AuthProvider(r.Context(), provider)
	if err != nil {
		s.internal(w, err)
		return
	}
	if !info.Enabled || info.ClientID == "" || !info.HasSecret {
		s.problem(w, 409, "provider_disabled", "Enable this provider before testing it")
		return
	}
	spec, ok := s.oauthSpec(provider)
	if !ok {
		s.problem(w, 404, "provider_not_found", "The sign-in provider was not found")
		return
	}
	session := r.Context().Value(sessionKey).(store.Session)
	target, state, err := s.oauthBegin(r.Context(), spec, info, "test", session.User.ID)
	if err != nil {
		s.internal(w, err)
		return
	}
	s.setOAuthStateCookie(w, r, state)
	s.json(w, 200, map[string]string{"url": target})
}

func (s *Server) savePasswordPolicy(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if input.Enabled == nil {
		s.problem(w, 422, "invalid_policy", "Choose whether password sign-in is enabled")
		return
	}
	actor := r.Context().Value(sessionKey).(store.Session).User.ID
	if err := s.db.SetPasswordLoginEnabled(r.Context(), *input.Enabled, actor, time.Now()); err != nil {
		s.userError(w, err)
		return
	}
	s.json(w, 200, map[string]bool{"ok": true})
}

func (s *Server) authMethods(w http.ResponseWriter, r *http.Request) {
	password, err := s.db.PasswordLoginEnabled(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	methods := map[string]bool{"password": password}
	for _, provider := range store.AuthProviders {
		item, err := s.db.AuthProvider(r.Context(), provider)
		if err != nil {
			s.internal(w, err)
			return
		}
		methods[provider] = item.Enabled && item.ClientID != "" && item.HasSecret
	}
	s.json(w, 200, methods)
}
