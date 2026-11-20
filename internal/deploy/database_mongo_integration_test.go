package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/store"
)

// An existing standalone database converts in place, serves transactions to a
// connected application, survives a password rotation and still backs up.
func TestDockerManagedMongoReplicaSet(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_DATABASE_RESOURCES") != "1" {
		t.Skip("set SHIPPER_TEST_DATABASE_RESOURCES=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	docker, e := engine.New()
	if e != nil {
		t.Fatal(e)
	}
	defer docker.Close()
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	r := &DockerRuntime{Client: docker.Client(), Instance: auth.Token(), DataDir: dir}
	_ = NewWorker(db, r, events.New(), dir)
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		cs, _ := r.Client.ContainerList(clean, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		for _, c := range cs.Items {
			_ = r.removeOwned(clean, c.ID, "")
		}
		vs, _ := r.Client.VolumeList(clean, client.VolumeListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		for _, v := range vs.Items {
			_, _ = r.Client.VolumeRemove(clean, v.Name, client.VolumeRemoveOptions{})
		}
		ns, _ := r.Client.NetworkList(clean, client.NetworkListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		for _, n := range ns.Items {
			_, _ = r.Client.NetworkRemove(clean, n.ID, client.NetworkRemoveOptions{})
		}
	})
	c, e := NamedDatabaseResourceConfig("mongodb", "8", "shop", "orders")
	if e != nil {
		t.Fatal(e)
	}
	if e = r.pull(ctx, c.Image, func(string) {}); e != nil {
		t.Fatal(e)
	}
	app := store.Application{ID: auth.Token()[:24], Name: "mongo", Config: c}
	start := func(config store.Config) store.Deployment {
		t.Helper()
		d := store.Deployment{ID: auth.Token()[:24], ApplicationID: app.ID, Config: config, ImageID: config.Image, ResolvedMethod: "image", RuntimePort: 27017}
		if d.ContainerID, e = r.Start(ctx, d); e != nil {
			t.Fatal(e)
		}
		if e = r.Healthy(ctx, d); e != nil {
			logs, _ := r.Client.ContainerLogs(ctx, d.ContainerID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Tail: "40"})
			if logs != nil {
				raw := new(strings.Builder)
				buf := make([]byte, 32<<10)
				n, _ := logs.Read(buf)
				raw.Write(buf[:n])
				t.Log(raw.String())
			}
			t.Fatal("Database did not become ready", e)
		}
		return d
	}
	mongosh := func(id, uri, script string) (string, error) {
		out, err := r.execEnv(ctx, id, []string{"sh", "-c", `exec mongosh "$URI" --quiet --norc --eval "$SCRIPT"`}, []string{"URI=" + uri, "SCRIPT=" + script})
		return strings.TrimSpace(string(out)), err
	}
	local := func(config store.Config) string {
		env := databaseEnvironment(config)
		return mongoConnection(store.BackupSpec{Host: "127.0.0.1", User: env["MONGO_INITDB_ROOT_USERNAME"], Password: env["MONGO_INITDB_ROOT_PASSWORD"]}, "orders")
	}

	// A database created before replica sets: the same volume and credentials,
	// started as a plain standalone server.
	legacy := c
	legacy.Database = nil
	old := store.Deployment{ID: auth.Token()[:24], ApplicationID: app.ID, Config: legacy, ImageID: c.Image, ResolvedMethod: "image"}
	if old.ContainerID, e = r.Start(ctx, old); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		if _, e = mongosh(old.ContainerID, local(c), `db.items.insertOne({ name: "kept" })`); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("standalone server never accepted writes", e)
		}
		time.Sleep(time.Second)
	}
	if _, e = mongosh(old.ContainerID, local(c), `db.items.insertOne({ name: "rs.status" }); try { rs.status(); quit(1) } catch (e) {}`); e != nil {
		t.Fatal("legacy server already ran a replica set", e)
	}
	if e = r.removeOwned(ctx, old.ContainerID, app.ID); e != nil {
		t.Fatal(e)
	}

	d := start(c)
	if out, e := mongosh(d.ContainerID, local(c), `print(rs.status().set + " " + rs.conf().members[0].host + " " + db.items.countDocuments())`); e != nil || out != "rs0 db-"+app.ID+":27017 2" {
		t.Fatalf("conversion: %q %v", out, e)
	}
	if e = r.reconcileApplicationAliases(ctx, map[string]store.Deployment{app.ID: d}); e != nil {
		t.Fatal(e)
	}
	consumerConfig := store.Config{Method: "image", Image: c.Image, StartupPolicy: "process", StartCommand: "sleep 900", Networking: store.NetworkAccess{Applications: []string{app.ID}}}
	consumer := store.Deployment{ID: auth.Token()[:24], ApplicationID: auth.Token()[:24], Config: consumerConfig, ImageID: c.Image, ResolvedMethod: "image"}
	if consumer.ContainerID, e = r.Start(ctx, consumer); e != nil {
		t.Fatal(e)
	}
	uri, e := DatabaseURI(d)
	if e != nil || !strings.Contains(uri, "replicaSet=rs0") || !strings.Contains(uri, "/orders?") {
		t.Fatal(uri, e)
	}
	// What Prisma needs: a multi-document transaction through the copied string,
	// with the driver discovering the member by its private name.
	transaction := `const s = db.getMongo().startSession(); s.startTransaction(); s.getDatabase("orders").items.insertOne({ name: "atomic" }); s.commitTransaction(); print(db.getSiblingDB("orders").items.countDocuments())`
	if out, e := mongosh(consumer.ContainerID, uri, transaction); e != nil || out != "3" {
		t.Fatalf("transaction: %q %v", out, e)
	}

	// Rotation changes the stored user first, then a release carries the new secret.
	password := auth.Token()
	if e = r.SetDatabasePassword(ctx, d, password); e != nil {
		t.Fatal(e)
	}
	next, e := WithDatabasePassword(c, password)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.removeOwned(ctx, d.ContainerID, app.ID); e != nil {
		t.Fatal(e)
	}
	rotated := start(next)
	if e = r.reconcileApplicationAliases(ctx, map[string]store.Deployment{app.ID: rotated}); e != nil {
		t.Fatal(e)
	}
	if _, e = mongosh(consumer.ContainerID, uri, `db.items.countDocuments()`); e == nil {
		t.Fatal("the old password still signs in")
	}
	uri, _ = DatabaseURI(rotated)
	if out, e := mongosh(consumer.ContainerID, uri, transaction); e != nil || out != "4" {
		t.Fatalf("rotated transaction: %q %v", out, e)
	}
	spec, e := DatabaseBackupSpec(rotated)
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(dir, "backup.archive")
	if _, e = r.DumpDatabase(ctx, rotated, spec, file); e != nil {
		t.Fatal("backup after conversion", e)
	}
	if info, e := os.Stat(file); e != nil || info.Size() == 0 {
		t.Fatal("empty backup", e)
	}
}
