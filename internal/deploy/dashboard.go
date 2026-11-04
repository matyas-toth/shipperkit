package deploy

import (
	"context"
	"errors"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"net/url"
	"shipper/internal/store"
	"strings"
)

func ValidateDashboard(c ProxyConfig) error {
	u, e := url.Parse(c.DashboardURL)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return errors.New("Dashboard HTTPS needs SHIPPER_PUBLIC_URL with a public HTTPS hostname")
	}
	domains, e := NormalizeDomains([]store.Domain{{Hostname: u.Hostname(), HTTPS: true}})
	if e != nil || len(domains) == 0 {
		return errors.New("Dashboard HTTPS needs a public hostname")
	}
	target, e := url.Parse(c.DashboardUpstream)
	if e != nil || target.Scheme != "http" || target.Hostname() == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" || (target.Path != "" && target.Path != "/") {
		return errors.New("SHIPPER_DASHBOARD_UPSTREAM must be an HTTP origin reachable from the proxy")
	}
	if c.ACMEEmail == "" || c.HTTPPort != 80 || c.HTTPSPort != 443 || c.BindIP == "127.0.0.1" || c.BindIP == "::1" {
		return errors.New("Dashboard HTTPS needs an ACME email and publicly bound ports 80 and 443")
	}
	return nil
}

func (r *DockerRuntime) connectDashboard(ctx context.Context) error {
	if r.Config.DashboardContainer == "" {
		return nil
	}
	c, e := r.Client.ContainerInspect(ctx, r.Config.DashboardContainer, client.ContainerInspectOptions{})
	if e != nil {
		return errors.New("Automatic dashboard networking requires the packaged Shipper container")
	}
	if c.Container.Config.Labels["io.shipper.control-plane"] != "true" {
		return errors.New("Dashboard container ownership check failed")
	}
	if _, ok := c.Container.NetworkSettings.Networks[r.network()]; ok {
		return nil
	}
	_, e = r.Client.NetworkConnect(ctx, r.network(), client.NetworkConnectOptions{Container: c.Container.ID, EndpointConfig: &network.EndpointSettings{Aliases: []string{"dashboard-" + r.Instance[:12]}}})
	return e
}

func (r *DockerRuntime) dashboardRoute(config map[string]any) {
	if r.Config.DashboardUpstream == "" {
		return
	}
	u, e := url.Parse(r.Config.DashboardURL)
	if e != nil {
		return
	}
	httpConfig := config["http"].(map[string]any)
	for _, key := range []string{"routers", "services", "middlewares"} {
		if httpConfig[key] == nil {
			httpConfig[key] = map[string]any{}
		}
	}
	routers := httpConfig["routers"].(map[string]any)
	services := httpConfig["services"].(map[string]any)
	middlewares := httpConfig["middlewares"].(map[string]any)
	rule := "Host(`" + u.Hostname() + "`)"
	services["shipper-dashboard"] = map[string]any{"loadBalancer": map[string]any{"servers": []any{map[string]string{"url": strings.TrimSuffix(r.Config.DashboardUpstream, "/")}}}}
	middlewares["dashboard-identity"] = map[string]any{"headers": map[string]any{"customResponseHeaders": map[string]string{"X-Shipper-Instance": r.Instance}}}
	routers["shipper-dashboard-http"] = map[string]any{"rule": rule, "priority": 100000, "service": "shipper-dashboard", "entryPoints": []string{"web"}, "middlewares": []string{"https-redirect"}}
	routers["shipper-dashboard"] = map[string]any{"rule": rule, "priority": 100000, "service": "shipper-dashboard", "entryPoints": []string{"websecure"}, "middlewares": []string{"dashboard-identity"}, "tls": map[string]any{"certResolver": "letsencrypt"}}
}
