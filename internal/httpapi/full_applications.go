package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/store"
)

func (s *Server) branches(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Repository string `json:"repository"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	result, e := deploy.PublicBranches(r.Context(), input.Repository)
	if e != nil {
		s.problem(w, 422, "repository_unavailable", e.Error())
		return
	}
	s.json(w, 200, result)
}
func (s *Server) domainOptions(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]bool{"canIncludeWWW": deploy.CanIncludeWWW(r.URL.Query().Get("hostname"))})
}
func (s *Server) parseVariables(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Content string `json:"content"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	items, e := deploy.ParseDotenv(input.Content)
	if e != nil {
		s.problem(w, 422, "invalid_environment", e.Error())
		return
	}
	s.json(w, 200, map[string]any{"items": items})
}
func (s *Server) revealVariable(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name    string `json:"name"`
		Service string `json:"service"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	variables := a.Config.Variables
	if input.Service != "" {
		variables = nil
		for _, svc := range a.Config.Services {
			if svc.Name == input.Service {
				variables = svc.Variables
			}
		}
	}
	if input.Service != "" && a.ActiveDeploymentID != "" {
		if release, e := s.db.Deployment(r.Context(), a.ActiveDeploymentID); e == nil {
			for _, v := range store.ComposeVariables(release, input.Service) {
				found := false
				for _, override := range variables {
					if override.Name == v.Name {
						found = true
					}
				}
				if !found {
					variables = append(variables, v)
				}
			}
		}
	}
	for _, v := range variables {
		if v.Name == input.Name {
			s.json(w, 200, map[string]string{"value": v.Value})
			return
		}
	}
	s.problem(w, 404, "not_found", "The variable was not found")
}
func (s *Server) renameApplication(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if !validName(input.Name) {
		s.problem(w, 422, "invalid_name", "Enter an application name of up to 80 characters")
		return
	}
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if e = s.db.Rename(r.Context(), a.ID, input.Name); e != nil {
		s.internal(w, e)
		return
	}
	s.hub.Publish()
	s.json(w, 200, map[string]bool{"saved": true})
}
func (s *Server) updateDomains(w http.ResponseWriter, r *http.Request) {
	if !s.mutationAvailable(w) {
		return
	}
	var input struct {
		Domains    []store.Domain `json:"domains"`
		IncludeWWW []string       `json:"includeWWW"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if (a.SourceKind != "git" && a.SourceKind != "compose") || a.Method == "" {
		s.problem(w, 422, "unsupported_application", "Domains can be changed for applications deployed by Shipper")
		return
	}
	for _, id := range input.IncludeWWW {
		found := false
		for _, domain := range input.Domains {
			if domain.ID == id {
				found = true
				if !deploy.CanIncludeWWW(domain.Hostname) {
					s.problem(w, 422, "invalid_domain", "Automatic www is available for root domains")
					return
				}
				input.Domains = append(input.Domains, store.Domain{ID: auth.Token()[:16], Hostname: "www." + strings.ToLower(strings.TrimSpace(domain.Hostname)), HTTPS: domain.HTTPS, Service: domain.Service, Redirect: true})
				break
			}
		}
		if !found {
			s.problem(w, 422, "invalid_domain", "The root domain was not found")
			return
		}
	}
	domains, e := deploy.NormalizeDomains(input.Domains)
	if e != nil {
		s.problem(w, 422, "invalid_domains", e.Error())
		return
	}
	for _, d := range domains {
		if d.HTTPS && s.cfg.Proxy.ACMEEmail == "" {
			s.problem(w, 422, "https_unconfigured", "Set SHIPPER_ACME_EMAIL before enabling HTTPS")
			return
		}
	}
	if !s.validateComposeDomains(w, r, a.Config, domains, a.ActiveDeploymentID) {
		return
	}
	if a.Config.StartupPolicy == "process" && len(domains) > 0 {
		s.problem(w, 422, "worker_domains", "Process-only workers do not have public domains")
		return
	}
	a.Config.Domains = domains
	s.enqueue(w, r, a, "domains", false)
}
func (s *Server) lifecycle(w http.ResponseWriter, r *http.Request) {
	if !s.mutationAvailable(w) {
		return
	}
	var input struct {
		Action  string `json:"action"`
		Service string `json:"service"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if input.Action != "start" && input.Action != "stop" && input.Action != "restart" {
		s.problem(w, 422, "invalid_action", "Choose Start, Stop, or Restart")
		return
	}
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if a.ActiveDeploymentID == "" || (a.SourceKind != "git" && a.SourceKind != "compose") {
		s.problem(w, 409, "no_active_deployment", "Deploy the application before using lifecycle controls")
		return
	}
	if input.Service != "" {
		if a.DesiredState == "stopped" && input.Action != "stop" {
			s.problem(w, 409, "application_stopped", "Start the application before starting an individual service")
			return
		}
		if a.Method != "compose" {
			s.problem(w, 422, "invalid_service", "Service actions require a Compose application")
			return
		}
		release, e := s.db.Deployment(r.Context(), a.ActiveDeploymentID)
		if e != nil {
			s.internal(w, e)
			return
		}
		found := false
		for _, svc := range release.Services {
			if svc.Name == input.Service {
				found = true
			}
		}
		if !found {
			s.problem(w, 422, "invalid_service", "Choose a service from the active release")
			return
		}
		s.enqueueJob(w, r, a, store.Deployment{Operation: "service_" + input.Action, Service: input.Service}, false)
		return
	}
	s.enqueue(w, r, a, input.Action, false)
}
func (s *Server) runtimeLogs(w http.ResponseWriter, r *http.Request) {
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if s.cfg.Worker == nil {
		s.problem(w, 503, "logs_unavailable", "Runtime logs are unavailable")
		return
	}
	source, ok := s.cfg.Worker.Runtime.(deploy.RuntimeLogSource)
	if !ok {
		s.problem(w, 503, "logs_unavailable", "Runtime logs are unavailable")
		return
	}
	containerID := r.URL.Query().Get("container")
	configuration := a.Config
	if a.ActiveDeploymentID != "" {
		release, err := s.db.Deployment(r.Context(), a.ActiveDeploymentID)
		if err != nil {
			s.internal(w, err)
			return
		}
		configuration.Variables = append(append([]store.Variable(nil), configuration.Variables...), release.Config.Variables...)
		configuration.Services = append(append([]store.ServiceConfig(nil), configuration.Services...), release.Config.Services...)
		for _, value := range deploy.ComposeMasks(release) {
			configuration.Variables = append(configuration.Variables, store.Variable{Name: "compose", Value: value, Secret: true})
		}
		if containerID == "" {
			containerID = release.ContainerID
		}
	}
	if containerID == "" {
		s.problem(w, 409, "no_running_container", "There is no application container to inspect")
		return
	}
	select {
	case s.logSlots <- struct{}{}:
		defer func() { <-s.logSlots }()
	default:
		s.problem(w, 429, "log_stream_limit", "Too many active runtime log streams")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.problem(w, 500, "stream_unavailable", "Streaming is unavailable")
		return
	}
	tail, _ := strconv.Atoi(r.URL.Query().Get("tail"))
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	lines := make(chan string, 128)
	finished := make(chan error, 1)
	go func() {
		finished <- source.Logs(ctx, a.ID, containerID, tail, func(line string) error {
			line = deploy.Redact(configuration, line)
			if len(line) > 4096 {
				line = line[:4096] + " [truncated]"
			}
			select {
			case lines <- line:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			default:
				return errors.New("The log client is too slow; reconnect to continue")
			}
		})
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	send := func(event string, value any) bool {
		controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
		data, _ := json.Marshal(value)
		if _, e := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); e != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !send("reset", map[string]string{"containerId": containerID}) {
		return
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case line := <-lines:
			if !send("log", map[string]string{"line": line}) {
				return
			}
		case e := <-finished:
			for {
				select {
				case line := <-lines:
					if !send("log", map[string]string{"line": line}) {
						return
					}
				default:
					if e != nil && !errors.Is(e, context.Canceled) && !errors.Is(e, io.EOF) {
						send("failure", map[string]string{"message": deploy.Redact(configuration, e.Error())})
					} else {
						send("end", map[string]bool{"complete": true})
					}
					return
				}
			}
		case <-heartbeat.C:
			if _, e := s.session(r); e != nil {
				return
			}
			if !send("heartbeat", map[string]bool{"alive": true}) {
				return
			}
		}
	}
}
