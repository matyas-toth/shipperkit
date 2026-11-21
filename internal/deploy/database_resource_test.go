package deploy

import (
	"strings"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"os"
	"shipper/internal/auth"
	"shipper/internal/events"
	"shipper/internal/store"
	"strconv"
	"testing"
)

func TestDatabaseResourcePresets(t *testing.T) {
	for engine, versions := range databaseVersions {
		for version := range versions {
			a, e := DatabaseResourceConfig(engine, version)
			if e != nil {
				t.Fatal(e)
			}
			b, _ := DatabaseResourceConfig(engine, version)
			if len(a.Volumes) != 1 || a.StartupPolicy != "process" || a.Image == "" || a.Database.Engine != engine {
				t.Fatal(a)
			}
			if e = ValidateBuild(&a); e != nil {
				t.Fatal(engine, version, e)
			}
			for _, v := range a.Variables {
				if v.Secret {
					if databaseEnvironment(b)[v.Name] == v.Value {
						t.Fatal("credentials reused")
					}
					for _, pub := range store.PublicConfig(a).Variables {
						if pub.Name == v.Name && pub.Value != "" {
							t.Fatal("secret disclosed")
						}
					}
				}
			}
			if engine == "redis" {
				d := store.Deployment{ID: "release", ApplicationID: "resource", ContainerID: "container", Config: a, ResolvedMethod: "image"}
				uri, e := DatabaseURI(d)
				if e != nil {
					t.Fatal(e)
				}
				parsed, e := url.Parse(uri)
				if e != nil || parsed.Scheme != "redis" || parsed.Hostname() != "db-resource" {
					t.Fatal(uri, e)
				}
				password, _ := parsed.User.Password()
				if password != databaseEnvironment(a)["REDIS_PASSWORD"] {
					t.Fatal("wrong Redis credential")
				}
				a.StartCommand = "redis-server --protected-mode no"
				d.Config = a
				if _, e = DatabaseURI(d); e == nil {
					t.Fatal("custom Redis auth inferred")
				}
			}
		}
	}
	if _, e := DatabaseResourceConfig("postgresql", "latest;evil"); e == nil {
		t.Fatal("unvetted version accepted")
	}
}
func TestDatabaseResourceEngineConnections(t *testing.T) {
	cases := []struct {
		engine, version, scheme, database string
		port                              int
		user, password                    string
	}{
		{"mongodb", "8", "mongodb", "app", 27017, "app", "MONGO_INITDB_ROOT_PASSWORD"},
		{"clickhouse", "25.3", "clickhouse", "app", 9000, "app", "CLICKHOUSE_PASSWORD"},
	}
	for _, tc := range cases {
		c, e := DatabaseResourceConfig(tc.engine, tc.version)
		if e != nil {
			t.Fatal(tc.engine, e)
		}
		d := store.Deployment{ID: "release", ApplicationID: "resource", ContainerID: "container", Config: c, ResolvedMethod: "image"}
		raw, e := DatabaseURI(d)
		if e != nil {
			t.Fatal(tc.engine, e)
		}
		u, e := url.Parse(raw)
		if e != nil || u.Scheme != tc.scheme || u.Hostname() != "db-resource" || u.Port() != strconv.Itoa(tc.port) || u.Path != "/"+tc.database || u.User.Username() != tc.user {
			t.Fatal(tc.engine, raw, e)
		}
		password, _ := u.User.Password()
		if password != databaseEnvironment(c)[tc.password] {
			t.Fatal(tc.engine, "wrong password")
		}
		if tc.engine == "mongodb" && (u.Query().Get("authSource") != "admin" || u.Query().Get("replicaSet") != MongoReplicaSet) {
			t.Fatal("Mongo authentication database missing")
		}
	}
	c, e := DatabaseResourceConfig("sqlite", "3.50")
	if e != nil {
		t.Fatal(e)
	}
	d := store.Deployment{ID: "release", ApplicationID: "resource", ContainerID: "container", Config: c, ResolvedMethod: "image"}
	if raw, e := DatabaseURI(d); e != nil || raw != "sqlite:///home/sqlite/app.db" {
		t.Fatal(raw, e)
	}
	c.Database.Domain = "db.example.test"
	if e = ValidateDatabaseResource(c); e == nil {
		t.Fatal("SQLite accepted a public domain")
	}
}

func TestDatabaseConnectionStrings(t *testing.T) {
	cases := []struct {
		engine, version, private, public, legacy string
	}{
		{"postgresql", "18", "postgresql://app:%s@db-resource:5432/app", "postgresql://app:%s@db.example.test:5432/app?sslmode=require", "postgresql://app:%s@db-resource:5432/app?sslmode=require"},
		{"redis", "8", "redis://default:%s@db-resource:6379/0", "rediss://default:%s@db.example.test:6379/0", "rediss://default:%s@db-resource:6379/0"},
		{"mongodb", "8", "mongodb://app:%s@db-resource:27017/app?authSource=admin&replicaSet=rs0", "mongodb://app:%s@db.example.test:27017/app?authSource=admin&directConnection=true&tls=true", "mongodb://app:%s@db-resource:27017/app?authSource=admin&replicaSet=rs0"},
	}
	for _, tc := range cases {
		c, e := DatabaseResourceConfig(tc.engine, tc.version)
		if e != nil {
			t.Fatal(e)
		}
		password := ""
		for _, v := range c.Variables {
			if v.Secret {
				password = url.UserPassword("x", v.Value).String()[2:]
			}
		}
		d := store.Deployment{ID: "release", ApplicationID: "resource", ContainerID: "container", Config: c, ResolvedMethod: "image"}
		if _, e = DatabaseConnectionURI(d, DatabaseAccess{Public: true}); e == nil {
			t.Fatal(tc.engine, "public string without a domain")
		}
		d.Config.Database.Domain = "db.example.test"
		if e = ValidateDatabaseResource(d.Config); e != nil {
			t.Fatal(tc.engine, e)
		}
		for _, check := range []struct {
			access DatabaseAccess
			want   string
		}{{DatabaseAccess{}, tc.private}, {DatabaseAccess{Public: true}, tc.public}, {DatabaseAccess{LegacyTLS: true}, tc.legacy}} {
			got, e := DatabaseConnectionURI(d, check.access)
			if want := strings.Replace(check.want, "%s", password, 1); e != nil || got != want {
				t.Fatalf("%s %+v\n got %s\nwant %s (%v)", tc.engine, check.access, got, want, e)
			}
		}
	}
	for _, engine := range []string{"mysql", "mariadb"} {
		version := map[string]string{"mysql": "8.4", "mariadb": "11.8"}[engine]
		c, _ := DatabaseResourceConfig(engine, version)
		c.Database.Domain = "db.example.test"
		if e := ValidateDatabaseResource(c); e == nil {
			t.Fatal(engine, "accepted a domain the proxy cannot terminate")
		}
	}
	c, _ := DatabaseResourceConfig("postgresql", "18")
	for _, domain := range []string{"DB.example.test", "db", "db..example.test", "-db.example.test"} {
		c.Database.Domain = domain
		if e := ValidateDatabaseResource(c); e == nil {
			t.Fatal("accepted domain", domain)
		}
	}
	c.Database.Domain = "db.example.test"
	c.Networking.Ports = []store.PublishedPort{{Protocol: "tcp", BindIP: "0.0.0.0", HostPort: 5432, ContainerPort: 5432}}
	if e := ValidateDatabaseResource(c); e == nil || !strings.Contains(e.Error(), "Port 5432 serves the public domain") {
		t.Fatal("published the domain's port", e)
	}
}

func TestDatabaseDomainRoutes(t *testing.T) {
	postgres, _ := DatabaseResourceConfig("postgresql", "18")
	postgres.Database.Domain = "pg.example.test"
	redis, _ := DatabaseResourceConfig("redis", "8")
	redis.Database.Domain = "cache.example.test"
	private, _ := DatabaseResourceConfig("mongodb", "8")
	apps := []store.Application{{ID: "pg"}, {ID: "cache"}, {ID: "mongo"}}
	active := map[string]store.Deployment{
		"pg":    {ID: "release-pg", ApplicationID: "pg", Config: postgres, RuntimePort: 5432},
		"cache": {ID: "release-cache", ApplicationID: "cache", Config: redis, RuntimePort: 6379},
		"mongo": {ID: "release-mongo", ApplicationID: "mongo", Config: private, RuntimePort: 27017},
	}
	if got := proxyDatabases(apps, active); strings.Join(got, ",") != "postgresql,redis" {
		t.Fatal("entrypoints", got)
	}
	tcp, _ := Routes(apps, active)["tcp"].(map[string]any)
	routers, _ := tcp["routers"].(map[string]any)
	services, _ := tcp["services"].(map[string]any)
	if len(routers) != 2 || len(services) != 2 {
		t.Fatal("routes", tcp)
	}
	pg := routers["db-pg"].(map[string]any)
	if pg["rule"] != "HostSNI(`pg.example.test`)" || pg["entryPoints"].([]string)[0] != "postgresql" || pg["tls"].(map[string]any)["certResolver"] != "letsencrypt" {
		t.Fatal("postgres router", pg)
	}
	server := services["db-cache"].(map[string]any)["loadBalancer"].(map[string]any)["servers"].([]any)[0].(map[string]string)
	if server["address"] != "site-release-cache:6379" {
		t.Fatal("redis service", server)
	}
	if _, ok := Routes(nil, nil)["tcp"]; ok {
		t.Fatal("tcp routes without databases")
	}
}
func TestDatabaseResourceBackupPlans(t *testing.T) {
	for _, engine := range []string{"mongodb", "clickhouse", "sqlite"} {
		version := map[string]string{"mongodb": "8", "clickhouse": "25.3", "sqlite": "3.50"}[engine]
		c, e := DatabaseResourceConfig(engine, version)
		if e != nil {
			t.Fatal(engine, e)
		}
		spec, e := DatabaseBackupSpec(store.Deployment{ApplicationID: "resource", Config: c})
		if e != nil {
			t.Fatal(engine, e)
		}
		if engine == "mongodb" {
			if spec.Kind != "database" || spec.Engine != "mongodb" || spec.User == "" || spec.Password == "" || spec.Database != "app" {
				t.Fatal("unexpected MongoDB backup spec", spec)
			}
		} else if spec.Kind != "volume" || spec.Database != "" {
			t.Fatal("unexpected volume backup spec", engine, spec)
		}
	}
}
func TestDatabaseResourceNativeTLSIdentity(t *testing.T) {
	r := DockerRuntime{DataDir: t.TempDir()}
	files, e := r.databaseCertificate("resource")
	if e != nil {
		t.Fatal(e)
	}
	block, _ := pem.Decode(files["server.crt"])
	cert, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(files["ca.crt"])
	for _, host := range []string{"db-resource", "app-resource", "localhost", "127.0.0.1"} {
		if _, e = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: host}); e != nil {
			t.Fatal(host, e)
		}
	}
	if _, e = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "other.example.test"}); e == nil {
		t.Fatal("incorrect hostname accepted")
	}
	next, e := r.databaseCertificate("resource")
	if e != nil || string(next["ca.crt"]) != string(files["ca.crt"]) {
		t.Fatal("CA changed across releases", e)
	}
	root, _ := r.databaseTLSRoot("resource")
	if info, e := os.Stat(root); e != nil || !info.IsDir() {
		t.Fatal(e)
	}
	if _, e = r.databaseCertificate("../escape"); e == nil {
		t.Fatal("unsafe TLS directory")
	}
}
func TestDatabaseResourceDefaultPlansSurviveRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	c, _ := DatabaseResourceConfig("postgresql", "18")
	a := store.Application{ID: auth.Token()[:24], Name: "Database", Config: c}
	d, e := db.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	d.State = "succeeded"
	db.SaveDeployment(ctx, d)
	db.SetActive(ctx, a.ID, d.ID)
	for i := 0; i < 2; i++ {
		w := NewWorker(db, nil, events.New(), dir)
		if e = w.EnsureDatabaseBackupPlans(ctx); e != nil {
			t.Fatal(e)
		}
	}
	plans, e := db.BackupPlans(ctx, a.ID)
	if e != nil || len(plans) != 1 || plans[0].Config.Engine != "postgres" || plans[0].Config.Password == "" || !plans[0].Enabled || plans[0].NextAt == 0 {
		t.Fatal(plans, e)
	}
	raw, e := os.ReadFile(dir + string(os.PathSeparator) + "shipper.db")
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte(plans[0].Config.Password)) {
		t.Fatal("backup password stored in plaintext")
	}
}
