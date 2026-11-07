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

func (s *Server) effectiveProxy(ctx context.Context) deploy.ProxyConfig {
	config := s.cfg.Proxy
	if s.cfg.EffectiveProxy != nil {
		if next, e := s.cfg.EffectiveProxy(ctx); e == nil {
			config = next
		}
	}
	return config
}

func (s *Server) proxyInfo(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, s.effectiveProxy(r.Context()))
}

func (s *Server) saveProxy(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Domain string `json:"domain"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	domain, e := deploy.NormalizeDashboardDomain(input.Domain)
	if e != nil {
		s.problem(w, 422, "invalid_domain", e.Error())
		return
	}
	previous := s.dashboardName()
	if domain != "" {
		if !s.hasACME() {
			s.problem(w, 422, "https_unconfigured", "Automatic HTTPS is unavailable on this installation")
			return
		}
		apps, e := s.db.Applications(r.Context())
		if e != nil {
			s.internal(w, e)
			return
		}
		for _, app := range apps {
			inUse := app.Config.Domain == domain
			for _, item := range app.Domains {
				inUse = inUse || item.Hostname == domain
			}
			if inUse {
				s.problem(w, 409, "domain_in_use", "This domain is already assigned to an application")
				return
			}
		}
	}
	if e = s.db.SaveSetting(r.Context(), "dashboard_domain", domain); e != nil {
		s.internal(w, e)
		return
	}
	s.setDashboard(domain)
	if s.cfg.Worker == nil || s.cfg.Operations == nil {
		s.json(w, 200, map[string]any{"domain": domain, "state": "saved"})
		return
	}
	revision, e := s.operator().RepairRevision(r.Context())
	if e != nil {
		s.internal(w, e)
		return
	}
	id := auth.Token()[:24]
	if e = s.db.EnqueueMaintenance(r.Context(), store.MaintenanceRun{Actor: r.Context().Value(sessionKey).(store.Session).User.Email, ID: id, Kind: "proxy", Revision: revision}); e != nil {
		_ = s.db.SaveSetting(r.Context(), "dashboard_domain", previous)
		s.setDashboard(previous)
		s.problem(w, 409, "operation_in_progress", "Maintenance is already queued")
		return
	}
	s.cfg.Worker.Trigger()
	s.json(w, 202, map[string]any{"domain": domain, "id": id, "state": "queued"})
}

func (s *Server) hasACME() bool {
	if s.cfg.Proxy.ACMEEmail != "" {
		return true
	}
	if s.cfg.Proxy.AutoHTTPS {
		contact, e := s.db.ACMEContact(context.Background())
		return e == nil && contact != ""
	}
	return false
}
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
		ds[i] = publicDeployment(r, ds[i])
	}
	var active *store.Deployment
	var database *deploy.DatabaseEndpoint
	if a.ActiveDeploymentID != "" {
		if d, e := s.db.Deployment(r.Context(), a.ActiveDeploymentID); e == nil {
			database = deploy.DatabaseEndpointFor(d)
			public := publicDeployment(r, d)
			active = &public
		}
	}
	s.json(w, 200, map[string]any{"application": publicApplication(r, a), "deployments": ds, "activeDeployment": active, "database": database})
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
	ID       string             `json:"id,omitempty"`
	Revision string             `json:"revision,omitempty"`
	Commit   string             `json:"commit,omitempty"`
	Name     string             `json:"name"`
	Config   store.StaticConfig `json:"config"`
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

	if c.Domain == "" && c.Method != "compose" && c.StartupPolicy != "process" && !s.cfg.Proxy.AutoHTTPS {
		c.Domain = id + ".localhost"
		c.HTTPS = false
	}
	if e := deploy.Validate(c, s.cfg.Proxy.AutoHTTPS); e != nil {
		s.problem(w, 422, "invalid_configuration", e.Error())
		return false
	}
	for _, d := range c.Domains {
		if d.HTTPS && !s.hasACME() {
			s.problem(w, 422, "https_unconfigured", "Automatic HTTPS is unavailable on this installation")
			return false
		}
	}
	if s.dashboardDomain(c.Domain) {
		s.problem(w, 422, "reserved_domain", "This hostname belongs to the Shipper dashboard")
		return false
	}
	for _, domain := range c.Domains {
		if s.dashboardDomain(domain.Hostname) {
			s.problem(w, 422, "reserved_domain", "This hostname belongs to the Shipper dashboard")
			return false
		}
	}
	if c.HTTPS && !s.hasACME() {
		s.problem(w, 422, "https_unconfigured", "Automatic HTTPS is unavailable on this installation")
		return false
	}
	return true
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
var reviewedCommit = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

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
	s.json(w, 202, publicDeployment(r, d))
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
	if input.ID != "" {
		if !regexp.MustCompile(`^[a-f0-9]{24}$`).MatchString(input.ID) || input.Revision == "" {
			s.problem(w, 422, "invalid_plan", "Use the identity and revision returned by plan")
			return
		}
		a.ID = input.ID
	}
	if e := store.MergeVariables(&a.Config, store.Config{}); e != nil {
		s.problem(w, 422, "invalid_variables", e.Error())
		return
	}
	if a.Name == "" && a.Config.LocalSource != "" {
		a.Name = "Local application"
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
	if e := s.validateSource(r.Context(), &a.Config); e != nil {
		s.problem(w, 422, "invalid_source", e.Error())
		return
	}
	if !s.validateConfig(w, &a.Config, a.ID) {
		return
	}
	planning := strings.HasSuffix(r.URL.Path, "/plan")
	if input.Commit != "" && (!reviewedCommit.MatchString(input.Commit) || a.Config.Repository == "" || !planning && input.ID == "") {
		s.problem(w, 422, "invalid_commit", "Use the full commit from the Git source inspection")
		return
	}
	if !s.validateComposeDomains(w, r, a.Config, a.Config.Domains, "") {
		return
	}
	if a.Config.LocalSource != "" {
		source, e := s.db.SourceSnapshot(r.Context(), a.Config.LocalSource)
		if e != nil || source.ApplicationID != "" && source.ApplicationID != a.ID {
			if e == nil && input.ID == "" && !strings.HasSuffix(r.URL.Path, "/plan") {
				old, found, err := s.db.ReplayRequest(r.Context(), r.Header.Get("Idempotency-Key"), a, store.Deployment{Operation: "deploy"}, true)
				if err == nil && found && old.ApplicationID == source.ApplicationID {
					s.json(w, 202, publicDeployment(r, old))
					return
				}
			}
			s.problem(w, 403, "source_denied", "Upload a source for this application")
			return
		}
	}
	revision := s.db.ChangeRevision(store.Application{}, a, input.Commit)
	if planning {
		s.json(w, 200, map[string]any{"applicationId": a.ID, "revision": revision, "operation": "deploy", "name": a.Name, "commit": input.Commit, "config": store.PublicConfig(a.Config), "changes": []string{"Create application", "Prepare the source and start services", "Preserve declared persistent storage"}})
		return
	}
	if input.ID != "" && input.Revision != revision {
		s.problem(w, 409, "configuration_changed", "The planned inputs changed; review again")
		return
	}
	s.enqueueJob(w, r, a, store.Deployment{Operation: "deploy", Commit: input.Commit}, true)
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
	if !a.Managed() {
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
	if e := s.validateSource(r.Context(), &a.Config); e != nil {
		s.problem(w, 422, "invalid_source", e.Error())
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
	if a.Config.LocalSource != "" {
		source, e := s.db.SourceSnapshot(r.Context(), a.Config.LocalSource)
		if e != nil || source.ApplicationID != "" && source.ApplicationID != a.ID {
			s.problem(w, 403, "source_denied", "Upload a source for this application")
			return
		}
	}
	if strings.HasSuffix(r.URL.Path, "/changes") {
		old := current.Config
		if current.ActiveDeploymentID != "" {
			if release, e := s.db.Deployment(r.Context(), current.ActiveDeploymentID); e == nil {
				old = release.Config
			}
		}
		old.AutoDeploy = current.Config.AutoDeploy
		plan := deploy.PlanChanges(old, a.Config)
		plan.Revision = s.db.ChangeRevision(current, a)
		s.json(w, 200, plan)
		return
	}
	if input.Revision != "" && input.Revision != s.db.ChangeRevision(current, a) {
		old, found, err := s.db.ReplayRequest(r.Context(), r.Header.Get("Idempotency-Key"), a, store.Deployment{NoCache: input.NoCache})
		if err == nil && found {
			s.json(w, 202, publicDeployment(r, old))
			return
		}
		s.problem(w, 409, "configuration_changed", "Configuration or planned inputs changed; review again")
		return
	}
	operation := "deploy"
	if input.Config != nil && input.Revision != "" {
		operation = "apply"
	}
	if operation == "apply" && deploy.PlanChanges(current.Config, a.Config).Operation == "metadata" {
		operation = "configure"
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
	if !a.Managed() {
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
	d.Config.Domains = []store.Domain{domain}
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
	req, _ := http.NewRequestWithContext(ctx, "GET", scheme+"://"+d.Config.Domain+domainProbePath(domain), nil)
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
	if res.TLS != nil && len(res.TLS.PeerCertificates) > 0 {
		cert := res.TLS.PeerCertificates[0]
		domain.CertificateExpiresAt = cert.NotAfter.Unix()
		domain.CertificateIssuer = cert.Issuer.CommonName
	}
	if (res.StatusCode < 200 || res.StatusCode >= 400) && !(domain.Maintenance && res.StatusCode == 503) || res.Header.Get("X-Shipper-Deployment") != d.ID {
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
	s.json(w, 200, publicDeployment(r, d))
}
