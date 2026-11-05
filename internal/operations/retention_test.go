package operations

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"shipper/internal/store"
)

func TestReviewedProxyRepairRejectsChangedConfiguration(t *testing.T) {
	db, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	called := 0
	s := Service{DB: db, Instance: "fixture", RepairProxy: func(context.Context) error { called++; return nil }}
	revision, e := s.RepairRevision(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.EnqueueMaintenance(ctx, store.MaintenanceRun{ID: "stale", Kind: "proxy", Revision: revision}); e != nil {
		t.Fatal(e)
	}
	s.ConfigIdentity = "changed"
	if !s.ProcessMaintenance(ctx) || called != 0 {
		t.Fatal("Stale repair executed")
	}
	revision, e = s.RepairRevision(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.EnqueueMaintenance(ctx, store.MaintenanceRun{ID: "fresh", Kind: "proxy", Revision: revision}); e != nil {
		t.Fatal(e)
	}
	if !s.ProcessMaintenance(ctx) || called != 1 {
		t.Fatal("Reviewed repair did not execute")
	}
}
func TestDependencyNetworkFactsRemainSeparateFromProtocolProof(t *testing.T) {
	d := store.Deployment{ComposePlan: `{"project":{"services":{"api":{"depends_on":{"db":{"condition":"service_started"}}}}}}`}
	facts := []store.Observation{}
	for _, service := range []string{"api", "db"} {
		raw, _ := json.Marshal(map[string]any{"service": service, "state": "running", "networks": []string{"owned"}})
		facts = append(facts, store.Observation{Kind: "service", Details: raw, ObservedAt: time.Now().Unix()})
	}
	result := Dependencies(d, facts)
	if len(result) != 1 || result[0].State != "healthy" || result[0].Protocol != "not_checked" {
		t.Fatal(result)
	}
	facts[1].Stale = true
	result = Dependencies(d, facts)
	if result[0].State != "unknown" {
		t.Fatal("Stale evidence treated as current", result)
	}
}
