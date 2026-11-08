package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"shipper/internal/deploy"
	"shipper/internal/store"
)

func (s *Server) networkingRoutes(r chi.Router) {
	r.Get("/applications/{id}/networking", s.applicationNetworking)
}
func (s *Server) applicationNetworking(w http.ResponseWriter, r *http.Request) {
	a, err := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.resourceError(w, err)
		return
	}
	if !a.Managed() {
		s.problem(w, 422, "unmanaged_application", "Networking requires an application deployed by Shipper")
		return
	}
	apps, err := s.db.Applications(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	available := []map[string]string{}
	for _, other := range apps {
		if other.Managed() && other.ID != a.ID && other.State != "deleted" {
			available = append(available, map[string]string{"id": other.ID, "name": other.Name, "host": "app-" + other.ID})
		}
	}
	s.json(w, 200, map[string]any{"networking": a.Config.Networking, "host": "app-" + a.ID, "applications": available})
}
func (s *Server) validateNetworkingConfig(w http.ResponseWriter, c *store.Config, application string) bool {
	if err := deploy.ValidateNetworkAccess(application, &c.Networking); err != nil {
		s.problem(w, 422, "invalid_networking", err.Error())
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, id := range c.Networking.Applications {
		app, err := s.db.Application(ctx, id)
		if err != nil || !app.Managed() || app.State == "deleted" {
			s.problem(w, 422, "invalid_network_application", "Choose an application deployed by this Shipper instance")
			return false
		}
	}
	for _, p := range c.Networking.Ports {
		if c.Method != "compose" && p.Service != "" {
			s.problem(w, 422, "invalid_network_service", "Service ports require a Compose application")
			return false
		}
	}
	// Pending desired configurations also reserve ports, closing the interval
	// between review/enqueue and Docker's own atomic socket binding.
	apps, err := s.db.Applications(ctx)
	if err != nil {
		s.internal(w, err)
		return false
	}
	for _, app := range apps {
		if app.ID == application || app.State == "deleted" {
			continue
		}
		configurations := []store.Config{app.Config}
		if app.LatestDeployment != nil {
			configurations = append(configurations, app.LatestDeployment.Config)
		}
		for _, other := range configurations {
			for _, bound := range other.Networking.Ports {
				for _, port := range c.Networking.Ports {
					if bound.HostPort == port.HostPort && bound.Protocol == port.Protocol {
						s.problem(w, 409, "port_in_use", "The public port is reserved by another application")
						return false
					}
				}
			}
		}
	}
	if len(c.Networking.Ports) > 0 {
		if s.cfg.Worker == nil {
			s.problem(w, 503, "networking_unavailable", "Networking requires an available Docker runtime")
			return false
		}
		runtime, ok := s.cfg.Worker.Runtime.(*deploy.DockerRuntime)
		if !ok {
			s.problem(w, 503, "networking_unavailable", "Networking requires an available Docker runtime")
			return false
		}
		if err := runtime.ValidatePublishedPorts(ctx, application, c.Networking.Ports); err != nil {
			s.problem(w, 409, "port_unavailable", err.Error())
			return false
		}
	}
	return true
}
