package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/store"
)

func (s *Server) proxyInfo(w http.ResponseWriter, r *http.Request) { s.json(w, 200, s.cfg.Proxy) }
func (s *Server) application(w http.ResponseWriter, r *http.Request) {
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	ds, e := s.db.Deployments(r.Context(), a.ID)
	if e != nil {
		s.internal(w, e)
		return
	}
	for i := range ds {
		ds[i] = store.PublicDeployment(ds[i])
	}
	var active *store.Deployment
	if a.ActiveDeploymentID != "" {
		if d, e := s.db.Deployment(r.Context(), a.ActiveDeploymentID); e == nil {
			public := store.PublicDeployment(d)
			active = &public
		}
	}
	s.json(w, 200, map[string]any{"application": store.PublicApplication(a), "deployments": ds, "activeDeployment": active})
}
func (s *Server) resourceError(w http.ResponseWriter, e error) {
	if errors.Is(e, sql.ErrNoRows) {
		s.problem(w, 404, "not_found", "The application was not found")
	} else {
		s.internal(w, e)
	}
}
func (s *Server) deploymentLogs(w http.ResponseWriter, r *http.Request) {
	d, e := s.db.Deployment(r.Context(), chi.URLParam(r, "deployment"))
	if e != nil || d.ApplicationID != chi.URLParam(r, "id") {
		s.problem(w, 404, "not_found", "The deployment was not found")
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if after < 0 {
		after = 0
	}
	lines, e := s.db.Logs(r.Context(), d.ID, after)
	if e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, map[string]any{"items": lines, "state": d.State})
}

type applicationInput struct {
	Name   string             `json:"name"`
	Config store.StaticConfig `json:"config"`
}

func validName(name string) bool {
	return name != "" && len(name) <= 80 && !strings.ContainsFunc(name, unicode.IsControl)
}
func (s *Server) mutationAvailable(w http.ResponseWriter) bool {
	if s.cfg.Worker == nil {
		s.problem(w, 503, "deployments_unavailable", "Deployment workers are unavailable")
		return false
	}
	return true
}
func (s *Server) validateConfig(w http.ResponseWriter, c *store.StaticConfig, id string) bool {
	if len(c.Domains) > 0 {
		domains, e := deploy.NormalizeDomains(c.Domains)
		if e != nil {
			s.problem(w, 422, "invalid_domains", e.Error())
			return false
		}
		c.Domains = domains
		primary := store.PrimaryDomain(domains)
		c.Domain = primary.Hostname
		c.HTTPS = primary.HTTPS
	}

	if c.Domain == "" && c.Method != "compose" && c.StartupPolicy != "process" {
		c.Domain = id + ".localhost"
		c.HTTPS = false
	}
	if e := deploy.Validate(c); e != nil {
		s.problem(w, 422, "invalid_configuration", e.Error())
		return false
	}
	for _, d := range c.Domains {
		if d.HTTPS && s.cfg.Proxy.ACMEEmail == "" {
			s.problem(w, 422, "https_unconfigured", "Set SHIPPER_ACME_EMAIL before enabling HTTPS")
			return false
		}
	}
	if c.HTTPS && s.cfg.Proxy.ACMEEmail == "" {
		s.problem(w, 422, "https_unconfigured", "Set SHIPPER_ACME_EMAIL before enabling HTTPS")
		return false
	}
	return true
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func (s *Server) enqueue(w http.ResponseWriter, r *http.Request, a store.Application, operation string, create bool, noCache ...bool) {
	s.enqueueJob(w, r, a, store.Deployment{Operation: operation, NoCache: len(noCache) > 0 && noCache[0]}, create)
}
func (s *Server) enqueueJob(w http.ResponseWriter, r *http.Request, a store.Application, job store.Deployment, create bool) {
	job.ID = auth.Token()[:24]
	job.Actor = r.Context().Value(sessionKey).(store.Session).User.Email

	key := r.Header.Get("Idempotency-Key")
	if key != "" && !keyPattern.MatchString(key) {
		s.problem(w, 422, "invalid_idempotency_key", "Use an idempotency key of at most 128 letters, numbers, or ._:-")
		return
	}
	d, e := s.db.Enqueue(r.Context(), a, job, key, create)
	if e != nil {
		switch {
		case errors.Is(e, store.ErrBusy):
			s.problem(w, 409, "operation_in_progress", e.Error())
		case errors.Is(e, store.ErrDomainInUse):
			s.problem(w, 409, "domain_in_use", e.Error())
		case errors.Is(e, store.ErrIdempotencyConflict):
			s.problem(w, 409, "idempotency_conflict", e.Error())
		default:
			s.internal(w, e)
		}
		return
	}
	s.cfg.Worker.Trigger()
	s.hub.Publish()
	w.Header().Set("Location", "/api/applications/"+d.ApplicationID)
	s.json(w, 202, store.PublicDeployment(d))
}
func (s *Server) createApplication(w http.ResponseWriter, r *http.Request) {
	if !s.mutationAvailable(w) {
		return
	}
	var input applicationInput
	if !s.decode(w, r, &input) {
		return
	}
	a := store.Application{ID: auth.Token()[:24], Name: strings.TrimSpace(input.Name), Config: input.Config}
	if e := store.MergeVariables(&a.Config, store.Config{}); e != nil {
		s.problem(w, 422, "invalid_variables", e.Error())
		return
	}
	if a.Name == "" && a.Config.Method == "compose" && a.Config.ComposeSource == "paste" {
		a.Name = "Compose application"
	}
	if a.Name == "" {
		u, e := url.Parse(a.Config.Repository)
		if e == nil {
			a.Name = strings.TrimSuffix(path.Base(u.Path), ".git")
		}
	}
	if !validName(a.Name) {
		s.problem(w, 422, "invalid_name", "Enter an application name of up to 80 characters")
		return
	}
	if !s.validateConfig(w, &a.Config, a.ID) {
		return
	}
	if !s.validateComposeDomains(w, r, a.Config, a.Config.Domains, "") {
		return
	}
	s.enqueue(w, r, a, "deploy", true)
}
func (s *Server) redeploy(w http.ResponseWriter, r *http.Request) {
	if !s.mutationAvailable(w) {
		return
	}
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if (a.SourceKind != "git" && a.SourceKind != "compose") || a.Method == "" {
		s.problem(w, 422, "unsupported_method", "Only applications deployed by Shipper can be redeployed")
		return
	}
	var input struct {
		Config   *store.StaticConfig `json:"config"`
		Name     *string             `json:"name"`
		NoCache  bool                `json:"noCache"`
		Revision string              `json:"revision"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	current := a
	if input.Revision != "" && input.Revision != s.db.ConfigRevision(a) {
		s.problem(w, 409, "configuration_changed", "Configuration changed; review the changes again")
		return
	}
	if input.Config != nil {
		next := *input.Config
		if (a.Config.Method == "compose") != (next.Method == "compose") {
			s.problem(w, 422, "incompatible_method", "Create a new application to switch between Compose and a single-container deployment")
			return
		}
		var release store.Deployment
		if a.ActiveDeploymentID != "" {
			release, e = s.db.Deployment(r.Context(), a.ActiveDeploymentID)
			if e != nil {
				s.internal(w, e)
				return
			}
		}
		if e := store.MergeReleaseVariables(&next, a.Config, release); e != nil {
			s.problem(w, 422, "invalid_variables", e.Error())
			return
		}
		next.Domain = a.Config.Domain
		next.HTTPS = a.Config.HTTPS
		next.Domains = nil
		a.Config = next
	}
	if input.Name != nil {
		a.Name = strings.TrimSpace(*input.Name)
	}
	if !validName(a.Name) {
		s.problem(w, 422, "invalid_name", "Enter an application name of up to 80 characters")
		return
	}
	if !s.validateConfig(w, &a.Config, a.ID) {
		return
	}
	if a.Domains != nil && len(a.Domains) == 0 {
		a.Config.Domain = ""
		a.Config.HTTPS = false
	}
	if a.Config.StartupPolicy == "process" && len(a.Domains) > 0 {
		s.problem(w, 422, "worker_domains", "Remove public domains before using process readiness")
		return
	}
	if !s.validateComposeDomains(w, r, a.Config, a.Domains, a.ActiveDeploymentID) {
		return
	}
	if strings.HasSuffix(r.URL.Path, "/changes") {
		old := current.Config
		if current.ActiveDeploymentID != "" {
			if release, e := s.db.Deployment(r.Context(), current.ActiveDeploymentID); e == nil {
				old = release.Config
			}
		}
		plan := deploy.PlanChanges(old, a.Config)
		plan.Revision = s.db.ConfigRevision(current)
		s.json(w, 200, plan)
		return
	}
	operation := "deploy"
	if input.Config != nil && input.Revision != "" {
		operation = "apply"
	}
	s.enqueue(w, r, a, operation, false, input.NoCache)
}
func (s *Server) deleteApplication(w http.ResponseWriter, r *http.Request) {
	if !s.mutationAvailable(w) {
		return
	}
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	var input struct {
		ConfirmName string `json:"confirmName"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if input.ConfirmName != a.Name {
		s.problem(w, 422, "confirmation_required", "Enter the application name to confirm deletion")
		return
	}
	if (a.SourceKind != "git" && a.SourceKind != "compose") || a.Method == "" {
		s.problem(w, 422, "unsupported_method", "This application was discovered from Docker and cannot be deleted here")
		return
	}
	s.enqueue(w, r, a, "delete", false)
}

// Probe only public DNS targets, pin the connection to those answers, disallow
// redirects, and verify TLS normally. The response must identify this release.
func (s *Server) checkDomain(w http.ResponseWriter, r *http.Request) {
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if a.ActiveDeploymentID == "" {
		s.problem(w, 409, "no_active_deployment", "Deploy the application before checking its domain")
		return
	}
	d, e := s.db.Deployment(r.Context(), a.ActiveDeploymentID)
	if e != nil {
		s.internal(w, e)
		return
	}
	var input struct {
		DomainID string `json:"domainId"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	domain := store.PrimaryDomain(a.Domains)
	if input.DomainID != "" {
		found := false
		for _, item := range a.Domains {
			if item.ID == input.DomainID {
				domain = item
				found = true
				break
			}
		}
		if !found {
			s.problem(w, 404, "not_found", "The domain was not found")
			return
		}
	}
	if domain.Hostname == "" {
		domain = store.Domain{ID: "primary", Hostname: d.Config.Domain, HTTPS: d.Config.HTTPS, Primary: true}
	}
	d.Config.Domain = domain.Hostname
	d.Config.HTTPS = domain.HTTPS
	respond := func(state, message string) {
		s.db.SetDomainCheck(r.Context(), a.ID, domain, state, message)
		s.hub.Publish()
		s.json(w, 200, map[string]string{"state": state, "message": message, "domain": domain.Hostname})
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	scheme := "http"
	port := "80"
	if d.Config.HTTPS {
		scheme = "https"
		port = "443"
	}
	local := strings.HasSuffix(d.Config.Domain, ".localhost")
	if local {
		if s.cfg.Worker == nil || s.cfg.Worker.Runtime == nil {
			s.problem(w, 503, "proxy_unavailable", "The application proxy is unavailable")
			return
		}
		verified, err := s.cfg.Worker.Runtime.ProbeLocal(ctx, d)
		if err != nil {
			respond("unreachable", "The domain is not reachable through this server")
			return
		}
		if !verified {
			respond("route_error", "The domain is not serving this deployment")
			return
		}
		respond("verified", "Verified")
		return
	}
	var addresses []netip.Addr
	if local {
		addresses = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
		port = strconv.Itoa(s.cfg.Proxy.HTTPPort)
	} else {
		addresses, e = net.DefaultResolver.LookupNetIP(ctx, "ip", d.Config.Domain)
		if e != nil || len(addresses) == 0 {
			respond("dns_pending", "Point the domain's A or AAAA record to this server")
			return
		}
		for _, ip := range addresses {
			if !deploy.PublicAddress(ip) {
				respond("dns_error", "The domain must resolve to a public server address")
				return
			}
		}
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 4 * time.Second}).DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 7 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequestWithContext(ctx, "GET", scheme+"://"+d.Config.Domain+"/", nil)
	res, e := client.Do(req)
	if e != nil {
		state := "unreachable"
		message := "The domain is not reachable through this server"
		if d.Config.HTTPS {
			state = "tls_pending"
			message = "HTTPS verification failed; check DNS and Traefik certificate logs"
		}
		respond(state, message)
		return
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if (res.StatusCode < 200 || res.StatusCode >= 400) || res.Header.Get("X-Shipper-Deployment") != d.ID {
		respond("route_error", fmt.Sprintf("The domain is not serving this deployment (HTTP %d)", res.StatusCode))
		return
	}
	respond("verified", "Verified")
}

func (s *Server) deployment(w http.ResponseWriter, r *http.Request) {
	d, e := s.db.Deployment(r.Context(), chi.URLParam(r, "deployment"))
	if e != nil || d.ApplicationID != chi.URLParam(r, "id") {
		s.problem(w, 404, "not_found", "The deployment was not found")
		return
	}
	s.json(w, 200, store.PublicDeployment(d))
}
