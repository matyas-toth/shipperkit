package httpapi

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"shipper/internal/deploy"
	"shipper/internal/store"
	"time"
)

// Only the configured canonical dashboard is probed; no arbitrary URL is accepted.
func (s *Server) checkProxy(w http.ResponseWriter, r *http.Request) {
	config := s.effectiveProxy(r.Context())
	u, e := url.Parse(config.DashboardURL)
	if e != nil || u.Hostname() == "" || config.DashboardUpstream == "" {
		s.problem(w, 409, "dashboard_unconfigured", "Configure dashboard HTTPS before checking it")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	result := map[string]any{"state": "dns_pending", "hostname": u.Hostname(), "checkedAt": time.Now().Unix()}
	respond := func(state, message string) {
		result["state"] = state
		result["message"] = message
		s.json(w, 200, result)
	}
	ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", u.Hostname())
	if e != nil || len(ips) == 0 {
		respond("dns_pending", "Point the dashboard domain to this server")
		return
	}
	addresses := []string{}
	for _, ip := range ips {
		if !deploy.PublicAddress(ip) {
			respond("dns_error", "The dashboard must resolve to a public server address")
			return
		}
		addresses = append(addresses, ip.String())
	}
	result["addresses"] = addresses
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 4 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), "443"))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 7 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequestWithContext(ctx, "GET", config.DashboardURL+"/api/health", nil)
	res, e := client.Do(req)
	if e != nil {
		respond("tls_pending", "HTTPS verification failed; check DNS, ports 80/443 and the proxy logs")
		return
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.TLS != nil && len(res.TLS.PeerCertificates) > 0 {
		cert := res.TLS.PeerCertificates[0]
		result["certificateExpiresAt"] = cert.NotAfter.Unix()
		result["certificateIssuer"] = cert.Issuer.CommonName
	}
	if res.StatusCode != 200 || res.Header.Get("X-Shipper-Instance") != s.cfg.InstanceID {
		respond("route_error", "The hostname is not serving this Shipper instance")
		return
	}
	respond("verified", "Verified")
}

func (s *Server) dashboardDomain(host string) bool {
	return host != "" && s.dashboardName() == host
}

func domainProbePath(d store.Domain) string {
	if d.PathPrefix != "" {
		return d.PathPrefix + "/"
	}
	return "/"
}
