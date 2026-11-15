package httpapi

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) catalogIndex(w http.ResponseWriter, r *http.Request) {
	index := s.catalog.Index()
	checkedAt, cached, lastError := s.catalog.State()
	s.json(w, 200, map[string]any{
		"version": index.Version, "source": index.Source, "generated": index.Generated,
		"updatedAt": index.UpdatedAt, "checkedAt": checkedAt, "fromCache": cached, "error": lastError,
		"categories": index.Categories, "items": index.Items,
	})
}

func (s *Server) catalogService(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "service"))
	if id == "" || len(id) > 80 {
		s.problem(w, 404, "not_found", "The service was not found")
		return
	}
	service, ok := s.catalog.Current().Get(id)
	if !ok {
		s.problem(w, 404, "not_found", "The service was not found")
		return
	}
	s.json(w, 200, service)
}

func (s *Server) refreshCatalog(w http.ResponseWriter, r *http.Request) {
	remote := strings.TrimSpace(os.Getenv("SHIPPER_CATALOG_URL"))
	if remote == "off" || remote == "disabled" {
		s.problem(w, 409, "catalog_updates_disabled", "Catalog updates are disabled on this installation")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := s.catalog.Refresh(ctx, remote); err != nil {
		s.problem(w, 502, "catalog_refresh_failed", err.Error())
		return
	}
	index := s.catalog.Index()
	s.json(w, 200, map[string]any{"version": index.Version, "count": len(index.Items), "updatedAt": index.UpdatedAt})
}
