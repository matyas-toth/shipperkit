package deploy

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"shipper/internal/store"
)

type snapshotRuntime struct {
	Runtime
	lines []string
}

func (r snapshotRuntime) LogSnapshot(_ context.Context, _ string, _ string, _ int, emit func(string) error) error {
	for _, line := range r.lines {
		if e := emit(line); e != nil {
			return e
		}
	}
	return nil
}

func TestCaptureFailureLogsStoresRedactedSnapshot(t *testing.T) {
	db, e := store.Open(filepath.Join(t.TempDir(), "data"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	worker := Worker{DB: db, Runtime: snapshotRuntime{lines: []string{"starting", "token super-secret", "crashed"}}}
	deployment := store.Deployment{ID: "deployment", ApplicationID: "app", Config: store.StaticConfig{Variables: []store.Variable{{Name: "TOKEN", Value: "super-secret", Secret: true}}}}
	worker.captureFailureLogs(context.Background(), deployment, []failureTarget{{Service: "garage", ContainerID: "container"}})
	raw, ok, e := db.DeploymentDiagnosis(context.Background(), "deployment")
	if e != nil || !ok {
		t.Fatalf("diagnosis snapshot: %v %v", ok, e)
	}
	if strings.Contains(string(raw), "super-secret") {
		t.Fatalf("snapshot contains an unredacted secret: %s", raw)
	}
	var snapshot FailureSnapshot
	if e = json.Unmarshal(raw, &snapshot); e != nil {
		t.Fatal(e)
	}
	if len(snapshot.Services) != 1 || snapshot.Services[0].Service != "garage" || len(snapshot.Services[0].Lines) != 3 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestRedactDeploymentKeepsApplicationIdentity(t *testing.T) {
	deployment := store.Deployment{
		ID:              "deployment",
		ApplicationID:   "46511337df01561aa2981789",
		ApplicationName: "Garage",
		Service:         "garage",
		Services:        []store.Service{{Name: "garage"}},
		Config:          store.StaticConfig{Variables: []store.Variable{{Name: "TOKEN", Value: "super-secret", Secret: true}}},
		ComposePlan:     `{"masks":["Garage","garage","46511337df01561aa2981789","dfbe08d5fa03","super-secret"]}`,
	}
	line := "Container shp-dfbe08d5fa03-46511337df01561aa2981789-garage Garage token super-secret"
	got := RedactDeployment(deployment, line)
	for _, expected := range []string{"Garage", "garage", "46511337df01561aa2981789", "dfbe08d5fa03"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected %q in %q", expected, got)
		}
	}
	if strings.Contains(got, "super-secret") {
		t.Fatalf("secret remained in %q", got)
	}
}
