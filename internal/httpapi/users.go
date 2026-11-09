package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"shipper/internal/auth"
	"shipper/internal/store"
)

func (s *Server) userRoutes(r chi.Router) {
	r.Get("/users", s.users)
	r.With(s.csrf, s.limit).Post("/users", s.createUser)
	r.Get("/users/{user}", s.user)
	r.Get("/users/{user}/audit", s.userAudit)
	r.With(s.csrf, s.limit).Post("/users/{user}/role", s.updateUserRole)
	r.With(s.csrf, s.limit).Post("/users/{user}/disable", s.disableUser)
	r.With(s.csrf, s.limit).Post("/users/{user}/enable", s.enableUser)
	r.With(s.csrf, s.limit).Post("/users/{user}/password", s.resetUserPassword)
	r.With(s.csrf, s.limit).Post("/users/{user}/password/remove", s.removeUserPassword)
	r.With(s.csrf, s.limit).Post("/users/{user}/factor/reset", s.resetUserFactor)
	r.With(s.csrf, s.limit).Post("/users/{user}/identities/{provider}/unlink", s.unlinkIdentity)
	r.With(s.csrf, s.limit).Post("/users/{user}/delete", s.deleteUser)
}

func normalizeEmail(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func (s *Server) userError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrUserNotFound), errors.Is(err, sql.ErrNoRows):
		s.problem(w, 404, "user_not_found", "The user was not found")
	case errors.Is(err, store.ErrLastAdmin):
		s.problem(w, 409, "last_admin", err.Error())
	case errors.Is(err, store.ErrIdentityConflict), errors.Is(err, store.ErrProviderLinked):
		s.problem(w, 409, "identity_conflict", err.Error())
	default:
		s.internal(w, err)
	}
}

func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.Users(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	s.json(w, 200, map[string]any{"items": items})
}

func (s *Server) user(w http.ResponseWriter, r *http.Request) {
	item, err := s.db.User(r.Context(), chi.URLParam(r, "user"))
	if err != nil {
		s.userError(w, err)
		return
	}
	s.json(w, 200, item)
}

func (s *Server) userAudit(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.UserAudit(r.Context(), chi.URLParam(r, "user"))
	if err != nil {
		s.internal(w, err)
		return
	}
	s.json(w, 200, map[string]any{"items": items})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	email := normalizeEmail(input.Email)
	if !validEmail(email) {
		s.problem(w, 422, "invalid_email", "Enter a valid email address")
		return
	}
	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = store.RoleViewer
	}
	if !store.ValidRole(role) {
		s.problem(w, 422, "invalid_role", "Choose a valid role")
		return
	}
	exists, err := s.db.UserEmailExists(r.Context(), email)
	if err != nil {
		s.internal(w, err)
		return
	}
	if exists {
		s.problem(w, 409, "email_exists", "A user with that email already exists")
		return
	}
	user := store.User{ID: auth.Token()[:24], Email: email, Role: role, CreatedAt: time.Now().Unix()}
	if strings.TrimSpace(input.Password) != "" {
		hash, err := auth.Password(input.Password)
		if err != nil {
			s.problem(w, 422, "invalid_password", err.Error())
			return
		}
		user.PasswordHash = hash
		user.HasPassword = true
		user.PasswordChangeRequired = true
	}
	actor := r.Context().Value(sessionKey).(store.Session).User.ID
	if err = s.db.CreateUser(r.Context(), user, actor, time.Now()); err != nil {
		s.userError(w, err)
		return
	}
	s.json(w, 201, user)
}

func (s *Server) updateUserRole(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Role string `json:"role"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if !store.ValidRole(input.Role) {
		s.problem(w, 422, "invalid_role", "Choose a valid role")
		return
	}
	actor := r.Context().Value(sessionKey).(store.Session).User.ID
	if err := s.db.UpdateUserRole(r.Context(), chi.URLParam(r, "user"), input.Role, actor, time.Now()); err != nil {
		s.userError(w, err)
		return
	}
	s.json(w, 200, map[string]bool{"ok": true})
}

func (s *Server) setDisabled(disabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := r.Context().Value(sessionKey).(store.Session).User.ID
		id := chi.URLParam(r, "user")
		if err := s.db.SetUserDisabled(r.Context(), id, disabled, actor, time.Now()); err != nil {
			s.userError(w, err)
			return
		}
		if disabled && id == actor {
			s.revokeStreams()
		}
		s.json(w, 200, map[string]bool{"ok": true})
	}
}

func (s *Server) disableUser(w http.ResponseWriter, r *http.Request) { s.setDisabled(true)(w, r) }
func (s *Server) enableUser(w http.ResponseWriter, r *http.Request)  { s.setDisabled(false)(w, r) }

func (s *Server) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password string `json:"password"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	hash, err := auth.Password(input.Password)
	if err != nil {
		s.problem(w, 422, "invalid_password", err.Error())
		return
	}
	actor := r.Context().Value(sessionKey).(store.Session).User.ID
	if err = s.db.ResetUserPassword(r.Context(), chi.URLParam(r, "user"), hash, true, actor, time.Now()); err != nil {
		s.userError(w, err)
		return
	}
	s.revokeStreams()
	s.json(w, 200, map[string]bool{"ok": true})
}

func (s *Server) removeUserPassword(w http.ResponseWriter, r *http.Request) {
	actor := r.Context().Value(sessionKey).(store.Session).User.ID
	if err := s.db.RemoveUserPassword(r.Context(), chi.URLParam(r, "user"), actor, time.Now()); err != nil {
		s.userError(w, err)
		return
	}
	s.json(w, 200, map[string]bool{"ok": true})
}

func (s *Server) resetUserFactor(w http.ResponseWriter, r *http.Request) {
	actor := r.Context().Value(sessionKey).(store.Session).User.ID
	if err := s.db.ResetUserFactor(r.Context(), chi.URLParam(r, "user"), actor, time.Now()); err != nil {
		s.userError(w, err)
		return
	}
	s.revokeStreams()
	s.json(w, 200, map[string]bool{"ok": true})
}

func (s *Server) unlinkIdentity(w http.ResponseWriter, r *http.Request) {
	actor := r.Context().Value(sessionKey).(store.Session).User.ID
	if err := s.db.UnlinkIdentity(r.Context(), chi.URLParam(r, "user"), chi.URLParam(r, "provider"), actor, time.Now()); err != nil {
		s.userError(w, err)
		return
	}
	s.json(w, 200, map[string]bool{"ok": true})
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	actor := r.Context().Value(sessionKey).(store.Session).User.ID
	id := chi.URLParam(r, "user")
	if err := s.db.DeleteUser(r.Context(), id, actor, time.Now()); err != nil {
		s.userError(w, err)
		return
	}
	if id == actor {
		s.revokeStreams()
	}
	s.json(w, 200, map[string]bool{"ok": true})
}
