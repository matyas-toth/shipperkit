package operations

import (
	"context"
	"path/filepath"
	"testing"

	"shipper/internal/store"
)

func TestSampleMetricsPersistsBoundedSample(t *testing.T) {
	t.Setenv("SHIPPER_DISABLE_HOST_METRICS", "")
	dir := filepath.Join(t.TempDir(), "data")
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	service := Service{DB: db, DataDir: dir}
	service.SampleMetrics(context.Background())
	sample, ok, e := db.LatestMetricSample(context.Background())
	if e != nil || !ok {
		t.Fatalf("latest sample: %+v %v %v", sample, ok, e)
	}
	if sample.SampledAt == 0 || sample.DiskTotal == 0 {
		t.Fatalf("unexpected sample: %+v", sample)
	}
}
