package store

import "errors"

// databaseCredentialKeys are only read while the images initialize their data.
var databaseCredentialKeys = []string{"POSTGRES_USER", "POSTGRES_DB", "POSTGRES_PASSWORD", "MYSQL_USER", "MYSQL_DATABASE", "MYSQL_PASSWORD", "MYSQL_ROOT_PASSWORD", "MARIADB_USER", "MARIADB_DATABASE", "MARIADB_PASSWORD", "MARIADB_ROOT_PASSWORD", "REDIS_PASSWORD", "MONGO_INITDB_ROOT_USERNAME", "MONGO_INITDB_ROOT_PASSWORD", "MONGO_INITDB_DATABASE", "CLICKHOUSE_USER", "CLICKHOUSE_PASSWORD", "CLICKHOUSE_DB"}

// ValidateDatabaseTransition protects initialization-only credentials and data
// formats. Upgrades need a new database; passwords change through a rotation.
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
	for _, key := range databaseCredentialKeys {
		if oldEnv[key] != nextEnv[key] {
			return errors.New("Database initialization credentials cannot change after creation; rotate the password in database settings")
		}
	}
	return nil
}

// KeepDatabaseCredentials carries the credentials a database uses now into an
// earlier release, since restoring a release never rewinds its stored users.
func KeepDatabaseCredentials(next *Config, current Config) {
	if next.Database == nil || current.Database == nil {
		return
	}
	values := map[string]Variable{}
	for _, v := range current.Variables {
		values[v.Name] = v
	}
	next.Variables = append([]Variable(nil), next.Variables...)
	for i, v := range next.Variables {
		if kept, ok := values[v.Name]; ok && databaseCredential(v.Name) {
			next.Variables[i] = kept
		}
	}
}

func databaseCredential(name string) bool {
	for _, key := range databaseCredentialKeys {
		if key == name {
			return true
		}
	}
	return false
}
