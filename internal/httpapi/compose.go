package httpapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"shipper/internal/deploy"
	"shipper/internal/store"
)

func (s *Server) previewCompose(w http.ResponseWriter, r *http.Request) {
	if !s.mutationAvailable(w) {
		return
	}
	var input struct {
		Config        store.Config `json:"config"`
		ApplicationID string       `json:"applicationId"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	c := input.Config
	c.Method = "compose"
	if input.ApplicationID != "" {
		a, e := s.db.Application(r.Context(), input.ApplicationID)
		if e != nil {
			s.resourceError(w, e)
			return
		}
		var release store.Deployment
		if a.ActiveDeploymentID != "" {
			release, e = s.db.Deployment(r.Context(), a.ActiveDeploymentID)
			if e != nil {
				s.internal(w, e)
				return
			}
		}
		if e = store.MergeReleaseVariables(&c, a.Config, release); e != nil {
			s.problem(w, 422, "invalid_configuration", e.Error())
			return
		}
	}
	if !s.validateConfig(w, &c, "preview") {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	root := filepath.Join(s.cfg.Worker.DataDir, "work")
	if e := os.MkdirAll(root, 0700); e != nil {
		s.internal(w, e)
		return
	}
	dir, e := os.MkdirTemp(root, "inspect-")
	if e != nil {
		s.internal(w, e)
		return
	}
	defer os.RemoveAll(dir)
	repository := filepath.Join(dir, "repository")
	if c.ComposeSource == "paste" {
		e = os.MkdirAll(repository, 0700)
	} else {
		_, e = s.cfg.Worker.Source.Clone(ctx, c, dir, func(string) {})
	}
	if e != nil {
		s.problem(w, 422, "repository_unavailable", deploy.Redact(c, e.Error()))
		return
	}
	appRoot, e := deploy.SourceRoot(repository, c)
	if e != nil {
		s.problem(w, 422, "invalid_configuration", e.Error())
		return
	}
	files := []string{}
	if c.ComposeSource != "paste" {
		filepath.WalkDir(appRoot, func(file string, entry os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if entry.IsDir() {
				if entry.Name() == ".git" || entry.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			name := strings.ToLower(entry.Name())
			if (strings.Contains(name, "compose") && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml"))) && len(files) < 64 {
				rel, _ := filepath.Rel(appRoot, file)
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		if c.ComposeFile == "compose.yaml" {
			if _, e = os.Stat(filepath.Join(appRoot, c.ComposeFile)); os.IsNotExist(e) && len(files) > 0 {
				c.ComposeFile = files[0]
			}
		}
	}
	p, e := deploy.LoadCompose(ctx, c, appRoot, "shipper-preview", repository)
	if e != nil {
		s.problem(w, 422, "invalid_compose", deploy.Redact(c, e.Error()))
		return
	}
	s.json(w, 200, map[string]any{"services": deploy.ComposeServices(p, c), "files": files, "composeFile": c.ComposeFile})
}

func (s *Server) composeSource(w http.ResponseWriter, r *http.Request) {
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	s.json(w, 200, map[string]string{"content": a.Config.ComposeYAML})
}

func (s *Server) validateComposeDomains(w http.ResponseWriter, r *http.Request, c store.Config, domains []store.Domain, active string) bool {
	if c.Method != "compose" {
		for _, d := range domains {
			if d.Service != "" {
				s.problem(w, 422, "invalid_service", "Service domains require a Compose application")
				return false
			}
		}
		return true
	}
	services := []store.Service{}
	if active != "" {
		release, e := s.db.Deployment(r.Context(), active)
		if e != nil {
			s.internal(w, e)
			return false
		}
		services = release.Services
	}
	for _, domain := range domains {
		if domain.Service == "" {
			s.problem(w, 422, "invalid_service", "Choose a service for each domain")
			return false
		}
		if active != "" {
			found := false
			for _, svc := range services {
				if svc.Name == domain.Service && svc.Port > 0 {
					found = true
				}
			}
			if !found {
				s.problem(w, 422, "invalid_service", "Set an HTTP port for the service and deploy it before attaching a domain")
				return false
			}
		}
	}
	return true
}
