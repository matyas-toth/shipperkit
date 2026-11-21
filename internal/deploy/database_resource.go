package deploy

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"shipper/internal/auth"
	"shipper/internal/store"
	"strings"
	"time"
)

var databaseVersions = map[string]map[string]string{
	"postgresql": {"16": "postgres:16", "17": "postgres:17", "18": "postgres:18"},
	"mysql":      {"8.4": "mysql:8.4"}, "mariadb": {"11.4": "mariadb:11.4", "11.8": "mariadb:11.8"},
	"mongodb":    {"7": "mongo:7", "8": "mongo:8"},
	"clickhouse": {"24.8": "clickhouse/clickhouse-server:24.8", "25.3": "clickhouse/clickhouse-server:25.3"},
	"redis":      {"7": "redis:7", "8": "redis:8"},
	"sqlite":     {"3.50": "keinos/sqlite3:3.50.4"},
}

// DatabaseResourceConfig generates initialization secrets on the server only.
func DatabaseResourceConfig(engine, version string) (store.Config, error) {
	image := databaseVersions[engine][version]
	if image == "" {
		return store.Config{}, errors.New("Choose a supported database engine and version")
	}
	c := store.Config{Database: &store.DatabaseResource{Engine: engine, Version: version}, Method: "image", Image: image, StartupPolicy: "process", RootDirectory: ".", MemoryMB: 512, Runtime: store.RuntimePolicy{StartupSeconds: 300}, Variables: []store.Variable{}, Volumes: []store.Volume{}}
	add := func(name, value string, secret bool) {
		c.Variables = append(c.Variables, store.Variable{Name: name, Value: value, Secret: secret, Scope: "runtime"})
	}
	password := auth.Token()
	switch engine {
	case "postgresql":
		c.Port = 5432
		add("POSTGRES_USER", "app", false)
		add("POSTGRES_DB", "app", false)
		add("POSTGRES_PASSWORD", password, true)
		path := "/var/lib/postgresql/data"
		if version == "18" {
			path = "/var/lib/postgresql"
		}
		c.Volumes = append(c.Volumes, store.Volume{Name: "data", Path: path})
	case "mysql", "mariadb":
		c.Port = 3306
		prefix := "MYSQL_"
		if engine == "mariadb" {
			prefix = "MARIADB_"
		}
		add(prefix+"USER", "app", false)
		add(prefix+"DATABASE", "app", false)
		add(prefix+"PASSWORD", password, true)
		add(prefix+"ROOT_PASSWORD", auth.Token(), true)
		c.Volumes = append(c.Volumes, store.Volume{Name: "data", Path: "/var/lib/mysql"})
	case "redis":
		c.Port = 6379
		c.Volumes = append(c.Volumes, store.Volume{Name: "data", Path: "/data"})
		add("REDIS_PASSWORD", password, true)
	case "mongodb":
		c.Port = 27017
		add("MONGO_INITDB_ROOT_USERNAME", "app", false)
		add("MONGO_INITDB_ROOT_PASSWORD", password, true)
		add("MONGO_INITDB_DATABASE", "app", false)
		c.Volumes = append(c.Volumes, store.Volume{Name: "data", Path: "/data/db"})
	case "clickhouse":
		c.Port = 9000
		c.MemoryMB = 1024
		add("CLICKHOUSE_DB", "app", false)
		add("CLICKHOUSE_USER", "app", false)
		add("CLICKHOUSE_PASSWORD", password, true)
		add("CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT", "1", false)
		c.Volumes = append(c.Volumes, store.Volume{Name: "data", Path: "/var/lib/clickhouse"})
	case "sqlite":
		c.Port = 0
		c.MemoryMB = 128
		c.StartCommand = "touch /home/sqlite/app.db && exec sleep infinity"
		c.Volumes = append(c.Volumes, store.Volume{Name: "data", Path: "/home/sqlite"})
	}
	return c, nil
}

// DatabaseIdentityKeys names the variables holding the user and database an
// engine creates on its first start. Engines without them return empty keys.
func DatabaseIdentityKeys(engine string) (string, string) {
	switch engine {
	case "postgresql":
		return "POSTGRES_USER", "POSTGRES_DB"
	case "mysql":
		return "MYSQL_USER", "MYSQL_DATABASE"
	case "mariadb":
		return "MARIADB_USER", "MARIADB_DATABASE"
	case "mongodb":
		return "MONGO_INITDB_ROOT_USERNAME", "MONGO_INITDB_DATABASE"
	case "clickhouse":
		return "CLICKHOUSE_USER", "CLICKHOUSE_DB"
	}
	return "", ""
}

var databaseIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)
var reservedDatabaseUsers = map[string]bool{"root": true, "default": true, "admin": true, "mysql": true, "public": true}
var reservedDatabaseNames = map[string]bool{"admin": true, "local": true, "config": true, "mysql": true, "information_schema": true, "performance_schema": true, "sys": true, "system": true, "template0": true, "template1": true}

// NamedDatabaseResourceConfig sets the user and database once, at creation. The
// official images only read them while initializing an empty data directory.
func NamedDatabaseResourceConfig(engine, version, user, database string) (store.Config, error) {
	c, e := DatabaseResourceConfig(engine, version)
	if e != nil {
		return c, e
	}
	userKey, databaseKey := DatabaseIdentityKeys(engine)
	if userKey == "" {
		return c, nil
	}
	user, database = strings.TrimSpace(user), strings.TrimSpace(database)
	if user == "" {
		user = "app"
	}
	if database == "" {
		database = "app"
	}
	if !databaseIdentifier.MatchString(user) || reservedDatabaseUsers[user] || strings.HasPrefix(user, "pg_") {
		return c, errors.New("Use a user name of lowercase letters, digits and underscores that is not reserved")
	}
	if !databaseIdentifier.MatchString(database) || reservedDatabaseNames[database] || strings.HasPrefix(database, "pg_") {
		return c, errors.New("Use a database name of lowercase letters, digits and underscores that is not reserved")
	}
	for i := range c.Variables {
		switch c.Variables[i].Name {
		case userKey:
			c.Variables[i].Value = user
		case databaseKey:
			c.Variables[i].Value = database
		}
	}
	return c, nil
}
func databaseEnvironment(c store.Config) map[string]string {
	m := map[string]string{}
	for _, v := range c.Variables {
		if v.Scope == "" || v.Scope == "runtime" || v.Scope == "both" {
			m[v.Name] = v.Value
		}
	}
	return m
}
// databaseTLSCapable engines keep offering TLS inside the private network, so
// clients that ask for it still get it. None of them require it.
func databaseTLSCapable(engine string) bool {
	return engine == "postgresql" || engine == "mysql" || engine == "mariadb"
}
func ValidateDatabaseResource(c store.Config) error {
	if c.Database == nil {
		return nil
	}
	preset, e := DatabaseResourceConfig(c.Database.Engine, c.Database.Version)
	if e != nil {
		return e
	}
	if c.Method != "image" || c.Image != preset.Image || c.StartupPolicy != "process" || c.Port != preset.Port || c.StartCommand != preset.StartCommand || len(c.Hooks) > 0 {
		return errors.New("Managed databases use their engine's image, port and startup command")
	}
	if len(c.Volumes) != 1 || c.Volumes[0] != preset.Volumes[0] {
		return errors.New("Keep the database data volume at its engine's data directory")
	}
	if c.Database.Cache && c.Database.Engine != "redis" {
		return errors.New("Only Redis databases run as a cache")
	}
	if domain := c.Database.Domain; domain != "" {
		if store.DatabaseDomainPorts[c.Database.Engine] == 0 {
			return errors.New("MySQL, MariaDB and ClickHouse connect from outside through a published port")
		}
		if domain != strings.ToLower(domain) || len(domain) > 253 || !domainPattern.MatchString(domain) {
			return errors.New("Enter a domain such as db.example.com")
		}
		for _, p := range c.Networking.Ports {
			if p.Protocol == "tcp" && p.HostPort == store.DatabaseDomainPorts[c.Database.Engine] {
				return fmt.Errorf("Port %d serves the public domain. Remove it from published ports", p.HostPort)
			}
		}
	}
	env := databaseEnvironment(c)
	for _, v := range preset.Variables {
		if env[v.Name] == "" {
			return errors.New("Keep all generated database credentials")
		}
		if v.Secret {
			protected := false
			for _, actual := range c.Variables {
				if actual.Name == v.Name && actual.Secret {
					protected = true
				}
			}
			if !protected {
				return errors.New("Keep database passwords marked as secrets")
			}
		}
		if v.Secret && (len(env[v.Name]) < 24 || strings.ContainsAny(env[v.Name], "\x00\r\n")) {
			return errors.New("Database passwords must contain at least 24 characters")
		}
	}
	return nil
}

// EnsureDatabaseBackupPlans retries reconciliation through the durable worker after crashes.
func (w *Worker) EnsureDatabaseBackupPlans(ctx context.Context) error {
	apps, e := w.DB.Applications(ctx)
	if e != nil {
		return e
	}
	for _, a := range apps {
		if a.Config.Database == nil || a.ActiveDeploymentID == "" {
			continue
		}
		plans, e := w.DB.BackupPlans(ctx, a.ID)
		if e != nil {
			return e
		}
		if len(plans) > 0 {
			continue
		}
		d, e := w.DB.Deployment(ctx, a.ActiveDeploymentID)
		if e != nil {
			return e
		}
		if d.Config.Database == nil {
			continue
		}
		c, e := DatabaseBackupSpec(d)
		if e != nil {
			return e
		}
		if c.Kind == "volume" {
			runtime, ok := w.Runtime.(*DockerRuntime)
			if !ok {
				continue
			}
			c.Volume = runtime.prefix() + "-" + a.ID + "-data"
		}
		next, e := NextSchedule("0 3 * * *", "UTC", time.Now())
		if e != nil {
			return e
		}
		if e = w.DB.SaveBackupPlan(ctx, store.BackupPlan{ID: "database-" + a.ID, ApplicationID: a.ID, Name: "Daily backup", Config: c, Enabled: true, Schedule: "0 3 * * *", Timezone: "UTC", NextAt: next.Unix()}); e != nil {
			return e
		}
	}
	return nil
}
func DatabaseBackupSpec(d store.Deployment) (store.BackupSpec, error) {
	if d.Config.Database == nil {
		return store.BackupSpec{}, errors.New("Choose a managed database")
	}
	c := store.BackupSpec{Kind: "database", Database: "app", Host: "127.0.0.1", TLS: "require", Keep: 7, Engine: d.Config.Database.Engine}
	env := databaseEnvironment(d.Config)
	switch c.Engine {
	case "postgresql":
		c.Engine = "postgres"
		c.User = env["POSTGRES_USER"]
		c.Password = env["POSTGRES_PASSWORD"]
		c.Database = env["POSTGRES_DB"]
	case "mysql":
		c.User = "root"
		c.Password = env["MYSQL_ROOT_PASSWORD"]
		c.Database = env["MYSQL_DATABASE"]
	case "mariadb":
		c.User = "root"
		c.Password = env["MARIADB_ROOT_PASSWORD"]
		c.Database = env["MARIADB_DATABASE"]
	case "redis":
		c.Kind = "volume"
		c.Engine = "redis"
		c.Database = ""
	case "mongodb":
		c.Engine = "mongodb"
		c.User = env["MONGO_INITDB_ROOT_USERNAME"]
		c.Password = env["MONGO_INITDB_ROOT_PASSWORD"]
		c.Database = env["MONGO_INITDB_DATABASE"]
	case "clickhouse", "sqlite":
		c.Kind = "volume"
		c.Engine = d.Config.Database.Engine
		c.Database = ""
	}
	return c, nil
}
