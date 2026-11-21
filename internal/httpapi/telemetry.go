package httpapi

import "net/http"

func (s *Server) telemetryState(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Telemetry == nil {
		s.json(w, 200, map[string]any{"allowed": false, "enabled": false, "lastSent": 0})
		return
	}
	state, err := s.cfg.Telemetry.State(r.Context())
	if err != nil {
		s.problem(w, 500, "telemetry_unavailable", "Usage data could not be prepared")
		return
	}
	s.json(w, 200, state)
}

func (s *Server) saveTelemetry(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if s.cfg.Telemetry == nil {
		s.problem(w, 409, "telemetry_unavailable", "Usage data is turned off by the environment")
		return
	}
	if err := s.cfg.Telemetry.SetEnabled(r.Context(), input.Enabled); err != nil {
		s.problem(w, 409, "telemetry_unavailable", err.Error())
		return
	}
	s.telemetryState(w, r)
}
