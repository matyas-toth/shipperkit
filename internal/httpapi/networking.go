package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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
	// Every application shares one private network, so its stable name is all
	// another application needs to reach it.
	host := "app-" + a.ID
	if a.Config.Database != nil {
		host = "db-" + a.ID
	}
	s.json(w, 200, map[string]any{"networking": a.Config.Networking, "host": host})
}

func (s *Server) validateNetworkingConfig(w http.ResponseWriter, c *store.Config, application string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := deploy.ValidateNetworkAccess(application, &c.Networking); err != nil {
		s.problem(w, 422, "invalid_networking", err.Error())
		return false
	}
	c.Networking.Applications = nil
	for _, p := range c.Networking.Ports {
		if c.Method != "compose" && p.Service != "" {
			s.problem(w, 422, "invalid_network_service", "Service ports require a Compose application")
			return false
		}
	}
	domain, domainPort := "", 0
	if c.Database != nil {
		c.Database.Domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(c.Database.Domain)), ".")
		if c.Database.Domain != "" {
			domain, domainPort = c.Database.Domain, store.DatabaseDomainPorts[c.Database.Engine]
		}
	}
	// Pending desired configurations also reserve ports, closing the interval
	// between review/enqueue and Docker's own atomic socket binding.
	apps, err := s.db.Applications(ctx)
	if err != nil {
		s.internal(w, err)
		return false
	}
	// The proxy already listens on the engine's port while another database of
	// the same engine has a domain, so only the first one needs a free port.
	proxyListens := false
	for _, app := range apps {
		if app.ID == application {
			// The database already serving a domain is the reason the proxy holds the port.
			if domain != "" && app.ActiveDeploymentID != "" {
				serving, e := s.db.Deployment(ctx, app.ActiveDeploymentID)
				proxyListens = proxyListens || e == nil && serving.Config.Database != nil && serving.Config.Database.Domain != "" && store.DatabaseDomainPorts[serving.Config.Database.Engine] == domainPort
			}
			continue
		}
		if app.State == "deleted" {
			continue
		}
		configurations := []store.Config{app.Config}
		if app.LatestDeployment != nil {
			configurations = append(configurations, app.LatestDeployment.Config)
		}
		for _, other := range configurations {
			otherPort := 0
			if other.Database != nil && other.Database.Domain != "" {
				otherPort = store.DatabaseDomainPorts[other.Database.Engine]
				if domain != "" && otherPort == domainPort {
					proxyListens = true
					if other.Database.Domain == domain {
						s.problem(w, 409, "domain_in_use", "Another database already uses this domain")
						return false
					}
				}
			}
			for _, bound := range other.Networking.Ports {
				if domain != "" && bound.HostPort == domainPort && bound.Protocol == "tcp" {
					s.problem(w, 409, "port_in_use", fmt.Sprintf("Port %d is published by %s. Remove it there to serve this domain", domainPort, app.Name))
					return false
				}
				for _, port := range c.Networking.Ports {
					if bound.HostPort == port.HostPort && bound.Protocol == port.Protocol {
						s.problem(w, 409, "port_in_use", "The public port is reserved by another application")
						return false
					}
				}
			}
			for _, port := range c.Networking.Ports {
				if otherPort != 0 && port.HostPort == otherPort && port.Protocol == "tcp" {
					s.problem(w, 409, "port_in_use", fmt.Sprintf("Port %d serves the public domain of %s", otherPort, app.Name))
					return false
				}
			}
		}
	}
	ports := c.Networking.Ports
	if domain != "" && domainPort != 0 && !proxyListens {
		ports = append(append([]store.PublishedPort{}, ports...), store.PublishedPort{Protocol: "tcp", BindIP: "0.0.0.0", HostPort: domainPort, ContainerPort: domainPort})
	}
	if len(ports) > 0 {
		if s.cfg.Worker == nil {
			s.problem(w, 503, "networking_unavailable", "Networking requires an available Docker runtime")
			return false
		}
		runtime, ok := s.cfg.Worker.Runtime.(*deploy.DockerRuntime)
		if !ok {
			s.problem(w, 503, "networking_unavailable", "Networking requires an available Docker runtime")
			return false
		}
		if err := runtime.ValidatePublishedPorts(ctx, application, ports); err != nil {
			s.problem(w, 409, "port_unavailable", err.Error())
			return false
		}
	}
	return true
}
