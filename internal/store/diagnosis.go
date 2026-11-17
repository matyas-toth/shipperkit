package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) SaveDeploymentDiagnosis(ctx context.Context, deploymentID, applicationID string, capturedAt int64, snapshot []byte) error {
	_, e := s.db.ExecContext(ctx, `INSERT INTO deployment_diagnoses(deployment_id,application_id,captured_at,snapshot_json)
		VALUES(?,?,?,?)
		ON CONFLICT(deployment_id) DO UPDATE SET application_id=excluded.application_id,captured_at=excluded.captured_at,snapshot_json=excluded.snapshot_json`,
		deploymentID, applicationID, capturedAt, string(snapshot))
	return e
}

func (s *Store) DeploymentDiagnosis(ctx context.Context, deploymentID string) ([]byte, bool, error) {
	var raw string
	e := s.db.QueryRowContext(ctx, "SELECT snapshot_json FROM deployment_diagnoses WHERE deployment_id=?", deploymentID).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	return []byte(raw), true, nil
}

func (s *Store) PruneDeploymentDiagnoses(ctx context.Context, before int64) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM deployment_diagnoses WHERE captured_at<?", before)
	return e
}
