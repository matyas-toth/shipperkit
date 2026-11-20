package deploy

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"shipper/internal/store"
)

func TestNamedDatabaseResourceSetsUserAndDatabaseOnce(t *testing.T) {
	c, e := NamedDatabaseResourceConfig("postgresql", "18", "shop", "orders")
	if e != nil {
		t.Fatal(e)
	}
	env := databaseEnvironment(c)
	if env["POSTGRES_USER"] != "shop" || env["POSTGRES_DB"] != "orders" {
		t.Fatalf("identity %v", env)
	}
	if e := ValidateDatabaseResource(c); e != nil {
		t.Fatal(e)
	}
	c, _ = NamedDatabaseResourceConfig("mongodb", "8", "", "")
	if env := databaseEnvironment(c); env["MONGO_INITDB_ROOT_USERNAME"] != "app" || env["MONGO_INITDB_DATABASE"] != "app" {
		t.Fatalf("defaults %v", env)
	}
	for _, tc := range [][2]string{{"root", "app"}, {"app", "admin"}, {"App", "app"}, {"app", "my-db"}, {"pg_user", "app"}, {"app", "1db"}} {
		if _, e := NamedDatabaseResourceConfig("mysql", "8.4", tc[0], tc[1]); e == nil {
			t.Fatalf("accepted %v", tc)
		}
	}
	if c, e := NamedDatabaseResourceConfig("redis", "8", "ignored", "ignored"); e != nil || databaseEnvironment(c)["REDIS_PASSWORD"] == "" {
		t.Fatal("redis has no user or database to name", e)
	}
}

func TestManagedMongoRunsAsReplicaSet(t *testing.T) {
	c, _ := DatabaseResourceConfig("mongodb", "8")
	d := store.Deployment{ID: "release", ApplicationID: "0123456789abcdef01234567", Config: c, ResolvedMethod: "image"}
	cfg := &container.Config{}
	configureMongo(d, cfg)
	if cfg.Hostname != "db-0123456789abcdef01234567" || !slices.Contains(cfg.Cmd, "--replSet") || !slices.Contains(cfg.Cmd, mongoKeyFile) || cfg.Entrypoint != nil {
		t.Fatalf("mongo config %+v", cfg)
	}
	if cfg.Healthcheck == nil || !strings.Contains(cfg.Healthcheck.Test[1], `host: "db-0123456789abcdef01234567:27017"`) || !strings.Contains(cfg.Healthcheck.Test[1], "isWritablePrimary") {
		t.Fatalf("health %v", cfg.Healthcheck)
	}
	// Plain Mongo images keep their own command.
	plain := &container.Config{}
	configureMongo(store.Deployment{Config: store.Config{Method: "image", Image: "mongo:8"}}, plain)
	if plain.Cmd != nil || plain.Hostname != "" {
		t.Fatalf("plain image changed %+v", plain)
	}
	raw := mongoConnection(store.BackupSpec{Host: "127.0.0.1", User: "app", Password: "secret"}, "app")
	u, _ := url.Parse(raw)
	if u.Query().Get("directConnection") != "true" || u.Query().Get("authSource") != "admin" {
		t.Fatal(raw)
	}
}

func TestRedisCacheBoundsMemory(t *testing.T) {
	c, _ := DatabaseResourceConfig("redis", "8")
	if args := redisCacheArgs(c); args != nil {
		t.Fatalf("default %v", args)
	}
	c.Database.Cache = true
	if args := strings.Join(redisCacheArgs(c), " "); args != "--maxmemory 384mb --maxmemory-policy allkeys-lru" {
		t.Fatal(args)
	}
	if e := ValidateDatabaseResource(c); e != nil {
		t.Fatal(e)
	}
	p, _ := DatabaseResourceConfig("postgresql", "18")
	p.Database.Cache = true
	if ValidateDatabaseResource(p) == nil {
		t.Fatal("postgres accepted cache mode")
	}
}

func TestDatabasePasswordRotationConfig(t *testing.T) {
	c, _ := DatabaseResourceConfig("mysql", "8.4")
	next, e := WithDatabasePassword(c, strings.Repeat("a", 64))
	if e != nil || databaseEnvironment(next)["MYSQL_PASSWORD"] != strings.Repeat("a", 64) || databaseEnvironment(c)["MYSQL_PASSWORD"] == strings.Repeat("a", 64) {
		t.Fatal("rotation must copy, not alias, the variables", e)
	}
	if store.ValidateDatabaseTransition(c, next) == nil {
		t.Fatal("a plain edit changed the password")
	}
	// Restoring an older release keeps the password the database uses now.
	old := c
	store.KeepDatabaseCredentials(&old, next)
	if databaseEnvironment(old)["MYSQL_PASSWORD"] != strings.Repeat("a", 64) {
		t.Fatal("rollback rewound credentials")
	}
	s, _ := DatabaseResourceConfig("sqlite", "3.50")
	if _, e := WithDatabasePassword(s, strings.Repeat("a", 64)); e == nil {
		t.Fatal("sqlite has no password")
	}
}

func TestDatabaseOptionsRecreateWithoutRebuilding(t *testing.T) {
	c, _ := DatabaseResourceConfig("redis", "8")
	next := c
	next.Database = &store.DatabaseResource{Engine: "redis", Version: "8", Cache: true}
	plan := PlanChanges(c, next)
	if plan.Operation != "recreate" || strings.Join(plan.Changes, ",") != "Database settings" {
		t.Fatalf("plan %+v", plan)
	}
}
