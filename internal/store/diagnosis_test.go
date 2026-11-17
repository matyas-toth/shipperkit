package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDeploymentDiagnosisRoundTripAndPrune(t *testing.T) {
	db, e := Open(filepath.Join(t.TempDir(), "data"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	if e = db.SaveDeploymentDiagnosis(ctx, "deployment", "app", 100, []byte(`{"services":[]}`)); e != nil {
		t.Fatal(e)
	}
	raw, ok, e := db.DeploymentDiagnosis(ctx, "deployment")
	if e != nil || !ok || string(raw) != `{"services":[]}` {
		t.Fatalf("diagnosis: %s %v %v", raw, ok, e)
	}
	if e = db.SaveDeploymentDiagnosis(ctx, "deployment", "app", 200, []byte(`{"services":[{"service":"api"}]}`)); e != nil {
		t.Fatal(e)
	}
	raw, ok, e = db.DeploymentDiagnosis(ctx, "deployment")
	if e != nil || !ok || string(raw) != `{"services":[{"service":"api"}]}` {
		t.Fatalf("updated diagnosis: %s %v %v", raw, ok, e)
	}
	if e = db.PruneDeploymentDiagnoses(ctx, 300); e != nil {
		t.Fatal(e)
	}
	_, ok, e = db.DeploymentDiagnosis(ctx, "deployment")
	if e != nil || ok {
		t.Fatalf("pruned diagnosis still present: %v %v", ok, e)
	}
}
