package httpapi

import (
	"math"
	"net/http"
	"time"

	"shipper/internal/store"
)

const (
	overviewWindow = 24 * time.Hour
	overviewStep   = 10 * time.Minute
)

type overviewValue struct {
	Percent *float64 `json:"percent"`
	Used    int64    `json:"used,omitempty"`
	Total   int64    `json:"total,omitempty"`
}

type overviewSeries struct {
	From   int64      `json:"from"`
	To     int64      `json:"to"`
	Step   int64      `json:"step"`
	CPU    []*float64 `json:"cpu"`
	Memory []*float64 `json:"memory"`
	Disk   []*float64 `json:"disk"`
}

type overviewResponse struct {
	SampledAt      int64             `json:"sampledAt"`
	Stale          bool              `json:"stale"`
	Vantage        string            `json:"vantage"`
	CPU            overviewValue     `json:"cpu"`
	Memory         overviewValue     `json:"memory"`
	Disk           overviewValue     `json:"disk"`
	Load1          float64           `json:"load1"`
	Containers     map[string]int    `json:"containers"`
	Series         overviewSeries    `json:"series"`
	Thresholds     map[string]string `json:"thresholds"`
	Measuring      bool              `json:"measuring"`
	UnavailableCPU bool              `json:"unavailableCpu"`
}

func percent(used, total int64) *float64 {
	if total <= 0 {
		return nil
	}
	value := math.Round(float64(used)/float64(total)*1000) / 10
	return &value
}

func roundPercent(value float64) *float64 {
	value = math.Round(value*10) / 10
	return &value
}

func overviewState(samples []store.MetricSample, metric string) string {
	if len(samples) == 0 {
		return "unknown"
	}
	recent := samples
	if metric == "disk" {
		recent = samples[len(samples)-1:]
	} else if len(samples) > 5 {
		recent = samples[len(samples)-5:]
	}
	var total float64
	var count int
	for _, sample := range recent {
		value := sample.CPUPercent
		if metric == "memory" {
			value = 0
			if sample.MemoryTotal > 0 {
				value = float64(sample.MemoryUsed) / float64(sample.MemoryTotal) * 100
			}
		}
		if metric == "disk" {
			value = 0
			if sample.DiskTotal > 0 {
				value = float64(sample.DiskUsed) / float64(sample.DiskTotal) * 100
			}
		}
		if value < 0 {
			continue
		}
		total += value
		count++
	}
	if count == 0 {
		return "unknown"
	}
	average := total / float64(count)
	switch {
	case metric == "disk" && average >= 90, metric != "disk" && average >= 95:
		return "critical"
	case metric == "disk" && average >= 80, metric != "disk" && average >= 85:
		return "warning"
	default:
		return "healthy"
	}
}

// buildOverview downsamples the stored samples into ten-minute buckets. The
// dashboard never needs the raw ten thousand rows, and a compact series keeps
// the pulse off the critical path.
func buildOverview(now time.Time, samples []store.MetricSample) overviewResponse {
	latest := store.MetricSample{}
	if len(samples) > 0 {
		latest = samples[len(samples)-1]
	}
	to := now.Unix()
	from := to - int64(overviewWindow/time.Second)
	points := int(overviewWindow/overviewStep) + 1
	cpu := make([]*float64, points)
	memory := make([]*float64, points)
	disk := make([]*float64, points)
	cpuSum := make([]float64, points)
	memorySum := make([]float64, points)
	diskSum := make([]float64, points)
	counts := make([]int, points)
	seen := make([]int, points)
	for _, sample := range samples {
		index := int((sample.SampledAt - from) / int64(overviewStep/time.Second))
		if index < 0 || index >= points {
			continue
		}
		seen[index]++
		if sample.CPUPercent >= 0 {
			cpuSum[index] += sample.CPUPercent
			counts[index]++
		}
		if sample.MemoryTotal > 0 {
			memorySum[index] += float64(sample.MemoryUsed) / float64(sample.MemoryTotal) * 100
		}
		if sample.DiskTotal > 0 {
			diskSum[index] += float64(sample.DiskUsed) / float64(sample.DiskTotal) * 100
		}
	}
	for index := 0; index < points; index++ {
		if counts[index] > 0 {
			cpu[index] = roundPercent(cpuSum[index] / float64(counts[index]))
		}
		if seen[index] > 0 {
			memory[index] = roundPercent(memorySum[index] / float64(seen[index]))
			disk[index] = roundPercent(diskSum[index] / float64(seen[index]))
		}
	}
	cpuValue := percentOrNil(latest.CPUPercent)
	if latest.SampledAt == 0 {
		cpuValue = nil
	}
	response := overviewResponse{
		SampledAt: latest.SampledAt,
		Vantage:   "host",
		CPU:       overviewValue{Percent: cpuValue},
		Memory:    overviewValue{Used: latest.MemoryUsed, Total: latest.MemoryTotal, Percent: percent(latest.MemoryUsed, latest.MemoryTotal)},
		Disk:      overviewValue{Used: latest.DiskUsed, Total: latest.DiskTotal, Percent: percent(latest.DiskUsed, latest.DiskTotal)},
		Load1:     latest.Load1,
		Containers: map[string]int{
			"running": latest.ContainersRunning,
			"total":   latest.ContainersTotal,
		},
		Series: overviewSeries{From: from, To: to, Step: int64(overviewStep / time.Second), CPU: cpu, Memory: memory, Disk: disk},
		Thresholds: map[string]string{
			"cpu":    overviewState(samples, "cpu"),
			"memory": overviewState(samples, "memory"),
			"disk":   overviewState(samples, "disk"),
		},
	}
	if latest.SampledAt == 0 {
		response.Stale = true
		response.Vantage = "unavailable"
	} else {
		response.Stale = now.Unix()-latest.SampledAt > 180
	}
	if len(samples) == 0 {
		response.UnavailableCPU = true
	} else if latest.CPUPercent < 0 {
		response.Measuring = true
	}
	if latest.MemoryTotal == 0 && latest.SampledAt != 0 {
		response.Vantage = "limited"
	}
	return response
}

func percentOrNil(value float64) *float64 {
	if value < 0 {
		return nil
	}
	return roundPercent(value)
}

func (s *Server) systemOverview(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	since := now.Add(-overviewWindow).Unix()
	samples, e := s.db.MetricSamples(r.Context(), since)
	if e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, buildOverview(now, samples))
}
