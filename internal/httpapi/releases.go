package httpapi

import (
	"context"
	"github.com/go-chi/chi/v5"
	"net/http"
	"shipper/internal/deploy"
	"shipper/internal/store"
)

func (s *Server) rollbackRelease(w http.ResponseWriter, r *http.Request) {
	if !s.mutationAvailable(w) {
		return
	}
	var input struct {
		Revision string `json:"revision"`
		Review   bool   `json:"review"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	target, e := s.db.Deployment(r.Context(), chi.URLParam(r, "deployment"))
	if e != nil || target.ApplicationID != a.ID {
		s.problem(w, 404, "not_found", "The recorded release was not found")
		return
	}
	if target.State != "succeeded" || target.ImageID == "" || target.ID == a.ActiveDeploymentID || (target.Operation != "deploy" && target.Operation != "apply" && target.Operation != "rollback") {
		s.problem(w, 409, "invalid_release", "Choose a previous successful release")
		return
	}
	checker, ok := s.cfg.Worker.Runtime.(interface {
		CheckRelease(context.Context, store.Deployment) error
	})
	if !ok {
		s.problem(w, 503, "rollback_unavailable", "Release recovery is unavailable")
		return
	}
	if e = checker.CheckRelease(r.Context(), target); e != nil {
		s.problem(w, 409, "artifact_unavailable", e.Error())
		return
	}
	// Domains and trigger policy have independent histories and stay current.
	next := target.Config
	next.AutoDeploy = a.Config.AutoDeploy
	if next.SourceID == "" || next.Method == "image" || next.Method == "inline" {
		next.AutoDeploy = store.AutoDeploy{}
	}
	next.Domains = nil
	next.Domain = a.Config.Domain
	next.HTTPS = a.Config.HTTPS
	if !s.validateComposeDomains(w, r, next, a.Domains, target.ID) {
		return
	}
	if next.StartupPolicy == "process" && len(a.Domains) > 0 {
		s.problem(w, 409, "worker_domains", "Remove public domains before restoring this worker release")
		return
	}
	revision := s.db.ConfigRevision(a) + ":" + target.ID
	if input.Review {
		changes := deploy.PlanChanges(a.Config, next).Changes
		changes = append(changes, "Recorded image and runtime settings", "Current domains retained", "Persistent data retained", "Deployment migration hooks skipped")
		s.json(w, 200, deploy.ChangePlan{Operation: "rollback", Changes: changes, Revision: revision})
		return
	}
	if input.Revision != revision {
		s.problem(w, 409, "stale_review", "The application changed; review the release again")
		return
	}
	a.Config = next
	s.enqueueJob(w, r, a, store.Deployment{Operation: "rollback", RollbackID: target.ID, Commit: target.Commit}, false)
}
