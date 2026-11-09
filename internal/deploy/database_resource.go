package deploy

import (
	"context"
	"errors"
	"net"
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
func databaseEnvironment(c store.Config) map[string]string {
	m := map[string]string{}
	for _, v := range c.Variables {
		if v.Scope == "" || v.Scope == "runtime" || v.Scope == "both" {
			m[v.Name] = v.Value
		}
	}
	return m
}
func databaseTLSCapable(engine string) bool {
	return engine == "postgresql" || engine == "mysql" || engine == "mariadb" || engine == "redis"
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
	host := c.Database.TLSHostname
	if host != "" && !databaseTLSCapable(c.Database.Engine) {
		return errors.New("This database engine does not expose TLS through Shipper")
	}
	if host != "" && net.ParseIP(host) == nil && (!regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9.-]{0,251}[a-zA-Z0-9])?$`).MatchString(host) || strings.Contains(host, "..")) {
		return errors.New("Enter a database TLS hostname or IP address")
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
