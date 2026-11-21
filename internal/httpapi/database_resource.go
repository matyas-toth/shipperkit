package httpapi

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"log/slog"
	"net/http"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/store"
	"time"
)

func (s *Server) databaseResourceRoutes(r chi.Router) {
	r.With(s.csrf, s.limit).Post("/database-resources/prepare", s.prepareDatabaseResource)
	r.With(s.csrf, s.limit).Post("/applications/{id}/database/import", s.importDatabaseDump)
	r.With(s.csrf, s.limit).Post("/applications/{id}/database/password", s.rotateDatabasePassword)
}
func (s *Server) prepareDatabaseResource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Engine   string `json:"engine"`
		Version  string `json:"version"`
		User     string `json:"user"`
		Database string `json:"database"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	config, err := deploy.NamedDatabaseResourceConfig(input.Engine, input.Version, input.User, input.Database)
	if err != nil {
		s.problem(w, 422, "invalid_database", err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.json(w, 200, map[string]any{"config": config})
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
	if r.FormValue("revision") != s.db.ConfigRevision(a) || (a.Config.Database.Engine == "redis" && r.FormValue("confirm") != a.Name) {
		message := "Review the current database before importing a copy"
		if a.Config.Database.Engine == "redis" {
			message = "Review the current database and enter its name to import a snapshot"
		}
		s.problem(w, 409, "review_changed", message)
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
func (s *Server) rotateDatabasePassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if p := principal(r); p != nil && (!p.Allows("secrets", id) || !p.Allows("destructive", id)) {
		s.problem(w, 403, "capability_denied", "Rotating a database password requires secrets and destructive permissions")
		return
	}
	if !s.mutationAvailable(w) {
		return
	}
	var input struct {
		Review   bool   `json:"review"`
		Revision string `json:"revision"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	a, e := s.db.Application(r.Context(), id)
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if a.Config.Database == nil || deploy.DatabasePasswordKey(a.Config.Database.Engine) == "" {
		s.problem(w, 422, "invalid_database", "This database has no password to rotate")
		return
	}
	if a.ActiveDeploymentID == "" {
		s.problem(w, 409, "database_unavailable", "Deploy the database before rotating its password")
		return
	}
	revision := s.db.ConfigRevision(a)
	if input.Review {
		s.json(w, 200, map[string]string{"revision": revision})
		return
	}
	if input.Revision != revision {
		s.problem(w, 409, "review_changed", "The database changed; review the rotation again")
		return
	}
	if busy, e := s.db.ApplicationBusy(r.Context(), a.ID); e != nil || busy {
		s.problem(w, 409, "operation_in_progress", store.ErrBusy.Error())
		return
	}
	runtime, ok := s.cfg.Worker.Runtime.(*deploy.DockerRuntime)
	if !ok {
		s.problem(w, 503, "unavailable", "Docker is unavailable")
		return
	}
	d, e := s.db.Deployment(r.Context(), a.ActiveDeploymentID)
	if e != nil {
		s.internal(w, e)
		return
	}
	key := deploy.DatabasePasswordKey(a.Config.Database.Engine)
	previous := ""
	for _, v := range d.Config.Variables {
		if v.Name == key {
			previous = v.Value
		}
	}
	password := auth.Token()
	next, e := deploy.WithDatabasePassword(a.Config, password)
	if e != nil {
		s.problem(w, 422, "invalid_database", e.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if e = runtime.SetDatabasePassword(ctx, d, password); e != nil {
		s.problem(w, 422, "rotation_failed", e.Error())
		return
	}
	a.Config = next
	job := store.Deployment{ID: auth.Token()[:24], Operation: "apply", CredentialRotation: true, Actor: r.Context().Value(sessionKey).(store.Session).User.Email}
	queued, e := s.db.Enqueue(r.Context(), a, job, "", false)
	if e != nil {
		// Keep the database on the password every saved setting still uses.
		if previous != "" {
			_ = runtime.SetDatabasePassword(context.WithoutCancel(ctx), d, previous)
		}
		if errors.Is(e, store.ErrBusy) {
			s.problem(w, 409, "operation_in_progress", e.Error())
			return
		}
		s.internal(w, e)
		return
	}
	_ = s.db.AppendLog(r.Context(), queued.ID, "queued", "Database password rotated; connected applications need the new connection string")
	// Scheduled backups sign in as the same user.
	if plans, e := s.db.BackupPlans(r.Context(), a.ID); e == nil {
		for _, p := range plans {
			if p.Config.Kind == "database" && previous != "" && p.Config.Password == previous {
				p.Config.Password = password
				if e := s.db.SaveBackupPlan(r.Context(), p); e != nil {
					slog.Warn("backup plan password update failed", "plan", p.ID, "error", e)
				}
			}
		}
	}
	s.cfg.Worker.Trigger()
	s.hub.Publish()
	s.json(w, 202, publicDeployment(r, queued))
}
