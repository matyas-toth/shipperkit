package store

import "errors"

// ValidateDatabaseTransition protects initialization-only engine credentials and
// data formats. Upgrades and credential rotation require an engine-aware workflow.
func ValidateDatabaseTransition(old, next Config) error {
	if old.Database == nil {
		return nil
	}
	if next.Database == nil || next.Database.Engine != old.Database.Engine || next.Database.Version != old.Database.Version {
		return errors.New("Create a new database and import a tested backup to change its engine or version")
	}
	oldEnv, nextEnv := map[string]string{}, map[string]string{}
	for _, v := range old.Variables {
		if v.Scope == "" || v.Scope == "runtime" || v.Scope == "both" {
			oldEnv[v.Name] = v.Value
		}
	}
	for _, v := range next.Variables {
		if v.Scope == "" || v.Scope == "runtime" || v.Scope == "both" {
			nextEnv[v.Name] = v.Value
		}
	}
	for _, key := range []string{"POSTGRES_USER", "POSTGRES_DB", "POSTGRES_PASSWORD", "MYSQL_USER", "MYSQL_DATABASE", "MYSQL_PASSWORD", "MYSQL_ROOT_PASSWORD", "MARIADB_USER", "MARIADB_DATABASE", "MARIADB_PASSWORD", "MARIADB_ROOT_PASSWORD", "REDIS_PASSWORD"} {
		if oldEnv[key] != nextEnv[key] {
			return errors.New("Database initialization credentials cannot change after creation; use an engine-aware credential rotation")
		}
	}
	return nil
}
