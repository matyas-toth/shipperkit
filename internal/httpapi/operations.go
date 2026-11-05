package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/operations"
	"shipper/internal/store"
)

func (s *Server) operationsRoutes(r chi.Router) {
	r.Get("/capabilities", s.capabilities)
	r.Get("/operation-tokens", s.operationTokens)
	r.With(s.csrf, s.limit).Post("/operation-tokens", s.createOperationToken)
	r.With(s.csrf, s.limit).Post("/operation-tokens/{token}/revoke", s.revokeOperationToken)
	r.Get("/system/operations", s.systemOperations)
	r.Get("/applications/{id}/diagnostics", s.diagnostics)
	r.Get("/applications/{id}/observations", s.observations)
	r.Get("/applications/{id}/incidents", s.incidents)
	r.With(s.csrf, s.limit).Post("/plan", s.createApplication)
	r.With(s.csrf, s.limit).Post("/sources", s.uploadSource)
	r.Get("/notifications", s.notificationSettings)
	r.With(s.csrf, s.limit).Post("/notifications", s.saveNotifications)
	r.With(s.csrf, s.limit).Post("/notifications/test", s.testNotification)
	r.With(s.csrf, s.limit).Post("/system/recheck", s.recheckSystem)
	r.Get("/system/retention", s.retention)
	r.With(s.csrf, s.limit).Post("/system/retention", s.saveRetention)
	r.With(s.csrf, s.limit).Post("/system/repair", s.repairProxy)
}
func (s *Server) repairProxy(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Review   bool   `json:"review"`
		Revision string `json:"revision"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	revision, e := s.operator().RepairRevision(r.Context())
	if e != nil {
		s.internal(w, e)
		return
	}
	if input.Review {
		s.json(w, 200, map[string]any{"operation": "recreate", "revision": revision, "changes": []string{"Ensure the owned proxy and edge network", "Reapply current application routes", "Preserve certificates, volumes and application containers"}})
		return
	}
	if input.Revision != revision {
		s.problem(w, 409, "repair_changed", "System configuration changed; review again")
		return
	}
	if s.cfg.Operations == nil || s.cfg.Worker == nil {
		s.problem(w, 503, "unavailable", "Proxy maintenance is unavailable")
		return
	}
	id := auth.Token()[:24]
	if e = s.db.EnqueueMaintenance(r.Context(), store.MaintenanceRun{Actor: r.Context().Value(sessionKey).(store.Session).User.Email, ID: id, Kind: "proxy", Revision: revision}); e != nil {
		s.problem(w, 409, "operation_in_progress", "Maintenance is already queued")
		return
	}
	s.cfg.Worker.Trigger()
	s.json(w, 202, map[string]string{"id": id, "state": "queued"})
}
func (s *Server) operator() *operations.Service {
	if s.cfg.Operations != nil {
		return s.cfg.Operations
	}
	return &operations.Service{DB: s.db, Monitor: s.monitor, Instance: s.cfg.InstanceID}
}
func (s *Server) retention(w http.ResponseWriter, r *http.Request) {
	p, e := s.operator().Retention(r.Context())
	if e != nil {
		s.problem(w, 503, "retention_unavailable", "Cleanup cannot be reviewed while Docker or references are unavailable")
		return
	}
	runs, e := s.db.MaintenanceRuns(r.Context())
	if e != nil {
		s.internal(w, e)
		return
	}
	automatic, e := s.db.PrivateSetting(r.Context(), "automatic_retention", "false")
	if e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, map[string]any{"plan": p, "runs": runs, "automatic": automatic == "true"})
}
func (s *Server) saveRetention(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Revision  string `json:"revision"`
		Automatic *bool  `json:"automatic"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if input.Automatic != nil {
		if e := s.db.SavePrivateSetting(r.Context(), "automatic_retention", strconv.FormatBool(*input.Automatic)); e != nil {
			s.internal(w, e)
			return
		}
		s.json(w, 200, map[string]bool{"automatic": *input.Automatic})
		return
	}
	if s.cfg.Worker == nil || s.cfg.Operations == nil {
		s.problem(w, 503, "unavailable", "Maintenance is unavailable")
		return
	}
	p, e := s.operator().Retention(r.Context())
	if e != nil || p.Revision != input.Revision {
		s.problem(w, 409, "retention_changed", "Cleanup candidates changed; review again")
		return
	}
	id := auth.Token()[:24]
	if e = s.db.EnqueueMaintenance(r.Context(), store.MaintenanceRun{Actor: r.Context().Value(sessionKey).(store.Session).User.Email, ID: id, Revision: p.Revision}); e != nil {
		s.problem(w, 409, "operation_in_progress", "A maintenance operation is already queued")
		return
	}
	s.cfg.Worker.Trigger()
	s.json(w, 202, map[string]string{"id": id, "state": "queued"})
}
func (s *Server) observations(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, e := s.db.Application(r.Context(), id); e != nil {
		s.resourceError(w, e)
		return
	}
	items, e := s.db.Observations(r.Context(), id)
	if e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, map[string]any{"items": items})
}
func (s *Server) incidents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, e := s.db.Application(r.Context(), id); e != nil {
		s.resourceError(w, e)
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if r.URL.Query().Get("after") == "" {
		after = -1
	}
	items, e := s.db.OperationEvents(r.Context(), id, after)
	if e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, map[string]any{"items": items, "limit": 100})
}
func (s *Server) systemOperations(w http.ResponseWriter, r *http.Request) {
	facts, e := s.db.Observations(r.Context(), "")
	if e != nil {
		s.internal(w, e)
		return
	}
	status, e := s.db.NotificationStatus(r.Context())
	if e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, map[string]any{"observedAt": time.Now().Unix(), "items": facts, "notifications": status, "sampleIntervalSeconds": 15, "maxContainersPerCycle": 32, "bootReadiness": "not_verified", "docker": s.monitor.Health()})
}
func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	a, e := s.db.Application(r.Context(), id)
	if e != nil {
		s.resourceError(w, e)
		return
	}
	facts, e := s.db.Observations(r.Context(), id)
	if e != nil {
		s.internal(w, e)
		return
	}
	current := []store.Observation{}
	for _, fact := range facts {
		if fact.Kind == "service" {
			found := false
			for _, c := range a.Containers {
				found = found || c.ID == fact.Reference
			}
			if !found {
				continue
			}
		}
		current = append(current, fact)
	}
	facts = current
	checks := []map[string]any{}
	add := func(layer, state, summary string, at int64) {
		checks = append(checks, map[string]any{"layer": layer, "state": state, "summary": summary, "observedAt": at})
	}
	health := s.monitor.Health()
	daemon := "failed"
	if health.Connected {
		daemon = "healthy"
	}
	add("daemon", daemon, "Docker connection", health.CheckedAt)
	var serving *store.Deployment
	dependencies := []operations.Dependency{}
	changes := []string{}
	if a.ActiveDeploymentID != "" {
		if release, err := s.db.Deployment(r.Context(), a.ActiveDeploymentID); err == nil {
			dependencies = operations.Dependencies(release, facts)
			v := publicDeployment(r, release)
			serving = &v
			changes = deploy.PlanChanges(release.Config, a.Config).Changes
		}
	}
	ready := "unknown"
	if a.DesiredState == "stopped" {
		ready = "stopped"
	} else if health.Connected && len(a.Containers) > 0 {
		ready = "healthy"
		for _, c := range a.Containers {
			if c.State != "running" || c.Health == "unhealthy" {
				ready = "warning"
			}
		}
	}
	add("runtime", ready, "Observed container state", health.ObservedAt)
	for _, d := range a.Domains {
		state := d.VerificationState
		if state == "" {
			state = "unknown"
		}
		add("route", state, d.Hostname, d.CheckedAt)
		if d.HTTPS {
			state = "unknown"
			if d.CertificateExpiresAt > time.Now().Unix() {
				state = "healthy"
			} else if d.CertificateExpiresAt > 0 {
				state = "failed"
			}
			add("certificate", state, d.Hostname, d.CheckedAt)
		}
	}
	latest := a.LatestDeployment
	stage := ""
	logs := []store.LogLine{}
	runs := []store.TaskRun{}
	if latest != nil {
		logs, _ = s.db.TailLogs(r.Context(), latest.ID, 30)
		for _, l := range logs {
			if l.Step != "failed" {
				stage = l.Step
			}
		}
		all, _ := s.db.TaskRuns(r.Context(), id)
		for _, run := range all {
			if run.DeploymentID == latest.ID {
				run.Command = store.Command{}
				runs = append(runs, run)
			}
		}
	}
	details := r.URL.Query().Get("include") == "logs"
	if latest != nil && (latest.Operation == "deploy" || latest.Operation == "apply" || latest.Operation == "rollback") {
		build := "unknown"
		if latest.ImageID != "" {
			build = "healthy"
		} else if latest.State == "failed" && (stage == "cloning" || stage == "building" || stage == "packaging" || stage == "pulling") {
			build = "failed"
		}
		add("build", build, "Latest release artifact", latest.StartedAt)
		readiness := "unknown"
		if latest.State == "succeeded" {
			readiness = "healthy"
		} else if latest.State == "failed" && (stage == "checking" || stage == "starting") {
			readiness = "failed"
		}
		add("readiness", readiness, "Latest deployment readiness", latest.FinishedAt)
	}
	for _, run := range runs {
		add("task", run.State, run.Stage, run.FinishedAt)
	}
	for _, dependency := range dependencies {
		add("dependency-network", dependency.State, dependency.Service+" → "+dependency.Dependency, dependency.ObservedAt)
	}
	if details {
		if p := principal(r); p != nil && !p.Allows("logs", id) {
			s.problem(w, 403, "capability_denied", "Log access needs the logs capability")
			return
		}
	} else {
		logs = nil
	}
	for i := range logs {
		logs[i].Line = deploy.Redact(a.Config, logs[i].Line)
		if serving != nil {
			logs[i].Line = deploy.Redact(serving.Config, logs[i].Line)
		}
	}
	duration := int64(0)
	if latest != nil {
		end := latest.FinishedAt
		if end == 0 {
			end = time.Now().Unix()
		}
		duration = max(0, end-latest.CreatedAt)
	}
	s.json(w, 200, map[string]any{"schemaVersion": "1", "observedAt": time.Now().Unix(), "applicationId": id, "name": a.Name, "desiredState": a.DesiredState, "servingRelease": serving, "latestOperation": func() any {
		if latest == nil {
			return nil
		}
		return publicDeployment(r, *latest)
	}(), "failedStage": stage, "durationSeconds": duration, "checks": checks, "observations": facts, "changesSinceServing": changes, "taskResults": runs, "logs": logs, "logReference": "/api/v1/applications/" + id + "/deployments/", "vantage": "Shipper control plane", "unknowns": []string{"Independent external reachability is not measured", "Boot service readiness is not measured", "Application-level dependency protocols need declared health checks"}})
}
func (s *Server) notificationSettings(w http.ResponseWriter, r *http.Request) {
	c, e := operations.LoadNotifications(r.Context(), s.db)
	if e != nil {
		s.internal(w, e)
		return
	}
	c.Configured = c.Webhook != "" || c.SMTP != ""
	c.Webhook = ""
	c.Password = ""
	s.json(w, 200, c)
}
func (s *Server) saveNotifications(w http.ResponseWriter, r *http.Request) {
	var c operations.NotificationConfig
	if !s.decode(w, r, &c) {
		return
	}
	old, e := operations.LoadNotifications(r.Context(), s.db)
	if e != nil {
		s.internal(w, e)
		return
	}
	if c.Webhook == "" {
		c.Webhook = old.Webhook
	}
	if c.Password == "" {
		c.Password = old.Password
	}
	if e = c.Validate(); e != nil {
		s.problem(w, 422, "invalid_notification", e.Error())
		return
	}
	raw, _ := json.Marshal(c)
	if e = s.db.SavePrivateSetting(r.Context(), "notifications", string(raw)); e != nil {
		s.internal(w, e)
		return
	}
	s.notificationSettings(w, r)
}
func (s *Server) testNotification(w http.ResponseWriter, r *http.Request) {
	c, e := operations.LoadNotifications(r.Context(), s.db)
	if e != nil {
		s.internal(w, e)
		return
	}
	if !c.Enabled {
		s.problem(w, 422, "notifications_disabled", "Enable notifications first")
		return
	}
	e = operations.Send(r.Context(), c, store.OperationEvent{Kind: "test", State: "healthy", Summary: "Shipper notification test", CreatedAt: time.Now().Unix()})
	if e != nil {
		s.problem(w, 502, "delivery_failed", "Notification delivery failed")
		return
	}
	s.json(w, 200, map[string]string{"state": "delivered"})
}
func (s *Server) recheckSystem(w http.ResponseWriter, r *http.Request) {
	s.monitor.Trigger()
	s.json(w, 202, map[string]string{"state": "queued"})
}
func (s *Server) uploadSource(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Worker == nil {
		s.problem(w, 503, "unavailable", "Source storage is unavailable")
		return
	}
	app := r.URL.Query().Get("application")
	if app != "" {
		if _, e := s.db.Application(r.Context(), app); e != nil {
			s.resourceError(w, e)
			return
		}
	}
	select {
	case s.protectionSlots <- struct{}{}:
		defer func() { <-s.protectionSlots }()
	default:
		s.problem(w, 429, "upload_busy", "Another archive operation is running")
		return
	}
	// Sources are tar bodies, authenticated before any disk write.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(10 * time.Minute))
	r.Body = http.MaxBytesReader(w, r.Body, 70<<20)
	v, e := s.db.AcceptSource(r.Context(), r.Body, r.Context().Value(sessionKey).(store.Session).User.ID, app, r.URL.Query().Get("provenance"))
	if e != nil {
		s.problem(w, 422, "invalid_source", e.Error())
		return
	}
	s.json(w, 201, v)
}
