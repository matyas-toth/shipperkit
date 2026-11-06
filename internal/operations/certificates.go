package operations

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"time"

	"shipper/internal/netguard"
	"shipper/internal/store"
)

type certificateProbe struct {
	State   string `json:"state"`
	Message string `json:"message"`
	Issuer  string `json:"issuer,omitempty"`
	Expiry  int64  `json:"expiresAt,omitempty"`
}

// Pin a public DNS answer for the request, verify the ordinary certificate
// chain, and verify that the response belongs to the expected serving route.
func probeCertificate(ctx context.Context, host, path, header, identity string, maintenance bool) certificateProbe {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addresses, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if e != nil || len(addresses) == 0 {
		return certificateProbe{State: "dns_pending", Message: "Point the domain to this server"}
	}
	for _, ip := range addresses {
		if !netguard.Public(ip) {
			return certificateProbe{State: "dns_error", Message: "The domain resolves to a non-public address"}
		}
	}
	result := certificateProbe{State: "tls_pending", Message: "HTTPS is not trusted or reachable from Shipper"}
	// Limit fanout. Trying both families avoids treating a single unavailable
	// IPv6 answer as evidence that a working IPv4 route has no certificate.
	for _, ip := range addresses[:min(4, len(addresses))] {
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), "443"))
			},
			TLSHandshakeTimeout: 2 * time.Second,
		}
		client := &http.Client{Transport: transport, Timeout: 2500 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		target := url.URL{Scheme: "https", Host: host, Path: path}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			break
		}
		res, err := client.Do(req)
		if err == nil {
			res.Body.Close()
			cert := res.TLS.PeerCertificates[0]
			result.Issuer = cert.Issuer.CommonName
			result.Expiry = cert.NotAfter.Unix()
			if res.Header.Get(header) == identity && (res.StatusCode < 400 || maintenance && res.StatusCode == 503) {
				result.State, result.Message = "verified", "Trusted HTTPS"
				transport.CloseIdleConnections()
				break
			}
			result.State, result.Message = "route_error", "HTTPS serves a different route or deployment"
		}
		transport.CloseIdleConnections()
	}
	return result
}

type certificateSchedule struct {
	next, firstFailure time.Time
	delay              time.Duration
}

func (s *Service) checkCertificate(ctx context.Context, apps []store.Application) {
	dashboardURL := s.dashboardURL(ctx)
	if dashboardURL == "" {
		return
	}
	if s.certificates == nil {
		s.certificates = make(map[string]certificateSchedule)
	}
	type target struct {
		id, app, host, path, header, identity string
		domain                                store.Domain
	}
	targets := []target{}
	if u, e := url.Parse(dashboardURL); e == nil && u.Scheme == "https" && u.Hostname() != "" {
		if contact, e := s.DB.ACMEContact(ctx); e == nil && contact != "" {
			targets = append(targets, target{id: "certificate/dashboard", host: u.Hostname(), path: "/api/health", header: "X-Shipper-Instance", identity: s.Instance})
		}
	}
	for _, a := range apps {
		if a.ActiveDeploymentID == "" || a.DesiredState == "stopped" {
			continue
		}
		for _, d := range a.Domains {
			if !d.HTTPS || d.Hostname == "" {
				continue
			}
			targets = append(targets, target{"certificate/" + a.ID + "/" + d.ID, a.ID, d.Hostname, d.PathPrefix + "/", "X-Shipper-Deployment", a.ActiveDeploymentID, d})
		}
	}
	if len(s.certificates) > 2*len(targets)+32 {
		live := make(map[string]bool, len(targets))
		for _, t := range targets {
			live[t.id+"/"+t.host] = true
		}
		for key := range s.certificates {
			if !live[key] {
				delete(s.certificates, key)
			}
		}
	}
	now := time.Now()
	for i := range targets {
		index := (s.certificateOffset + i) % len(targets)
		t := targets[index]
		key := t.id + "/" + t.host
		item := s.certificates[key]
		if now.Before(item.next) {
			continue
		}
		s.certificateOffset = (index + 1) % len(targets)
		result := probeCertificate(ctx, t.host, t.path, t.header, t.identity, t.domain.Maintenance)
		if ctx.Err() != nil {
			return
		}
		state := "healthy"
		if result.State == "verified" {
			item = certificateSchedule{next: now.Add(5 * time.Minute)}
			if result.Expiry < now.Add(14*24*time.Hour).Unix() {
				state, result.Message = "warning", "Certificate expires within 14 days"
			}
		} else {
			if item.firstFailure.IsZero() {
				item.firstFailure = now
			}
			item.delay = min(max(item.delay*2, 30*time.Second), 10*time.Minute)
			item.next = now.Add(item.delay)
			state = "pending"
			if now.Sub(item.firstFailure) >= 5*time.Minute {
				state = "warning"
			}
		}
		s.certificates[key] = item
		details, _ := json.Marshal(result)
		s.record(ctx, store.Observation{ID: t.id, ApplicationID: t.app, Kind: "certificate", State: state, Reference: t.host, Summary: result.Message, Details: details})
		if t.app != "" {
			d := t.domain
			d.CertificateIssuer, d.CertificateExpiresAt = result.Issuer, result.Expiry
			changed := d.VerificationState != result.State || t.domain.CertificateExpiresAt != d.CertificateExpiresAt
			if e := s.DB.SetDomainCheck(ctx, t.app, d, result.State, result.Message, t.identity); e == nil && changed && s.Publish != nil {
				s.Publish()
			}
		}
		return
	}
}
