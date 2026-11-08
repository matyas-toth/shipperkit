package httpapi

import (
	"context"
	"github.com/go-chi/chi/v5"
	"net/http"
	"shipper/internal/deploy"
	"time"
)

func (s *Server) databaseResourceRoutes(r chi.Router) {
	r.With(s.csrf, s.limit).Post("/database-resources/prepare", s.prepareDatabaseResource)
	r.With(s.csrf, s.limit).Post("/applications/{id}/database/ca", s.databaseCA)
	r.With(s.csrf, s.limit).Post("/applications/{id}/database/import", s.importDatabaseDump)
}
func (s *Server) prepareDatabaseResource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Engine  string `json:"engine"`
		Version string `json:"version"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	config, err := deploy.DatabaseResourceConfig(input.Engine, input.Version)
	if err != nil {
		s.problem(w, 422, "invalid_database", err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.json(w, 200, map[string]any{"config": config})
}
func (s *Server) databaseCA(w http.ResponseWriter, r *http.Request) {
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if a.Config.Database == nil {
		s.problem(w, 422, "invalid_database", "Choose a managed database")
		return
	}
	if s.cfg.Worker == nil {
		s.problem(w, 503, "unavailable", "Database protection is unavailable")
		return
	}
	runtime, ok := s.cfg.Worker.Runtime.(*deploy.DockerRuntime)
	if !ok {
		s.problem(w, 503, "unavailable", "Docker is unavailable")
		return
	}
	ca, e := runtime.DatabaseCA(a.ID)
	if e != nil {
		s.problem(w, 409, "ca_unavailable", "Deploy the database before downloading its CA")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="database-ca.crt"`)
	w.Write(ca)
}
func (s *Server) importDatabaseDump(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if p := principal(r); p != nil && (!p.Allows("secrets", id) || !p.Allows("destructive", id)) {
		s.problem(w, 403, "capability_denied", "Database imports require secrets and destructive permissions")
		return
	}
	a, e := s.db.Application(r.Context(), id)
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if a.Config.Database == nil || a.ActiveDeploymentID == "" {
		s.problem(w, 409, "database_unavailable", "Deploy the database before importing a dump")
		return
	}
	if s.cfg.Worker == nil {
		s.problem(w, 503, "unavailable", "Database protection is unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, deploy.MaxDatabaseImportBytes+(1<<20))
	if e = r.ParseMultipartForm(1 << 20); e != nil {
		s.problem(w, 413, "invalid_upload", "Choose a database dump smaller than 512 MiB")
		return
	}
	defer r.MultipartForm.RemoveAll()
	if r.FormValue("confirm") != a.Name || r.FormValue("revision") != s.db.ConfigRevision(a) {
		s.problem(w, 409, "review_changed", "Review the current database and enter its name to import a copy")
		return
	}
	f, _, e := r.FormFile("file")
	if e != nil {
		s.problem(w, 422, "missing_dump", "Choose a native database dump")
		return
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	run, e := s.cfg.Worker.ImportDatabaseDump(ctx, id, r.FormValue("target"), f)
	if e != nil {
		s.problem(w, 422, "import_failed", e.Error())
		return
	}
	s.json(w, 202, run)
}
