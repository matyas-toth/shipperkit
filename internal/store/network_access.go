package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type PublishedPort struct {
	Service       string `json:"service,omitempty"`
	Protocol      string `json:"protocol"`
	BindIP        string `json:"bindIP"`
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
}

type NetworkAccess struct {
	Applications []string        `json:"applications"`
	Ports        []PublishedPort `json:"ports"`
}

var ErrPortInUse = errors.New("The public port is reserved by another application")

// Network claims are checked inside the same SQLite write transaction as the
// desired configuration. Concurrent enqueue requests cannot claim the same port.
func (s *Store) checkNetworkClaims(ctx context.Context, tx *sql.Tx, app string, access NetworkAccess) error {
	if len(access.Ports) == 0 && len(access.Applications) == 0 {
		return nil
	}
	for _, target := range access.Applications {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM applications WHERE id=? AND deleted_at=0 AND source_kind IN ('git','compose','image','inline','local') AND method!=''", target).Scan(&count); err != nil {
			return err
		}
		if count != 1 || target == app {
			return errors.New("Choose another managed application")
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT config_json FROM applications WHERE id!=? AND deleted_at=0 UNION ALL SELECT d.config_json FROM applications a JOIN deployments d ON d.id=a.active_deployment_id WHERE a.id!=? AND a.deleted_at=0`, app, app)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return err
		}
		var config struct {
			Networking NetworkAccess `json:"networking"`
		}
		if err = json.Unmarshal([]byte(raw), &config); err != nil {
			return err
		}
		for _, other := range config.Networking.Ports {
			for _, port := range access.Ports {
				if other.HostPort == port.HostPort && other.Protocol == port.Protocol {
					return fmt.Errorf("%w: %d/%s", ErrPortInUse, port.HostPort, port.Protocol)
				}
			}
		}
	}
	return rows.Err()
}
