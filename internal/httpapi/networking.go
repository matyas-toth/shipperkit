package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"reflect"
	"regexp"
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

// privateHost matches the stable private names Shipper publishes for every
// application, as they appear in copied connection strings and URLs.
var privateHost = regexp.MustCompile(`\b(?:app|db)-([a-f0-9]{24})\b`)

// linkReferencedApplications connects an application to every other
// application its variables already address by private name. Pasting a copied
// connection string is the intent to connect, so it works on the first deploy.
func (s *Server) linkReferencedApplications(ctx context.Context, c *store.Config, application string) {
	linked := map[string]bool{application: true}
	for _, id := range c.Networking.Applications {
		linked[id] = true
	}
	variables := append([]store.Variable{}, c.Variables...)
	for _, service := range c.Services {
		variables = append(variables, service.Variables...)
	}
	for _, v := range variables {
		for _, match := range privateHost.FindAllStringSubmatch(v.Value, -1) {
			id := match[1]
			if linked[id] {
				continue
			}
			linked[id] = true
			if app, err := s.db.Application(ctx, id); err == nil && app.Managed() && app.State != "deleted" {
				c.Networking.Applications = append(c.Networking.Applications, id)
			}
		}
	}
}

// describeConnections names the applications a change connects or disconnects,
// which a bare "Networking" entry hides, including links made automatically.
func (s *Server) describeConnections(ctx context.Context, changes []string, before, after store.NetworkAccess) []string {
	had := map[string]bool{}
	for _, id := range before.Applications {
		had[id] = true
	}
	has := map[string]bool{}
	lines := []string{}
	name := func(id string) string {
		if app, err := s.db.Application(ctx, id); err == nil {
			return app.Name
		}
		return "a deleted application"
	}
	for _, id := range after.Applications {
		has[id] = true
		if !had[id] {
			lines = append(lines, "Connects privately to "+name(id))
		}
	}
	for _, id := range before.Applications {
		if !has[id] {
			lines = append(lines, "Disconnects from "+name(id))
		}
	}
	if len(lines) == 0 {
		return changes
	}
	out := []string{}
	for _, change := range changes {
		// Ports still need the generic entry; connections are listed by name.
		if change != "Networking" || !reflect.DeepEqual(before.Ports, after.Ports) {
			out = append(out, change)
		}
	}
	return append(out, lines...)
}

func (s *Server) validateNetworkingConfig(w http.ResponseWriter, c *store.Config, application string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.linkReferencedApplications(ctx, c, application)
	if err := deploy.ValidateNetworkAccess(application, &c.Networking); err != nil {
		s.problem(w, 422, "invalid_networking", err.Error())
		return false
	}
	kept := []string{}
	for _, id := range c.Networking.Applications {
		app, err := s.db.Application(ctx, id)
		if errors.Is(err, sql.ErrNoRows) || err == nil && app.State == "deleted" {
			// A deleted application's connection is dropped instead of blocking
			// every later change to the applications that listed it.
			continue
		}
		if err != nil || !app.Managed() {
			s.problem(w, 422, "invalid_network_application", "Choose an application deployed by this Shipper instance")
			return false
		}
		kept = append(kept, id)
	}
	if c.Networking.Applications != nil {
		c.Networking.Applications = kept
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
