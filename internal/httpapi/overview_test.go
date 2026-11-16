package httpapi

import (
	"testing"
	"time"

	"shipper/internal/store"
)

func TestBuildOverviewDownsamplesAndEvaluatesThresholds(t *testing.T) {
	now := time.Unix(1794476825, 0)
	samples := []store.MetricSample{
		{SampledAt: now.Unix() - 1200, CPUPercent: 20, MemoryUsed: 2, MemoryTotal: 8, DiskUsed: 40, DiskTotal: 100},
		{SampledAt: now.Unix() - 600, CPUPercent: 30, MemoryUsed: 3, MemoryTotal: 8, DiskUsed: 40, DiskTotal: 100},
		{SampledAt: now.Unix(), CPUPercent: 40, MemoryUsed: 4, MemoryTotal: 8, DiskUsed: 40, DiskTotal: 100},
	}
	result := buildOverview(now, samples)
	if len(result.Series.CPU) != 145 || len(result.Series.Memory) != 145 || len(result.Series.Disk) != 145 {
		t.Fatalf("unexpected series length: %d %d %d", len(result.Series.CPU), len(result.Series.Memory), len(result.Series.Disk))
	}
	if result.CPU.Percent == nil || *result.CPU.Percent != 40 {
		t.Fatalf("unexpected CPU value: %v", result.CPU.Percent)
	}
	if result.Memory.Percent == nil || *result.Memory.Percent != 50 {
		t.Fatalf("unexpected memory value: %v", result.Memory.Percent)
	}
	if result.Disk.Percent == nil || *result.Disk.Percent != 40 {
		t.Fatalf("unexpected disk value: %v", result.Disk.Percent)
	}
	if result.Thresholds["cpu"] != "healthy" || result.Thresholds["memory"] != "healthy" || result.Thresholds["disk"] != "healthy" {
		t.Fatalf("unexpected thresholds: %v", result.Thresholds)
	}
	if result.Measuring || result.Stale || result.Vantage != "host" {
		t.Fatalf("unexpected state: measuring=%v stale=%v vantage=%q", result.Measuring, result.Stale, result.Vantage)
	}
	last := result.Series.Disk[len(result.Series.Disk)-1]
	if last == nil || *last != 40 {
		t.Fatalf("unexpected last disk point: %v", last)
	}
}

func TestBuildOverviewReportsMeasuringAndUnavailable(t *testing.T) {
	now := time.Unix(1794476825, 0)
	measuring := buildOverview(now, []store.MetricSample{{SampledAt: now.Unix(), CPUPercent: -1, MemoryUsed: 1, MemoryTotal: 8, DiskUsed: 1, DiskTotal: 10}})
	if !measuring.Measuring || measuring.CPU.Percent != nil || measuring.UnavailableCPU {
		t.Fatalf("unexpected first sample state: %+v", measuring)
	}
	empty := buildOverview(now, nil)
	if !empty.Stale || !empty.UnavailableCPU || empty.Vantage != "unavailable" {
		t.Fatalf("unexpected empty state: %+v", empty)
	}
}

func TestBuildOverviewMarksDiskCriticalAndMemoryWarning(t *testing.T) {
	now := time.Unix(1794476825, 0)
	result := buildOverview(now, []store.MetricSample{{SampledAt: now.Unix(), CPUPercent: 10, MemoryUsed: 9, MemoryTotal: 10, DiskUsed: 95, DiskTotal: 100}})
	if result.Thresholds["disk"] != "critical" {
		t.Fatalf("disk threshold = %q", result.Thresholds["disk"])
	}
	if result.Thresholds["memory"] != "warning" {
		t.Fatalf("memory threshold = %q", result.Thresholds["memory"])
	}
}
