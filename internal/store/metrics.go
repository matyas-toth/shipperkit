package store

import (
	"context"
	"database/sql"
	"errors"
)

// MetricSample is one bounded host measurement. CPU is -1 until a second
// sample exists and a delta can be calculated.
type MetricSample struct {
	SampledAt         int64   `json:"sampledAt"`
	CPUPercent        float64 `json:"cpuPercent"`
	MemoryUsed        int64   `json:"memoryUsed"`
	MemoryTotal       int64   `json:"memoryTotal"`
	DiskUsed          int64   `json:"diskUsed"`
	DiskTotal         int64   `json:"diskTotal"`
	Load1             float64 `json:"load1"`
	ContainersRunning int     `json:"containersRunning"`
	ContainersTotal   int     `json:"containersTotal"`
}

func (s *Store) SaveMetricSample(ctx context.Context, v MetricSample) error {
	_, e := s.db.ExecContext(ctx, `INSERT INTO metric_samples(sampled_at,cpu_percent,memory_used,memory_total,disk_used,disk_total,load1,containers_running,containers_total)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(sampled_at) DO UPDATE SET cpu_percent=excluded.cpu_percent,memory_used=excluded.memory_used,memory_total=excluded.memory_total,disk_used=excluded.disk_used,disk_total=excluded.disk_total,load1=excluded.load1,containers_running=excluded.containers_running,containers_total=excluded.containers_total`,
		v.SampledAt, v.CPUPercent, v.MemoryUsed, v.MemoryTotal, v.DiskUsed, v.DiskTotal, v.Load1, v.ContainersRunning, v.ContainersTotal)
	return e
}

func (s *Store) MetricSamples(ctx context.Context, since int64) ([]MetricSample, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT sampled_at,cpu_percent,memory_used,memory_total,disk_used,disk_total,load1,containers_running,containers_total
		FROM metric_samples WHERE sampled_at>=? ORDER BY sampled_at`, since)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []MetricSample{}
	for rows.Next() {
		var v MetricSample
		if e = rows.Scan(&v.SampledAt, &v.CPUPercent, &v.MemoryUsed, &v.MemoryTotal, &v.DiskUsed, &v.DiskTotal, &v.Load1, &v.ContainersRunning, &v.ContainersTotal); e != nil {
			return nil, e
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *Store) LatestMetricSample(ctx context.Context) (MetricSample, bool, error) {
	var v MetricSample
	e := s.db.QueryRowContext(ctx, `SELECT sampled_at,cpu_percent,memory_used,memory_total,disk_used,disk_total,load1,containers_running,containers_total
		FROM metric_samples ORDER BY sampled_at DESC LIMIT 1`).Scan(&v.SampledAt, &v.CPUPercent, &v.MemoryUsed, &v.MemoryTotal, &v.DiskUsed, &v.DiskTotal, &v.Load1, &v.ContainersRunning, &v.ContainersTotal)
	if errors.Is(e, sql.ErrNoRows) {
		return MetricSample{}, false, nil
	}
	return v, e == nil, e
}

func (s *Store) PruneMetricSamples(ctx context.Context, before int64) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM metric_samples WHERE sampled_at<?", before)
	return e
}
