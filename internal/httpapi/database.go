package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"shipper/internal/deploy"
)

func (s *Server) databaseConnection(w http.ResponseWriter, r *http.Request) {
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
	uri, err := deploy.DatabaseURI(d)
	if err != nil {
		s.problem(w, 422, "connection_unavailable", err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.json(w, 200, map[string]string{"uri": uri, "deploymentId": d.ID})
}
