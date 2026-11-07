package deploy

import (
	"net/url"
	"shipper/internal/store"
	"strings"
	"testing"
)

func TestDatabaseConnections(t *testing.T) {
	for _, tc := range []struct {
		image, user, password, database, scheme string
		env                                     map[string]string
	}{
		{"postgres:17", "postgres", "p@ss:/?#%", "postgres", "postgresql", map[string]string{"POSTGRES_PASSWORD": "p@ss:/?#%"}},
		{"docker.io/library/postgres:17", "team@user", "secret", "my/db ?", "postgresql", map[string]string{"POSTGRES_USER": "team@user", "POSTGRES_PASSWORD": "secret", "POSTGRES_DB": "my/db ?"}},
		{"mysql:8", "app", "secret", "shop", "mysql", map[string]string{"MYSQL_USER": "app", "MYSQL_PASSWORD": "secret", "MYSQL_DATABASE": "shop"}},
		{"mariadb:11", "app", "maria", "shop", "mysql", map[string]string{"MYSQL_PASSWORD": "wrong", "MARIADB_USER": "app", "MARIADB_PASSWORD": "maria", "MARIADB_DATABASE": "shop"}},
		{"mongo:8", "root", "secret", "shop", "mongodb", map[string]string{"MONGO_INITDB_ROOT_USERNAME": "root", "MONGO_INITDB_ROOT_PASSWORD": "secret", "MONGO_INITDB_DATABASE": "shop"}},
		{"redis:7-alpine", "", "", "0", "redis", nil},
		{"valkey/valkey:8", "", "", "0", "redis", nil},
	} {
		t.Run(tc.image, func(t *testing.T) {
			d := store.Deployment{ID: "release", ContainerID: "container", ResolvedMethod: "image", Config: store.Config{Method: "image", Image: tc.image}}
			for name, value := range tc.env {
				d.Config.Variables = append(d.Config.Variables, store.Variable{Name: name, Value: value, Scope: "runtime"})
			}
			raw, err := DatabaseURI(d)
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			password, _ := u.User.Password()
			if u.Scheme != tc.scheme || u.Hostname() != "site-release" || u.User.Username() != tc.user || password != tc.password || u.Path != "/"+tc.database || u.Fragment != "" {
				t.Fatalf("Incorrect connection components for %s", tc.image)
			}
			if tc.scheme == "mongodb" && u.Query().Get("authSource") != "admin" {
				t.Fatal("Mongo root authentication database missing")
			}
		})
	}
	d := store.Deployment{ID: "release", ContainerID: "container", ResolvedMethod: "image", Config: store.Config{Method: "image", Image: "postgres:17", Variables: []store.Variable{{Name: "POSTGRES_PASSWORD", Value: "build-only", Scope: "build"}}}}
	if _, err := DatabaseURI(d); err == nil {
		t.Fatal("Build-only credentials accepted")
	}
	d.Config.Variables = []store.Variable{{Name: "POSTGRES_PASSWORD_FILE", Value: "/run/secrets/password"}}
	if _, err := DatabaseURI(d); err == nil {
		t.Fatal("File credential guessed")
	}
	d.Config.Image = "redis:7"
	d.Config.Variables = nil
	d.RuntimePort = 6380
	if uri, err := DatabaseURI(d); err != nil || !strings.Contains(uri, ":6380/") {
		t.Fatal("Runtime port ignored", err)
	}
	d.Config.StartCommand = "redis-server --requirepass secret"
	if _, err := DatabaseURI(d); err == nil {
		t.Fatal("Custom authentication guessed")
	}
	d.Config.StartCommand = ""
	d.Config.Variables = []store.Variable{{Name: "REDIS_PASSWORD", Value: "ignored"}}
	if _, err := DatabaseURI(d); err == nil {
		t.Fatal("Unsupported Redis variable treated as authentication")
	}
	for _, image := range []string{"example/postgres:17", "ghcr.io/example/postgres:17", "nginx:alpine"} {
		d.Config.Image = image
		if DatabaseEndpointFor(d) != nil {
			t.Fatal("Unrecognized image accepted", image)
		}
	}
	d.Config.Image = "postgres:17"
	d.ResolvedMethod = "compose"
	if DatabaseEndpointFor(d) != nil {
		t.Fatal("Compose database exposed")
	}
}

func TestDatabaseCapabilityScope(t *testing.T) {
	for _, tc := range []struct {
		method, image string
		task          bool
		want          int
	}{
		{"image", "postgres:17", false, 5},
		{"image", "redis:7-alpine", false, 5},
		{"image", "redis:7-alpine", true, 0},
		{"image", "custom/redis:7", false, 0},
		{"dockerfile", "postgres:17", false, 0},
		{"static", "", false, 0},
	} {
		d := store.Deployment{ResolvedMethod: tc.method, Config: store.Config{Method: tc.method, Image: tc.image}}
		if tc.task {
			d.TaskCommand = &store.Command{}
		}
		if got := len(databaseCapabilities(d)); got != tc.want {
			t.Fatalf("Wrong capability allowance for %s %s task=%v", tc.method, tc.image, tc.task)
		}
	}
}
