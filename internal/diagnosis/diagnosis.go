package diagnosis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"shipper/internal/deploy"
	"shipper/internal/inventory"
	"shipper/internal/store"
)

// Diagnosis is the bounded explanation of a failed deployment. It is derived
// from stored logs and the last observed container facts, never from a model.
type Diagnosis struct {
	SchemaVersion       int                  `json:"schemaVersion"`
	GeneratedAt         int64                `json:"generatedAt"`
	Vantage             string               `json:"vantage"`
	Application         DiagnosisApplication `json:"application"`
	Deployment          DiagnosisDeployment  `json:"deployment"`
	Cause               DiagnosisCause       `json:"cause"`
	Checks              []DiagnosisCheck     `json:"checks"`
	Services            []DiagnosisService   `json:"services"`
	Logs                []DiagnosisLog       `json:"logs"`
	ChangesSinceServing []string             `json:"changesSinceServing"`
	Unknowns            []string             `json:"unknowns"`
	NextActions         []DiagnosisAction    `json:"nextActions"`
}

type DiagnosisApplication struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Method string `json:"method"`
}

type DiagnosisDeployment struct {
	ID              string `json:"id"`
	Operation       string `json:"operation"`
	State           string `json:"state"`
	Stage           string `json:"stage"`
	DurationSeconds int64  `json:"durationSeconds"`
	CreatedAt       int64  `json:"createdAt"`
	Error           string `json:"error,omitempty"`
}

type DiagnosisCause struct {
	Code       string   `json:"code"`
	Summary    string   `json:"summary"`
	Detail     string   `json:"detail"`
	Confidence string   `json:"confidence"`
	Evidence   []string `json:"evidence"`
}

type DiagnosisCheck struct {
	Layer      string `json:"layer"`
	State      string `json:"state"`
	Summary    string `json:"summary"`
	ObservedAt int64  `json:"observedAt"`
}

type DiagnosisService struct {
	Name        string   `json:"name"`
	Image       string   `json:"image,omitempty"`
	State       string   `json:"state"`
	Health      string   `json:"health"`
	ExitCode    int      `json:"exitCode"`
	Restarts    int      `json:"restarts"`
	OOMKilled   bool     `json:"oomKilled"`
	CPUPercent  *float64 `json:"cpuPercent,omitempty"`
	MemoryBytes *int64   `json:"memoryBytes,omitempty"`
	MemoryLimit *int64   `json:"memoryLimit,omitempty"`
	RuntimePort int      `json:"runtimePort,omitempty"`
	HealthPath  string   `json:"healthPath,omitempty"`
}

type DiagnosisLog struct {
	Service string   `json:"service"`
	Lines   []string `json:"lines"`
}

type DiagnosisAction struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

type observationDetails struct {
	Service     string   `json:"service"`
	ExitCode    int      `json:"exitCode"`
	Restarts    int      `json:"restarts"`
	OOMKilled   bool     `json:"oomKilled"`
	CPUPercent  *float64 `json:"cpuPercent"`
	MemoryBytes *int64   `json:"memoryBytes"`
	MemoryLimit *int64   `json:"memoryLimit"`
}

type causeMatch struct {
	Code       string
	Summary    string
	Detail     string
	Confidence string
	Index      int
}

func deploymentStage(logs []store.LogLine) string {
	stage := ""
	for _, line := range logs {
		if line.Step != "" && line.Step != "failed" {
			stage = line.Step
		}
	}
	return stage
}

// deploymentLimit matches the deploy worker's limit for one deployment.
const deploymentLimit = 30 * time.Minute

// killedProcess explains a "signal: killed" line, which Shipper's own build and clone limits also produce.
func killedProcess(logs []store.LogLine, d store.Deployment, i int) causeMatch {
	if d.StartedAt > 0 && d.FinishedAt-d.StartedAt >= int64(deploymentLimit/time.Second) {
		return causeMatch{Code: "build_failed", Summary: "The deployment hit its 30 minute limit.", Confidence: "high", Index: i}
	}
	if deploymentStage(logs[:i+1]) == "cloning" {
		return causeMatch{Code: "build_failed", Summary: "The Git clone hit its time or size limit.", Confidence: "medium", Index: i}
	}
	return causeMatch{Code: "build_failed", Summary: "A build process was killed.", Confidence: "medium", Index: i}
}

func classifyFailure(logs []store.LogLine, d store.Deployment) causeMatch {
	for i := len(logs) - 1; i >= 0; i-- {
		line := strings.ToLower(logs[i].Line)
		switch {
		case strings.Contains(line, "did not pass its http health check"):
			return causeMatch{Code: "readiness_failed", Summary: "The candidate started, but its HTTP health check did not pass.", Confidence: "high", Index: i}
		case strings.Contains(line, "container command failed"):
			return causeMatch{Code: "container_exited", Summary: "A container command exited before the release became ready.", Confidence: "high", Index: i}
		case strings.Contains(line, "is unhealthy") || strings.Contains(line, "unhealthy"):
			return causeMatch{Code: "healthcheck_failed", Summary: "A service started, but its health check never passed.", Confidence: "high", Index: i}
		case strings.Contains(line, "exited with code") || strings.Contains(line, "exit code"):
			return causeMatch{Code: "container_exited", Summary: "A container exited before the release became ready.", Confidence: "high", Index: i}
		case strings.Contains(line, "pull access denied") || strings.Contains(line, "manifest unknown") || strings.Contains(line, "failed to resolve reference") || strings.Contains(line, "unauthorized: authentication required"):
			return causeMatch{Code: "image_pull_failed", Summary: "The release image could not be pulled.", Confidence: "high", Index: i}
		case strings.Contains(line, "no space left on device"):
			return causeMatch{Code: "disk_pressure", Summary: "The host ran out of disk space.", Confidence: "high", Index: i}
		case strings.Contains(line, "port is already allocated") || strings.Contains(line, "address already in use"):
			return causeMatch{Code: "port_mismatch", Summary: "A service could not bind its port.", Confidence: "high", Index: i}
		case strings.Contains(line, "out of memory"):
			return causeMatch{Code: "oom_killed", Summary: "A container was killed for using too much memory.", Confidence: "high", Index: i}
		case strings.Contains(line, "signal: killed"):
			return killedProcess(logs, d, i)
		case strings.Contains(line, "permission denied"):
			return causeMatch{Code: "permission_denied", Summary: "A file or directory could not be accessed.", Confidence: "medium", Index: i}
		}
	}
	if d.State == "failed" {
		stage := deploymentStage(logs)
		if stage == "cloning" || stage == "building" || stage == "packaging" || stage == "pulling" {
			return causeMatch{Code: "build_failed", Summary: "The build failed before a release image was created.", Confidence: "medium", Index: len(logs) - 1}
		}
	}
	return causeMatch{Code: "unknown", Summary: "The deployment failed, but Shipper could not isolate the cause.", Confidence: "low", Index: len(logs) - 1}
}

// specificCauses name root causes that the generic readiness failure hides.
// They are checked in order, most specific first.
var specificCauses = []struct {
	needles []string
	match   causeMatch
}{
	{[]string{"to be run as a replica set", "only allowed on a replica set member", "transactions are not supported by this deployment"}, causeMatch{Code: "mongodb_replica_set", Summary: "MongoDB transactions need a replica set. Redeploy a managed MongoDB database to run it as one, or start your own MongoDB image with --replSet.", Confidence: "high"}},
	{[]string{"name or service not known", "no such host", "enotfound", "temporary failure in name resolution", "could not translate host name", "nodename nor servname", "could not resolve host", "bad address"}, causeMatch{Code: "host_unresolved", Summary: "The application could not resolve a host it connects to. Check the hostname and connect both applications in Networking.", Confidence: "high"}},
	{[]string{"econnrefused", "connection refused"}, causeMatch{Code: "connection_refused", Summary: "A service the application connects to refused the connection.", Confidence: "medium"}},
	{[]string{"unable to set group id", "unable to set user id", "setgroups", "chown(", "changing ownership of", "setgid: operation not permitted", "setuid: operation not permitted", "can't drop privilege"}, causeMatch{Code: "permissions_restricted", Summary: "The image changes file owners or switches users at startup. Turn on ownership changes and user switching in Runtime settings.", Confidence: "high"}},
	{[]string{"before start failed"}, causeMatch{Code: "before_start_failed", Summary: "The before start command failed, so the application never started.", Confidence: "high"}},
}

func specificCause(lines []string) (causeMatch, string, bool) {
	for _, cause := range specificCauses {
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.ToLower(lines[i])
			for _, needle := range cause.needles {
				if strings.Contains(line, needle) {
					detail := lines[i]
					if cause.match.Code == "before_start_failed" {
						// "exit status 1" says nothing; the command's own error comes first.
						if found := errorLine(lines[:i]); found != "" {
							detail = found
						}
					}
					return cause.match, detail, true
				}
			}
		}
	}
	return causeMatch{}, "", false
}

var errorWords = []string{"error", "fatal", "exception", "panic", "failed", "refused", "not known", "timeout", "timed out", "denied", "cannot", "unable", "not found"}

// errorLine picks the newest line that reads like an error, skipping warnings,
// so the detail is never an unrelated progress or warning line.
func errorLine(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.ToLower(strings.TrimSpace(lines[i]))
		if line == "" || strings.Contains(line, "warn") {
			continue
		}
		for _, word := range errorWords {
			if strings.Contains(line, word) {
				return strings.TrimSpace(lines[i])
			}
		}
	}
	return ""
}

func evidenceWindow(logs []store.LogLine, index int, redact func(string) string) []string {
	if index < 0 || len(logs) == 0 {
		return []string{}
	}
	start := max(0, index-2)
	end := min(len(logs), index+1)
	items := []string{}
	for _, line := range logs[start:end] {
		value := strings.TrimSpace(redact(line.Line))
		if value != "" {
			items = append(items, value)
		}
	}
	return items
}

func observationFor(observations []store.Observation, containerID string) *observationDetails {
	for _, observation := range observations {
		if observation.Kind != "service" || observation.Reference != containerID || len(observation.Details) == 0 {
			continue
		}
		var details observationDetails
		if e := json.Unmarshal(observation.Details, &details); e == nil {
			return &details
		}
	}
	return nil
}

func diagnosisServices(app store.Application, d store.Deployment, containers []store.Container, observations []store.Observation) []DiagnosisService {
	items := []DiagnosisService{}
	seen := map[string]bool{}
	for _, container := range containers {
		if container.ApplicationID != app.ID && container.DeploymentID != d.ID {
			continue
		}
		name := container.Service
		if name == "" {
			name = container.Name
		}
		seen[name] = true
		service := DiagnosisService{Name: name, Image: container.Image, State: container.State, Health: container.Health, RuntimePort: d.RuntimePort}
		if details := observationFor(observations, container.ID); details != nil {
			service.ExitCode = details.ExitCode
			service.Restarts = details.Restarts
			service.OOMKilled = details.OOMKilled
			service.CPUPercent = details.CPUPercent
			service.MemoryBytes = details.MemoryBytes
			service.MemoryLimit = details.MemoryLimit
		}
		items = append(items, service)
	}
	for _, service := range d.Services {
		if seen[service.Name] {
			continue
		}
		items = append(items, DiagnosisService{Name: service.Name, Image: service.Image, State: "unknown", Health: "unknown", RuntimePort: service.Port, HealthPath: service.HealthPath})
	}
	if len(items) == 0 {
		name := d.Service
		if name == "" {
			name = app.Name
		}
		items = append(items, DiagnosisService{Name: name, State: "unknown", Health: "unknown", RuntimePort: d.RuntimePort})
	}
	return items
}

func diagnosisChecks(app store.Application, d store.Deployment, health inventory.Health, stage string, services []DiagnosisService) []DiagnosisCheck {
	checks := []DiagnosisCheck{}
	daemon := "failed"
	if health.Connected {
		daemon = "healthy"
	}
	checks = append(checks, DiagnosisCheck{Layer: "daemon", State: daemon, Summary: "Docker connection", ObservedAt: health.CheckedAt})
	build := "unknown"
	if d.ImageID != "" {
		build = "healthy"
	}
	if d.State == "failed" && (stage == "cloning" || stage == "building" || stage == "pulling" || stage == "packaging") {
		build = "failed"
	}
	checks = append(checks, DiagnosisCheck{Layer: "build", State: build, Summary: "Release artifact", ObservedAt: d.StartedAt})
	runtime := "unknown"
	for _, service := range services {
		switch {
		case service.State == "running" && (service.Health == "" || service.Health == "healthy"):
			runtime = "healthy"
		case service.State == "running" && service.Health == "unhealthy", service.State == "exited", service.State == "dead":
			runtime = "failed"
		}
	}
	summary := "Observed container state"
	if len(services) == 1 {
		summary = services[0].Name + " is " + services[0].State
		if services[0].Health != "" && services[0].Health != "none" {
			summary += ", health " + services[0].Health
		}
	}
	checks = append(checks, DiagnosisCheck{Layer: "runtime", State: runtime, Summary: summary, ObservedAt: health.ObservedAt})
	readiness := "unknown"
	if d.State == "failed" && (stage == "starting" || stage == "checking") {
		readiness = "failed"
	} else if d.State == "succeeded" {
		readiness = "healthy"
	}
	checks = append(checks, DiagnosisCheck{Layer: "readiness", State: readiness, Summary: "Readiness check", ObservedAt: d.FinishedAt})
	return checks
}

func diagnosisActions(app store.Application, d store.Deployment, cause string, stage string) []DiagnosisAction {
	base := "/applications/" + app.ID
	actions := []DiagnosisAction{}
	switch cause {
	case "build_failed", "image_pull_failed":
		actions = append(actions, DiagnosisAction{ID: "source", Label: "Open source settings", Kind: "navigate", Target: base + "?tab=settings&section=source"})
	case "healthcheck_failed", "container_exited", "readiness_failed", "port_mismatch", "oom_killed":
		actions = append(actions, DiagnosisAction{ID: "logs", Label: "Open runtime logs", Kind: "navigate", Target: base + "?tab=logs"})
		actions = append(actions, DiagnosisAction{ID: "environment", Label: "Open environment", Kind: "navigate", Target: base + "?tab=env"})
	case "host_unresolved", "connection_refused":
		actions = append(actions, DiagnosisAction{ID: "networking", Label: "Open networking", Kind: "navigate", Target: base + "?tab=networking"})
		actions = append(actions, DiagnosisAction{ID: "environment", Label: "Open environment", Kind: "navigate", Target: base + "?tab=env"})
	case "permissions_restricted":
		actions = append(actions, DiagnosisAction{ID: "runtime", Label: "Open runtime settings", Kind: "navigate", Target: base + "?tab=settings&section=runtime"})
		actions = append(actions, DiagnosisAction{ID: "logs", Label: "Open runtime logs", Kind: "navigate", Target: base + "?tab=logs"})
	case "mongodb_replica_set":
		actions = append(actions, DiagnosisAction{ID: "environment", Label: "Open environment", Kind: "navigate", Target: base + "?tab=env"})
	case "before_start_failed":
		actions = append(actions, DiagnosisAction{ID: "logs", Label: "Open runtime logs", Kind: "navigate", Target: base + "?tab=logs"})
		actions = append(actions, DiagnosisAction{ID: "settings", Label: "Open settings", Kind: "navigate", Target: base + "?tab=settings"})
	case "disk_pressure":
		actions = append(actions, DiagnosisAction{ID: "system", Label: "Open system", Kind: "navigate", Target: "/system"})
	}
	if d.State == "failed" {
		actions = append(actions, DiagnosisAction{ID: "retry", Label: "Retry release", Kind: "retry", Target: d.ID})
	}
	actions = append(actions, DiagnosisAction{ID: "copy", Label: "Copy for agent", Kind: "copy", Target: "diagnosis"})
	if stage == "" {
		return actions
	}
	return actions
}

// DiagnoseDeployment explains a failed deployment from bounded stored evidence.
// It never runs host commands and never reveals unmasked configuration values.
func DiagnoseDeployment(ctx context.Context, db *store.Store, appID, deploymentID string, health inventory.Health) (Diagnosis, error) {
	app, e := db.Application(ctx, appID)
	if e != nil {
		return Diagnosis{}, e
	}
	d, e := db.Deployment(ctx, deploymentID)
	if e != nil {
		return Diagnosis{}, e
	}
	if d.ApplicationID != appID {
		return Diagnosis{}, fmt.Errorf("deployment does not belong to this application")
	}
	logs, _ := db.TailLogs(ctx, deploymentID, 100)
	observations, _ := db.Observations(ctx, appID)
	containers, _ := db.Containers(ctx)
	filtered := []store.Container{}
	for _, container := range containers {
		if container.DeploymentID == deploymentID {
			filtered = append(filtered, container)
		}
	}
	serviceLogs := []deploy.FailureServiceLog{}
	if raw, ok, _ := db.DeploymentDiagnosis(ctx, deploymentID); ok {
		var snapshot deploy.FailureSnapshot
		if json.Unmarshal(raw, &snapshot) == nil {
			serviceLogs = snapshot.Services
		}
	}
	stage := deploymentStage(logs)
	match := classifyFailure(logs, d)
	redact := func(value string) string {
		return deploy.RedactDeployment(d, value)
	}
	detail := strings.TrimSpace(redact(d.Error))
	if detail == "" || match.Code != "unknown" {
		if match.Index >= 0 && match.Index < len(logs) {
			detail = strings.TrimSpace(redact(logs[match.Index].Line))
		}
	}
	serviceDetail := ""
	serviceLines := []string{}
	for _, service := range serviceLogs {
		lines := []string{}
		for _, line := range service.Lines {
			if value := strings.TrimSpace(redact(line)); value != "" {
				lines = append(lines, value)
			}
		}
		serviceLines = append(serviceLines, lines...)
		if serviceDetail == "" {
			serviceDetail = errorLine(lines)
		}
		if serviceDetail == "" && len(lines) > 0 {
			serviceDetail = lines[len(lines)-1]
		}
	}
	if serviceDetail != "" {
		detail = serviceDetail
	}
	deploymentLines := []string{}
	for _, line := range logs {
		deploymentLines = append(deploymentLines, strings.TrimSpace(redact(line.Line)))
	}
	specificEvidence := ""
	if d.State == "failed" {
		if cause, line, ok := specificCause(append(deploymentLines, serviceLines...)); ok {
			match.Code, match.Summary, match.Confidence = cause.Code, cause.Summary, cause.Confidence
			detail, specificEvidence = line, line
		}
	}
	if len(detail) > 600 {
		detail = detail[:600]
	}
	services := diagnosisServices(app, d, filtered, observations)
	checks := diagnosisChecks(app, d, health, stage, services)
	diagLogs := []DiagnosisLog{}
	for _, service := range serviceLogs {
		lines := []string{}
		for _, line := range service.Lines {
			value := strings.TrimSpace(redact(line))
			if value != "" {
				lines = append(lines, value)
			}
		}
		if len(lines) > 20 {
			lines = lines[len(lines)-20:]
		}
		if len(lines) > 0 {
			diagLogs = append(diagLogs, DiagnosisLog{Service: service.Service, Lines: lines})
		}
	}
	lines := []string{}
	for _, line := range logs {
		value := strings.TrimSpace(redact(line.Line))
		if value != "" {
			lines = append(lines, value)
		}
	}
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	if len(lines) > 0 {
		diagLogs = append(diagLogs, DiagnosisLog{Service: "", Lines: lines})
	}
	changes := []string{}
	if app.ActiveDeploymentID != "" {
		if release, err := db.Deployment(ctx, app.ActiveDeploymentID); err == nil {
			changes = deploy.PlanChanges(release.Config, app.Config).Changes
		}
	}
	unknowns := []string{"The application protocol was not tested from outside the host.", "Only the last 100 deployment log lines were inspected."}
	if len(filtered) == 0 {
		unknowns = append(unknowns, "Container state was removed during cleanup; the deployment logs are the remaining evidence.")
	}
	if len(serviceLogs) == 0 && (match.Code == "healthcheck_failed" || match.Code == "container_exited" || match.Code == "readiness_failed" || match.Code == "oom_killed" || match.Code == "before_start_failed") {
		unknowns = append(unknowns, "The failed service logs were not captured before cleanup; only the deployment output is available.")
	}
	evidence := evidenceWindow(logs, match.Index, redact)
	for _, service := range serviceLogs {
		start := max(0, len(service.Lines)-2)
		for _, line := range service.Lines[start:] {
			value := strings.TrimSpace(redact(line))
			if value != "" {
				evidence = append(evidence, value)
			}
		}
	}
	if specificEvidence != "" {
		evidence = append(evidence, specificEvidence)
	}
	if len(evidence) > 6 {
		evidence = evidence[len(evidence)-6:]
	}
	// A failure line can carry a whole log tail; the full logs follow below.
	for i, line := range evidence {
		if len(line) > 240 {
			cut := 240
			for cut > 0 && !utf8.RuneStart(line[cut]) {
				cut--
			}
			evidence[i] = line[:cut] + "…"
		}
	}
	duration := int64(0)
	if d.StartedAt > 0 {
		end := d.FinishedAt
		if end == 0 {
			end = time.Now().Unix()
		}
		duration = max(0, end-d.CreatedAt)
	}
	return Diagnosis{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().Unix(),
		Vantage:       "control-plane",
		Application:   DiagnosisApplication{ID: app.ID, Name: app.Name, Method: DiagnosisMethod(app)},
		Deployment:    DiagnosisDeployment{ID: d.ID, Operation: d.Operation, State: d.State, Stage: stage, DurationSeconds: duration, CreatedAt: d.CreatedAt, Error: redact(d.Error)},
		Cause: DiagnosisCause{
			Code:       match.Code,
			Summary:    match.Summary,
			Detail:     detail,
			Confidence: match.Confidence,
			Evidence:   evidence,
		},
		Checks:              checks,
		Services:            services,
		Logs:                diagLogs,
		ChangesSinceServing: changes,
		Unknowns:            unknowns,
		NextActions:         diagnosisActions(app, d, match.Code, stage),
	}, nil
}

func DiagnosisMethod(app store.Application) string {
	if app.Method == "" {
		return app.ActiveMethod
	}
	return app.Method
}
