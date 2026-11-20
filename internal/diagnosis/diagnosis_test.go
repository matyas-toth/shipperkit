package diagnosis

import (
	"strings"
	"testing"

	"shipper/internal/inventory"
	"shipper/internal/store"
)

func TestClassifyFailureRecognisesCommonSignatures(t *testing.T) {
	tests := []struct {
		name   string
		logs   []store.LogLine
		state  string
		code   string
		conf   string
	}{
		{
			name:  "healthcheck",
			logs:  []store.LogLine{{Step: "starting", Line: "Container shp-garage-1 is unhealthy"}},
			state: "failed", code: "healthcheck_failed", conf: "high",
		},
		{
			name:  "readiness command",
			logs:  []store.LogLine{{Step: "checking", Line: "The candidate did not pass its HTTP health check: Container command failed (1): missing configuration"}},
			state: "failed", code: "readiness_failed", conf: "high",
		},
		{
			name:  "container exit",
			logs:  []store.LogLine{{Step: "starting", Line: "Container shp-api exited with code 1"}},
			state: "failed", code: "container_exited", conf: "high",
		},
		{
			name:  "image pull",
			logs:  []store.LogLine{{Step: "pulling", Line: "pull access denied for private/api"}},
			state: "failed", code: "image_pull_failed", conf: "high",
		},
		{
			name:  "disk",
			logs:  []store.LogLine{{Step: "building", Line: "write /var/lib/docker: no space left on device"}},
			state: "failed", code: "disk_pressure", conf: "high",
		},
		{
			name:  "build fallback",
			logs:  []store.LogLine{{Step: "building", Line: "npm ERR! missing script: build"}},
			state: "failed", code: "build_failed", conf: "medium",
		},
		{
			name:  "unknown",
			logs:  []store.LogLine{{Step: "starting", Line: "something went wrong"}},
			state: "failed", code: "unknown", conf: "low",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := classifyFailure(test.logs, store.Deployment{State: test.state})
			if result.Code != test.code || result.Confidence != test.conf {
				t.Fatalf("got %s/%s, want %s/%s", result.Code, result.Confidence, test.code, test.conf)
			}
		})
	}
}

func TestEvidenceWindowRedactsAndBounds(t *testing.T) {
	logs := []store.LogLine{
		{Line: "secret one"},
		{Line: "secret two"},
		{Line: "Container app is unhealthy"},
		{Line: "after"},
	}
	redact := func(value string) string { return strings.ReplaceAll(value, "secret", "[REDACTED]") }
	items := evidenceWindow(logs, 2, redact)
	if len(items) != 3 {
		t.Fatalf("evidence length = %d", len(items))
	}
	if items[0] != "[REDACTED] one" || items[2] != "Container app is unhealthy" {
		t.Fatalf("unexpected evidence: %v", items)
	}
}

func TestDiagnosisChecksMarksRuntimeAndReadiness(t *testing.T) {
	checks := diagnosisChecks(
		store.Application{ID: "app"},
		store.Deployment{ID: "dep", State: "failed", ImageID: "compose"},
		inventory.Health{Connected: true},
		"starting",
		[]DiagnosisService{{Name: "garage", State: "running", Health: "unhealthy"}},
	)
	states := map[string]string{}
	for _, check := range checks {
		states[check.Layer] = check.State
	}
	if states["daemon"] != "healthy" || states["build"] != "healthy" || states["runtime"] != "failed" || states["readiness"] != "failed" {
		t.Fatalf("unexpected checks: %v", states)
	}
}

func TestDiagnosisServicesUsesObservationDetails(t *testing.T) {
	app := store.Application{ID: "app"}
	d := store.Deployment{ID: "dep", Services: []store.Service{{Name: "garage", Image: "dxflrs/garage:v2.3.0"}}}
	containers := []store.Container{{ID: "container", Name: "shp-garage", ApplicationID: "app", Service: "garage", State: "running", Health: "unhealthy"}}
	observations := []store.Observation{{Kind: "service", Reference: "container", Details: []byte(`{"service":"garage","exitCode":0,"restarts":2,"cpuPercent":8.5,"memoryBytes":1048576}`)}}
	items := diagnosisServices(app, d, containers, observations)
	if len(items) != 1 {
		t.Fatalf("services = %d", len(items))
	}
	if items[0].Name != "garage" || items[0].Restarts != 2 || items[0].CPUPercent == nil || *items[0].CPUPercent != 8.5 {
		t.Fatalf("unexpected service: %+v", items[0])
	}
}

func TestSpecificCausesNameTheRootCause(t *testing.T) {
	for _, tc := range []struct {
		lines []string
		code  string
		want  string
	}{
		{[]string{"npm warn config production Use `--omit=dev` instead.", "Error: MongoDB error", "Kind: Server selection timeout: failed to lookup address information: Name or service not known", "before start failed: exit status 1"}, "host_unresolved", "Name or service not known"},
		{[]string{"wget: bad address 'db-0123'"}, "host_unresolved", "bad address"},
		{[]string{"Error: connect ECONNREFUSED 10.0.0.2:5432"}, "connection_refused", "ECONNREFUSED"},
		{[]string{"npm warn deprecated thing", "Error: Cannot find module 'prisma'", "before start failed: exit status 1"}, "before_start_failed", "Cannot find module"},
	} {
		cause, detail, ok := specificCause(tc.lines)
		if !ok || cause.Code != tc.code || !strings.Contains(detail, tc.want) {
			t.Fatalf("%v: got %s %q", tc.lines, cause.Code, detail)
		}
	}
	if line := errorLine([]string{"Error: real problem", "npm warn config production", "Datasource loaded"}); line != "Error: real problem" {
		t.Fatalf("error line %q", line)
	}
}
