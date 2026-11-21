package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"shipper/internal/deploy"
)

func (s *Server) databaseConnection(w http.ResponseWriter, r *http.Request) {
	var input struct {
		// Public asks for the string that connects through the database's domain.
		Public bool `json:"public"`
	}
	if !s.decodeOptional(w, r, &input) {
		return
	}
	a, err := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.resourceError(w, err)
		return
	}
	if a.ActiveDeploymentID == "" {
		s.problem(w, 409, "database_unavailable", "Deploy the database before copying its connection string")
		return
	}
	d, err := s.db.Deployment(r.Context(), a.ActiveDeploymentID)
	if err != nil {
		s.resourceError(w, err)
		return
	}
	access := deploy.DatabaseAccess{Public: input.Public}
	if s.cfg.Worker != nil && !input.Public {
		if runtime, ok := s.cfg.Worker.Runtime.(*deploy.DockerRuntime); ok {
			access.LegacyTLS = runtime.DatabaseLegacyTLS(r.Context(), d)
		}
	}
	uri, err := deploy.DatabaseConnectionURI(d, access)
	if err != nil {
		s.problem(w, 422, "connection_unavailable", err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.json(w, 200, map[string]string{"uri": uri, "deploymentId": d.ID})
}
