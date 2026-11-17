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
