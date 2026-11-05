package httpapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"shipper/internal/deploy"
	"shipper/internal/store"
)

func (s *Server) inspectSource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Config store.Config `json:"config"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if !s.mutationAvailable(w) {
		return
	}
	c := input.Config
	c.Hooks = nil
	c.Domain = "inspection.localhost"
	c.Domains = nil
	c.HTTPS = false
	c.ComposeSource = "git"
	if c.Repository == "" && c.LocalSource == "" {
		s.problem(w, 422, "git_required", "Choose a Git repository")
		return
	}
	// Inspect source identity and files without requiring valid runtime options.
	c.Method = "auto"
	c.Port = 0
	c.StartupPolicy = ""
	if e := s.validateSource(r.Context(), &c); e != nil {
		s.problem(w, 422, "invalid_source", e.Error())
		return
	}
	if e := deploy.Validate(&c); e != nil {
		s.problem(w, 422, "invalid_source", e.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	root := filepath.Join(s.cfg.Worker.DataDir, "work")
	if e := os.MkdirAll(root, 0700); e != nil {
		s.internal(w, e)
		return
	}
	dir, e := os.MkdirTemp(root, "inspection-")
	if e != nil {
		s.internal(w, e)
		return
	}
	defer os.RemoveAll(dir)
	commit, e := s.cfg.Worker.Source.Clone(ctx, c, dir, func(string) {})
	if e != nil {
		s.problem(w, 422, "inspection_failed", e.Error())
		return
	}
	source, e := deploy.SourceRoot(filepath.Join(dir, "repository"), c)
	if e != nil {
		s.problem(w, 422, "invalid_directory", e.Error())
		return
	}
	result := deploy.InspectSource(source)
	if c.LocalSource == "" {
		result.Commit = commit
	}
	s.json(w, 200, result)
}
