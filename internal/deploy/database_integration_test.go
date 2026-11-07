package deploy

import (
	"context"
	"github.com/moby/moby/client"
	"os"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/store"
	"strings"
	"testing"
	"time"
)

func TestDockerDatabaseConnectionBetweenApplications(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_DATABASE") != "1" {
		t.Skip("set SHIPPER_TEST_DATABASE=1 for real Docker networking")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	engine, err := engine.New()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	r := &DockerRuntime{Client: engine.Client(), Instance: auth.Token()}
	if _, err = r.Client.NetworkCreate(ctx, r.network(), client.NetworkCreateOptions{Driver: "bridge", Labels: r.labels("", "")}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		containers, err := r.Client.ContainerList(clean, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		if err != nil {
			t.Error(err)
			return
		}
		for _, c := range containers.Items {
			if err = r.removeOwned(clean, c.ID, ""); err != nil {
				t.Error(err)
			}
		}
		volumes, err := r.Client.VolumeList(clean, client.VolumeListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		if err != nil {
			t.Error(err)
			return
		}
		for _, v := range volumes.Items {
			if owned(v.Labels, r.Instance, "") {
				if _, err = r.Client.VolumeRemove(clean, v.Name, client.VolumeRemoveOptions{}); err != nil {
					t.Error(err)
				}
			}
		}
		if n, err := r.Client.NetworkInspect(clean, r.network(), client.NetworkInspectOptions{}); err == nil && owned(n.Network.Labels, r.Instance, "") {
			if _, err = r.Client.NetworkRemove(clean, n.Network.ID, client.NetworkRemoveOptions{}); err != nil {
				t.Error(err)
			}
		}
	})
	const image = "redis:7-alpine"
	if err = r.pull(ctx, image, func(line string) { t.Log(line) }); err != nil {
		t.Fatal(err)
	}
	start := func(app string, config store.Config, port int) store.Deployment {
		t.Helper()
		d := store.Deployment{ID: auth.Token()[:24], ApplicationID: app, ResolvedMethod: "image", ImageID: config.Image, RuntimePort: port, Config: config}
		var err error
		d.ContainerID, err = r.Start(ctx, d)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	redis := store.Config{Method: "image", Image: image, StartupPolicy: "process", MemoryMB: 64, Volumes: []store.Volume{{Name: "data", Path: "/data"}}}
	database, other := start("database", redis, 6379), start("consumer", redis, 6379)
	uri, err := DatabaseURI(database)
	if err != nil {
		t.Fatal(err)
	}
	ready := false
	var lastOutput string
	var lastError error
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
		out, err := r.exec(ctx, other.ContainerID, []string{"redis-cli", "-u", uri, "PING"})
		lastOutput, lastError = string(out), err
		if err == nil && strings.TrimSpace(string(out)) == "PONG" {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		logCtx, stopLogs := context.WithTimeout(ctx, 2*time.Second)
		defer stopLogs()
		_ = r.Logs(logCtx, other.ApplicationID, other.ContainerID, 20, func(line string) error { t.Log(line); return nil })
		_ = r.Logs(logCtx, database.ApplicationID, database.ContainerID, 20, func(line string) error { t.Log(line); return nil })
		t.Fatal("Another Shipper application could not connect using the generated URI", lastOutput, lastError)
	}
	if out, err := r.exec(ctx, other.ContainerID, []string{"redis-cli", "-u", uri, "SET", "shipper-test", "connected"}); err != nil || strings.TrimSpace(string(out)) != "OK" {
		t.Fatal("Cross-application write failed", err)
	}
	if out, err := r.exec(ctx, database.ContainerID, []string{"redis-cli", "GET", "shipper-test"}); err != nil || strings.TrimSpace(string(out)) != "connected" {
		t.Fatal("Database did not receive the write", err)
	}
	info, err := r.Client.ContainerInspect(ctx, database.ContainerID, client.ContainerInspectOptions{})
	if err != nil || len(info.Container.HostConfig.PortBindings) != 0 {
		t.Fatal("Database ports were publicly bound", err)
	}
	// Verify a password-protected SQL image as well as Redis, including URL escaping.
	const postgresImage = "postgres:17-alpine"
	if err = r.pull(ctx, postgresImage, func(line string) { t.Log(line) }); err != nil {
		t.Fatal(err)
	}
	postgres := store.Config{Method: "image", Image: postgresImage, StartupPolicy: "process", MemoryMB: 128, Volumes: []store.Volume{{Name: "data", Path: "/var/lib/postgresql/data"}}, Variables: []store.Variable{{Name: "POSTGRES_PASSWORD", Value: "fixture@password:/?#%", Secret: true, Scope: "runtime"}, {Name: "POSTGRES_DB", Value: "shop", Scope: "runtime"}}}
	sqlDatabase, sqlClient := start("postgres-database", postgres, 5432), start("postgres-consumer", postgres, 5432)
	sqlURI, err := DatabaseURI(sqlDatabase)
	if err != nil {
		t.Fatal(err)
	}
	sqlReady := false
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); {
		out, err := r.exec(ctx, sqlClient.ContainerID, []string{"psql", sqlURI, "-qAtc", "CREATE TEMP TABLE shipper_connection (value integer); INSERT INTO shipper_connection VALUES (42); SELECT value FROM shipper_connection;"})
		lastOutput, lastError = string(out), err
		if err == nil && strings.TrimSpace(string(out)) == "42" {
			sqlReady = true
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !sqlReady {
		logCtx, stopLogs := context.WithTimeout(ctx, 2*time.Second)
		defer stopLogs()
		_ = r.Logs(logCtx, sqlDatabase.ApplicationID, sqlDatabase.ContainerID, 20, func(line string) error { t.Log(line); return nil })
		t.Fatal("PostgreSQL could not complete a cross-application authenticated read/write", lastOutput, lastError)
	}
	info, err = r.Client.ContainerInspect(ctx, sqlDatabase.ContainerID, client.ContainerInspectOptions{})
	if err != nil || len(info.Container.HostConfig.PortBindings) != 0 {
		t.Fatal("SQL database ports were publicly bound", err)
	}

}
