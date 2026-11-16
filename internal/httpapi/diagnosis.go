package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"shipper/internal/diagnosis"
)

func (s *Server) deploymentDiagnosis(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	deploymentID := chi.URLParam(r, "deployment")
	result, e := diagnosis.DiagnoseDeployment(r.Context(), s.db, appID, deploymentID, s.monitor.Health())
	if e != nil {
		s.resourceError(w, e)
		return
	}
	s.json(w, 200, result)
}
