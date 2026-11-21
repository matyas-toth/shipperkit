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
	// Applications is accepted for older clients and ignored. Every application
	// shares one private network, so there is nothing to connect.
	Applications []string        `json:"applications,omitempty"`
	Ports        []PublishedPort `json:"ports"`
}

var ErrPortInUse = errors.New("The public port is reserved by another application")
var ErrDatabaseDomainInUse = errors.New("Another database already uses this domain")

// Network claims are checked inside the same SQLite write transaction as the
// desired configuration. Concurrent enqueue requests cannot claim the same port.
func (s *Store) checkNetworkClaims(ctx context.Context, tx *sql.Tx, app string, c Config) error {
	access := c.Networking
	domain, domainPort := "", 0
	if c.Database != nil && c.Database.Domain != "" {
		domain, domainPort = c.Database.Domain, DatabaseDomainPorts[c.Database.Engine]
	}
	if len(access.Ports) == 0 && domain == "" {
		return nil
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
			Networking NetworkAccess     `json:"networking"`
			Database   *DatabaseResource `json:"database"`
		}
		if err = json.Unmarshal([]byte(raw), &config); err != nil {
			return err
		}
		otherDomain, otherPort := "", 0
		if config.Database != nil && config.Database.Domain != "" {
			otherDomain, otherPort = config.Database.Domain, DatabaseDomainPorts[config.Database.Engine]
		}
		if domain != "" && otherDomain == domain && otherPort == domainPort {
			return ErrDatabaseDomainInUse
		}
		for _, other := range config.Networking.Ports {
			if other.HostPort == domainPort && other.Protocol == "tcp" {
				return fmt.Errorf("%w: %d/tcp", ErrPortInUse, domainPort)
			}
			for _, port := range access.Ports {
				if other.HostPort == port.HostPort && other.Protocol == port.Protocol {
					return fmt.Errorf("%w: %d/%s", ErrPortInUse, port.HostPort, port.Protocol)
				}
			}
		}
		for _, port := range access.Ports {
			if otherPort != 0 && port.HostPort == otherPort && port.Protocol == "tcp" {
				return fmt.Errorf("%w: %d/tcp", ErrPortInUse, port.HostPort)
			}
		}
	}
	return rows.Err()
}
