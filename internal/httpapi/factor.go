package httpapi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skip2/go-qrcode"
	"shipper/internal/auth"
	"shipper/internal/store"
)

var errFactorGate = errors.New("factor required")

func (s *Server) factorRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(s.requireBrowserSession, s.csrf)
		r.Post("/auth/logout", s.logout)
		r.With(s.limit, s.hashCapacity).Post("/auth/factor/start", s.startFactor)
		r.With(s.limit).Post("/auth/factor/confirm", s.confirmFactor)
		r.With(s.limit).Post("/auth/factor/verify", s.verifyFactor)
		r.With(s.limit, s.hashCapacity).Post("/auth/factor/change", s.changeFactor)
	})
}

func (s *Server) requireBrowserSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := s.browserSession(r)
		if err != nil {
			s.problem(w, 401, "unauthenticated", "Sign in to continue")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, session)))
	})
}

func (s *Server) authChanges() <-chan struct{} {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	return s.authChanged
}

func (s *Server) revokeStreams() {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	close(s.authChanged)
	s.authChanged = make(chan struct{})
}

func (s *Server) writeSession(w http.ResponseWriter, status int, session store.Session, codes []string) {
	body := map[string]any{"setupRequired": false, "user": session.User, "csrfToken": session.CSRF, "factor": session.Factor}
	if step := session.AuthStep(); step != "" {
		body["authStep"] = step
	}
	if len(codes) != 0 {
		body["recoveryCodes"] = codes
	}
	s.json(w, status, body)
}

func sessionHash(r *http.Request) string {
	cookie, _ := r.Cookie(cookieName)
	return auth.Hash(cookie.Value)
}

func (s *Server) factorError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, store.ErrFactorCode):
		s.problem(w, 422, "invalid_factor", e.Error())
	case errors.Is(e, store.ErrEnrollment):
		s.problem(w, 409, "enrollment_expired", e.Error())
	case errors.Is(e, store.ErrFactorRequired):
		s.problem(w, 409, "factor_policy", e.Error())
	case errors.Is(e, sql.ErrNoRows):
		s.problem(w, 401, "unauthenticated", "Sign in to continue")
	default:
		s.internal(w, e)
	}
}

func (s *Server) verifyPassword(w http.ResponseWriter, r *http.Request, session store.Session, password string) bool {
	user, e := s.db.FindUser(r.Context(), session.User.Email)
	if e != nil || !auth.Verify(user.PasswordHash, password) {
		s.problem(w, 422, "invalid_password", "The password is incorrect")
		return false
	}
	return true
}

func (s *Server) startFactor(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password string `json:"password"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	session := r.Context().Value(sessionKey).(store.Session)
	if session.Factor.Enrolled {
		s.problem(w, 409, "already_enrolled", "Two-factor authentication is already enabled")
		return
	}
	if !s.verifyPassword(w, r, session, input.Password) {
		return
	}
	secret := auth.FactorSecret()
	account := session.User.Email
	if parsed, _ := url.Parse(s.cfg.PublicURL); parsed != nil && parsed.Hostname() != "" {
		account += " (" + parsed.Hostname() + ")"
	}
	image, e := qrcode.Encode(auth.FactorURI(secret, account), qrcode.Medium, 220)
	if e != nil {
		s.internal(w, e)
		return
	}
	if e = s.db.StartFactor(r.Context(), sessionHash(r), session.User.ID, secret, time.Now()); e != nil {
		s.factorError(w, e)
		return
	}
	s.json(w, 200, map[string]string{"key": secret, "qr": "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)})
}

func (s *Server) confirmFactor(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	session := r.Context().Value(sessionKey).(store.Session)
	if session.Factor.Enrolled {
		s.problem(w, 409, "already_enrolled", "Two-factor authentication is already enabled")
		return
	}
	token, csrf := auth.Token(), auth.Token()
	codes, e := s.db.ConfirmFactor(r.Context(), sessionHash(r), session.User.ID, input.Code, auth.Hash(token), csrf, time.Now())
	if e != nil {
		s.factorError(w, e)
		return
	}
	s.revokeStreams()
	s.factorSession(w, r, token, codes)
}

func (s *Server) verifyFactor(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	session := r.Context().Value(sessionKey).(store.Session)
	if session.AuthStep() != "verify" {
		s.problem(w, 409, "factor_not_pending", "There is no pending sign-in")
		return
	}
	token, csrf := auth.Token(), auth.Token()
	if e := s.db.VerifyFactor(r.Context(), sessionHash(r), session.User.ID, input.Code, auth.Hash(token), csrf, time.Now()); e != nil {
		s.factorError(w, e)
		return
	}
	s.factorSession(w, r, token, nil)
}

func (s *Server) changeFactor(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Action   string `json:"action"`
		Password string `json:"password"`
		Code     string `json:"code,omitempty"`
		Required *bool  `json:"required,omitempty"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	session := r.Context().Value(sessionKey).(store.Session)
	if session.AuthStep() != "" {
		s.problem(w, 403, "factor_required", "Complete two-factor authentication")
		return
	}
	if !s.verifyPassword(w, r, session, input.Password) {
		return
	}
	switch strings.TrimSpace(input.Action) {
	case "policy":
		if input.Required == nil {
			s.problem(w, 422, "invalid_policy", "Choose whether to require two-factor authentication")
			return
		}
		if e := s.db.SetFactorPolicy(r.Context(), sessionHash(r), session.User.ID, input.Code, *input.Required, time.Now()); e != nil {
			s.factorError(w, e)
			return
		}
		s.revokeStreams()
		fresh, e := s.browserSession(r)
		if e != nil {
			s.internal(w, e)
			return
		}
		s.writeSession(w, 200, fresh, nil)
	case "disable", "recovery_codes":
		token, csrf := auth.Token(), auth.Token()
		codes, e := s.db.ChangeFactor(r.Context(), sessionHash(r), session.User.ID, input.Code, auth.Hash(token), csrf, input.Action == "disable", time.Now())
		if e != nil {
			s.factorError(w, e)
			return
		}
		s.revokeStreams()
		s.factorSession(w, r, token, codes)
	default:
		s.problem(w, 422, "invalid_factor_action", "Choose a supported account action")
	}
}

func (s *Server) factorSession(w http.ResponseWriter, r *http.Request, token string, codes []string) {
	fresh, e := s.db.Session(r.Context(), auth.Hash(token))
	if e != nil {
		s.internal(w, e)
		return
	}
	s.cookie(w, token, fresh.ExpiresAt)
	s.writeSession(w, 200, fresh, codes)
}
