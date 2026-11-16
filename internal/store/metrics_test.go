package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMetricSamplesRoundTripAndPrune(t *testing.T) {
	db, e := Open(filepath.Join(t.TempDir(), "data"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	items := []MetricSample{
		{SampledAt: 100, CPUPercent: -1, MemoryUsed: 1, MemoryTotal: 2, DiskUsed: 3, DiskTotal: 4, Load1: 0.5, ContainersRunning: 1, ContainersTotal: 2},
		{SampledAt: 200, CPUPercent: 25, MemoryUsed: 1, MemoryTotal: 2, DiskUsed: 3, DiskTotal: 4, Load1: 0.7, ContainersRunning: 2, ContainersTotal: 2},
	}
	for _, item := range items {
		if e = db.SaveMetricSample(ctx, item); e != nil {
			t.Fatal(e)
		}
	}
	got, e := db.MetricSamples(ctx, 0)
	if e != nil || len(got) != 2 {
		t.Fatalf("samples: %v %v", got, e)
	}
	if got[1].CPUPercent != 25 || got[1].ContainersRunning != 2 {
		t.Fatalf("unexpected sample: %+v", got[1])
	}
	latest, ok, e := db.LatestMetricSample(ctx)
	if e != nil || !ok || latest.SampledAt != 200 {
		t.Fatalf("latest: %+v %v %v", latest, ok, e)
	}
	if e = db.PruneMetricSamples(ctx, 150); e != nil {
		t.Fatal(e)
	}
	got, e = db.MetricSamples(ctx, 0)
	if e != nil || len(got) != 1 || got[0].SampledAt != 200 {
		t.Fatalf("after prune: %v %v", got, e)
	}
}
