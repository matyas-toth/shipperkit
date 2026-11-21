package store

import (
	"context"
	"time"
)

// TelemetryCounts holds instance-wide totals for anonymous usage reports. It
// never carries names, hostnames, repositories, emails or any other value an
// operator typed in.
type TelemetryCounts struct {
	Users            int
	FactorUsers      int
	LinkedIdentities int
	AccessTokens     int
	BackupPlans      int
	Tasks            int
	Connections      map[string]int
	Deployments      int
	FailedDeploys    int
	Rollbacks        int
	DeployMedianSec  int64
}

// TelemetryCounts summarises activity since the given time with plain
// aggregate queries so a report costs a handful of index scans.
func (s *Store) TelemetryCounts(ctx context.Context, since time.Time) (TelemetryCounts, error) {
	out := TelemetryCounts{Connections: map[string]int{}}
	scalars := []struct {
		query  string
		target *int
	}{
		{"SELECT COUNT(*) FROM administrators WHERE disabled=0", &out.Users},
		{"SELECT COUNT(*) FROM account_factors", &out.FactorUsers},
		{"SELECT COUNT(DISTINCT user_id) FROM user_identities", &out.LinkedIdentities},
		{"SELECT COUNT(*) FROM backup_plans WHERE enabled=1", &out.BackupPlans},
		{"SELECT COUNT(*) FROM tasks t JOIN applications a ON a.id=t.application_id WHERE a.deleted_at=0", &out.Tasks},
	}
	for _, item := range scalars {
		if err := s.db.QueryRowContext(ctx, item.query).Scan(item.target); err != nil {
			return out, err
		}
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM operation_tokens WHERE expires_at>?", time.Now().Unix()).Scan(&out.AccessTokens); err != nil {
		return out, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT kind,COUNT(*) FROM connections GROUP BY kind")
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var kind string
		var count int
		if err = rows.Scan(&kind, &count); err != nil {
			rows.Close()
			return out, err
		}
		out.Connections[kind] = count
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	cutoff := since.Unix()
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN state='failed' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN operation='rollback' THEN 1 ELSE 0 END),0)
		FROM deployments WHERE created_at>=?`, cutoff).Scan(&out.Deployments, &out.FailedDeploys, &out.Rollbacks); err != nil {
		return out, err
	}
	// Rows come back sorted, so the median is the middle value.
	durations, err := s.db.QueryContext(ctx, `SELECT finished_at-started_at FROM deployments
		WHERE created_at>=? AND state='succeeded' AND started_at>0 AND finished_at>=started_at
		ORDER BY finished_at-started_at LIMIT 1000`, cutoff)
	if err != nil {
		return out, err
	}
	values := []int64{}
	for durations.Next() {
		var value int64
		if err = durations.Scan(&value); err != nil {
			durations.Close()
			return out, err
		}
		values = append(values, value)
	}
	durations.Close()
	if len(values) > 0 {
		out.DeployMedianSec = values[len(values)/2]
	}
	return out, durations.Err()
}
