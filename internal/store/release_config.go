package store

import (
	"context"
	"database/sql"
)

// UpdateDeploymentConfig rewrites a release's stored configuration. It is only
// for settings that take effect without replacing the release's containers,
// such as the public domain of a database.
func (s *Store) UpdateDeploymentConfig(ctx context.Context, id string, change func(*Config)) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.updateConfigRow(ctx, tx, "deployments", id, change); e != nil {
		return e
	}
	return tx.Commit()
}

// AddVolumes records persistent volumes on both the application and one of its
// releases, skipping any path that is already declared. Both rows change
// together so the application shows no pending change afterwards.
func (s *Store) AddVolumes(ctx context.Context, appID, deploymentID string, volumes []Volume) error {
	if len(volumes) == 0 {
		return nil
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	add := func(c *Config) {
		for _, v := range volumes {
			found := false
			for _, existing := range c.Volumes {
				found = found || existing.Path == v.Path || existing.Name == v.Name
			}
			if !found {
				c.Volumes = append(c.Volumes, v)
			}
		}
	}
	if e = s.updateConfigRow(ctx, tx, "applications", appID, add); e != nil {
		return e
	}
	if e = s.updateConfigRow(ctx, tx, "deployments", deploymentID, add); e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Store) updateConfigRow(ctx context.Context, tx *sql.Tx, table, id string, change func(*Config)) error {
	var raw string
	if e := tx.QueryRowContext(ctx, "SELECT config_json FROM "+table+" WHERE id=?", id).Scan(&raw); e != nil {
		return e
	}
	var c Config
	if e := s.unmarshalConfig(raw, &c); e != nil {
		return e
	}
	change(&c)
	next, e := s.marshalConfig(c)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "UPDATE "+table+" SET config_json=? WHERE id=?", next, id)
	return e
}
