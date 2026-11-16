package operations

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"os"
	"sync"
	"time"

	"shipper/internal/store"
)

var errHostMetricsUnavailable = errors.New("host metrics are not available on this platform")

type hostMetrics struct {
	mu        sync.Mutex
	cpuTotal  uint64
	cpuIdle   uint64
	lastAt    time.Time
	lastPrune time.Time
}

func clampPercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Min(100, math.Max(0, value))
}

// SampleMetrics records one bounded host sample. CPU is -1 until a second
// sample exists, so the UI can say "Measuring" instead of inventing 0%.
func (s *Service) SampleMetrics(ctx context.Context) {
	if os.Getenv("SHIPPER_DISABLE_HOST_METRICS") == "1" {
		return
	}
	sample := store.MetricSample{SampledAt: time.Now().Unix(), CPUPercent: -1}
	if disk, e := Disk(s.DataDir); e == nil && disk.Total > 0 {
		sample.DiskTotal = int64(disk.Total)
		sample.DiskUsed = int64(disk.Total - disk.Free)
	}
	if total, idle, e := readCPUTimes(); e == nil && total > 0 {
		s.metrics.mu.Lock()
		if !s.metrics.lastAt.IsZero() && total > s.metrics.cpuTotal && s.metrics.cpuTotal > 0 {
			totalDelta := total - s.metrics.cpuTotal
			idleDelta := idle - s.metrics.cpuIdle
			sample.CPUPercent = clampPercent(100 * (1 - float64(idleDelta)/float64(totalDelta)))
		}
		s.metrics.cpuTotal, s.metrics.cpuIdle = total, idle
		s.metrics.lastAt = time.Now()
		s.metrics.mu.Unlock()
	}
	if used, total, e := readMemory(); e == nil && total > 0 {
		sample.MemoryUsed = used
		sample.MemoryTotal = total
	}
	if load, e := readLoad1(); e == nil {
		sample.Load1 = load
	}
	if containers, e := s.DB.Containers(ctx); e == nil {
		sample.ContainersTotal = len(containers)
		for _, container := range containers {
			if container.State == "running" {
				sample.ContainersRunning++
			}
		}
	}
	if e := s.DB.SaveMetricSample(ctx, sample); e != nil && ctx.Err() == nil {
		slog.Warn("Host metric sample could not be saved", "error", e)
	}
	s.metrics.mu.Lock()
	prune := s.metrics.lastPrune.IsZero() || time.Since(s.metrics.lastPrune) > 6*time.Hour
	if prune {
		s.metrics.lastPrune = time.Now()
	}
	s.metrics.mu.Unlock()
	if prune {
		if e := s.DB.PruneMetricSamples(ctx, time.Now().Add(-7*24*time.Hour).Unix()); e != nil && ctx.Err() == nil {
			slog.Warn("Host metric retention failed", "error", e)
		}
	}
}
