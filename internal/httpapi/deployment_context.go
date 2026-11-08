package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"shipper/internal/deploy"
	"shipper/internal/store"
)

// deploymentContext exports only the selected deployment's retained logs and
// source identity. Runtime variables, credentials and command configuration are
// deliberately omitted from the portable handoff.
func (s *Server) deploymentContext(w http.ResponseWriter, r *http.Request) {
	d, err := s.db.Deployment(r.Context(), chi.URLParam(r, "deployment"))
	if err != nil || d.ApplicationID != chi.URLParam(r, "id") {
		s.problem(w, 404, "not_found", "The deployment was not found")
		return
	}
	a, err := s.db.Application(r.Context(), d.ApplicationID)
	if err != nil {
		s.resourceError(w, err)
		return
	}
	redact := func(value string) string { return deploy.Redact(a.Config, deploy.RedactDeployment(d, value)) }
	logs := []store.LogLine{}
	stage := ""
	var cursor int64
	for {
		batch, e := s.db.Logs(r.Context(), d.ID, cursor)
		if e != nil {
			s.internal(w, e)
			return
		}
		for i := range batch {
			if batch[i].Step != "failed" && batch[i].Step != "cancelled" && batch[i].Step != "interrupted" {
				stage = batch[i].Step
			}
			batch[i].Line = redact(batch[i].Line)
		}
		logs = append(logs, batch...)
		if len(batch) < 500 {
			break
		}
		cursor = batch[len(batch)-1].Sequence
	}
	stamp := func(at int64) any {
		if at == 0 {
			return nil
		}
		return time.Unix(at, 0).UTC().Format(time.RFC3339)
	}
	end := d.FinishedAt
	if end == 0 {
		end = time.Now().Unix()
	}
	s.json(w, 200, map[string]any{
		"schemaVersion": "1", "exportedAt": stamp(time.Now().Unix()),
		"application": map[string]any{"id": a.ID, "name": a.Name, "state": a.State, "desiredState": a.DesiredState, "servingDeploymentId": a.ActiveDeploymentID},
		"deployment":  map[string]any{"id": d.ID, "operation": d.Operation, "status": d.State, "stage": stage, "buildSystem": d.ResolvedMethod, "error": redact(d.Error), "createdAt": stamp(d.CreatedAt), "startedAt": stamp(d.StartedAt), "finishedAt": stamp(d.FinishedAt), "durationSeconds": max(0, end-d.CreatedAt)},
		"source":      map[string]any{"repository": d.Config.Repository, "branch": d.Config.Branch, "commit": d.Commit, "directory": d.Config.RootDirectory, "image": d.Config.Image, "digest": d.Digest, "localSnapshotId": d.Config.LocalSource},
		"logs":        logs, "logScope": "All retained deployment logs", "logLimit": 10000,
	})
}
