package deploy

import (
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
				if e != nil || parsed.Scheme != "rediss" || parsed.Hostname() != "db-resource" {
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
		if tc.engine == "mongodb" && u.Query().Get("authSource") != "admin" {
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
	c.Database.TLSHostname = "db.example.test"
	if e = ValidateDatabaseResource(c); e == nil {
		t.Fatal("SQLite accepted public TLS")
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
	files, e := r.databaseCertificate("resource", "db.example.test")
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
	for _, host := range []string{"db-resource", "app-resource", "localhost", "127.0.0.1", "db.example.test"} {
		if _, e = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: host}); e != nil {
			t.Fatal(host, e)
		}
	}
	if _, e = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "other.example.test"}); e == nil {
		t.Fatal("incorrect hostname accepted")
	}
	next, e := r.databaseCertificate("resource", "next.example.test")
	if e != nil || string(next["ca.crt"]) != string(files["ca.crt"]) {
		t.Fatal("CA changed across releases", e)
	}
	root, _ := r.databaseTLSRoot("resource")
	if info, e := os.Stat(root); e != nil || !info.IsDir() {
		t.Fatal(e)
	}
	if _, e = r.databaseCertificate("../escape", ""); e == nil {
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
