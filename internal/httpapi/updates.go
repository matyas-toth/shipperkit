package httpapi

import (
	"errors"
	"net/http"

	"shipper/internal/updates"
)

func (s *Server) updateState(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Updates == nil {
		s.json(w, 200, updates.State{Enabled: false, CurrentVersion: s.cfg.Version, Channel: updates.Main, Status: updates.Status{State: "idle"}})
		return
	}
	s.json(w, 200, s.cfg.Updates.State())
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Updates == nil {
		s.json(w, 200, updates.Status{State: "idle"})
		return
	}
	s.json(w, 200, s.cfg.Updates.Status())
}

func (s *Server) checkUpdates(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Updates == nil {
		s.problem(w, 503, "updates_unavailable", "Updates are unavailable in this installation")
		return
	}
	state, err := s.cfg.Updates.Check(r.Context())
	if !state.Enabled {
		s.problem(w, 503, "updates_unavailable", "Updates are unavailable in this installation")
		return
	}
	if err == nil {
		if autoErr := s.cfg.Updates.MaybeAutoApply(r.Context()); autoErr != nil {
			state = s.cfg.Updates.State()
			state.Error = autoErr.Error()
		} else {
			state = s.cfg.Updates.State()
		}
	}
	s.json(w, 200, state)
}

func (s *Server) saveUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Updates == nil {
		s.problem(w, 503, "updates_unavailable", "Updates are unavailable in this installation")
		return
	}
	var input struct {
		Channel string `json:"channel"`
		Auto    bool   `json:"auto"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	state, err := s.cfg.Updates.SetSettings(r.Context(), updates.Channel(input.Channel), input.Auto)
	if err != nil {
		s.problem(w, 422, "invalid_update_settings", err.Error())
		return
	}
	state, _ = s.cfg.Updates.Check(r.Context())
	if err = s.cfg.Updates.MaybeAutoApply(r.Context()); err != nil {
		state = s.cfg.Updates.State()
		state.Error = err.Error()
	} else {
		state = s.cfg.Updates.State()
	}
	s.json(w, 200, state)
}

func (s *Server) applyUpdate(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Updates == nil {
		s.problem(w, 503, "updates_unavailable", "Updates are unavailable in this installation")
		return
	}
	status, err := s.cfg.Updates.Apply(r.Context())
	if err != nil {
		switch {
		case errors.Is(err, updates.ErrInProgress):
			s.problem(w, 409, "update_in_progress", err.Error())
		case errors.Is(err, updates.ErrNoUpdate):
			s.problem(w, 409, "no_update", err.Error())
		case errors.Is(err, updates.ErrUnavailable):
			s.problem(w, 503, "updates_unavailable", err.Error())
		default:
			s.problem(w, 422, "update_failed", err.Error())
		}
		return
	}
	s.json(w, 202, status)
}
